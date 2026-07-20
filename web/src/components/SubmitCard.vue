<script setup>
import { ref, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { submitDownload } from '../api'

const emit = defineEmits(['submitted'])

const rawText = ref('')
// web 前端默认保存目录（与后端 /api/v1/defaults 的 default_path 保持一致）
const DEFAULT_SAVE_DIR = 'Video/douyin'
const saveDir = ref(DEFAULT_SAVE_DIR)
const submitting = ref(false)

// 内容类型开关。视频(mp4)是核心产物,始终下载,用 disabled 勾选框表示;
// 其余附件默认不勾——只有勾选的才下载。
const content = ref({
  video: true, // 锁定:视频本体,始终下载(仅作展示,不发给后端)
  music: false,
  cover: false,
  avatar: false,
  json: false,
})

// 从「分享文案」里提取真实链接：优先抓 http(s) 链接，再兜底抓不带协议的抖音短链
// 提取后做归一化(去尾斜杠)再去重,避免同一链接因尾斜杠差异被当成两条
function normalizeUrl(u) {
  return u.replace(/[.,，。!！?？]+$/, '').replace(/\/+$/, '')
}

function extractUrls(text) {
  if (!text) return []
  const found = new Set()
  // 1) 带协议的链接（覆盖 https://v.douyin.com/xxx、www.douyin.com/video/xxx 等）
  const urlRe = /https?:\/\/[^\s，。、,）)】]+/g
  let m
  while ((m = urlRe.exec(text)) !== null) {
    found.add(normalizeUrl(m[0]))
  }
  // 2) 兜底：不带协议的 v.douyin.com/xxxxx 裸短链
  const shortRe = /\bv\.douyin\.com\/[A-Za-z0-9_-]+/g
  while ((m = shortRe.exec(text)) !== null) {
    found.add(normalizeUrl('https://' + m[0]))
  }
  return Array.from(found)
}

const urls = computed(() => extractUrls(rawText.value))

async function handleSubmit() {
  if (urls.value.length === 0) {
    ElMessage.warning('没有识别到有效的抖音链接')
    return
  }
  submitting.value = true
  const dir = saveDir.value
  // 只把附件开关发给后端(video 本体始终下载,无需发送)
  const payload = {
    music: !!content.value.music,
    cover: !!content.value.cover,
    avatar: !!content.value.avatar,
    json: !!content.value.json,
  }
  const results = await Promise.allSettled(
    urls.value.map((url) => submitDownload(url, dir, payload))
  )

  const ok = results.filter((r) => r.status === 'fulfilled').length
  const fail = results.length - ok
  if (ok > 0) {
    ElMessage.success(`已提交 ${ok} 个任务`)
  }
  if (fail > 0) {
    const firstErr = results.find((r) => r.status === 'rejected')
    const msg = firstErr?.reason?.message || '未知错误'
    ElMessage.error(`${fail} 个提交失败:${msg}`)
  }

  submitting.value = false
  if (ok > 0) {
    rawText.value = ''
    emit('submitted')
  }
}
</script>

<template>
  <el-card shadow="never" class="submit-card">
    <template #header>
      <div class="card-header">
        <span>提交下载</span>
        <span class="hint">支持粘贴整段抖音分享文案,自动提取链接、批量提交、自动去重</span>
      </div>
    </template>

    <el-input
      v-model="rawText"
      type="textarea"
      :rows="4"
      placeholder="粘贴抖音链接或整段分享文案,例如:9.99 复制打开抖音 https://v.douyin.com/xxxxx/"
      resize="vertical"
    />

    <div class="save-dir">
      <span class="dir-label">保存目录</span>
      <el-input
        v-model="saveDir"
        size="default"
        placeholder="留空则使用后端默认 ./Downloaded/"
        clearable
      />
      <el-button text @click="saveDir = DEFAULT_SAVE_DIR">重置默认</el-button>
    </div>

    <div class="content-types">
      <span class="dir-label">下载内容</span>
      <div class="checks">
        <el-checkbox v-model="content.video" disabled>视频(mp4)必选</el-checkbox>
        <el-checkbox v-model="content.music">音乐(mp3)</el-checkbox>
        <el-checkbox v-model="content.cover">封面</el-checkbox>
        <el-checkbox v-model="content.avatar">头像</el-checkbox>
        <el-checkbox v-model="content.json">元数据</el-checkbox>
      </div>
      <span class="hint2">仅勾选的内容会被下载,默认只下载视频</span>
    </div>

    <div class="actions">
      <span class="count" v-if="urls.length">识别到 {{ urls.length }} 个链接</span>
      <span class="count" v-else>未识别到链接</span>
      <el-button
        type="primary"
        :loading="submitting"
        :disabled="urls.length === 0"
        @click="handleSubmit"
      >
        提交下载
      </el-button>
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
.save-dir {
  margin-top: 12px;
  display: flex;
  align-items: center;
  gap: 8px;
}
.dir-label {
  font-size: 13px;
  color: #606266;
  white-space: nowrap;
}
.save-dir :deep(.el-input) {
  flex: 1;
}
.content-types {
  margin-top: 12px;
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.checks {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-wrap: wrap;
}
.hint2 {
  font-size: 12px;
  color: #909399;
}
.actions {
  margin-top: 12px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.count {
  font-size: 13px;
  color: #606266;
}
@media (max-width: 600px) {
  .save-dir {
    flex-wrap: wrap;
  }
}
</style>
