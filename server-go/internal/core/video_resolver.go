package core

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"github.com/xuziyue/douyin-downloader/internal/utils"
)

// VideoInfo holds the resolved video URL and metadata for streaming.
type VideoInfo struct {
	VideoURL    string
	VideoHdrs   map[string]string
	Filename    string
	AwemeID     string
	Title       string
}

var qualityTargetWidth = map[string]int{
	"1440p": 2560, "1080p": 1920, "720p": 1280,
	"540p": 960, "480p": 854, "360p": 640,
}

// BuildNoWatermarkURL selects the best no-watermark video URL from aweme data.
func BuildNoWatermarkURL(ctx context.Context, awemeData map[string]any, quality string, apiClient *DouyinAPIClient) (*VideoInfo, error) {
	if quality == "" {
		quality = "highest"
	}

	video, _ := awemeData["video"].(map[string]any)
	if video == nil {
		video = map[string]any{}
	}

	playAddr := pickPreferredPlayAddr(video, quality)
	if playAddr == nil {
		playAddr = map[string]any{}
	}

	urlList, _ := playAddr["url_list"].([]any)
	var urlCandidates []string
	for _, u := range urlList {
		if s, ok := u.(string); ok && s != "" {
			urlCandidates = append(urlCandidates, s)
		}
	}
	// Sort: no-watermark first
	sortCandidates(urlCandidates)

	downloadHeaders := func(ua ...string) map[string]string {
		h := map[string]string{
			"Referer":         "https://www.douyin.com/",
			"Accept":          "*/*",
			"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
		}
		if len(ua) > 0 && ua[0] != "" {
			h["User-Agent"] = ua[0]
		} else {
			h["User-Agent"] = apiClient.UserAgent()
		}
		return h
	}

	var fallbackCandidate *urlCandidate
	var watermarkedCandidate *urlCandidate

	for _, candidate := range urlCandidates {
		parsed, err := url.Parse(candidate)
		if err != nil {
			continue
		}
		headers := downloadHeaders()
		isWatermarked := isWatermarkedMediaURL(candidate)

		if strings.HasSuffix(parsed.Host, "douyin.com") {
			if !strings.Contains(candidate, "X-Bogus=") && !strings.Contains(candidate, "a_bogus=") {
				signedURL, _, ua := apiClient.signer.Build(candidate)
				headers = downloadHeaders(ua)
				if isWatermarked {
					if watermarkedCandidate == nil {
						watermarkedCandidate = &urlCandidate{signedURL, headers}
					}
					continue
				}
				return &VideoInfo{VideoURL: signedURL, VideoHdrs: headers}, nil
			}
			if isWatermarked {
				if watermarkedCandidate == nil {
					watermarkedCandidate = &urlCandidate{candidate, headers}
				}
				continue
			}
			return &VideoInfo{VideoURL: candidate, VideoHdrs: headers}, nil
		}

		if isWatermarked {
			if watermarkedCandidate == nil {
				watermarkedCandidate = &urlCandidate{candidate, headers}
			}
		} else {
			if fallbackCandidate == nil {
				fallbackCandidate = &urlCandidate{candidate, headers}
			}
		}
	}

	if fallbackCandidate != nil {
		return &VideoInfo{VideoURL: fallbackCandidate.url, VideoHdrs: fallbackCandidate.headers}, nil
	}

	// Fallback: use /aweme/v1/play/ endpoint
	uri := getString(playAddr, "uri")
	if uri == "" {
		uri = getString(video, "vid")
	}
	if uri == "" {
		if dlAddr, ok := video["download_addr"].(map[string]any); ok {
			uri = getString(dlAddr, "uri")
		}
	}

	if uri != "" {
		ratio := pickRatio(quality)
		params := url.Values{
			"video_id":   {uri},
			"ratio":      {ratio},
			"line":       {"0"},
			"is_play_url": {"1"},
			"watermark":  {"0"},
			"source":     {"PackSourceEnum_PUBLISH"},
		}
		signedURL, ua := apiClient.BuildSignedPath("/aweme/v1/play/", params)
		return &VideoInfo{VideoURL: signedURL, VideoHdrs: downloadHeaders(ua)}, nil
	}

	if watermarkedCandidate != nil {
		return &VideoInfo{VideoURL: watermarkedCandidate.url, VideoHdrs: watermarkedCandidate.headers}, nil
	}

	return nil, fmt.Errorf("no playable video URL found")
}

type urlCandidate struct {
	url     string
	headers map[string]string
}

func sortCandidates(urls []string) {
	for i := 0; i < len(urls); i++ {
		for j := i + 1; j < len(urls); j++ {
			if !strings.Contains(urls[i], "watermark=0") && strings.Contains(urls[j], "watermark=0") {
				urls[i], urls[j] = urls[j], urls[i]
			}
		}
	}
}

func isWatermarkedMediaURL(u string) bool {
	return !strings.Contains(u, "watermark=0")
}

func pickPreferredPlayAddr(video map[string]any, quality string) map[string]any {
	for _, key := range []string{"play_addr_h264", "play_addr_265", "play_addr_256", "play_addr"} {
		if addr, ok := video[key].(map[string]any); ok && addr != nil {
			return addr
		}
	}
	return nil
}

func pickRatio(quality string) string {
	normalised := strings.ToLower(strings.TrimSpace(quality))
	switch normalised {
	case "highest":
		return "1080p"
	case "lowest":
		return "540p"
	}
	if _, ok := qualityTargetWidth[normalised]; ok {
		return normalised
	}
	return "1080p"
}

func getString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// ResolveVideo takes a URL, resolves it to a video stream info.
func ResolveVideo(ctx context.Context, rawURL string, apiClient *DouyinAPIClient, quality string) (*VideoInfo, error) {
	// Handle short URL
	if utils.IsShortURL(rawURL) {
		resolved, err := apiClient.ResolveShortURL(ctx, utils.NormalizeShortURL(rawURL))
		if err != nil {
			return nil, fmt.Errorf("short URL resolve failed: %w", err)
		}
		if resolved == "" {
			return nil, fmt.Errorf("短链解析失败")
		}
		rawURL = resolved
	}

	parsed := ParseURL(rawURL)
	if parsed == nil {
		return nil, fmt.Errorf("无法识别的抖音链接")
	}
	if parsed.Type != "video" {
		return nil, fmt.Errorf("仅支持视频链接, 当前类型: %s", parsed.Type)
	}
	if parsed.AwemeID == "" {
		return nil, fmt.Errorf("未能从链接提取视频 ID")
	}

	awemeData, err := apiClient.GetVideoDetail(ctx, parsed.AwemeID)
	if err != nil {
		if _, ok := err.(*LoginRequiredError); ok {
			return nil, fmt.Errorf("抖音 Cookie 已过期, 请更新 config.yml 中的 cookies")
		}
		return nil, fmt.Errorf("获取视频详情失败: %w", err)
	}
	if awemeData == nil {
		return nil, fmt.Errorf("获取视频详情失败 (Cookie 可能过期或被风控)")
	}

	info, err := BuildNoWatermarkURL(ctx, awemeData, quality, apiClient)
	if err != nil {
		return nil, err
	}

	title := strings.TrimSpace(getString(awemeData, "desc"))
	if title == "" {
		title = parsed.AwemeID
	}
	info.Filename = utils.SanitizeFilename(title, 80) + ".mp4"
	info.AwemeID = parsed.AwemeID
	info.Title = title

	slog.Debug("Resolved video", "aweme_id", parsed.AwemeID, "title", title)
	return info, nil
}
