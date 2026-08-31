package server

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	browserfallback "github.com/xuziyue/douyin-downloader/internal/browser"
	"github.com/xuziyue/douyin-downloader/internal/config"
	"github.com/xuziyue/douyin-downloader/internal/storage"
)

func (b *BatchService) browserHelperPath() (string, error) {
	configDir := b.deps.Config.ConfigDir()
	candidates := []string{
		filepath.Join(configDir, "tools", "browser-fallback.mjs"),
		filepath.Join(filepath.Dir(configDir), "tools", "browser-fallback.mjs"),
		filepath.Join("tools", "browser-fallback.mjs"),
	}
	// The server may be launched from a subdirectory (e.g. server-go/cmd/server)
	// while tools/ lives at the repo root; walk up from the working directory.
	candidates = append(candidates, ancestorCandidates("tools/browser-fallback.mjs", 4)...)
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("tools/browser-fallback.mjs not found")
}

// ancestorCandidates returns dir/relative looked up in the working directory
// and each of up to depth ancestors, nearest first.
func ancestorCandidates(relative string, depth int) []string {
	var out []string
	dir, err := os.Getwd()
	if err != nil {
		return nil
	}
	for i := 0; i <= depth; i++ {
		out = append(out, filepath.Join(dir, relative))
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return out
}

// tryBrowserFallback is used only when the normal Douyin API path fails on the
// first user-post page. It scrapes visible work links with Chromium, then feeds
// them into the exact same SQLite dedup/history path as API-discovered items.
func (b *BatchService) tryBrowserFallback(ctx context.Context, id, userURL, secUID, nickname string, maxItems int, incremental bool) error {
	cfg := b.deps.Config.Config.BrowserFallback
	if cfg == nil {
		cfg = &config.BrowserFallbackCfg{Enabled: true, MaxScrolls: 240, IdleRounds: 8, WaitTimeoutSeconds: 480}
	}
	if !cfg.Enabled {
		return fmt.Errorf("browser fallback disabled")
	}
	helper, err := b.browserHelperPath()
	if err != nil {
		return err
	}
	items, err := browserfallback.FetchUserPosts(
		ctx,
		helper,
		userURL,
		b.deps.CookieMgr.GetCookies(),
		maxItems,
		cfg.Headless,
		browserfallback.ScrollOptions{
			MaxScrolls:         cfg.MaxScrolls,
			IdleRounds:         cfg.IdleRounds,
			WaitTimeoutSeconds: cfg.WaitTimeoutSeconds,
		},
	)
	if err != nil {
		return err
	}

	for _, raw := range items {
		if raw.AwemeID == "" {
			continue
		}
		known := false
		if b.db != nil {
			known, _ = b.db.HasAweme(raw.AwemeID)
		}
		if incremental && known {
			b.update(id, func(j *BatchJob) { j.Skipped++ })
			continue
		}
		kind := raw.Type
		if kind == "" {
			kind = "video"
		}
		item := BatchItem{
			AwemeID: raw.AwemeID,
			Title:   raw.AwemeID,
			Type:    kind,
			URL:     raw.URL,
			Known:   known,
		}
		if item.URL == "" {
			item.URL = "https://www.douyin.com/video/" + raw.AwemeID
		}
		if b.db != nil {
			_ = b.db.RecordDownload(storage.AwemeRecord{
				AwemeID:      item.AwemeID,
				AwemeType:    item.Type,
				Title:        item.Title,
				AuthorName:   nickname,
				AuthorSecUID: secUID,
				CreateTime:   sql.NullInt64{},
				JobID:        id,
			})
		}
		b.update(id, func(j *BatchJob) {
			j.Items = append(j.Items, item)
			j.Total = len(j.Items) + j.Skipped
			j.Success = len(j.Items)
		})
	}
	return nil
}
