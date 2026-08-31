package server

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	browserfallback "github.com/xuziyue/douyin-downloader/internal/browser"
	"github.com/xuziyue/douyin-downloader/internal/config"
	"github.com/xuziyue/douyin-downloader/internal/core"
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

	// The scraper only yields IDs and URLs; fetch desc/create_time via the
	// detail API (still reachable unsigned) so the UI shows real titles and
	// dates, and the incremental baseline works for fallback-discovered items.
	titles, createTimes := b.enrichFallbackItems(ctx, items)

	for idx, raw := range items {
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
		title := titles[idx]
		if title == "" {
			title = raw.AwemeID
		}
		createTime := createTimes[idx]
		item := BatchItem{
			AwemeID:    raw.AwemeID,
			Title:      title,
			Type:       kind,
			URL:        raw.URL,
			CreateTime: createTime,
			Known:      known,
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
				CreateTime:   sql.NullInt64{Int64: createTime, Valid: createTime > 0},
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

// enrichFallbackItems fills in titles and create times for scraped items via
// the video detail API, with modest concurrency. Items that fail enrichment
// keep empty values and fall back to the raw aweme ID.
func (b *BatchService) enrichFallbackItems(ctx context.Context, items []browserfallback.Item) ([]string, []int64) {
	titles := make([]string, len(items))
	createTimes := make([]int64, len(items))
	apiClient := core.NewDouyinAPIClient(b.deps.CookieMgr.GetCookies(), b.deps.Config.Config.Proxy)
	defer apiClient.Close()

	var wg sync.WaitGroup
	sem := make(chan struct{}, 3)
	for idx, raw := range items {
		if raw.AwemeID == "" {
			continue
		}
		wg.Add(1)
		go func(idx int, awemeID string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if ctx.Err() != nil {
				return
			}
			detail, err := apiClient.GetVideoDetail(ctx, awemeID)
			if err != nil || detail == nil {
				slog.Warn("fallback item enrichment failed", "aweme_id", awemeID, "error", err)
				return
			}
			if desc := strings.TrimSpace(stringValue(detail, "desc")); desc != "" {
				titles[idx] = desc
			}
			createTimes[idx] = int64Value(detail["create_time"])
		}(idx, raw.AwemeID)
	}
	wg.Wait()
	return titles, createTimes
}
