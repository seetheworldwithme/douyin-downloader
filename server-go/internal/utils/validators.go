package utils

import (
	"net/url"
	"regexp"
	"strings"
)

var windowsReservedStems = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true,
	"COM5": true, "COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true,
	"LPT5": true, "LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
}

var (
	illegalFilenameChars = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f#]`)
	multiUnderscore      = regexp.MustCompile(`_+`)
	multiSpace           = regexp.MustCompile(` +`)
)

func hostMatches(host, base string) bool {
	return host == base || strings.HasSuffix(host, "."+base)
}

func isDouyinWebHost(host string) bool {
	return hostMatches(host, "douyin.com") || hostMatches(host, "iesdouyin.com")
}

func isLiveReplayPath(host, path string) bool {
	if isDouyinWebHost(host) {
		return strings.HasPrefix(path, "/vsdetail/")
	}
	return host == "webcast.amemv.com" && strings.HasPrefix(path, "/douyin/webcast/reflow/episode/")
}

// ValidateURL checks if a URL string has both scheme and host.
func ValidateURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return u.Scheme != "" && u.Host != ""
}

// SanitizeFilename removes illegal characters and normalizes the filename.
func SanitizeFilename(filename string, maxLength int) string {
	filename = strings.ReplaceAll(filename, "\n", " ")
	filename = strings.ReplaceAll(filename, "\r", " ")
	filename = illegalFilenameChars.ReplaceAllString(filename, "_")
	filename = multiUnderscore.ReplaceAllString(filename, "_")
	filename = multiSpace.ReplaceAllString(filename, " ")
	filename = strings.Trim(filename, "._- ")

	if maxLength > 0 && len([]rune(filename)) > maxLength {
		runes := []rune(filename)
		filename = string(runes[:maxLength])
		filename = strings.TrimRight(filename, "._- ")
	}

	// Check Windows reserved stems
	dotIdx := strings.IndexByte(filename, '.')
	stem := filename
	if dotIdx >= 0 {
		stem = filename[:dotIdx]
	}
	if windowsReservedStems[strings.ToUpper(stem)] {
		if len(filename) < maxLength {
			filename = "_" + filename
		} else {
			filename = "_" + filename[:maxLength-1]
		}
	}

	if filename == "" {
		return "untitled"
	}
	return filename
}

var shortURLHosts = []string{"v.douyin.com", "v.iesdouyin.com", "iesdouyin.com"}

// IsShortURL checks if the URL is a Douyin short link that needs resolving.
func IsShortURL(rawURL string) bool {
	if rawURL == "" {
		return false
	}
	candidate := strings.TrimSpace(rawURL)
	lowered := strings.ToLower(candidate)
	for _, scheme := range []string{"https://", "http://"} {
		if strings.HasPrefix(lowered, scheme) {
			lowered = lowered[len(scheme):]
			break
		}
	}
	for _, host := range shortURLHosts {
		if strings.HasPrefix(lowered, host+"/") || lowered == host {
			return true
		}
	}
	return false
}

// NormalizeShortURL ensures a short URL has an https:// prefix.
func NormalizeShortURL(rawURL string) string {
	stripped := strings.TrimSpace(rawURL)
	if stripped == "" {
		return ""
	}
	lower := strings.ToLower(stripped)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return stripped
	}
	return "https://" + stripped
}

// ParseURLType determines the type of Douyin URL.
// Returns: "video", "user", "gallery", "collection", "music", "live", "live_replay", "short", or "".
func ParseURLType(rawURL string) string {
	if IsShortURL(rawURL) {
		return "short"
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	host := strings.ToLower(parsed.Hostname())
	path := parsed.Path

	if isLiveReplayPath(host, path) {
		return "live_replay"
	}

	if !isDouyinWebHost(host) {
		return ""
	}

	// modal_id param means single video view on any page
	qs := parsed.Query()
	if modalID := qs.Get("modal_id"); strings.TrimSpace(modalID) != "" {
		return "video"
	}

	if host == "live.douyin.com" {
		return "live"
	}
	if strings.Contains(path, "/video/") {
		return "video"
	}
	if strings.Contains(path, "/user/") {
		return "user"
	}
	if strings.Contains(path, "/note/") || strings.Contains(path, "/gallery/") || strings.Contains(path, "/slides/") {
		return "gallery"
	}
	if strings.Contains(path, "/collection/") || strings.Contains(path, "/mix/") {
		return "collection"
	}
	if strings.Contains(path, "/music/") {
		return "music"
	}
	if strings.Contains(path, "/live/") {
		return "live"
	}
	return ""
}
