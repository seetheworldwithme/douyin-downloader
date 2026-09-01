# douyin-downloader

## 定位
抖音视频下载器。在 Web 界面粘贴抖音分享链接,即可无水印流式下载视频或图集。
后端是单个 Go 二进制(`server-go/`);前端是 Vue + Vite PWA(`web/`),构建产物
输出到 `server/static/`。

## 架构

| 层 | 位置 | 说明 |
|----|------|------|
| Go 后端 | `server-go/` | module `github.com/xuziyue/douyin-downloader` |
| 前端 | `web/` | Vue 3 + Vite + vite-plugin-pwa;构建产物 → `../server/static` |
| 安卓端 | `android-app/` | Kotlin + Jetpack Compose 原生应用(独立于 web),调用同一套 REST API |
| 构建产物 | `server/static/` | 由 Go 服务 SPA 回退和 nginx 容器共同托管 |
| 配置 | `config.yml`、`config.example.yml` | YAML 配置 |
| Cookies | `.cookies.json`、`config/cookies.json` | 运行时敏感数据(gitignore) |
| 部署 | `docker/` | edge-nginx TLS + nginx 静态容器 + 宿主机 Go 进程 |
| 安卓文档 | `docs/APK.md` | APK 构建/安装说明 |

### Go 包结构(`server-go/internal/`)

| 包 | 职责 |
|----|------|
| `cmd/server` | 入口:解析 `-config/-host/-port` 参数、组装依赖、启动服务 |
| `auth` | Cookie 管理(持久化到 `.cookies.json`) |
| `config` | YAML 配置加载/合并、环境变量覆盖(`DOUYIN_*`)、cookie 解析 |
| `core` | 抖音 API 客户端、URL 解析、视频/图集解析 |
| `server` | REST 处理器、类 JWT 鉴权、CORS、SPA 静态托管 |
| `utils` | 反爬签名(a_bogus / X-Bogus)、cookie/文件名/URL 工具 |

### REST API(`/api/v1/`)

| 接口 | 方法 | 鉴权 | 用途 |
|------|------|------|------|
| `/health` | GET | — | 存活探测 |
| `/login` | POST | — | 用户名/密码 → token |
| `/resolve` | POST | token | 解析抖音 URL → 标题/文件名/aweme_id/类型(`video`\|`images`)/图片数/是否有音乐 |
| `/stream` | GET | token | 流式返回解析后的媒体。视频走代理(不落盘)。图集支持 `mode=images`(单图原图 / 多图 ZIP,流式)或 `mode=video`(ffmpeg 合成带原声的幻灯片视频;需服务器有 ffmpeg — `ffmpeg_path` 配置或 PATH,并发转码上限 2) |

非 `/api/` 路径回退到 SPA `index.html`。

## AI 协作须知

### 仓库内开发
- Go ≥ 1.26(`go.mod`:`go 1.26.4`)。无 CGO 依赖,`CGO_ENABLED=0 go build` 可用。
- 本地构建与运行(仓库根目录):
  - 后端:`cd server-go && go build -o ../.bin/server ./cmd/server && ../.bin/server -config ../config.yml`
  - 前端开发:`cd web && npm install && npm run dev`(Vite 代理 `/api` → `127.0.0.1:8000`)
  - 前端生产构建:`cd web && npm run build` → `server/static/`(PWA service-worker 压缩需 Node 20+;`start.sh` 会 source nvm 处理)
  - 安卓:`bash build-android-app.sh`(Windows Git Bash)→ `release/apk/douyin-downloader.apk`,debug 包可直接安装;release 需 `BUILD_VARIANT=release`(自配签名)。也可用 Android Studio 打开 `android-app/`。详见 `docs/APK.md`
- 配置默认值在 `server-go/internal/config/default_config.go`;加载/合并在
  `loader.go`;cookie 解析在 `GetCookies()`。

### 测试
- `cd server-go && go test ./...`(各包单元测试,`*_test.go`)。
- 静态检查/编译:`cd server-go && go vet ./... && go build ./...`。

### 部署
- 在服务器上从仓库根目录一键执行:`bash start.sh`(直接在服务器上运行,无 SSH 包装)。
- `start.sh` 重建前端 → 重编 Go 二进制 → 重启进程 → 重启 nginx 容器。
- edge-nginx 终结 TLS;`/` → nginx 静态(8083),`/api/` → Go 服务(8000)。

### 注意事项
- 后端只实现「单条链接解析 + 流式下载」流程(视频 + 图集)。用户主页 / 点赞 /
  合集 / 音乐 / 直播 / 评论等批量模式未实现,如需新增请在
  `server-go/internal/server/` 下加接口。
- 签名栈(`utils/abogus.go` + `utils/xbogus.go` + gmsm 依赖)目前只有
  `/aweme/v1/play/` 回退路径在用;常规详情接口是故意不签名的(签名会被抖音
  WAF 拒绝),见 `api_client.go` 内注释。删除签名栈前先确认回退路径。
