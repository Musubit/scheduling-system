import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { ScheduleEntry, ScheduleView } from '@/types'
import { PERIODS, DAY_NAMES } from '@/types'

/**
 * 课表状态：视图模式、排课数据
 */
export const useScheduleStore = defineStore('schedule', () => {
  // ===== 视图模式 =====
  const currentView = ref<ScheduleView>('week')
  const currentWeek = ref(8)  // 当前第几周
  const currentMonth = ref(new Date().getMonth() + 1)
  const currentYear = ref(new Date().getFullYear())

  function switchView(view: ScheduleView) {
    currentView.value = view
  }

  function prevWeek() {
    if (currentWeek.value > 1) currentWeek.value--
  }

  function nextWeek() {
    if (currentWeek.value < 20) currentWeek.value++
  }

  function prevMonth() {
    if (currentMonth.value === 1) {
      currentMonth.value = 12
      currentYear.value--
    } else {
      currentMonth.value--
    }
  }

  function nextMonth() {
    if (currentMonth.value === 12) {
      currentMonth.value = 1
      currentYear.value++
    } else {
      currentMonth.value++
    }
  }

  // ===== 排课数据 =====
  const entries = ref<ScheduleEntry[]>([])
  const totalCourses = computed(() => entries.value.length)
  const conflictCount = ref(0)

  // 按天+节次查找课程
  function getEntryAt(day: number, period: number): ScheduleEntry | undefined {
    return entries.value.find(e => e.dayOfWeek === day && e.startPeriod === period)
  }

  // 加载课表（从后端）
  async function loadSchedule(semester: string) {
    // TODO: 阶段3接入 Wails binding
    entries.value = []
  }

  return {
    currentView,
    currentWeek,
    currentMonth,
    currentYear,
    switchView,
    prevWeek,
    nextWeek,
    prevMonth,
    nextMonth,
    entries,
    totalCourses,
    conflictCount,
    getEntryAt,
    loadSchedule,
  }
})
