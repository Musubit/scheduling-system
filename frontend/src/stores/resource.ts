import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { Teacher, Classroom, Course, ClassGroup, TeachingTask } from '@/types'
import { GetTeachers, GetClassrooms, GetCourses, GetClassGroups, GetActiveSemester } from '../../bindings/scheduling-system/backend/services/resourceservice'
import { ListTeachingTasks } from '../../bindings/scheduling-system/backend/services/teachingtaskservice'
import { useAppStore } from './app'

/**
 * 资源管理状态：教师、教室、课程、班级、教学任务
 */
export const useResourceStore = defineStore('resource', () => {
  const activeTab = ref<'teacher' | 'classroom' | 'course' | 'class' | 'teachingTask'>('teacher')
  const isLoading = ref(false)
  const appStore = useAppStore()

  // Match dept against filter (all depts are now Chinese names)
  function deptMatch(itemDept: string): boolean {
    if (appStore.deptFilter === '全部院系') return true
    return itemDept === appStore.deptFilter
  }

	function switchTab(tab: 'teacher' | 'classroom' | 'course' | 'class' | 'teachingTask') {
	  activeTab.value = tab
	}

	  // ===== Data =====
	  const teachers = ref<Teacher[]>([])
	  const classrooms = ref<Classroom[]>([])
	  const courses = ref<Course[]>([])
	  const classGroups = ref<ClassGroup[]>([])
	  const teachingTasks = ref<TeachingTask[]>([])

	  // ===== Filters =====
	  const teacherSearch = ref('')
	  const classroomSearch = ref('')
	  const courseSearch = ref('')
	  const classSearch = ref('')
	  const teachingTaskSearch = ref('')

  // ===== Computed filtered lists =====
  const filteredTeachers = computed(() => {
    let list = teachers.value
    if (teacherSearch.value) {
      const q = teacherSearch.value.toLowerCase()
      list = list.filter(t => t.name.includes(q) || t.code.toLowerCase().includes(q))
    }
    list = list.filter(t => deptMatch(t.dept))
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
    list = list.filter(c => deptMatch(c.dept))
    return list
  })

	  const filteredClasses = computed(() => {
	    let list = classGroups.value
	    if (classSearch.value) {
	      const q = classSearch.value.toLowerCase()
	      list = list.filter(c => c.name.includes(q) || c.code.toLowerCase().includes(q))
	    }
	    list = list.filter(c => deptMatch(c.dept))
	    return list
	  })

	  const filteredTeachingTasks = computed(() => {
	    let list = teachingTasks.value
	    if (teachingTaskSearch.value) {
	      const q = teachingTaskSearch.value.toLowerCase()
	      list = list.filter(t => 
	        (t.course?.name || '').toLowerCase().includes(q) ||
	        (t.teacher?.name || '').toLowerCase().includes(q)
	      )
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
	      teachers.value = (t || []) as Teacher[]
	      classrooms.value = (c || []) as Classroom[]
	      courses.value = (co || []) as Course[]
	      classGroups.value = (cg || []) as ClassGroup[]
	    } catch (e) {
	      console.warn('Failed to load resources from Go backend, using empty data:', e)
	    } finally {
	      isLoading.value = false
	    }
	  }

	  async function loadTeachingTasks(semesterID: number) {
	    try {
	      teachingTasks.value = (await ListTeachingTasks(semesterID) || []) as TeachingTask[]
	    } catch (e) {
	      console.warn('Failed to load teaching tasks:', e)
	    }
	  }

	  return {
	    activeTab,
	    isLoading,
	    switchTab,
	    teachers, classrooms, courses, classGroups, teachingTasks,
	    filteredTeachers, filteredClassrooms, filteredCourses, filteredClasses, filteredTeachingTasks,
	    teacherSearch, classroomSearch, courseSearch, classSearch, teachingTaskSearch,
	    loadAll, loadTeachingTasks,
	  }
})
