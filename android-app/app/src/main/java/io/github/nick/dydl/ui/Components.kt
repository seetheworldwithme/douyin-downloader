package io.github.nick.dydl.ui

import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.MusicNote
import androidx.compose.material3.Icon
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.material3.Text
import androidx.compose.material3.TextFieldColors
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/** 玻璃质感卡片:白 4% 填充 + 白 9% 描边 + 大圆角 */
@Composable
fun GlassCard(
    modifier: Modifier = Modifier,
    content: @Composable ColumnScope.() -> Unit,
) {
    Column(
        modifier = modifier
            .clip(RoundedCornerShape(24.dp))
            .background(CardFill)
            .border(1.dp, CardStroke, RoundedCornerShape(24.dp))
            .padding(24.dp),
        content = content,
    )
}

/**
 * 主 CTA 按钮:抖音红→青 斜向渐变,近黑粗体文字保证两端对比度。
 * [busy] 时显示小型加载圈并禁用点击。
 */
@Composable
fun GradientButton(
    text: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    busy: Boolean = false,
) {
    val shape = RoundedCornerShape(16.dp)
    Box(
        modifier = modifier
            .clip(shape)
            .background(CtaGradient)
            .clickable(enabled = enabled && !busy, onClick = onClick)
            .heightIn(min = 52.dp)
            .padding(horizontal = 28.dp, vertical = 14.dp),
        contentAlignment = Alignment.Center,
    ) {
        if (busy) {
            androidx.compose.material3.CircularProgressIndicator(
                modifier = Modifier.size(20.dp),
                color = InkBlack,
                strokeWidth = 2.5.dp,
            )
        } else {
            Text(
                text,
                color = InkBlack,
                fontSize = 16.sp,
                fontWeight = FontWeight.Bold,
            )
        }
    }
}

/** TikTok 风格「故障」logo:青/红错位 + 白色主体 */
@Composable
fun TikTokLogo(modifier: Modifier = Modifier) {
    Box(modifier) {
        Icon(
            Icons.Rounded.MusicNote, null,
            tint = DouyinCyan,
            modifier = Modifier.offset(x = (-1.5).dp).fillMaxSize(),
        )
        Icon(
            Icons.Rounded.MusicNote, null,
            tint = DouyinRed,
            modifier = Modifier.offset(x = 1.5.dp).fillMaxSize(),
        )
        Icon(
            Icons.Rounded.MusicNote, null,
            tint = TextPrimary,
            modifier = Modifier.fillMaxSize(),
        )
    }
}

/** 健康状态胶囊:圆点 + 文字,底色为白 6% 的胶囊 */
@Composable
fun StatusPill(color: Color, label: String, modifier: Modifier = Modifier) {
    val dot by animateColorAsState(color, animationSpec = tween(300), label = "dot")
    Row(
        modifier = modifier
            .clip(RoundedCornerShape(50))
            .background(FillFaint)
            .padding(horizontal = 10.dp, vertical = 5.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box(
            Modifier
                .size(7.dp)
                .clip(CircleShape)
                .background(dot),
        )
        Spacer(Modifier.width(6.dp))
        Text(label, fontSize = 12.sp, color = TextSecondary)
    }
}

/** 暗色输入框配色:透明感容器 + 青色焦点描边/光标 */
@Composable
fun darkTextFieldColors(): TextFieldColors = OutlinedTextFieldDefaults.colors(
    focusedTextColor = TextPrimary,
    unfocusedTextColor = TextPrimary,
    focusedBorderColor = DouyinCyan,
    unfocusedBorderColor = Color(0x2EFFFFFF),
    focusedContainerColor = Color(0x12FFFFFF),
    unfocusedContainerColor = Color(0x12FFFFFF),
    cursorColor = DouyinCyan,
    focusedLabelColor = DouyinCyan,
    unfocusedLabelColor = TextMuted,
    focusedPlaceholderColor = TextMuted,
    unfocusedPlaceholderColor = TextMuted,
    focusedTrailingIconColor = DouyinCyan,
    unfocusedTrailingIconColor = TextMuted,
)
