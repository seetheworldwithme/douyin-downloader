<script setup>
import { ref, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { resolveVideo, streamUrl } from '../api'

const raw = ref('')
const busy = ref(false)

// 图集解析结果 + 下载方式选择弹窗
const imageChoice = ref(null)
const showImageDialog = ref(false)
// busy 时标记当前正在下载哪种(mode),用于按钮 loading 态
const busyMode = ref('')

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
// (安卓 App 用户请使用原生版 android-app/,直接存系统相册)
function triggerBrowserDownload(mode) {
  const a = document.createElement('a')
  a.href = streamUrl(url.value, mode)
  a.download = '' // 让服务器 Content-Disposition 决定文件名
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
}

async function handleDownload() {
  if (!url.value) {
    ElMessage.warning('没有识别到抖音链接,请粘贴视频/图集链接或分享文案')
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

  // 2) 图集:多图弹选择框(下载图片 / 合成视频),单图直接确认下载
  if (info.type === 'images') {
    if ((info.image_count || 0) > 1) {
      imageChoice.value = info
      showImageDialog.value = true
      busy.value = false
      return
    }
    try {
      await ElMessageBox.confirm(
        `标题:${info.title}\n将下载 1 张图片`,
        '确认下载该图片?',
        { confirmButtonText: '下载', cancelButtonText: '取消', type: 'info' },
      )
    } catch (_e) {
      busy.value = false
      return // 用户取消
    }
    await runDownload('images')
    return
  }

  // 3) 视频:确认框 -> 触发下载
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

  await runDownload(null)
}

// mode:null=视频 / 'images'=图片 / 'video'=图集合成视频;返回是否成功
async function runDownload(mode) {
  busy.value = true
  busyMode.value = mode || 'video'
  try {
    triggerBrowserDownload(mode)
    return true
  } catch (e) {
    ElMessage.error(e.message || '下载失败')
    return false
  } finally {
    busy.value = false
    busyMode.value = ''
  }
}

function cancelImageDialog() {
  if (busy.value) return
  showImageDialog.value = false
  imageChoice.value = null
}

// 图集弹窗里的两种下载方式;成功后关弹窗,失败保留以便换另一种方式重试
async function downloadGallery(mode) {
  const info = imageChoice.value
  if (!info || busy.value) return

  const ok = await runDownload(mode)
  if (ok) showImageDialog.value = false
}
</script>

<template>
  <el-card shadow="never" class="submit-card">
    <template #header>
      <div class="card-header">
        <span>下载视频 / 图集</span>
        <span class="hint">
          支持抖音视频和图集链接;浏览器另存为
        </span>
      </div>
    </template>

    <el-input
      v-model="raw"
      type="textarea"
      :rows="3"
      placeholder="粘贴抖音视频/图集链接,或整段分享文案(例如:1.53 复制打开抖音……https://v.douyin.com/xxxxx/)"
      resize="vertical"
    />

    <div class="extracted" v-if="url">
      <span class="label">识别到链接:</span><code>{{ url }}</code>
    </div>

    <div class="actions">
      <el-button
        type="primary"
        :loading="busy"
        :disabled="!isValid"
        @click="handleDownload"
      >
        下载
      </el-button>
    </div>

    <!-- 图集下载方式选择:多张图片时让用户选"下载图片"还是"合成视频" -->
    <el-dialog
      v-model="showImageDialog"
      title="图集下载方式"
      width="440px"
      :close-on-click-modal="!busy"
      :close-on-press-escape="!busy"
      :show-close="!busy"
      @closed="imageChoice = null"
    >
      <div v-if="imageChoice" class="gallery-info">
        <p class="line">
          <span class="label">标题:</span>{{ imageChoice.title }}
        </p>
        <p class="line">
          <span class="label">图片:</span>
          共 {{ imageChoice.image_count }} 张<template v-if="imageChoice.has_music">(含原声音乐)</template>
        </p>
        <p class="tip">
          「合成视频」会把图片按原声时长合成为 MP4,服务器需要十几秒到几分钟处理,请耐心等待;
          「下载图片」立即打包下载 ZIP 原图。
        </p>
      </div>
      <template #footer>
        <el-button :disabled="busy" @click="cancelImageDialog">取消</el-button>
        <el-button
          type="primary"
          plain
          :loading="busy && busyMode === 'images'"
          @click="downloadGallery('images')"
        >
          下载图片
        </el-button>
        <el-button
          type="primary"
          :loading="busy && busyMode === 'video'"
          @click="downloadGallery('video')"
        >
          合成视频下载
        </el-button>
      </template>
    </el-dialog>
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
.gallery-info .line {
  margin: 6px 0;
  font-size: 14px;
  color: #303133;
  word-break: break-all;
}
.gallery-info .label {
  color: #909399;
  margin-right: 4px;
}
.gallery-info .tip {
  margin: 10px 0 0;
  font-size: 12px;
  color: #909399;
  line-height: 1.6;
}
</style>
