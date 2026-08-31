package server

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// NewEnhanced preserves the existing single-post routes and layers the batch,
// task-center, history and cookie-management APIs on top.
func NewEnhanced(deps *ServerDeps, host string, port int) *Server {
	s := &Server{deps: deps}
	attachBatchService(s, deps)

	base := s.routes()
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v1/jobs", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			s.requireUser(s.handleCreateBatchJob)(w, r)
		case http.MethodGet:
			s.requireUser(s.handleListBatchJobs)(w, r)
		default:
			writeError(w, 405, "method not allowed")
		}
	})
	mux.HandleFunc("/api/v1/jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			s.requireUser(s.handleGetBatchJob)(w, r)
			return
		}
		writeError(w, 405, "method not allowed")
	})
	mux.HandleFunc("/api/v1/jobs/{id}/retry", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			s.requireUser(s.handleRetryBatchJob)(w, r)
			return
		}
		writeError(w, 405, "method not allowed")
	})
	mux.HandleFunc("/api/v1/batch/prepare", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			s.requireUser(s.handlePrepareBatchStream)(w, r)
			return
		}
		writeError(w, 405, "method not allowed")
	})
	mux.HandleFunc("/api/v1/batch/stream", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			s.requireUser(s.handleBatchStream)(w, r)
			return
		}
		writeError(w, 405, "method not allowed")
	})
	mux.HandleFunc("/api/v1/history", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			s.requireUser(s.handleHistory)(w, r)
			return
		}
		writeError(w, 405, "method not allowed")
	})
	mux.HandleFunc("/api/v1/cookies/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			s.requireUser(s.handleCookieStatus)(w, r)
			return
		}
		writeError(w, 405, "method not allowed")
	})
	mux.HandleFunc("/api/v1/cookies/import", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			s.requireUser(s.handleCookieImport)(w, r)
			return
		}
		writeError(w, 405, "method not allowed")
	})

	mux.Handle("/", base)

	s.srv = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", host, port),
		Handler:      s.corsMiddleware(mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 0,
		IdleTimeout:  120 * time.Second,
	}
	return s
}

func (s *Server) ShutdownEnhanced(ctx context.Context) error {
	detachBatchService(s)
	return s.Shutdown(ctx)
}
