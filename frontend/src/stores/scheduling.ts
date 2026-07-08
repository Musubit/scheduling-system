import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { SchedulingConfig, SchedulingResult } from '@/types'
import { RunScheduling } from '../../bindings/scheduling-system/services/schedulingservice'
import type { models } from '../../bindings/scheduling-system/services/models'

export const useSchedulingStore = defineStore('scheduling', () => {
  const config = ref<SchedulingConfig>({
    scope: '全校所有院系',
    semester: '2025-2026 第二学期',
    strategy: 'teacher_first',
    iterations: 5000,
    constraints: ['no_consecutive_teacher', 'course_dispersed', 'large_class_large_room', 'coordinated_classes'],
  })

  const constraintOptions = [
    { key: 'teacher_preference', label: '教师偏好时段（避免早课/晚课）' },
    { key: 'course_dispersed', label: '同一课程分散安排（不集中在同一天）' },
    { key: 'teacher_days_limit', label: '教师到校天数限制（每周≤3天）' },
    { key: 'low_floor_preference', label: '优先低楼层教室' },
    { key: 'avoid_saturday', label: '尽量避开周六排课' },
    { key: 'avoid_sunday', label: '尽量避开周日排课' },
  ]

  const strategyOptions = [
    { value: 'teacher_first', label: '教师时间优先' },
    { value: 'room_utilization', label: '教室利用率优先' },
    { value: 'student_balance', label: '学生课表均衡优先' },
  ]

  const isRunning = ref(false)
  const progress = ref(0)
  const result = ref<SchedulingResult | null>(null)
  const logs = ref<string[]>([])

  const progressText = computed(() => {
    if (progress.value >= 100) return '排课完成 100%'
    return `已完成 ${progress.value}%`
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
    isRunning.value = false
  }

  // Real scheduling via Go backend
  async function startScheduling() {
    if (isRunning.value) return
    isRunning.value = true
    progress.value = 5
    logs.value = ['🔍 正在加载课程和教师数据...']

    try {
      // Load locked slots from localStorage
      let lockedSlots: { dayOfWeek: number; startPeriod: number; span: number }[] = []
      try {
        const saved = localStorage.getItem('locked-time-slots')
        if (saved) { lockedSlots = JSON.parse(saved) }
      } catch {}

      const goConfig: models.SchedulingConfig = {
        scope: config.value.scope,
        semester: config.value.semester,
        strategy: config.value.strategy,
        iterations: config.value.iterations,
        constraints: config.value.constraints,
        lockedSlots: lockedSlots.length > 0 ? lockedSlots : undefined,
      }
      progress.value = 20
      logs.value.push('⚙️ 正在清空旧课表，准备排课...')

      const goResult = await RunScheduling(goConfig)
      progress.value = 90
      logs.value.push(...(goResult?.logs || []))

      if (goResult) {
        result.value = {
          totalCourses: goResult.totalCourses || 0,
          scheduled: goResult.scheduled || 0,
          conflicts: goResult.conflicts || 0,
          utilization: goResult.utilization || 0,
          score: goResult.score,
          scoreDetail: goResult.scoreDetail ? {
            total: goResult.scoreDetail.total || 0,
            teacherPref: goResult.scoreDetail.teacherPref || 0,
            courseSpacing: goResult.scoreDetail.courseSpacing || 0,
            teacherDays: goResult.scoreDetail.teacherDays || 0,
            lowFloorPref: goResult.scoreDetail.lowFloorPref || 0,
          } : undefined,
          logs: goResult.logs || [],
        }
      }
      progress.value = 100
      logs.value.push('✅ 排课完成！正在加载课表...')
      // Refresh schedule views and navigate to schedule
      const { useScheduleStore } = await import('./schedule')
      const { useAppStore } = await import('./app')
      const appStore = useAppStore()
      await useScheduleStore().loadSchedule(appStore.semesterFilter)
      appStore.navigateTo('schedule', '周视图课表')
    } catch (e) {
      console.warn('Go backend scheduling not available:', e)
      result.value = { totalCourses: 0, scheduled: 0, conflicts: 0, utilization: 0, logs: ['后端调度服务不可用，请检查Go服务是否运行'] }
      progress.value = 100
    }
    isRunning.value = false
  }

  function stopScheduling() { isRunning.value = false }

  return {
    config, constraintOptions, strategyOptions,
    isRunning, progress, result, logs, progressText,
    toggleConstraint, resetProgress, startScheduling, stopScheduling,
  }
})
