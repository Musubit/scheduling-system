<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { NTag, NButton, NEmpty, NSpin, NProgress, NCard } from 'naive-ui'
import { useAppStore } from '../stores/app'

const appStore = useAppStore()

// ---- State ----
const loading = ref(false)
const snapshots = ref<any[]>([])
const selectedSnapshot = ref<any | null>(null)

// ---- Data ----
interface Snapshot {
  ID: number
  semester: string
  dept: string
  trigger: string
  hardPassed: boolean
  totalScore: number
  teacherPref: number
  courseSpacing: number
  teacherDays: number
  lowFloorPref: number
  weekendAvoid?: number
  pePeriodPref?: number
  studentFatigue?: number
  totalEntries: number
  solveTimeMs: number
  solver: string
  createdAt: string
  details?: SnapshotDetail[]
}

interface SnapshotDetail {
  ID: number
  entityType: string
  entityCode: string
  entityName: string
  earlyPenalty: number
  latePenalty: number
  daysActual: number
  daysTarget: number
  avgFloor: number
  entryCount: number
  daysCount: number
  summary: string
}

// ---- Actions ----
async function loadSnapshots() {
  loading.value = true
  try {
    // Dynamic import of Go binding — works at build time
    const { GetSnapshots } = await import('../../bindings/scheduling-system/backend/services/snapshotservice')
    const result = await GetSnapshots(appStore.semesterFilter)
    snapshots.value = result || []
    if (snapshots.value.length > 0 && !selectedSnapshot.value) {
      selectSnapshot(snapshots.value[0])
    }
  } catch {
    snapshots.value = []
  } finally {
    loading.value = false
  }
}

async function loadSnapshotDetail(snapshot: any) {
  try {
    const { GetSnapshotWithDetails } = await import('../../bindings/scheduling-system/backend/services/snapshotservice')
    const detail = await GetSnapshotWithDetails(snapshot.ID)
    selectedSnapshot.value = detail
  } catch {
    selectedSnapshot.value = snapshot
  }
}

function selectSnapshot(snapshot: any) {
  if (snapshot.details && snapshot.details.length > 0) {
    selectedSnapshot.value = snapshot
  } else {
    loadSnapshotDetail(snapshot)
  }
}

async function generateManualReport() {
  try {
    loading.value = true
    const { CreateManualSnapshot } = await import('../../bindings/scheduling-system/backend/services/snapshotservice')
    await CreateManualSnapshot(appStore.semesterFilter)
    await loadSnapshots()
  } catch (e: any) {
    console.warn('Failed to generate manual report:', e)
  } finally {
    loading.value = false
  }
}

async function deleteSnapshot(snapshot: any) {
  try {
    const { DeleteSnapshot } = await import('../../bindings/scheduling-system/backend/services/snapshotservice')
    await DeleteSnapshot(snapshot.ID)
    if (selectedSnapshot.value?.ID === snapshot.ID) {
      selectedSnapshot.value = null
    }
    await loadSnapshots()
  } catch (e: any) {
    console.warn('Failed to delete snapshot:', e)
  }
}

async function deleteAllSnapshots() {
  if (!snapshots.value.length) return
  if (!confirm(`确定要删除全部 ${snapshots.value.length} 条验证报告吗？此操作不可撤销。`)) return
  try {
    loading.value = true
    const { DeleteSnapshot } = await import('../../bindings/scheduling-system/backend/services/snapshotservice')
    for (const snap of snapshots.value) {
      await DeleteSnapshot(snap.ID)
    }
    selectedSnapshot.value = null
    await loadSnapshots()
  } catch (e: any) {
    console.warn('Failed to delete all snapshots:', e)
  } finally {
    loading.value = false
  }
}

function exportPDF() {
  window.print()
}

// Compute perCategoryMax from the snapshot scores
const perCategoryMax = computed(() => {
  if (!selectedSnapshot.value) return 25
  let count = 0
  const s = selectedSnapshot.value
  if (s.teacherPref > 0) count++
  if (s.courseSpacing > 0) count++
  if (s.teacherDays > 0) count++
  if (s.lowFloorPref > 0) count++
  if ((s.weekendAvoid || 0) > 0) count++
  if ((s.pePeriodPref || 0) > 0) count++
  if ((s.studentFatigue || 0) > 0) count++
  return count > 0 ? Math.round(100 / count * 100) / 100 : 25
})

// Sort details by "拖后腿" (most penalties first)
const rankedDetails = computed(() => {
  if (!selectedSnapshot.value?.details) return []
  return [...selectedSnapshot.value.details].sort((a: SnapshotDetail, b: SnapshotDetail) => {
    const penaltyA = a.earlyPenalty + a.latePenalty + Math.max(0, a.daysActual - a.daysTarget)
    const penaltyB = b.earlyPenalty + b.latePenalty + Math.max(0, b.daysActual - b.daysTarget)
    return penaltyB - penaltyA
  })
})

const scoreColor = (score: number) => {
  if (score >= 80) return '#18a058'
  if (score >= 60) return '#f0a020'
  return '#d03050'
}

const triggerLabel = (trigger: string) => {
  return trigger === 'auto' ? '自动生成' : '手动生成'
}

const formatDate = (d: string) => {
  if (!d) return ''
  return new Date(d).toLocaleString('zh-CN')
}

onMounted(() => {
  loadSnapshots()
})
</script>

<template>
  <div class="report-page">
    <!-- Header -->
    <div class="report-header no-print">
      <h2 class="report-title">验证报告</h2>
      <div class="report-actions">
        <n-button type="primary" @click="generateManualReport">
          生成报告
        </n-button>
        <n-button @click="exportPDF" :disabled="!selectedSnapshot">
          导出 PDF
        </n-button>
        <n-button type="error" @click="deleteAllSnapshots" :disabled="snapshots.length === 0" secondary>
          一键清除
        </n-button>
      </div>
    </div>

    <n-spin :show="loading">
      <div v-if="snapshots.length === 0 && !loading" class="empty-state">
        <n-empty description="暂无排课快照，请先运行排课引擎">
          <template #extra>
            <n-button @click="loadSnapshots">刷新</n-button>
          </template>
        </n-empty>
      </div>

      <div v-else class="report-content">
        <!-- Snapshot list sidebar -->
        <div class="snapshot-list no-print">
          <div
            v-for="snap in snapshots"
            :key="snap.ID"
            class="snapshot-card"
            :class="{ active: selectedSnapshot?.ID === snap.ID }"
            @click="selectSnapshot(snap)"
          >
            <button class="snap-delete-btn" @click.stop="deleteSnapshot(snap)" title="删除此快照">×</button>
            <div class="snap-date">{{ formatDate(snap.CreatedAt || snap.createdAt) }}</div>
            <div class="snap-meta">
              <n-tag :type="snap.trigger === 'auto' ? 'info' : 'warning'" size="small">
                {{ triggerLabel(snap.trigger) }}
              </n-tag>
              <span class="snap-score" :style="{ color: scoreColor(snap.totalScore) }">
                {{ snap.totalScore?.toFixed(1) }}分
              </span>
            </div>
          </div>
        </div>

        <!-- Detail area -->
        <div v-if="selectedSnapshot" class="detail-area">
          <!-- Score overview -->
          <n-card title="综合评分" size="small" class="score-card">
            <div class="total-score" :style="{ color: scoreColor(selectedSnapshot.totalScore) }">
              {{ selectedSnapshot.totalScore?.toFixed(1) }}
              <span class="score-unit">/ 100</span>
            </div>
            <div class="score-bars">
              <div class="score-bar-item">
                <span>教师偏好</span>
                <n-progress
                  type="line"
                  :percentage="Math.round((selectedSnapshot.teacherPref / perCategoryMax) * 1000) / 10"
                  :color="scoreColor(selectedSnapshot.teacherPref / perCategoryMax * 100)"
                  :height="16"
                />
                <span class="bar-value">{{ selectedSnapshot.teacherPref?.toFixed(1) }}</span>
              </div>
              <div class="score-bar-item">
                <span>课程间隔</span>
                <n-progress
                  type="line"
                  :percentage="Math.round((selectedSnapshot.courseSpacing / perCategoryMax) * 1000) / 10"
                  :color="scoreColor(selectedSnapshot.courseSpacing / perCategoryMax * 100)"
                  :height="16"
                />
                <span class="bar-value">{{ selectedSnapshot.courseSpacing?.toFixed(1) }}</span>
              </div>
              <div class="score-bar-item">
                <span>到校天数</span>
                <n-progress
                  type="line"
                  :percentage="Math.round((selectedSnapshot.teacherDays / perCategoryMax) * 1000) / 10"
                  :color="scoreColor(selectedSnapshot.teacherDays / perCategoryMax * 100)"
                  :height="16"
                />
                <span class="bar-value">{{ selectedSnapshot.teacherDays?.toFixed(1) }}</span>
              </div>
              <div class="score-bar-item">
                <span>低楼层</span>
                <n-progress
                  type="line"
                  :percentage="Math.round((selectedSnapshot.lowFloorPref / perCategoryMax) * 1000) / 10"
                  :color="scoreColor(selectedSnapshot.lowFloorPref / perCategoryMax * 100)"
                  :height="16"
                />
                <span class="bar-value">{{ selectedSnapshot.lowFloorPref?.toFixed(1) }}</span>
              </div>
              <div class="score-bar-item" v-if="selectedSnapshot.weekendAvoid !== undefined">
                <span>周末避让</span>
                <n-progress
                  type="line"
                  :percentage="Math.round(((selectedSnapshot.weekendAvoid || 0) / perCategoryMax) * 1000) / 10"
                  :color="scoreColor((selectedSnapshot.weekendAvoid || 0) / perCategoryMax * 100)"
                  :height="16"
                />
                <span class="bar-value">{{ (selectedSnapshot.weekendAvoid || 0).toFixed(1) }}</span>
              </div>
              <div class="score-bar-item" v-if="(selectedSnapshot.pePeriodPref || 0) > 0">
                <span>体育课时段</span>
                <n-progress
                  type="line"
                  :percentage="Math.round(((selectedSnapshot.pePeriodPref || 0) / perCategoryMax) * 1000) / 10"
                  :color="scoreColor((selectedSnapshot.pePeriodPref || 0) / perCategoryMax * 100)"
                  :height="16"
                />
                <span class="bar-value">{{ (selectedSnapshot.pePeriodPref || 0).toFixed(1) }}</span>
              </div>
              <div class="score-bar-item" v-if="(selectedSnapshot.studentFatigue || 0) > 0">
                <span>学生疲劳度</span>
                <n-progress
                  type="line"
                  :percentage="Math.round(((selectedSnapshot.studentFatigue || 0) / perCategoryMax) * 1000) / 10"
                  :color="scoreColor((selectedSnapshot.studentFatigue || 0) / perCategoryMax * 100)"
                  :height="16"
                />
                <span class="bar-value">{{ (selectedSnapshot.studentFatigue || 0).toFixed(1) }}</span>
              </div>
            </div>
          </n-card>

          <!-- Hard constraint status -->
          <n-card title="硬约束合规" size="small" class="check-card">
            <div class="check-items">
              <div class="check-item">
                <span class="check-icon">{{ selectedSnapshot.hardPassed ? '✅' : '❌' }}</span>
                <span>所有硬约束{{ selectedSnapshot.hardPassed ? '通过' : '未通过' }}</span>
              </div>
              <div class="check-meta">
                已排 {{ selectedSnapshot.totalEntries }} 条课表 ·
                求解耗时 {{ selectedSnapshot.solveTimeMs }}ms ·
                求解器 {{ selectedSnapshot.solver }}
              </div>
            </div>
          </n-card>

          <!-- Teacher detail table ("拖后腿排行榜") -->
          <n-card title="教师评分明细" size="small" class="detail-card" v-if="rankedDetails.length > 0">
            <div class="detail-table">
              <div class="detail-header">
                <span class="col-name">教师</span>
                <span class="col-num">早课</span>
                <span class="col-num">晚课</span>
                <span class="col-num">到校/目标</span>
                <span class="col-num">均楼层</span>
                <span class="col-summary">摘要</span>
              </div>
              <div
                v-for="d in rankedDetails"
                :key="d.ID"
                class="detail-row"
              >
                <span class="col-name">{{ d.entityName }}</span>
                <span class="col-num" :class="{ warn: d.earlyPenalty > 0 }">{{ d.earlyPenalty }}</span>
                <span class="col-num" :class="{ warn: d.latePenalty > 0 }">{{ d.latePenalty }}</span>
                <span class="col-num" :class="{ warn: d.daysActual > d.daysTarget }">
                  {{ d.daysActual }}/{{ d.daysTarget }}
                </span>
                <span class="col-num">{{ d.avgFloor?.toFixed(1) }}</span>
                <span class="col-summary">{{ d.summary }}</span>
              </div>
            </div>
          </n-card>

          <!-- Print-only report header -->
          <div class="print-only print-header">
            <h1>排课验证报告</h1>
            <p>学期：{{ selectedSnapshot.semester }} · 院系：{{ selectedSnapshot.dept || '全校' }}</p>
            <p>生成时间：{{ formatDate(selectedSnapshot.CreatedAt || selectedSnapshot.createdAt) }}</p>
            <p>生成方式：{{ triggerLabel(selectedSnapshot.trigger) }}</p>
          </div>
        </div>
      </div>
    </n-spin>
  </div>
</template>

<style scoped>
.report-page {
  height: 100%;
  display: flex;
  flex-direction: column;
}

.report-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
  flex-shrink: 0;
}

.report-title {
  font-size: 18px;
  font-weight: 600;
  color: var(--b3-theme-on-background);
  margin: 0;
}

.report-actions {
  display: flex;
  gap: 12px;
}

.empty-state {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 300px;
}

.report-content {
  display: flex;
  gap: 20px;
  flex: 1;
  min-height: 0;
}

.snapshot-list {
  width: 240px;
  flex-shrink: 0;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.snapshot-card {
  padding: 12px;
  border-radius: 6px;
  border: 1px solid var(--b3-border-color);
  cursor: pointer;
  transition: all 0.2s;
  position: relative;
}

.snap-delete-btn {
  position: absolute;
  top: 4px;
  right: 6px;
  width: 22px;
  height: 22px;
  border: none;
  background: transparent;
  color: var(--b3-text-color-3);
  font-size: 16px;
  line-height: 1;
  cursor: pointer;
  border-radius: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
  opacity: 0;
  transition: opacity 0.15s, color 0.15s, background 0.15s;
}

.snapshot-card:hover .snap-delete-btn {
  opacity: 1;
}

.snap-delete-btn:hover {
  color: #d03050;
  background: rgba(208, 48, 80, 0.1);
}

.snapshot-card:hover {
  border-color: var(--b3-theme-primary);
}

.snapshot-card.active {
  border-color: var(--b3-theme-primary);
  background: var(--b3-card-background);
}

.snap-date {
  font-size: 12px;
  color: var(--b3-text-color-2);
  margin-bottom: 6px;
}

.snap-meta {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.snap-score {
  font-size: 16px;
  font-weight: 700;
}

.detail-area {
  flex: 1;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-width: 0;
}

.score-card .total-score {
  font-size: 48px;
  font-weight: 800;
  text-align: center;
  margin: 12px 0;
}

.score-unit {
  font-size: 18px;
  font-weight: 400;
  opacity: 0.6;
}

.score-bars {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.score-bar-item {
  display: flex;
  align-items: center;
  gap: 8px;
}

.score-bar-item > span:first-child {
  width: 70px;
  font-size: 13px;
  flex-shrink: 0;
}

.score-bar-item .n-progress {
  flex: 1;
}

.bar-value {
  width: 40px;
  text-align: right;
  font-size: 13px;
  font-weight: 600;
}

.check-items {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.check-item {
  font-size: 14px;
}

.check-icon {
  margin-right: 6px;
}

.check-meta {
  font-size: 12px;
  color: var(--b3-text-color-2);
  margin-top: 4px;
}

/* Detail table */
.detail-table {
  font-size: 13px;
}

.detail-header, .detail-row {
  display: flex;
  gap: 8px;
  padding: 6px 0;
  border-bottom: 1px solid var(--b3-border-color);
}

.detail-header {
  font-weight: 600;
  color: var(--b3-text-color-2);
  font-size: 12px;
}

.col-name { width: 80px; flex-shrink: 0; }
.col-num  { width: 50px; text-align: center; flex-shrink: 0; }
.col-summary { flex: 1; font-size: 12px; color: var(--b3-text-color-2); }

.warn { color: var(--b3-theme-error); font-weight: 600; }

/* Print styles */
.print-only { display: none; }

@media print {
  .no-print { display: none !important; }
  .print-only { display: block !important; }
  .print-header { margin-bottom: 24px; }
  .print-header h1 { font-size: 24px; margin-bottom: 8px; }
  .print-header p { margin: 4px 0; color: #666; }
  .detail-area { overflow: visible; }
}
</style>
