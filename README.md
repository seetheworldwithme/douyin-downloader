# Douyin Downloader

A no-watermark Douyin (TikTok China) video downloader built on a **Go backend + Vue frontend**: paste a Douyin link, resolve it server-side, and stream the video back as a download — nothing is saved on the server.

The frontend is an installable PWA and can be packaged into an Android APK via Capacitor.

## Features

- Paste a Douyin link (video / image-gallery / short link `v.douyin.com`) → resolves title and the no-watermark source
- Streaming download: the backend proxies the video stream; the browser saves it directly; the server stores nothing
- Auth: username/password → token; API endpoints require a token
- Configurable proxy, video quality, and cookies
- PWA: add to home screen, offline shell with live data; APK packaging supported

## Stack

| Layer | Tech | Location |
|-------|------|----------|
| Backend | Go (`net/http`, pure-Go SQLite `modernc.org/sqlite`, no CGO) | `server-go/` |
| Frontend | Vue 3 + Vite + vite-plugin-pwa | `web/` |
| Build output | static files, served by the Go SPA fallback and the nginx container | `server/static/` |
| Config | YAML (`config.yml`) | repo root |
| Deploy | edge-nginx (TLS) + nginx static container + host Go process | `docker/` |

## Quick start

### Prerequisites
- Go ≥ 1.26
- Node.js ≥ 20 (frontend build)

### Local development
```bash
# 1) Backend
cd server-go
go build -o ../.bin/server ./cmd/server
../.bin/server -config ../config.yml          # listens on 127.0.0.1:8000 by default

# 2) Frontend (separate terminal; Vite proxies /api → :8000)
cd web
npm install
npm run dev                                    # http://localhost:5173
```

### Production build
```bash
cd web && npm install && npm run build          # outputs to ../server/static
cd ../server-go && go build -o ../.bin/server ./cmd/server
./.bin/server -config config.yml
```

## Configuration (`config.yml`)

Copy `config.example.yml` to `config.yml` and fill it in. Key fields:

| Field | Description |
|-------|-------------|
| `cookie` / `cookies` | Douyin login cookie (string or key/value); or set `auto_cookie: true` to read `.cookies.json` / `config/cookies.json` |
| `proxy` | HTTP proxy (e.g. `http://127.0.0.1:7890`) |
| `video_quality` | quality policy, default `highest` |
| `auth.username` / `auth.password` | Web login credentials |
| `auth.secret` | token signing secret; if empty, a random one is generated per start (old tokens invalidated on restart) |
| `server.cors_origins` | allowed frontend origins |

Env overrides (prefix `DOUYIN_`): `DOUYIN_COOKIE`, `DOUYIN_PATH`, `DOUYIN_THREAD`, `DOUYIN_PROXY`.

## REST API (`/api/v1`)

| Endpoint | Method | Auth | Description |
|----------|--------|------|-------------|
| `/health` | GET | — | liveness |
| `/login` | POST | — | username/password → token |
| `/resolve` | POST | token | resolve a link → title / filename / aweme_id |
| `/stream` | GET | token | stream the resolved video (proxied, not saved) |

## Deploy

On the server (where the repo is cloned), from the repo root:

```bash
bash start.sh
```

`start.sh` runs **on the server directly** (no SSH wrapper). It will: ① build the frontend → `server/static`; ② compile and start the Go backend in the background (`127.0.0.1:8000`); ③ start the nginx static container (`127.0.0.1:8083`). Public HTTPS is terminated by edge-nginx: `/` → nginx, `/api/` → Go backend. Edge-nginx is a separate repo and is not managed by this script.

See [`docker/README.md`](docker/README.md) for details.

## Project layout

```
server-go/          Go backend (cmd/server + internal/{auth,config,control,core,server,storage,utils})
web/                Vue frontend (builds to server/static)
server/static/      frontend build output (Go SPA fallback + nginx)
start.sh            one-command server bring-up (frontend build + Go backend + nginx container)
docker/             nginx / edge-proxy config (no deploy scripts — use root start.sh)
config.yml          runtime config (gitignored)
config.example.yml  config template
.cookies.json       cookie credentials (gitignored)
```

## Note

The Go backend currently implements only the single-video resolve + streaming-download web flow. The earlier Python CLI's batch-download modes (user posts / likes / mixes / music / live recording / comments / transcription) were not ported. To restore them, add new endpoints under `server-go/internal/server/`.
