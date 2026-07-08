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

  // ===== 约束切换 =====
  function toggleConstraint(key: string) {
    const idx = config.value.constraints.indexOf(key)
    if (idx >= 0) {
      config.value.constraints.splice(idx, 1)
    } else {
      config.value.constraints.push(key)
    }
  }

  // ===== 执行排课 =====
  let timer: ReturnType<typeof setInterval> | null = null

  async function startScheduling() {
    if (isRunning.value) return
    isRunning.value = true
    progress.value = 0
    result.value = null
    logs.value = [`[14:32:01] INFO  排课引擎启动，共 248 门课程待排`, `[14:32:02] INFO  加载约束条件：${config.value.constraints.length} 条规则已启用`]

    // 模拟排课进度
    timer = setInterval(() => {
      if (!isRunning.value) return
      progress.value += Math.random() * 4 + 1
      if (progress.value >= 100) {
        progress.value = 100
        stopScheduling()
        result.value = {
          totalCourses: 248,
          scheduled: 186,
          conflicts: 3,
          utilization: 0.942,
          logs: [],
        }
        logs.value.push('[14:35:26] INFO  排课完成！已排 186 门，教室利用率 94.2%')
        logs.value.push('[14:35:26] WARN  剩余 62 门课程需手动调整')
      } else if (Math.random() < 0.3) {
        const msgs = [
          `[14:32:${String(Math.floor(Math.random() * 60)).padStart(2, '0')}] INFO  第 ${Math.floor(progress.value / 5)} 轮迭代完成`,
          `[14:33:${String(Math.floor(Math.random() * 60)).padStart(2, '0')}] WARN  发现教师时间冲突`,
          `[14:34:${String(Math.floor(Math.random() * 60)).padStart(2, '0')}] ERR  教室容量不足`,
        ]
        logs.value.push(msgs[Math.floor(Math.random() * msgs.length)])
      }
    }, 200)
  }

  function stopScheduling() {
    isRunning.value = false
    if (timer) {
      clearInterval(timer)
      timer = null
    }
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
    toggleConstraint,
    startScheduling,
    stopScheduling,
  }
})
