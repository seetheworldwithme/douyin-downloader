package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/xuziyue/douyin-downloader/internal/config"
	"github.com/xuziyue/douyin-downloader/internal/server"
)

func main() {
	configPath := flag.String("config", "config.yml", "Path to config.yml")
	host := flag.String("host", "127.0.0.1", "Server host")
	port := flag.Int("port", 8000, "Server port")
	flag.Parse()

	// Load config
	cfg, err := config.NewConfigLoader(*configPath)
	if err != nil {
		slog.Error("Failed to load config", "path", *configPath, "error", err)
		os.Exit(1)
	}

	slog.Info("Config loaded",
		"path", *configPath,
		"video_quality", cfg.Config.VideoQuality,
		"proxy", cfg.Config.Proxy,
	)

	// Create server deps
	deps := server.NewServerDeps(cfg)

	// Start server
	srv := server.New(deps, *host, *port)

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		slog.Info("Shutting down...")
		srv.Shutdown(nil)
	}()

	if err := srv.Run(); err != nil {
		slog.Error("Server failed", "error", err)
		os.Exit(1)
	}

	fmt.Println("Server stopped")
}
