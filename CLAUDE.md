# douyin-downloader Claude Guidance

Read `AGENTS.md` for the full project rules.

## Architecture

- **Backend**: Go server in `server-go/` (module `github.com/xuziyue/douyin-downloader`).
  Entry point `server-go/cmd/server/main.go`; REST API under `server-go/internal/server/`.
- **Frontend**: Vue + Vite app in `web/`, built output lands in `server/static/`
  (served by the Go server's SPA fallback and by the nginx container in `docker/`).
- **Config**: `config.yml` (YAML) + `.cookies.json` / `config/cookies.json` — read by the Go server.
