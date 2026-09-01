# 抖音下载器（Douyin Downloader）

一个基于 **Go 后端 + Vue 前端** 的抖音无水印视频下载器：粘贴抖音链接，在线解析并流式下载视频，无需在服务器落盘。

前端为可安装的 PWA；安卓端为原生应用（Kotlin + Jetpack Compose），位于 `android-app/`。

## 功能

- 粘贴抖音链接（视频 / 图文 / 短链 `v.douyin.com`）→ 解析标题与无水印视频源
- 流式下载：后端中转视频流，浏览器直接另存为文件，服务器不保存视频
- 登录鉴权：用户名 / 密码换取 token，接口需带 token 访问
- 可配置代理、视频清晰度、Cookie
- PWA：可添加到主屏幕，离线壳 + 实时数据
- 原生安卓 App（Kotlin + Compose）：视频/图片直接存系统相册 —— `bash build-android-app.sh` 打 APK

## 技术栈

| 层 | 技术 | 位置 |
|----|------|------|
| 后端 | Go（`net/http`，无需 CGO） | `server-go/` |
| 前端 | Vue 3 + Vite + vite-plugin-pwa | `web/` |
| 安卓 | Kotlin + Jetpack Compose（原生） | `android-app/` |
| 前端构建产物 | 静态文件，由 Go 服务 SPA 兜底托管，也由 nginx 容器托管 | `server-go/static/` |
| 配置 | YAML（`config.yml`） | 仓库根 |
| 部署 | edge-nginx（TLS 终结）+ nginx 静态容器 + 宿主机 Go 进程 | `docker/` |

## 快速开始

### 前置依赖
- Go ≥ 1.26
- Node.js ≥ 20（前端构建）

### 本地开发
```bash
# 1) 后端
cd server-go
go build -o ../.bin/server ./cmd/server
../.bin/server -config ../config.yml          # 默认监听 127.0.0.1:8000

# 2) 前端(另一个终端,Vite dev server 代理 /api → :8000)
cd web
npm install
npm run dev                                    # http://localhost:5173
```

### 生产构建
```bash
cd web && npm install && npm run build          # 产物输出到 ../server-go/static
cd ../server-go && go build -o ../.bin/server ./cmd/server
./.bin/server -config config.yml
```

## 配置（`config.yml`）

复制 `config.example.yml` 为 `config.yml` 并填写。关键字段：

| 字段 | 说明 |
|------|------|
| `cookie` / `cookies` | 抖音登录 Cookie（字符串或键值对）；也可设 `auto_cookie: true` 自动读取 `.cookies.json` / `config/cookies.json` |
| `proxy` | HTTP 代理（如 `http://127.0.0.1:7890`） |
| `video_quality` | 清晰度策略，默认 `highest` |
| `auth.username` / `auth.password` | Web 登录账号密码 |
| `auth.secret` | token 签名密钥；不填则每次启动随机生成（重启后旧 token 失效） |
| `server.cors_origins` | 允许的前端来源 |

环境变量覆盖：`DOUYIN_COOKIE`、`DOUYIN_PROXY`、`DOUYIN_FFMPEG_PATH`。

## REST API（`/api/v1`）

| 接口 | 方法 | 鉴权 | 说明 |
|------|------|------|------|
| `/health` | GET | — | 存活检查 |
| `/login` | POST | — | 用户名密码 → token |
| `/resolve` | POST | token | 解析链接 → 标题 / 文件名 / aweme_id |
| `/stream` | GET | token | 流式下载视频（中转，不落盘） |

## 部署

在服务器上（代码已 clone 到本地），仓库根执行：

```bash
bash start.sh
```

`start.sh` **直接在服务器上运行**（无 SSH 包装），会：① 构建前端 → `server-go/static`；② 编译并后台启动 Go 后端（`127.0.0.1:8000`）；③ 启动 nginx 静态容器（`127.0.0.1:8083`）。对外 HTTPS 由 edge-nginx 终结：`/` → nginx，`/api/` → Go 后端。edge-nginx 是独立仓库，本脚本不管理。

详见 [`docker/README.md`](docker/README.md)。

## 项目结构

```
server-go/          Go 后端（cmd/server + internal/{auth,config,core,server,utils}）
web/                Vue 前端（构建到 server-go/static）
server-go/static/      前端构建产物（Go SPA 兜底 + nginx 托管）
start.sh            服务器一键启动（构建前端 + Go 后端 + nginx 容器）
docker/             nginx / edge-proxy 配置（无部署脚本，用根 start.sh）
config.yml          运行配置（gitignored）
config.example.yml  配置模板
.cookies.json       Cookie 凭据（gitignored）
```

## 说明

本仓库后端目前只实现「单条链接解析 + 流式下载」的 Web 能力（视频 / 图集均支持）。批量下载模式（用户主页 / 点赞 / 合集 / 音乐 / 直播录制 / 评论采集 / 转写等）尚未实现，如需新增，可在 `server-go/internal/server/` 下新增接口。
