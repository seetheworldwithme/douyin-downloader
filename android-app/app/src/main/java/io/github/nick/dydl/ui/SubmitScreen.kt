package io.github.nick.dydl.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import io.github.nick.dydl.AppViewModel
import io.github.nick.dydl.DialogState
import io.github.nick.dydl.util.UrlExtractor

/** 下载卡片 —— 对应 web/src/components/SubmitCard.vue */
@Composable
fun SubmitScreen(vm: AppViewModel) {
    var raw by remember { mutableStateOf("") }
    val busy by vm.busy.collectAsState()
    val progress by vm.progress.collectAsState()
    val busyMode by vm.busyMode.collectAsState()
    val dialog by vm.dialog.collectAsState()

    // 实时提取链接(对应 computed url)
    val url = UrlExtractor.extract(raw.trim())

    Card(modifier = Modifier.fillMaxWidth()) {
        Column(Modifier.padding(20.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Text("下载视频 / 图集", style = MaterialTheme.typography.titleMedium)
                Spacer(Modifier.weight(1f))
                Text("保存到相册", fontSize = 12.sp, color = HintGray)
            }

            OutlinedTextField(
                value = raw,
                onValueChange = { raw = it },
                label = {
                    Text("粘贴抖音视频/图集链接,或整段分享文案\n(例如:1.53 复制打开抖音……https://v.douyin.com/xxxxx/)")
                },
                minLines = 3,
                modifier = Modifier.fillMaxWidth(),
            )

            if (url.isNotEmpty()) {
                Row {
                    Text("识别到链接: ", fontSize = 13.sp, color = HintGray)
                    Text(
                        url,
                        fontSize = 13.sp,
                        fontFamily = FontFamily.Monospace,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
            }

            if (progress >= 0) {
                Column {
                    Text("正在保存…$progress%", fontSize = 13.sp, color = HintGray)
                    LinearProgressIndicator(
                        progress = { progress / 100f },
                        modifier = Modifier.fillMaxWidth(),
                    )
                }
            }

            Button(
                onClick = { vm.submit(url) },
                enabled = url.isNotEmpty() && !busy,
                modifier = Modifier
                    .align(androidx.compose.ui.Alignment.End)
                    .heightIn(min = 48.dp),
            ) {
                if (busy && progress < 0) {
                    CircularProgressIndicator(
                        modifier = Modifier.heightIn(max = 20.dp),
                        color = MaterialTheme.colorScheme.onPrimary,
                        strokeWidth = 2.dp,
                    )
                } else {
                    Text(if (progress >= 0) "下载中…" else "下载")
                }
            }
        }
    }

    when (val d = dialog) {
        is DialogState.VideoConfirm -> {
            ConfirmDialog(
                title = "确认下载该视频?",
                lines = listOf("标题:${d.info.title}", "文件名:${d.info.filename}"),
                confirmText = "下载",
                busy = busy,
                onConfirm = { vm.confirmDownload(null) },
                onDismiss = { vm.dismissDialog() },
            )
        }
        is DialogState.SingleImageConfirm -> {
            ConfirmDialog(
                title = "确认下载该图片?",
                lines = listOf("标题:${d.info.title}", "将下载 1 张图片"),
                confirmText = "下载",
                busy = busy,
                onConfirm = { vm.confirmDownload(null) },
                onDismiss = { vm.dismissDialog() },
            )
        }
        is DialogState.GalleryChoice -> {
            AlertDialog(
                onDismissRequest = { vm.dismissDialog() },
                title = { Text("图集下载方式") },
                text = {
                    Column {
                        Text("标题:${d.info.title}")
                        Text("图片: 共 ${d.info.image_count} 张" + if (d.info.has_music) "(含原声音乐)" else "")
                        Spacer(Modifier.height(8.dp))
                        Text(
                            "「合成视频」会把图片按原声时长合成为 MP4,服务器需要十几秒到几分钟处理,请耐心等待;\n「下载图片」立即打包下载 ZIP 原图。",
                            fontSize = 12.sp,
                            color = HintGray,
                            lineHeight = 18.sp,
                        )
                    }
                },
                confirmButton = {
                    Row {
                        OutlinedButton(
                            onClick = { vm.confirmDownload("images") },
                            enabled = !busy,
                            modifier = Modifier.heightIn(min = 48.dp),
                        ) {
                            if (busy && busyMode == "images") {
                                CircularProgressIndicator(
                                    modifier = Modifier.heightIn(max = 20.dp),
                                    strokeWidth = 2.dp,
                                )
                            } else Text("下载图片")
                        }
                        Spacer(Modifier.width(8.dp))
                        Button(
                            onClick = { vm.confirmDownload("video") },
                            enabled = !busy,
                            modifier = Modifier.heightIn(min = 48.dp),
                        ) {
                            if (busy && busyMode == "video") {
                                CircularProgressIndicator(
                                    modifier = Modifier.heightIn(max = 20.dp),
                                    color = MaterialTheme.colorScheme.onPrimary,
                                    strokeWidth = 2.dp,
                                )
                            } else Text("合成视频下载")
                        }
                    }
                },
                dismissButton = {
                    TextButton(
                        onClick = { vm.dismissDialog() },
                        enabled = !busy,
                    ) { Text("取消") }
                },
            )
        }
        DialogState.None -> {}
    }
}

/** 视频/单图确认框(对应 ElMessageBox.confirm) */
@Composable
private fun ConfirmDialog(
    title: String,
    lines: List<String>,
    confirmText: String,
    busy: Boolean,
    onConfirm: () -> Unit,
    onDismiss: () -> Unit,
) {
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(title) },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                lines.forEach { Text(it, fontSize = 14.sp) }
            }
        },
        confirmButton = {
            Button(onClick = onConfirm, enabled = !busy, modifier = Modifier.heightIn(min = 48.dp)) {
                Text(confirmText)
            }
        },
        dismissButton = {
            TextButton(onClick = onDismiss, enabled = !busy) { Text("取消") }
        },
    )
}
