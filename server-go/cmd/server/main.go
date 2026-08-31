package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/xuziyue/douyin-downloader/internal/config"
	"github.com/xuziyue/douyin-downloader/internal/server"
)

// resolveConfigPath finds the config when the server is launched from a
// subdirectory (e.g. `cd server-go && go run ./cmd/server`): walk up from the
// working directory looking for the file, so the repo-root config.yml is found
// regardless of where the process starts.
func resolveConfigPath(path string) string {
	if _, err := os.Stat(path); err == nil {
		return path
	}
	name := filepath.Base(path)
	dir, err := os.Getwd()
	if err != nil {
		return path
	}
	for {
		candidate := filepath.Join(dir, name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return path
		}
		dir = parent
	}
}

func main() {
	configPath := flag.String("config", "config.yml", "Path to config.yml")
	host := flag.String("host", "127.0.0.1", "Server host")
	port := flag.Int("port", 8000, "Server port")
	flag.Parse()

	resolved := resolveConfigPath(*configPath)
	if resolved != *configPath {
		slog.Info("Config not found in working directory; using ancestor config", "path", resolved)
	}
	// Runtime files (.cookies.json, sqlite db, downloads) resolve relative to
	// the working directory; move next to the config so they live in one place
	// no matter where the server was launched from.
	if err := os.Chdir(filepath.Dir(resolved)); err != nil {
		slog.Error("Failed to change to config directory", "dir", filepath.Dir(resolved), "error", err)
		os.Exit(1)
	}

	cfg, err := config.NewConfigLoader(resolved)
	if err != nil {
		slog.Error("Failed to load config", "path", resolved, "error", err)
		os.Exit(1)
	}

	slog.Info("Config loaded", "path", resolved,
		"video_quality", cfg.Config.VideoQuality, "proxy", cfg.Config.Proxy)

	deps := server.NewServerDeps(cfg)
	srv := server.NewEnhanced(deps, *host, *port)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		slog.Info("Shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.ShutdownEnhanced(ctx); err != nil {
			slog.Warn("Graceful shutdown failed", "error", err)
		}
	}()

	if err := srv.Run(); err != nil && err != http.ErrServerClosed {
		slog.Error("Server failed", "error", err)
		os.Exit(1)
	}
	fmt.Println("Server stopped")
}
