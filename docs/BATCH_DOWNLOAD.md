# Web 批量下载、任务中心与作品库

本分支在原有「单视频 / 图集解析 + 流式下载」能力之上增加完整的 Web 批量工作流：作者发布/点赞扫描、合集扫描、SQLite 增量去重、真正的 ZIP 批量下载、持久化任务中心、作品库筛选、浏览器兜底和 Cookie 管理。

## Web 页面

登录后页面分为四个工作区：

1. **链接下载**：保留原来的单视频 / 图集下载。
2. **批量下载**：扫描作者发布、点赞或合集，支持全选/多选并流式下载 ZIP。
3. **任务中心**：查看运行中和历史任务，失败/中断任务可重新执行。
4. **作品库**：查询 SQLite 中已发现作品，按关键词、作者和类型筛选并重新下载。

## 批量下载

### 作者发布

粘贴：

```text
https://www.douyin.com/user/<sec_uid>
```

内容类型选择「发布作品」。

### 作者点赞

使用相同作者主页链接，内容类型选择「点赞作品」。点赞列表是否可访问取决于目标账号可见性以及当前 Cookie 权限。

### 合集

粘贴：

```text
https://www.douyin.com/collection/<mix_id>
https://www.douyin.com/mix/<mix_id>
```

页面会自动使用合集模式。

### 真正批量下载

扫描完成后作品默认全部选中，也可以取消或重新选择。点击「下载选中 ZIP」后：

```text
浏览器
  ↓ GET /api/v1/batch/stream
Go 服务
  ↓ 逐个解析作品
Douyin CDN
  ↓ 边下载边写 ZIP
浏览器下载
```

视频直接写入 ZIP；图集以子目录形式写入原图。整个过程不会把媒体长期保存到服务器磁盘。单项失败时 ZIP 内会增加 `errors/<aweme_id>.txt`，其他作品继续处理。

## 新增 API

均位于 `/api/v1`，沿用现有 token 鉴权。

| 方法 | 路径 | 作用 |
|---|---|---|
| POST | `/jobs` | 创建批量扫描任务 |
| GET | `/jobs` | 查看运行中 + SQLite 历史任务 |
| GET | `/jobs/{id}` | 查看任务状态与当前进程中的作品结果 |
| POST | `/jobs/{id}/retry` | 使用原 URL / mode / maxItems / incremental 重新执行 |
| GET | `/batch/stream?job_id=...&ids=...` | 把选中作品流式打包 ZIP |
| GET | `/history` | 查询作品库，支持筛选 |
| GET | `/cookies/status` | 查看 Cookie 状态 |
| POST | `/cookies/import` | 导入浏览器 Cookie 请求头 |

创建作者发布任务：

```json
{
  "url": "https://www.douyin.com/user/MS4wLjAB...",
  "mode": "post",
  "max_items": 50,
  "incremental": true
}
```

作者点赞：

```json
{
  "url": "https://www.douyin.com/user/MS4wLjAB...",
  "mode": "like",
  "max_items": 50,
  "incremental": true
}
```

合集：

```json
{
  "url": "https://www.douyin.com/collection/1234567890",
  "mode": "mix",
  "max_items": 100,
  "incremental": true
}
```

任务状态：

```text
queued -> running -> completed / failed
```

如果服务在任务运行时重启，SQLite 中原来的 `queued/running` 记录会在任务中心显示为 `interrupted`，可直接重新执行。

## SQLite 去重与任务持久化

扫描到的作品元数据写入现有 `aweme` 表，但不会伪造 `file_path`：

- `HasAweme()`：判断作品是否已经发现过；
- `IsDownloaded()`：仍只把存在真实 `file_path` 的记录视为已下载；
- 发布作品增量模式使用 `author_sec_uid + create_time` 快速停止，同时用 `aweme_id` 精确去重；
- 点赞/合集增量主要使用 `aweme_id` 去重，因为作品作者和时间线并不等同于被扫描账号；
- `job` 表持久化 `mode / incremental / max_items`，服务重启后任务中心仍可展示并重新执行。

## 作品库筛选

`GET /api/v1/history` 支持：

```text
q       标题 / 作者 / aweme_id 模糊搜索
author  作者模糊筛选
type    video / images
limit   1-500
offset  偏移量
```

Web「作品库」提供对应搜索框、作者筛选、类型筛选和重新下载按钮。

## API 重试与浏览器兜底

正常链路继续复用 `core.DouyinAPIClient.RequestJSON()` 的指数退避：

```text
1s -> 2s -> 5s
```

发布作品主页 API 在首屏连续失败时，可选尝试 Playwright 浏览器兜底。点赞和合集目前主要依赖 Web API，这与参考项目当前的能力边界一致。

安装可选浏览器依赖：

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

### Web 导入

在「批量下载」页面右上角点击 Cookie，粘贴已登录浏览器请求里的完整 Cookie 请求头。后端解析后写入 `.cookies.json`，原始 Cookie 字符串不会返回给前端。

### 浏览器辅助登录

```bash
cd tools
npm install
npx playwright install chromium
cd ..
node tools/cookie-login.mjs .cookies.json
```

登录完成后脚本自动提取 Cookie 并保存。不要把 `.cookies.json` 提交到 Git。

## CI

本分支新增 `.github/workflows/ci.yml`。每个 PR 自动执行：

```text
Go backend:
  go test ./...
  go vet ./...
  go build ./...

Vue frontend:
  npm ci
  npm run build
```

## 部署

仍使用原来的：

```bash
bash start.sh
```

原 `/health`、`/login`、`/resolve`、`/stream` 行为保持不变，新功能作为增强层挂载。
