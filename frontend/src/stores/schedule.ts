import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { ScheduleEntry, ScheduleView } from '@/types'
import { GetScheduleEntries } from '../../bindings/scheduling-system/backend/services/resourceservice'

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

  // Perspective filtering — three dimensions: teacher, classroom, class
  const perspective = ref<'teacher' | 'classroom' | 'class'>('teacher')
  const selectedTeacherId = ref<number | null>(null)
  const selectedClassroomId = ref<number | null>(null)
  const selectedClassId = ref<number | null>(null)

  const displayEntries = computed(() => {
    if (perspective.value === 'teacher' && selectedTeacherId.value) {
      return entries.value.filter(e => e.teacherId === selectedTeacherId.value)
    }
    if (perspective.value === 'classroom' && selectedClassroomId.value) {
      return entries.value.filter(e => e.classroomId === selectedClassroomId.value)
    }
    if (perspective.value === 'class' && selectedClassId.value) {
      return entries.value.filter(e => {
        // Check if entry's class group matches
        if (e.classGroupId === selectedClassId.value) return true
        // Also check via TeachingTask association
        const tt = (e as any).teachingTask
        if (tt?.classes) {
          return tt.classes.some((c: any) => c.classGroupId === selectedClassId.value || c.ClassGroupID === selectedClassId.value)
        }
        return false
      })
    }
    return []
  })

  function setPerspective(p: 'teacher' | 'classroom' | 'class') {
    perspective.value = p
    selectedTeacherId.value = null
    selectedClassroomId.value = null
    selectedClassId.value = null
  }

  const totalCourses = computed(() => entries.value.length)
  const filteredCount = computed(() => displayEntries.value.length)

  function getEntryAt(day: number, period: number): ScheduleEntry | undefined {
    return displayEntries.value.find(e => e.dayOfWeek === day && period >= e.startPeriod && period < e.startPeriod + e.span)
  }

  async function loadSchedule(semester: string) {
    isLoading.value = true
    try {
      const data = await GetScheduleEntries(semester)
      entries.value = (data || []) as ScheduleEntry[]
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
    entries, displayEntries, totalCourses, filteredCount, isLoading,
    perspective, selectedTeacherId, selectedClassroomId, selectedClassId,
    setPerspective,
    getEntryAt, loadSchedule,
  }
})
