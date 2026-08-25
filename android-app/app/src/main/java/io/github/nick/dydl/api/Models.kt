package io.github.nick.dydl.api

import kotlinx.serialization.Serializable

/** POST /login 响应 */
@Serializable
data class LoginResp(val token: String)

/** POST /resolve 响应 —— 对应 web/src/api.js 的 resolveVideo 注释 */
@Serializable
data class ResolveInfo(
    val title: String,
    val filename: String,
    val aweme_id: String = "",
    /** "video" | "images" */
    val type: String = "video",
    val image_count: Int = 0,
    val has_music: Boolean = false,
)

/** FastAPI 风格错误体 {"detail": "..."} */
@Serializable
data class ErrorBody(val detail: String? = null)

/** 业务异常:message 直接展示给用户 */
class ApiException(message: String) : Exception(message)

/** 401 —— 登录过期,ViewModel 捕获后退回登录页(对应 api.js 的 dd-auth-expired 事件) */
class AuthExpiredException : Exception("登录已过期,请重新登录")
