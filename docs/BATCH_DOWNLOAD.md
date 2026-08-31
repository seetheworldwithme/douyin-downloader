# Web 批量下载与任务中心

本分支在原有「单视频 / 图集解析 + 流式下载」能力之上增加作者主页批量扫描、SQLite 增量去重、异步任务中心、浏览器兜底和 Cookie 管理。

## Web 使用

登录 Web 后，原下载卡片下方会出现「主页批量下载」工作区：

1. 粘贴 `https://www.douyin.com/user/<sec_uid>` 作者主页链接。
2. 设置最多扫描条数（1–500）。
3. 默认开启「增量模式」。开启后，数据库中已经发现过的作品会跳过；再次扫描同一作者时主要返回新作品。
4. 点击「开始扫描」。前端创建异步任务并轮询任务状态，不会长时间占住一次 HTTP 请求。
5. 扫描结束后可直接对每条视频 / 图集复用现有 `/api/v1/stream` 接口下载，视频仍然是服务端中转流，不在服务器持久化视频文件。

## 新增 API

均位于 `/api/v1`，除登录外均沿用现有 token 鉴权。

| 方法 | 路径 | 作用 |
|---|---|---|
| POST | `/jobs` | 创建作者主页扫描任务 |
| GET | `/jobs` | 查看当前进程中的最近任务 |
| GET | `/jobs/{id}` | 查看任务状态与发现的作品 |
| GET | `/history?limit=100` | 查看 SQLite 中最近发现的作品 |
| GET | `/cookies/status` | 查看 Cookie 是否已配置/基础校验是否通过 |
| POST | `/cookies/import` | 导入浏览器 Cookie 请求头 |

创建任务示例：

```json
{
  "url": "https://www.douyin.com/user/MS4wLjAB...",
  "max_items": 50,
  "incremental": true
}
```

任务状态：`queued` → `running` → `completed` / `failed`。

## SQLite 去重与增量

扫描到的作品元数据会写入现有 `aweme` 表，但不会伪造 `file_path`。因此：

- `HasAweme()` 用于“是否已经发现过”的增量判断；
- 原有 `IsDownloaded()` 仍只把真正存在下载路径的记录视作“已下载”；
- 同一作者再次增量扫描时，会使用 `author_sec_uid + create_time` 作为快速停止基线，并同时用 `aweme_id` 做精确去重。

## API 重试

正常链路继续复用 `core.DouyinAPIClient.RequestJSON()` 的指数退避重试（1s / 2s / 5s）。只有主页 API 在首屏连续失败时，才尝试 Playwright 浏览器兜底。

## Playwright 浏览器兜底（可选）

Go 服务本身不新增浏览器依赖。只有需要兜底时才调用 `tools/browser-fallback.mjs`。

安装：

```bash
cd tools
npm install
npx playwright install chromium
```

确保 `config.yml` 中启用：

```yaml
browser_fallback:
  enabled: true
  headless: true
```

工作流程：

```text
Douyin Web API
    ↓ 失败/风控
Node + Playwright Chromium
    ↓
滚动作者主页，收集 video / note / gallery 链接
    ↓
写入同一任务与 SQLite 去重流程
```

如果 Node、Playwright 或 Chromium 未安装，服务不会启动失败；只是兜底不可用，并保留正常 API 链路的错误信息。

## 自动获取 Cookie

### 方式一：Web 导入

在「主页批量下载」卡片右上角点击 Cookie，粘贴已登录浏览器请求中的完整 `Cookie` 请求头。后端解析后保存到 `.cookies.json`（0600 权限）。

### 方式二：浏览器辅助登录

在项目根目录执行：

```bash
cd tools
npm install
npx playwright install chromium
cd ..
node tools/cookie-login.mjs .cookies.json
```

脚本会打开抖音网页。完成登录后，它检测到 `ttwid`、`odin_tt`、`passport_csrf_token` 等关键 Cookie 后自动写入 `.cookies.json` 并退出。

> 在远程无桌面服务器上，推荐先在本地执行登录脚本，再安全地把 `.cookies.json` 放到服务器配置目录；不要提交 Cookie 到 Git。

## 部署

仍然使用原来的：

```bash
bash start.sh
```

`cmd/server` 已切换为增强版 Server，原 `/health`、`/login`、`/resolve`、`/stream` 和 SPA 行为保持不变。
