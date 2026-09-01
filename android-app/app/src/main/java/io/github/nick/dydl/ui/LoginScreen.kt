package io.github.nick.dydl.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import io.github.nick.dydl.AppViewModel

/** 登录卡片 —— 对应 web/src/components/LoginCard.vue */
@Composable
fun LoginScreen(vm: AppViewModel) {
    var username by rememberSaveable { mutableStateOf("") }
    var password by rememberSaveable { mutableStateOf("") }
    var showPassword by rememberSaveable { mutableStateOf(false) }
    val busy by vm.loginBusy.collectAsState()

    GlassCard(Modifier.fillMaxWidth()) {
        Column(verticalArrangement = Arrangement.spacedBy(16.dp)) {
            Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
                Text("欢迎回来", style = MaterialTheme.typography.titleLarge)
                Text("登录后开始下载视频与图集", fontSize = 13.sp, color = HintGray)
            }

            OutlinedTextField(
                value = username,
                onValueChange = { username = it },
                label = { Text("用户名") },
                singleLine = true,
                shape = androidx.compose.foundation.shape.RoundedCornerShape(14.dp),
                colors = darkTextFieldColors(),
                modifier = Modifier.fillMaxWidth(),
            )
            OutlinedTextField(
                value = password,
                onValueChange = { password = it },
                label = { Text("密码") },
                singleLine = true,
                shape = androidx.compose.foundation.shape.RoundedCornerShape(14.dp),
                colors = darkTextFieldColors(),
                visualTransformation =
                    if (showPassword) VisualTransformation.None else PasswordVisualTransformation(),
                // 安全键盘:关闭联想/自动更正,并让系统按密码输入法处理
                keyboardOptions = KeyboardOptions(
                    keyboardType = KeyboardType.Password,
                    autoCorrect = false,
                ),
                trailingIcon = {
                    Text(
                        if (showPassword) "隐藏" else "显示",
                        fontSize = 12.sp,
                        color = HintGray,
                        modifier = Modifier
                            .padding(horizontal = 8.dp)
                            .heightIn(min = 32.dp),
                    )
                },
                modifier = Modifier.fillMaxWidth(),
            )
            GradientButton(
                text = "登录",
                onClick = { vm.login(username, password) },
                enabled = !busy,
                busy = busy,
                modifier = Modifier.fillMaxWidth(),
            )
        }
    }
}
