package io.github.nick.dydl

import android.app.Application
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import io.github.nick.dydl.api.ApiClient
import io.github.nick.dydl.api.AuthExpiredException
import io.github.nick.dydl.api.ResolveInfo
import io.github.nick.dydl.download.MediaStoreDownloader
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

/** 服务器健康状态(对应网页版 header 的小圆点) */
enum class Health { UNKNOWN, ONLINE, OFFLINE }

/** 解析成功后弹出的确认弹窗类型 */
sealed interface DialogState {
    data object None : DialogState
    /** 视频下载确认 */
    data class VideoConfirm(val info: ResolveInfo) : DialogState
    /** 单图下载确认 */
    data class SingleImageConfirm(val info: ResolveInfo) : DialogState
    /** 多图图集:选择"下载图片 ZIP"还是"合成视频 MP4" */
    data class GalleryChoice(val info: ResolveInfo) : DialogState
}

/**
 * 全局状态:登录态 / 健康检查 / 解析 / 下载。
 * 状态流映射自 web/src/App.vue + SubmitCard.vue。
 */
class AppViewModel(app: Application) : AndroidViewModel(app) {

    private val downloader = MediaStoreDownloader(app)

    val loggedIn = MutableStateFlow(ApiClient.token.isNotEmpty())
    val health = MutableStateFlow(Health.UNKNOWN)

    // 登录页
    val loginBusy = MutableStateFlow(false)

    // 下载页
    val busy = MutableStateFlow(false)
    /** 原生下载进度(0-100),-1 表示当前不在下载中 */
    val progress = MutableStateFlow(-1)
    /** busy 时标记正在下载哪种 mode,用于按钮 loading 态 */
    val busyMode = MutableStateFlow("")
    val dialog = MutableStateFlow<DialogState>(DialogState.None)

    /** 一次性提示(SnackBar,对应 ElMessage) */
    private val _message = MutableStateFlow<String?>(null)
    val message: StateFlow<String?> = _message.asStateFlow()

    private var healthJob: Job? = null

    init {
        ApiClient.init(app)
        startHealthPolling()
    }

    /** 5 秒一次健康检查(对应 App.vue 的 setInterval) */
    private fun startHealthPolling() {
        healthJob?.cancel()
        healthJob = viewModelScope.launch {
            while (true) {
                try {
                    ApiClient.health()
                    health.value = Health.ONLINE
                } catch (_: Exception) {
                    health.value = Health.OFFLINE
                }
                delay(5000)
            }
        }
    }

    fun consumeMessage() {
        _message.value = null
    }

    private fun notify(msg: String) {
        _message.value = msg
    }

    fun login(username: String, password: String) {
        if (username.isBlank() || password.isBlank()) {
            notify("请输入用户名和密码")
            return
        }
        viewModelScope.launch {
            loginBusy.value = true
            try {
                ApiClient.login(username.trim(), password)
                loggedIn.value = true
            } catch (e: AuthExpiredException) {
                notify(e.message ?: "登录失败")
            } catch (e: Exception) {
                notify(e.message ?: "登录失败")
            } finally {
                loginBusy.value = false
            }
        }
    }

    fun logout() {
        ApiClient.clearToken()
        loggedIn.value = false
    }

    /** 下载入口:先解析预览,再按类型弹确认框(对应 SubmitCard.handleDownload) */
    fun submit(url: String) {
        if (url.isEmpty()) {
            notify("没有识别到抖音链接,请粘贴视频/图集链接或分享文案")
            return
        }
        pendingUrl = url
        viewModelScope.launch {
            busy.value = true
            try {
                val info = try {
                    ApiClient.resolve(url)
                } catch (e: AuthExpiredException) {
                    onAuthExpired()
                    notify(e.message ?: "解析失败")
                    return@launch
                } catch (e: Exception) {
                    notify(e.message ?: "解析失败")
                    return@launch
                }
                dialog.value = when {
                    info.type == "images" && info.image_count > 1 -> DialogState.GalleryChoice(info)
                    info.type == "images" -> DialogState.SingleImageConfirm(info)
                    else -> DialogState.VideoConfirm(info)
                }
            } finally {
                busy.value = false
            }
        }
    }

    private fun onAuthExpired() {
        loggedIn.value = false
        dialog.value = DialogState.None
    }

    fun dismissDialog() {
        if (busy.value) return
        dialog.value = DialogState.None
    }

    /** 用户在弹窗里点了确认/选择下载方式 */
    fun confirmDownload(mode: String?) {
        val state = dialog.value
        val info = when (state) {
            is DialogState.VideoConfirm -> state.info
            is DialogState.SingleImageConfirm -> state.info
            is DialogState.GalleryChoice -> state.info
            DialogState.None -> return
        }
        when (state) {
            is DialogState.VideoConfirm ->
                runDownload(info, mode = null, filename = info.filename,
                    mime = "video/mp4", album = "movies",
                    successMsg = "已保存到相册:Movies/${MediaStoreDownloader.FOLDER}")
            is DialogState.SingleImageConfirm ->
                runDownload(info, mode = "images", filename = "${info.filename}.jpg",
                    mime = "image/jpeg", album = "pictures",
                    successMsg = "已保存到相册:Pictures/${MediaStoreDownloader.FOLDER}")
            is DialogState.GalleryChoice ->
                if (mode == "images") {
                    runDownload(info, mode = "images", filename = "${info.filename}.zip",
                        mime = "application/zip", album = "downloads",
                        successMsg = "已保存到:Download/${MediaStoreDownloader.FOLDER}")
                } else {
                    runDownload(info, mode = "video", filename = "${info.filename}.mp4",
                        mime = "video/mp4", album = "movies",
                        successMsg = "已保存到相册:Movies/${MediaStoreDownloader.FOLDER}")
                }
            DialogState.None -> {}
        }
    }

    private fun runDownload(
        info: ResolveInfo,
        mode: String?,
        filename: String,
        mime: String,
        album: String,
        successMsg: String,
    ) {
        viewModelScope.launch {
            busy.value = true
            busyMode.value = mode ?: "video"
            progress.value = 0
            try {
                val url = ApiClient.streamUrl(pendingUrl, mode)
                downloader.download(url, filename, mime, album).collect { p ->
                    progress.value = p
                }
                notify(successMsg)
                dialog.value = DialogState.None
            } catch (e: AuthExpiredException) {
                onAuthExpired()
            } catch (e: Exception) {
                notify(e.message ?: "下载失败")
                // 图集弹窗失败保留,便于换另一种方式重试;其它弹窗关闭
                if (dialog.value !is DialogState.GalleryChoice) dialog.value = DialogState.None
            } finally {
                busy.value = false
                busyMode.value = ""
                progress.value = -1
            }
        }
    }

    // 下载需要原始抖音链接(stream 的 url 参数);解析和下载之间持有它
    private var pendingUrl: String = ""
}
