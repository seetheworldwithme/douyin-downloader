package io.github.nick.dydl

import android.os.Bundle
import androidx.activity.ComponentActivity
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
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
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
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.lifecycle.viewmodel.compose.viewModel
import io.github.nick.dydl.ui.DouyinTheme
import io.github.nick.dydl.ui.HealthOffline
import io.github.nick.dydl.ui.HealthOnline
import io.github.nick.dydl.ui.HintGray
import io.github.nick.dydl.ui.LoginScreen
import io.github.nick.dydl.ui.SubmitScreen

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        setContent {
            DouyinTheme {
                DouyinApp()
            }
        }
    }
}

/** 根布局:顶部 header + 内容区 —— 对应 web/src/App.vue */
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

    Column(
        Modifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background),
    ) {
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
                    .padding(horizontal = 16.dp, vertical = 20.dp),
            ) {
                if (!loggedIn) {
                    // 登录卡片居中、限宽(对应 LoginCard max-width:380px)
                    Box(Modifier.fillMaxWidth(), contentAlignment = Alignment.TopCenter) {
                        Box(Modifier.widthIn(max = 380.dp)) {
                            LoginScreen(vm)
                        }
                    }
                } else {
                    SubmitScreen(vm)
                }
            }
            SnackbarHost(
                snackbar,
                Modifier
                    .align(Alignment.BottomCenter)
                    .padding(bottom = 16.dp),
            )
        }
    }
}

/** 顶栏:logo + 标题 | 健康点 + 登出 —— 对应 App.vue 的 app-header */
@Composable
private fun Header(health: Health, loggedIn: Boolean, onLogout: () -> Unit) {
    Column {
        Row(
            Modifier
                .fillMaxWidth()
                .background(MaterialTheme.colorScheme.surface)
                .padding(horizontal = 16.dp, vertical = 12.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text("🎬", fontSize = 20.sp)
            Spacer(Modifier.size(8.dp))
            Text("抖音下载器", fontSize = 17.sp, style = MaterialTheme.typography.titleMedium)
            Spacer(Modifier.weight(1f))
            Box(
                Modifier
                    .size(10.dp)
                    .clip(CircleShape)
                    .background(
                        when (health) {
                            Health.ONLINE -> HealthOnline
                            Health.OFFLINE -> HealthOffline
                            Health.UNKNOWN -> Color(0xFFC0C4CC)
                        },
                    ),
            )
            Spacer(Modifier.size(6.dp))
            Text(
                when (health) {
                    Health.ONLINE -> "在线"
                    Health.OFFLINE -> "离线"
                    Health.UNKNOWN -> "检测中…"
                },
                fontSize = 13.sp,
                color = HintGray,
            )
            if (loggedIn) {
                Spacer(Modifier.size(12.dp))
                TextButton(onClick = onLogout) { Text("登出") }
            }
        }
        // 对应网页版的 border-bottom: 1px solid #ebeef5
        Box(
            Modifier
                .fillMaxWidth()
                .height(1.dp)
                .background(MaterialTheme.colorScheme.outline),
        )
    }
}
