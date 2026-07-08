import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { PageId, ScheduleView } from '@/types'
import { GetActiveSemester, GetSemesters } from '../../bindings/scheduling-system/services/resourceservice'

/**
 * 全局应用状态：主题、导航、侧栏
 */
export const useAppStore = defineStore('app', () => {
  // ===== 主题 =====
  const theme = ref<'light' | 'dark'>('light')

  function toggleTheme() {
    theme.value = theme.value === 'light' ? 'dark' : 'light'
    document.documentElement.setAttribute('data-theme', theme.value)
  }

  function setTheme(t: 'light' | 'dark') {
    theme.value = t
    document.documentElement.setAttribute('data-theme', t)
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
    }
    return titles[currentPage.value]
  })

  interface NavItem {
    label: string
    icon: string
    children?: { label: string; page: PageId; scheduleView?: ScheduleView }[]
  }

  const navGroups = ref<NavItem[]>([
    {
      label: '课表中心',
      icon: 'calendar',
      children: [
        { label: '周视图课表', page: 'schedule', scheduleView: 'week' },
        { label: '时间线视图', page: 'schedule', scheduleView: 'timeline' },
        { label: '月视图', page: 'schedule', scheduleView: 'month' },
      ],
    },
    {
      label: '资源管理',
      icon: 'people',
      children: [
        { label: '教师管理', page: 'resource' },
        { label: '教室管理', page: 'resource' },
        { label: '课程管理', page: 'resource' },
        { label: '班级管理', page: 'resource' },
      ],
    },
    {
      label: '排课引擎',
      icon: 'settings',
      children: [
        { label: '自动排课', page: 'scheduling' },
        { label: '验证报告', page: 'report' },
      ],
    },
    {
      label: '系统设置',
      icon: 'sun',
      children: [
        { label: '基本设置', page: 'settings' },
        { label: '学期管理', page: 'settings' },
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
    if (breadcrumb) {
      breadcrumbPath.value = [pageTitle.value, breadcrumb]
    }
  }

  // ===== 搜索 =====
  const searchQuery = ref('')
  const deptFilter = ref('全部院系')
  const semesterFilter = ref('')  // loaded from active semester
  const semesters = ref<Array<{ ID: number; name: string }>>([])  // all semesters from DB
  const pendingScheduleNav = ref(false) // trigger confirmation dialog after scheduling

  const semesterOptions = computed(() =>
    semesters.value.map(s => ({ label: s.name, value: s.name }))
  )

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
      semesters.value = result || []
    } catch { /* backend unavailable */ }
  }

  return {
    // theme
    theme,
    toggleTheme,
    setTheme,
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
    pendingScheduleNav,
    loadSemesters,
  }
})
