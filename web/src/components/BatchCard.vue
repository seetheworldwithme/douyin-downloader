<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import {
  batchStreamUrl,
  createBatchJob,
  getBatchJob,
  getCookieStatus,
  importCookies,
  streamUrl,
} from '../api'

const raw = ref('')
const maxItems = ref(50)
const incremental = ref(true)
const mode = ref('post')
const busy = ref(false)
const job = ref(null)
const selectedIds = ref([])
const cookieStatus = ref(null)
const showCookieDialog = ref(false)
const cookieText = ref('')
const savingCookie = ref(false)
let pollTimer = null

function normalizeUrl(u) {
  return u.replace(/[.,，。!！?？]+$/, '').replace(/\/+$/, '')
}
function extractUrl(text) {
  if (!text) return ''
  const m = /https?:\/\/[^\s，。、,）)】]+/.exec(text)
  if (m) return normalizeUrl(m[0])
  const s = /\bv\.douyin\.com\/[A-Za-z0-9_-]+/.exec(text)
  if (s) return normalizeUrl('https://' + s[0])
  return ''
}

const url = computed(() => extractUrl(raw.value.trim()))
const isCollectionUrl = computed(() => /\/(collection|mix)\//.test(url.value))
const running = computed(() => job.value && ['queued', 'running'].includes(job.value.status))
const items = computed(() => job.value?.items || [])
const allSelected = computed(() => items.value.length > 0 && selectedIds.value.length === items.value.length)
const progressText = computed(() => {
  if (!job.value) return ''
  if (job.value.status === 'queued') return '等待执行'
  if (job.value.status === 'running') return `已发现 ${job.value.success || 0} 条`
  if (job.value.status === 'completed') {
    return `完成：${job.value.success || 0} 条，跳过 ${job.value.skipped || 0} 条`
  }
  if (job.value.status === 'interrupted') return '任务已中断，可在任务中心重新执行'
  return job.value.error || '任务失败'
})

const modeLabel = computed(() => ({ post: '发布作品', like: '点赞作品', mix: '合集作品' }[job.value?.mode || mode.value] || '发布作品'))

async function refreshCookieStatus() {
  try {
    cookieStatus.value = await getCookieStatus()
  } catch (_e) {
    cookieStatus.value = null
  }
}

function stopPoll() {
  if (pollTimer) clearTimeout(pollTimer)
  pollTimer = null
}

async function pollJob(id) {
  stopPoll()
  try {
    const data = await getBatchJob(id)
    job.value = data
    if (['queued', 'running'].includes(data.status)) {
      pollTimer = setTimeout(() => pollJob(id), 1200)
    } else if (data.status === 'completed') {
      selectedIds.value = (data.items || []).map((item) => item.aweme_id)
    }
  } catch (e) {
    ElMessage.error(e.message || '读取任务失败')
  }
}

async function startBatch() {
  if (!url.value) {
    ElMessage.warning('请粘贴抖音作者主页或合集链接')
    return
  }
  let actualMode = mode.value
  if (isCollectionUrl.value) actualMode = 'mix'
  if (!isCollectionUrl.value && actualMode === 'mix') {
    ElMessage.warning('合集模式需要粘贴 /collection/ 或 /mix/ 合集链接')
    return
  }

  busy.value = true
  selectedIds.value = []
  try {
    const data = await createBatchJob(url.value, Number(maxItems.value) || 50, incremental.value, actualMode)
    job.value = data
    pollJob(data.job_id)
  } catch (e) {
    ElMessage.error(e.message || '创建任务失败')
  } finally {
    busy.value = false
  }
}

function downloadItem(item) {
  const a = document.createElement('a')
  a.href = streamUrl(item.url, item.type === 'images' ? 'images' : undefined)
  a.download = ''
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
}

function toggleAll() {
  selectedIds.value = allSelected.value ? [] : items.value.map((item) => item.aweme_id)
}

function downloadSelected() {
  if (!job.value?.job_id || selectedIds.value.length === 0) {
    ElMessage.warning('请至少选择一个作品')
    return
  }
  const a = document.createElement('a')
  a.href = batchStreamUrl(job.value.job_id, selectedIds.value)
  a.download = ''
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  ElMessage.success(`正在准备 ${selectedIds.value.length} 个作品的 ZIP 流式下载`)
}

async function saveCookie() {
  if (!cookieText.value.trim()) {
    ElMessage.warning('请粘贴 Cookie 字符串')
    return
  }
  savingCookie.value = true
  try {
    const result = await importCookies(cookieText.value.trim())
    cookieStatus.value = result
    cookieText.value = ''
    showCookieDialog.value = false
    ElMessage.success(result.valid ? 'Cookie 已保存并通过基础校验' : 'Cookie 已保存，但缺少部分关键字段')
  } catch (e) {
    ElMessage.error(e.message || 'Cookie 保存失败')
  } finally {
    savingCookie.value = false
  }
}

function formatTime(ts) {
  if (!ts) return '时间未知'
  return new Date(ts * 1000).toLocaleString()
}

onMounted(refreshCookieStatus)
onBeforeUnmount(stopPoll)
</script>

<template>
  <el-card shadow="never" class="batch-card">
    <template #header>
      <div class="card-header">
        <div>
          <div class="title">批量下载</div>
          <div class="hint">作者发布/点赞、合集扫描；SQLite 去重；支持选中作品流式打包 ZIP</div>
        </div>
        <el-button text @click="showCookieDialog = true">
          Cookie
          <el-tag class="cookie-tag" size="small" :type="cookieStatus?.valid ? 'success' : 'warning'">
            {{ cookieStatus?.valid ? '可用' : '待检查' }}
          </el-tag>
        </el-button>
      </div>
    </template>

    <el-input
      v-model="raw"
      type="textarea"
      :rows="2"
      placeholder="粘贴作者主页 https://www.douyin.com/user/... 或合集 https://www.douyin.com/collection/..."
      resize="vertical"
    />

    <div v-if="url" class="extracted">
      <span>识别到：</span><code>{{ url }}</code>
      <el-tag v-if="isCollectionUrl" size="small" type="success">合集链接</el-tag>
    </div>

    <div class="mode-row">
      <span class="option-label">内容类型</span>
      <el-radio-group v-model="mode" :disabled="running || isCollectionUrl">
        <el-radio-button value="post">发布作品</el-radio-button>
        <el-radio-button value="like">点赞作品</el-radio-button>
        <el-radio-button value="mix">合集链接</el-radio-button>
      </el-radio-group>
      <span class="muted">合集链接会自动切换为合集模式；点赞列表需账号可见且 Cookie 有权限。</span>
    </div>

    <div class="options">
      <div class="option">
        <span>最多扫描</span>
        <el-input-number v-model="maxItems" :min="1" :max="500" :step="10" controls-position="right" />
        <span>条</span>
      </div>
      <div class="option">
        <span>增量模式</span>
        <el-switch v-model="incremental" />
        <span class="muted">{{ incremental ? '跳过数据库中已有作品' : '返回最近作品，即使之前扫过' }}</span>
      </div>
      <el-button type="primary" :loading="busy" :disabled="!url || running" @click="startBatch">
        {{ running ? '扫描中…' : '开始扫描' }}
      </el-button>
    </div>

    <div v-if="job" class="job-box">
      <div class="job-summary">
        <div>
          <strong>{{ job.author_nickname || '正在读取信息…' }}</strong>
          <el-tag size="small" effect="plain" class="mode-tag">{{ modeLabel }}</el-tag>
          <span class="muted job-id">{{ job.job_id }}</span>
        </div>
        <el-tag
          :type="job.status === 'completed' ? 'success' : ['failed', 'interrupted'].includes(job.status) ? 'danger' : 'primary'"
          effect="plain"
        >
          {{ progressText }}
        </el-tag>
      </div>

      <el-alert
        v-if="['failed', 'interrupted'].includes(job.status)"
        :title="job.error || progressText"
        type="error"
        :closable="false"
        show-icon
      />

      <template v-if="items.length">
        <div class="selection-bar">
          <el-button size="small" @click="toggleAll">{{ allSelected ? '取消全选' : '全选' }}</el-button>
          <span>已选择 {{ selectedIds.length }} / {{ items.length }}</span>
          <el-button
            type="primary"
            size="small"
            :disabled="selectedIds.length === 0 || job.status !== 'completed'"
            @click="downloadSelected"
          >
            下载选中 ZIP
          </el-button>
        </div>

        <div class="items">
          <div v-for="item in items" :key="item.aweme_id" class="item-row">
            <el-checkbox v-model="selectedIds" :value="item.aweme_id" />
            <div class="item-main">
              <div class="item-title" :title="item.title">{{ item.title }}</div>
              <div class="item-meta">
                <el-tag size="small" effect="plain">{{ item.type === 'images' ? '图集' : '视频' }}</el-tag>
                <span>{{ formatTime(item.create_time) }}</span>
                <span v-if="item.known" class="known">此前已发现</span>
              </div>
            </div>
            <el-button size="small" type="primary" plain @click="downloadItem(item)">单独下载</el-button>
          </div>
        </div>
      </template>

      <el-empty
        v-else-if="job.status === 'completed'"
        description="没有发现新的作品；如果开启了增量模式，这通常表示已经是最新"
        :image-size="70"
      />
    </div>

    <el-dialog v-model="showCookieDialog" title="更新抖音 Cookie" width="560px">
      <p class="cookie-help">
        从已登录抖音的浏览器请求中复制 Cookie 请求头，粘贴到这里。服务端只保存解析后的 Cookie 到
        <code>.cookies.json</code>，不会把原始字符串返回给前端。
      </p>
      <el-input
        v-model="cookieText"
        type="textarea"
        :rows="6"
        placeholder="ttwid=...; odin_tt=...; passport_csrf_token=..."
      />
      <template #footer>
        <el-button @click="showCookieDialog = false">取消</el-button>
        <el-button type="primary" :loading="savingCookie" @click="saveCookie">保存 Cookie</el-button>
      </template>
    </el-dialog>
  </el-card>
</template>

<style scoped>
.card-header,
.job-summary,
.options,
.option,
.item-row,
.item-meta,
.mode-row,
.selection-bar,
.extracted {
  display: flex;
  align-items: center;
}
.card-header,
.job-summary,
.item-row {
  justify-content: space-between;
}
.card-header { gap: 12px; }
.title { font-weight: 600; }
.hint,
.muted,
.extracted,
.item-meta,
.cookie-help,
.selection-bar { color: #909399; font-size: 12px; }
.cookie-tag,
.mode-tag { margin-left: 6px; }
.extracted { margin-top: 10px; gap: 8px; word-break: break-all; }
.extracted code,
.cookie-help code {
  background: #f4f4f5;
  padding: 1px 5px;
  border-radius: 3px;
}
.mode-row {
  gap: 12px;
  margin-top: 14px;
  flex-wrap: wrap;
}
.option-label { font-size: 14px; }
.options {
  margin-top: 14px;
  flex-wrap: wrap;
  gap: 14px 20px;
}
.option { gap: 8px; }
.options > .el-button { margin-left: auto; }
.job-box {
  margin-top: 18px;
  border-top: 1px solid #ebeef5;
  padding-top: 16px;
}
.job-summary { gap: 12px; margin-bottom: 12px; }
.job-id { margin-left: 8px; }
.selection-bar {
  justify-content: flex-end;
  gap: 10px;
  margin: 12px 0;
}
.items { display: flex; flex-direction: column; gap: 8px; }
.item-row {
  gap: 12px;
  padding: 10px 12px;
  border: 1px solid #ebeef5;
  border-radius: 8px;
  background: #fafafa;
}
.item-main { min-width: 0; flex: 1; }
.item-title {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 14px;
}
.item-meta { gap: 8px; margin-top: 5px; }
.known { color: #e6a23c; }
.cookie-help { line-height: 1.6; margin-top: 0; }
@media (max-width: 600px) {
  .options,
  .mode-row { align-items: flex-start; flex-direction: column; }
  .options > .el-button { width: 100%; margin-left: 0; }
  .job-summary { align-items: flex-start; flex-direction: column; }
  .job-id { display: block; margin: 4px 0 0; word-break: break-all; }
  .selection-bar { justify-content: space-between; flex-wrap: wrap; }
  .item-row { align-items: flex-start; }
}
</style>
