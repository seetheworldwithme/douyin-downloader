// 服务器基址:网页与后端同源部署,留空走相对 /api/v1。
const SERVER_KEY = 'dd_server'

export function getServerBase() {
  const saved = (localStorage.getItem(SERVER_KEY) || '').trim().replace(/\/+$/, '')
  if (saved) return saved
  return ''
}

function apiBase() {
  return getServerBase() + '/api/v1'
}

const TOKEN_KEY = 'dd_token'
const AUTH_EXPIRED_EVENT = 'dd-auth-expired'

export function getToken() {
  return localStorage.getItem(TOKEN_KEY) || ''
}

export function setToken(token) {
  localStorage.setItem(TOKEN_KEY, token)
}

export function clearToken() {
  localStorage.removeItem(TOKEN_KEY)
}

async function request(path, options = {}) {
  const headers = { 'Content-Type': 'application/json', ...(options.headers || {}) }
  const token = getToken()
  if (token) headers['Authorization'] = `Bearer ${token}`
  const resp = await fetch(`${apiBase()}${path}`, { ...options, headers })
  if (resp.status === 401) {
    clearToken()
    window.dispatchEvent(new CustomEvent(AUTH_EXPIRED_EVENT))
    throw new Error('登录已过期,请重新登录')
  }
  if (!resp.ok) {
    let detail = `${resp.status} ${resp.statusText}`
    try {
      const body = await resp.json()
      if (body && body.detail) detail = body.detail
    } catch (_e) {
      /* 非 JSON 错误体 */
    }
    throw new Error(detail)
  }
  const text = await resp.text()
  return text ? JSON.parse(text) : null
}

export async function login(username, password) {
  const data = await request('/login', {
    method: 'POST',
    body: JSON.stringify({ username, password }),
  })
  if (data && data.token) setToken(data.token)
  return data
}

export function getHealth() {
  return request('/health')
}

export function resolveVideo(url) {
  return request('/resolve', {
    method: 'POST',
    body: JSON.stringify({ url }),
  })
}

export function streamUrl(url, mode) {
  const params = { url, token: getToken() }
  if (mode) params.mode = mode
  return `${getServerBase()}/api/v1/stream?${new URLSearchParams(params).toString()}`
}

// 创建用户主页批量扫描任务。后端异步分页读取作品并写入 SQLite。
export function createBatchJob(url, maxItems = 50, incremental = true) {
  return request('/jobs', {
    method: 'POST',
    body: JSON.stringify({ url, max_items: maxItems, incremental }),
  })
}

export function getBatchJob(jobId) {
  return request(`/jobs/${encodeURIComponent(jobId)}`)
}

export function listBatchJobs() {
  return request('/jobs')
}

export function getHistory(limit = 100) {
  return request(`/history?limit=${encodeURIComponent(limit)}`)
}

export function getCookieStatus() {
  return request('/cookies/status')
}

export function importCookies(cookie) {
  return request('/cookies/import', {
    method: 'POST',
    body: JSON.stringify({ cookie }),
  })
}
