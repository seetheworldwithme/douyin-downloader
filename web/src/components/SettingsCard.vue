<script setup>
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { getServerBase, setServerBase, getHealth } from '../api'

const emit = defineEmits(['saved'])

// 初始值取已保存的服务器地址;为空时给个占位提示
const input = ref(getServerBase())
const testing = ref(false)
const status = ref('idle') // idle / online / offline

function normalize(value) {
  return (value || '').trim().replace(/\/+$/, '')
}

async function save() {
  const base = normalize(input.value)
  setServerBase(base)
  input.value = base
  if (!base) {
    status.value = 'idle'
    emit('saved', base)
    return
  }
  // 立即探测一次,给用户即时反馈
  testing.value = true
  try {
    await getHealth()
    status.value = 'online'
    ElMessage.success('服务器地址已保存,连接正常')
  } catch (_e) {
    status.value = 'offline'
    ElMessage.warning('已保存,但当前连不上服务器(稍后会自动重试)')
  } finally {
    testing.value = false
    emit('saved', base)
  }
}
</script>

<template>
  <el-card shadow="never" class="settings-card">
    <template #header>
      <div class="card-header">
        <span>服务器设置</span>
        <span class="hint">APK / 自托管前端需填写;网页同源访问留空即可</span>
      </div>
    </template>

    <p class="desc">
      填入部署了下载服务(Python <code>--serve</code>)的地址,例如
      <code>https://your-nas.example.com:8000</code>。
    </p>

    <el-input
      v-model="input"
      placeholder="https://your-server:8000(留空=与网页同源)"
      clearable
    >
      <template #prepend>服务器</template>
    </el-input>

    <div v-if="status === 'online'" class="status online">● 已连接</div>
    <div v-else-if="status === 'offline'" class="status offline">● 暂时连不上</div>

    <div class="actions">
      <el-button type="primary" :loading="testing" @click="save">保存并测试</el-button>
    </div>
  </el-card>
</template>

<style scoped>
.card-header {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 4px;
}
.hint {
  font-size: 12px;
  color: #909399;
  font-weight: normal;
}
.desc {
  margin: 0 0 12px;
  font-size: 13px;
  color: #606266;
  line-height: 1.6;
}
.desc code,
.extracted code {
  background: #f4f4f5;
  padding: 1px 6px;
  border-radius: 3px;
  word-break: break-all;
}
.status {
  margin-top: 10px;
  font-size: 13px;
}
.status.online {
  color: #67c23a;
}
.status.offline {
  color: #e6a23c;
}
.actions {
  margin-top: 14px;
  text-align: right;
}
</style>
