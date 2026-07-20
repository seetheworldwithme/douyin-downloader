<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { listJobs } from '../api'

const jobs = ref([])
const loading = ref(false)
const error = ref('')
let timer = null

const POLL_ACTIVE = 2500 // 有进行中任务时 2.5s 轮询
const POLL_IDLE = 15000 // 全部终态时降频到 15s

function statusTag(status) {
  switch (status) {
    case 'success':
      return { type: 'success', text: '成功' }
    case 'failed':
      return { type: 'danger', text: '失败' }
    case 'running':
      return { type: 'warning', text: '进行中' }
    default:
      return { type: 'info', text: '排队中' }
  }
}

function truncateUrl(url) {
  if (!url) return ''
  return url.length > 42 ? url.slice(0, 39) + '…' : url
}

function truncateFile(name) {
  if (!name) return '—'
  return name.length > 26 ? name.slice(0, 23) + '…' : name
}

function formatTime(iso) {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  const pad = (n) => String(n).padStart(2, '0')
  return `${d.getMonth() + 1}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(
    d.getMinutes()
  )}:${pad(d.getSeconds())}`
}

function formatBytes(n) {
  if (!n && n !== 0) return '—'
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  if (n < 1024 * 1024 * 1024) return `${(n / 1024 / 1024).toFixed(1)} MB`
  return `${(n / 1024 / 1024 / 1024).toFixed(2)} GB`
}

function formatSpeed(bps) {
  if (!bps || bps <= 0) return '—'
  return formatBytes(bps) + '/s'
}

// 字节百分比;无 total_bytes(如直播录制)则返回 null,前端降级为不确定态
function bytePercent(job) {
  if (job.status === 'success') return 100
  if (job.total_bytes && job.total_bytes > 0) {
    return Math.min(100, Math.round((job.downloaded_bytes / job.total_bytes) * 100))
  }
  return null
}

function progressStatus(job) {
  if (job.status === 'success') return 'success'
  if (job.status === 'failed') return 'exception'
  return undefined // 默认蓝色
}

const hasActive = computed(() =>
  jobs.value.some((j) => j.status === 'pending' || j.status === 'running')
)

const sortedJobs = computed(() =>
  [...jobs.value].sort((a, b) =>
    (b.created_at || '').localeCompare(a.created_at || '')
  )
)

async function refresh() {
  loading.value = true
  error.value = ''
  try {
    const { jobs: list } = await listJobs()
    jobs.value = list || []
  } catch (e) {
    error.value = e.message || '加载失败'
  } finally {
    loading.value = false
    scheduleNext()
  }
}

function scheduleNext() {
  if (timer) {
    clearTimeout(timer)
    timer = null
  }
  timer = setTimeout(refresh, hasActive.value ? POLL_ACTIVE : POLL_IDLE)
}

function manualRefresh() {
  refresh()
}

defineExpose({ refresh })

onMounted(() => {
  refresh()
})

onBeforeUnmount(() => {
  if (timer) clearTimeout(timer)
})
</script>

<template>
  <el-card shadow="never" class="job-card">
    <template #header>
      <div class="card-header">
        <span>下载任务</span>
        <div class="header-actions">
          <span class="status-line" v-if="hasActive">⏳ 正在轮询…</span>
          <el-button text :loading="loading" @click="manualRefresh">刷新</el-button>
        </div>
      </div>
    </template>

    <el-alert
      v-if="error"
      :title="`加载任务列表失败:${error}`"
      type="error"
      show-icon
      :closable="false"
      style="margin-bottom: 12px"
    />

    <!-- 移动端:卡片式列表 -->
    <div class="mobile-jobs">
      <div v-for="row in sortedJobs" :key="row.job_id" class="m-job">
        <div class="m-row">
          <a :href="row.url" target="_blank" rel="noopener" class="job-url" :title="row.url">
            {{ truncateUrl(row.url) }}
          </a>
          <el-tag :type="statusTag(row.status).type" effect="light" size="small">
            <span v-if="row.status === 'running'">⏳ </span>
            {{ statusTag(row.status).text }}
          </el-tag>
        </div>
        <el-progress
          v-if="bytePercent(row) !== null"
          :percentage="bytePercent(row)"
          :status="progressStatus(row)"
          :stroke-width="14"
          :indeterminate="row.status === 'running' && bytePercent(row) === 0"
        />
        <div class="m-meta">
          <span>📁 {{ truncateFile(row.current_file) }}</span>
          <span v-if="row.status === 'running'">⚡ {{ formatSpeed(row.speed_bps) }}</span>
          <span>{{ formatBytes(row.downloaded_bytes) }}<template v-if="row.total_bytes"> / {{ formatBytes(row.total_bytes) }}</template></span>
        </div>
        <div class="m-meta">
          <span>作品 {{ row.item_done }}/{{ row.item_total || '?' }}</span>
          <span>✅{{ row.success }} ❌{{ row.failed }} ⏭{{ row.skipped }}</span>
        </div>
        <div class="m-meta muted" v-if="row.save_dir">📂 {{ row.save_dir }}</div>
        <div class="m-meta muted" v-if="row.error">⚠ {{ row.error }}</div>
        <div class="m-meta muted">{{ formatTime(row.created_at) }}</div>
      </div>
      <div v-if="sortedJobs.length === 0" class="m-empty">暂无任务,在上方提交一个抖音链接试试</div>
    </div>

    <!-- 桌面端:表格 -->
    <el-table
      class="desktop-jobs"
      :data="sortedJobs"
      v-loading="loading"
      empty-text="暂无任务,在上方提交一个抖音链接试试"
      stripe
    >
      <el-table-column label="链接" min-width="200">
        <template #default="{ row }">
          <a :href="row.url" target="_blank" rel="noopener" class="job-url" :title="row.url">
            {{ truncateUrl(row.url) }}
          </a>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="100" align="center">
        <template #default="{ row }">
          <el-tag :type="statusTag(row.status).type" effect="light" size="small">
            <span v-if="row.status === 'running'">⏳ </span>
            {{ statusTag(row.status).text }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="进度" min-width="200">
        <template #default="{ row }">
          <el-progress
            v-if="bytePercent(row) !== null"
            :percentage="bytePercent(row)"
            :status="progressStatus(row)"
            :stroke-width="14"
            :indeterminate="row.status === 'running' && bytePercent(row) === 0"
          />
          <span v-else class="muted">—</span>
        </template>
      </el-table-column>
      <el-table-column label="当前文件" min-width="160">
        <template #default="{ row }">{{ truncateFile(row.current_file) }}</template>
      </el-table-column>
      <el-table-column label="网速" width="100" align="center">
        <template #default="{ row }">
          <span v-if="row.status === 'running'">{{ formatSpeed(row.speed_bps) }}</span>
          <span v-else class="muted">—</span>
        </template>
      </el-table-column>
      <el-table-column label="字节" width="120" align="center">
        <template #default="{ row }">
          {{ formatBytes(row.downloaded_bytes) }}<template v-if="row.total_bytes"> / {{ formatBytes(row.total_bytes) }}</template>
        </template>
      </el-table-column>
      <el-table-column label="作品" width="100" align="center">
        <template #default="{ row }">{{ row.item_done }}/{{ row.item_total || '?' }}</template>
      </el-table-column>
      <el-table-column label="成功/失败/跳过" width="130" align="center">
        <template #default="{ row }">
          <span class="num-ok">{{ row.success }}</span> /
          <span class="num-fail">{{ row.failed }}</span> /
          <span class="num-skip">{{ row.skipped }}</span>
        </template>
      </el-table-column>
      <el-table-column label="保存目录" min-width="140">
        <template #default="{ row }">
          <span v-if="row.save_dir" class="muted" :title="row.save_dir">📂 {{ row.save_dir }}</span>
          <span v-else class="muted">—</span>
        </template>
      </el-table-column>
      <el-table-column label="创建时间" width="150">
        <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
      </el-table-column>
      <el-table-column label="错误" min-width="160">
        <template #default="{ row }">
          <span v-if="row.error" class="job-error" :title="row.error">{{ row.error }}</span>
          <span v-else class="muted">—</span>
        </template>
      </el-table-column>
    </el-table>
  </el-card>
</template>

<style scoped>
.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.header-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}
.status-line {
  font-size: 12px;
  color: #e6a23c;
  display: inline-flex;
  align-items: center;
  gap: 4px;
}
.job-url {
  color: #409eff;
  text-decoration: none;
  word-break: break-all;
}
.job-url:hover {
  text-decoration: underline;
}
.num-ok {
  color: #67c23a;
  font-weight: 600;
}
.num-fail {
  color: #f56c6c;
  font-weight: 600;
}
.num-skip {
  color: #909399;
}
.job-error {
  font-size: 12px;
  color: #f56c6c;
  display: inline-block;
  max-width: 200px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.muted {
  color: #909399;
}

/* 移动端卡片列表(默认隐藏,窄屏显示) */
.mobile-jobs {
  display: none;
}
.m-job {
  border: 1px solid #ebeef5;
  border-radius: 8px;
  padding: 10px 12px;
  margin-bottom: 10px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.m-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
}
.m-meta {
  font-size: 12px;
  color: #606266;
  display: flex;
  justify-content: space-between;
  gap: 8px;
  flex-wrap: wrap;
}
.m-meta.muted {
  color: #909399;
}
.m-empty {
  text-align: center;
  color: #909399;
  padding: 24px;
  font-size: 13px;
}

/* 桌面端表格(默认显示,窄屏隐藏) */
.desktop-jobs {
  display: block;
}

@media (max-width: 768px) {
  .mobile-jobs {
    display: block;
  }
  .desktop-jobs {
    display: none;
  }
}
</style>
