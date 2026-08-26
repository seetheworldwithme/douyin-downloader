package server

import (
	"archive/zip"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/xuziyue/douyin-downloader/internal/auth"
	"github.com/xuziyue/douyin-downloader/internal/config"
	"github.com/xuziyue/douyin-downloader/internal/core"
	"github.com/xuziyue/douyin-downloader/internal/utils"
)

const tokenTTL = 7 * 24 * time.Hour

var defaultCORS = []string{
	"http://localhost:5173",
	"http://127.0.0.1:5173",
	"https://localhost",
	"http://localhost",
	"capacitor://localhost",
}

// ServerDeps holds shared dependencies for the server.
type ServerDeps struct {
	Config       *config.ConfigLoader
	CookieMgr    *auth.CookieManager
	AuthUsername string
	AuthPassword string
	AuthSecret   string
	AuthAccounts map[string]string
	CorsOrigins  []string
}

// Server is the REST API server.
type Server struct {
	deps *ServerDeps
	srv  *http.Server
}

func NewServerDeps(cfg *config.ConfigLoader) *ServerDeps {
	cookieFile := filepath.Join(cfg.ConfigDir(), ".cookies.json")
	cookieMgr := auth.NewCookieManager(cookieFile)

	initialCookies := cfg.GetCookies()
	if len(initialCookies) > 0 {
		cookieMgr.SetCookies(initialCookies)
	} else {
		cookieMgr.GetCookies() // trigger load from disk
	}

	authCfg := cfg.Config.Auth
	username := authCfg.Username
	if username == "" {
		username = "xuziyue"
	}
	password := authCfg.Password
	if password == "" {
		password = "mmjsxu666555"
	}

	secret := strings.TrimSpace(authCfg.Secret)
	if secret == "" {
		b := make([]byte, 32)
		rand.Read(b)
		secret = base64.RawURLEncoding.EncodeToString(b)
		slog.Warn("auth.secret not configured, using temporary secret (tokens will be lost on restart)")
	}

	accounts := map[string]string{}
	if username != "" && password != "" {
		accounts[username] = password
	}
	for _, u := range authCfg.Users {
		un := strings.TrimSpace(u["username"])
		if un != "" {
			accounts[un] = u["password"]
		}
	}

	// 始终放行 WebView 壳的 origin(https://localhost 安卓、capacitor://localhost
	// iOS,兼容旧版 Capacitor APK;原生 android-app/ 走 OkHttp 不受 CORS 限制):
	// 即便 config.yml 自定义了 cors_origins 也合并而非覆盖,
	// 避免 APK 跨域请求被全量拦截("一直离线 / 登录失败")。
	corsOrigins := append([]string{}, defaultCORS...)
	seen := make(map[string]bool, len(defaultCORS))
	for _, o := range defaultCORS {
		seen[o] = true
	}
	for _, o := range cfg.Config.Server.CorsOrigins {
		if o != "" && !seen[o] {
			corsOrigins = append(corsOrigins, o)
			seen[o] = true
		}
	}

	return &ServerDeps{
		Config:       cfg,
		CookieMgr:    cookieMgr,
		AuthUsername: username,
		AuthPassword: password,
		AuthSecret:   secret,
		AuthAccounts: accounts,
		CorsOrigins:  corsOrigins,
	}
}

// --- JWT-like HMAC token ---

func b64encode(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

func b64decode(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}

func issueToken(username, secret string) (string, error) {
	header, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	now := time.Now().Unix()
	payload, _ := json.Marshal(map[string]int64{
		"sub": 0, // we store username separately
		"iat": now,
		"exp": now + int64(tokenTTL.Seconds()),
	})

	// Store username in payload
	var pm map[string]any
	json.Unmarshal(payload, &pm)
	pm["sub"] = username
	payload, _ = json.Marshal(pm)

	signingInput := b64encode(header) + "." + b64encode(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingInput))
	sig := mac.Sum(nil)

	return signingInput + "." + b64encode(sig), nil
}

func verifyToken(tokenStr, secret string) (map[string]any, bool) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, false
	}

	signingInput := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingInput))
	expectedSig := mac.Sum(nil)

	actualSig, err := b64decode(parts[2])
	if err != nil {
		return nil, false
	}

	if !hmac.Equal(expectedSig, actualSig) {
		return nil, false
	}

	payloadBytes, err := b64decode(parts[1])
	if err != nil {
		return nil, false
	}

	var payload map[string]any
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return nil, false
	}

	exp, ok := payload["exp"].(float64)
	if !ok || int64(exp) < time.Now().Unix() {
		return nil, false
	}

	return payload, true
}

// --- Auth Middleware ---

func (s *Server) requireUser(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var rawToken string

		// Try Authorization header
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
			rawToken = strings.TrimSpace(authHeader[7:])
		}

		// Try query param ?token=
		if rawToken == "" {
			rawToken = r.URL.Query().Get("token")
		}

		if rawToken == "" {
			writeError(w, 401, "未登录")
			return
		}

		payload, ok := verifyToken(rawToken, s.deps.AuthSecret)
		if !ok {
			writeError(w, 401, "登录已过期, 请重新登录")
			return
		}

		ctx := context.WithValue(r.Context(), "user", payload["sub"])
		next(w, r.WithContext(ctx))
	}
}

// --- Handlers ---

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"detail": msg})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid request body")
		return
	}

	expected, ok := s.deps.AuthAccounts[req.Username]
	if !ok || expected != req.Password {
		writeError(w, 401, "用户名或密码错误")
		return
	}

	token, err := issueToken(req.Username, s.deps.AuthSecret)
	if err != nil {
		writeError(w, 500, "token generation failed")
		return
	}
	writeJSON(w, 200, map[string]string{"token": token})
}

type resolveRequest struct {
	URL string `json:"url"`
}

func (s *Server) handleResolve(w http.ResponseWriter, r *http.Request) {
	var req resolveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid request body")
		return
	}
	if req.URL == "" {
		writeError(w, 400, "url is required")
		return
	}

	info, err := s.resolveMedia(r.Context(), req.URL)
	if err != nil {
		s.handleResolveError(w, err)
		return
	}
	defer info.APIClient.Close()

	resp := map[string]any{
		"title":       info.Title,
		"aweme_id":    info.AwemeID,
		"type":        info.Type,
		"image_count": len(info.Images),
	}
	if info.Type == "images" {
		resp["filename"] = info.BaseName
		resp["has_music"] = len(info.MusicURLs) > 0
	} else {
		resp["filename"] = info.Video.Filename
	}
	writeJSON(w, 200, resp)
}

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	videoURL := r.URL.Query().Get("url")
	if videoURL == "" {
		writeError(w, 400, "url is required")
		return
	}

	info, err := s.resolveMedia(r.Context(), videoURL)
	if err != nil {
		s.handleResolveError(w, err)
		return
	}

	mode := r.URL.Query().Get("mode")
	switch {
	case info.Type == "images" && mode == "video":
		s.streamImageVideo(w, r, info)
	case info.Type == "images":
		s.streamImages(w, r, info)
	default:
		s.streamVideo(w, r, info)
	}
}

// streamVideo proxies the upstream no-watermark video without saving to disk.
func (s *Server) streamVideo(w http.ResponseWriter, r *http.Request, res *resolveResult) {
	defer res.APIClient.Close()
	info := res.Video

	// Make upstream request
	upstreamReq, err := http.NewRequestWithContext(r.Context(), "GET", info.VideoURL, nil)
	if err != nil {
		writeError(w, 500, fmt.Sprintf("internal error: %v", err))
		return
	}
	for k, v := range info.VideoHdrs {
		upstreamReq.Header.Set(k, v)
	}

	resp, err := res.APIClient.Client().Do(upstreamReq)
	if err != nil {
		writeError(w, 502, fmt.Sprintf("upstream error: %v", err))
		return
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 200))
		resp.Body.Close()
		writeError(w, 502, fmt.Sprintf("上游返回 %d: %s", resp.StatusCode, string(body)))
		return
	}

	// Set response headers
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "video/mp4"
	}

	setContentDisposition(w, info.Filename, "video.mp4")
	w.Header().Set("Content-Type", contentType)
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		w.Header().Set("Content-Length", cl)
	}
	w.WriteHeader(200)

	// Stream and cleanup
	defer resp.Body.Close()
	io.Copy(w, resp.Body)
}

// streamImages serves a gallery post as raw images: a single image is streamed
// directly, the index param streams one specific image, and multiple images
// without index are packaged into a ZIP on the fly.
func (s *Server) streamImages(w http.ResponseWriter, r *http.Request, res *resolveResult) {
	defer res.APIClient.Close()
	ctx := r.Context()

	if len(res.Images) == 0 {
		writeError(w, 400, "该链接没有可下载的图片")
		return
	}

	// Fetch the first image up front so failures still get a proper status code.
	firstData, firstType, err := core.DownloadImage(ctx, res.APIClient, res.Images[0])
	if err != nil {
		writeError(w, 502, fmt.Sprintf("图片下载失败: %v", err))
		return
	}

	width := max(len(strconv.Itoa(len(res.Images))), 2)
	entryName := func(i int, ext string) string {
		return fmt.Sprintf("%s_%0*d%s", res.BaseName, width, i+1, ext)
	}

	// index 参数:只输出第 index 张原图(安卓端逐张保存进相册,不打包 ZIP)。
	if idxStr := r.URL.Query().Get("index"); idxStr != "" {
		idx, convErr := strconv.Atoi(idxStr)
		if convErr != nil || idx < 0 || idx >= len(res.Images) {
			writeError(w, 400, "invalid image index")
			return
		}
		data, ctype := firstData, firstType
		if idx > 0 {
			data, ctype, err = core.DownloadImage(ctx, res.APIClient, res.Images[idx])
			if err != nil {
				writeError(w, 502, fmt.Sprintf("图片下载失败: %v", err))
				return
			}
		}
		ext := core.ImageExt(data, ctype)
		setContentDisposition(w, entryName(idx, ext), "image"+ext)
		w.Header().Set("Content-Type", core.ImageContentType(ext))
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.WriteHeader(200)
		w.Write(data)
		return
	}

	if len(res.Images) == 1 {
		ext := core.ImageExt(firstData, firstType)
		setContentDisposition(w, res.BaseName+ext, "image"+ext)
		w.Header().Set("Content-Type", core.ImageContentType(ext))
		w.Header().Set("Content-Length", strconv.Itoa(len(firstData)))
		w.WriteHeader(200)
		w.Write(firstData)
		return
	}

	setContentDisposition(w, res.BaseName+".zip", "images.zip")
	w.Header().Set("Content-Type", "application/zip")
	w.WriteHeader(200)

	zw := zip.NewWriter(w)
	defer zw.Close()
	writeEntry := func(name string, data []byte) error {
		hdr := &zip.FileHeader{Name: name, Method: zip.Store, Modified: time.Now()}
		hdr.SetMode(0o644)
		fw, err := zw.CreateHeader(hdr)
		if err != nil {
			return err
		}
		_, err = fw.Write(data)
		return err
	}

	if err := writeEntry(entryName(0, core.ImageExt(firstData, firstType)), firstData); err != nil {
		slog.Error("zip write failed", "error", err)
		return
	}
	for i := 1; i < len(res.Images); i++ {
		data, ctype, err := core.DownloadImage(ctx, res.APIClient, res.Images[i])
		if err != nil {
			// Headers are already sent; leave a note in the archive and stop.
			slog.Error("gallery image download failed", "index", i, "error", err)
			writeEntry("下载失败说明.txt", []byte(fmt.Sprintf("第 %d 张图片下载失败: %v\n", i+1, err)))
			return
		}
		if err := writeEntry(entryName(i, core.ImageExt(data, ctype)), data); err != nil {
			slog.Error("zip write failed", "index", i, "error", err)
			return
		}
	}
}

// composeSlots caps concurrent ffmpeg encodes — composition is CPU intensive
// and a small VPS cannot run several at once.
var composeSlots = make(chan struct{}, 2)

// streamImageVideo composes the gallery into an MP4 slideshow (with the post's
// background music when available) and serves the resulting file.
func (s *Server) streamImageVideo(w http.ResponseWriter, r *http.Request, res *resolveResult) {
	defer res.APIClient.Close()
	ctx := r.Context()

	ffmpegPath, err := core.FindFFmpeg(s.deps.Config.Config.FFmpegPath)
	if err != nil {
		writeError(w, 500, `服务器未安装 ffmpeg,无法把图集合成为视频;请选择"下载图片",或在服务器安装 ffmpeg(如 apt install ffmpeg)后重试`)
		return
	}

	select {
	case composeSlots <- struct{}{}:
		defer func() { <-composeSlots }()
	case <-ctx.Done():
		return
	}

	workDir, err := os.MkdirTemp("", "dydl-compose-*")
	if err != nil {
		writeError(w, 500, fmt.Sprintf("创建临时目录失败: %v", err))
		return
	}
	defer os.RemoveAll(workDir)

	var images []core.SlideshowImage
	for i, item := range res.Images {
		data, ctype, err := core.DownloadImage(ctx, res.APIClient, item)
		if err != nil {
			writeError(w, 502, fmt.Sprintf("第 %d 张图片下载失败: %v", i+1, err))
			return
		}
		path := filepath.Join(workDir, fmt.Sprintf("img_%03d%s", i, core.ImageExt(data, ctype)))
		if err := os.WriteFile(path, data, 0o644); err != nil {
			writeError(w, 500, fmt.Sprintf("写入临时文件失败: %v", err))
			return
		}
		images = append(images, core.SlideshowImage{Path: path, Width: item.Width, Height: item.Height})
	}

	audioPath := ""
	if len(res.MusicURLs) > 0 {
		data, _, err := core.DownloadMusic(ctx, res.APIClient, res.MusicURLs)
		if err != nil {
			slog.Warn("背景音乐下载失败, 改为无声合成", "error", err)
		} else {
			audioPath = filepath.Join(workDir, "music"+core.AudioExt(data))
			if err := os.WriteFile(audioPath, data, 0o644); err != nil {
				slog.Warn("写入音乐临时文件失败, 改为无声合成", "error", err)
				audioPath = ""
			}
		}
	}

	perImage := core.SlideshowPerImageDuration(len(res.Images), res.MusicDurationMS)
	outPath, err := core.ComposeSlideshow(ctx, ffmpegPath, workDir, images, audioPath, perImage)
	if err != nil {
		slog.Error("slideshow compose failed", "aweme_id", res.AwemeID, "error", err)
		writeError(w, 500, fmt.Sprintf("视频合成失败: %v", err))
		return
	}

	f, err := os.Open(outPath)
	if err != nil {
		writeError(w, 500, fmt.Sprintf("打开合成结果失败: %v", err))
		return
	}
	defer f.Close()

	setContentDisposition(w, res.BaseName+".mp4", "video.mp4")
	http.ServeContent(w, r, "", time.Time{}, f)
}

type resolveResult struct {
	APIClient *core.DouyinAPIClient
	*core.MediaInfo
}

func (s *Server) resolveMedia(ctx context.Context, rawURL string) (*resolveResult, error) {
	quality := s.deps.Config.Config.VideoQuality

	apiClient := core.NewDouyinAPIClient(
		s.deps.CookieMgr.GetCookies(),
		s.deps.Config.Config.Proxy,
	)

	info, err := core.ResolveMedia(ctx, rawURL, apiClient, quality)
	if err != nil {
		apiClient.Close()
		return nil, err
	}

	return &resolveResult{
		APIClient:  apiClient,
		MediaInfo:  info,
	}, nil
}

// setContentDisposition sets an RFC 5987 encoded attachment filename.
func setContentDisposition(w http.ResponseWriter, filename, asciiFallback string) {
	encodedFilename := url.QueryEscape(filename)
	ascii := strings.Map(func(r rune) rune {
		if r > 127 {
			return -1
		}
		if r == '"' {
			return -1
		}
		return r
	}, filename)
	if strings.TrimSpace(ascii) == "" {
		ascii = asciiFallback
	}
	w.Header().Set("Content-Disposition",
		fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, ascii, encodedFilename))
}

func (s *Server) handleResolveError(w http.ResponseWriter, err error) {
	// Check for specific error types
	msg := err.Error()
	switch {
	case strings.Contains(msg, "Cookie 已过期") || strings.Contains(msg, "login required"):
		writeError(w, 401, msg)
	case strings.Contains(msg, "无法识别") || strings.Contains(msg, "仅支持") || strings.Contains(msg, "未能"):
		writeError(w, 400, msg)
	case strings.Contains(msg, "未找到"):
		writeError(w, 404, msg)
	default:
		slog.Error("resolve_video failed", "error", err)
		writeError(w, 500, fmt.Sprintf("内部错误: %v", err))
	}
}

// --- CORS ---

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowed := false

		for _, o := range s.deps.CorsOrigins {
			if o == "*" || o == origin {
				allowed = true
				break
			}
		}

		if allowed {
			if origin == "" && len(s.deps.CorsOrigins) > 0 && s.deps.CorsOrigins[0] == "*" {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "*")
		}

		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// --- Router ---

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	// API v1 endpoints
	mux.HandleFunc("/api/v1/health", s.handleHealth)
	mux.HandleFunc("/api/v1/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			s.handleLogin(w, r)
		} else {
			writeError(w, 405, "method not allowed")
		}
	})
	mux.HandleFunc("/api/v1/resolve", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			s.requireUser(s.handleResolve)(w, r)
		} else {
			writeError(w, 405, "method not allowed")
		}
	})
	mux.HandleFunc("/api/v1/stream", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			s.requireUser(s.handleStream)(w, r)
		} else {
			writeError(w, 405, "method not allowed")
		}
	})

	// Static file serving (SPA)
	staticDir := filepath.Join(filepath.Dir(s.deps.Config.ConfigDir()), "server", "static")
	// For now, serve from server-go/server/static if available
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeError(w, 404, "not found")
			return
		}
		// SPA fallback
		http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
	})

	return s.corsMiddleware(mux)
}

// New creates a new server instance.
func New(deps *ServerDeps, host string, port int) *Server {
	s := &Server{deps: deps}
	s.srv = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", host, port),
		Handler:      s.routes(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 0, // no write timeout for streaming
		IdleTimeout:  120 * time.Second,
	}
	return s
}

// Run starts the server.
func (s *Server) Run() error {
	slog.Info("Starting REST server", "addr", s.srv.Addr)
	return s.srv.ListenAndServe()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}

func init() {
	// Ensure utils package is used
	_ = utils.SanitizeFilename
}
