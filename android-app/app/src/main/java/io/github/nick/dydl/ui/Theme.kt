package io.github.nick.dydl.ui

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color

// 配色对齐网页版 Element Plus 色系(App.vue / SubmitCard.vue)
private val Primary = Color(0xFF409EFF)      // el-button primary
private val Success = Color(0xFF67C23A)      // 健康点在线
private val Error = Color(0xFFF56C6C)        // 健康点离线 / 错误
private val PageBg = Color(0xFFF5F7FA)       // 页面背景
private val TextPrimary = Color(0xFF303133)  // 主文字
private val TextSecondary = Color(0xFF606266)// 次要文字
private val TextHint = Color(0xFF909399)     // 提示文字
private val Border = Color(0xFFEBEEF5)       // 分隔线

private val LightColors = lightColorScheme(
    primary = Primary,
    onPrimary = Color.White,
    background = PageBg,
    onBackground = TextPrimary,
    surface = Color.White,
    onSurface = TextPrimary,
    surfaceVariant = Color.White,
    onSurfaceVariant = TextSecondary,
    outline = Border,
    error = Error,
)

private val DarkColors = darkColorScheme(
    primary = Primary,
    error = Error,
)

@Composable
fun DouyinTheme(content: @Composable () -> Unit) {
    // 网页版只有亮色;深色模式仅做基本适配,避免夜间刺眼
    val colors = if (isSystemInDarkTheme()) DarkColors else LightColors
    MaterialTheme(colorScheme = colors, content = content)
}

// 暴露给页面的语义色(健康点等)
val HealthOnline = Success
val HealthOffline = Error
val HintGray = TextHint
