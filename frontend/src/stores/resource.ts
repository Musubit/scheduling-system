import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { Teacher, Classroom, Course, ClassGroup } from '@/types'
import { GetTeachers, GetClassrooms, GetCourses, GetClassGroups } from '../../bindings/scheduling-system/services/resourceservice'
import type { models } from '../../bindings/scheduling-system/services/models'

/**
 * 资源管理状态：教师、教室、课程、班级
 */
export const useResourceStore = defineStore('resource', () => {
  const activeTab = ref<'teacher' | 'classroom' | 'course' | 'class'>('teacher')
  const isLoading = ref(false)

  function switchTab(tab: 'teacher' | 'classroom' | 'course' | 'class') {
    activeTab.value = tab
  }

  // ===== Data =====
  const teachers = ref<Teacher[]>([])
  const classrooms = ref<Classroom[]>([])
  const courses = ref<Course[]>([])
  const classGroups = ref<ClassGroup[]>([])

  // ===== Filters =====
  const teacherSearch = ref('')
  const teacherDeptFilter = ref('全部院系')
  const classroomSearch = ref('')
  const courseSearch = ref('')
  const classSearch = ref('')

  // ===== Computed filtered lists =====
  const filteredTeachers = computed(() => {
    let list = teachers.value
    if (teacherSearch.value) {
      const q = teacherSearch.value.toLowerCase()
      list = list.filter(t => t.name.includes(q) || t.code.toLowerCase().includes(q))
    }
    if (teacherDeptFilter.value !== '全部院系') {
      list = list.filter(t => t.dept === teacherDeptFilter.value)
    }
    return list
  })

  const filteredClassrooms = computed(() => {
    if (!classroomSearch.value) return classrooms.value
    const q = classroomSearch.value.toLowerCase()
    return classrooms.value.filter(c => c.name.toLowerCase().includes(q) || c.code.toLowerCase().includes(q))
  })

  const filteredCourses = computed(() => {
    if (!courseSearch.value) return courses.value
    const q = courseSearch.value.toLowerCase()
    return courses.value.filter(c => c.name.includes(q) || c.code.toLowerCase().includes(q))
  })

  const filteredClasses = computed(() => {
    if (!classSearch.value) return classGroups.value
    const q = classSearch.value.toLowerCase()
    return classGroups.value.filter(c => c.name.includes(q) || c.code.toLowerCase().includes(q))
  })

  // ===== Load from Go backend =====
  async function loadAll() {
    isLoading.value = true
    try {
      const [t, c, co, cg] = await Promise.all([
        GetTeachers(),
        GetClassrooms(),
        GetCourses(),
        GetClassGroups(),
      ])
      teachers.value = t || []
      classrooms.value = c || []
      courses.value = co || []
      classGroups.value = cg || []
    } catch (e) {
      console.warn('Failed to load resources from Go backend, using empty data:', e)
    } finally {
      isLoading.value = false
    }
  }

  return {
    activeTab,
    isLoading,
    switchTab,
    teachers, classrooms, courses, classGroups,
    filteredTeachers, filteredClassrooms, filteredCourses, filteredClasses,
    teacherSearch, teacherDeptFilter, classroomSearch, courseSearch, classSearch,
    loadAll,
  }
})
