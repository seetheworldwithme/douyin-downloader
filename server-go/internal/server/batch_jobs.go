package server

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/xuziyue/douyin-downloader/internal/core"
	"github.com/xuziyue/douyin-downloader/internal/storage"
	"github.com/xuziyue/douyin-downloader/internal/utils"
)

type BatchItem struct {
	AwemeID    string `json:"aweme_id"`
	Title      string `json:"title"`
	Type       string `json:"type"`
	CreateTime int64  `json:"create_time"`
	URL        string `json:"url"`
	Known      bool   `json:"known"`
}

type BatchJob struct {
	JobID          string      `json:"job_id"`
	URL            string      `json:"url"`
	Status         string      `json:"status"`
	CreatedAt      string      `json:"created_at"`
	StartedAt      string      `json:"started_at,omitempty"`
	FinishedAt     string      `json:"finished_at,omitempty"`
	Total          int         `json:"total"`
	Success        int         `json:"success"`
	Failed         int         `json:"failed"`
	Skipped        int         `json:"skipped"`
	Error          string      `json:"error,omitempty"`
	AuthorNickname string      `json:"author_nickname,omitempty"`
	AuthorSecUID   string      `json:"author_sec_uid,omitempty"`
	Incremental    bool        `json:"incremental"`
	MaxItems       int         `json:"max_items"`
	Items          []BatchItem `json:"items,omitempty"`
}

type BatchService struct {
	deps *ServerDeps
	db   *storage.Database
	mu   sync.RWMutex
	jobs map[string]*BatchJob
}

var batchServices sync.Map

func attachBatchService(s *Server, deps *ServerDeps) *BatchService {
	b := NewBatchService(deps)
	batchServices.Store(s, b)
	return b
}

func batchServiceFor(s *Server) *BatchService {
	if v, ok := batchServices.Load(s); ok {
		return v.(*BatchService)
	}
	return nil
}

func detachBatchService(s *Server) {
	if v, ok := batchServices.LoadAndDelete(s); ok {
		v.(*BatchService).Close()
	}
}

func NewBatchService(deps *ServerDeps) *BatchService {
	b := &BatchService{deps: deps, jobs: make(map[string]*BatchJob)}
	if deps.Config.Config.Database {
		db := storage.NewDatabase(deps.Config.Config.DatabasePath)
		if err := db.Initialize(); err != nil {
			slog.Error("batch database init failed; continuing without persistence", "error", err)
		} else {
			b.db = db
		}
	}
	return b
}

func (b *BatchService) Close() {
	if b != nil && b.db != nil {
		_ = b.db.Close()
	}
}

func newJobID() string {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err == nil {
		return fmt.Sprintf("job_%d_%s", time.Now().Unix(), hex.EncodeToString(buf))
	}
	return fmt.Sprintf("job_%d", time.Now().UnixNano())
}

func (b *BatchService) Create(rawURL string, maxItems int, incremental bool) *BatchJob {
	if maxItems <= 0 {
		maxItems = 50
	}
	if maxItems > 500 {
		maxItems = 500
	}
	job := &BatchJob{JobID: newJobID(), URL: strings.TrimSpace(rawURL), Status: "queued",
		CreatedAt: time.Now().UTC().Format(time.RFC3339), Incremental: incremental,
		MaxItems: maxItems, Items: []BatchItem{}}
	b.mu.Lock()
	b.jobs[job.JobID] = job
	b.mu.Unlock()
	b.persist(job)
	go b.run(job.JobID)
	return b.snapshot(job.JobID)
}

func (b *BatchService) snapshot(id string) *BatchJob {
	b.mu.RLock()
	defer b.mu.RUnlock()
	job := b.jobs[id]
	if job == nil {
		return nil
	}
	cp := *job
	cp.Items = append([]BatchItem(nil), job.Items...)
	return &cp
}

func (b *BatchService) List() []*BatchJob {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]*BatchJob, 0, len(b.jobs))
	for _, job := range b.jobs {
		cp := *job
		cp.Items = append([]BatchItem(nil), job.Items...)
		out = append(out, &cp)
	}
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].CreatedAt > out[i].CreatedAt {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	if len(out) > 50 {
		out = out[:50]
	}
	return out
}

func (b *BatchService) update(id string, fn func(*BatchJob)) *BatchJob {
	b.mu.Lock()
	job := b.jobs[id]
	if job != nil {
		fn(job)
	}
	b.mu.Unlock()
	if job != nil {
		b.persist(job)
	}
	return b.snapshot(id)
}

func (b *BatchService) fail(id string, err error) {
	b.update(id, func(job *BatchJob) {
		job.Status = "failed"
		job.Error = err.Error()
		job.Failed++
		job.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	})
}

func (b *BatchService) persist(job *BatchJob) {
	if b.db == nil || job == nil {
		return
	}
	cp := *job
	if err := b.db.UpsertJob(storage.JobRecord{JobID: cp.JobID, URL: cp.URL, Status: cp.Status,
		CreatedAt: cp.CreatedAt, StartedAt: cp.StartedAt, FinishedAt: cp.FinishedAt,
		Total: int64(cp.Total), Success: int64(cp.Success), Failed: int64(cp.Failed), Skipped: int64(cp.Skipped),
		Error: cp.Error, AuthorNickname: cp.AuthorNickname, AuthorSecUID: cp.AuthorSecUID,
		Overrides: map[string]any{"incremental": cp.Incremental, "max_items": cp.MaxItems}}); err != nil {
		slog.Warn("persist batch job failed", "job_id", cp.JobID, "error", err)
	}
}

func stringValue(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	if s, ok := m[key].(string); ok {
		return s
	}
	return ""
}

func int64Value(v any) int64 {
	switch n := v.(type) {
	case int:
		return int64(n)
	case int64:
		return n
	case float64:
		return int64(n)
	case json.Number:
		i, _ := n.Int64()
		return i
	}
	return 0
}

func awemeKind(item map[string]any) string {
	if images, ok := item["images"].([]any); ok && len(images) > 0 {
		return "images"
	}
	return "video"
}

func (b *BatchService) run(id string) {
	job := b.update(id, func(job *BatchJob) {
		job.Status = "running"
		job.StartedAt = time.Now().UTC().Format(time.RFC3339)
	})
	if job == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	apiClient := core.NewDouyinAPIClient(b.deps.CookieMgr.GetCookies(), b.deps.Config.Config.Proxy)
	defer apiClient.Close()
	rawURL := job.URL
	if utils.IsShortURL(rawURL) {
		resolved, err := apiClient.ResolveShortURL(ctx, utils.NormalizeShortURL(rawURL))
		if err != nil {
			b.fail(id, fmt.Errorf("短链解析失败: %w", err))
			return
		}
		if resolved == "" {
			b.fail(id, fmt.Errorf("短链没有返回有效跳转地址"))
			return
		}
		rawURL = resolved
	}
	parsed := core.ParseURL(rawURL)
	if parsed == nil || parsed.Type != "user" || parsed.SecUID == "" {
		b.fail(id, fmt.Errorf("批量任务仅支持抖音用户主页链接"))
		return
	}

	profile, err := apiClient.GetUserInfo(ctx, parsed.SecUID)
	if err != nil {
		b.fail(id, fmt.Errorf("获取用户信息失败: %w", err))
		return
	}
	nickname := stringValue(profile, "nickname")
	b.update(id, func(job *BatchJob) { job.AuthorNickname, job.AuthorSecUID = nickname, parsed.SecUID })

	var baseline int64
	if job.Incremental && b.db != nil {
		baseline, _ = b.db.GetLatestSeenAwemeTime(parsed.SecUID)
	}

	cursor := int64(0)
	seenCursor := map[int64]bool{}
	stop := false
	for page := 0; page < 100 && !stop; page++ {
		if seenCursor[cursor] && page > 0 {
			break
		}
		seenCursor[cursor] = true
		resp, err := apiClient.GetUserPost(ctx, parsed.SecUID, cursor, 20)
		if err != nil {
			b.fail(id, fmt.Errorf("获取主页作品失败: %w", err))
			return
		}
		if len(resp.Items) == 0 {
			break
		}
		for _, raw := range resp.Items {
			awemeID := stringValue(raw, "aweme_id")
			if awemeID == "" {
				continue
			}
			createTime := int64Value(raw["create_time"])
			if job.Incremental && baseline > 0 && createTime > 0 && createTime <= baseline {
				stop = true
				break
			}
			known := false
			if b.db != nil {
				known, _ = b.db.HasAweme(awemeID)
			}
			if job.Incremental && known {
				b.update(id, func(j *BatchJob) { j.Skipped++ })
				continue
			}
			title := strings.TrimSpace(stringValue(raw, "desc"))
			if title == "" {
				title = awemeID
			}
			item := BatchItem{AwemeID: awemeID, Title: title, Type: awemeKind(raw), CreateTime: createTime,
				URL: "https://www.douyin.com/video/" + awemeID, Known: known}
			if b.db != nil {
				metadata, _ := json.Marshal(raw)
				_ = b.db.RecordDownload(storage.AwemeRecord{AwemeID: awemeID, AwemeType: item.Type, Title: title,
					AuthorName: nickname, AuthorSecUID: parsed.SecUID,
					CreateTime: sql.NullInt64{Int64: createTime, Valid: createTime > 0}, Metadata: string(metadata), JobID: id})
			}
			current := b.update(id, func(j *BatchJob) {
				j.Items = append(j.Items, item)
				j.Total = len(j.Items) + j.Skipped
				j.Success = len(j.Items)
			})
			if current != nil && len(current.Items) >= job.MaxItems {
				stop = true
				break
			}
		}
		if stop || !resp.HasMore || resp.MaxCursor == cursor {
			break
		}
		cursor = resp.MaxCursor
	}

	final := b.update(id, func(job *BatchJob) {
		job.Status = "completed"
		job.Total = len(job.Items) + job.Skipped
		job.Success = len(job.Items)
		job.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	})
	if b.db != nil && final != nil {
		_ = b.db.TouchHistory(final.URL, int64(final.Total), int64(final.Success))
	}
}

type createBatchRequest struct {
	URL         string `json:"url"`
	MaxItems    int    `json:"max_items"`
	Incremental bool   `json:"incremental"`
}

func (s *Server) handleCreateBatchJob(w http.ResponseWriter, r *http.Request) {
	b := batchServiceFor(s)
	if b == nil {
		writeError(w, 503, "batch service unavailable")
		return
	}
	var req createBatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid request body")
		return
	}
	if strings.TrimSpace(req.URL) == "" {
		writeError(w, 400, "url is required")
		return
	}
	writeJSON(w, http.StatusAccepted, b.Create(req.URL, req.MaxItems, req.Incremental))
}

func (s *Server) handleListBatchJobs(w http.ResponseWriter, r *http.Request) {
	b := batchServiceFor(s)
	if b == nil {
		writeError(w, 503, "batch service unavailable")
		return
	}
	writeJSON(w, 200, map[string]any{"items": b.List()})
}

func (s *Server) handleGetBatchJob(w http.ResponseWriter, r *http.Request) {
	b := batchServiceFor(s)
	if b == nil {
		writeError(w, 503, "batch service unavailable")
		return
	}
	job := b.snapshot(r.PathValue("id"))
	if job == nil {
		writeError(w, 404, "job not found")
		return
	}
	writeJSON(w, 200, job)
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	b := batchServiceFor(s)
	if b == nil || b.db == nil {
		writeJSON(w, 200, map[string]any{"items": []any{}})
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := b.db.ListRecentAwemes(limit)
	if err != nil {
		writeError(w, 500, fmt.Sprintf("读取历史失败: %v", err))
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

type cookieImportRequest struct { Cookie string `json:"cookie"` }

func (s *Server) handleCookieStatus(w http.ResponseWriter, r *http.Request) {
	cookies := s.deps.CookieMgr.GetCookies()
	writeJSON(w, 200, map[string]any{"configured": len(cookies) > 0,
		"valid": s.deps.CookieMgr.ValidateCookies(), "count": len(cookies)})
}

func (s *Server) handleCookieImport(w http.ResponseWriter, r *http.Request) {
	var req cookieImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid request body")
		return
	}
	cookies := utils.ParseCookieHeader(strings.TrimSpace(req.Cookie))
	if len(cookies) == 0 {
		writeError(w, 400, "未解析到有效 Cookie")
		return
	}
	s.deps.CookieMgr.SetCookies(cookies)
	writeJSON(w, 200, map[string]any{"ok": true, "valid": s.deps.CookieMgr.ValidateCookies(), "count": len(cookies)})
}
