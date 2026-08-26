package io.github.nick.dydl.api

import android.content.Context
import android.content.SharedPreferences
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlinx.serialization.json.Json
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import java.util.concurrent.TimeUnit

/**
 * API 客户端 —— 从 web/src/api.js 移植。
 *
 * 服务器基址已内置(BuildConfig.SERVER_BASE),无需用户配置;
 * token 持久化在 SharedPreferences,与网页版 localStorage(dd_token)行为一致。
 */
object ApiClient {

    private const val PREFS_NAME = "dd_prefs"
    private const val TOKEN_KEY = "dd_token"

    private val json = Json { ignoreUnknownKeys = true; isLenient = true }
    private val JSON_MEDIA = "application/json; charset=utf-8".toMediaType()

    private lateinit var prefs: SharedPreferences

    /** 常规 JSON 接口用;下载流走 MediaStoreDownloader 自己的长超时客户端 */
    private val client = OkHttpClient.Builder()
        .connectTimeout(15, TimeUnit.SECONDS)
        .readTimeout(60, TimeUnit.SECONDS)
        .build()

    /** 服务器基址,内置默认(同 web/src/api.js 的 DEFAULT_SERVER) */
    val serverBase: String get() = io.github.nick.dydl.BuildConfig.SERVER_BASE

    var token: String
        get() = if (::prefs.isInitialized) prefs.getString(TOKEN_KEY, "") ?: "" else ""
        private set(value) {
            if (::prefs.isInitialized) prefs.edit().putString(TOKEN_KEY, value).apply()
        }

    fun init(context: Context) {
        if (!::prefs.isInitialized) {
            prefs = context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
        }
    }

    fun clearToken() {
        token = ""
    }

    private fun apiBase() = "$serverBase/api/v1"

    /** 统一请求:带 token、解析错误 detail、401 抛 AuthExpiredException */
    private suspend fun request(path: String, body: String? = null): String? =
        withContext(Dispatchers.IO) {
            val builder = Request.Builder().url(apiBase() + path)
            if (body != null) builder.post(body.toRequestBody(JSON_MEDIA)) else builder.get()
            val t = token
            if (t.isNotEmpty()) builder.header("Authorization", "Bearer $t")

            client.newCall(builder.build()).execute().use { resp ->
                if (resp.code == 401) {
                    clearToken()
                    throw AuthExpiredException()
                }
                val text = resp.body?.string() ?: ""
                if (!resp.isSuccessful) {
                    val detail = runCatching { json.decodeFromString(ErrorBody.serializer(), text).detail }
                        .getOrNull()
                    throw ApiException(detail ?: "HTTP ${resp.code}")
                }
                text.ifEmpty { null }
            }
        }

    /** 登录,成功后写入 token */
    suspend fun login(username: String, password: String) {
        val payload = kotlinx.serialization.json.buildJsonObject {
            put("username", kotlinx.serialization.json.JsonPrimitive(username))
            put("password", kotlinx.serialization.json.JsonPrimitive(password))
        }.toString()
        val resp = request("/login", payload)
        val token = runCatching { json.decodeFromString(LoginResp.serializer(), resp ?: "").token }
            .getOrNull()
        if (token.isNullOrEmpty()) throw ApiException("登录失败:响应缺少 token")
        this.token = token
    }

    /** 健康检查(公开),失败抛异常由调用方标记离线 */
    suspend fun health() {
        request("/health")
    }

    /** 解析视频/图集链接(预览) */
    suspend fun resolve(url: String): ResolveInfo {
        val payload = kotlinx.serialization.json.buildJsonObject {
            put("url", kotlinx.serialization.json.JsonPrimitive(url))
        }.toString()
        return json.decodeFromString(ResolveInfo.serializer(), request("/resolve", payload) ?: "")
    }

    /**
     * 构造流式下载 URL,供 MediaStoreDownloader 原生拉取。
     * GET 请求无法携带 Authorization header,所以 token 走 query 参数(同网页版)。
     * mode:图集专用 —— "images" 下载图片 / "video" 合成 MP4;视频不传。
     * index:mode=images 时可指定只取第 index 张原图(逐张保存进相册)。
     */
    fun streamUrl(url: String, mode: String? = null, index: Int? = null): String {
        val sb = StringBuilder("$serverBase/api/v1/stream?url=")
            .append(java.net.URLEncoder.encode(url, "UTF-8"))
            .append("&token=").append(java.net.URLEncoder.encode(token, "UTF-8"))
        if (mode != null) sb.append("&mode=").append(mode)
        if (index != null) sb.append("&index=").append(index)
        return sb.toString()
    }
}
