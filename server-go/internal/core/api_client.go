package core

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/xuziyue/douyin-downloader/internal/utils"
)

const (
	baseURL           = "https://www.douyin.com"
	loginRequiredCode = 2483
)

var userAgentPool = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Safari/537.36",
}

// LoginRequiredError signals that Douyin rejected a request because
// the session is not logged in.
type LoginRequiredError struct {
	StatusCode int
	StatusMsg  string
	Path       string
}

func (e *LoginRequiredError) Error() string {
	return fmt.Sprintf("login required (status_code=%d) at %s: %s", e.StatusCode, e.Path, e.StatusMsg)
}

func isLoginRequired(data map[string]any) bool {
	code, _ := toInt(data["status_code"])
	msg := fmt.Sprintf("%v", data["status_msg"])
	return code == loginRequiredCode || strings.Contains(msg, "请先登录")
}

// DouyinAPIClient is the HTTP client for Douyin's web API.
type DouyinAPIClient struct {
	cookies       map[string]string
	client        *http.Client
	headers       map[string]string
	signer        *utils.XBogus
	abogusEnabled bool
}

// NewDouyinAPIClient creates a new API client.
func NewDouyinAPIClient(cookies map[string]string, proxy string) *DouyinAPIClient {
	cookies = utils.SanitizeCookies(cookies)
	// Persisted msToken is frequently stale; sending a stale/invalid msToken
	// triggers Douyin's WAF (HTTP 403). These read endpoints work fine without
	// it, so drop it from the cookie jar rather than risk an expired token.
	delete(cookies, "msToken")
	ua := userAgentPool[rand.Intn(len(userAgentPool))]

	transport := &http.Transport{
		MaxIdleConns:        10,
		IdleConnTimeout:     30 * time.Second,
		DisableCompression:  false,
		MaxIdleConnsPerHost: 5,
	}
	if proxy != "" {
		if proxyURL, err := url.Parse(proxy); err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	}

	return &DouyinAPIClient{
		cookies: cookies,
		client: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		},
		headers: map[string]string{
			"User-Agent":      ua,
			"Referer":         "https://www.douyin.com/?recommend=1",
			"Accept":          "*/*",
			"Accept-Language": "zh-CN,zh;q=0.9,en-US;q=0.8,en;q=0.7",
		},
		signer:        utils.NewXBogus(ua),
		abogusEnabled: true,
	}
}

func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	case json.Number:
		i, err := n.Int64()
		return int(i), err == nil
	case string:
		return 0, false
	}
	return 0, false
}

func toInt64(v any) int64 {
	switch n := v.(type) {
	case int:
		return int64(n)
	case int64:
		return n
	case float64:
		return int64(n)
	case json.Number:
		i, _ := n.Int64()
		return i
	}
	return 0
}

func (c *DouyinAPIClient) defaultQuery() url.Values {
	params := url.Values{
		"device_platform":      {"webapp"},
		"aid":                  {"6383"},
		"channel":              {"channel_pc_web"},
		"update_version_code":  {"170400"},
		"pc_client_type":       {"1"},
		"pc_libra_divert":      {"Windows"},
		"version_code":         {"290100"},
		"version_name":         {"29.1.0"},
		"cookie_enabled":       {"true"},
		"screen_width":         {"1536"},
		"screen_height":        {"864"},
		"browser_language":     {"zh-CN"},
		"browser_platform":     {"Win32"},
		"browser_name":         {"Chrome"},
		"browser_version":      {"139.0.0.0"},
		"browser_online":       {"true"},
		"engine_name":          {"Blink"},
		"engine_version":       {"139.0.0.0"},
		"os_name":              {"Windows"},
		"os_version":           {"10"},
		"cpu_core_num":         {"16"},
		"device_memory":        {"8"},
		"platform":             {"PC"},
		"downlink":             {"10"},
		"effective_type":       {"4g"},
		"round_trip_time":      {"200"},
		"support_h265":         {"1"},
		"support_dash":         {"1"},
		"uifid":                {""},
	}
	// msToken is intentionally omitted: a stale persisted token triggers
	// Douyin's WAF (403), and these endpoints succeed without it.
	return params
}

// BuildSignedPath constructs a signed URL for the given API path and params.
func (c *DouyinAPIClient) BuildSignedPath(path string, params url.Values) (string, string) {
	query := params.Encode()
	baseURLStr := baseURL + path

	// Try A-Bogus first
	if c.abogusEnabled {
		if signed, ua := c.buildAbogusURL(baseURLStr, query); signed != "" {
			return signed, ua
		}
	}

	// Fallback to X-Bogus
	fullURL := baseURLStr + "?" + query
	signedURL, _, ua := c.signer.Build(fullURL)
	return signedURL, ua
}

func (c *DouyinAPIClient) buildAbogusURL(baseURLStr, query string) (string, string) {
	fp := utils.GenerateFingerprint("Chrome")
	ab := utils.NewABogus(fp, c.headers["User-Agent"])
	signedParams, _, ua, _ := ab.GenerateAbogus(query, "")
	return baseURLStr + "?" + signedParams, ua
}

// RequestJSON performs an UNSIGNED GET and returns the parsed JSON, retrying a
// few times with backoff. Douyin rotates its signing algorithm; the reverse-
// engineered signers (a_bogus / X-Bogus) here fall out of sync and their
// signatures get rejected by Douyin's WAF (HTTP 403 / verify page). An unsigned
// request with a valid cookie still succeeds for these read endpoints, so
// signing is intentionally omitted (see doGet).
func (c *DouyinAPIClient) RequestJSON(ctx context.Context, path string, params url.Values, maxRetries int) (map[string]any, error) {
	if maxRetries <= 0 {
		maxRetries = 3
	}
	delays := []time.Duration{1 * time.Second, 2 * time.Second, 5 * time.Second}
	baseURLStr := baseURL + path

	// doGet issues one UNSIGNED GET. Signing (a_bogus / X-Bogus) is intentionally
	// disabled here: the reverse-engineered signers produce signatures Douyin's
	// WAF now rejects (a single bad signature triggers a verify page), while an
	// unsigned request with a valid cookie succeeds for these read endpoints.
	doGet := func() (map[string]any, error) {
		fullURL := baseURLStr + "?" + params.Encode()
		ua := c.headers["User-Agent"]

		req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
		if err != nil {
			return nil, err
		}
		for k, v := range c.headers {
			req.Header.Set(k, v)
		}
		req.Header.Set("User-Agent", ua)
		for k, v := range c.cookies {
			req.AddCookie(&http.Cookie{Name: k, Value: v})
		}

		resp, err := c.client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			// 403 etc. = WAF/anti-bot rejection; retry after backoff.
			return nil, &httpStatusError{code: resp.StatusCode, path: path}
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		if len(body) == 0 {
			// Empty 200 — anti-bot signal.
			return nil, errEmpty200
		}

		var data map[string]any
		if err := json.Unmarshal(body, &data); err != nil {
			// Non-JSON 200 — usually a verify/captcha page.
			slog.Warn("Non-JSON 200 response", "path", path, "len", len(body))
			return nil, errNonJSON
		}

		if isLoginRequired(data) {
			code, _ := toInt(data["status_code"])
			return nil, &LoginRequiredError{
				StatusCode: code,
				StatusMsg:  fmt.Sprintf("%v", data["status_msg"]),
				Path:       path,
			}
		}
		return data, nil
	}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		// Douyin's风控 responses (403 / verify page / empty 200) are intermittent,
		// so retry a few times with backoff.
		data, err := doGet()
		if err == nil {
			return data, nil
		}
		lastErr = err
		// Login-required is terminal — stop immediately.
		if _, ok := err.(*LoginRequiredError); ok {
			return nil, err
		}

		if attempt < maxRetries-1 {
			select {
			case <-time.After(delays[min(attempt, len(delays)-1)]):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}

	slog.Error("Request failed after retries", "path", path, "error", lastErr)
	if lastErr == nil {
		lastErr = fmt.Errorf("request failed for %s", path)
	}
	return map[string]any{}, lastErr
}

// httpStatusError carries a non-200 HTTP status.
type httpStatusError struct {
	code int
	path string
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("HTTP %d for %s", e.code, e.path)
}

var (
	errEmpty200 = fmt.Errorf("empty 200 response (anti-bot)")
	errNonJSON  = fmt.Errorf("non-JSON 200 response (verify page)")
)

// --- API Methods ---

// GetVideoDetail fetches details for a single video.
func (c *DouyinAPIClient) GetVideoDetail(ctx context.Context, awemeID string) (map[string]any, error) {
	var lastStatus int
	var lastMsg string
	gotResponse := false
	for _, aid := range []string{"6383", "1128"} {
		params := c.defaultQuery()
		params.Set("aweme_id", awemeID)
		params.Set("aid", aid)

		data, err := c.RequestJSON(ctx, "/aweme/v1/web/aweme/detail/", params, 3)
		if err != nil {
			return nil, err
		}
		if len(data) == 0 {
			// Empty 200 (anti-bot) — try the other aid.
			continue
		}
		gotResponse = true
		if detail, ok := data["aweme_detail"].(map[string]any); ok && detail != nil {
			return detail, nil
		}
		// aweme_detail absent — capture Douyin's own status for diagnostics.
		if sc, ok := toInt(data["status_code"]); ok {
			lastStatus = sc
		}
		if sm := strings.TrimSpace(fmt.Sprintf("%v", data["status_msg"])); sm != "" && sm != "<nil>" {
			lastMsg = sm
		}
		slog.Warn("aweme_detail absent in detail response",
			"aweme_id", awemeID, "aid", aid,
			"status_code", data["status_code"], "status_msg", data["status_msg"])
	}

	if !gotResponse {
		return nil, fmt.Errorf("详情接口无响应(疑似风控/网络,多次空 200)")
	}
	if lastStatus != 0 || lastMsg != "" {
		return nil, fmt.Errorf("抖音拒绝: status_code=%d %s(Cookie 可能过期或被风控)", lastStatus, lastMsg)
	}
	return nil, fmt.Errorf("响应中无 aweme_detail(视频可能已删除/私密)")
}

// ResolveShortURL follows a short URL redirect to get the real URL.
func (c *DouyinAPIClient) ResolveShortURL(ctx context.Context, shortURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", shortURL, nil)
	if err != nil {
		return "", err
	}
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}

	// Don't follow redirects — we want the Location header
	c.client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	defer func() { c.client.CheckRedirect = nil }()

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	resp.Body.Close()

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return resp.Header.Get("Location"), nil
	}
	return "", nil
}

// UserAgent returns the client's User-Agent string.
func (c *DouyinAPIClient) UserAgent() string {
	return c.headers["User-Agent"]
}

// Client returns the underlying HTTP client (for streaming).
func (c *DouyinAPIClient) Client() *http.Client {
	return c.client
}
