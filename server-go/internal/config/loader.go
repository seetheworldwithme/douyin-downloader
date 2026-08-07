package config

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
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
			fileConfig := DefaultConfig()
			if err := yaml.Unmarshal(data, fileConfig); err != nil {
				return nil, fmt.Errorf("parse YAML %s: %w", configPath, err)
			}
			cl.mergeConfig(fileConfig)
		}
	}

	cl.applyEnvOverrides()
	return cl, nil
}

func (cl *ConfigLoader) mergeConfig(fileConfig *Config) {
	// Simple field-by-field merge: only override if the file provides a non-zero value.
	// For YAML, missing keys remain as defaults, so we can just use the parsed config
	// if the file was valid. But to handle partial overrides correctly, we merge
	// individual non-zero fields.

	cfg := cl.Config

	if fileConfig.Path != "" {
		cfg.Path = fileConfig.Path
	}
	if fileConfig.Music != false || fileConfig.Path != "" {
		cfg.Music = fileConfig.Music
		cfg.Cover = fileConfig.Cover
		cfg.Avatar = fileConfig.Avatar
		cfg.JSON = fileConfig.JSON
	}
	if fileConfig.FilenameTemplate != "" {
		cfg.FilenameTemplate = fileConfig.FilenameTemplate
	}
	if fileConfig.FolderTemplate != "" {
		cfg.FolderTemplate = fileConfig.FolderTemplate
	}
	if fileConfig.AuthorDir != "" {
		cfg.AuthorDir = fileConfig.AuthorDir
	}
	cfg.GroupByMode = fileConfig.GroupByMode
	cfg.DownloadPinned = fileConfig.DownloadPinned

	if len(fileConfig.Mode) > 0 {
		cfg.Mode = fileConfig.Mode
	}
	if len(fileConfig.Number) > 0 {
		for k, v := range fileConfig.Number {
			cfg.Number[k] = v
		}
	}
	if len(fileConfig.Increase) > 0 {
		for k, v := range fileConfig.Increase {
			cfg.Increase[k] = v
		}
	}
	if fileConfig.Thread > 0 {
		cfg.Thread = fileConfig.Thread
	}
	if fileConfig.RetryTimes > 0 {
		cfg.RetryTimes = fileConfig.RetryTimes
	}
	if fileConfig.RateLimit > 0 {
		cfg.RateLimit = fileConfig.RateLimit
	}
	if fileConfig.Proxy != "" {
		cfg.Proxy = fileConfig.Proxy
	}
	if fileConfig.VideoQuality != "" {
		cfg.VideoQuality = fileConfig.VideoQuality
	}
	cfg.Database = fileConfig.Database
	if fileConfig.DatabasePath != "" {
		cfg.DatabasePath = fileConfig.DatabasePath
	}

	// Complex fields: override if the file has non-default values
	if fileConfig.Transcript.Model != "" {
		cfg.Transcript = fileConfig.Transcript
	}
	if fileConfig.Auth.Username != "" || len(fileConfig.Auth.Users) > 0 {
		cfg.Auth = fileConfig.Auth
	}
	if fileConfig.Server.MaxJobs != 0 {
		cfg.Server = fileConfig.Server
	}

	cfg.Cookies = fileConfig.Cookies
	cfg.Cookie = fileConfig.Cookie
	cfg.AutoCookie = fileConfig.AutoCookie

	if len(fileConfig.Link) > 0 {
		cfg.Link = fileConfig.Link
	}

	cfg.StartTime = fileConfig.StartTime
	cfg.EndTime = fileConfig.EndTime
}

func (cl *ConfigLoader) applyEnvOverrides() {
	if v := os.Getenv("DOUYIN_COOKIE"); v != "" {
		cl.Config.Cookie = v
	}
	if v := os.Getenv("DOUYIN_PATH"); v != "" {
		cl.Config.Path = v
	}
	if v := os.Getenv("DOUYIN_THREAD"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cl.Config.Thread = n
		}
	}
	if v := os.Getenv("DOUYIN_PROXY"); v != "" {
		cl.Config.Proxy = v
	}
}

// Get returns a value from the config for a given key.
func (cl *ConfigLoader) Get(key string) any {
	switch key {
	case "path":
		return cl.Config.Path
	case "thread":
		return cl.Config.Thread
	case "proxy":
		return cl.Config.Proxy
	case "rate_limit":
		return cl.Config.RateLimit
	case "retry_times":
		return cl.Config.RetryTimes
	case "music":
		return cl.Config.Music
	case "cover":
		return cl.Config.Cover
	case "avatar":
		return cl.Config.Avatar
	case "json":
		return cl.Config.JSON
	case "database":
		return cl.Config.Database
	case "database_path":
		return cl.Config.DatabasePath
	case "auth":
		return cl.Config.Auth
	case "server":
		return cl.Config.Server
	case "cookies":
		return cl.Config.Cookies
	case "cookie":
		return cl.Config.Cookie
	case "video_quality":
		return cl.Config.VideoQuality
	default:
		return nil
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

// GetLinks returns the configured links list.
func (cl *ConfigLoader) GetLinks() []string {
	return cl.Config.Link
}

// Validate checks that essential config fields are set.
func (cl *ConfigLoader) Validate() bool {
	if len(cl.Config.Link) == 0 {
		return false
	}
	if cl.Config.Path == "" {
		return false
	}
	return true
}

// Save persists UI-editable keys back to the config file.
func (cl *ConfigLoader) Save() bool {
	if cl.configPath == "" {
		return false
	}
	data, err := yaml.Marshal(cl.Config)
	if err != nil {
		slog.Warn("Failed to marshal config", "error", err)
		return false
	}
	if err := os.WriteFile(cl.configPath, data, 0600); err != nil {
		slog.Warn("Failed to write config", "path", cl.configPath, "error", err)
		return false
	}
	return true
}

// ConfigDir returns the directory containing the config file.
func (cl *ConfigLoader) ConfigDir() string {
	if cl.configPath == "" {
		return "."
	}
	return filepath.Dir(cl.configPath)
}
