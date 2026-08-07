package auth

import (
	"encoding/json"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

const f2ConfURL = "https://raw.githubusercontent.com/Johnserf-Seed/f2/main/f2/conf/conf.yaml"

var (
	cachedConf    map[string]any
	cachedAt      time.Time
	cacheTTL      = time.Hour
	confCacheMu   sync.Mutex
)

// MsTokenManager generates and refreshes the msToken required by Douyin API.
type MsTokenManager struct {
	userAgent     string
	confURL       string
	timeoutSecs   int
}

// NewMsTokenManager creates a new MsTokenManager.
func NewMsTokenManager(userAgent string) *MsTokenManager {
	return &MsTokenManager{
		userAgent:   userAgent,
		confURL:     f2ConfURL,
		timeoutSecs: 15,
	}
}

// IsValidMSToken checks if a token has valid length (164 or 184).
func IsValidMSToken(token string) bool {
	token = strings.TrimSpace(token)
	l := len(token)
	return l == 164 || l == 184
}

// GenFalseMSToken generates a random fallback msToken.
func GenFalseMSToken() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 182)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b) + "=="
}

// EnsureMSToken returns a valid msToken, generating one if needed.
func (m *MsTokenManager) EnsureMSToken(cookies map[string]string) string {
	if current := strings.TrimSpace(cookies["msToken"]); current != "" {
		return current
	}
	if real := m.GenRealMSToken(); real != "" {
		return real
	}
	return GenFalseMSToken()
}

// GenRealMSToken attempts to generate a real msToken via the mssdk endpoint.
func (m *MsTokenManager) GenRealMSToken() string {
	conf := m.loadF2MSTokenConf()
	if conf == nil {
		return ""
	}

	payload := map[string]any{
		"magic":         conf["magic"],
		"version":       conf["version"],
		"dataType":      conf["dataType"],
		"strData":       conf["strData"],
		"ulr":           conf["ulr"],
		"tspFromClient": time.Now().UnixMilli(),
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", conf["url"].(string), strings.NewReader(string(body)))
	if err != nil {
		return ""
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("User-Agent", m.userAgent)

	client := &http.Client{Timeout: time.Duration(m.timeoutSecs) * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		slog.Warn("Failed to generate real msToken", "error", err)
		return ""
	}
	defer resp.Body.Close()

	for _, cookie := range resp.Cookies() {
		if cookie.Name == "msToken" && cookie.Value != "" {
			token := strings.TrimSpace(cookie.Value)
			if IsValidMSToken(token) {
				return token
			}
		}
	}
	return ""
}

func (m *MsTokenManager) loadF2MSTokenConf() map[string]any {
	confCacheMu.Lock()
	defer confCacheMu.Unlock()

	if cachedConf != nil && time.Since(cachedAt) < cacheTTL {
		return cachedConf
	}

	client := &http.Client{Timeout: time.Duration(m.timeoutSecs) * time.Second}
	resp, err := client.Get(m.confURL)
	if err != nil {
		slog.Warn("Failed to load F2 msToken config", "error", err)
		return nil
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Warn("Failed to read F2 config response", "error", err)
		return nil
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		slog.Warn("Failed to parse F2 config YAML", "error", err)
		return nil
	}

	// Navigate: f2.douyin.msToken
	f2, ok := raw["f2"].(map[string]any)
	if !ok {
		return nil
	}
	douyin, ok := f2["douyin"].(map[string]any)
	if !ok {
		return nil
	}
	msConf, ok := douyin["msToken"].(map[string]any)
	if !ok {
		return nil
	}

	required := []string{"url", "magic", "version", "dataType", "ulr", "strData"}
	for _, key := range required {
		if _, ok := msConf[key]; !ok {
			slog.Warn("F2 msToken config incomplete", "missing", key)
			return nil
		}
	}

	cachedConf = msConf
	cachedAt = time.Now()
	return msConf
}
