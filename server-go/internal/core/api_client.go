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
	"sync"
	"time"

	"github.com/xuziyue/douyin-downloader/internal/auth"
	"github.com/xuziyue/douyin-downloader/internal/utils"
)

const (
	baseURL            = "https://www.douyin.com"
	msTokenTTL         = 30 * time.Minute
	loginRequiredCode  = 2483
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

// PagedResponse is the normalized result of paginated API calls.
type PagedResponse struct {
	Items     []map[string]any
	HasMore   bool
	MaxCursor int64
	Status    int
	Raw       map[string]any
}

// DouyinAPIClient is the HTTP client for Douyin's web API.
type DouyinAPIClient struct {
	cookies      map[string]string
	Proxy        string
	client       *http.Client
	headers      map[string]string
	signer       *utils.XBogus
	msTokenMgr   *auth.MsTokenManager

	mu               sync.Mutex
	msToken          string
	msTokenAcquiredAt time.Time
	empty200Streak   int
	abogusEnabled    bool
}

// NewDouyinAPIClient creates a new API client.
func NewDouyinAPIClient(cookies map[string]string, proxy string) *DouyinAPIClient {
	cookies = utils.SanitizeCookies(cookies)
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
		Proxy:   strings.TrimSpace(proxy),
		client: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		},
		headers: map[string]string{
			"User-Agent":      ua,
			"Referer":         "https://www.douyin.com/?recommend=1",
			"Accept":          "*/*",
			"Accept-Encoding": "gzip, deflate",
			"Accept-Language": "zh-CN,zh;q=0.9,en-US;q=0.8,en;q=0.7",
		},
		signer:         utils.NewXBogus(ua),
		msTokenMgr:     auth.NewMsTokenManager(ua),
		msToken:        strings.TrimSpace(cookies["msToken"]),
		abogusEnabled:  true,
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

// ensureMSToken refreshes the msToken if expired.
func (c *DouyinAPIClient) ensureMSToken() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.msToken != "" && time.Since(c.msTokenAcquiredAt) < msTokenTTL {
		return c.msToken
	}

	token := c.msTokenMgr.EnsureMSToken(c.cookies)
	c.msToken = strings.TrimSpace(token)
	c.msTokenAcquiredAt = time.Now()
	if c.msToken != "" {
		c.cookies["msToken"] = c.msToken
	}
	return c.msToken
}

func (c *DouyinAPIClient) defaultQuery() url.Values {
	msToken := c.ensureMSToken()
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
		"msToken":              {msToken},
	}
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

// RequestJSON performs a signed GET request and returns the parsed JSON.
func (c *DouyinAPIClient) RequestJSON(ctx context.Context, path string, params url.Values, maxRetries int) (map[string]any, error) {
	if maxRetries <= 0 {
		maxRetries = 3
	}
	delays := []time.Duration{1 * time.Second, 2 * time.Second, 5 * time.Second}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		signedURL, ua := c.BuildSignedPath(path, params)

		req, err := http.NewRequestWithContext(ctx, "GET", signedURL, nil)
		if err != nil {
			lastErr = err
			continue
		}
		for k, v := range c.headers {
			req.Header.Set(k, v)
		}
		req.Header.Set("User-Agent", ua)

		// Set cookies
		for k, v := range c.cookies {
			req.AddCookie(&http.Cookie{Name: k, Value: v})
		}

		resp, err := c.client.Do(req)
		if err != nil {
			lastErr = err
			if attempt < maxRetries-1 {
				select {
				case <-time.After(delays[min(attempt, len(delays)-1)]):
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			continue
		}

		if resp.StatusCode == 200 {
			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				lastErr = err
				continue
			}
			if len(body) == 0 {
				// Empty 200 — anti-bot signal
				slog.Warn("Empty 200 response", "path", path, "attempt", attempt+1)
				c.mu.Lock()
				c.empty200Streak++
				if c.empty200Streak >= 2 {
					slog.Info("Invalidating msToken after empty-200s", "streak", c.empty200Streak)
					c.msToken = ""
					c.msTokenAcquiredAt = time.Time{}
				}
				c.mu.Unlock()
				lastErr = fmt.Errorf("empty 200 response for %s (anti-bot)", path)
				if attempt < maxRetries-1 {
					select {
					case <-time.After(delays[min(attempt, len(delays)-1)]):
					case <-ctx.Done():
						return nil, ctx.Err()
					}
				}
				continue
			}

			var data map[string]any
			if err := json.Unmarshal(body, &data); err != nil {
				slog.Warn("Non-JSON 200 response", "path", path, "len", len(body))
				return map[string]any{}, nil
			}

			c.mu.Lock()
			c.empty200Streak = 0
			c.mu.Unlock()

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

		resp.Body.Close()
		if resp.StatusCode < 500 && resp.StatusCode != 429 {
			slog.Error("Request failed", "path", path, "status", resp.StatusCode)
			return map[string]any{}, nil
		}
		lastErr = fmt.Errorf("HTTP %d for %s", resp.StatusCode, path)

		if attempt < maxRetries-1 {
			select {
			case <-time.After(delays[min(attempt, len(delays)-1)]):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}

	slog.Error("Request failed after retries", "path", path, "error", lastErr)
	return map[string]any{}, lastErr
}

// normalizePagedResponse extracts items, has_more, and cursor from a raw response.
func normalizePagedResponse(raw map[string]any, itemKeys ...string) *PagedResponse {
	keys := append([]string{"items"}, append(itemKeys, "aweme_list", "mix_list", "music_list")...)
	var items []map[string]any
	for _, key := range keys {
		if val, ok := raw[key]; ok {
			if arr, ok := val.([]any); ok {
				for _, item := range arr {
					if m, ok := item.(map[string]any); ok {
						items = append(items, m)
					}
				}
				break
			}
		}
	}

	hasMore := false
	if hm, ok := raw["has_more"]; ok {
		switch v := hm.(type) {
		case bool:
			hasMore = v
		case float64:
			hasMore = int(v) != 0
		}
	}

	maxCursor := toInt64(raw["max_cursor"])
	if maxCursor == 0 {
		maxCursor = toInt64(raw["cursor"])
	}

	status, _ := toInt(raw["status_code"])

	return &PagedResponse{
		Items:     items,
		HasMore:   hasMore,
		MaxCursor: maxCursor,
		Status:    status,
		Raw:       raw,
	}
}

// --- API Methods ---

// GetVideoDetail fetches details for a single video.
func (c *DouyinAPIClient) GetVideoDetail(ctx context.Context, awemeID string) (map[string]any, error) {
	for _, aid := range []string{"6383", "1128"} {
		params := c.defaultQuery()
		params.Set("aweme_id", awemeID)
		params.Set("aid", aid)

		data, err := c.RequestJSON(ctx, "/aweme/v1/web/aweme/detail/", params, 3)
		if err != nil {
			return nil, err
		}
		if len(data) == 0 {
			continue
		}
		if detail, ok := data["aweme_detail"].(map[string]any); ok && detail != nil {
			return detail, nil
		}
	}
	return nil, nil
}

// GetUserPost fetches a page of a user's posted videos.
func (c *DouyinAPIClient) GetUserPost(ctx context.Context, secUID string, maxCursor int64, count int) (*PagedResponse, error) {
	if count <= 0 {
		count = 18
	}
	params := c.defaultQuery()
	params.Set("sec_user_id", secUID)
	params.Set("max_cursor", fmt.Sprintf("%d", maxCursor))
	params.Set("count", fmt.Sprintf("%d", count))
	params.Set("locate_query", "false")
	params.Set("show_live_replay_strategy", "1")
	params.Set("need_time_list", "1")
	params.Set("time_list_query", "0")
	params.Set("whale_cut_token", "")
	params.Set("cut_version", "1")
	params.Set("publish_video_strategy_type", "2")

	raw, err := c.RequestJSON(ctx, "/aweme/v1/web/aweme/post/", params, 3)
	if err != nil {
		return nil, err
	}
	return normalizePagedResponse(raw, "aweme_list"), nil
}

// GetUserLike fetches a page of a user's liked videos.
func (c *DouyinAPIClient) GetUserLike(ctx context.Context, secUID string, maxCursor int64, count int) (*PagedResponse, error) {
	if count <= 0 {
		count = 20
	}
	params := c.defaultQuery()
	params.Set("sec_user_id", secUID)
	params.Set("max_cursor", fmt.Sprintf("%d", maxCursor))
	params.Set("count", fmt.Sprintf("%d", count))
	params.Set("locate_query", "false")

	raw, err := c.RequestJSON(ctx, "/aweme/v1/web/aweme/favorite/", params, 3)
	if err != nil {
		return nil, err
	}
	return normalizePagedResponse(raw, "aweme_list"), nil
}

// GetUserInfo fetches a user's profile.
func (c *DouyinAPIClient) GetUserInfo(ctx context.Context, secUID string) (map[string]any, error) {
	params := c.defaultQuery()
	params.Set("sec_user_id", secUID)

	data, err := c.RequestJSON(ctx, "/aweme/v1/web/user/profile/other/", params, 3)
	if err != nil {
		return nil, err
	}
	if user, ok := data["user"].(map[string]any); ok {
		return user, nil
	}
	return nil, nil
}

// GetMixDetail fetches details for a mix/collection.
func (c *DouyinAPIClient) GetMixDetail(ctx context.Context, mixID string) (map[string]any, error) {
	params := c.defaultQuery()
	params.Set("mix_id", mixID)

	data, err := c.RequestJSON(ctx, "/aweme/v1/web/mix/detail/", params, 3)
	if err != nil {
		return nil, err
	}
	if info, ok := data["mix_info"].(map[string]any); ok {
		return info, nil
	}
	if detail, ok := data["mix_detail"].(map[string]any); ok {
		return detail, nil
	}
	return data, nil
}

// GetMixAweme fetches videos in a mix/collection.
func (c *DouyinAPIClient) GetMixAweme(ctx context.Context, mixID string, cursor int64, count int) (*PagedResponse, error) {
	if count <= 0 {
		count = 20
	}
	params := c.defaultQuery()
	params.Set("mix_id", mixID)
	params.Set("cursor", fmt.Sprintf("%d", cursor))
	params.Set("count", fmt.Sprintf("%d", count))

	raw, err := c.RequestJSON(ctx, "/aweme/v1/web/mix/aweme/", params, 3)
	if err != nil {
		return nil, err
	}
	return normalizePagedResponse(raw, "aweme_list"), nil
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

// Close releases any resources (currently a no-op for net/http).
func (c *DouyinAPIClient) Close() {}

// SignURL signs a raw URL using X-Bogus.
func (c *DouyinAPIClient) SignURL(rawURL string) (string, string, string) {
	return c.signer.Build(rawURL)
}

// UserAgent returns the client's User-Agent string.
func (c *DouyinAPIClient) UserAgent() string {
	return c.headers["User-Agent"]
}

// Client returns the underlying HTTP client (for streaming).
func (c *DouyinAPIClient) Client() *http.Client {
	return c.client
}

// Cookies returns the current cookies.
func (c *DouyinAPIClient) Cookies() map[string]string {
	return c.cookies
}
