# Douyin Downloader

A Go + Vue Douyin downloader with single-post streaming downloads, author/likes/collection batch scanning, server-streamed ZIP downloads, a persistent task center, SQLite media library, PWA support, and a native Android app.

See [README.zh-CN.md](README.zh-CN.md) for the full Chinese documentation and [docs/BATCH_DOWNLOAD.md](docs/BATCH_DOWNLOAD.md) for batch workflow details.

## Web workspaces

- Single link download: video / gallery
- Batch download: author posts, author likes, collection links
- Task center: running + persisted SQLite task history, retry supported
- Media library: search/filter discovered items and re-download

## Batch ZIP

Selected works are fetched from Douyin and written directly into a ZIP response. Media is not persistently stored on the server. Gallery posts are written as image folders inside the ZIP.

## Development

```bash
cd server-go
go test ./...
go vet ./...
go build ./...

cd ../web
npm ci
npm run build
```

Pull requests run the same backend and frontend checks through GitHub Actions.
