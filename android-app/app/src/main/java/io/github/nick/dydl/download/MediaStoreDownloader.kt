package io.github.nick.dydl.download

import android.content.ContentValues
import android.content.Context
import android.net.Uri
import android.os.Build
import android.provider.MediaStore
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.flow
import kotlinx.coroutines.flow.flowOn
import okhttp3.OkHttpClient
import okhttp3.Request
import java.io.OutputStream
import java.util.concurrent.TimeUnit

/**
 * 把服务端流式文件写入系统媒体库 —— 移植自 mobile/native/SaveToGalleryPlugin.java,
 * 去掉 Capacitor 插件壳,改为 suspend + Flow<Progress>(下载进度)。
 *
 * mime → MediaStore 分区:视频 → Movies(相册)、图片 → Pictures(相册)、
 * zip 等 → Download。
 */
class MediaStoreDownloader(private val context: Context) {

    /** 下载进度:有 Content-Length 时上报百分比,否则 indeterminate(界面走流动动画) */
    data class Progress(val percent: Int, val indeterminate: Boolean = false)

    companion object {
        const val FOLDER = "抖音下载器"
        private const val BUF_SIZE = 256 * 1024

        /** 保存目标:MediaStore collection + 相对目录 */
        data class Target(val collection: Uri, val relativePath: String)

        fun targetFor(album: String, volume: String = MediaStore.VOLUME_EXTERNAL_PRIMARY): Target =
            when (album) {
                "pictures" -> Target(
                    MediaStore.Images.Media.getContentUri(volume),
                    "Pictures/$FOLDER",
                )
                "downloads" -> Target(
                    MediaStore.Downloads.getContentUri(volume),
                    "Download/$FOLDER",
                )
                else -> Target(
                    MediaStore.Video.Media.getContentUri(volume),
                    "Movies/$FOLDER",
                )
            }

        /** 去掉文件名非法字符;没有扩展名时按 MIME 补一个(纯函数,单测覆盖) */
        fun sanitize(name: String, mime: String): String {
            var safe = name.replace(Regex("""[\\/:*?"<>|]"""), "_").trim()
            if (safe.isEmpty()) safe = "douyin"
            if (!safe.contains('.')) {
                var ext = ".bin"
                if (mime.startsWith("video/")) ext = if (mime.contains("webm")) ".webm" else ".mp4"
                else if (mime == "image/jpeg") ext = ".jpg"
                else if (mime.startsWith("image/")) ext = "." + mime.substring("image/".length)
                else if (mime == "application/zip") ext = ".zip"
                safe += ext
            }
            return safe
        }
    }

    /**
     * 拉取 url 并写入 MediaStore,emit 进度(有 Content-Length 为百分比,否则 indeterminate);
     * 失败时删除半成品记录后抛异常。
     *
     * 图集"合成视频"在服务端要跑 ffmpeg,首字节可能几十秒后才到,
     * 所以读超时给足(它是两次数据包之间的间隔,不是总时长)。
     */
    fun download(url: String, filename: String, mime: String, album: String): Flow<Progress> = flow {
        val client = OkHttpClient.Builder()
            .connectTimeout(30, TimeUnit.SECONDS)
            .readTimeout(300, TimeUnit.SECONDS)
            .build()

        val response = client.newCall(Request.Builder().url(url).build()).execute()
        response.use { resp ->
            if (!resp.isSuccessful) {
                var detail = "上游返回 ${resp.code}"
                val text = resp.body?.string()
                if (!text.isNullOrEmpty()) detail += ": " + text.take(400)
                throw IllegalStateException(detail)
            }
            val body = resp.body ?: throw IllegalStateException("空响应体")
            val total = body.contentLength()
            val contentType = if (mime.isBlank()) "video/mp4" else mime
            val target = targetFor(album)
            val safeName = sanitize(filename, contentType)

            val resolver = context.contentResolver
            val values = ContentValues().apply {
                put(MediaStore.MediaColumns.DISPLAY_NAME, safeName)
                put(MediaStore.MediaColumns.MIME_TYPE, contentType)
                put(MediaStore.MediaColumns.RELATIVE_PATH, target.relativePath)
                // API 29+ 起支持 IS_PENDING:写入期间对其它应用不可见,写完清零
                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
                    put(MediaStore.MediaColumns.IS_PENDING, 1)
                }
            }
            var uri: Uri? = null
            try {
                uri = resolver.insert(target.collection, values)
                    ?: throw IllegalStateException("无法创建媒体记录(存储卷可能不可用)")
                val out: OutputStream = resolver.openOutputStream(uri, "w")
                    ?: throw IllegalStateException("无法打开输出流")
                var read = 0L
                var pct = -1
                if (total <= 0) emit(Progress(0, indeterminate = true))
                out.use { sink ->
                    body.byteStream().use { input ->
                        val buf = ByteArray(BUF_SIZE)
                        while (true) {
                            val n = input.read(buf)
                            if (n == -1) break
                            sink.write(buf, 0, n)
                            read += n
                            if (total > 0) {
                                val p = (read * 100 / total).toInt()
                                if (p != pct) {
                                    pct = p
                                    emit(Progress(pct))
                                }
                            }
                        }
                        sink.flush()
                    }
                }

                // 写完:清除 IS_PENDING,媒体库立即可见
                val done = ContentValues()
                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
                    done.put(MediaStore.MediaColumns.IS_PENDING, 0)
                }
                resolver.update(uri, done, null, null)
                emit(Progress(100))
            } catch (e: Exception) {
                // 失败时删除半成品记录,避免媒体库残留损坏项
                uri?.let { runCatching { resolver.delete(it, null, null) } }
                throw e
            }
        }
    }.flowOn(Dispatchers.IO)
}
