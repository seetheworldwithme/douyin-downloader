package config

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/xuziyue/douyin-downloader/internal/utils"
)

// ConfigLoader handles loading and merging configuration from defaults,
// YAML files, and environment variables.
type ConfigLoader struct {
	configPath string
	Config     *Config
}

// NewConfigLoader creates a loader, reading from the given path if it exists.
// YAML 缺失键保留默认值,Unmarshal 本身即完成合并。
func NewConfigLoader(configPath string) (*ConfigLoader, error) {
	cl := &ConfigLoader{
		configPath: configPath,
		Config:     DefaultConfig(),
	}

	if configPath != "" {
		if _, err := os.Stat(configPath); err == nil {
			data, err := os.ReadFile(configPath)
			if err != nil {
				return nil, fmt.Errorf("read config %s: %w", configPath, err)
			}
			if err := yaml.Unmarshal(data, cl.Config); err != nil {
				return nil, fmt.Errorf("parse YAML %s: %w", configPath, err)
			}
		}
	}

	cl.applyEnvOverrides()
	return cl, nil
}

func (cl *ConfigLoader) applyEnvOverrides() {
	if v := os.Getenv("DOUYIN_COOKIE"); v != "" {
		cl.Config.Cookie = v
	}
	if v := os.Getenv("DOUYIN_PROXY"); v != "" {
		cl.Config.Proxy = v
	}
	if v := os.Getenv("DOUYIN_FFMPEG_PATH"); v != "" {
		cl.Config.FFmpegPath = v
	}
}

// GetCookies resolves cookies from config (string, map, or auto).
func (cl *ConfigLoader) GetCookies() map[string]string {
	// Try cookies map first
	if cl.Config.Cookies != nil {
		switch v := cl.Config.Cookies.(type) {
		case map[string]string:
			return utils.SanitizeCookies(v)
		case map[string]any:
			result := make(map[string]string)
			for k, val := range v {
				result[k] = fmt.Sprintf("%v", val)
			}
			return utils.SanitizeCookies(result)
		case string:
			if strings.TrimSpace(strings.ToLower(v)) == "auto" {
				return cl.loadAutoCookies()
			}
			return utils.SanitizeCookies(utils.ParseCookieHeader(v))
		}
	}

	// Try cookie string
	if cl.Config.Cookie != "" {
		if strings.TrimSpace(strings.ToLower(cl.Config.Cookie)) == "auto" {
			return cl.loadAutoCookies()
		}
		return utils.SanitizeCookies(utils.ParseCookieHeader(cl.Config.Cookie))
	}

	// Auto cookie
	if cl.autoCookieEnabled() {
		return cl.loadAutoCookies()
	}

	return map[string]string{}
}

func (cl *ConfigLoader) autoCookieEnabled() bool {
	switch v := cl.Config.AutoCookie.(type) {
	case bool:
		return v
	case string:
		lv := strings.ToLower(strings.TrimSpace(v))
		return lv == "1" || lv == "true" || lv == "yes" || lv == "on"
	}
	return false
}

func (cl *ConfigLoader) loadAutoCookies() map[string]string {
	for _, p := range cl.candidateAutoCookiePaths() {
		cookies := cl.loadCookieFile(p)
		if cookies != nil {
			if len(cookies) > 0 {
				slog.Info("Loaded auto cookies", "path", p)
			}
			return cookies
		}
	}
	return map[string]string{}
}

func (cl *ConfigLoader) candidateAutoCookiePaths() []string {
	var configDir string
	if cl.configPath != "" {
		configDir, _ = filepath.Abs(filepath.Dir(cl.configPath))
	} else {
		configDir, _ = filepath.Abs(".")
	}

	searchRoots := []string{configDir, filepath.Dir(configDir)}
	if cwd, err := filepath.Abs("."); err == nil {
		searchRoots = append(searchRoots, cwd)
	}

	var candidates []string
	seen := map[string]bool{}
	for _, root := range searchRoots {
		for _, rel := range []string{"config/cookies.json", ".cookies.json"} {
			p := filepath.Join(root, rel)
			abs, _ := filepath.Abs(p)
			if !seen[abs] {
				seen[abs] = true
				candidates = append(candidates, abs)
			}
		}
	}
	return candidates
}

func (cl *ConfigLoader) loadCookieFile(path string) map[string]string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		slog.Warn("Failed to parse cookie file", "path", path, "error", err)
		return map[string]string{}
	}
	result := make(map[string]string)
	for k, v := range raw {
		result[k] = fmt.Sprintf("%v", v)
	}
	return utils.SanitizeCookies(result)
}

// ConfigDir returns the directory containing the config file.
func (cl *ConfigLoader) ConfigDir() string {
	if cl.configPath == "" {
		return "."
	}
	return filepath.Dir(cl.configPath)
}
