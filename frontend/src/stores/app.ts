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
	      'schedule-center': '课表方案',
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
	        { label: '课表方案', page: 'schedule-center' },
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

  // ===== 全局学期上下文 (Single Source of Truth) =====
  // v0.5.5 修订：后端 Semester 已改为结构化字段（academicYear + term + status）。
  // 为避免同期重构大量旧读取点，store 在 loadSemesters() 里派生兼容 `name`/`isActive`。
  const semesters = ref<Array<{ ID: number; name: string; isActive?: boolean; academicYear?: string; term?: string; status?: string; startDate?: string }>>([])
  const currentSemesterId = ref<number>(0)

  // 与后端 Semester.DisplayName() 保持一致，前端 SettingsPage 中 displayName() 同源。
  function semesterDisplayName(s: { academicYear?: string; term?: string; name?: string }): string {
    if (s.name) return s.name // 兼容旧 mock/测试数据
    const label = s.term === 'SECOND' ? '第二学期' : '第一学期'
    return `${s.academicYear || ''}${label}`
  }

  const currentSemester = computed(() =>
    semesters.value.find(s => s.ID === currentSemesterId.value) || null
  )
  const currentSemesterName = computed(() => currentSemester.value?.name || '')

  // semesterFilter: backward-compat writable computed synced to currentSemesterId
  // Legacy code reads appStore.semesterFilter (string) - this proxies to the new state.
  const semesterFilter = computed({
    get: () => currentSemesterName.value,
    set: (name: string) => {
      const sem = semesters.value.find(s => s.name === name)
      if (sem) setCurrentSemester(sem.ID)
    },
  })

  // Legacy: stable ref for n-select options (derived from semesters)
  const semesterOptions = computed(() =>
    semesters.value.map(s => ({ label: s.name, value: s.name }))
  )

  // v0.5.5 P2: 结构化 select options —— 消费端应逐步迁移到这个 computed，
  // 而不再依赖 semesters[].name / semesters[].isActive。label 附带状态标记
  // (● 当前 / ○ 预排 / □ 归档)，value 是 semester.ID(数字)。
  const semesterSelectOptions = computed(() =>
    semesters.value.map(s => {
      let mark = ''
      if (s.status === 'active') mark = '（当前）'
      else if (s.status === 'planned') mark = '（预排）'
      else if (s.status === 'archived') mark = '（已归档）'
      return {
        label: semesterDisplayName(s) + mark,
        value: s.ID,
      }
    })
  )

  // ===== 学期切换入口 =====
  // Encapsulates all side-effects of switching semester.
  // Sprint 3+ pages will call this instead of各自维护状态.
  async function setCurrentSemester(id: number) {
    if (id === currentSemesterId.value) return
    currentSemesterId.value = id
    const name = currentSemesterName.value
    if (!name) return
    // Dynamic import to avoid circular dependency
    const { useScheduleStore } = await import('./schedule')
    const { useResourceStore } = await import('./resource')
    useScheduleStore().loadSchedule(id)
    useResourceStore().loadTeachingTasks(id)
  }

  // ===== 学期初始化 =====
  // GetActiveSemester() 仅用于确定默认学期，不作为页面级状态来源。
  async function initSemester() {
    await loadSemesters()
    try {
      const sem = await GetActiveSemester()
      if (sem && sem.ID) {
        currentSemesterId.value = sem.ID
      } else if (semesters.value.length > 0) {
        currentSemesterId.value = semesters.value[0].ID
      }
    } catch { /* no active semester */ }
  }
  initSemester()

  async function loadSemesters() {
    try {
      const result = await GetSemesters()
      // 派生兼容字段：把后端结构化学期投影成 {name, isActive} 供其他旧代码继续读。
      semesters.value = (result || []).map((s: any) => ({
        ID: s.ID,
        academicYear: s.academicYear,
        term: s.term,
        status: s.status,
        startDate: s.startDate,
        endDate: s.endDate,
        name: semesterDisplayName(s),
        isActive: s.status === 'active',
      }))
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
	    // global semester context
	    currentSemesterId,
	    currentSemester,
	    currentSemesterName,
	    setCurrentSemester,
	    // legacy compat (proxied to currentSemesterId)
	    semesterFilter,
	    semesterOptions,
	    // v0.5.5 P2: structured select options — prefer this over semesterOptions
	    semesterSelectOptions,
	    semesters,
	    // helper: derive display name from a semester row (backend DisplayName() equivalent)
	    semesterDisplayName,
	    loadSemesters,
	    // filters
	    searchQuery,
	    deptFilter,
  }
})
