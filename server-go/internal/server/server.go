package server

import (
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
	"path/filepath"
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

	// 始终放行 Capacitor WebView 的 origin(https://localhost 安卓、
	// capacitor://localhost iOS):即便 config.yml 自定义了 cors_origins 也
	// 合并而非覆盖,避免 APK 跨域请求被全量拦截("一直离线 / 登录失败")。
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

	info, err := s.resolveVideo(r.Context(), req.URL)
	if err != nil {
		s.handleResolveError(w, err)
		return
	}
	defer info.APIClient.Close()

	writeJSON(w, 200, map[string]string{
		"title":    info.Title,
		"filename": info.Filename,
		"aweme_id": info.AwemeID,
	})
}

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	videoURL := r.URL.Query().Get("url")
	if videoURL == "" {
		writeError(w, 400, "url is required")
		return
	}

	info, err := s.resolveVideo(r.Context(), videoURL)
	if err != nil {
		s.handleResolveError(w, err)
		return
	}

	// Make upstream request
	upstreamReq, err := http.NewRequestWithContext(r.Context(), "GET", info.VideoURL, nil)
	if err != nil {
		info.APIClient.Close()
		writeError(w, 500, fmt.Sprintf("internal error: %v", err))
		return
	}
	for k, v := range info.VideoHdrs {
		upstreamReq.Header.Set(k, v)
	}

	resp, err := info.APIClient.Client().Do(upstreamReq)
	if err != nil {
		info.APIClient.Close()
		writeError(w, 502, fmt.Sprintf("upstream error: %v", err))
		return
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 200))
		resp.Body.Close()
		info.APIClient.Close()
		writeError(w, 502, fmt.Sprintf("上游返回 %d: %s", resp.StatusCode, string(body)))
		return
	}

	// Set response headers
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "video/mp4"
	}

	// Content-Disposition with RFC 5987 encoding
	encodedFilename := url.QueryEscape(info.Filename)
	asciiFallback := strings.Map(func(r rune) rune {
		if r > 127 {
			return -1
		}
		if r == '"' {
			return -1
		}
		return r
	}, info.Filename)
	if strings.TrimSpace(asciiFallback) == "" {
		asciiFallback = "video.mp4"
	}

	w.Header().Set("Content-Disposition",
		fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, asciiFallback, encodedFilename))
	w.Header().Set("Content-Type", contentType)
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		w.Header().Set("Content-Length", cl)
	}
	w.WriteHeader(200)

	// Stream and cleanup
	defer resp.Body.Close()
	defer info.APIClient.Close()
	io.Copy(w, resp.Body)
}

type resolveResult struct {
	APIClient *core.DouyinAPIClient
	*core.VideoInfo
}

func (s *Server) resolveVideo(ctx context.Context, rawURL string) (*resolveResult, error) {
	quality := s.deps.Config.Config.VideoQuality

	apiClient := core.NewDouyinAPIClient(
		s.deps.CookieMgr.GetCookies(),
		s.deps.Config.Config.Proxy,
	)

	info, err := core.ResolveVideo(ctx, rawURL, apiClient, quality)
	if err != nil {
		apiClient.Close()
		return nil, err
	}

	return &resolveResult{
		APIClient:  apiClient,
		VideoInfo:  info,
	}, nil
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
