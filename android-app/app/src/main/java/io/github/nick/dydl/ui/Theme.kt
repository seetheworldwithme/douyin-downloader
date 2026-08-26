package io.github.nick.dydl.ui

import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Typography
import androidx.compose.material3.darkColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.sp

// ── 抖音品牌色 ──────────────────────────────────────────────
val DouyinCyan = Color(0xFF25F4EE) // 抖音青
val DouyinRed = Color(0xFFFE2C55)  // 抖音红

// ── 暗色底色阶 ──────────────────────────────────────────────
val InkBlack = Color(0xFF15151C)      // 页面底色
val InkElevated = Color(0xFF1E1E28)   // 页面顶部渐变
val CardFill = Color(0x14FFFFFF)      // 玻璃卡片填充(白 8%)
val CardStroke = Color(0x29FFFFFF)    // 玻璃卡片描边(白 16%)
val TextPrimary = Color(0xFFF5F5F7)   // 主文字
val TextSecondary = Color(0xFFB6B6C2) // 次要文字
val TextMuted = Color(0xFF8A8A96)     // 提示文字

// CTA 渐变:抖音红 → 抖音青(斜向),文字用近黑保证两端对比度
val CtaGradient = Brush.linearGradient(listOf(DouyinRed, DouyinCyan))

// 半透明白色阶(chip 填充 / 进度轨道 / 幽灵按钮描边)
val FillFaint = Color(0x1FFFFFFF)   // 白 12%
val TrackFill = Color(0x2EFFFFFF)   // 白 18%
val StrokeFaint = Color(0x33FFFFFF) // 白 20%

// 进度条渐变:青 → 红
val ProgressGradient = Brush.linearGradient(listOf(DouyinCyan, DouyinRed))

private val DarkColors = darkColorScheme(
    primary = DouyinRed,
    onPrimary = Color.White,
    secondary = DouyinCyan,
    onSecondary = InkBlack,
    tertiary = DouyinCyan,
    background = InkBlack,
    onBackground = TextPrimary,
    surface = InkElevated,
    onSurface = TextPrimary,
    surfaceVariant = CardFill,
    onSurfaceVariant = TextSecondary,
    surfaceContainer = InkElevated,
    surfaceContainerHigh = Color(0xFF262632),
    surfaceContainerLow = InkBlack,
    outline = CardStroke,
    outlineVariant = CardStroke,
    error = Color(0xFFFF6B7E),
    onError = InkBlack,
    scrim = Color(0xCC000000),
)

// 标题略加重,正文保持默认节奏
private val AppTypography = Typography().let { base ->
    base.copy(
        titleLarge = base.titleLarge.copy(fontWeight = FontWeight.Bold, letterSpacing = 0.2.sp),
        titleMedium = base.titleMedium.copy(fontWeight = FontWeight.SemiBold),
        labelLarge = base.labelLarge.copy(fontWeight = FontWeight.SemiBold),
    )
}

/** 全局主题:抖音品牌暗色,不随系统亮色切换 */
@Composable
fun DouyinTheme(content: @Composable () -> Unit) {
    MaterialTheme(
        colorScheme = DarkColors,
        typography = AppTypography,
        content = content,
    )
}

// 暴露给页面的语义色(健康点等)
val HealthOnline = DouyinCyan
val HealthOffline = Color(0xFFFF6B7E)
val HintGray = TextMuted
