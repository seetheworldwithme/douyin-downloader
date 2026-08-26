package io.github.nick.dydl.ui

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.animation.expandVertically
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.shrinkVertically
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.Download
import androidx.compose.material.icons.rounded.Link
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
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
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
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
    val indeterminate by vm.indeterminate.collectAsState()
    val busyMode by vm.busyMode.collectAsState()
    val dialog by vm.dialog.collectAsState()

    // 实时提取链接(对应 computed url)
    val url = UrlExtractor.extract(raw.trim())

    GlassCard(Modifier.fillMaxWidth()) {
        Column(verticalArrangement = Arrangement.spacedBy(14.dp)) {
            // 卡片头:渐变底图标 + 标题 + 「保存到相册」胶囊
            Row(verticalAlignment = Alignment.CenterVertically) {
                Box(
                    Modifier
                        .size(40.dp)
                        .clip(RoundedCornerShape(12.dp))
                        .background(CtaGradient)
                        .padding(8.dp),
                    contentAlignment = Alignment.Center,
                ) {
                    Icon(Icons.Rounded.Download, null, tint = InkBlack, modifier = Modifier.size(22.dp))
                }
                Spacer(Modifier.size(12.dp))
                Text("下载视频 / 图集", style = MaterialTheme.typography.titleMedium)
                Spacer(Modifier.weight(1f))
                Text(
                    "保存到相册",
                    fontSize = 11.sp,
                    color = DouyinCyan,
                    modifier = Modifier
                        .clip(RoundedCornerShape(50))
                        .background(DouyinCyan.copy(alpha = 0.10f))
                        .padding(horizontal = 10.dp, vertical = 4.dp),
                )
            }

            OutlinedTextField(
                value = raw,
                onValueChange = { raw = it },
                placeholder = {
                    Text(
                        "粘贴抖音视频/图集链接,或整段分享文案\n(例如:1.53 复制打开抖音……https://v.douyin.com/xxxxx/)",
                    )
                },
                minLines = 3,
                shape = RoundedCornerShape(14.dp),
                colors = darkTextFieldColors(),
                modifier = Modifier.fillMaxWidth(),
            )

            // 识别到的链接 chip
            AnimatedVisibility(
                visible = url.isNotEmpty(),
                enter = fadeIn() + expandVertically(),
                exit = fadeOut() + shrinkVertically(),
            ) {
                Row(
                    Modifier
                        .clip(RoundedCornerShape(10.dp))
                        .background(FillFaint)
                        .padding(horizontal = 10.dp, vertical = 8.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Icon(
                        Icons.Rounded.Link, null,
                        tint = DouyinCyan,
                        modifier = Modifier.size(14.dp),
                    )
                    Spacer(Modifier.size(6.dp))
                    Text(
                        url,
                        fontSize = 12.sp,
                        fontFamily = FontFamily.Monospace,
                        color = TextSecondary,
                        maxLines = 1,
                    )
                }
            }

            // 下载进度
            AnimatedVisibility(visible = progress >= 0) {
                Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                    Text(
                        if (indeterminate) "正在保存…" else "正在保存…$progress%",
                        fontSize = 12.sp,
                        color = HintGray,
                    )
                    if (indeterminate) {
                        // 无 Content-Length(如合成视频):流动动画
                        IndeterminateBar(Modifier.fillMaxWidth())
                    } else {
                        // 自绘渐变进度条(该版本 LinearProgressIndicator 不支持 brush)
                        Box(
                            Modifier
                                .fillMaxWidth()
                                .height(6.dp)
                                .clip(RoundedCornerShape(3.dp))
                                .background(TrackFill),
                        ) {
                            Box(
                                Modifier
                                    .fillMaxWidth((progress / 100f).coerceIn(0.01f, 1f))
                                    .height(6.dp)
                                    .background(ProgressGradient),
                            )
                        }
                    }
                }
            }

            GradientButton(
                text = if (progress >= 0) "下载中…" else "开始下载",
                onClick = { vm.submit(url) },
                enabled = url.isNotEmpty() && !busy,
                busy = busy && progress < 0,
                modifier = Modifier.fillMaxWidth(),
            )
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
                        Spacer(Modifier.size(10.dp))
                        Text(
                            "「合成视频」会把图片按原声时长合成为 MP4,服务器需要十几秒到几分钟处理,请耐心等待;\n「下载图片」逐张下载原图,直接保存进相册。",
                            fontSize = 12.sp,
                            color = HintGray,
                            lineHeight = 18.sp,
                        )
                        Spacer(Modifier.size(12.dp))
                        Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                            GhostButton(
                                text = "下载图片",
                                busy = busy && busyMode == "images",
                                enabled = !busy,
                                onClick = { vm.confirmDownload("images") },
                            )
                            GradientButton(
                                text = "合成视频下载",
                                onClick = { vm.confirmDownload("video") },
                                enabled = !busy,
                                busy = busy && busyMode == "video",
                            )
                        }
                    }
                },
                confirmButton = {},
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

/** 不定进度条:渐变色块在轨道上来回扫动(服务器未返回总大小时使用) */
@Composable
private fun IndeterminateBar(modifier: Modifier = Modifier) {
    val transition = rememberInfiniteTransition(label = "indeterminate")
    val sweep by transition.animateFloat(
        initialValue = 0f,
        targetValue = 1f,
        animationSpec = infiniteRepeatable(tween(1100, easing = LinearEasing), RepeatMode.Restart),
        label = "sweep",
    )
    Canvas(
        modifier
            .height(6.dp)
            .clip(RoundedCornerShape(3.dp)),
    ) {
        val corner = CornerRadius(size.height / 2)
        drawRoundRect(color = TrackFill, cornerRadius = corner)
        val seg = size.width * 0.35f
        val left = -seg + sweep * (size.width + seg)
        drawRoundRect(
            brush = ProgressGradient,
            topLeft = Offset(left, 0f),
            size = Size(seg, size.height),
            cornerRadius = corner,
        )
    }
}

/** 次级按钮:白 12% 底 + 白 20% 描边的幽灵按钮 */
@Composable
private fun GhostButton(
    text: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    busy: Boolean = false,
) {
    val shape = RoundedCornerShape(16.dp)
    Box(
        modifier = modifier
            .fillMaxWidth()
            .clip(shape)
            .background(FillFaint)
            .border(1.dp, StrokeFaint, shape)
            .clickable(enabled = enabled && !busy, onClick = onClick)
            .heightIn(min = 52.dp),
        contentAlignment = Alignment.Center,
    ) {
        if (busy) {
            CircularProgressIndicator(
                modifier = Modifier.size(20.dp),
                color = DouyinCyan,
                strokeWidth = 2.5.dp,
            )
        } else {
            Text(text, color = TextPrimary, fontSize = 15.sp, fontWeight = FontWeight.SemiBold)
        }
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
        title = { Text(title, fontWeight = FontWeight.SemiBold) },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                lines.forEach { Text(it, fontSize = 14.sp, color = TextSecondary) }
            }
        },
        confirmButton = {
            Button(
                onClick = onConfirm,
                enabled = !busy,
                colors = ButtonDefaults.buttonColors(containerColor = DouyinRed),
                modifier = Modifier.heightIn(min = 48.dp),
            ) {
                Text(confirmText, fontWeight = FontWeight.SemiBold)
            }
        },
        dismissButton = {
            TextButton(onClick = onDismiss, enabled = !busy) { Text("取消") }
        },
    )
}
