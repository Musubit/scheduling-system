<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { NButton, NSelect, NEmpty, NSpin, useDialog, useMessage } from 'naive-ui'
import { useAppStore } from '../stores/app'
import { useScheduleStore } from '../stores/schedule'
import {
  ListVersions,
  GetVersionWithDetails,
  CompareVersions,
  DeleteVersion,
  RestoreVersion,
  RenameVersion,
  CreateManualReport,
  ClearSemesterVersions,
} from '../../bindings/scheduling-system/backend/services/versionservice'
import type { ScheduleVersion } from '../../bindings/scheduling-system/backend/models/models'
import VersionReport from '../components/version/VersionReport.vue'
import { scoreColor } from '../utils/score'

const appStore = useAppStore()
const scheduleStore = useScheduleStore()
const dialog = useDialog()
const message = useMessage()

// ---- Version list ----
const versions = ref<ScheduleVersion[]>([])
const loading = ref(false)
const selectedId = ref<number | null>(null)
const clearing = ref(false)

// ---- Perspective (报告 | 对比) ----
type Perspective = 'report' | 'compare'
const perspective = ref<Perspective>('report')
const perspectives: { label: string; value: Perspective }[] = [
  { label: '报告', value: 'report' },
  { label: '对比', value: 'compare' },
]

// ---- Compare state ----
const compareTargetId = ref<number | null>(null)
const comparing = ref(false)
const compareResult = ref<any | null>(null)

const compareOptions = computed(() =>
  versions.value
    .filter(v => v.ID !== selectedId.value)
    .map(v => ({ label: v.name || '未命名', value: v.ID }))
)

// ---- Inline rename ----
const editingId = ref<number | null>(null)
const editName = ref('')

// ---- Data loading ----
async function loadVersions() {
  loading.value = true
  try {
    const data = await ListVersions(appStore.currentSemesterId)
    versions.value = data || []
    if (!selectedId.value && versions.value.length > 0) {
      selectedId.value = versions.value[0].ID
    }
  } catch {
    versions.value = []
  } finally {
    loading.value = false
  }
}

function selectVersion(id: number) {
  selectedId.value = id
  compareResult.value = null
}

// Auto-select compare target when switching to compare tab
watch(perspective, (p) => {
  if (p === 'compare' && !compareTargetId.value && versions.value.length > 1) {
    const current = versions.value.find(v => v.ID === selectedId.value)
    const idx = current ? versions.value.indexOf(current) : -1
    // Pick the next older version
    const target = versions.value[idx + 1] || versions.value.find(v => v.ID !== selectedId.value)
    if (target) compareTargetId.value = target.ID
  }
})

// ---- Actions ----
async function handleCompare() {
  if (!selectedId.value || !compareTargetId.value) {
    message.warning('请选择两个不同的版本')
    return
  }
  comparing.value = true
  compareResult.value = null
  try {
    const res = await CompareVersions(selectedId.value, compareTargetId.value)
    compareResult.value = res
  } catch (e: any) {
    message.error('对比失败: ' + (e?.message || e))
  } finally {
    comparing.value = false
  }
}

function confirmRestore(id: number, name: string) {
  dialog.warning({
    title: '恢复版本',
    content: `确定要将「${name}」恢复为当前课表吗？当前课表将被替换。`,
    positiveText: '恢复',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await RestoreVersion(id)
        message.success('版本已恢复')
        await loadVersions()
        await scheduleStore.loadSchedule(appStore.currentSemesterId)
      } catch (e: any) {
        message.error('恢复失败: ' + (e?.message || e))
      }
    },
  })
}

function confirmDelete(id: number) {
  dialog.warning({
    title: '确认删除',
    content: '确定要删除这个版本吗？此操作不可撤销。',
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await DeleteVersion(id)
        if (selectedId.value === id) selectedId.value = null
        message.success('已删除')
        await loadVersions()
      } catch {
        message.error('删除失败')
      }
    },
  })
}

function confirmClearAll() {
  dialog.warning({
    title: '确认清空',
    content: `确定要删除全部 ${versions.value.length} 个版本吗？此操作不可撤销。`,
    positiveText: '确认清空',
    negativeText: '取消',
    positiveButtonProps: { type: 'error' },
    onPositiveClick: async () => {
      clearing.value = true
      try {
        await ClearSemesterVersions(appStore.currentSemesterId)
        selectedId.value = null
        message.success('已清空')
        await loadVersions()
      } catch {
        message.error('清空失败')
      } finally {
        clearing.value = false
      }
    },
  })
}

async function generateReport() {
  try {
    loading.value = true
    await CreateManualReport(appStore.currentSemesterId)
    await loadVersions()
  } catch (e: any) {
    message.error('生成失败: ' + (e?.message || e))
  } finally {
    loading.value = false
  }
}

// ---- Rename ----
function startEditing(id: number, name: string) {
  editingId.value = id
  editName.value = name || ''
}

function cancelEditing() {
  editingId.value = null
  editName.value = ''
}

async function commitRename(id: number) {
  const newName = editName.value.trim()
  if (!newName || newName.length > 100) { cancelEditing(); return }
  const v = versions.value.find(v => v.ID === id)
  if (!v || newName === (v.name || '').trim()) { cancelEditing(); return }
  try {
    await RenameVersion(id, newName)
    cancelEditing()
    message.success('已重命名')
    await loadVersions()
  } catch (e: any) {
    message.error('重命名失败: ' + (e?.message || e))
  }
}

function handleRenameKeydown(e: KeyboardEvent, id: number) {
  if (e.key === 'Enter') { e.preventDefault(); commitRename(id) }
  else if (e.key === 'Escape') { e.preventDefault(); cancelEditing() }
}

// ---- View in main schedule ----
async function viewInSchedule(id: number) {
  await scheduleStore.loadVersionEntries(id)
  appStore.navigateTo('schedule', '版本查看')
}

// ---- Helpers ----
function sourceLabel(src: string): string {
  const map: Record<string, string> = {
    AutoGenerate: '自动排课', ManualAdjust: '手动调整',
    Import: '导入', Restore: '恢复', Copy: '复制',
  }
  return map[src] || src
}

function formatTime(iso?: string): string {
  if (!iso) return ''
  const d = new Date(iso)
  return `${d.getMonth() + 1}/${d.getDate()} ${d.getHours().toString().padStart(2, '0')}:${d.getMinutes().toString().padStart(2, '0')}`
}

// Entry diff helpers
const DAY_NAMES = ['周一', '周二', '周三', '周四', '周五', '周六', '周日']

onMounted(loadVersions)
</script>

<template>
  <div class="version-panel-page">
    <!-- Left sidebar: version list -->
    <div class="version-sidebar">
      <div class="sidebar-header">
        <span class="sidebar-title">版本列表</span>
        <div class="sidebar-actions">
          <NButton size="tiny" type="primary" @click="generateReport" :loading="loading">生成</NButton>
          <NButton size="tiny" type="error" ghost @click="confirmClearAll" :disabled="versions.length === 0" :loading="clearing">清空</NButton>
        </div>
      </div>

      <NSpin :show="loading">
        <NEmpty v-if="!loading && versions.length === 0" description="暂无版本" size="small" class="sidebar-empty" />

        <div v-else class="version-list">
          <div
            v-for="v in versions"
            :key="v.ID"
            class="version-item"
            :class="{ active: selectedId === v.ID }"
            @click="selectVersion(v.ID)"
          >
            <div class="version-item-main">
              <!-- Inline rename -->
              <div v-if="editingId === v.ID" class="rename-inline" @click.stop>
                <input
                  v-model="editName"
                  class="rename-input"
                  maxlength="100"
                  @keydown="handleRenameKeydown($event, v.ID)"
                  @blur="commitRename(v.ID)"
                  autofocus
                />
              </div>
              <div v-else class="version-name" :title="'双击重命名'" @dblclick.stop="startEditing(v.ID, v.name)">
                {{ v.name || '未命名' }}
              </div>
              <div class="version-meta">
                <span class="meta-score" :style="{ color: scoreColor(v.finalScore ?? v.totalScore) }">
                  {{ (v.finalScore ?? v.totalScore)?.toFixed(1) }}分
                </span>
                <span class="meta-count">{{ v.entryCount }}项</span>
                <span class="meta-source" v-if="v.source">{{ sourceLabel(v.source) }}</span>
              </div>
            </div>
            <div class="version-item-actions">
              <NButton size="tiny" quaternary type="info" @click.stop="viewInSchedule(v.ID)">查看课表</NButton>
              <NButton size="tiny" quaternary type="info" @click.stop="startEditing(v.ID, v.name)">改名</NButton>
              <NButton size="tiny" quaternary type="warning" @click.stop="confirmRestore(v.ID, v.name)">恢复</NButton>
              <NButton size="tiny" quaternary type="error" @click.stop="confirmDelete(v.ID)">删除</NButton>
            </div>
          </div>
        </div>
      </NSpin>
    </div>

    <!-- Right content area -->
    <div class="version-content">
      <!-- Header -->
      <div class="content-header">
        <h2 class="content-title">课表方案</h2>
        <div class="content-spacer" />
        <span class="content-version-name" v-if="selectedId">
          {{ versions.find(v => v.ID === selectedId)?.name || '' }}
        </span>
      </div>

      <!-- Perspective bar -->
      <div class="perspective-bar">
        <div class="perspective-tabs">
          <button
            v-for="p in perspectives"
            :key="p.value"
            class="perspective-tab"
            :class="{ active: perspective === p.value }"
            @click="perspective = p.value"
          >{{ p.label }}</button>
        </div>
        <div class="perspective-filters">
          <!-- Compare controls -->
          <template v-if="perspective === 'compare'">
            <span class="compare-label">对比目标:</span>
            <NSelect
              v-model:value="compareTargetId"
              :options="compareOptions"
              placeholder="选择对比版本"
              size="small"
              style="width: 200px"
              clearable
            />
            <NButton size="small" type="primary" @click="handleCompare" :loading="comparing" :disabled="!compareTargetId">
              开始对比
            </NButton>
          </template>
        </div>
      </div>

      <!-- Content -->
      <div class="content-body">
        <NEmpty v-if="!selectedId" description="请在左侧选择一个版本" class="content-empty" />

        <!-- Report tab -->
        <div v-else-if="perspective === 'report'" class="tab-report">
          <VersionReport :version-id="selectedId" />
        </div>

        <!-- Compare tab -->
        <div v-else-if="perspective === 'compare'" class="tab-compare">
          <NEmpty v-if="!compareResult" description="选择对比目标并点击「开始对比」" />

          <template v-else-if="compareResult">
            <!-- Overall delta -->
            <div class="compare-overview">
              <div class="delta-card">
                <span class="delta-label">总分</span>
                <span class="delta-value" :style="{ color: scoreColor((compareResult.a?.finalScore ?? 0) + compareResult.scoreDelta) }">
                  {{ (compareResult.a?.finalScore ?? compareResult.a?.totalScore)?.toFixed(1) }}
                  →
                  {{ (compareResult.b?.finalScore ?? compareResult.b?.totalScore)?.toFixed(1) }}
                  <span class="delta-diff" :style="{ color: compareResult.scoreDelta > 0 ? '#18a058' : compareResult.scoreDelta < 0 ? '#d03050' : 'inherit' }">
                    {{ compareResult.scoreDelta > 0 ? '+' : '' }}{{ compareResult.scoreDelta?.toFixed(1) }}
                  </span>
                </span>
              </div>
              <div class="delta-card">
                <span class="delta-label">冲突</span>
                <span class="delta-value">
                  <span v-if="compareResult.conflictDelta > 0" style="color:#18a058">已解决 ✓</span>
                  <span v-else-if="compareResult.conflictDelta < 0" style="color:#d03050">新增 ✗</span>
                  <span v-else>无变化</span>
                </span>
              </div>
              <div class="delta-card">
                <span class="delta-label">课时</span>
                <span class="delta-value">
                  {{ compareResult.a?.entryCount }} → {{ compareResult.b?.entryCount }}
                  <span :style="{ color: compareResult.entryDelta > 0 ? '#18a058' : compareResult.entryDelta < 0 ? '#d03050' : 'inherit' }">
                    ({{ compareResult.entryDelta > 0 ? '+' : '' }}{{ compareResult.entryDelta }})
                  </span>
                </span>
              </div>
            </div>

            <!-- Entry diffs -->
            <div class="compare-section" v-if="compareResult.entryDiffs?.length">
              <h3 class="section-title">课表变化</h3>
              <div class="entry-diff-list">
                <div v-for="d in compareResult.entryDiffs" :key="d.type + d.taskId" class="entry-diff-row">
                  <span class="diff-type" :class="d.type">
                    {{ d.type === 'added' ? '➕' : d.type === 'removed' ? '➖' : '✏️' }}
                  </span>
                  <span class="diff-course">{{ d.course }}</span>
                  <span class="diff-teacher">{{ d.teacher }}</span>
                  <span class="diff-move" v-if="d.type === 'moved'">
                    {{ DAY_NAMES[d.oldDay] }} 第{{ d.oldStart + 1 }}节 → {{ DAY_NAMES[d.newDay] }} 第{{ d.newStart + 1 }}节
                  </span>
                  <span class="diff-move" v-else-if="d.type === 'added'">
                    {{ DAY_NAMES[d.newDay] }} 第{{ d.newStart + 1 }}节
                  </span>
                  <span class="diff-move" v-else>
                    {{ DAY_NAMES[d.oldDay] }} 第{{ d.oldStart + 1 }}节
                  </span>
                </div>
              </div>
            </div>

            <!-- Teacher diffs -->
            <div class="compare-section" v-if="compareResult.teacherDiffs?.length">
              <h3 class="section-title">教师变化</h3>
              <div class="teacher-diff-list">
                <div class="teacher-diff-header">
                  <span>教师</span><span>课时</span><span>早课</span><span>晚课</span><span>到校天数</span><span>状态</span>
                </div>
                <div v-for="t in compareResult.teacherDiffs" :key="t.code" class="teacher-diff-row">
                  <span>{{ t.name }}（{{ t.code }}）</span>
                  <span>{{ t.entryDelta > 0 ? '+' : '' }}{{ t.entryDelta }}</span>
                  <span :class="{ warn: t.earlyDelta > 0 }">{{ t.earlyDelta > 0 ? '+' : '' }}{{ t.earlyDelta }}</span>
                  <span :class="{ warn: t.lateDelta > 0 }">{{ t.lateDelta > 0 ? '+' : '' }}{{ t.lateDelta }}</span>
                  <span>{{ t.daysActualA }} → {{ t.daysActualB }}（{{ t.daysTarget }}）</span>
                  <span class="status-tag" :class="t.status">
                    {{ ({ improved: '改善', regressed: '退化', unchanged: '不变', added: '新增', removed: '移除' } as Record<string,string>)[t.status] || t.status }}
                  </span>
                </div>
              </div>
            </div>
          </template>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.version-panel-page {
  display: flex;
  gap: 20px;
  height: 100%;
  min-height: 0;
}

/* ---- Left sidebar ---- */
.version-sidebar {
  width: 260px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  border: 1px solid var(--b3-border-color);
  border-radius: var(--b3-border-radius);
  background: var(--b3-theme-surface);
  overflow: hidden;
}

.sidebar-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 14px;
  border-bottom: 1px solid var(--b3-border-color);
  flex-shrink: 0;
}

.sidebar-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--b3-theme-on-background);
}

.sidebar-actions {
  display: flex;
  gap: 6px;
}

.sidebar-empty {
  padding: 40px 16px;
}

.version-list {
  flex: 1;
  overflow-y: auto;
  padding: 8px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.version-item {
  padding: 10px 12px;
  border-radius: 6px;
  border: 1px solid transparent;
  cursor: pointer;
  transition: all 0.15s;
}

.version-item:hover {
  background: var(--b3-theme-primary-lightest);
  border-color: var(--b3-theme-primary-light);
}

.version-item.active {
  background: var(--b3-theme-primary-lightest);
  border-color: var(--b3-theme-primary);
}

.version-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--b3-theme-on-background);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  margin-bottom: 4px;
}

.rename-inline { margin-bottom: 4px; }
.rename-input {
  width: 100%;
  font-size: 13px;
  padding: 2px 6px;
  border: 1px solid var(--b3-theme-primary);
  border-radius: 4px;
  outline: none;
  background: var(--b3-theme-background);
  color: var(--b3-theme-on-background);
}

.version-meta {
  display: flex;
  gap: 8px;
  font-size: 11px;
  color: var(--b3-theme-on-surface-light);
}

.meta-score { font-weight: 700; }
.meta-source {
  color: var(--b3-theme-primary);
  background: var(--b3-theme-primary-lightest);
  padding: 0 4px;
  border-radius: 2px;
}

.version-item-actions {
  display: flex;
  gap: 4px;
  margin-top: 6px;
  opacity: 0;
  transition: opacity 0.15s;
}

.version-item:hover .version-item-actions { opacity: 1; }

/* ---- Right content ---- */
.version-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
}

.content-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
  flex-shrink: 0;
}

.content-title {
  font-size: 18px;
  font-weight: 600;
  color: var(--b3-theme-on-background);
  margin: 0;
}

.content-spacer { flex: 1; }

.content-version-name {
  font-size: 13px;
  color: var(--b3-theme-on-surface-light);
}

/* ---- Perspective bar ---- */
.perspective-bar {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 16px;
  flex-shrink: 0;
}

.perspective-tabs {
  display: flex;
  gap: 2px;
  background: var(--b3-theme-surface);
  border: 1px solid var(--b3-border-color);
  border-radius: 6px;
  padding: 2px;
}

.perspective-tab {
  padding: 6px 16px;
  font-size: 13px;
  font-weight: 500;
  color: var(--b3-theme-on-surface);
  background: transparent;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  transition: all 0.15s;
  white-space: nowrap;
}

.perspective-tab:hover {
  color: var(--b3-theme-primary);
  background: var(--b3-theme-primary-lightest);
}

.perspective-tab.active {
  color: #fff;
  background: var(--b3-theme-primary);
}

.perspective-filters {
  display: flex;
  align-items: center;
  gap: 8px;
}

.compare-label {
  font-size: 12px;
  color: var(--b3-theme-on-surface-light);
}

/* ---- Content body ---- */
.content-body {
  flex: 1;
  overflow-y: auto;
  min-height: 0;
}

.content-empty {
  padding: 80px 0;
}

/* ---- Compare ---- */
.compare-overview {
  display: flex;
  gap: 16px;
  margin-bottom: 20px;
}

.delta-card {
  flex: 1;
  padding: 14px 16px;
  background: var(--b3-theme-surface);
  border: 1px solid var(--b3-border-color);
  border-radius: 8px;
  text-align: center;
}

.delta-label {
  display: block;
  font-size: 12px;
  color: var(--b3-theme-on-surface-light);
  margin-bottom: 6px;
}

.delta-value {
  font-size: 15px;
  font-weight: 600;
  color: var(--b3-theme-on-background);
}

.delta-diff {
  font-weight: 700;
  margin-left: 4px;
}

.compare-section {
  margin-bottom: 20px;
}

.section-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--b3-theme-on-background);
  margin-bottom: 10px;
  padding-bottom: 6px;
  border-bottom: 1px solid var(--b3-border-color);
}

.entry-diff-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.entry-diff-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 10px;
  font-size: 13px;
  border-radius: 4px;
  background: var(--b3-theme-surface);
}

.diff-type { font-size: 14px; width: 20px; text-align: center; flex-shrink: 0; }
.diff-course { font-weight: 600; min-width: 80px; }
.diff-teacher { color: var(--b3-theme-on-surface-light); min-width: 60px; }
.diff-move { color: var(--b3-theme-on-surface); font-size: 12px; }

.teacher-diff-list {
  font-size: 13px;
}

.teacher-diff-header,
.teacher-diff-row {
  display: flex;
  gap: 8px;
  padding: 6px 10px;
  border-bottom: 1px solid var(--b3-border-color);
}

.teacher-diff-header {
  font-weight: 600;
  color: var(--b3-theme-on-surface-light);
  font-size: 12px;
}

.teacher-diff-header span,
.teacher-diff-row span {
  flex: 1;
  text-align: center;
}

.teacher-diff-row span:first-child {
  flex: 1.5;
  text-align: left;
}

.warn { color: #d03050; font-weight: 600; }

.status-tag {
  font-size: 11px;
  padding: 1px 6px;
  border-radius: 3px;
  font-weight: 600;
}
.status-tag.improved { color: #18a058; background: rgba(24,160,88,0.1); }
.status-tag.regressed { color: #d03050; background: rgba(208,48,80,0.1); }
.status-tag.unchanged { color: var(--b3-theme-on-surface-light); }
.status-tag.added { color: #3575f0; background: rgba(53,117,240,0.1); }
.status-tag.removed { color: #f0a020; background: rgba(240,160,32,0.1); }
</style>
