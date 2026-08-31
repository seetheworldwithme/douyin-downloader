# douyin-downloader

## Purpose
A Douyin (TikTok China) downloader with a Go backend and Vue PWA. It supports single video/gallery streaming downloads plus batch scanning/downloading for author posts, author likes, and collection links. Batch media is streamed into ZIP responses without persistent server-side media storage.

## Architecture

| Layer | Location | Notes |
|-------|----------|-------|
| Go backend | `server-go/` | module `github.com/xuziyue/douyin-downloader` |
| Frontend | `web/` | Vue 3 + Vite + Element Plus + vite-plugin-pwa |
| Android app | `android-app/` | Kotlin + Jetpack Compose native app |
| Build output | `server/static/` | served by Go SPA fallback + nginx container |
| Config | `config.yml`, `config.example.yml` | YAML |
| Cookies | `.cookies.json`, `config/cookies.json` | runtime secrets (gitignored) |
| Deploy | `docker/` | edge-nginx TLS + nginx static + host Go process |

### Go package layout (`server-go/internal/`)

| Package | Responsibility |
|---------|----------------|
| `auth` | Cookie + MS-token management |
| `config` | YAML config load/merge and env overrides |
| `control` | Rate limiter, retry handler, queue manager |
| `core` | Douyin API client, URL parser, video/gallery resolver |
| `browser` | Optional Node/Playwright bridge for user-post fallback |
| `server` | REST handlers, auth, CORS, single streaming, batch jobs, ZIP streaming |
| `storage` | SQLite task/media/history persistence |
| `utils` | signatures, logging, cookie/filename helpers |

### REST API (`/api/v1/`)

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/health` | GET | Liveness |
| `/login` | POST | Username/password -> token |
| `/resolve` | POST | Resolve one video/gallery |
| `/stream` | GET | Stream one video/gallery |
| `/jobs` | POST | Create batch scan job (`post`, `like`, `mix`) |
| `/jobs` | GET | List active + persisted jobs |
| `/jobs/{id}` | GET | Get job status/current in-memory results |
| `/jobs/{id}/retry` | POST | Retry a persisted/finished job |
| `/batch/stream` | GET | Stream selected current job items into ZIP |
| `/history` | GET | Query SQLite media library |
| `/cookies/status` | GET | Cookie status |
| `/cookies/import` | POST | Import Cookie header |

## Web workspaces

`web/src/App.vue` exposes four tabs after login:

1. Link download (`SubmitCard.vue`)
2. Batch download (`BatchCard.vue`)
3. Task center (`TaskCenter.vue`)
4. Media library (`HistoryCard.vue`)

## Batch modes

- `post`: author `/user/{sec_uid}` published works
- `like`: author `/user/{sec_uid}` liked works (visibility/Cookie dependent)
- `mix`: direct `/collection/{mix_id}` or `/mix/{mix_id}` collection

The optional browser fallback currently targets `post` first-page API failures. Like/mix mainly depend on the web API.

## Storage semantics

- Scanned metadata is persisted to SQLite `aweme` rows without inventing a `file_path`.
- `HasAweme()` means "previously discovered" and drives incremental scanning.
- `IsDownloaded()` still means an actual persisted file path exists.
- Job mode/incremental/max-items are stored in the `job.overrides` JSON.
- Historical queued/running rows are exposed as `interrupted` after process restart and can be retried.

## For AI Agents

### Working in this repo
- Go >= 1.26 (`server-go/go.mod`). Pure-Go SQLite; `CGO_ENABLED=0` builds work.
- Backend: `cd server-go && go test ./... && go vet ./... && go build ./...`
- Frontend: `cd web && npm ci && npm run build`
- Dev frontend: `cd web && npm run dev`
- Android: `bash build-android-app.sh`
- Deploy: `bash start.sh`

### Optional Playwright

```bash
cd tools
npm install
npx playwright install chromium
```

Cookie helper:

```bash
node tools/cookie-login.mjs .cookies.json
```

### CI

`.github/workflows/ci.yml` runs Go tests/vet/build and Vue install/build for pull requests.
