package core

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// ImageItem is one image of a gallery (图集) post: candidate URLs plus the
// dimensions reported by the API (used to size the composed video).
type ImageItem struct {
	URLs   []string
	Width  int
	Height int
}

// MediaInfo is the unified resolve result for both video and gallery posts.
type MediaInfo struct {
	// Type is "video" or "images".
	Type string
	// Video is set when Type == "video".
	Video *VideoInfo
	// Images is set when Type == "images".
	Images []ImageItem
	// MusicURLs are candidate URLs for the post's background music
	// (attached to composed videos when available).
	MusicURLs []string
	// MusicDurationMS is the music duration in milliseconds (0 if unknown).
	MusicDurationMS int64

	AwemeID  string
	Title    string
	// BaseName is the sanitized filename without extension.
	BaseName string
}

const (
	maxImageBytes = 50 << 20
	maxMusicBytes = 30 << 20
)

// extractImages pulls the gallery image list out of an aweme detail payload.
// Returns nil for regular video posts.
func extractImages(awemeData map[string]any) []ImageItem {
	rawList, ok := awemeData["images"].([]any)
	if !ok {
		return nil
	}
	var items []ImageItem
	for _, entry := range rawList {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		item := ImageItem{
			Width:  int(toInt64(m["width"])),
			Height: int(toInt64(m["height"])),
		}
		seen := map[string]bool{}
		addURLs := func(list any) {
			arr, ok := list.([]any)
			if !ok {
				return
			}
			for _, u := range arr {
				if s, ok := u.(string); ok && s != "" && !seen[s] {
					seen[s] = true
					item.URLs = append(item.URLs, s)
				}
			}
		}
		addURLs(m["url_list"])
		addURLs(m["download_url_list"])
		if len(item.URLs) > 0 {
			items = append(items, item)
		}
	}
	return items
}

// extractMusic pulls background music candidates and duration out of an aweme
// detail payload.
func extractMusic(awemeData map[string]any) (urls []string, durationMS int64) {
	music, ok := awemeData["music"].(map[string]any)
	if !ok {
		return nil, 0
	}
	durationMS = toInt64(music["duration"])
	playURL, ok := music["play_url"].(map[string]any)
	if !ok {
		return nil, durationMS
	}
	urlList, ok := playURL["url_list"].([]any)
	if !ok {
		return nil, durationMS
	}
	seen := map[string]bool{}
	for _, u := range urlList {
		if s, ok := u.(string); ok && s != "" && !seen[s] {
			seen[s] = true
			urls = append(urls, s)
		}
	}
	return urls, durationMS
}

func mediaDownloadHeaders(ua string) map[string]string {
	return map[string]string{
		"Referer":         "https://www.douyin.com/",
		"Accept":          "*/*",
		"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
		"User-Agent":      ua,
	}
}

// DownloadImage fetches one gallery image, trying each candidate URL with
// retries until one returns a usable body.
func DownloadImage(ctx context.Context, client *DouyinAPIClient, item ImageItem) (data []byte, contentType string, err error) {
	return downloadFirstAvailable(ctx, client, item.URLs, maxImageBytes, 2)
}

// DownloadMusic fetches the post's background audio (best effort: callers may
// compose a silent video when this fails).
func DownloadMusic(ctx context.Context, client *DouyinAPIClient, urls []string) (data []byte, contentType string, err error) {
	if len(urls) == 0 {
		return nil, "", fmt.Errorf("没有可用的音乐地址")
	}
	return downloadFirstAvailable(ctx, client, urls, maxMusicBytes, 2)
}

func downloadFirstAvailable(ctx context.Context, client *DouyinAPIClient, urls []string, maxBytes int64, attempts int) ([]byte, string, error) {
	var lastErr error
	for _, u := range urls {
		data, ctype, err := downloadWithRetry(ctx, client, u, maxBytes, attempts)
		if err == nil {
			return data, ctype, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("没有可用的下载地址")
	}
	return nil, "", lastErr
}

func downloadWithRetry(ctx context.Context, client *DouyinAPIClient, rawURL string, maxBytes int64, attempts int) ([]byte, string, error) {
	if attempts < 1 {
		attempts = 1
	}
	headers := mediaDownloadHeaders(client.UserAgent())
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(time.Duration(attempt) * 500 * time.Millisecond):
			case <-ctx.Done():
				return nil, "", ctx.Err()
			}
		}

		req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
		if err != nil {
			return nil, "", err
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}

		resp, err := client.Client().Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode != 200 {
			resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			continue
		}

		data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		if int64(len(data)) > maxBytes {
			lastErr = fmt.Errorf("文件超过大小上限 %d MB", maxBytes>>20)
			continue
		}
		if len(data) == 0 {
			lastErr = fmt.Errorf("空响应")
			continue
		}
		slog.Debug("media downloaded", "url_host", hostOf(rawURL), "bytes", len(data))
		return data, normalizeContentType(resp.Header.Get("Content-Type")), nil
	}
	return nil, "", lastErr
}

func normalizeContentType(ct string) string {
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	return strings.ToLower(strings.TrimSpace(ct))
}

func hostOf(rawURL string) string {
	if i := strings.Index(rawURL, "://"); i >= 0 {
		rest := rawURL[i+3:]
		if j := strings.IndexAny(rest, "/"); j >= 0 {
			return rest[:j]
		}
		return rest
	}
	return rawURL
}

// ImageExt picks a file extension for an image body, preferring magic bytes
// over the (often generic) Content-Type reported by the CDN.
func ImageExt(data []byte, contentType string) string {
	if len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 {
		return ".jpg"
	}
	if len(data) >= 8 && string(data[1:4]) == "PNG" {
		return ".png"
	}
	if len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return ".webp"
	}
	switch contentType {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "image/heic", "image/heif":
		return ".heic"
	}
	return ".jpg"
}

// ImageContentType maps an ImageExt result back to a MIME type.
func ImageContentType(ext string) string {
	switch ext {
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	case ".heic":
		return "image/heic"
	default:
		return "image/jpeg"
	}
}

// AudioExt picks a file extension for a music body so that ffmpeg can probe
// the container. Extension is only a hint; ffmpeg sniffs the content anyway.
func AudioExt(data []byte) string {
	if len(data) >= 3 && (string(data[0:3]) == "ID3" || data[0] == 0xFF) {
		return ".mp3"
	}
	if len(data) >= 12 && string(data[4:8]) == "ftyp" {
		return ".m4a"
	}
	if len(data) >= 4 && string(data[0:4]) == "OggS" {
		return ".ogg"
	}
	return ".mp3"
}
