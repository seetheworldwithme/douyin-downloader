package utils

import (
	"testing"
)

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"hello world", 80, "hello world"},
		{"file<name>", 80, "file_name"},
		{"a///b", 80, "a_b"},
		{"  spaces  ", 80, "spaces"},
		{"CON.txt", 80, "_CON.txt"},
		{"", 80, "untitled"},
		{"very_long_name_here_exceeding_max", 10, "very_long"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := SanitizeFilename(tt.input, tt.maxLen)
			if got != tt.expected {
				t.Errorf("SanitizeFilename(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.expected)
			}
		})
	}
}

func TestIsShortURL(t *testing.T) {
	tests := []struct {
		url      string
		expected bool
	}{
		{"https://v.douyin.com/abc123", true},
		{"v.douyin.com/abc", true},
		{"https://www.douyin.com/video/123", false},
		{"", false},
		{"https://v.iesdouyin.com/x", true},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			if got := IsShortURL(tt.url); got != tt.expected {
				t.Errorf("IsShortURL(%q) = %v, want %v", tt.url, got, tt.expected)
			}
		})
	}
}

func TestNormalizeShortURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"v.douyin.com/abc", "https://v.douyin.com/abc"},
		{"https://v.douyin.com/abc", "https://v.douyin.com/abc"},
		{"http://v.douyin.com/x", "http://v.douyin.com/x"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := NormalizeShortURL(tt.input); got != tt.expected {
				t.Errorf("NormalizeShortURL(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestParseURLType(t *testing.T) {
	tests := []struct {
		url      string
		expected string
	}{
		{"https://www.douyin.com/video/123456", "video"},
		{"https://www.douyin.com/user/MS4wLjABAAAA123", "user"},
		{"https://www.douyin.com/note/123456", "gallery"},
		{"https://www.douyin.com/collection/123456", "collection"},
		{"https://www.douyin.com/music/123456", "music"},
		{"https://live.douyin.com/123456", "live"},
		{"https://www.douyin.com/vsdetail/123456", "live_replay"},
		{"https://v.douyin.com/abc", "short"},
		{"https://example.com/foo", ""},
		{"https://www.douyin.com/discover?modal_id=12345", "video"},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			if got := ParseURLType(tt.url); got != tt.expected {
				t.Errorf("ParseURLType(%q) = %q, want %q", tt.url, got, tt.expected)
			}
		})
	}
}

func TestSanitizeCookies(t *testing.T) {
	input := map[string]string{
		"ttwid":    "abc123",
		"  msToken ": "xyz",
		"":         "empty",
		"bad@key":  "val",
	}
	result := SanitizeCookies(input)
	if result["ttwid"] != "abc123" {
		t.Errorf("expected ttwid=abc123, got %s", result["ttwid"])
	}
	if _, ok := result["  msToken "]; ok {
		t.Error("untrimmed key should be trimmed")
	}
	if result["msToken"] != "xyz" {
		t.Errorf("expected trimmed msToken=xyz, got %s", result["msToken"])
	}
	if _, ok := result[""]; ok {
		t.Error("empty key should be filtered")
	}
	if _, ok := result["bad@key"]; ok {
		t.Error("invalid key should be filtered")
	}
}

func TestParseCookieHeader(t *testing.T) {
	result := ParseCookieHeader("ttwid=abc; msToken=xyz; bad@key=val")
	if result["ttwid"] != "abc" {
		t.Errorf("expected ttwid=abc, got %s", result["ttwid"])
	}
	if result["msToken"] != "xyz" {
		t.Errorf("expected msToken=xyz, got %s", result["msToken"])
	}
	if _, ok := result["bad@key"]; ok {
		t.Error("invalid key should be filtered")
	}
}

func TestXBogusBuild(t *testing.T) {
	xb := NewXBogus("Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/139.0.0.0")
	testURL := "https://www.douyin.com/aweme/v1/web/aweme/detail/?device_platform=webapp&aid=6383&aweme_id=12345"

	signedURL, xbogus, ua := xb.Build(testURL)

	if signedURL == "" {
		t.Error("signedURL should not be empty")
	}
	if !contains(signedURL, "X-Bogus=") {
		t.Error("signedURL should contain X-Bogus parameter")
	}
	if xbogus == "" {
		t.Error("xbogus should not be empty")
	}
	if len(xbogus) != 28 {
		t.Errorf("xbogus length should be 28, got %d", len(xbogus))
	}
	if ua == "" {
		t.Error("user agent should not be empty")
	}
}

func TestXBogusDeterministic(t *testing.T) {
	// Same UA and timestamp-free input should produce consistent structure
	// Note: XBogus includes timestamp so output varies; we check format only
	xb1 := NewXBogus("TestUA")
	_, bogus1, _ := xb1.Build("test_url_1")
	_, bogus2, _ := xb1.Build("test_url_1")

	// Both should be valid length
	if len(bogus1) != 28 || len(bogus2) != 28 {
		t.Errorf("expected length 28, got %d and %d", len(bogus1), len(bogus2))
	}
}

func TestABogusGenerate(t *testing.T) {
	ab := NewABogus("", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/131.0.0.0")
	params := "device_platform=webapp&aid=6383&channel=channel_pc_web&aweme_id=12345"
	body := ""

	signedParams, abogus, ua, _ := ab.GenerateAbogus(params, body)

	if signedParams == "" {
		t.Error("signedParams should not be empty")
	}
	if !contains(signedParams, "a_bogus=") {
		t.Error("signedParams should contain a_bogus parameter")
	}
	if abogus == "" {
		t.Error("abogus should not be empty")
	}
	// A-Bogus typically produces ~160 chars
	if len(abogus) < 100 {
		t.Errorf("abogus seems too short: %d chars", len(abogus))
	}
	if ua == "" {
		t.Error("user agent should not be empty")
	}
}

func TestGenerateFingerprint(t *testing.T) {
	fp := GenerateFingerprint("Chrome")
	if fp == "" {
		t.Error("fingerprint should not be empty")
	}
	// Should contain pipe-separated values
	if !contains(fp, "|") {
		t.Error("fingerprint should contain pipe separators")
	}
	// Should contain Win32 platform
	if !contains(fp, "Win32") {
		t.Error("Chrome fingerprint should have Win32 platform")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
