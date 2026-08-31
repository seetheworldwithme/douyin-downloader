<script setup>
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { getHistory, streamUrl } from '../api'

const items = ref([])
const loading = ref(false)
const q = ref('')
const author = ref('')
const type = ref('all')
const limit = ref(100)

function formatTime(ts) {
  if (!ts) return '-'
  return new Date(ts * 1000).toLocaleString()
}

async function load() {
  loading.value = true
  try {
    const data = await getHistory({
      limit: limit.value,
      q: q.value.trim(),
      type: type.value,
      author: author.value.trim(),
    })
    items.value = data?.items || []
  } catch (e) {
    ElMessage.error(e.message || '读取作品库失败')
  } finally {
    loading.value = false
  }
}

function reset() {
  q.value = ''
  author.value = ''
  type.value = 'all'
  load()
}

function download(item) {
  const a = document.createElement('a')
  a.href = streamUrl(item.url, item.type === 'images' ? 'images' : undefined)
  a.download = ''
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
}

onMounted(load)
</script>

<template>
  <el-card shadow="never">
    <template #header>
      <div class="header-row">
        <div>
          <div class="title">作品库</div>
          <div class="hint">SQLite 中保存扫描发现的作品，可按标题、作者和类型筛选并重新下载。</div>
        </div>
        <el-button :loading="loading" @click="load">刷新</el-button>
      </div>
    </template>

    <div class="filters">
      <el-input v-model="q" clearable placeholder="搜索标题 / 作者 / aweme_id" @keyup.enter="load" />
      <el-input v-model="author" clearable placeholder="作者筛选" @keyup.enter="load" />
      <el-select v-model="type" class="type-select">
        <el-option label="全部类型" value="all" />
        <el-option label="视频" value="video" />
        <el-option label="图集" value="images" />
      </el-select>
      <el-select v-model="limit" class="limit-select">
        <el-option label="50 条" :value="50" />
        <el-option label="100 条" :value="100" />
        <el-option label="200 条" :value="200" />
        <el-option label="500 条" :value="500" />
      </el-select>
      <el-button type="primary" :loading="loading" @click="load">筛选</el-button>
      <el-button @click="reset">重置</el-button>
    </div>

    <el-empty v-if="!loading && items.length === 0" description="作品库里还没有记录" :image-size="80" />

    <div v-else v-loading="loading" class="library">
      <div v-for="item in items" :key="item.aweme_id" class="library-row">
        <div class="main">
          <div class="title-row">
            <strong :title="item.title">{{ item.title || item.aweme_id }}</strong>
            <el-tag size="small" effect="plain">{{ item.type === 'images' ? '图集' : '视频' }}</el-tag>
            <el-tag v-if="item.download_time" size="small" type="success" effect="plain">有下载记录</el-tag>
            <el-tag v-else size="small" type="info" effect="plain">已发现</el-tag>
          </div>
          <div class="meta">
            <span>{{ item.author_name || '作者未知' }}</span>
            <span>发布 {{ formatTime(item.create_time) }}</span>
            <span>aweme_id: {{ item.aweme_id }}</span>
          </div>
        </div>
        <el-button size="small" type="primary" plain @click="download(item)">下载</el-button>
      </div>
    </div>
  </el-card>
</template>

<style scoped>
.header-row,
.filters,
.library-row,
.title-row,
.meta {
  display: flex;
  align-items: center;
}
.header-row,
.library-row { justify-content: space-between; gap: 16px; }
.title { font-weight: 600; }
.hint,
.meta { color: #909399; font-size: 12px; }
.filters {
  gap: 10px;
  margin-bottom: 18px;
  flex-wrap: wrap;
}
.filters > .el-input { flex: 1; min-width: 180px; }
.type-select { width: 130px; }
.limit-select { width: 100px; }
.library { display: flex; flex-direction: column; gap: 9px; min-height: 80px; }
.library-row {
  padding: 12px 14px;
  border: 1px solid #ebeef5;
  border-radius: 9px;
}
.main { flex: 1; min-width: 0; }
.title-row { gap: 8px; min-width: 0; flex-wrap: wrap; }
.title-row strong {
  min-width: 0;
  max-width: 560px;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}
.meta { gap: 12px; margin-top: 6px; flex-wrap: wrap; }
@media (max-width: 600px) {
  .header-row,
  .library-row { align-items: flex-start; flex-direction: column; }
  .filters { align-items: stretch; flex-direction: column; }
  .filters > .el-input,
  .type-select,
  .limit-select { width: 100%; }
  .library-row > .el-button { width: 100%; }
}
</style>
