# 抖音下载器（Douyin Downloader）

一个基于 **Go 后端 + Vue 前端** 的抖音无水印下载器，支持单作品下载、作者主页/点赞/合集批量扫描、真正的流式 ZIP 批量下载、任务中心、SQLite 作品库、PWA 和原生 Android App。

前端为可安装的 PWA；安卓端为原生应用（Kotlin + Jetpack Compose），位于 `android-app/`。

## Web 功能

- 单个视频 / 图集解析与下载
- 作者主页发布作品批量扫描
- 作者点赞作品批量扫描
- 合集链接批量扫描
- SQLite 增量去重
- 多选 / 全选作品，后端流式打包 ZIP（服务器不长期保存媒体）
- 持久化任务中心，失败/中断任务可重新执行
- SQLite 作品库，支持关键词 / 作者 / 类型筛选
- Cookie 状态检查与 Web 导入
- 可选 Playwright 浏览器兜底
- PWA，可添加到主屏幕

## 技术栈

| 层 | 技术 | 位置 |
|----|------|------|
| 后端 | Go（`net/http`，纯 Go SQLite `modernc.org/sqlite`，无需 CGO） | `server-go/` |
| 前端 | Vue 3 + Vite + Element Plus + vite-plugin-pwa | `web/` |
| 安卓 | Kotlin + Jetpack Compose（原生） | `android-app/` |
| 前端构建产物 | 静态文件，由 Go 服务 SPA 兜底托管，也由 nginx 容器托管 | `server/static/` |
| 配置 | YAML（`config.yml`） | 仓库根 |
| 部署 | edge-nginx（TLS 终结）+ nginx 静态容器 + 宿主机 Go 进程 | `docker/` |

## Web 页面

登录后包含四个工作区：

1. **链接下载**：单视频 / 图集。
2. **批量下载**：发布作品、点赞作品、合集；支持多选和 ZIP 下载。
3. **任务中心**：运行中 + SQLite 历史任务，可重新执行。
4. **作品库**：查询已发现作品并重新下载。

## 快速开始

### 前置依赖

- Go ≥ 1.26
- Node.js ≥ 20

### 本地开发

```bash
# 后端
cd server-go
go build -o ../.bin/server ./cmd/server
../.bin/server -config ../config.yml

# 前端（另一个终端）
cd web
npm install
npm run dev
```

### 生产构建

```bash
cd web && npm install && npm run build
cd ../server-go && go build -o ../.bin/server ./cmd/server
./.bin/server -config config.yml
```

## REST API（`/api/v1`）

| 接口 | 方法 | 说明 |
|------|------|------|
| `/health` | GET | 存活检查 |
| `/login` | POST | 用户名密码换 token |
| `/resolve` | POST | 单作品解析 |
| `/stream` | GET | 单作品流式下载 |
| `/jobs` | POST | 创建批量扫描任务 |
| `/jobs` | GET | 任务中心列表 |
| `/jobs/{id}` | GET | 任务状态 / 当前作品结果 |
| `/jobs/{id}/retry` | POST | 重新执行任务 |
| `/batch/stream` | GET | 选中作品流式打包 ZIP |
| `/history` | GET | SQLite 作品库查询 |
| `/cookies/status` | GET | Cookie 状态 |
| `/cookies/import` | POST | Web 导入 Cookie |

## 批量模式

作者主页：

```text
https://www.douyin.com/user/<sec_uid>
```

可选择：

- `post`：发布作品
- `like`：点赞作品

合集：

```text
https://www.douyin.com/collection/<mix_id>
https://www.douyin.com/mix/<mix_id>
```

使用 `mix` 模式。

## 浏览器兜底与自动 Cookie（可选）

```bash
cd tools
npm install
npx playwright install chromium
```

自动登录并保存 Cookie：

```bash
node tools/cookie-login.mjs .cookies.json
```

详细说明见 [`docs/BATCH_DOWNLOAD.md`](docs/BATCH_DOWNLOAD.md)。

## CI

PR 会自动执行：

```bash
cd server-go
go test ./...
go vet ./...
go build ./...

cd ../web
npm ci
npm run build
```

## 部署

```bash
bash start.sh
```

`start.sh` 构建前端、编译并启动 Go 后端、启动 nginx 静态容器。对外 HTTPS 由 edge-nginx 终结。

## 项目结构

```text
server-go/          Go 后端
web/                Vue Web / PWA
android-app/        Kotlin 原生 Android
server/static/      Web 构建产物
server-go/internal/server/
  batch_jobs.go     批量任务与模式编排
  batch_stream.go   流式 ZIP 批量下载
server-go/internal/storage/
  database.go       SQLite 基础存储
  query.go          任务/作品库查询
web/src/components/
  SubmitCard.vue    单链接下载
  BatchCard.vue     批量下载
  TaskCenter.vue    任务中心
  HistoryCard.vue   作品库
tools/              Playwright 浏览器兜底与 Cookie 登录
```
