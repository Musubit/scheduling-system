import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { EnrichedScheduleEntry, ScheduleEntry, ScheduleView } from '@/types'
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

  // v0.6.2: Use EnrichedScheduleEntry — flattened read-model from TA+SE JOIN.
  const entries = ref<EnrichedScheduleEntry[]>([])

  function isInWeek(entry: EnrichedScheduleEntry, week: number): boolean {
    if (!entry.weeks) return true
    const parts = entry.weeks.split('-')
    if (parts.length !== 2) return true
    const start = parseInt(parts[0], 10)
    const end = parseInt(parts[1], 10)
    if (isNaN(start) || isNaN(end)) return true
    return week >= start && week <= end
  }

  // Perspective filtering — two dimensions: teacher, class
  const perspective = ref<'teacher' | 'class'>('teacher')
  const selectedTeacherId = ref<number | null>(null)
  const selectedClassId = ref<number | null>(null)

  const displayEntries = computed(() => {
    if (perspective.value === 'teacher' && selectedTeacherId.value) {
      return entries.value.filter(e => e.teacherId === selectedTeacherId.value && isInWeek(e, currentWeek.value))
    }
    if (perspective.value === 'class' && selectedClassId.value) {
      return entries.value.filter(e => {
        if (!isInWeek(e, currentWeek.value)) return false
        // EnrichedScheduleEntry has flat classGroupIds[] from TA+SE JOIN
        return e.classGroupIds.includes(selectedClassId.value)
      })
    }
    return []
  })

  function setPerspective(p: 'teacher' | 'class') {
    perspective.value = p
    selectedTeacherId.value = null
    selectedClassId.value = null
  }

  const filteredCount = computed(() => displayEntries.value.length)

  function getEntryAt(day: number, period: number): EnrichedScheduleEntry | undefined {
    return displayEntries.value.find(e => e.dayOfWeek === day && period >= e.startPeriod && period < e.startPeriod + e.span)
  }

  async function loadSchedule(semesterId: number) {
    isLoading.value = true
    try {
      const data = await GetScheduleEntries(semesterId)
      // Backend returns EnrichedScheduleEntry with flat fields.
      // Components expect nested course/teacher/classroom objects — add compatibility wrappers.
      entries.value = (data || []).map(e => ({
        ...e,
        course: { ID: e.courseId, name: e.courseName, code: e.courseCode, credit: e.courseCredit, dept: '' } as any,
        teacher: { ID: e.teacherId, name: e.teacherName } as any,
        classroom: e.classroomId != null ? {
          ID: e.classroomId, name: e.classroomName,
          floor: e.classroomFloor, type: e.classroomType, code: e.classroomCode,
          building: e.buildingName ? { name: e.buildingName } : undefined,
        } as any : undefined,
      })) as any as EnrichedScheduleEntry[]
    } catch (e) {
      console.warn('Failed to load schedule from Go backend:', e)
      entries.value = []
    } finally {
      isLoading.value = false
    }
  }

  // ---- Version browsing support (Epic H2-2) ----

  const viewMode = ref<'current' | 'version'>('current')
  const versionName = ref<string>('')

  /** Load a historical schedule version into the store for read-only viewing. */
  async function loadVersionEntries(versionId: number) {
    viewMode.value = 'version'
    versionName.value = ''
    entries.value = []
    isLoading.value = true

    try {
      const { GetVersion } = await import('../../bindings/scheduling-system/backend/services/versionservice')
      const { GetCourses, GetTeachers, GetClassrooms } =
        await import('../../bindings/scheduling-system/backend/services/resourceservice')
      const { ListTeachingTasks } =
        await import('../../bindings/scheduling-system/backend/services/teachingtaskservice')

      const [version, courses, teachers, classrooms] = await Promise.all([
        GetVersion(versionId),
        GetCourses().catch(() => []),
        GetTeachers().catch(() => []),
        GetClassrooms().catch(() => []),
      ])

      if (!version) {
        viewMode.value = 'current'
        return
      }

      versionName.value = version.name || ''

      // Load teaching tasks for the version's semester
      const teachingTasks = await ListTeachingTasks(version.semesterId).catch(() => [])

      // Build lookup maps for resource resolution
      const courseById = new Map<number, any>((courses || []).map((c: any) => [c.ID, c]))
      const teacherById = new Map<number, any>((teachers || []).map((t: any) => [t.ID, t]))
      const classroomById = new Map<number, any>((classrooms || []).map((c: any) => [c.ID, c]))
      const teachingTaskById = new Map<number, any>((teachingTasks || []).map((t: any) => [t.ID, t]))

      // Convert version entries to ScheduleEntry format
      entries.value = (version.entries || []).map((e: any) => ({
        id: (e.originalEntryId || e.ID) as number,
        dayOfWeek: e.dayOfWeek,
        startPeriod: e.startPeriod,
        span: e.span,
        weeks: e.weeks || '1-16',
        teacherId: e.teacherId,
        teacherName: teacherById.get(e.teacherId)?.name || '',
        courseId: e.courseId,
        courseName: courseById.get(e.courseId)?.name || '',
        classGroupIds: e.classGroupIds || [],
        classGroupNames: [],
        classroomId: e.classroomId,
        semesterId: version.semesterId,
        scheduleVersionId: versionId,
      })) as EnrichedScheduleEntry[]
    } catch (e) {
      console.warn('Failed to load version:', e)
      viewMode.value = 'current'
      entries.value = []
    } finally {
      isLoading.value = false
    }
  }

  /** Return to the live schedule view. */
  async function clearVersionView() {
    viewMode.value = 'current'
    versionName.value = ''
    const { useAppStore } = await import('./app')
    loadSchedule(useAppStore().currentSemesterId)
  }

  // ---- Manual adjustment tracking ----

  const dirtyMoveCount = ref(0)
  function markDirty() {
    dirtyMoveCount.value++
  }
  function clearDirty() {
    dirtyMoveCount.value = 0
  }

  return {
    currentView, currentWeek, currentMonth, currentYear,
    switchView, prevWeek, nextWeek, prevMonth, nextMonth,
    entries, displayEntries, filteredCount, isLoading,
    perspective, selectedTeacherId, selectedClassId,
    setPerspective,
    getEntryAt, loadSchedule,
    viewMode, versionName,
    loadVersionEntries, clearVersionView,
    dirtyMoveCount, markDirty, clearDirty,
  }
})
