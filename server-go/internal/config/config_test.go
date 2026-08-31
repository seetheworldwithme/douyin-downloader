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

func TestBrowserFallbackMerge(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("absent block keeps defaults", func(t *testing.T) {
		configPath := filepath.Join(tmpDir, "no_fallback.yml")
		os.WriteFile(configPath, []byte("path: ./Downloads/\n"), 0644)
		cfg, err := NewConfigLoader(configPath)
		if err != nil {
			t.Fatalf("failed: %v", err)
		}
		fb := cfg.Config.BrowserFallback
		if fb == nil {
			t.Fatal("expected default browser fallback config")
		}
		if !fb.Enabled || fb.Headless || fb.MaxScrolls != 240 {
			t.Errorf("expected defaults, got %+v", fb)
		}
	})

	t.Run("present block overrides", func(t *testing.T) {
		configPath := filepath.Join(tmpDir, "fallback.yml")
		os.WriteFile(configPath, []byte("browser_fallback:\n  enabled: false\n  headless: true\n  max_scrolls: 60\n"), 0644)
		cfg, err := NewConfigLoader(configPath)
		if err != nil {
			t.Fatalf("failed: %v", err)
		}
		fb := cfg.Config.BrowserFallback
		if fb == nil {
			t.Fatal("expected browser fallback config")
		}
		if fb.Enabled {
			t.Error("expected enabled=false from file")
		}
		if !fb.Headless {
			t.Error("expected headless=true from file")
		}
		if fb.MaxScrolls != 60 {
			t.Errorf("expected max_scrolls=60, got %d", fb.MaxScrolls)
		}
		// unset numeric fields keep defaults
		if fb.IdleRounds != 8 || fb.WaitTimeoutSeconds != 480 {
			t.Errorf("expected defaults for unset fields, got %+v", fb)
		}
	})
}

func TestGetLinks(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Link = []string{"https://douyin.com/user/123", "https://douyin.com/video/456"}
	cl := &ConfigLoader{Config: cfg}

	links := cl.GetLinks()
	if len(links) != 2 {
		t.Fatalf("expected 2 links, got %d", len(links))
	}
}

func TestValidate(t *testing.T) {
	cfg := DefaultConfig()
	cl := &ConfigLoader{Config: cfg}

	// No links = invalid
	if cl.Validate() {
		t.Error("expected invalid (no links)")
	}

	// With links and path = valid
	cl.Config.Link = []string{"https://douyin.com/user/test"}
	if !cl.Validate() {
		t.Error("expected valid")
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
