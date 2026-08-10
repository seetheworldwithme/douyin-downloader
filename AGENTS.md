# douyin-downloader

## Purpose
A Douyin (TikTok China) video downloader. Paste a Douyin link in the web UI, get the
no-watermark video back as a streaming download. Backend is a single Go binary
(`server-go/`); frontend is a Vue + Vite PWA (`web/`) that builds to `server/static/`.

## Architecture

| Layer | Location | Notes |
|-------|----------|-------|
| Go backend | `server-go/` | module `github.com/xuziyue/douyin-downloader` |
| Frontend | `web/` | Vue 3 + Vite + vite-plugin-pwa; builds to `../server/static` |
| Build output | `server/static/` | served by Go SPA fallback + nginx container |
| Config | `config.yml`, `config.example.yml` | YAML, same schema the Python version used |
| Cookies | `.cookies.json`, `config/cookies.json` | runtime secrets (gitignored) |
| Deploy | `docker/` | edge-nginx TLS + nginx static container + host Go process |

### Go package layout (`server-go/internal/`)

| Package | Responsibility |
|---------|----------------|
| `cmd/server` | Entry point: parses `-config/-host/-port` flags, wires deps, runs server |
| `auth` | Cookie + MS-token management |
| `config` | YAML config load/merge, env overrides (`DOUYIN_*`), cookie resolution |
| `control` | Rate limiter, retry handler, queue manager |
| `core` | Douyin API client, URL parser, video resolver |
| `server` | REST handlers, JWT-like auth, CORS, SPA static serving |
| `storage` | SQLite (history/aweme/jobs) via `modernc.org/sqlite` (pure Go, no CGO) |
| `utils` | Anti-bot signatures (a/bogus, xbogus), logging, cookie/filename helpers |

### REST API (`/api/v1/`)

| Endpoint | Method | Auth | Purpose |
|----------|--------|------|---------|
| `/health` | GET | — | Liveness |
| `/login` | POST | — | Username/password → token |
| `/resolve` | POST | token | Resolve a Douyin URL → title/filename/aweme_id |
| `/stream` | GET | token | Stream the resolved video (proxied; no on-disk save) |

Non-`/api/` paths fall back to the SPA `index.html`.

## For AI Agents

### Working in this repo
- Go ≥ 1.26 (`go.mod`: `go 1.26.4`). Pure-Go SQLite — `CGO_ENABLED=0 go build` works.
- Build & run locally (from repo root):
  - Backend: `cd server-go && go build -o ../.bin/server ./cmd/server && ../.bin/server -config ../config.yml`
  - Frontend dev: `cd web && npm install && npm run dev` (Vite proxies `/api` → `127.0.0.1:8000`)
  - Frontend prod build: `cd web && npm run build` → `server/static/`
- Config defaults live in `server-go/internal/config/default_config.go`;
  loader/merge in `loader.go`; cookie resolution in `GetCookies()`.

### Testing
- `cd server-go && go test ./...` (unit tests per package, `*_test.go`).
- Lint/build: `cd server-go && go vet ./... && go build ./...`.

### Deploy
- One command from repo root: `bash docker/deploy.sh` (SSH deploys to the server).
- `docker/start.sh` rebuilds frontend, recompiles the Go binary, restarts it, restarts nginx container.
- Edge-nginx terminates TLS; `/` → nginx static (8083), `/api/` → Go server (8000).

### Scope note
The Go backend implements the **single-video web download** flow only
(resolve + stream). The old Python CLI batch modes (user posts/likes/mixes/music/live/
transcription/comments) are not ported. Add new server endpoints under `server-go/internal/server/`
if batch features are needed again.
