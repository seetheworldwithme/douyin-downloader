# Web 批量下载、任务中心与作品库

本分支在原有「单视频 / 图集解析 + 流式下载」能力之上增加完整的 Web 批量工作流：作者发布/点赞扫描、合集扫描、SQLite 增量去重、真正的 ZIP 批量下载、持久化任务中心、作品库筛选、浏览器兜底和 Cookie 管理。

## Web 页面

登录后页面分为四个工作区：

1. **链接下载**：保留原来的单视频 / 图集下载。
2. **批量下载**：扫描作者发布、点赞或合集，支持全选/多选并流式下载 ZIP。
3. **任务中心**：查看运行中和历史任务，失败/中断任务可重新执行。
4. **作品库**：查询 SQLite 中已发现作品，按关键词、作者和类型筛选并重新下载。

## 批量模式

### 作者发布

粘贴 `https://www.douyin.com/user/<sec_uid>`，选择 `post`。

### 作者点赞

使用相同作者主页链接，选择 `like`。点赞列表是否可访问取决于账号可见性以及当前 Cookie 权限。

### 合集

支持：

```text
https://www.douyin.com/collection/<mix_id>
https://www.douyin.com/mix/<mix_id>
```

页面自动使用 `mix` 模式。

## 真正批量下载

扫描完成后作品默认全部选中，也可以取消或重新选择。

为了支持一次选择数百个作品，前端不会把全部 `aweme_id` 塞进 GET URL，而是使用两步流程：

```text
POST /api/v1/batch/prepare
  body: job_id + ids[]
        ↓
返回 10 分钟有效的一次性 ticket
        ↓
GET /api/v1/batch/stream?ticket=...
        ↓
Go 服务逐个从 Douyin 拉取媒体
        ↓
边拉取边写 ZIP 响应
```

这样不会撞浏览器/Nginx 请求行长度限制，也不会让浏览器先把整个 ZIP 缓存在内存中。

ZIP 内容：

- 视频：直接写入 `.mp4`
- 图集：以作品子目录写入原图
- 单项失败：写入 `errors/<aweme_id>.txt`，其余作品继续

响应设置 `X-Accel-Buffering: no`，减少反向代理对大 ZIP 的缓冲。媒体不会长期保存到服务器磁盘。

## API

均位于 `/api/v1`，沿用现有 token 鉴权。

| 方法 | 路径 | 作用 |
|---|---|---|
| POST | `/jobs` | 创建批量扫描任务 |
| GET | `/jobs` | 查看运行中 + SQLite 历史任务 |
| GET | `/jobs/{id}` | 查看任务状态与当前进程中的作品结果 |
| POST | `/jobs/{id}/retry` | 使用原参数重新执行 |
| POST | `/batch/prepare` | 验证选中作品并创建短期下载 ticket |
| GET | `/batch/stream?ticket=...` | 消费一次性 ticket，流式返回 ZIP |
| GET | `/history` | 查询作品库 |
| GET | `/cookies/status` | Cookie 状态 |
| POST | `/cookies/import` | 导入 Cookie |

创建任务：

```json
{
  "url": "https://www.douyin.com/user/MS4wLjAB...",
  "mode": "post",
  "max_items": 50,
  "incremental": true
}
```

任务状态：`queued -> running -> completed / failed`。

如果服务在任务运行时重启，SQLite 中原来的 `queued/running` 记录会在任务中心显示为 `interrupted`，可重新执行。

## SQLite 去重与任务持久化

扫描作品写入 `aweme` 表，但不会伪造 `file_path`：

- `HasAweme()` 表示以前是否发现过；
- `IsDownloaded()` 仍只表示存在真实下载路径；
- `post` 增量使用 `author_sec_uid + create_time` 快速停止，同时用 `aweme_id` 精确去重；
- `like/mix` 主要按 `aweme_id` 去重；
- `job.overrides` 保存 `mode / incremental / max_items`。

## 作品库

`GET /history` 支持：

```text
q       标题 / 作者 / aweme_id 模糊搜索
author  作者模糊筛选
type    video / images
limit   1-500
offset  偏移量
```

## 重试与浏览器兜底

正常 API 继续使用 1s / 2s / 5s 指数退避。发布作品首屏 API 连续失败时，可选 Playwright 兜底；点赞和合集目前主要依赖 Web API。

安装：

```bash
cd tools
npm install
npx playwright install chromium
```

`config.yml`：

```yaml
browser_fallback:
  enabled: true
  headless: true
```

## Cookie

Web 可直接导入 Cookie。也可以：

```bash
node tools/cookie-login.mjs .cookies.json
```

不要把 `.cookies.json` 提交到 Git。

## CI

`.github/workflows/ci.yml` 在 PR 自动执行：

```text
Go:   go test ./... + go vet ./... + go build ./...
Vue:  npm ci + npm run build
```

## 部署

仍使用：

```bash
bash start.sh
```

原 `/health`、`/login`、`/resolve`、`/stream` 保持兼容。
