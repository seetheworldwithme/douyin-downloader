<script setup>
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { getHealth, getDefaults } from './api'
import SubmitCard from './components/SubmitCard.vue'
import JobTable from './components/JobTable.vue'

// 后端健康状态:unknown / online / offline
const health = ref('unknown')
const defaultPath = ref('') // web 默认保存目录(Video/douyin)
const currentPath = ref('') // 后端 config 实际保存目录
const jobTableRef = ref(null)
let healthTimer = null

async function checkHealth() {
  try {
    await getHealth()
    health.value = 'online'
  } catch (_e) {
    health.value = 'offline'
  }
}

async function loadDefaults() {
  try {
    const d = await getDefaults()
    defaultPath.value = d.default_path || ''
    currentPath.value = d.current_path || ''
  } catch (_e) {
    /* 忽略 */
  }
}

function onSubmitted() {
  // 提交后立即刷新一次任务列表
  jobTableRef.value?.refresh()
}

onMounted(() => {
  checkHealth()
  loadDefaults()
  healthTimer = setInterval(checkHealth, 5000)
})

onBeforeUnmount(() => {
  if (healthTimer) clearInterval(healthTimer)
})
</script>

<template>
  <div class="app">
    <header class="app-header">
      <div class="title">
        <span class="logo">🎬</span>
        <span>抖音下载器 · 控制台</span>
      </div>
      <div class="header-right">
        <span class="path-info" :title="`默认 ${defaultPath} / 当前 ${currentPath}`">
          📂 <span class="path-label">默认:</span>{{ defaultPath || '—' }}
        </span>
        <span class="health">
          <span class="dot" :class="health" />
          <span class="health-text">
            {{ health === 'online' ? '在线' : health === 'offline' ? '离线' : '检测中…' }}
          </span>
        </span>
      </div>
    </header>

    <main class="app-main">
      <SubmitCard @submitted="onSubmitted" />
      <JobTable ref="jobTableRef" />
    </main>

    <footer class="app-footer">
      <span>提交链接后任务在后台异步执行,列表会自动轮询刷新状态。</span>
    </footer>
  </div>
</template>

<style>
* {
  box-sizing: border-box;
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
.app {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
}
.app-header {
  background: #fff;
  border-bottom: 1px solid #ebeef5;
  padding: 0 24px;
  height: 60px;
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
.path-info {
  font-size: 13px;
  color: #606266;
  max-width: 280px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.path-label {
  color: #909399;
  margin-right: 2px;
}
@media (max-width: 600px) {
  .app-header {
    padding: 0 12px;
  }
  .title {
    font-size: 15px;
  }
  .header-right {
    gap: 8px;
  }
  .path-info {
    font-size: 11px;
    max-width: 130px;
  }
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
  padding: 24px;
  display: flex;
  flex-direction: column;
  gap: 24px;
}
.app-footer {
  text-align: center;
  font-size: 12px;
  color: #909399;
  padding: 12px;
}
</style>
