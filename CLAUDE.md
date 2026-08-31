# Claude Code Notes

This repository is a Go + Vue Douyin downloader.

Key workflows:

- Single video/gallery: `/api/v1/resolve` + `/api/v1/stream`
- Batch scan: `/api/v1/jobs` with `mode=post|like|mix`
- Batch ZIP: `/api/v1/batch/stream`
- Task center: `/api/v1/jobs`, persisted in SQLite
- Media library: `/api/v1/history`
- Cookie management: `/api/v1/cookies/status`, `/api/v1/cookies/import`

Web UI tabs live in `web/src/App.vue` and use:

- `SubmitCard.vue`
- `BatchCard.vue`
- `TaskCenter.vue`
- `HistoryCard.vue`

Before committing backend/frontend changes run:

```bash
cd server-go && go test ./... && go vet ./... && go build ./...
cd ../web && npm ci && npm run build
```

GitHub Actions runs these checks for pull requests.
