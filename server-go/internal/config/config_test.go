package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.VideoQuality != "highest" {
		t.Errorf("expected video_quality=highest, got %s", cfg.VideoQuality)
	}
	if cfg.Auth.Username != "xuziyue" {
		t.Errorf("expected auth.username=xuziyue, got %s", cfg.Auth.Username)
	}
}

func TestLoadConfigFromFile(t *testing.T) {
	// Create a temp config file(含已删除的旧键,验证被静默忽略)
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")
	configContent := `
thread: 10
proxy: "http://127.0.0.1:7890"
video_quality: "720p"
link:
  - https://www.douyin.com/user/test123
auth:
  username: testuser
  password: testpass
`
	os.WriteFile(configPath, []byte(configContent), 0644)

	cfg, err := NewConfigLoader(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Config.Proxy != "http://127.0.0.1:7890" {
		t.Errorf("expected proxy, got %s", cfg.Config.Proxy)
	}
	if cfg.Config.VideoQuality != "720p" {
		t.Errorf("expected video_quality=720p, got %s", cfg.Config.VideoQuality)
	}
	if cfg.Config.Auth.Username != "testuser" {
		t.Errorf("expected auth.username=testuser, got %s", cfg.Config.Auth.Username)
	}
}

func TestEnvOverrides(t *testing.T) {
	os.Setenv("DOUYIN_PROXY", "http://proxy:8080")
	defer os.Unsetenv("DOUYIN_PROXY")

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")
	os.WriteFile(configPath, []byte("video_quality: \"480p\"\n"), 0644)

	cfg, err := NewConfigLoader(configPath)
	if err != nil {
		t.Fatalf("failed: %v", err)
	}

	if cfg.Config.Proxy != "http://proxy:8080" {
		t.Errorf("expected proxy from env, got %s", cfg.Config.Proxy)
	}
	if cfg.Config.VideoQuality != "480p" {
		t.Errorf("expected video_quality from file to survive, got %s", cfg.Config.VideoQuality)
	}
}
