import { defineStore, storeToRefs } from 'pinia'
import { ref, computed } from 'vue'
import type { Teacher, Classroom, Course, ClassGroup } from '@/types'
import { GetTeachers, GetClassrooms, GetCourses, GetClassGroups } from '../../bindings/scheduling-system/services/resourceservice'
import type { models } from '../../bindings/scheduling-system/services/models'
import { useAppStore } from './app'

/**
 * 资源管理状态：教师、教室、课程、班级
 */
export const useResourceStore = defineStore('resource', () => {
  const activeTab = ref<'teacher' | 'classroom' | 'course' | 'class'>('teacher')
  const isLoading = ref(false)
  const appStore = useAppStore()

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
    if (appStore.deptFilter !== '全部院系') {
      list = list.filter(t => t.dept === appStore.deptFilter)
    }
    return list
  })

  const filteredClassrooms = computed(() => {
    if (!classroomSearch.value) return classrooms.value
    const q = classroomSearch.value.toLowerCase()
    return classrooms.value.filter(c => c.name.toLowerCase().includes(q) || c.code.toLowerCase().includes(q))
  })

  const filteredCourses = computed(() => {
    let list = courses.value
    if (courseSearch.value) {
      const q = courseSearch.value.toLowerCase()
      list = list.filter(c => c.name.includes(q) || c.code.toLowerCase().includes(q))
    }
    if (appStore.deptFilter !== '全部院系') {
      list = list.filter(c => c.dept === appStore.deptFilter)
    }
    return list
  })

  const filteredClasses = computed(() => {
    let list = classGroups.value
    if (classSearch.value) {
      const q = classSearch.value.toLowerCase()
      list = list.filter(c => c.name.includes(q) || c.code.toLowerCase().includes(q))
    }
    if (appStore.deptFilter !== '全部院系') {
      list = list.filter(c => c.dept === appStore.deptFilter)
    }
    return list
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
    teacherSearch, classroomSearch, courseSearch, classSearch,
    loadAll,
  }
})
