import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { ScheduleEntry, ScheduleView } from '@/types'
import { GetScheduleEntries } from '../../bindings/scheduling-system/services/resourceservice'

export const useScheduleStore = defineStore('schedule', () => {
  const currentView = ref<ScheduleView>('week')
  const currentWeek = ref(8)
  const currentMonth = ref(new Date().getMonth() + 1)
  const currentYear = ref(new Date().getFullYear())
  const isLoading = ref(false)

  function switchView(view: ScheduleView) { currentView.value = view }
  function prevWeek() { if (currentWeek.value > 1) currentWeek.value-- }
  function nextWeek() { if (currentWeek.value < 20) currentWeek.value++ }
  function prevMonth() {
    if (currentMonth.value === 1) { currentMonth.value = 12; currentYear.value-- }
    else { currentMonth.value-- }
  }
  function nextMonth() {
    if (currentMonth.value === 12) { currentMonth.value = 1; currentYear.value++ }
    else { currentMonth.value++ }
  }

  const entries = ref<ScheduleEntry[]>([])
  const totalCourses = computed(() => entries.value.length)
  const conflictCount = ref(0)

  function getEntryAt(day: number, period: number): ScheduleEntry | undefined {
    return entries.value.find(e => e.dayOfWeek === day && e.startPeriod === period)
  }

  async function loadSchedule(semester: string) {
    isLoading.value = true
    try {
      const data = await GetScheduleEntries(semester)
      entries.value = data || []
    } catch (e) {
      console.warn('Failed to load schedule from Go backend:', e)
      entries.value = []
    } finally {
      isLoading.value = false
    }
  }

  return {
    currentView, currentWeek, currentMonth, currentYear,
    switchView, prevWeek, nextWeek, prevMonth, nextMonth,
    entries, totalCourses, conflictCount, isLoading,
    getEntryAt, loadSchedule,
  }
})
