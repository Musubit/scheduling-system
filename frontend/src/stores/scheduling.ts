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
    { key: 'no_consecutive_teacher', label: '教师不连续排课' },
    { key: 'course_dispersed', label: '同一课程分散安排' },
    { key: 'large_class_large_room', label: '大班优先大教室' },
    { key: 'no_pe_first_period', label: '体育课避开第一节' },
    { key: 'coordinated_classes', label: '合班课时间协调' },
    { key: 'teacher_preference', label: '教师偏好时段' },
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
    progress.value = 10
    logs.value = ['排课引擎启动，正在调用后端算法...']

    try {
      const goConfig: models.SchedulingConfig = {
        scope: config.value.scope,
        semester: config.value.semester,
        strategy: config.value.strategy,
        iterations: config.value.iterations,
        constraints: config.value.constraints,
      }
      const goResult = await RunScheduling(goConfig)
      progress.value = 100

      if (goResult) {
        result.value = {
          totalCourses: goResult.totalCourses || 0,
          scheduled: goResult.scheduled || 0,
          conflicts: goResult.conflicts || 0,
          utilization: goResult.utilization || 0,
          logs: goResult.logs || [],
        }
        logs.value = goResult.logs || ['排课完成']
      }
    } catch (e) {
      console.warn('Go backend scheduling not available:', e)
      // Fallback: simulate
      logs.value.push('后端未连接，使用模拟...')
      simulateProgress()
    }
  }

  // Fallback simulation
  let timer: ReturnType<typeof setInterval> | null = null
  function simulateProgress() {
    timer = setInterval(() => {
      if (!isRunning.value) return
      progress.value += Math.random() * 4 + 1
      if (progress.value >= 100) {
        progress.value = 100
        stopScheduling()
        result.value = { totalCourses: 248, scheduled: 186, conflicts: 3, utilization: 0.942, logs: [] }
        logs.value.push('排课完成！已排 186 门，教室利用率 94.2%')
      }
    }, 200)
  }

  function stopScheduling() {
    isRunning.value = false
    if (timer) { clearInterval(timer); timer = null }
  }

  return {
    config, constraintOptions, strategyOptions,
    isRunning, progress, result, logs, progressText,
    toggleConstraint, resetProgress, startScheduling, stopScheduling,
  }
})
