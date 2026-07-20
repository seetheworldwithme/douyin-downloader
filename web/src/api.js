// 统一走相对路径:开发时由 Vite 代理 /api -> :8000,生产时与 FastAPI 同源
const BASE = '/api/v1'

async function request(path, options = {}) {
  const resp = await fetch(`${BASE}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  })
  if (!resp.ok) {
    let detail = `${resp.status} ${resp.statusText}`
    try {
      const body = await resp.json()
      if (body && body.detail) detail = body.detail
    } catch (_e) {
      /* 忽略非 JSON 错误体 */
    }
    throw new Error(detail)
  }
  // 204 或空体
  const text = await resp.text()
  return text ? JSON.parse(text) : null
}

// 提交单个下载任务 -> { job_id, status, url }
// content: { music, cover, avatar, json } 可选附件开关;视频 mp4 本体始终下载
export function submitDownload(url, saveDir, content) {
  const body = { url }
  if (saveDir && saveDir.trim()) body.save_dir = saveDir.trim()
  if (content && typeof content === 'object') body.content = content
  return request('/download', {
    method: 'POST',
    body: JSON.stringify(body),
  })
}

// 默认/当前保存目录 -> { default_path, current_path }
export function getDefaults() {
  return request('/defaults')
}

// 列出所有任务 -> { jobs: [...] }
export function listJobs() {
  return request('/jobs')
}

// 清空所有任务记录 -> { cleared: N }
export function clearJobs() {
  return request('/jobs', { method: 'DELETE' })
}

// 查询单个任务
export function getJob(jobId) {
  return request(`/jobs/${jobId}`)
}

// 健康检查 -> { status: 'ok' }
export function getHealth() {
  return request('/health')
}
