# Douyin Downloader

A no-watermark Douyin (TikTok China) video downloader built on a **Go backend + Vue frontend**: paste a Douyin link, resolve it server-side, and stream the video back as a download — nothing is saved on the server.

The frontend is an installable PWA; a native Android app (Kotlin + Jetpack Compose) lives in `android-app/`.

## Features

- Paste a Douyin link (video / image-gallery / short link `v.douyin.com`) → resolves title and the no-watermark source
- Streaming download: the backend proxies the video stream; the browser saves it directly; the server stores nothing
- Auth: username/password → token; API endpoints require a token
- Configurable proxy, video quality, and cookies
- PWA: add to home screen, offline shell with live data
- Native Android app (Kotlin + Compose): saves videos/images straight to the system gallery — `bash build-android-app.sh` → APK

## Stack

| Layer | Tech | Location |
|-------|------|----------|
| Backend | Go (`net/http`, no CGO) | `server-go/` |
| Frontend | Vue 3 + Vite + vite-plugin-pwa | `web/` |
| Android | Kotlin + Jetpack Compose (native) | `android-app/` |
| Build output | static files, served by the Go SPA fallback and the nginx container | `server-go/static/` |
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
cd web && npm install && npm run build          # outputs to ../server-go/static
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

Env overrides: `DOUYIN_COOKIE`, `DOUYIN_PROXY`, `DOUYIN_FFMPEG_PATH`.

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

`start.sh` runs **on the server directly** (no SSH wrapper). It will: ① build the frontend → `server-go/static`; ② compile and start the Go backend in the background (`127.0.0.1:8000`); ③ start the nginx static container (`127.0.0.1:8083`). Public HTTPS is terminated by edge-nginx: `/` → nginx, `/api/` → Go backend. Edge-nginx is a separate repo and is not managed by this script.

See [`docker/README.md`](docker/README.md) for details.

## Project layout

```
server-go/          Go backend (cmd/server + internal/{auth,config,core,server,utils})
web/                Vue frontend (builds to server-go/static)
server-go/static/      frontend build output (Go SPA fallback + nginx)
start.sh            one-command server bring-up (frontend build + Go backend + nginx container)
docker/             nginx / edge-proxy config (no deploy scripts — use root start.sh)
config.yml          runtime config (gitignored)
config.example.yml  config template
.cookies.json       cookie credentials (gitignored)
```

## Note

The backend currently implements only the single-post resolve + streaming-download web flow (videos and gallery posts). Batch-download modes (user posts / likes / mixes / music / live recording / comments / transcription) are not implemented. To add them, create new endpoints under `server-go/internal/server/`.
