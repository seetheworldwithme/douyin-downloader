<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { listBatchJobs, retryBatchJob } from '../api'

const jobs = ref([])
const loading = ref(false)
const retryingId = ref('')
let timer = null

const runningCount = computed(() => jobs.value.filter((j) => ['queued', 'running'].includes(j.status)).length)
const failedCount = computed(() => jobs.value.filter((j) => ['failed', 'interrupted'].includes(j.status)).length)
const completedCount = computed(() => jobs.value.filter((j) => j.status === 'completed').length)

function modeText(mode) {
  return { post: '发布作品', like: '点赞作品', mix: '合集作品' }[mode] || '发布作品'
}
function statusText(status) {
  return {
    queued: '等待中',
    running: '运行中',
    completed: '已完成',
    failed: '失败',
    interrupted: '已中断',
  }[status] || status
}
function statusType(status) {
  if (status === 'completed') return 'success'
  if (status === 'failed' || status === 'interrupted') return 'danger'
  if (status === 'running') return 'primary'
  return 'info'
}
function formatDate(value) {
  if (!value) return '-'
  const d = new Date(value)
  return Number.isNaN(d.getTime()) ? value : d.toLocaleString()
}

async function refresh(silent = false) {
  if (!silent) loading.value = true
  try {
    const data = await listBatchJobs()
    jobs.value = data?.items || []
  } catch (e) {
    if (!silent) ElMessage.error(e.message || '读取任务中心失败')
  } finally {
    if (!silent) loading.value = false
  }
}

async function retry(job) {
  retryingId.value = job.job_id
  try {
    const created = await retryBatchJob(job.job_id)
    ElMessage.success(`已创建重试任务 ${created.job_id}`)
    await refresh(true)
  } catch (e) {
    ElMessage.error(e.message || '重新执行失败')
  } finally {
    retryingId.value = ''
  }
}

function startTimer() {
  timer = setInterval(() => refresh(true), 3000)
}

onMounted(() => {
  refresh()
  startTimer()
})
onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
})
</script>

<template>
  <el-card shadow="never">
    <template #header>
      <div class="header-row">
        <div>
          <div class="title">任务中心</div>
          <div class="hint">运行中任务来自内存，历史任务来自 SQLite；服务重启后记录仍保留。</div>
        </div>
        <el-button :loading="loading" @click="refresh()">刷新</el-button>
      </div>
    </template>

    <div class="stats">
      <div class="stat"><strong>{{ runningCount }}</strong><span>运行中</span></div>
      <div class="stat"><strong>{{ completedCount }}</strong><span>已完成</span></div>
      <div class="stat"><strong>{{ failedCount }}</strong><span>失败/中断</span></div>
      <div class="stat"><strong>{{ jobs.length }}</strong><span>最近任务</span></div>
    </div>

    <el-empty v-if="!loading && jobs.length === 0" description="还没有批量任务" :image-size="80" />

    <div v-else class="jobs">
      <div v-for="job in jobs" :key="job.job_id" class="job-row">
        <div class="job-main">
          <div class="job-title-row">
            <strong>{{ job.author_nickname || '未命名任务' }}</strong>
            <el-tag size="small" effect="plain">{{ modeText(job.mode) }}</el-tag>
            <el-tag size="small" :type="statusType(job.status)">{{ statusText(job.status) }}</el-tag>
          </div>
          <div class="url" :title="job.url">{{ job.url }}</div>
          <div class="meta">
            <span>创建 {{ formatDate(job.created_at) }}</span>
            <span>总计 {{ job.total || 0 }}</span>
            <span>发现 {{ job.success || 0 }}</span>
            <span>跳过 {{ job.skipped || 0 }}</span>
            <span v-if="job.failed">失败 {{ job.failed }}</span>
          </div>
          <el-alert
            v-if="job.error"
            class="error-box"
            :title="job.error"
            type="error"
            :closable="false"
            show-icon
          />
        </div>
        <div class="actions">
          <el-button
            size="small"
            :loading="retryingId === job.job_id"
            :disabled="['queued', 'running'].includes(job.status)"
            @click="retry(job)"
          >
            重新执行
          </el-button>
        </div>
      </div>
    </div>
  </el-card>
</template>

<style scoped>
.header-row,
.job-title-row,
.meta,
.stats,
.job-row {
  display: flex;
  align-items: center;
}
.header-row,
.job-row { justify-content: space-between; gap: 16px; }
.title { font-weight: 600; }
.hint,
.meta,
.url { color: #909399; font-size: 12px; }
.stats {
  gap: 12px;
  margin-bottom: 18px;
  flex-wrap: wrap;
}
.stat {
  min-width: 110px;
  padding: 12px 16px;
  background: #f5f7fa;
  border-radius: 8px;
  display: flex;
  flex-direction: column;
}
.stat strong { font-size: 22px; }
.stat span { margin-top: 3px; color: #909399; font-size: 12px; }
.jobs { display: flex; flex-direction: column; gap: 10px; }
.job-row {
  padding: 13px 14px;
  border: 1px solid #ebeef5;
  border-radius: 9px;
}
.job-main { flex: 1; min-width: 0; }
.job-title-row { gap: 8px; flex-wrap: wrap; }
.url {
  margin-top: 6px;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}
.meta { margin-top: 7px; gap: 12px; flex-wrap: wrap; }
.error-box { margin-top: 8px; }
.actions { flex: none; }
@media (max-width: 600px) {
  .header-row,
  .job-row { align-items: flex-start; flex-direction: column; }
  .actions,
  .actions .el-button { width: 100%; }
  .stats { display: grid; grid-template-columns: 1fr 1fr; }
  .stat { min-width: 0; }
}
</style>
