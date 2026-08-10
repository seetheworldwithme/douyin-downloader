<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { Capacitor } from '@capacitor/core'
import { getHealth, getToken, clearToken, getServerBase } from './api'
import LoginCard from './components/LoginCard.vue'
import SubmitCard from './components/SubmitCard.vue'
import SettingsCard from './components/SettingsCard.vue'

// 是否跑在 Capacitor 安卓壳里。网页(同源部署)serverBase 留空即正确,
// 无需强制配置;只有 APP(跨域)首启才必须先填地址。
const isNative = Capacitor.isNativePlatform()

// 服务器地址。localStorage 非响应式,这里用 ref 镜像,SettingsCard 保存时
// 通过 @saved 回写,驱动 mustConfigure 计算。
const serverBase = ref(getServerBase())
const showSettings = ref(false)
// 仅在 APP 里未配地址时强制显示设置;网页同源时空地址就是正确状态。
const mustConfigure = computed(() => isNative && !serverBase.value)

const loggedIn = ref(!!getToken())
const health = ref('unknown') // unknown / online / offline
let healthTimer = null

async function checkHealth() {
  try {
    await getHealth()
    health.value = 'online'
  } catch (_e) {
    health.value = 'offline'
  }
}

function onLoggedIn() {
  loggedIn.value = true
}
function logout() {
  clearToken()
  loggedIn.value = false
}
// token 过期(api.js 在 401 时派发)→ 自动退回登录页
function onAuthExpired() {
  loggedIn.value = false
}

function onSettingsSaved(base) {
  serverBase.value = base
  // 配置成功后自动收起设置面板(留空时保持展开,提示用户继续配置)
  if (base) showSettings.value = false
  checkHealth()
}

onMounted(() => {
  checkHealth()
  healthTimer = setInterval(checkHealth, 5000)
  window.addEventListener('dd-auth-expired', onAuthExpired)
})
onBeforeUnmount(() => {
  if (healthTimer) clearInterval(healthTimer)
  window.removeEventListener('dd-auth-expired', onAuthExpired)
})
</script>

<template>
  <div class="app">
    <header class="app-header">
      <div class="title">
        <span class="logo">🎬</span>
        <span>抖音下载器</span>
      </div>
      <div class="header-right">
        <span class="health">
          <span class="dot" :class="health" />
          <span class="health-text">
            {{ health === 'online' ? '在线' : health === 'offline' ? '离线' : '检测中…' }}
          </span>
        </span>
        <el-button text @click="showSettings = !showSettings">设置</el-button>
        <el-button v-if="loggedIn" text @click="logout">登出</el-button>
      </div>
    </header>

    <main class="app-main">
      <!-- APP 首启未配地址时强制设置;网页同源时直接进登录 -->
      <SettingsCard v-if="mustConfigure || showSettings" @saved="onSettingsSaved" />
      <template v-else>
        <LoginCard v-if="!loggedIn" @logged-in="onLoggedIn" />
        <SubmitCard v-else />
      </template>
    </main>
  </div>
</template>

<style>
* {
  box-sizing: border-box;
}
/* 触摸目标放大到安卓 Material 推荐的 ≥40dp;不影响 header 里的 text 按钮 */
.el-button:not(.is-text):not(.is-link) {
  min-height: 40px;
}
html,
body {
  margin: 0;
  padding: 0;
  background: #f5f7fa;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang SC',
    'Hiragino Sans GB', 'Microsoft YaHei', sans-serif;
  color: #303133;
}
/* dvh:移动端动态工具栏下更可靠;100vh 在 WebView 里会偏大 */
.app {
  min-height: 100dvh;
  display: flex;
  flex-direction: column;
}
/* 顶部留状态栏、左右留圆角/刘海安全区(安卓 15 edge-to-edge) */
.app-header {
  background: #fff;
  border-bottom: 1px solid #ebeef5;
  padding-top: env(safe-area-inset-top);
  padding-left: max(24px, env(safe-area-inset-left));
  padding-right: max(24px, env(safe-area-inset-right));
  padding-bottom: 0;
  height: calc(60px + env(safe-area-inset-top));
  box-sizing: border-box;
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.title {
  font-size: 18px;
  font-weight: 600;
  display: flex;
  align-items: center;
  gap: 8px;
}
.logo {
  font-size: 22px;
}
.header-right {
  display: flex;
  align-items: center;
  gap: 16px;
}
.health {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: #606266;
}
.dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: #c0c4cc;
  display: inline-block;
}
.dot.online {
  background: #67c23a;
  box-shadow: 0 0 0 3px rgba(103, 194, 58, 0.2);
}
.dot.offline {
  background: #f56c6c;
  box-shadow: 0 0 0 3px rgba(245, 108, 108, 0.2);
}
.app-main {
  flex: 1;
  max-width: 760px;
  width: 100%;
  margin: 0 auto;
  padding-top: 24px;
  /* 底部留出手势导航条安全区 */
  padding-bottom: max(24px, env(safe-area-inset-bottom));
  padding-left: max(24px, env(safe-area-inset-left));
  padding-right: max(24px, env(safe-area-inset-right));
  display: flex;
  flex-direction: column;
  gap: 24px;
}
@media (max-width: 600px) {
  .app-header {
    padding-left: max(12px, env(safe-area-inset-left));
    padding-right: max(12px, env(safe-area-inset-right));
  }
  .title {
    font-size: 15px;
  }
  .header-right {
    gap: 8px;
  }
}
</style>
