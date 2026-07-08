import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { SchedulingConfig, SchedulingResult } from '@/types'

/**
 * 排课引擎状态
 */
export const useSchedulingStore = defineStore('scheduling', () => {
  // ===== 排课配置 =====
  const config = ref<SchedulingConfig>({
    scope: '全校所有院系',
    semester: '2025-2026 第二学期',
    strategy: 'teacher_first',
    iterations: 5000,
    constraints: [
      'no_consecutive_teacher',  // 教师不连续排课
      'course_dispersed',         // 同一课程分散安排
      'large_class_large_room',   // 大班优先大教室
      'coordinated_classes',      // 合班课时间协调
    ],
  })

  // 约束选项
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

  // ===== 排课状态 =====
  const isRunning = ref(false)
  const progress = ref(0)
  const result = ref<SchedulingResult | null>(null)
  const logs = ref<string[]>([])

  const progressText = computed(() => {
    if (progress.value >= 100) return '排课完成 100%'
    return `已完成 ${progress.value}%`
  })

  function resetProgress() {
    progress.value = 0
    logs.value = []
    result.value = null
    isRunning.value = false
  }

  // ===== 执行排课 =====
  async function startScheduling() {
    if (isRunning.value) return
    isRunning.value = true
    progress.value = 0
    logs.value = []

    // TODO: 阶段3接入 Wails binding 调用 Go 排课算法
    // 当前为模拟进度
    logs.value.push('[排课] 排课引擎启动...')
  }

  return {
    config,
    constraintOptions,
    strategyOptions,
    isRunning,
    progress,
    result,
    logs,
    progressText,
    resetProgress,
    startScheduling,
  }
})
