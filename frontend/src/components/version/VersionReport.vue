<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { NCard, NProgress, NEmpty, NSpin } from 'naive-ui'
import { GetVersionWithDetails, AnalyzeTeacherWorkload } from '../../../bindings/scheduling-system/backend/services/versionservice'
import type { ScheduleVersion, VersionDetail } from '../../../bindings/scheduling-system/backend/models/models'
import type { TeacherWorkloadInfo } from '../../../bindings/scheduling-system/backend/services/models'
import { scoreColor, starRating } from '../../utils/score'

const props = defineProps<{
  versionId: number | null
}>()

const loading = ref(false)
const version = ref<ScheduleVersion | null>(null)
const workloadData = ref<TeacherWorkloadInfo[]>([])

async function loadData() {
  if (!props.versionId) {
    version.value = null
    workloadData.value = []
    return
  }
  loading.value = true
  try {
    const detail = await GetVersionWithDetails(props.versionId)
    version.value = detail
    // Load workload using the version's semester ID
    try {
      workloadData.value = await AnalyzeTeacherWorkload(detail.semesterId) || []
    } catch {
      workloadData.value = []
    }
  } catch {
    version.value = null
    workloadData.value = []
  } finally {
    loading.value = false
  }
}

// Use watch only (no onMounted) to avoid double-fire on mount
watch(() => props.versionId, loadData, { immediate: true })

// ---- Scoring helpers ----
const perCategoryMax = computed(() => version.value?.perCategoryMax || 25)

// Parse categoryMaxes JSON once per version change
const parsedCategoryMaxes = computed<Record<string, number>>(() => {
  const v = version.value
  if (!v) return {}
  if (typeof v.categoryMaxes === 'string' && v.categoryMaxes) {
    try { return JSON.parse(v.categoryMaxes) } catch { return {} }
  }
  if (typeof v.categoryMaxes === 'object' && v.categoryMaxes) {
    return v.categoryMaxes as Record<string, number>
  }
  return {}
})

function getCategoryMax(field: string): number {
  const maxes = parsedCategoryMaxes.value
  if (maxes[field] != null && maxes[field] > 0) return maxes[field]
  return perCategoryMax.value
}

const categoryDefs: { key: string; label: string; field: string }[] = [
  { key: 'teacherPref', label: '教师偏好', field: 'teacherPref' },
  { key: 'courseSpacing', label: '课程分布', field: 'courseSpacing' },
  { key: 'teacherDays', label: '到校天数', field: 'teacherDays' },
  { key: 'lowFloorPref', label: '低楼层', field: 'lowFloorPref' },
  { key: 'weekendAvoid', label: '周末避让', field: 'weekendAvoid' },
  { key: 'pePeriodPref', label: '体育课时段', field: 'pePeriodPref' },
  { key: 'studentFatigue', label: '学生疲劳度', field: 'studentFatigue' },
]

const gradeLabel = computed(() => {
  const s = version.value?.finalScore ?? version.value?.totalScore ?? 0
  if (s >= 95) return { label: 'A+', color: '#18a058' }
  if (s >= 85) return { label: 'A', color: '#18a058' }
  if (s >= 75) return { label: 'B', color: '#3575f0' }
  if (s >= 60) return { label: 'C', color: '#f0a020' }
  return { label: 'D', color: '#d03050' }
})

// Type-safe accessor for category score fields
const categoryScores = computed(() => {
  const v = version.value
  if (!v) return {} as Record<string, number>
  return {
    teacherPref: v.teacherPref ?? 0,
    courseSpacing: v.courseSpacing ?? 0,
    teacherDays: v.teacherDays ?? 0,
    lowFloorPref: v.lowFloorPref ?? 0,
    weekendAvoid: v.weekendAvoid ?? 0,
    pePeriodPref: v.pePeriodPref ?? 0,
    studentFatigue: v.studentFatigue ?? 0,
  } as Record<string, number>
})

const rankedDetails = computed(() => {
  if (!version.value?.details) return []
  return [...version.value.details].sort((a: VersionDetail, b: VersionDetail) => {
    const penaltyA = a.earlyPenalty + a.latePenalty + Math.max(0, a.daysActual - a.daysTarget)
    const penaltyB = b.earlyPenalty + b.latePenalty + Math.max(0, b.daysActual - b.daysTarget)
    return penaltyB - penaltyA
  })
})

const suggestions = computed(() => {
  if (!version.value) return []
  const max = perCategoryMax.value
  const tips: { label: string; text: string }[] = []
  const pct = (field: string) => {
    const val = categoryScores.value[field]
    return val !== undefined ? (val / max) * 100 : 100
  }
  if (pct('teacherPref') < 60) tips.push({ label: '教师偏好', text: '部分教师被安排在不偏好的时段（早课/晚课），可调整教师偏好设置或增加该约束权重' })
  if (pct('courseSpacing') < 60) tips.push({ label: '课程分布', text: '部分课程集中在相邻日期或同一天，建议启用"课程分散度"约束并提高权重' })
  if (pct('teacherDays') < 60) tips.push({ label: '到校天数', text: '部分教师到校天数超过目标值，可调整教师 MaxDays 设置' })
  if (pct('lowFloorPref') < 60) tips.push({ label: '低楼层', text: '偏好低楼层的教师被分配到较高楼层，可增加低楼层教室资源' })
  if (pct('weekendAvoid') < 60) tips.push({ label: '周末避让', text: '较多课程被排在周六/周日，可启用"避开周末"约束' })
  if (pct('pePeriodPref') < 60) tips.push({ label: '体育课时段', text: '体育课未优先排在推荐时段（3-4节或7-8节）' })
  if (pct('studentFatigue') < 60) tips.push({ label: '学生疲劳度', text: '部分班级连续课时超过4节，建议分散该班级课程' })
  return tips
})
</script>

<template>
  <div class="version-report">
    <NSpin :show="loading">
      <NEmpty v-if="!loading && !version" description="请选择一个版本查看报告" />

      <template v-else-if="version">
        <!-- Score overview -->
        <NCard title="综合评分" size="small" class="score-card">
          <div class="total-score-row">
            <div class="total-score" :style="{ color: scoreColor(version.finalScore ?? version.totalScore) }">
              {{ (version.finalScore ?? version.totalScore)?.toFixed(2) }}
              <span class="score-unit">/ 100</span>
            </div>
            <span class="grade-badge" :style="{ background: gradeLabel.color }">
              {{ gradeLabel.label }}
            </span>
          </div>
          <div class="score-bars">
            <template v-for="cat in categoryDefs" :key="cat.key">
              <div class="score-bar-item" v-if="categoryScores[cat.field] !== undefined">
                <span>{{ cat.label }}</span>
                <NProgress
                  type="line"
                  :percentage="Math.round((categoryScores[cat.field] / getCategoryMax(cat.field)) * 1000) / 10"
                  :color="scoreColor((categoryScores[cat.field] / getCategoryMax(cat.field)) * 100)"
                  :height="16"
                />
                <span class="star-rating" :style="{ color: scoreColor((categoryScores[cat.field] / getCategoryMax(cat.field)) * 100) }">
                  {{ starRating(categoryScores[cat.field], getCategoryMax(cat.field)) }}
                </span>
                <span class="bar-value">{{ categoryScores[cat.field].toFixed(2) }}</span>
              </div>
            </template>
          </div>
        </NCard>

        <!-- Suggestions -->
        <NCard title="改善建议" size="small" class="suggest-card" v-if="suggestions.length > 0">
          <div class="suggest-list">
            <div v-for="tip in suggestions" :key="tip.label" class="suggest-item">
              <span class="suggest-label">{{ tip.label }}</span>
              <span class="suggest-text">{{ tip.text }}</span>
            </div>
          </div>
        </NCard>

        <!-- Teacher workload -->
        <NCard title="教师负载分析" size="small" class="workload-card" v-if="workloadData.length > 0">
          <div class="workload-table">
            <div class="wl-header">
              <span class="wl-name">教师</span>
              <span class="wl-num">总课时</span>
              <span class="wl-dist">每日分布</span>
              <span class="wl-num">最多/日</span>
              <span class="wl-score">均衡</span>
              <span class="wl-tip">建议</span>
            </div>
            <div v-for="w in workloadData" :key="w.teacherId" class="wl-row" :class="{ 'wl-warn': w.balanceScore < 50 }">
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
        </NCard>

        <!-- Hard constraint -->
        <NCard title="硬约束合规" size="small" class="check-card">
          <div class="check-items">
            <div class="check-item">
              <span class="check-icon">{{ version.hardPassed ? '✅' : '❌' }}</span>
              <span>所有硬约束{{ version.hardPassed ? '通过' : '未通过' }}</span>
            </div>
            <div class="check-meta">
              已排 {{ version.entryCount }} 条课表 ·
              求解耗时 {{ version.solveTimeMs }}ms ·
              求解器 {{ version.solver }}
            </div>
          </div>
        </NCard>

        <!-- Teacher detail table -->
        <NCard title="教师评分明细" size="small" class="detail-card" v-if="rankedDetails.length > 0">
          <div class="detail-table">
            <div class="detail-header">
              <span class="col-name">教师</span>
              <span class="col-num">早课</span>
              <span class="col-num">晚课</span>
              <span class="col-num">到校/目标</span>
              <span class="col-num">均楼层</span>
              <span class="col-summary">摘要</span>
            </div>
            <div v-for="d in rankedDetails" :key="d.ID" class="detail-row">
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
        </NCard>
      </template>
    </NSpin>
  </div>
</template>

<style scoped>
.version-report {
  display: flex;
  flex-direction: column;
  gap: 16px;
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
.score-bar-item .n-progress { flex: 1; }
.score-bar-item :deep(.n-progress-icon--as-text) { white-space: nowrap; }
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
.suggest-list { display: flex; flex-direction: column; gap: 8px; }
.suggest-item {
  display: flex; gap: 8px; font-size: 13px;
  padding: 6px 10px; border-radius: 6px;
  background: var(--b3-card-background);
  border-left: 3px solid #f0a020;
}
.suggest-label { font-weight: 700; color: #f0a020; white-space: nowrap; flex-shrink: 0; }
.suggest-text { color: var(--b3-text-color-1); line-height: 1.5; }

/* Workload */
.workload-table { font-size: 13px; }
.wl-header, .wl-row {
  display: flex; gap: 8px; padding: 5px 0;
  border-bottom: 1px solid var(--b3-border-color); align-items: center;
}
.wl-header { font-weight: 600; color: var(--b3-text-color-2); font-size: 12px; }
.wl-row.wl-warn { background: rgba(208, 48, 80, 0.04); }
.wl-name { width: 70px; flex-shrink: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.wl-num  { width: 42px; text-align: center; flex-shrink: 0; }
.wl-dist { width: 120px; flex-shrink: 0; display: flex; gap: 4px; justify-content: center; }
.wl-dot  { font-size: 11px; color: var(--b3-text-color-3); min-width: 12px; text-align: center; }
.wl-dot.active { color: var(--b3-theme-primary); font-weight: 700; }
.wl-score { width: 38px; text-align: center; font-weight: 700; flex-shrink: 0; }
.wl-tip  { flex: 1; font-size: 12px; color: var(--b3-text-color-2); }
.wl-num.warn { color: #d03050; font-weight: 700; }

/* Check */
.check-items { display: flex; flex-direction: column; gap: 4px; }
.check-item { font-size: 14px; }
.check-icon { margin-right: 6px; }
.check-meta { font-size: 12px; color: var(--b3-text-color-2); margin-top: 4px; }

/* Detail table */
.detail-table { font-size: 13px; }
.detail-header, .detail-row {
  display: flex; gap: 8px; padding: 6px 0;
  border-bottom: 1px solid var(--b3-border-color);
}
.detail-header { font-weight: 600; color: var(--b3-text-color-2); font-size: 12px; }
.col-name { width: 80px; flex-shrink: 0; }
.col-num  { width: 50px; text-align: center; flex-shrink: 0; }
.col-summary { flex: 1; font-size: 12px; color: var(--b3-text-color-2); }
.warn { color: var(--b3-theme-error); font-weight: 600; }
</style>
