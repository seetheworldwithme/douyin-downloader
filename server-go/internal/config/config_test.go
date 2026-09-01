package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Path != "./Downloaded/" {
		t.Errorf("expected path=./Downloaded/, got %s", cfg.Path)
	}
	if cfg.Thread != 5 {
		t.Errorf("expected thread=5, got %d", cfg.Thread)
	}
	if cfg.VideoQuality != "highest" {
		t.Errorf("expected video_quality=highest, got %s", cfg.VideoQuality)
	}
	if cfg.Database != true {
		t.Error("expected database=true")
	}
	if cfg.Auth.Username != "xuziyue" {
		t.Errorf("expected auth.username=xuziyue, got %s", cfg.Auth.Username)
	}
}

func TestLoadConfigFromFile(t *testing.T) {
	// Create a temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")
	configContent := `
path: ./Downloads/
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

	if cfg.Config.Path != "./Downloads/" {
		t.Errorf("expected path=./Downloads/, got %s", cfg.Config.Path)
	}
	if cfg.Config.Thread != 10 {
		t.Errorf("expected thread=10, got %d", cfg.Config.Thread)
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
	os.Setenv("DOUYIN_THREAD", "20")
	os.Setenv("DOUYIN_PROXY", "http://proxy:8080")
	defer os.Unsetenv("DOUYIN_THREAD")
	defer os.Unsetenv("DOUYIN_PROXY")

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")
	os.WriteFile(configPath, []byte("path: ./Downloads/\n"), 0644)

	cfg, err := NewConfigLoader(configPath)
	if err != nil {
		t.Fatalf("failed: %v", err)
	}

	if cfg.Config.Thread != 20 {
		t.Errorf("expected thread=20 from env, got %d", cfg.Config.Thread)
	}
	if cfg.Config.Proxy != "http://proxy:8080" {
		t.Errorf("expected proxy from env, got %s", cfg.Config.Proxy)
	}
}
