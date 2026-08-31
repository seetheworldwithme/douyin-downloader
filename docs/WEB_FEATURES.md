# Web 功能矩阵

| 页面 | 前端组件 | 后端接口 | 状态 |
|---|---|---|---|
| 链接下载 | `SubmitCard.vue` | `/resolve`, `/stream` | 完成 |
| 批量下载 | `BatchCard.vue` | `/jobs`, `/jobs/{id}`, `/batch/stream` | 完成 |
| 任务中心 | `TaskCenter.vue` | `/jobs`, `/jobs/{id}/retry` | 完成 |
| 作品库 | `HistoryCard.vue` | `/history` | 完成 |
| Cookie 管理 | `BatchCard.vue` dialog | `/cookies/status`, `/cookies/import` | 完成 |

## 批量模式

- `post`: 作者发布作品，支持首屏 Playwright 兜底。
- `like`: 作者点赞作品，依赖账号可见性与 Cookie。
- `mix`: 直接扫描 `/collection/{mix_id}` 或 `/mix/{mix_id}`。

## 流式 ZIP

`/batch/stream` 不生成服务器持久媒体文件：视频直接写 ZIP，图集写入子目录，单项失败写 `errors/<aweme_id>.txt`，其余作品继续处理。
