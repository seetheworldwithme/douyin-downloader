<script setup>
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { getHealth, getToken, clearToken } from './api'
import LoginCard from './components/LoginCard.vue'
import SubmitCard from './components/SubmitCard.vue'
import BatchCard from './components/BatchCard.vue'
import TaskCenter from './components/TaskCenter.vue'
import HistoryCard from './components/HistoryCard.vue'

const loggedIn = ref(!!getToken())
const health = ref('unknown')
const activeTab = ref('single')
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
  activeTab.value = 'single'
}
function onAuthExpired() {
  loggedIn.value = false
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
        <el-button v-if="loggedIn" text @click="logout">登出</el-button>
      </div>
    </header>

    <main class="app-main">
      <LoginCard v-if="!loggedIn" @logged-in="onLoggedIn" />
      <div v-else class="workspace">
        <el-tabs v-model="activeTab" class="workspace-tabs" stretch>
          <el-tab-pane label="链接下载" name="single" lazy>
            <SubmitCard />
          </el-tab-pane>
          <el-tab-pane label="批量下载" name="batch" lazy>
            <BatchCard />
          </el-tab-pane>
          <el-tab-pane label="任务中心" name="tasks" lazy>
            <TaskCenter />
          </el-tab-pane>
          <el-tab-pane label="作品库" name="history" lazy>
            <HistoryCard />
          </el-tab-pane>
        </el-tabs>
      </div>
    </main>
  </div>
</template>

<style>
* { box-sizing: border-box; }
.el-button:not(.is-text):not(.is-link) { min-height: 40px; }
html,
body {
  margin: 0;
  padding: 0;
  background: #f5f7fa;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang SC',
    'Hiragino Sans GB', 'Microsoft YaHei', sans-serif;
  color: #303133;
}
.app {
  min-height: 100dvh;
  display: flex;
  flex-direction: column;
}
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
.logo { font-size: 22px; }
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
  max-width: 980px;
  width: 100%;
  margin: 0 auto;
  padding-top: 18px;
  padding-bottom: max(24px, env(safe-area-inset-bottom));
  padding-left: max(24px, env(safe-area-inset-left));
  padding-right: max(24px, env(safe-area-inset-right));
}
.workspace { width: 100%; }
.workspace-tabs > .el-tabs__header {
  margin-bottom: 18px;
  padding: 0 8px;
  background: #fff;
  border: 1px solid #ebeef5;
  border-radius: 10px;
}
.workspace-tabs > .el-tabs__header .el-tabs__nav-wrap::after { display: none; }
.workspace-tabs > .el-tabs__header .el-tabs__item { height: 48px; }
@media (max-width: 600px) {
  .app-header {
    padding-left: max(12px, env(safe-area-inset-left));
    padding-right: max(12px, env(safe-area-inset-right));
  }
  .title { font-size: 15px; }
  .header-right { gap: 8px; }
  .app-main {
    padding-top: 12px;
    padding-left: max(12px, env(safe-area-inset-left));
    padding-right: max(12px, env(safe-area-inset-right));
  }
  .workspace-tabs > .el-tabs__header .el-tabs__item {
    padding: 0 8px;
    font-size: 13px;
  }
}
</style>
