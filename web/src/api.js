// 服务器基址:网页同源时留空(相对 /api/v1);原生 APK 默认直接用线上地址。
// 已内置默认值,无需用户配置。
import { Capacitor } from '@capacitor/core'

const DEFAULT_SERVER = 'https://douyin.xuziyue.work'
const SERVER_KEY = 'dd_server'

export function getServerBase() {
  const saved = (localStorage.getItem(SERVER_KEY) || '').trim().replace(/\/+$/, '')
  if (saved) return saved
  // APK 未配置时回退到线上默认;网页同源时留空
  return Capacitor.isNativePlatform() ? DEFAULT_SERVER : ''
}

// 统一走 apiBase:开发时由 Vite 代理 /api -> :8000,生产时与 FastAPI 同源,
// APK / 自托管时为绝对地址。
function apiBase() {
  return getServerBase() + '/api/v1'
}

const TOKEN_KEY = 'dd_token'
// token 过期时通知 App 退回登录页(自定义事件)
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
  // 204 或空体
  const text = await resp.text()
  return text ? JSON.parse(text) : null
}

// 登录 -> { token }(成功后写入 localStorage,后续请求自动携带)
export async function login(username, password) {
  const data = await request('/login', {
    method: 'POST',
    body: JSON.stringify({ username, password }),
  })
  if (data && data.token) setToken(data.token)
  return data
}

// 健康检查(公开) -> { status: 'ok' }
export function getHealth() {
  return request('/health')
}

// 解析视频/图集链接(预览) -> { title, filename, aweme_id, type, image_count, has_music }
export function resolveVideo(url) {
  return request('/resolve', {
    method: 'POST',
    body: JSON.stringify({ url }),
  })
}

// 构造流式下载 URL,供浏览器原生导航下载(<a> 点击触发另存为)或 APK 原生 OkHttp 拉取。
// 导航 GET / 原生请求无法携带 Authorization header,所以 token 走 query 参数。
// mode:图集链接专用 —— 'images' 下载图片(多图 ZIP)/ 'video' 合成为 MP4;视频链接无需传。
// 返回绝对地址(getServerBase 为空时退化为同源相对路径)。
export function streamUrl(url, mode) {
  const params = { url, token: getToken() }
  if (mode) params.mode = mode
  return `${getServerBase()}/api/v1/stream?${new URLSearchParams(params).toString()}`
}
