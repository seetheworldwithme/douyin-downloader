<script setup>
import { ref, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { resolveVideo, streamUrl } from '../api'

const raw = ref('')
const busy = ref(false)

// 粘贴的可能是整段分享文案(含文案 + 短链 + 尾部乱码),从中提取第一个抖音链接
function normalizeUrl(u) {
  return u.replace(/[.,，。!！?？]+$/, '').replace(/\/+$/, '')
}
function extractUrl(text) {
  if (!text) return ''
  // 1) 带协议的链接(覆盖 https://v.douyin.com/xxx、www.douyin.com/video/xxx 等)
  const m = /https?:\/\/[^\s，。、,）)】]+/.exec(text)
  if (m) return normalizeUrl(m[0])
  // 2) 兜底:不带协议的裸短链 v.douyin.com/xxxxx
  const s = /\bv\.douyin\.com\/[A-Za-z0-9_-]+/.exec(text)
  if (s) return normalizeUrl('https://' + s[0])
  return ''
}

const url = computed(() => extractUrl(raw.value.trim()))
const isValid = computed(() => !!url.value)

async function handleDownload() {
  if (!url.value) {
    ElMessage.warning('没有识别到抖音链接,请粘贴视频链接或分享文案')
    return
  }

  busy.value = true
  // 1) 先解析预览(失败在这里友好提示,不会触发浏览器跳错误页)
  let info
  try {
    info = await resolveVideo(url.value)
  } catch (e) {
    ElMessage.error(e.message || '解析失败')
    busy.value = false
    return
  }

  // 2) 确认框 -> 触发浏览器原生另存为(浏览器流式直写本地,不经 JS 内存)
  ElMessageBox.confirm(`标题:${info.title}\n文件名:${info.filename}`, '确认下载该视频?', {
    confirmButtonText: '下载',
    cancelButtonText: '取消',
    type: 'info',
  })
    .then(() => {
      const a = document.createElement('a')
      a.href = streamUrl(url.value)
      a.download = '' // 让服务器 Content-Disposition 决定文件名
      document.body.appendChild(a)
      a.click()
      document.body.removeChild(a)
    })
    .catch(() => {
      /* 用户取消 */
    })
    .finally(() => {
      busy.value = false
    })
}
</script>

<template>
  <el-card shadow="never" class="submit-card">
    <template #header>
      <div class="card-header">
        <span>下载视频</span>
        <span class="hint">仅支持单个抖音视频(/video/);粘贴分享文案会自动提取链接</span>
      </div>
    </template>

    <el-input
      v-model="raw"
      type="textarea"
      :rows="3"
      placeholder="粘贴抖音视频链接,或整段分享文案(例如:1.53 复制打开抖音……https://v.douyin.com/xxxxx/)"
      resize="vertical"
    />

    <div class="extracted" v-if="url">
      <span class="label">识别到链接:</span><code>{{ url }}</code>
    </div>

    <div class="actions">
      <el-button type="primary" :loading="busy" :disabled="!isValid" @click="handleDownload">
        下载视频
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
.extracted {
  margin-top: 10px;
  font-size: 13px;
  color: #606266;
  word-break: break-all;
}
.extracted .label {
  color: #909399;
  margin-right: 4px;
}
.extracted code {
  background: #f4f4f5;
  padding: 1px 6px;
  border-radius: 3px;
}
.actions {
  margin-top: 14px;
  text-align: right;
}
</style>
