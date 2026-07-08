import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { Teacher, Classroom, Course, ClassGroup } from '@/types'

/**
 * 资源管理状态：教师、教室、课程、班级
 */
export const useResourceStore = defineStore('resource', () => {
  // ===== 当前资源类型 Tab =====
  const activeTab = ref<'teacher' | 'classroom' | 'course' | 'class'>('teacher')

  function switchTab(tab: 'teacher' | 'classroom' | 'course' | 'class') {
    activeTab.value = tab
  }

  // ===== 教师 =====
  const teachers = ref<Teacher[]>([])
  const teacherSearch = ref('')
  const teacherDeptFilter = ref('全部院系')

  // ===== 教室 =====
  const classrooms = ref<Classroom[]>([])
  const classroomSearch = ref('')

  // ===== 课程 =====
  const courses = ref<Course[]>([])
  const courseSearch = ref('')

  // ===== 班级 =====
  const classGroups = ref<ClassGroup[]>([])
  const classSearch = ref('')

  // ===== 加载 =====
  async function loadAll() {
    // TODO: 阶段3接入 Wails binding
  }

  return {
    activeTab,
    switchTab,
    teachers,
    teacherSearch,
    teacherDeptFilter,
    classrooms,
    classroomSearch,
    courses,
    courseSearch,
    classGroups,
    classSearch,
    loadAll,
  }
})
