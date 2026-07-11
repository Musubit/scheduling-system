import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { SchedulingConfig, SchedulingResult, LockedTimeSlot } from '@/types'
import { RunScheduling } from '../../bindings/scheduling-system/backend/services/schedulingservice'
import { GetActiveSemester, GetSemesters, SaveSetting } from '../../bindings/scheduling-system/backend/services/resourceservice'
import { useUiStore } from './ui'

// Default locked slots: Thursday periods 4-7 (第5-8节)
const DEFAULT_LOCKED: LockedTimeSlot[] = [
  { dayOfWeek: 3, startPeriod: 4, span: 4 },
]

function ensureLockedSlots() {
  try {
    const saved = localStorage.getItem('locked-time-slots')
    if (!saved) {
      localStorage.setItem('locked-time-slots', JSON.stringify(DEFAULT_LOCKED))
      SaveSetting('locked_time_slots', JSON.stringify(DEFAULT_LOCKED)).catch((err: any) => {
        console.warn('[Scheduling] 默认锁定时段保存失败:', err)
      })
    }
  } catch { /* localStorage unavailable */ }
}

// Constraint weight presets
export interface ConstraintPreset {
  name: string
  label: string
  weights: Record<string, number>
}

export const CONSTRAINT_PRESETS: ConstraintPreset[] = [
  {
    name: 'balanced',
    label: '均衡（推荐）',
    weights: {
      teacher_preference: 50,
      course_dispersed: 50,
      teacher_days_limit: 50,
      low_floor_preference: 50,
      avoid_saturday: 30,
      avoid_sunday: 30,
      pe_preferred_periods: 50,
      student_fatigue: 50,
    },
  },
  {
    name: 'teacher_first',
    label: '保护教师体验',
    weights: {
      teacher_preference: 100,
      course_dispersed: 30,
      teacher_days_limit: 80,
      low_floor_preference: 30,
      avoid_saturday: 20,
      avoid_sunday: 20,
      pe_preferred_periods: 40,
      student_fatigue: 30,
    },
  },
  {
    name: 'dispersed',
    label: '课表分散均衡',
    weights: {
      teacher_preference: 30,
      course_dispersed: 100,
      teacher_days_limit: 50,
      low_floor_preference: 20,
      avoid_saturday: 60,
      avoid_sunday: 60,
      pe_preferred_periods: 50,
      student_fatigue: 50,
    },
  },
  {
    name: 'low_floor',
    label: '教室优先低层',
    weights: {
      teacher_preference: 40,
      course_dispersed: 40,
      teacher_days_limit: 40,
      low_floor_preference: 100,
      avoid_saturday: 20,
      avoid_sunday: 20,
      pe_preferred_periods: 30,
      student_fatigue: 30,
    },
  },
]

const constraintOptions = [
  { key: 'teacher_preference', label: '教师偏好时段（避免早课/晚课）' },
  { key: 'course_dispersed', label: '同一课程分散安排（不集中在同一天）' },
  { key: 'teacher_days_limit', label: '教师到校天数限制（按教师设置）' },
  { key: 'low_floor_preference', label: '优先低楼层教室' },
  { key: 'pe_preferred_periods', label: '体育课优先3-4节或7-8节' },
  { key: 'student_fatigue', label: '避免学生连续疲劳（连续≤4节课）' },
  { key: 'avoid_saturday', label: '尽量避开周六排课' },
  { key: 'avoid_sunday', label: '尽量避开周日排课' },
]

const engineOptions = [
  { value: 'auto', label: '智能（推荐）——自动选择最佳引擎' },
  { value: 'sa', label: 'SA优化——模拟退火多轮求解' },
  { value: 'ortools', label: 'OR-Tools精确——最优解引擎' },
]

export const useSchedulingStore = defineStore('scheduling', () => {
  const config = ref<SchedulingConfig>({
    scope: '全校所有院系',
    semester: '',  // populated from active semester
    strategy: 'auto',
    iterations: 5000,
    timeLimit: 60,
    constraints: ['teacher_preference', 'course_dispersed', 'teacher_days_limit', 'low_floor_preference', 'student_fatigue'],
  })

  // Constraint weights (0-100 per constraint)
  const constraintWeights = ref<Record<string, number>>({ ...CONSTRAINT_PRESETS[0].weights })
  const activePreset = ref<string>('balanced')
  const engine = ref<string>('auto')
  const activeSemesterId = ref<number>(0)
  const activeSemesterName = ref<string>('')
  const semesters = ref<Array<{ ID: number; name: string; isActive: boolean }>>([])
  const selectedSemesterId = ref<number>(0)

  // Load semesters list + active semester on init
  async function loadActiveSemester() {
    try {
      const sem = await GetActiveSemester()
      if (sem) {
        activeSemesterId.value = sem.ID
        activeSemesterName.value = sem.name
        config.value.semester = sem.name
        selectedSemesterId.value = sem.ID
      }
    } catch { /* no active semester */ }

    try {
      const list = await GetSemesters()
      if (list) {
        semesters.value = list.map(s => ({ ID: s.ID, name: s.name, isActive: s.isActive }))
      }
    } catch { /* backend unavailable */ }
  }

  // Apply preset weights
  function applyPreset(name: string) {
    activePreset.value = name
    const preset = CONSTRAINT_PRESETS.find(p => p.name === name)
    if (preset) {
      constraintWeights.value = { ...preset.weights }
    }
  }

  const isRunning = ref(false)
  const progress = ref(0)
  const result = ref<SchedulingResult | null>(null)
  const logs = ref<string[]>([])
  const progressHistory = ref<{ progress: number; stage: string }[]>([])

  const progressText = computed(() => {
    if (progress.value >= 100) return '排课完成 100%'
    return `已完成 ${progress.value}%`
  })

  /** Current stage label from progress history */
  const currentStage = computed(() => {
    const h = progressHistory.value
    if (h.length === 0) return ''
    return h[h.length - 1].stage
  })

  function toggleConstraint(key: string) {
    const idx = config.value.constraints.indexOf(key)
    if (idx >= 0) { config.value.constraints.splice(idx, 1) }
    else { config.value.constraints.push(key) }
  }

  function resetProgress() {
    progress.value = 0
    logs.value = []
    result.value = null
    progressHistory.value = []
    isRunning.value = false
  }

  // Start scheduling with current config
  async function startScheduling() {
    if (isRunning.value) return
    isRunning.value = true
    progress.value = 5
    logs.value = ['🔍 正在加载教学任务数据...']

    try {
      // Load locked slots from localStorage (pass raw JSON to avoid Wails enum serialization)
      let lockedSlotsJson = ''
      try {
        const saved = localStorage.getItem('locked-time-slots')
        if (saved) { lockedSlotsJson = saved }
      } catch {}

      // Build config for Go backend
      const selectedSem = semesters.value.find(s => s.ID === selectedSemesterId.value)
      const semesterName = selectedSem?.name || activeSemesterName.value
      if (!semesterName) {
        logs.value.push('❌ 未设置当前学期，请在系统设置中激活一个学期')
        result.value = { totalCourses: 0, scheduled: 0, tasksScheduled: 0, conflicts: 0, teacherConflicts: 0, roomConflicts: 0, classConflicts: 0, utilization: 0, logs: ['未设置当前学期'] }
        progress.value = 100
        isRunning.value = false
        return
      }
      const goConfig: any = {
        scope: config.value.scope,
        semester: semesterName,
        strategy: 'auto',
        iterations: 5000,
        timeLimit: 60,
        constraints: config.value.constraints,
        lockedSlotsJson: lockedSlotsJson || undefined,
        semesterId: selectedSemesterId.value || activeSemesterId.value,
        constraintWeights: constraintWeights.value,
      }
      progress.value = 20
      logs.value.push('⚙️ 正在清空旧课表，准备排课...')

      const goResult = await RunScheduling(goConfig)
      console.log('[DEBUG] RunScheduling result:', goResult)
      console.log('[DEBUG] logs field:', goResult?.logs)
      console.log('[DEBUG] error field:', goResult?.error)
      progress.value = 90
      logs.value.push(...(goResult?.logs || []))
      // Populate stage history from backend
      if (goResult?.progressHistory) {
        progressHistory.value = goResult.progressHistory
      }

      // Check for backend-reported errors (no data, missing resources, etc.)
      if (goResult?.error) {
        logs.value.push('❌ ' + goResult.error)
        result.value = {
          totalCourses: goResult.totalCourses || 0,
          scheduled: goResult.scheduled || 0,
          tasksScheduled: goResult.tasksScheduled || 0,
          conflicts: goResult.conflicts || 0,
          teacherConflicts: goResult.teacherConflicts || 0,
          roomConflicts: goResult.roomConflicts || 0,
          classConflicts: goResult.classConflicts || 0,
          utilization: goResult.utilization || 0,
          logs: goResult.logs || [],
        }
        progress.value = 100
        isRunning.value = false
        // Trigger error dialog via uiStore (decoupled from app store)
        const uiStore = useUiStore()
        uiStore.pendingScheduleError = goResult.error
        return
      }

      if (goResult) {
        result.value = {
          totalCourses: goResult.totalCourses || 0,
          scheduled: goResult.scheduled || 0,
          tasksScheduled: goResult.tasksScheduled || 0,
          conflicts: goResult.conflicts || 0,
          teacherConflicts: goResult.teacherConflicts || 0,
          roomConflicts: goResult.roomConflicts || 0,
          classConflicts: goResult.classConflicts || 0,
          utilization: goResult.utilization || 0,
          score: goResult.score,
          scoreDetail: goResult.scoreDetail ? {
            total: goResult.scoreDetail.total || 0,
            teacherPref: goResult.scoreDetail.teacherPref || 0,
            courseSpacing: goResult.scoreDetail.courseSpacing || 0,
            teacherDays: goResult.scoreDetail.teacherDays || 0,
            lowFloorPref: goResult.scoreDetail.lowFloorPref || 0,
            weekendAvoid: (goResult.scoreDetail as any).weekendAvoid || 0,
            pePeriodPref: (goResult.scoreDetail as any).pePeriodPref || 0,
            studentFatigue: (goResult.scoreDetail as any).studentFatigue || 0,
            perCategoryMax: goResult.scoreDetail.perCategoryMax || 25,
            enabledCategoryCount: goResult.scoreDetail.enabledCategoryCount || 4,
          } : undefined,
          logs: goResult.logs || [],
        }
      }
      progress.value = 100
      logs.value.push('✅ 排课完成！正在加载课表...')
      // Ask user whether to navigate to schedule
      const { useScheduleStore } = await import('./schedule')
      const { useAppStore } = await import('./app')
      const appStore = useAppStore()
      const uiStore = useUiStore()
      await useScheduleStore().loadSchedule(appStore.semesterFilter)
      // Show dialog via uiStore (decoupled)
      uiStore.pendingScheduleNav = true
    } catch (e) {
      console.warn('Go backend scheduling not available:', e)
      result.value = { totalCourses: 0, scheduled: 0, tasksScheduled: 0, conflicts: 0, teacherConflicts: 0, roomConflicts: 0, classConflicts: 0, utilization: 0, logs: ['后端调度服务不可用，请检查Go服务是否运行'] }
      progress.value = 100
    }
    isRunning.value = false
  }

  // Initialize — deferred to async to avoid blocking render
  function init() {
    ensureLockedSlots()
    loadActiveSemester()
  }
  // Defer to next microtask
  Promise.resolve().then(init)

  return {
    config, constraintOptions,
    constraintWeights, activePreset, engine, engineOptions,
    activeSemesterId, activeSemesterName, semesters, selectedSemesterId,
    CONSTRAINT_PRESETS,
    isRunning, progress, result, logs, progressHistory, progressText, currentStage,
    toggleConstraint, resetProgress, startScheduling,
    applyPreset,
  }
})
