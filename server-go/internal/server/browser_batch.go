package server

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	browserfallback "github.com/xuziyue/douyin-downloader/internal/browser"
	"github.com/xuziyue/douyin-downloader/internal/storage"
)

func (b *BatchService) browserHelperPath() (string, error) {
	configDir := b.deps.Config.ConfigDir()
	candidates := []string{
		filepath.Join(configDir, "tools", "browser-fallback.mjs"),
		filepath.Join(filepath.Dir(configDir), "tools", "browser-fallback.mjs"),
		filepath.Join("tools", "browser-fallback.mjs"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("tools/browser-fallback.mjs not found")
}

// tryBrowserFallback is used only when the normal Douyin API path fails on the
// first user-post page. It scrapes visible work links with Chromium, then feeds
// them into the exact same SQLite dedup/history path as API-discovered items.
func (b *BatchService) tryBrowserFallback(ctx context.Context, id, userURL, secUID, nickname string, maxItems int, incremental bool) error {
	cfg := b.deps.Config.Config.BrowserFallback
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
