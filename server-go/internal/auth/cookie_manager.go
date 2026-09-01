package auth

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/xuziyue/douyin-downloader/internal/utils"
)

// CookieManager manages Douyin cookies, persisting them to a JSON file.
type CookieManager struct {
	cookieFile string
	cookies    map[string]string
}

// NewCookieManager creates a CookieManager with the given file path.
func NewCookieManager(cookieFile string) *CookieManager {
	if cookieFile == "" {
		cookieFile = ".cookies.json"
	}
	return &CookieManager{
		cookieFile: cookieFile,
		cookies:    map[string]string{},
	}
}

// SetCookies sets and persists the cookies.
func (cm *CookieManager) SetCookies(cookies map[string]string) {
	cm.cookies = utils.SanitizeCookies(cookies)
	cm.save()
}

// GetCookies returns the cookies, loading from disk if needed.
func (cm *CookieManager) GetCookies() map[string]string {
	if len(cm.cookies) == 0 {
		cm.load()
	}
	return cm.cookies
}

func (cm *CookieManager) save() {
	dir := filepath.Dir(cm.cookieFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		slog.Error("Failed to create cookie dir", "dir", dir, "error", err)
		return
	}
	data, err := json.MarshalIndent(cm.cookies, "", "  ")
	if err != nil {
		slog.Error("Failed to marshal cookies", "error", err)
		return
	}
	if err := os.WriteFile(cm.cookieFile, data, 0600); err != nil {
		slog.Error("Failed to save cookies", "path", cm.cookieFile, "error", err)
	}
}

func (cm *CookieManager) load() {
	data, err := os.ReadFile(cm.cookieFile)
	if err != nil {
		return
	}
	var raw map[string]string
	if err := json.Unmarshal(data, &raw); err != nil {
		slog.Error("Failed to parse cookie file", "path", cm.cookieFile, "error", err)
		return
	}
	cm.cookies = utils.SanitizeCookies(raw)
}
