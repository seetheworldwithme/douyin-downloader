package io.github.nick.dydl

import android.content.ContentValues
import android.os.Build
import android.provider.MediaStore
import com.getcapacitor.JSObject
import com.getcapacitor.Plugin
import com.getcapacitor.PluginCall
import com.getcapacitor.PluginMethod
import com.getcapacitor.annotation.CapacitorPlugin
import okhttp3.OkHttpClient
import okhttp3.Request
import java.util.concurrent.TimeUnit

/**
 * SaveToGallery —— 把服务端流式 mp4 直接写入系统相册(Movies/抖音下载器)。
 *
 * 由前端 web/src/plugins/saveToGallery.js 通过 registerPlugin('SaveToGallery') 调用。
 * 进度经 notifyListeners("saveProgress", {percent}) 推回 JS。
 *
 * 放置位置:`cap add android` 后,复制到
 *   web/android/app/src/main/java/io/github/nick/dydl/SaveToGalleryPlugin.kt
 * Capacitor 会自动扫描带 @CapacitorPlugin 的类完成注册,无需改 MainActivity。
 *
 * 仅支持 API 29+(minSdk 29):走 MediaStore 分区存储,无需运行时存储权限。
 * 如需兼容 API ≤28,需追加 WRITE_EXTERNAL_STORAGE 权限 + 公共目录直写分支。
 */
@CapacitorPlugin(name = "SaveToGallery")
class SaveToGalleryPlugin : Plugin() {

    private val videoCollection
        get() = MediaStore.Video.Media.getContentUri(MediaStore.VOLUME_EXTERNAL_PRIMARY)

    @PluginMethod
    fun saveToGallery(call: PluginCall) {
        val url = call.getString("url")
        if (url.isNullOrBlank()) {
            call.reject("url is required")
            return
        }
        val filename = sanitize(call.getString("filename") ?: "video.mp4")

        // 网络 + 落盘在后台线程,避免阻塞 WebView。
        Thread {
            try {
                val client = OkHttpClient.Builder()
                    .connectTimeout(30, TimeUnit.SECONDS)
                    .readTimeout(60, TimeUnit.SECONDS)
                    .build()
                val response = client.newCall(Request.Builder().url(url).build()).execute()
                if (!response.isSuccessful) {
                    call.reject("上游返回 ${response.code}")
                    return@Thread
                }
                val body = response.body
                if (body == null) {
                    call.reject("空响应体")
                    return@Thread
                }
                val total = body.contentLength()

                val resolver = getContext().contentResolver
                val values = ContentValues().apply {
                    put(MediaStore.Video.Media.DISPLAY_NAME, filename)
                    put(MediaStore.Video.Media.MIME_TYPE, "video/mp4")
                    put(MediaStore.Video.Media.RELATIVE_PATH, "Movies/$FOLDER")
                    // API 29+ 起支持 IS_PENDING:写入期间对其它应用不可见,写完清零
                    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
                        put(MediaStore.Video.Media.IS_PENDING, 1)
                    }
                }
                val uri = resolver.insert(videoCollection, values)
                if (uri == null) {
                    call.reject("无法创建媒体记录(Movies 卷可能不可用)")
                    return@Thread
                }

                val out = resolver.openOutputStream(uri, "w")
                if (out == null) {
                    resolver.delete(uri, null, null)
                    call.reject("无法打开输出流")
                    return@Thread
                }
                try {
                    out.use { o ->
                        body.byteStream().use { input ->
                            val buf = ByteArray(256 * 1024)
                            var read = 0L
                            var pct = -1
                            while (true) {
                                val n = input.read(buf)
                                if (n == -1) break
                                o.write(buf, 0, n)
                                read += n
                                if (total > 0) {
                                    val p = (read * 100 / total).toInt()
                                    if (p != pct) {
                                        pct = p
                                        notifyProgress(pct)
                                    }
                                }
                            }
                            o.flush()
                        }
                    }
                    // 写完:清除 IS_PENDING,相册立即可见
                    val done = ContentValues().apply {
                        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
                            put(MediaStore.Video.Media.IS_PENDING, 0)
                        }
                    }
                    resolver.update(uri, done, null, null)
                    call.resolve(JSObject().put("uri", uri.toString()))
                } catch (e: Exception) {
                    // 失败时删除半成品记录,避免相册残留损坏项
                    try {
                        resolver.delete(uri, null, null)
                    } catch (_: Exception) {
                    }
                    call.reject(e.localizedMessage ?: "写入失败", e)
                }
            } catch (e: Exception) {
                call.reject(e.localizedMessage ?: "下载失败", e)
            }
        }.start()
    }

    private fun notifyProgress(percent: Int) {
        notifyListeners("saveProgress", JSObject().put("percent", percent))
    }

    /** 去掉文件名非法字符,保证以 .mp4 结尾。 */
    private fun sanitize(name: String): String {
        val safe = name.replace(Regex("[\\\\/:*?\"<>|]"), "_").trim().ifBlank { "video" }
        return if (safe.endsWith(".mp4", ignoreCase = true)) safe else "$safe.mp4"
    }

    companion object {
        private const val FOLDER = "抖音下载器"
    }
}
