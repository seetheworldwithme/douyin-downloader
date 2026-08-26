package io.github.nick.dydl

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.SystemBarStyle
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.lifecycle.viewmodel.compose.viewModel
import io.github.nick.dydl.ui.DouyinTheme
import io.github.nick.dydl.ui.HealthOffline
import io.github.nick.dydl.ui.HealthOnline
import io.github.nick.dydl.ui.InkBlack
import io.github.nick.dydl.ui.InkElevated
import io.github.nick.dydl.ui.LoginScreen
import io.github.nick.dydl.ui.StatusPill
import io.github.nick.dydl.ui.SubmitScreen
import io.github.nick.dydl.ui.TextSecondary
import io.github.nick.dydl.ui.TikTokLogo

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        // 全暗色设计:状态栏/导航栏透明 + 浅色图标关闭
        enableEdgeToEdge(
            statusBarStyle = SystemBarStyle.dark(android.graphics.Color.TRANSPARENT),
            navigationBarStyle = SystemBarStyle.dark(android.graphics.Color.TRANSPARENT),
        )
        setContent {
            DouyinTheme {
                DouyinApp()
            }
        }
    }
}

/** 根布局:渐变背景 + 透明顶栏 + 内容区 —— 对应 web/src/App.vue */
@Composable
fun DouyinApp(vm: AppViewModel = viewModel()) {
    val loggedIn by vm.loggedIn.collectAsState()
    val health by vm.health.collectAsState()
    val message by vm.message.collectAsState()
    val snackbar = remember { SnackbarHostState() }

    // 一次性提示消费后清空,避免重复弹出
    LaunchedEffect(message) {
        message?.let {
            snackbar.showSnackbar(it)
            vm.consumeMessage()
        }
    }

    Box(
        Modifier
            .fillMaxSize()
            .background(Brush.verticalGradient(listOf(InkElevated, InkBlack))),
    ) {
        Column(Modifier.fillMaxSize().statusBarsPadding()) {
            Header(
                health = health,
                loggedIn = loggedIn,
                onLogout = { vm.logout() },
            )
            Box(
                Modifier
                    .fillMaxWidth()
                    .weight(1f),
            ) {
                Column(
                    Modifier
                        .fillMaxSize()
                        .verticalScroll(rememberScrollState())
                        .padding(horizontal = 20.dp, vertical = 24.dp),
                    horizontalAlignment = Alignment.CenterHorizontally,
                ) {
                    // 卡片居中、限宽(对应 LoginCard max-width:380px)
                    Box(Modifier.widthIn(max = 420.dp).fillMaxWidth()) {
                        if (!loggedIn) {
                            LoginScreen(vm)
                        } else {
                            SubmitScreen(vm)
                        }
                    }
                }
                SnackbarHost(
                    snackbar,
                    Modifier
                        .align(Alignment.BottomCenter)
                        .padding(bottom = 16.dp)
                        .navigationBarsPadding(),
                )
            }
        }
    }
}

/** 顶栏:TikTok 双色 logo + 标题 | 健康胶囊 + 登出 —— 对应 App.vue 的 app-header */
@Composable
private fun Header(health: Health, loggedIn: Boolean, onLogout: () -> Unit) {
    Column {
        Row(
            Modifier
                .fillMaxWidth()
                .padding(horizontal = 20.dp, vertical = 14.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            TikTokLogo(Modifier.size(30.dp))
            Spacer(Modifier.size(10.dp))
            Text(
                "抖音下载器",
                fontSize = 18.sp,
                fontWeight = FontWeight.Bold,
                style = MaterialTheme.typography.titleLarge,
            )
            Spacer(Modifier.weight(1f))
            StatusPill(
                color = when (health) {
                    Health.ONLINE -> HealthOnline
                    Health.OFFLINE -> HealthOffline
                    Health.UNKNOWN -> Color(0xFF6E6E7A)
                },
                label = when (health) {
                    Health.ONLINE -> "在线"
                    Health.OFFLINE -> "离线"
                    Health.UNKNOWN -> "检测中"
                },
            )
            if (loggedIn) {
                Spacer(Modifier.size(8.dp))
                TextButton(onClick = onLogout) {
                    Text("登出", color = TextSecondary, fontSize = 14.sp)
                }
            }
        }
        // 顶栏底部分隔线(白 8%)
        Box(
            Modifier
                .fillMaxWidth()
                .padding(horizontal = 20.dp)
                .height(1.dp)
                .background(Color(0x14FFFFFF)),
        )
    }
}
