<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { NButton, useDialog, useMessage } from 'naive-ui'
import { useScheduleStore } from '../stores/schedule'
import { useAppStore } from '../stores/app'

interface VersionListItem {
  ID: number
  name: string
  source: string
  score: number
  entryCount: number
  solver?: string
  CreatedAt?: string
}

const scheduleStore = useScheduleStore()
const appStore = useAppStore()
const dialog = useDialog()
const message = useMessage()

const versions = ref<VersionListItem[]>([])
const isLoading = ref(false)

async function loadVersions() {
  isLoading.value = true
  try {
    const { ListVersions } = await import('../../bindings/scheduling-system/backend/services/versionservice')
    const data = await ListVersions(appStore.currentSemesterName)
    versions.value = (data || []) as VersionListItem[]
  } catch (e) {
    console.warn('Failed to load versions:', e)
    versions.value = []
  } finally {
    isLoading.value = false
  }
}

function viewVersion(id: number) {
  scheduleStore.loadVersionEntries(id).then(() => {
    appStore.navigateTo('schedule', scheduleStore.versionName || '历史版本查看')
  })
}

function confirmDelete(id: number) {
  dialog.warning({
    title: '确认删除',
    content: '确定要删除这个课表版本吗？此操作不可撤销。',
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        const { DeleteVersion } = await import('../../bindings/scheduling-system/backend/services/versionservice')
        await DeleteVersion(id)
        message.success('版本已删除')
        loadVersions()
      } catch {
        message.error('删除失败')
      }
    },
  })
}

function sourceLabel(src: string): string {
  const labels: Record<string, string> = {
    AutoGenerate: '自动排课',
    ManualAdjust: '手动调整',
    Import: '导入',
    Restore: '恢复',
    Copy: '复制',
  }
  return labels[src] || src
}

function formatTime(iso?: string): string {
  if (!iso) return ''
  const d = new Date(iso)
  return `${d.getMonth() + 1}/${d.getDate()} ${d.getHours().toString().padStart(2, '0')}:${d.getMinutes().toString().padStart(2, '0')}`
}

onMounted(loadVersions)
</script>

<template>
  <div class="schedule-center-page">
    <h2 class="page-title">课表中心</h2>
    <p class="page-desc">
      管理历史排课版本。点击「查看」浏览完整课表，点击「恢复」将版本设为当前课表（开发中）。
    </p>

    <div v-if="isLoading" class="loading">加载中...</div>

    <div v-else-if="versions.length === 0" class="empty-state">
      <p>暂无课表版本</p>
      <p class="empty-hint">运行自动排课后会自动创建版本，也可拖拽调整后手动保存。</p>
    </div>

    <div v-else class="version-list">
      <div
        v-for="v in versions"
        :key="v.ID"
        class="version-card"
        @click="viewVersion(v.ID)"
      >
        <div class="version-main">
          <div class="version-name">{{ v.name }}</div>
          <div class="version-badge" v-if="v.source">{{ sourceLabel(v.source) }}</div>
        </div>
        <div class="version-meta">
          <span class="meta-score" v-if="v.score != null">{{ v.score.toFixed(1) }} 分</span>
          <span class="meta-count">{{ v.entryCount }} 项</span>
          <span class="meta-time">{{ formatTime(v.CreatedAt) }}</span>
        </div>
        <div class="version-actions">
          <n-button size="tiny" type="primary" ghost @click.stop="viewVersion(v.ID)">查看</n-button>
          <n-button size="tiny" type="error" ghost @click.stop="confirmDelete(v.ID)">删除</n-button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.schedule-center-page {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.page-title {
  font-size: 18px;
  font-weight: 600;
  color: var(--b3-theme-on-background);
  margin-bottom: 6px;
}

.page-desc {
  font-size: 13px;
  color: var(--b3-theme-on-surface-light);
  margin-bottom: 20px;
}

.loading {
  text-align: center;
  color: var(--b3-theme-on-surface-light);
  padding: 40px;
  font-size: 14px;
}

.empty-state {
  text-align: center;
  padding: 60px 20px;
  color: var(--b3-theme-on-surface-light);
  font-size: 15px;
}

.empty-hint {
  font-size: 12px;
  margin-top: 8px;
  opacity: 0.7;
}

.version-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.version-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: var(--b3-theme-surface);
  border: 1px solid var(--b3-border-color);
  border-radius: var(--b3-border-radius);
  padding: 14px 16px;
  cursor: pointer;
  transition: background 0.12s, border-color 0.12s;
}

.version-card:hover {
  background: var(--b3-theme-primary-lightest);
  border-color: var(--b3-theme-primary-light);
}

.version-main {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.version-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--b3-theme-on-background);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.version-badge {
  font-size: 11px;
  color: var(--b3-theme-primary);
  background: var(--b3-theme-primary-lightest);
  padding: 1px 8px;
  border-radius: 3px;
  font-weight: 500;
  flex-shrink: 0;
}

.version-meta {
  display: flex;
  gap: 12px;
  flex-shrink: 0;
  font-size: 12px;
  color: var(--b3-theme-on-surface-light);
}

.meta-score {
  color: var(--b3-theme-primary);
  font-weight: 600;
}

.meta-count {
  color: var(--b3-theme-on-surface);
}

.meta-time {
  color: var(--b3-theme-on-surface-light);
}

.version-actions {
  display: flex;
  gap: 6px;
  flex-shrink: 0;
}
</style>
