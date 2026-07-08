<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { NTag, NButton } from 'naive-ui'
import { useAppStore } from '../stores/app'
import { useScheduleStore } from '../stores/schedule'
import { DetectConflicts } from '../../bindings/scheduling-system/services/conflictservice'

const appStore = useAppStore()
const scheduleStore = useScheduleStore()

interface ConflictDetail {
  label: string
  value: string
}

interface ConflictItem {
  id: number
  type: string
  description: string
  detail: string
  severity: 'error' | 'warning'
  info: ConflictDetail[]
  solutions: string[]
}

const defaultConflicts: ConflictItem[] = [
  {
    id: 1,
    type: '教师时间冲突',
    description: '王建国教授在周一第1-2节同时段有两门课程',
    detail: '高等数学(A301) vs 线性代数(B205)',
    severity: 'error',
    info: [
      { label: '冲突教师', value: '王建国 教授' },
      { label: '所属院系', value: '数学与统计学院' },
      { label: '冲突课程 A', value: '高等数学 - 周一 1-2节 - A301' },
      { label: '冲突课程 B', value: '线性代数 - 周一 1-2节 - B205' },
      { label: '选课人数', value: 'A: 180人 / B: 95人' },
      { label: '冲突类型', value: '同一教师同一时段' },
    ],
    solutions: [
      '将线性代数调至其他时段',
      '更换线性代数授课教师',
      '合并为合班授课',
    ],
  },
  {
    id: 2,
    type: '教室容量冲突',
    description: '计算机组成原理选课120人，分配教室A301仅80座',
    detail: '周三第3-4节 - A301',
    severity: 'error',
    info: [
      { label: '冲突课程', value: '计算机组成原理' },
      { label: '授课教师', value: '李伟 副教授' },
      { label: '选课人数', value: '120人' },
      { label: '分配教室', value: 'A301（容量80座）' },
      { label: '差额', value: '40人（教室容量不足50%）' },
      { label: '冲突类型', value: '教室容量不足' },
    ],
    solutions: [
      '调至大容量教室（推荐：D401阶梯教室，200座）',
      '限制选课人数为80人',
      '拆分为两个教学班',
    ],
  },
  {
    id: 3,
    type: '教室占用冲突',
    description: 'C502教室在周四第5-6节被两门课程同时占用',
    detail: '数据结构 vs 操作系统 - C502',
    severity: 'warning',
    info: [
      { label: '冲突教室', value: 'C502（多媒体教室）' },
      { label: '冲突时段', value: '周四 5-6节 (14:00-15:35)' },
      { label: '课程 A', value: '数据结构 - 张明远 教授' },
      { label: '课程 B', value: '操作系统 - 周海 副教授' },
      { label: '教室容量', value: '120座（两门课合计205人选课）' },
      { label: '冲突类型', value: '同一教室同一时段' },
    ],
    solutions: [
      '将数据结构调至C301',
      '将操作系统调至D401',
      '调整时段：A课程改至7-8节',
    ],
  },
]

const conflicts = ref<ConflictItem[]>(defaultConflicts)

// Load from Go backend
onMounted(async () => {
  try {
    const data = await DetectConflicts('2025-2026 第二学期')
    if (data && data.length > 0) {
      conflicts.value = data.map((c, i) => ({
        id: c.id || i + 1,
        type: c.type || '',
        description: c.description || '',
        detail: c.detail || '',
        severity: (c.severity === 'error' || c.severity === 'warning') ? c.severity : 'error',
        info: c.info || [],
        solutions: c.solutions || [],
      }))
    }
  } catch (e) {
    console.warn('Failed to load conflicts from Go backend, using defaults:', e)
  }
})
const selectedId = ref(1)
const selectedConflict = computed(() => conflicts.value.find(c => c.id === selectedId.value))
const selectedSolution = ref(-1)

function selectSolution(idx: number) {
  selectedSolution.value = selectedSolution.value === idx ? -1 : idx
}

function locateInSchedule() {
  appStore.navigateTo('schedule', '周视图课表')
  scheduleStore.switchView('week')
}

function dismissConflict() {
  conflicts.value = conflicts.value.filter(c => c.id !== selectedId.value)
  if (conflicts.value.length > 0) {
    selectedId.value = conflicts.value[0].id
    selectedSolution.value = -1
  }
}
</script>

<template>
  <div class="conflict-page">
    <div class="conflict-layout">
      <div class="conflict-list">
        <div class="list-header">
          <span>冲突列表</span>
          <n-tag type="error" size="small">{{ conflicts.length }} 项</n-tag>
        </div>
        <div
          v-for="c in conflicts"
          :key="c.id"
          class="conflict-item"
          :class="{ active: selectedId === c.id }"
          @click="selectedId = c.id; selectedSolution = -1"
        >
          <div class="conflict-type">
            <n-tag :type="c.severity === 'error' ? 'error' : 'warning'" size="tiny">!</n-tag>
            {{ c.type }}
          </div>
          <div class="conflict-desc">{{ c.description }}</div>
          <div class="conflict-sub">{{ c.detail }}</div>
        </div>
      </div>

      <div class="conflict-detail-panel" v-if="selectedConflict">
        <div class="detail-header">
          <h3>冲突详情</h3>
          <n-button size="small" text @click="locateInSchedule()">定位到课表</n-button>
        </div>

        <div class="detail-grid">
          <div v-for="item in selectedConflict.info" :key="item.label" class="detail-item">
            <span class="dl">{{ item.label }}</span>
            <span class="dv">{{ item.value }}</span>
          </div>
        </div>

        <div class="resolve-section">
          <h4>解决方案</h4>
          <div class="resolve-options">
            <div
              v-for="(sol, idx) in selectedConflict.solutions"
              :key="idx"
              class="resolve-option"
              :class="{ selected: selectedSolution === idx }"
              @click="selectSolution(idx)"
            >
              <span class="resolve-radio" :class="{ checked: selectedSolution === idx }"></span>
              {{ sol }}
            </div>
          </div>
          <div class="resolve-actions">
            <n-button size="small" @click="dismissConflict()">暂不处理</n-button>
            <n-button size="small" type="primary" :disabled="selectedSolution < 0">确认解决</n-button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.conflict-page { flex: 1; display: flex; flex-direction: column; min-height: 0; }
.conflict-layout { flex: 1; display: grid; grid-template-columns: 300px 1fr; gap: 20px; min-height: 0; }
.conflict-list { background: var(--b3-theme-surface); border: 1px solid var(--b3-border-color); border-radius: var(--b3-border-radius); overflow-y: auto; }
.list-header { display: flex; justify-content: space-between; align-items: center; padding: 12px 16px; font-size: 13px; font-weight: 600; color: var(--b3-theme-on-background); border-bottom: 1px solid var(--b3-border-color); }
.conflict-item { padding: 12px 16px; border-bottom: 1px solid var(--b3-border-color); cursor: pointer; transition: background 0.15s; }
.conflict-item:hover { background: var(--b3-list-hover); }
.conflict-item.active { background: var(--b3-theme-primary-lightest); border-left: 3px solid var(--b3-theme-primary); }
.conflict-type { font-size: 13px; font-weight: 600; color: var(--b3-theme-on-background); display: flex; align-items: center; gap: 6px; margin-bottom: 4px; }
.conflict-desc { font-size: 12px; color: var(--b3-theme-on-surface); margin-bottom: 2px; }
.conflict-sub { font-size: 11px; color: var(--b3-theme-on-surface-light); }
.conflict-detail-panel { background: var(--b3-theme-surface); border: 1px solid var(--b3-border-color); border-radius: var(--b3-border-radius); padding: 20px; overflow-y: auto; }
.detail-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
.detail-header h3 { font-size: 14px; font-weight: 600; color: var(--b3-theme-on-background); }
.detail-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; margin-bottom: 20px; }
.detail-item { padding: 6px 0; }
.dl { display: block; font-size: 11px; color: var(--b3-theme-on-surface-light); margin-bottom: 2px; }
.dv { font-size: 13px; color: var(--b3-theme-on-background); font-weight: 500; }
.resolve-section { border-top: 1px solid var(--b3-border-color); padding-top: 16px; }
.resolve-section h4 { font-size: 13px; font-weight: 600; color: var(--b3-theme-on-background); margin-bottom: 12px; }
.resolve-options { display: flex; flex-direction: column; gap: 8px; margin-bottom: 14px; }
.resolve-option { display: flex; align-items: center; gap: 8px; font-size: 13px; color: var(--b3-theme-on-surface); cursor: pointer; padding: 4px 0; }
.resolve-option.selected { color: var(--b3-theme-primary); }
.resolve-radio { width: 14px; height: 14px; border: 2px solid var(--b3-border-color); border-radius: 50%; flex-shrink: 0; }
.resolve-radio.checked { border-color: var(--b3-theme-primary); background: var(--b3-theme-primary); box-shadow: inset 0 0 0 3px var(--b3-theme-surface); }
.resolve-actions { display: flex; justify-content: flex-end; gap: 8px; }
</style>
