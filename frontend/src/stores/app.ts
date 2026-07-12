import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { PageId, ScheduleView } from '@/types'
import { GetActiveSemester, GetSemesters } from '../../bindings/scheduling-system/backend/services/resourceservice'

/**
 * 全局应用状态：主题、导航、侧栏
 */
export const useAppStore = defineStore('app', () => {
  // ===== 侧边栏折叠 =====
  const sidebarCollapsed = ref(
    localStorage.getItem('sidebar-collapsed') === 'true'
  )

  function toggleSidebar() {
    sidebarCollapsed.value = !sidebarCollapsed.value
    localStorage.setItem('sidebar-collapsed', String(sidebarCollapsed.value))
  }

  // ===== 导航 =====
  const currentPage = ref<PageId>('schedule')
  const breadcrumbPath = ref<string[]>(['课表中心', '周视图课表'])

  const pageTitle = computed(() => {
    const titles: Record<PageId, string> = {
      schedule: '课表中心',
      resource: '资源管理',
      scheduling: '自动排课',
      report: '验证报告',
      settings: '系统设置',
      history: '历史课表对比',
      system: '系统管理',
    }
    return titles[currentPage.value]
  })

  interface NavChild {
    label: string
    page: PageId
    scheduleView?: ScheduleView
    resourceTab?: 'teacher' | 'classroom' | 'course' | 'class' | 'teachingTask'
  }

  interface NavItem {
    label: string
    icon: string   // SVG path data
    children: NavChild[]
  }

  const navGroups = ref<NavItem[]>([
    {
      label: '课表中心',
      icon: 'M3 6h18M3 12h18M3 18h12',
      children: [
        { label: '周视图课表', page: 'schedule', scheduleView: 'week' },
        { label: '时间线视图', page: 'schedule', scheduleView: 'timeline' },
        { label: '月视图', page: 'schedule', scheduleView: 'month' },
      ],
    },
    {
      label: '资源管理',
      icon: 'M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2M9 3a4 4 0 0 1 0 7.75M23 21v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75',
      children: [
        { label: '教师管理', page: 'resource', resourceTab: 'teacher' },
        { label: '教室管理', page: 'resource', resourceTab: 'classroom' },
        { label: '课程管理', page: 'resource', resourceTab: 'course' },
        { label: '班级管理', page: 'resource', resourceTab: 'class' },
        { label: '教学任务管理', page: 'resource', resourceTab: 'teachingTask' },
      ],
    },
    {
      label: '排课引擎',
      icon: 'M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z',
      children: [
        { label: '自动排课', page: 'scheduling' },
        { label: '验证报告', page: 'report' },
        { label: '历史课表对比', page: 'history' },
      ],
    },
    {
      label: '系统设置',
      icon: 'M12 1v2M12 21v2M4.22 4.22l1.42 1.42M18.36 18.36l1.42 1.42M1 12h2M21 12h2M4.22 19.78l1.42-1.42M18.36 5.64l1.42-1.42',
      children: [
        { label: '基本设置', page: 'settings' },
        { label: '系统管理', page: 'system' },
      ],
    },
  ])

  // 当前展开的导航组
  const expandedGroups = ref<string[]>(['课表中心', '资源管理'])

  function toggleNavGroup(label: string) {
    const idx = expandedGroups.value.indexOf(label)
    if (idx >= 0) {
      expandedGroups.value.splice(idx, 1)
    } else {
      expandedGroups.value.push(label)
    }
  }

  function navigateTo(page: PageId, breadcrumb?: string) {
    currentPage.value = page
    if (page === 'schedule' || page === 'resource') {
      loadSemesters() // refresh dropdown — store init may have raced with Wails backend
    }
    if (breadcrumb) {
      breadcrumbPath.value = [pageTitle.value, breadcrumb]
    }
  }

  // ===== 搜索 =====
  const searchQuery = ref('')
  const deptFilter = ref('全部院系')
  const semesterFilter = ref('')  // loaded from active semester
  const semesters = ref<Array<{ ID: number; name: string }>>([])  // all semesters from DB
  const semesterOptions = ref<Array<{ label: string; value: string }>>([])  // stable ref for n-select

  // Init: load active semester and all semesters
  async function initSemester() {
    await loadSemesters()
    try {
      const sem = await GetActiveSemester()
      if (sem && sem.name) {
        semesterFilter.value = sem.name
      } else if (semesters.value.length > 0) {
        semesterFilter.value = semesters.value[0].name
      }
    } catch { /* no active semester */ }
  }
  initSemester()

  async function loadSemesters() {
    try {
      const result = await GetSemesters()
      console.log('[appStore] loadSemesters:', result?.map((s: any) => s.name))
      semesters.value = result || []
      semesterOptions.value = (result || []).map((s: any) => ({ label: s.name, value: s.name }))
    } catch (e) {
      console.warn('[appStore] loadSemesters FAILED:', e)
    }
  }

	return {
	    // sidebar
	    sidebarCollapsed,
    toggleSidebar,
    // nav
    currentPage,
    breadcrumbPath,
    pageTitle,
    navGroups,
    expandedGroups,
    toggleNavGroup,
    navigateTo,
    // filters
    searchQuery,
    deptFilter,
    semesterFilter,
    semesters,
    semesterOptions,
    loadSemesters,
  }
})
