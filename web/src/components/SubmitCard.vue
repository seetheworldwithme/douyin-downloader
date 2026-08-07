<script setup>
import { ref, computed, onBeforeUnmount } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { resolveVideo, streamUrl } from '../api'
import { Capacitor } from '@capacitor/core'
import { SaveToGallery } from '../plugins/saveToGallery'

const raw = ref('')
const busy = ref(false)
// 原生下载进度(0-100),-1 表示当前不在原生下载中
const progress = ref(-1)

// 是否运行在 Capacitor 安卓壳里(决定下载路径)
const isNative = Capacitor.isNativePlatform()
let progressListener = null

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

// 浏览器(PWA / PC / 安卓 Chrome):<a download> 触发原生另存为,落下载夹。
function triggerBrowserDownload() {
  const a = document.createElement('a')
  a.href = streamUrl(url.value)
  a.download = '' // 让服务器 Content-Disposition 决定文件名
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
}

// 安卓 APP(原生):OkHttp 拉服务端 /stream,写入 MediaStore.Video → 相册可见。
async function triggerNativeDownload(filename) {
  progress.value = 0
  progressListener = await SaveToGallery.addListener('saveProgress', (e) => {
    progress.value = typeof e.percent === 'number' ? e.percent : progress.value
  })
  try {
    await SaveToGallery.saveToGallery({ url: streamUrl(url.value), filename })
    ElMessage.success('已保存到相册:Movies/抖音下载器')
  } finally {
    if (progressListener) {
      await progressListener.remove()
      progressListener = null
    }
    progress.value = -1
  }
}

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

  // 2) 确认框 -> 按平台触发下载
  try {
    await ElMessageBox.confirm(
      `标题:${info.title}\n文件名:${info.filename}`,
      '确认下载该视频?',
      { confirmButtonText: '下载', cancelButtonText: '取消', type: 'info' },
    )
  } catch (_e) {
    busy.value = false
    return // 用户取消
  }

  try {
    if (isNative) {
      await triggerNativeDownload(info.filename)
    } else {
      triggerBrowserDownload()
    }
  } catch (e) {
    ElMessage.error(e.message || '下载失败')
  } finally {
    busy.value = false
  }
}

onBeforeUnmount(async () => {
  if (progressListener) {
    try {
      await progressListener.remove()
    } catch (_e) {
      /* 忽略 */
    }
  }
})
</script>

<template>
  <el-card shadow="never" class="submit-card">
    <template #header>
      <div class="card-header">
        <span>下载视频</span>
        <span class="hint">
          仅支持单个抖音视频(/video/);{{ isNative ? '保存到相册' : '浏览器另存为' }}
        </span>
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

    <div v-if="progress >= 0" class="progress">
      <span class="progress-label">正在保存到相册…{{ progress }}%</span>
      <el-progress :percentage="progress" :stroke-width="8" :show-text="false" />
    </div>

    <div class="actions">
      <el-button
        type="primary"
        :loading="busy"
        :disabled="!isValid"
        @click="handleDownload"
      >
        {{ progress >= 0 ? '下载中…' : '下载视频' }}
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
.progress {
  margin-top: 14px;
}
.progress-label {
  display: block;
  font-size: 13px;
  color: #606266;
  margin-bottom: 6px;
}
</style>
