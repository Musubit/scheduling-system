<script setup lang="ts">
import { useResourceStore } from '../stores/resource'
import { NButton, NInput, NInputNumber, NSwitch, NModal, NForm, NFormItem, NSelect, NDataTable, NSpace, NTag, useDialog, useMessage } from 'naive-ui'
import { DEPARTMENTS, DEPT_NAME_MAP } from '../types'
import type { TeachingTask } from '../types'
import { ref, computed, h, onMounted } from 'vue'
import * as RS from '../../bindings/scheduling-system/services/resourceservice'
import * as TS from '../../bindings/scheduling-system/services/teachingtaskservice'
import * as XLSX from 'xlsx'
import { useAppStore } from '../stores/app'

const appStore = useAppStore()
const resourceStore = useResourceStore()
const dialog = useDialog()
const message = useMessage()

onMounted(async () => { 
  resourceStore.loadAll()
  try {
    activeSemester.value = await RS.GetActiveSemester()
    if (activeSemester.value) {
      resourceStore.loadTeachingTasks(activeSemester.value.ID)
    }
  } catch { /* no active semester */ }
})

const tabLabels: Record<string, string> = { teacher: '教师', classroom: '教室', course: '课程', class: '班级', teachingTask: '教学任务' }

// ===== 教学任务状态 =====
const activeSemester = ref<any>(null)
const mergeableGroups = ref<any[]>([])

// Reactive search - switches store ref based on active tab
const searchText = computed({
  get: () => {
    switch (resourceStore.activeTab) {
      case 'teacher': return resourceStore.teacherSearch
      case 'classroom': return resourceStore.classroomSearch
      case 'course': return resourceStore.courseSearch
      case 'class': return resourceStore.classSearch
      case 'teachingTask': return resourceStore.teachingTaskSearch
      default: return ''
    }
  },
  set: (val: string) => {
    switch (resourceStore.activeTab) {
      case 'teacher': resourceStore.teacherSearch = val; break
      case 'classroom': resourceStore.classroomSearch = val; break
      case 'course': resourceStore.courseSearch = val; break
      case 'class': resourceStore.classSearch = val; break
      case 'teachingTask': resourceStore.teachingTaskSearch = val; break
    }
  }
})

// ===== 模拟数据 =====
const mockTeachers = [
  { id: 1, code: 'T001', name: '张建国', dept: '机械工程学院', status: 'active', weeklyHours: 12, preferNoEarly: true, preferNoLate: false, maxDaysPerWeek: 3, preferLowFloor: true },
  { id: 2, code: 'T002', name: '李明远', dept: '电气与电子工程学院', status: 'active', weeklyHours: 10, preferNoEarly: true, preferNoLate: false, maxDaysPerWeek: 3, preferLowFloor: true },
  { id: 3, code: 'T003', name: '王伟',   dept: '材料与化学工程学院', status: 'active', weeklyHours: 14, preferNoEarly: false, preferNoLate: false, maxDaysPerWeek: 3, preferLowFloor: false },
  { id: 4, code: 'T004', name: '刘芳',   dept: '外国语学院', status: 'active', weeklyHours: 16, preferNoEarly: true, preferNoLate: false, maxDaysPerWeek: 3, preferLowFloor: false },
  { id: 5, code: 'T005', name: '赵秀英', dept: '理学院', status: 'active', weeklyHours: 8, preferNoEarly: false, preferNoLate: false, maxDaysPerWeek: 3, preferLowFloor: false },
  { id: 6, code: 'T006', name: '孙志强', dept: '经济与管理学院', status: 'active', weeklyHours: 10, preferNoEarly: false, preferNoLate: false, maxDaysPerWeek: 3, preferLowFloor: true },
  { id: 7, code: 'T007', name: '周海',   dept: '计算机学院', status: 'inactive', weeklyHours: 0, preferNoEarly: false, preferNoLate: false, maxDaysPerWeek: 3, preferLowFloor: false },
  { id: 8, code: 'T008', name: '钱学林', dept: '生物工程与食品学院', status: 'active', weeklyHours: 6, preferNoEarly: false, preferNoLate: true, maxDaysPerWeek: 3, preferLowFloor: true },
]

const mockClassrooms = [
  { id: 1, code: 'A301', name: 'A301', building: 'A栋', floor: 3, capacity: 80,  type: '普通教室',   status: 'available' },
  { id: 2, code: 'B205', name: 'B205', building: 'B栋', floor: 2, capacity: 60,  type: '普通教室',   status: 'available' },
  { id: 3, code: 'C502', name: 'C502', building: 'C栋', floor: 5, capacity: 120, type: '多媒体教室', status: 'available' },
  { id: 4, code: 'D401', name: 'D401', building: 'D栋', floor: 4, capacity: 200, type: '阶梯教室',   status: 'maintenance' },
  { id: 5, code: 'A201', name: 'A201', building: 'A栋', floor: 2, capacity: 90,  type: '普通教室',   status: 'available' },
  { id: 6, code: 'E101', name: 'E101', building: 'E栋', floor: 1, capacity: 50,  type: '实验室',     status: 'available' },
  { id: 7, code: 'C301', name: 'C301', building: 'C栋', floor: 3, capacity: 100, type: '多媒体教室', status: 'available' },
]

const mockCourses = [
  { id: 1, code: 'CS301', name: '数据结构', dept: '计算机学院', credit: 4.0, type: '专业必修', hours: 64, status: 'active' },
  { id: 2, code: 'SC201', name: '高等数学', dept: '理学院', credit: 5.0, type: '公共必修', hours: 80, status: 'active' },
  { id: 3, code: 'EN101', name: '大学英语', dept: '外国语学院', credit: 3.0, type: '公共必修', hours: 48, status: 'active' },
  { id: 4, code: 'SC203', name: '大学物理', dept: '理学院', credit: 4.0, type: '公共必修', hours: 64, status: 'active' },
  { id: 5, code: 'CS302', name: '操作系统', dept: '计算机学院', credit: 4.0, type: '专业必修', hours: 64, status: 'active' },
  { id: 6, code: 'MX101', name: '马克思主义基本原理', dept: '马克思主义学院', credit: 2.0, type: '公共必修', hours: 32, status: 'active' },
  { id: 7, code: 'AD201', name: '设计素描', dept: '艺术设计学院', credit: 2.0, type: '专业必修', hours: 32, status: 'active' },
]

const mockClasses = [
  { id: 1, code: 'CS2301', name: '计算机2301', dept: '计算机学院', grade: 2023, students: 86, status: 'active' },
  { id: 2, code: 'CS2302', name: '计算机2302', dept: '计算机学院', grade: 2023, students: 82, status: 'active' },
  { id: 3, code: 'ME2301', name: '机械2301', dept: '机械工程学院', grade: 2023, students: 72, status: 'active' },
  { id: 4, code: 'EE2301', name: '电气2301', dept: '电气与电子工程学院', grade: 2023, students: 68, status: 'active' },
  { id: 5, code: 'CE2301', name: '土木2301', dept: '土木建筑与环境学院', grade: 2023, students: 55, status: 'active' },
  { id: 6, code: 'EM2301', name: '经管2301', dept: '经济与管理学院', grade: 2023, students: 78, status: 'active' },
  { id: 7, code: 'AD2301', name: '艺设2301', dept: '艺术设计学院', grade: 2023, students: 40, status: 'active' },
]

// ===== 列定义 =====
		const teacherColumns = [
		  { key: 'code', width: 80 },
		  { key: 'name', width: 100 },
		  { key: 'dept', width: 140 },
		  { key: 'status', width: 60, render: (row: any) => h(NSwitch, { size: 'small', value: row.status === 'active', onUpdateValue: () => toggleStatus(row) }) },
	  { key: 'actions', width: 140, render: () => h(NSpace, { size: 'small' }, { default: () => [h(NButton, { size: 'tiny', text: true }, { default: () => '编辑' }), h(NButton, { size: 'tiny', text: true, type: 'error' }, { default: () => '删除' }) ] }) },
	]

	const classroomColumns = [
	  { key: 'code', width: 80 },
	  { key: 'name', width: 100 },
	  { key: 'building', width: 70 },
	  { key: 'floor', width: 50 },
	  { key: 'capacity', width: 60 },
	  { key: 'type', width: 80 },
	  { key: 'status', width: 60, render: (row: any) => h(NSwitch, { size: 'small', value: row.status === 'available', onUpdateValue: () => toggleStatus(row) }) },
	  { key: 'actions', width: 140, render: () => h(NSpace, { size: 'small' }, { default: () => [h(NButton, { size: 'tiny', text: true }, { default: () => '编辑' }), h(NButton, { size: 'tiny', text: true, type: 'error' }, { default: () => '删除' }) ] }) },
	]

	const courseColumns = [
	  { key: 'code', width: 80 },
	  { key: 'name', width: 140 },
	  { key: 'dept', width: 120, render: (row: any) => DEPT_NAME_MAP[row.dept] || row.dept },
	  { key: 'credit', width: 50 },
	  { key: 'type', width: 90 },
	  { key: 'hours', width: 50 },
	  { key: 'status', width: 60, render: (row: any) => h(NSwitch, { size: 'small', value: row.status !== 'inactive', onUpdateValue: () => toggleStatus(row) }) },
	  { key: 'actions', width: 140, render: () => h(NSpace, { size: 'small' }, { default: () => [h(NButton, { size: 'tiny', text: true }, { default: () => '编辑' }), h(NButton, { size: 'tiny', text: true, type: 'error' }, { default: () => '删除' }) ] }) },
	]

	const classColumns = [
	  { key: 'code', width: 90 },
	  { key: 'name', width: 130 },
	  { key: 'dept', width: 140 },
	  { key: 'grade', width: 60 },
	  { key: 'students', width: 60 },
	  { key: 'status', width: 60, render: (row: any) => h(NSwitch, { size: 'small', value: row.status !== 'inactive', onUpdateValue: () => toggleStatus(row) }) },
	  { key: 'actions', width: 140, render: () => h(NSpace, { size: 'small' }, { default: () => [h(NButton, { size: 'tiny', text: true }, { default: () => '编辑' }), h(NButton, { size: 'tiny', text: true, type: 'error' }, { default: () => '删除' }) ] }) },
	]

const deptOptions = [
  { label: '全部院系', value: '全部院系' },
  ...DEPARTMENTS.map(d => ({ label: d.name, value: d.name })),
]

const deptFormOptions = DEPARTMENTS.map(d => ({ label: d.name, value: d.name }))

/** 中文模糊匹配：pattern 的每个字符必须按顺序出现在 label 中（不必连续）。 */
function fuzzyFilter(pattern: string, option: { label: string; value: any }): boolean {
  if (!pattern) return true
  const p = pattern.toLowerCase()
  const l = option.label.toLowerCase()
  // 子序列匹配：搜"职师"能匹配"职业技术师范学院"
  let pi = 0
  for (let li = 0; li < l.length && pi < p.length; li++) {
    if (l[li] === p[pi]) pi++
  }
  return pi === p.length
}

// ===== Modal state =====
const showModal = ref(false)
const editingItem = ref<any>(null)
const formData = ref<Record<string, any>>({})

function toggleStatus(row: any) {
  const tab = resourceStore.activeTab
  let newStatus: string
  if (tab === 'classroom') {
    newStatus = row.status === 'available' ? 'maintenance' : 'available'
  } else {
    newStatus = row.status === 'active' ? 'inactive' : 'active'
  }
  row.status = newStatus
  // Persist to backend
  try {
    callUpdate(tab, row.ID, { ...row, status: newStatus })
  } catch { /* fallback: already updated locally */ }
}

function openCreate() { 
  if (resourceStore.activeTab === 'teachingTask') {
    openTeachingTaskEdit()
    return
  }
  editingItem.value = null; formData.value = {}; showModal.value = true 
}
function openEdit(row: any) { 
  if (resourceStore.activeTab === 'teachingTask') {
    openTeachingTaskEdit(row)
    return
  }
  editingItem.value = row; formData.value = { ...row }; showModal.value = true 
}
function closeModal() { showModal.value = false; editingItem.value = null }

async function saveItem() {
  const tab = resourceStore.activeTab
  const data = getMockData(tab)
  try {
    if (editingItem.value) {
      // Update via Go backend, fallback to local
      await callUpdate(tab, editingItem.value.id, formData.value)
      const idx = data.findIndex((i: any) => i.id === editingItem.value.id)
      if (idx >= 0) Object.assign(data[idx], formData.value)
    } else {
      // Create via Go backend
      await callCreate(tab, formData.value)
      data.push({ ...formData.value, id: Date.now() })
    }
    resourceStore.loadAll()
  } catch (e) {
    console.warn('Go backend CRUD failed, using local:', e)
    // Fallback to local
    if (editingItem.value) {
      const idx = data.findIndex((i: any) => i.ID === editingItem.value.ID)
      if (idx >= 0) Object.assign(data[idx], formData.value)
    } else {
      data.push({ ...formData.value, ID: Date.now() })
    }
  }
  closeModal()
}

async function deleteItem(row: any) {
  dialog.warning({
    title: '确认删除',
    content: '确定要删除这条记录吗？此操作不可撤销。',
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      const data = getMockData(resourceStore.activeTab)
      try { await callDelete(resourceStore.activeTab, row.ID) } catch {}
      const idx = data.findIndex((i: any) => i.ID === row.ID)
      if (idx >= 0) data.splice(idx, 1)
      resourceStore.loadAll()
      message.success('已删除')
    },
  })
}

async function callCreate(tab: string, item: any) {
  const m = toModel(tab, item)
  switch (tab) {
    case 'teacher': await RS.CreateTeacher(m); break
    case 'classroom': await RS.CreateClassroom(m); break
    case 'course': await RS.CreateCourse(m); break
    case 'class': await RS.CreateClassGroup(m); break
    case 'teachingTask': {
      // Extract classGroupIDs from the form data
      const classIds = (formData.value._classIds || []) as number[]
      await TS.CreateTeachingTask({ courseId: m.courseId, teacherId: m.teacherId, semesterId: m.semesterId, status: 'active' }, classIds)
      break
    }
  }
}
async function callUpdate(tab: string, id: number, item: any) {
  const m = toModel(tab, item)
  switch (tab) {
    case 'teacher': await RS.UpdateTeacher(m); break
    case 'classroom': await RS.UpdateClassroom(m); break
    case 'course': await RS.UpdateCourse(m); break
    case 'class': await RS.UpdateClassGroup(m); break
    case 'teachingTask': {
      const classIds = (formData.value._classIds || []) as number[]
      await TS.UpdateTeachingTask(id, { courseId: m.courseId, teacherId: m.teacherId, semesterId: m.semesterId, status: 'active' }, classIds)
      break
    }
  }
}
async function callDelete(tab: string, id: number) {
  switch (tab) {
    case 'teacher': await RS.DeleteTeacher(id); break
    case 'classroom': await RS.DeleteClassroom(id); break
    case 'course': await RS.DeleteCourse(id); break
    case 'class': await RS.DeleteClassGroup(id); break
    case 'teachingTask': await TS.DeleteTeachingTask(id); break
  }
}
function toModel(tab: string, item: any): any {
  if (tab === 'teachingTask') {
    return { ...item, ID: item.ID || 0, status: item.status || 'active' }
  }
  return { ...item, ID: item.ID || 0 }
}

function getMockData(tab: string): any[] {
  if (tab === 'teachingTask') return resourceStore.teachingTasks as any[]
  const map: Record<string, any[]> = { teacher: mockTeachers, classroom: mockClassrooms, course: mockCourses, class: mockClasses }
  return map[tab] || []
}

const formFields = computed(() => {
  const fields: Record<string, { key: string; label: string; type?: string; options?: any[]; min?: number; max?: number; filterable?: boolean; placeholder?: string }[]> = {
    teacher: [
      { key: 'code', label: '工号' },
      { key: 'name', label: '姓名' },
      { key: 'dept', label: '院系', type: 'select', options: deptFormOptions, filterable: true },
      { key: 'preferNoEarly', label: '避免早课', type: 'switch' },
      { key: 'preferNoLate', label: '避免晚课', type: 'switch' },
      { key: 'maxDaysPerWeek', label: '每周最多到校天数', type: 'number', min: 1, max: 7 },
      { key: 'preferLowFloor', label: '优先低楼层', type: 'switch' },
      { key: 'unavailableSlots', label: '不可用时段(JSON)', type: 'textarea', placeholder: '[{"dayOfWeek":2,"startPeriod":0,"span":4}]' },
    ],
	    classroom: [
	      { key: 'code', label: '编号' },
	      { key: 'name', label: '教室名' },
	      { key: 'building', label: '教学楼' },
	      { key: 'floor', label: '楼层', type: 'number', min: 1 },
	      { key: 'capacity', label: '容量', type: 'number', min: 1 },
	      { key: 'type', label: '类型' },
	    ],
	    course: [
	      { key: 'code', label: '编号' },
	      { key: 'name', label: '课程名' },
	      { key: 'dept', label: '院系', type: 'select', options: deptFormOptions, filterable: true },
	      { key: 'credit', label: '学分', type: 'number', min: 0 },
	      { key: 'type', label: '类型' },
	      { key: 'hours', label: '课时', type: 'number', min: 1 },
	    ],
		    class: [
		      { key: 'code', label: '编号' },
		      { key: 'name', label: '班级名' },
		      { key: 'dept', label: '院系', type: 'select', options: deptFormOptions, filterable: true },
		      { key: 'grade', label: '年级', type: 'number', min: 2000 },
		      { key: 'students', label: '人数', type: 'number', min: 1 },
		    ],
		    teachingTask: [
		      { key: 'courseId', label: '课程', type: 'select', options: 'courses' as any, filterable: true },
		      { key: 'teacherId', label: '教师', type: 'select', options: 'teachers' as any, filterable: true },
		      { key: '_classIds', label: '班级', type: 'multiSelect', options: 'classGroups' as any, filterable: true },
		    ],
		  }
			  return fields[resourceStore.activeTab] || []
			})

// actionRender - shared by all tabs
const actionRender = (row: any) => h(NSpace, { size: 'small' }, { default: () => [
  h(NButton, { size: 'tiny', text: true, onClick: () => openEdit(row) }, { default: () => '编辑' }),
  h(NButton, { size: 'tiny', text: true, type: 'error', onClick: () => deleteItem(row) }, { default: () => '删除' }),
]})

// ===== 教学任务专用列 =====
const teachingTaskCols = [
  { key: 'courseName', width: 140, render: (row: any) => row.course?.name || '-' },
  { key: 'teacherName', width: 100, render: (row: any) => row.teacher?.name || '-' },
  { key: 'classes', width: 200, render: (row: any) => {
    const names = (row.classes || []).map((c: any) => c.classGroup?.name || c.classGroup?.code || '').filter(Boolean)
    return h('div', { style: 'display:flex;flex-wrap:wrap;gap:4px' }, names.map((n: string) => h(NTag, { size: 'small', bordered: false }, { default: () => n })))
  }},
  { key: 'status', width: 60, render: (row: any) => h(NSwitch, { size: 'small', value: row.status !== 'inactive', onUpdateValue: () => toggleStatus(row) }) },
  { key: 'actions', width: 140, render: actionRender },
]

// ===== 智能检测合班 =====
async function handleDetectMerge() {
  if (!activeSemester.value) {
    message.warning('请先在设置中激活一个学期')
    return
  }
  try {
    mergeableGroups.value = await TS.DetectMergeableTasks(activeSemester.value.ID) || []
    if (mergeableGroups.value.length === 0) {
      message.info('未发现可合班的教学任务')
    } else {
      message.success(`发现 ${mergeableGroups.value.length} 组可合班方案`)
    }
  } catch (e) {
    message.error('检测失败：' + (e as any).message)
  }
}

async function handleConfirmMerge(group: any) {
  // Group contains tasks array and classGroups array
  // We merge by keeping the first task and deleting the rest, updating classes
  if (!group.tasks || group.tasks.length < 2) return
  try {
    const firstTask = group.tasks[0]
    const allClassIds = group.classGroups.map((c: any) => c.ID)
    await TS.UpdateTeachingTask(firstTask.ID, {
      courseId: firstTask.courseId,
      teacherId: firstTask.teacherId,
      semesterId: firstTask.semesterId,
      status: 'active',
    }, allClassIds)
    // Delete remaining tasks
    for (let i = 1; i < group.tasks.length; i++) {
      await TS.DeleteTeachingTask(group.tasks[i].ID)
    }
    message.success('合班完成')
    mergeableGroups.value = []
    if (activeSemester.value) resourceStore.loadTeachingTasks(activeSemester.value.ID)
  } catch (e) {
    message.error('合班失败：' + (e as any).message)
  }
}

// 打开教学任务编辑时，加载课程/教师/班级选项
function openTeachingTaskEdit(row?: any) {
  const allCourses = resourceStore.courses.map(c => ({ label: `${c.code} ${c.name}`, value: c.ID }))
  const allTeachers = resourceStore.teachers.map(t => ({ label: `${t.code} ${t.name}`, value: t.ID }))
  const allClasses = resourceStore.classGroups.map(c => ({ label: `${c.code} ${c.name}`, value: c.ID }))
  // Store these options for the form to use
  ;(window as any).__ttCourseOptions = allCourses
  ;(window as any).__ttTeacherOptions = allTeachers
  ;(window as any).__ttClassOptions = allClasses
  
  if (row) {
    editingItem.value = row
    formData.value = { 
      courseId: row.courseId, 
      teacherId: row.teacherId, 
      semesterId: row.semesterId,
      _classIds: (row.classes || []).map((c: any) => c.classGroupId || c.ClassGroupID),
    }
  } else {
    editingItem.value = null
    formData.value = { semesterId: activeSemester.value?.ID || 0, _classIds: [] }
  }
  showModal.value = true
}

function resolveOptions(field: any): any[] {
  if (field.options === 'courses') return (window as any).__ttCourseOptions || []
  if (field.options === 'teachers') return (window as any).__ttTeacherOptions || []
  if (field.options === 'classGroups') return (window as any).__ttClassOptions || []
  return field.options || []
}

const teacherCols = [...teacherColumns.slice(0, -1), { key: 'actions', width: 140, render: actionRender }]
const classroomCols = [...classroomColumns.slice(0, -1), { key: 'actions', width: 140, render: actionRender }]
const courseCols = [...courseColumns.slice(0, -1), { key: 'actions', width: 140, render: actionRender }]
const classCols = [...classColumns.slice(0, -1), { key: 'actions', width: 140, render: actionRender }]

// ===== Excel Import / Export =====
const importFileRef = ref<HTMLInputElement>()

function triggerImport() { importFileRef.value?.click() }

async function handleFileChange(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (!file) return
  const reader = new FileReader()
  reader.onload = async (ev) => {
    try {
      const data = new Uint8Array(ev.target?.result as ArrayBuffer)
      const wb = XLSX.read(data, { type: 'array' })
      const ws = wb.Sheets[wb.SheetNames[0]]
      const rows = XLSX.utils.sheet_to_json<any>(ws, { header: 1 })
      if (rows.length < 2) { message.warning('文件为空或格式不正确'); return }
      const headers = rows[0] as string[]
      const dataRows = rows.slice(1).filter((r: any) => r.length > 0 && String(r[0]).trim())
      let count = 0
      let errors: string[] = []
      for (let i = 0; i < dataRows.length; i++) {
        const row = dataRows[i]
        const item: any = {}
        headers.forEach((h, j) => { item[h.trim()] = row[j] ?? '' })
        try {
          if (resourceStore.activeTab === 'teachingTask') {
            // 教学任务导入：利用后端 ImportTeachingTasks 实现编码→ID解析
            const courseCode = String(item.courseId || item.courseCode || '').trim()
            const teacherCode = String(item.teacherId || item.teacherCode || '').trim()
            const classCodes = String(item.classGroupIds || '').split(',').map((s: string) => s.trim()).filter(Boolean)
            if (!courseCode || !teacherCode) { errors.push(`第${i+2}行: 课程编号或教师编号为空`); continue }
            try {
              const imported: [number, string[]] = await TS.ImportTeachingTasks(activeSemester.value?.ID || 0, [[courseCode, teacherCode, classCodes.join(',')]])
              if (imported[0] > 0) count += imported[0]
              if (imported[1] && imported[1].length > 0) errors.push(...imported[1])
            } catch { errors.push(`第${i+2}行: 后端导入接口调用失败`) }
          } else {
            await callCreate(resourceStore.activeTab, item)
            count++
          }
        } catch (e) {
          errors.push(`第${i+2}行: ${(e as any)?.message || '未知错误'}`)
        }
      }
      if (count > 0) message.success(`成功导入 ${count} 条记录`)
      if (errors.length > 0) message.warning(`导入完成，${errors.length} 行失败：\n${errors.slice(0,5).join('\n')}${errors.length > 5 ? `\n...还有 ${errors.length - 5} 行错误` : ''}`)
      resourceStore.loadAll()
    } catch (err) {
      message.error('导入失败：' + (err as any).message)
    }
  }
  reader.readAsArrayBuffer(file)
  // Reset input
  if (e.target) (e.target as HTMLInputElement).value = ''
}

function downloadTemplate() {
  const tab = resourceStore.activeTab
  let headers: string[] = []
  let example: any[] = []
  switch (tab) {
    case 'teacher':
      headers = ['code', 'name', 'dept']
      example = ['T099', '张三', '理学院']
      break
    case 'classroom':
      headers = ['code', 'name', 'building', 'capacity', 'type']
      example = ['A999', 'A999', 'A栋', '100', '普通教室']
      break
    case 'course':
      headers = ['code', 'name', 'dept', 'credit', 'type', 'hours']
      example = ['CS999', '新课程', 'cs', '3.0', '专业选修', '48']
      break
    case 'class':
      headers = ['code', 'name', 'dept', 'grade', 'students']
      example = ['XX2301', '班级名', '计算机学院', '2023', '60']
      break
    case 'teachingTask':
      headers = ['courseId', 'teacherId', 'classGroupIds']
      example = ['1', '1', '1,2']
      break
  }
  const ws = XLSX.utils.aoa_to_sheet([headers, example])
  const wb = XLSX.utils.book_new()
  XLSX.utils.book_append_sheet(wb, ws, 'Sheet1')
  XLSX.writeFile(wb, `${tab}-template.xlsx`)
}
</script>

<template>
  <div class="resource-page">
    <div class="resource-toolbar">
      <n-input
        v-model:value="searchText"
        :placeholder="`搜索${tabLabels[resourceStore.activeTab] || ''}...`"
        clearable
        size="small"
        style="width: 240px"
      />
      <n-select
        v-model:value="appStore.deptFilter"
        :options="deptOptions"
        size="small"
        style="width: 160px"
      />
      <div class="spacer"></div>
      <n-button size="small" type="primary" @click="openCreate()">+ 新增</n-button>
      <n-button size="small" @click="triggerImport()">导入Excel</n-button>
      <n-button size="small" @click="downloadTemplate()">下载模板</n-button>
      <n-button v-if="resourceStore.activeTab === 'teachingTask'" size="small" type="warning" @click="handleDetectMerge()">智能检测合班</n-button>
      <input ref="importFileRef" type="file" accept=".xlsx,.xls" style="display:none" @change="handleFileChange" />
    </div>

	    <div class="resource-table">
	      <n-data-table v-if="resourceStore.activeTab === 'teacher'" :columns="teacherCols" :data="resourceStore.filteredTeachers" :single-line="false" size="small" />
	      <n-data-table v-else-if="resourceStore.activeTab === 'classroom'" :columns="classroomCols" :data="resourceStore.filteredClassrooms" :single-line="false" size="small" />
	      <n-data-table v-else-if="resourceStore.activeTab === 'course'" :columns="courseCols" :data="resourceStore.filteredCourses" :single-line="false" size="small" />
	      <n-data-table v-else-if="resourceStore.activeTab === 'class'" :columns="classCols" :data="resourceStore.filteredClasses" :single-line="false" size="small" />
	      <div v-else-if="resourceStore.activeTab === 'teachingTask'" class="teaching-task-area">
	        <!-- 智能检测面板 -->
	        <div v-if="mergeableGroups.length > 0" class="merge-panel">
	          <div class="merge-panel-title">💡 检测到可合班教学任务</div>
	          <div v-for="(g, gi) in mergeableGroups" :key="gi" class="merge-item">
	            <span class="merge-info">{{ g.courseName }} — {{ g.teacherName }}（{{ g.classGroups.length }}个班）</span>
	            <n-button size="tiny" type="primary" @click="handleConfirmMerge(g)">一键合班</n-button>
	          </div>
	        </div>
	        <n-data-table :columns="teachingTaskCols" :data="resourceStore.filteredTeachingTasks" :single-line="false" size="small" />
	      </div>
	    </div>

    <!-- Form Modal -->
    <n-modal v-model:show="showModal" preset="card" :title="(editingItem ? '编辑' : '新增') + (tabLabels[resourceStore.activeTab] || '')" style="width: 520px;" :mask-closable="false">
      <n-form label-placement="left" label-width="110" :style="{ padding: '8px 0' }">
        <n-form-item v-for="f in formFields" :key="f.key" :label="f.label">
	          <n-switch v-if="f.type === 'switch'" v-model:value="formData[f.key]" />
	          <n-input-number v-else-if="f.type === 'number'" v-model:value="formData[f.key]" :min="f.min" :max="f.max" :placeholder="'请输入' + f.label" clearable style="width:100%" />
	          <n-input v-else-if="f.type === 'textarea'" v-model:value="formData[f.key]" type="textarea" :rows="3" :placeholder="f.placeholder || ('请输入' + f.label)" clearable style="width:100%" />
	          <n-select v-else-if="f.type === 'select'" v-model:value="formData[f.key]" :options="resolveOptions(f)" :filterable="f.filterable" :filter="f.filterable ? fuzzyFilter : undefined" :clearable="true" :placeholder="'请选择' + f.label" />
	          <n-select v-else-if="f.type === 'multiSelect'" v-model:value="formData[f.key]" :options="resolveOptions(f)" :filterable="f.filterable" :filter="f.filterable ? fuzzyFilter : undefined" :multiple="true" :clearable="true" :placeholder="'请选择' + f.label" />
	          <n-input v-else v-model:value="formData[f.key]" :placeholder="'请输入' + f.label" clearable />
        </n-form-item>
      </n-form>
      <template #footer>
        <n-space justify="end">
          <n-button @click="closeModal()">取消</n-button>
          <n-button type="primary" @click="saveItem()">保存</n-button>
        </n-space>
      </template>
    </n-modal>
  </div>
</template>

<style scoped>
.resource-page {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.resource-toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 16px;
}

.spacer {
  flex: 1;
}

.resource-table {
  flex: 1;
  overflow: auto;
}

.placeholder {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 200px;
  color: var(--b3-theme-on-surface);
  font-size: 14px;
}

.teaching-task-area {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.merge-panel {
  background: var(--b3-warning-lightest, #fff8e1);
  border: 1px solid var(--b3-warning, #ff9800);
  border-radius: 8px;
  padding: 12px 16px;
}

.merge-panel-title {
  font-weight: 600;
  font-size: 14px;
  margin-bottom: 8px;
}

.merge-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 0;
  border-top: 1px solid var(--b3-border-color, #e0e0e0);
}

.merge-item:first-child {
  border-top: none;
}

.merge-info {
  font-size: 13px;
}
</style>
