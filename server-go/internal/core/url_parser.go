package core

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/xuziyue/douyin-downloader/internal/utils"
)

// parseURLType delegates to utils.ParseURLType to avoid circular imports.
func parseURLType(rawURL string) string {
	return utils.ParseURLType(rawURL)
}

// ParsedURL holds the result of URL parsing.
type ParsedURL struct {
	OriginalURL string
	Type        string // video, user, gallery, collection, music, live, live_replay
	AwemeID     string
	SecUID      string
	MixID       string
	NoteID      string
	RoomID      string
	EpisodeID   string
}

var (
	reVideoID    = regexp.MustCompile(`/video/(\d+)`)
	reModalID    = regexp.MustCompile(`modal_id=(\d+)`)
	reUserID     = regexp.MustCompile(`/user/([A-Za-z0-9._-]+)`)
	reMixID      = regexp.MustCompile(`/(?:collection|mix)/(\d+)`)
	reNoteID     = regexp.MustCompile(`/(?:note|gallery|slides)/(\d+)`)
	reLiveID     = regexp.MustCompile(`/live/(\d+)`)
	reLiveDomain = regexp.MustCompile(`live\.douyin\.com/(\d+)`)
)

// ParseURL parses a Douyin URL and extracts its type and ID fields.
func ParseURL(rawURL string) *ParsedURL {
	urlType := parseURLType(rawURL)
	if urlType == "" {
		return nil
	}

	result := &ParsedURL{
		OriginalURL: rawURL,
		Type:        urlType,
	}

	switch urlType {
	case "video":
		if m := reVideoID.FindStringSubmatch(rawURL); len(m) > 1 {
			result.AwemeID = m[1]
		}
		if result.AwemeID == "" {
			if m := reModalID.FindStringSubmatch(rawURL); len(m) > 1 {
				result.AwemeID = m[1]
			}
		}
	case "user":
		if m := reUserID.FindStringSubmatch(rawURL); len(m) > 1 {
			result.SecUID = m[1]
		}
	case "collection":
		if m := reMixID.FindStringSubmatch(rawURL); len(m) > 1 {
			result.MixID = m[1]
		}
	case "music":
		// ponytail: ID 不再解析 —— 服务器只支持视频/图集,music 链接仅分类后拒绝
	case "gallery":
		if m := reNoteID.FindStringSubmatch(rawURL); len(m) > 1 {
			result.NoteID = m[1]
			result.AwemeID = m[1]
		}
	case "live":
		if m := reLiveID.FindStringSubmatch(rawURL); len(m) > 1 {
			result.RoomID = m[1]
		}
		if result.RoomID == "" {
			if m := reLiveDomain.FindStringSubmatch(rawURL); len(m) > 1 {
				result.RoomID = m[1]
			}
		}
	case "live_replay":
		parsed, err := url.Parse(rawURL)
		if err == nil {
			path := parsed.Path
			if strings.HasPrefix(path, "/vsdetail/") {
				if m := regexp.MustCompile(`^/vsdetail/(\d+)(?:/|$)`).FindStringSubmatch(path); len(m) > 1 {
					result.EpisodeID = m[1]
				}
			} else if strings.HasPrefix(path, "/douyin/webcast/reflow/episode/") {
				if m := regexp.MustCompile(`^/douyin/webcast/reflow/episode/(\d+)(?:/|$)`).FindStringSubmatch(path); len(m) > 1 {
					result.EpisodeID = m[1]
				}
			}
		}
	}

	return result
}
