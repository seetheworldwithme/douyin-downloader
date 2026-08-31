package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/xuziyue/douyin-downloader/internal/config"
	"github.com/xuziyue/douyin-downloader/internal/server"
)

func main() {
	configPath := flag.String("config", "config.yml", "Path to config.yml")
	host := flag.String("host", "127.0.0.1", "Server host")
	port := flag.Int("port", 8000, "Server port")
	flag.Parse()

	cfg, err := config.NewConfigLoader(*configPath)
	if err != nil {
		slog.Error("Failed to load config", "path", *configPath, "error", err)
		os.Exit(1)
	}

	slog.Info("Config loaded", "path", *configPath,
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
