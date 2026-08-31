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
	Mode           string      `json:"mode"`
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

func normalizeBatchMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "like":
		return "like"
	case "mix", "collection":
		return "mix"
	default:
		return "post"
	}
}

func isBatchMode(mode string) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "post", "like", "mix", "collection":
		return true
	default:
		return false
	}
}

func (b *BatchService) Create(rawURL string, maxItems int, incremental bool, mode string) *BatchJob {
	if maxItems <= 0 {
		maxItems = 50
	}
	if maxItems > 500 {
		maxItems = 500
	}
	job := &BatchJob{
		JobID:       newJobID(),
		URL:         strings.TrimSpace(rawURL),
		Mode:        normalizeBatchMode(mode),
		Status:      "queued",
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
		Incremental: incremental,
		MaxItems:    maxItems,
		Items:       []BatchItem{},
	}
	b.mu.Lock()
	b.jobs[job.JobID] = job
	b.mu.Unlock()
	b.persist(job)
	go b.run(job.JobID)
	return b.snapshot(job.JobID)
}

func (b *BatchService) snapshot(id string) *BatchJob {
	b.mu.RLock()
	job := b.jobs[id]
	if job != nil {
		cp := *job
		cp.Items = append([]BatchItem(nil), job.Items...)
		b.mu.RUnlock()
		return &cp
	}
	b.mu.RUnlock()

	if b.db != nil {
		rec, err := b.db.GetJob(id)
		if err == nil && rec != nil {
			return storedJobToBatch(*rec)
		}
	}
	return nil
}

func storedJobToBatch(rec storage.StoredJob) *BatchJob {
	status := rec.Status
	errText := rec.Error
	if status == "queued" || status == "running" {
		status = "interrupted"
		if errText == "" {
			errText = "服务已重启，原任务未继续执行，可重新执行"
		}
	}
	return &BatchJob{
		JobID:          rec.JobID,
		URL:            rec.URL,
		Mode:           normalizeBatchMode(rec.Mode),
		Status:         status,
		CreatedAt:      rec.CreatedAt,
		StartedAt:      rec.StartedAt,
		FinishedAt:     rec.FinishedAt,
		Total:          int(rec.Total),
		Success:        int(rec.Success),
		Failed:         int(rec.Failed),
		Skipped:        int(rec.Skipped),
		Error:          errText,
		AuthorNickname: rec.AuthorNickname,
		AuthorSecUID:   rec.AuthorSecUID,
		Incremental:    rec.Incremental,
		MaxItems:       rec.MaxItems,
	}
}

func (b *BatchService) List() []*BatchJob {
	byID := map[string]*BatchJob{}
	if b.db != nil {
		if stored, err := b.db.ListRecentJobs(100); err == nil {
			for _, rec := range stored {
				byID[rec.JobID] = storedJobToBatch(rec)
			}
		} else {
			slog.Warn("list persisted jobs failed", "error", err)
		}
	}

	b.mu.RLock()
	for _, job := range b.jobs {
		cp := *job
		cp.Items = append([]BatchItem(nil), job.Items...)
		byID[job.JobID] = &cp
	}
	b.mu.RUnlock()

	out := make([]*BatchJob, 0, len(byID))
	for _, job := range byID {
		out = append(out, job)
	}
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].CreatedAt > out[i].CreatedAt {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	if len(out) > 100 {
		out = out[:100]
	}
	return out
}

func (b *BatchService) Retry(id string) (*BatchJob, error) {
	original := b.snapshot(id)
	if original == nil {
		return nil, fmt.Errorf("job not found")
	}
	if original.URL == "" {
		return nil, fmt.Errorf("job has no source URL")
	}
	return b.Create(original.URL, original.MaxItems, original.Incremental, original.Mode), nil
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
	if err := b.db.UpsertJob(storage.JobRecord{
		JobID: cp.JobID, URL: cp.URL, Status: cp.Status,
		CreatedAt: cp.CreatedAt, StartedAt: cp.StartedAt, FinishedAt: cp.FinishedAt,
		Total: int64(cp.Total), Success: int64(cp.Success), Failed: int64(cp.Failed), Skipped: int64(cp.Skipped),
		Error: cp.Error, AuthorNickname: cp.AuthorNickname, AuthorSecUID: cp.AuthorSecUID,
		Overrides: map[string]any{"mode": cp.Mode, "incremental": cp.Incremental, "max_items": cp.MaxItems},
	}); err != nil {
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

func authorFromAweme(raw map[string]any, fallbackName, fallbackSecUID string) (string, string) {
	author, _ := raw["author"].(map[string]any)
	name := strings.TrimSpace(stringValue(author, "nickname"))
	secUID := strings.TrimSpace(stringValue(author, "sec_uid"))
	if name == "" {
		name = fallbackName
	}
	if secUID == "" {
		secUID = fallbackSecUID
	}
	return name, secUID
}

func mixTitle(m map[string]any, mixID string) string {
	for _, key := range []string{"mix_name", "name", "desc", "title"} {
		if v := strings.TrimSpace(stringValue(m, key)); v != "" {
			return v
		}
	}
	return "合集 " + mixID
}

func (b *BatchService) run(id string) {
	job := b.update(id, func(job *BatchJob) {
		job.Status = "running"
		job.StartedAt = time.Now().UTC().Format(time.RFC3339)
		job.Error = ""
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
	if parsed == nil {
		b.fail(id, fmt.Errorf("无法识别的抖音链接"))
		return
	}

	mode := normalizeBatchMode(job.Mode)
	var secUID, nickname, mixID string
	switch parsed.Type {
	case "user":
		if mode == "mix" {
			b.fail(id, fmt.Errorf("合集模式请粘贴 /collection/ 或 /mix/ 合集链接"))
			return
		}
		if parsed.SecUID == "" {
			b.fail(id, fmt.Errorf("未能从主页链接提取 sec_uid"))
			return
		}
		secUID = parsed.SecUID
		profile, err := apiClient.GetUserInfo(ctx, secUID)
		if err != nil {
			b.fail(id, fmt.Errorf("获取用户信息失败: %w", err))
			return
		}
		nickname = stringValue(profile, "nickname")
		b.update(id, func(j *BatchJob) { j.AuthorNickname, j.AuthorSecUID = nickname, secUID })
	case "collection":
		if parsed.MixID == "" {
			b.fail(id, fmt.Errorf("未能从合集链接提取 mix_id"))
			return
		}
		mode = "mix"
		mixID = parsed.MixID
		detail, err := apiClient.GetMixDetail(ctx, mixID)
		if err != nil {
			b.fail(id, fmt.Errorf("获取合集信息失败: %w", err))
			return
		}
		nickname = mixTitle(detail, mixID)
		b.update(id, func(j *BatchJob) { j.Mode, j.AuthorNickname = "mix", nickname })
	default:
		b.fail(id, fmt.Errorf("批量任务支持作者主页或合集链接，当前类型: %s", parsed.Type))
		return
	}

	var baseline int64
	if mode == "post" && job.Incremental && b.db != nil && secUID != "" {
		baseline, _ = b.db.GetLatestSeenAwemeTime(secUID)
	}

	cursor := int64(0)
	seenCursor := map[int64]bool{}
	stop := false
	for page := 0; page < 100 && !stop; page++ {
		if seenCursor[cursor] && page > 0 {
			break
		}
		seenCursor[cursor] = true

		var resp *core.PagedResponse
		var err error
		switch mode {
		case "like":
			resp, err = apiClient.GetUserLike(ctx, secUID, cursor, 20)
		case "mix":
			resp, err = apiClient.GetMixAweme(ctx, mixID, cursor, 20)
		default:
			resp, err = apiClient.GetUserPost(ctx, secUID, cursor, 20)
		}
		if err != nil {
			if page == 0 && mode == "post" && parsed.Type == "user" {
				if fallbackErr := b.tryBrowserFallback(ctx, id, rawURL, secUID, nickname, job.MaxItems, job.Incremental); fallbackErr == nil {
					slog.Info("user API blocked; Playwright fallback succeeded", "job_id", id)
					stop = true
					break
				} else {
					slog.Warn("Playwright fallback unavailable", "job_id", id, "error", fallbackErr)
				}
			}
			b.fail(id, fmt.Errorf("获取批量作品失败(%s): %w", mode, err))
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
			if mode == "post" && job.Incremental && baseline > 0 && createTime > 0 && createTime <= baseline {
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
			item := BatchItem{
				AwemeID: awemeID,
				Title: title,
				Type: awemeKind(raw),
				CreateTime: createTime,
				URL: "https://www.douyin.com/video/" + awemeID,
				Known: known,
			}

			authorName, authorSecUID := authorFromAweme(raw, nickname, secUID)
			if b.db != nil {
				metadata, _ := json.Marshal(raw)
				_ = b.db.RecordDownload(storage.AwemeRecord{
					AwemeID: awemeID, AwemeType: item.Type, Title: title,
					AuthorName: authorName, AuthorSecUID: authorSecUID,
					CreateTime: sql.NullInt64{Int64: createTime, Valid: createTime > 0},
					Metadata: string(metadata), JobID: id,
				})
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

	final := b.update(id, func(j *BatchJob) {
		j.Status = "completed"
		j.Mode = mode
		j.Total = len(j.Items) + j.Skipped
		j.Success = len(j.Items)
		j.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	})
	if b.db != nil && final != nil {
		_ = b.db.TouchHistory(final.URL, final.Mode, int64(final.Total), int64(final.Success))
	}
}

type createBatchRequest struct {
	URL         string `json:"url"`
	Mode        string `json:"mode"`
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
	if strings.TrimSpace(req.Mode) != "" && !isBatchMode(req.Mode) {
		writeError(w, 400, "mode must be post, like or mix")
		return
	}
	writeJSON(w, http.StatusAccepted, b.Create(req.URL, req.MaxItems, req.Incremental, req.Mode))
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

func (s *Server) handleRetryBatchJob(w http.ResponseWriter, r *http.Request) {
	b := batchServiceFor(s)
	if b == nil {
		writeError(w, 503, "batch service unavailable")
		return
	}
	job, err := b.Retry(r.PathValue("id"))
	if err != nil {
		writeError(w, 404, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	b := batchServiceFor(s)
	if b == nil || b.db == nil {
		writeJSON(w, 200, map[string]any{"items": []any{}})
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	items, err := b.db.ListAwemes(storage.HistoryFilter{
		Limit: limit,
		Offset: offset,
		Query: r.URL.Query().Get("q"),
		Type: r.URL.Query().Get("type"),
		Author: r.URL.Query().Get("author"),
	})
	if err != nil {
		writeError(w, 500, fmt.Sprintf("读取作品库失败: %v", err))
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

type cookieImportRequest struct {
	Cookie string `json:"cookie"`
}

func (s *Server) handleCookieStatus(w http.ResponseWriter, r *http.Request) {
	cookies := s.deps.CookieMgr.GetCookies()
	writeJSON(w, 200, map[string]any{
		"configured": len(cookies) > 0,
		"valid": s.deps.CookieMgr.ValidateCookies(),
		"count": len(cookies),
	})
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
	writeJSON(w, 200, map[string]any{
		"ok": true,
		"valid": s.deps.CookieMgr.ValidateCookies(),
		"count": len(cookies),
	})
}
