<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { NTag, NButton, NEmpty, NSpin, NProgress, NCard } from 'naive-ui'
import { useAppStore } from '../stores/app'
import type { TeacherWorkloadInfo } from '../types'
import { jsPDF } from 'jspdf'
import html2canvas from 'html2canvas'

const appStore = useAppStore()

// ---- State ----
const loading = ref(false)
const snapshots = ref<any[]>([])
const selectedSnapshot = ref<any | null>(null)
const workloadData = ref<TeacherWorkloadInfo[]>([])

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
    const snapshotResult = await GetSnapshots(appStore.semesterFilter)
    snapshots.value = snapshotResult || []
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
  loadWorkload()
}

async function loadWorkload() {
  try {
    const { AnalyzeTeacherWorkload } = await import('../../bindings/scheduling-system/backend/services/snapshotservice')
    const data = await AnalyzeTeacherWorkload(appStore.semesterFilter)
    workloadData.value = data || []
  } catch {
    workloadData.value = []
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

async function exportPDF() {
  // (1) 数据检查
  if (!selectedSnapshot.value) {
    window.alert('请先生成或选择一份验证报告')
    return
  }

  const reportEl = document.querySelector('.report-content') as HTMLElement
  if (!reportEl) {
    window.alert('报告内容未加载，请刷新后重试')
    return
  }

  const snap = selectedSnapshot.value
  const EXPORT_W = 1200

  // (2) 创建离屏导出容器 + (3) 注入打印环境 CSS 变量
  const container = document.createElement('div')
  const b3Vars: Record<string, string> = {
    '--b3-theme-background': '#ffffff',
    '--b3-theme-surface': '#f6f6f6',
    '--b3-theme-on-surface': '#333333',
    '--b3-theme-on-background': '#222222',
    '--b3-theme-on-surface-light': '#999999',
    '--b3-border-color': '#e0e0e0',
    '--b3-border-radius': '6px',
    '--b3-border-radius-s': '4px',
    '--b3-theme-primary': '#3575f0',
    '--b3-theme-primary-light': '#5b8af7',
    '--b3-theme-primary-lightest': '#e8f0fe',
    '--b3-theme-error': '#e53935',
    '--b3-body-background': '#ffffff',
    '--b3-font-family': '"Microsoft YaHei","PingFang SC",sans-serif',
  }
  container.style.cssText = `position:fixed;left:-30000px;top:0;width:${EXPORT_W}px;background:#fff;padding:14px 18px 10px;font-family:"Microsoft YaHei","PingFang SC",sans-serif;color:#222;`
  for (const [k, v] of Object.entries(b3Vars)) container.style.setProperty(k, v)

  // DOM 渲染的标题头（避免 jsPDF 中文乱码）
  const dateStr = formatDate(snap.CreatedAt || snap.createdAt)
  const headerHtml = `<div style="font-size:20px;font-weight:700;margin-bottom:4px;line-height:1.4;">排课验证报告</div><div style="font-size:12px;color:#888;margin-bottom:12px;">学期：${snap.semester || ''} · 院系：${snap.dept || '全校'}　|　生成时间：${dateStr}　|　${triggerLabel(snap.trigger)}</div>`
  container.innerHTML = headerHtml

  // (4) 克隆报告主体
  const clone = reportEl.cloneNode(true) as HTMLElement

  // 移除 UI 元素
  clone.querySelector('.snapshot-list')?.remove()
  clone.querySelectorAll('.no-print').forEach(el => el.remove())

  // 强制显示 print-only 元素（scoped CSS 会隐藏它们）
  clone.querySelectorAll('.print-only').forEach(el => {
    (el as HTMLElement).style.display = 'block'
  })

  // 解除布局约束
  clone.style.setProperty('overflow', 'visible')
  clone.style.setProperty('height', 'auto')
  clone.style.setProperty('display', 'block')

  const detailArea = clone.querySelector('.detail-area') as HTMLElement
  if (detailArea) {
    detailArea.style.setProperty('overflow', 'visible')
    detailArea.style.setProperty('height', 'auto')
    detailArea.style.setProperty('flex', 'none')
    detailArea.style.setProperty('width', '100%')
  }

  container.appendChild(clone)
  document.body.appendChild(container)

  try {
    // (5) html2canvas 截图
    const canvas = await html2canvas(container, {
      scale: 3,
      useCORS: true,
      backgroundColor: '#ffffff',
    })

    // (6) jsPDF 输出（横向 A4，支持分页）
    const pdf = new jsPDF({ orientation: 'l', unit: 'mm', format: 'a4' })
    const pageW = pdf.internal.pageSize.getWidth()
    const pageH = pdf.internal.pageSize.getHeight()
    const imgW = pageW - 14
    const imgH = (canvas.height * imgW) / canvas.width
    const imgData = canvas.toDataURL('image/png')

    let heightLeft = imgH
    let position = 7
    pdf.addImage(imgData, 'PNG', 7, position, imgW, imgH)
    heightLeft -= (pageH - position)
    while (heightLeft > 0) {
      position -= pageH
      pdf.addPage()
      pdf.addImage(imgData, 'PNG', 7, position, imgW, imgH)
      heightLeft -= pageH
    }

    const fileDate = new Date().toISOString().slice(0, 10)
    const hash6 = Math.random().toString(16).slice(2, 8)
    pdf.save(`验证报告_${fileDate}_${hash6}.pdf`)
  } catch (err: any) {
    // (8) 异常处理
    window.alert('PDF 导出失败：' + (err?.message || err))
  } finally {
    // (7) 清理离屏容器
    document.body.removeChild(container)
  }
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

	// ---- Quality analysis (v0.3.0) ----

	/** Overall grade from total score */
	const gradeLabel = computed(() => {
	  const s = selectedSnapshot.value?.totalScore ?? 0
	  if (s >= 95) return { label: 'A+', color: '#18a058' }
	  if (s >= 85) return { label: 'A',  color: '#18a058' }
	  if (s >= 75) return { label: 'B',  color: '#3575f0' }
	  if (s >= 60) return { label: 'C',  color: '#f0a020' }
	  return { label: 'D', color: '#d03050' }
	})

	/** Category metadata for display */
	const categoryDefs: { key: string; label: string; field: string }[] = [
	  { key: 'teacherPref',    label: '教师偏好',   field: 'teacherPref' },
	  { key: 'courseSpacing',  label: '课程分布',   field: 'courseSpacing' },
	  { key: 'teacherDays',    label: '到校天数',   field: 'teacherDays' },
	  { key: 'lowFloorPref',   label: '低楼层',     field: 'lowFloorPref' },
	  { key: 'weekendAvoid',   label: '周末避让',   field: 'weekendAvoid' },
	  { key: 'pePeriodPref',   label: '体育课时段', field: 'pePeriodPref' },
	  { key: 'studentFatigue', label: '学生疲劳度', field: 'studentFatigue' },
	]

	/** Star rating string for a category (e.g. "★★★½☆") */
	function starRating(score: number, max: number): string {
	  if (max <= 0) return '☆☆☆☆☆'
	  const stars = Math.max(0, Math.min(5, (score / max) * 5))
	  const full = Math.floor(stars)
	  const half = stars - full >= 0.25 ? 1 : 0
	  const empty = 5 - full - half
	  return '★'.repeat(full) + (half ? '½' : '') + '☆'.repeat(empty)
	}

	/** Auto-generated suggestions when a category falls below 60% */
	const suggestions = computed(() => {
	  if (!selectedSnapshot.value) return []
	  const s = selectedSnapshot.value
	  const max = perCategoryMax.value
	  const tips: { label: string; text: string }[] = []
	  const pct = (field: string) => {
	    const v = s[field]
	    return v !== undefined && v !== null ? (v / max) * 100 : 100
	  }
	  if (pct('teacherPref') < 60)    tips.push({ label: '教师偏好', text: '部分教师被安排在不偏好的时段（早课/晚课），可调整教师偏好设置或增加该约束权重' })
	  if (pct('courseSpacing') < 60)  tips.push({ label: '课程分布', text: '部分课程集中在相邻日期或同一天，建议启用"课程分散度"约束并提高权重' })
	  if (pct('teacherDays') < 60)    tips.push({ label: '到校天数', text: '部分教师到校天数超过目标值，可调整教师 MaxDays 设置' })
	  if (pct('lowFloorPref') < 60)   tips.push({ label: '低楼层', text: '偏好低楼层的教师被分配到较高楼层，可增加低楼层教室资源' })
	  if (pct('weekendAvoid') < 60)   tips.push({ label: '周末避让', text: '较多课程被排在周六/周日，可启用"避开周末"约束' })
	  if (pct('pePeriodPref') < 60)   tips.push({ label: '体育课时段', text: '体育课未优先排在推荐时段（3-4节或7-8节）' })
	  if (pct('studentFatigue') < 60) tips.push({ label: '学生疲劳度', text: '部分班级连续课时超过4节，建议分散该班级课程' })
	  return tips
	})

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
                {{ snap.totalScore?.toFixed(2) }}分
              </span>
            </div>
          </div>
        </div>

        <!-- Detail area -->
        <div v-if="selectedSnapshot" class="detail-area">
          <!-- Score overview -->
          <n-card title="综合评分" size="small" class="score-card">
            <div class="total-score-row">
              <div class="total-score" :style="{ color: scoreColor(selectedSnapshot.totalScore) }">
                {{ selectedSnapshot.totalScore?.toFixed(2) }}
                <span class="score-unit">/ 100</span>
              </div>
              <span class="grade-badge" :style="{ background: gradeLabel.color }">
                {{ gradeLabel.label }}
              </span>
            </div>
            <div class="score-bars">
              <template v-for="cat in categoryDefs" :key="cat.key">
                <div class="score-bar-item" v-if="selectedSnapshot[cat.field] !== undefined && (selectedSnapshot[cat.field] || 0) >= 0">
                  <span>{{ cat.label }}</span>
                  <n-progress
                    type="line"
                    :percentage="Math.round(((selectedSnapshot[cat.field] || 0) / perCategoryMax) * 1000) / 10"
                    :color="scoreColor(((selectedSnapshot[cat.field] || 0) / perCategoryMax) * 100)"
                    :height="16"
                  />
                  <span class="star-rating" :style="{ color: scoreColor(((selectedSnapshot[cat.field] || 0) / perCategoryMax) * 100) }">
                    {{ starRating((selectedSnapshot[cat.field] || 0), perCategoryMax) }}
                  </span>
                  <span class="bar-value">{{ (selectedSnapshot[cat.field] || 0).toFixed(2) }}</span>
                </div>
              </template>
            </div>
          </n-card>

          <!-- Suggestions -->
          <n-card title="改善建议" size="small" class="suggest-card" v-if="suggestions.length > 0">
            <div class="suggest-list">
              <div v-for="tip in suggestions" :key="tip.label" class="suggest-item">
                <span class="suggest-label">{{ tip.label }}</span>
                <span class="suggest-text">{{ tip.text }}</span>
              </div>
            </div>
          </n-card>

          <!-- Teacher workload analysis -->
          <n-card title="教师负载分析" size="small" class="workload-card" v-if="workloadData.length > 0">
            <div class="workload-table">
              <div class="wl-header">
                <span class="wl-name">教师</span>
                <span class="wl-num">总课时</span>
                <span class="wl-dist">每日分布</span>
                <span class="wl-num">最多/日</span>
                <span class="wl-score">均衡</span>
                <span class="wl-tip">建议</span>
              </div>
              <div
                v-for="w in workloadData"
                :key="w.teacherId"
                class="wl-row"
                :class="{ 'wl-warn': w.balanceScore < 50 }"
              >
                <span class="wl-name">{{ w.teacherName }}</span>
                <span class="wl-num">{{ w.totalSessions }}</span>
                <span class="wl-dist">
                  <span v-for="(c, d) in w.dailyDistribution" :key="d"
                    class="wl-dot" :class="{ active: c > 0 }"
                    :title="'周' + ['一','二','三','四','五','六','日'][d] + ': ' + c + '节'"
                  >{{ c || '·' }}</span>
                </span>
                <span class="wl-num" :class="{ warn: w.maxDaily >= 5 }">{{ w.maxDaily }}</span>
                <span class="wl-score" :style="{ color: w.balanceScore >= 80 ? '#18a058' : w.balanceScore >= 50 ? '#f0a020' : '#d03050' }">
                  {{ w.balanceScore.toFixed(0) }}
                </span>
                <span class="wl-tip">{{ w.suggestion || '—' }}</span>
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
                <span class="col-num">{{ d.avgFloor?.toFixed(2) }}</span>
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

.score-card .total-score-row {
  display: flex;
  align-items: baseline;
  justify-content: center;
  gap: 12px;
  margin: 12px 0;
}

.score-card .total-score {
  font-size: 48px;
  font-weight: 800;
}

.grade-badge {
  font-size: 28px;
  font-weight: 800;
  color: #fff;
  padding: 2px 14px;
  border-radius: 8px;
  line-height: 1.3;
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

/* 修复 n-progress 内部百分比文本换行 */
.score-bar-item :deep(.n-progress-icon--as-text) {
  white-space: nowrap;
}

.bar-value {
  min-width: 48px;
  text-align: right;
  font-size: 13px;
  font-weight: 600;
  white-space: nowrap;
  flex-shrink: 0;
  overflow: hidden;
  text-overflow: ellipsis;
}

.star-rating {
  font-size: 15px;
  letter-spacing: 1px;
  white-space: nowrap;
  flex-shrink: 0;
}

/* Suggestions */
.suggest-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.suggest-item {
  display: flex;
  gap: 8px;
  font-size: 13px;
  padding: 6px 10px;
  border-radius: 6px;
  background: var(--b3-card-background);
  border-left: 3px solid #f0a020;
}

.suggest-label {
  font-weight: 700;
  color: #f0a020;
  white-space: nowrap;
  flex-shrink: 0;
}

.suggest-text {
  color: var(--b3-text-color-1);
  line-height: 1.5;
}

/* Workload analysis */
.workload-table {
  font-size: 13px;
}

.wl-header, .wl-row {
  display: flex;
  gap: 8px;
  padding: 5px 0;
  border-bottom: 1px solid var(--b3-border-color);
  align-items: center;
}

.wl-header {
  font-weight: 600;
  color: var(--b3-text-color-2);
  font-size: 12px;
}

.wl-row.wl-warn {
  background: rgba(208, 48, 80, 0.04);
}

.wl-name { width: 70px; flex-shrink: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.wl-num  { width: 42px; text-align: center; flex-shrink: 0; }
.wl-dist { width: 120px; flex-shrink: 0; display: flex; gap: 4px; justify-content: center; }
.wl-dot  { font-size: 11px; color: var(--b3-text-color-3); min-width: 12px; text-align: center; }
.wl-dot.active { color: var(--b3-theme-primary); font-weight: 700; }
.wl-score { width: 38px; text-align: center; font-weight: 700; flex-shrink: 0; }
.wl-tip  { flex: 1; font-size: 12px; color: var(--b3-text-color-2); }
.wl-num.warn { color: #d03050; font-weight: 700; }

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
