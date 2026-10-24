<template>
  <section class="history">
    <header class="history__head">
      <div>
        <h1 class="history__title">历史记录</h1>
        <p class="history__desc">过去每次处理的记录；没保存（下载）过的图片还能再保存。</p>
      </div>
      <button class="history__refresh" type="button" @click="refresh">刷新</button>
    </header>

    <p v-if="error" class="history__error" role="alert">{{ error.message }}</p>
    <p v-else-if="loading" class="history__hint">正在读取…</p>
    <p v-else-if="list.length === 0" class="history__hint">还没有处理过图片。</p>

    <ul v-else class="history__list">
      <li v-for="batch in list" :key="batch.id" class="batch" :class="{ 'batch--open': openKey === batch.batch_key }">
        <button class="batch__head" type="button" @click="toggleBatch(batch)">
          <span class="batch__time">{{ formatTime(batch.created_at) }}</span>
          <span class="batch__summary">{{ batch.summary }}</span>
          <span class="batch__counts">{{ countsText(batch) }}</span>
          <span v-if="batch.files_purged" class="batch__purged">图片已清理</span>
          <span class="batch__chevron" aria-hidden="true">{{ openKey === batch.batch_key ? '▾' : '▸' }}</span>
        </button>

        <div v-if="openKey === batch.batch_key" class="batch__detail">
          <p v-if="itemsLoading" class="history__hint">正在读取明细…</p>
          <p v-else-if="itemsError" class="history__error">{{ itemsError.message }}</p>
          <template v-else>
            <div class="batch__actions">
              <button
                v-if="downloadablePaths.length > 0"
                class="batch__download"
                type="button"
                @click="downloadAll"
              >
                全部下载（{{ downloadablePaths.length }}）
              </button>
              <span v-else class="history__hint">这批没有可保存的图片。</span>
            </div>
            <ul class="items">
              <li v-for="item in items" :key="item.id" class="item">
                <div class="item__thumb checkerboard">
                  <img v-if="item.thumbnail" :src="item.thumbnail" alt="" />
                  <span v-else class="item__mark">{{ item.status === 'success' ? '' : '!' }}</span>
                </div>
                <div class="item__main">
                  <p class="item__name" :title="item.source_path">{{ item.source_name }}</p>
                  <p class="item__meta">
                    {{ formatBytes(item.source_bytes) }}
                    <template v-if="item.status === 'success'">
                      → {{ formatBytes(item.output_bytes) }} · {{ item.width }} × {{ item.height }}
                    </template>
                  </p>
                  <p v-if="item.message" class="item__note" :title="item.message">{{ item.message }}</p>
                </div>
                <span class="item__status" :class="statusClass(item.status)">{{ statusText(item.status) }}</span>
                <span v-if="item.downloaded" class="item__saved">已保存过</span>
                <button
                  v-if="canDownloadItem(item)"
                  class="item__download"
                  type="button"
                  @click="downloadOne(item)"
                >
                  下载
                </button>
              </li>
            </ul>
          </template>
        </div>
      </li>
    </ul>

    <!-- 复用工作台的保存选项框（D-58：单张与全部共用）。 -->
    <DownloadDialog
      :open="downloadOpen"
      :groups="[{ batchKey: openKey, paths: downloadPaths }]"
      :sample-name="downloadSample"
      @close="downloadOpen = false"
      @downloaded="onDownloaded"
    />

    <footer v-if="total > pageSize" class="history__pager">
      <button class="history__refresh" type="button" :disabled="page <= 1" @click="loadPage(page - 1)">
        上一页
      </button>
      <span class="history__hint">第 {{ page }} / {{ Math.ceil(total / pageSize) }} 页</span>
      <button
        class="history__refresh"
        type="button"
        :disabled="page >= Math.ceil(total / pageSize)"
        @click="loadPage(page + 1)"
      >
        下一页
      </button>
    </footer>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ListHistoryBatches, ListHistoryItems } from '../../../wailsjs/go/main/App'
import { usePagedList } from '../../composables/usePagedList'
import { invoke, type AppError } from '../../lib/invoke'
import { newPageRequest, PageSize } from '../../lib/page'
import DownloadDialog from '../image/DownloadDialog.vue'
import { formatBytes } from '../image/format'

// 历史条目类型直接取绑定返回值（wails 生成物），不手写镜像。
type HistoryBatch = Awaited<ReturnType<typeof ListHistoryBatches>>['list'][number]
type HistoryItem = Awaited<ReturnType<typeof ListHistoryItems>>['list'][number]

// 历史记录（US-62~64）：一次处理一条；点开看明细，可把没保存的图片再保存。
const {
  list,
  total,
  page,
  pageSize,
  loading,
  error,
  load,
} = usePagedList<HistoryBatch>((req) => ListHistoryBatches(req), {
  pageSize: PageSize.Default,
  label: 'ListHistoryBatches',
})

const openKey = ref('')
const items = ref<HistoryItem[]>([])
const itemsLoading = ref(false)
const itemsError = ref<AppError | null>(null)
const openPurged = ref(false)

const downloadOpen = ref(false)
const downloadPaths = ref<string[]>([])
const downloadSample = ref('photo.jpg')

const downloadablePaths = computed(() =>
  items.value
    .filter((item) => item.status === 'success' && item.staging_name !== '' && !openPurged.value)
    .map((item) => item.source_path),
)

function refresh() {
  void load()
}

function loadPage(next: number) {
  void load({ page: next })
}

function toggleBatch(batch: HistoryBatch) {
  if (openKey.value === batch.batch_key) {
    openKey.value = ''
    return
  }
  openKey.value = batch.batch_key
  openPurged.value = batch.files_purged
  void loadItems()
}

async function loadItems() {
  if (!openKey.value) return
  itemsLoading.value = true
  itemsError.value = null
  try {
    const result = await invoke(
      () => ListHistoryItems(openKey.value, newPageRequest(1, PageSize.Max)),
      'ListHistoryItems',
    )
    items.value = result.list ?? []
  } catch (e) {
    itemsError.value = e as AppError
    items.value = []
  } finally {
    itemsLoading.value = false
  }
}

function canDownloadItem(item: HistoryItem) {
  return item.status === 'success' && item.staging_name !== '' && !openPurged.value
}

function downloadOne(item: HistoryItem) {
  if (!canDownloadItem(item)) return
  downloadSample.value = item.source_name
  downloadPaths.value = [item.source_path]
  downloadOpen.value = true
}

function downloadAll() {
  const paths = downloadablePaths.value
  if (paths.length === 0) return
  downloadSample.value = items.value.find((item) => item.source_path === paths[0])?.source_name ?? 'photo.jpg'
  downloadPaths.value = paths
  downloadOpen.value = true
}

function onDownloaded() {
  downloadOpen.value = false
  // 重新拉明细，让「已保存过」标记即时反映。
  void loadItems()
}

function formatTime(unix: number): string {
  const d = new Date(unix * 1000)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

function countsText(batch: HistoryBatch): string {
  const parts = [`已完成 ${batch.success}`]
  if (batch.failed > 0) parts.push(`失败 ${batch.failed}`)
  if (batch.skipped > 0) parts.push(`无需处理 ${batch.skipped}`)
  if (batch.canceled > 0) parts.push(`已取消 ${batch.canceled}`)
  return parts.join(' · ')
}

// 状态词与 GLOSSARY/QueueRow 同口径（展示映射，样式各异故未抽共享表）。
function statusText(status: string): string {
  switch (status) {
    case 'success': return '已完成'
    case 'failed': return '失败'
    case 'skipped': return '无需处理'
    case 'canceled': return '已取消'
    default: return status
  }
}

function statusClass(status: string): string {
  switch (status) {
    case 'success': return 'item__status--ok'
    case 'failed': return 'item__status--failed'
    case 'skipped': return 'item__status--skipped'
    default: return 'item__status--muted'
  }
}

onMounted(refresh)
</script>

<style scoped>
.history {
  height: 100%;
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  overflow-y: auto;
  padding-right: var(--space-2);
}

.history__head {
  flex: none;
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--space-4);
}

.history__title {
  margin: 0 0 var(--space-2);
  font-size: var(--font-size-xl);
}

.history__desc {
  margin: 0;
  color: var(--text-2);
  font-size: var(--font-size-sm);
}

.history__hint {
  margin: 0;
  color: var(--text-3);
  font-size: var(--font-size-sm);
}

.history__error {
  margin: 0;
  color: var(--color-danger);
  font-size: var(--font-size-sm);
}

.history__refresh {
  flex: none;
  border: 1px solid var(--border-subtle);
  background: var(--bg-raised);
  color: var(--text-2);
  padding: var(--space-2) var(--space-4);
  border-radius: var(--radius-md);
  font-family: inherit;
  font-size: var(--font-size-sm);
  cursor: pointer;
}

.history__refresh:hover:not(:disabled) {
  border-color: var(--color-primary-border);
  color: var(--text-1);
}

.history__refresh:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.history__list {
  margin: 0;
  padding: 0;
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}

.batch {
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  background: var(--bg-raised);
  overflow: hidden;
}

.batch--open {
  border-color: var(--color-primary-border);
}

.batch__head {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  width: 100%;
  padding: var(--space-3) var(--space-4);
  border: none;
  background: transparent;
  cursor: pointer;
  font-family: inherit;
  text-align: left;
}

.batch__head:hover {
  background: var(--bg-hover);
}

.batch__time {
  flex: none;
  color: var(--text-2);
  font-size: var(--font-size-sm);
  font-variant-numeric: tabular-nums;
}

.batch__summary {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--text-1);
  font-size: var(--font-size-sm);
}

.batch__counts {
  flex: none;
  color: var(--text-3);
  font-size: var(--font-size-xs);
}

.batch__purged {
  flex: none;
  padding: 2px var(--space-2);
  border-radius: var(--radius-full);
  background: var(--bg-hover);
  color: var(--text-3);
  font-size: var(--font-size-xs);
}

.batch__chevron {
  flex: none;
  color: var(--text-3);
}

.batch__detail {
  border-top: 1px solid var(--border-subtle);
  padding: var(--space-3) var(--space-4);
}

.batch__actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  margin-bottom: var(--space-3);
}

.batch__download {
  border: 1px solid var(--color-primary);
  background: var(--color-primary-soft);
  color: var(--color-primary);
  padding: var(--space-1) var(--space-3);
  border-radius: var(--radius-sm);
  font-family: inherit;
  font-size: var(--font-size-sm);
  cursor: pointer;
}

.items {
  margin: 0;
  padding: 0;
  list-style: none;
  display: flex;
  flex-direction: column;
}

.item {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-2) 0;
  border-bottom: 1px solid var(--border-subtle);
}

.item:last-child {
  border-bottom: none;
}

.item__thumb {
  flex: none;
  width: 44px;
  height: 44px;
  overflow: hidden;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--color-danger);
  font-size: var(--font-size-lg);
}

.item__thumb img {
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.item__main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.item__name {
  margin: 0;
  color: var(--text-1);
  font-size: var(--font-size-sm);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.item__meta {
  margin: 0;
  color: var(--text-3);
  font-size: var(--font-size-xs);
  font-variant-numeric: tabular-nums;
}

.item__note {
  margin: 0;
  color: var(--color-warning);
  font-size: var(--font-size-xs);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.item__status {
  flex: none;
  font-size: var(--font-size-xs);
  color: var(--text-2);
}

.item__status--ok {
  color: var(--color-success);
}

.item__status--failed {
  color: var(--color-danger);
}

.item__status--skipped {
  color: var(--color-warning);
}

.item__status--muted {
  color: var(--text-3);
}

.item__saved {
  flex: none;
  padding: 2px var(--space-2);
  border-radius: var(--radius-full);
  background: var(--bg-hover);
  color: var(--text-3);
  font-size: var(--font-size-xs);
}

.item__download {
  flex: none;
  border: 1px solid var(--border-subtle);
  background: transparent;
  color: var(--color-success);
  font-family: inherit;
  font-size: var(--font-size-sm);
  padding: var(--space-1) var(--space-2);
  border-radius: var(--radius-sm);
  cursor: pointer;
}

.item__download:hover {
  border-color: var(--color-success);
}

.history__pager {
  flex: none;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: var(--space-3);
}
</style>
