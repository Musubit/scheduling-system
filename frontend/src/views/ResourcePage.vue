<script setup lang="ts">
import { useResourceStore } from '../stores/resource'
import { NButton, NInput, NInputNumber, NSwitch, NModal, NForm, NFormItem, NSelect, NDataTable, NSpace, NTag, useDialog, useMessage } from 'naive-ui'
import { DEPARTMENTS, DAY_NAMES, PERIODS } from '../types'
import { ref, computed, h, onMounted } from 'vue'
import * as RS from '../../bindings/scheduling-system/backend/services/resourceservice'
import * as TS from '../../bindings/scheduling-system/backend/services/teachingtaskservice'
import * as XLSX from 'xlsx'
import { useAppStore } from '../stores/app'
import { fuzzyFilter } from '../utils/fuzzyFilter'

const fuzzyFilterFn = fuzzyFilter as any

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
	  { key: 'dept', width: 120 },
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
  try {
    if (editingItem.value) {
      // Update via Go backend
      await callUpdate(tab, editingItem.value.ID, formData.value)
    } else {
      // Create via Go backend
      await callCreate(tab, formData.value)
    }
    resourceStore.loadAll()
    if (tab === 'teachingTask' && activeSemester.value) {
      resourceStore.loadTeachingTasks(activeSemester.value.ID)
    }
  } catch (e) {
    console.warn('Go backend CRUD failed:', e)
    message.error('保存失败：' + ((e as any)?.message || String(e)))
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
      try { await callDelete(resourceStore.activeTab, row.ID) } catch {}
      resourceStore.loadAll()
      if (resourceStore.activeTab === 'teachingTask' && activeSemester.value) {
        resourceStore.loadTeachingTasks(activeSemester.value.ID)
      }
      message.success('已删除')
    },
  })
}

function getRealData(tab: string): any[] {
  switch (tab) {
    case 'teacher': return resourceStore.teachers as any[]
    case 'classroom': return resourceStore.classrooms as any[]
    case 'course': return resourceStore.courses as any[]
    case 'class': return resourceStore.classGroups as any[]
    case 'teachingTask': return resourceStore.teachingTasks as any[]
    default: return []
  }
}
const clearing = ref(false)
async function clearAll() {
  const tab = resourceStore.activeTab
  const data = getRealData(tab)
  if (data.length === 0) {
    message.info('当前没有数据可清空')
    return
  }
  dialog.warning({
    title: '确认一键清空',
    content: `即将删除「${tabLabels[tab]}」的全部 ${data.length} 条记录。此操作不可撤销，确定继续吗？`,
    positiveText: '确认清空',
    negativeText: '取消',
    onPositiveClick: async () => {
      clearing.value = true
      let deleted = 0
      for (const item of data) {
        try {
          await callDelete(tab, item.ID)
          deleted++
        } catch { /* skip failed */ }
      }
      resourceStore.loadAll()
      clearing.value = false
      message.success(`已清空 ${deleted} 条记录`)
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
      await TS.CreateTeachingTask({
        courseId: m.courseId, teacherId: m.teacherId, semesterId: m.semesterId, status: 'active',
        totalHours: formData.value.totalHours || 0, startWeek: formData.value.startWeek || 1,
        endWeek: formData.value.endWeek || 16, maxHoursPerWeek: formData.value.maxHoursPerWeek || 0,
      } as any, classIds)
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
      await TS.UpdateTeachingTask(id, {
        courseId: m.courseId, teacherId: m.teacherId, semesterId: m.semesterId, status: 'active',
        totalHours: formData.value.totalHours || 0, startWeek: formData.value.startWeek || 1,
        endWeek: formData.value.endWeek || 16, maxHoursPerWeek: formData.value.maxHoursPerWeek || 0,
      } as any, classIds)
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

const formFields = computed(() => {
  const fields: Record<string, { key: string; label: string; type?: string; options?: any[]; min?: number; max?: number; filterable?: boolean; placeholder?: string }[]> = {
    teacher: [
      { key: 'code', label: '工号（选填）' },
      { key: 'name', label: '姓名' },
      { key: 'dept', label: '院系', type: 'select', options: deptFormOptions, filterable: true },
      { key: 'preferNoEarly', label: '尽量避免1-2节', type: 'switch' },
      { key: 'preferNoLate', label: '尽量避免9-11节', type: 'switch' },
      { key: 'maxDaysPerWeek', label: '每周最多到校天数', type: 'number', min: 1, max: 7 },
      { key: 'preferLowFloor', label: '优先安排低楼层教室', type: 'switch' },
      { key: 'unavailableSlots', label: '不可用时间', type: 'timegrid' },
    ],
	    classroom: [
	      { key: 'code', label: '编号（选填）' },
	      { key: 'name', label: '教室名' },
	      { key: 'building', label: '教学楼' },
	      { key: 'floor', label: '楼层', type: 'number', min: 1 },
	      { key: 'capacity', label: '容量', type: 'number', min: 1 },
	      { key: 'type', label: '类型（选填）' },
	    ],
	    course: [
	      { key: 'code', label: '课程代码（选填）' },
	      { key: 'name', label: '课程名' },
	      { key: 'dept', label: '院系', type: 'select', options: deptFormOptions, filterable: true },
	      { key: 'hours', label: '学时（必填）', type: 'number', min: 1 },
	      { key: 'credit', label: '学分（选填）', type: 'number', min: 0 },
	      { key: 'type', label: '类型（选填）' },
	    ],
		    class: [
		      { key: 'code', label: '班级编号（选填）' },
		      { key: 'name', label: '班级名' },
		      { key: 'dept', label: '院系', type: 'select', options: deptFormOptions, filterable: true },
		      { key: 'students', label: '人数', type: 'number', min: 1 },
		      { key: 'grade', label: '年级（选填）', type: 'number', min: 2000 },
		    ],
		    teachingTask: [
		      { key: 'courseId', label: '课程', type: 'select', options: 'courses' as any, filterable: true },
		      { key: 'teacherId', label: '教师', type: 'select', options: 'teachers' as any, filterable: true },
		      { key: '_classIds', label: '班级', type: 'multiSelect', options: 'classGroups' as any, filterable: true },
		      { key: 'totalHours', label: '总学时', type: 'number', min: 1 },
		      { key: 'startWeek', label: '起始周', type: 'number', min: 1, max: 20 },
		      { key: 'endWeek', label: '结束周', type: 'number', min: 1, max: 20 },
		      { key: 'maxHoursPerWeek', label: '周最大学时（选填）', type: 'number', min: 0 },
		    ],
		  }
  return fields[resourceStore.activeTab] || []
})

// ===== 不可用时间网格 =====
// 时段块定义：startPeriod → span（晚间为3节，其余2节）
const PERIOD_BLOCK_SPANS: Record<number, number> = { 0: 2, 2: 2, 4: 2, 6: 2, 8: 3 }

// 从 PERIODS 动态生成行数据（标签、起始节、跨度、时间范围）
const timeGridRows = Object.entries(PERIOD_BLOCK_SPANS).map(([startStr, span]) => {
  const start = Number(startStr)
  const first = PERIODS[start]
  const last = PERIODS[start + span - 1]
  const startTime = first.time.split('\n')[0]
  const endTime = last.time.split('\n')[1] || last.time.split('\n')[0]
  return {
    label: `${first.num}-${last.num}节`,
    start,
    span,
    time: `${startTime}-${endTime}`,
  }
})

interface UnavailableSlot { dayOfWeek: number; startPeriod: number; span: number }

function getSlots(): UnavailableSlot[] {
  try {
    const json = formData.value.unavailableSlots || ''
    if (!json) return []
    const arr = JSON.parse(json)
    return Array.isArray(arr) ? arr : []
  } catch { return [] }
}

function isSlotUnavailable(day: number, start: number): boolean {
  if (resourceStore.activeTab !== 'teacher') return false
  return getSlots().some(s => s.dayOfWeek === day && s.startPeriod === start)
}

function toggleSlot(day: number, start: number, span: number) {
  const slots = getSlots()
  const idx = slots.findIndex(s => s.dayOfWeek === day && s.startPeriod === start)
  if (idx >= 0) { slots.splice(idx, 1) }
  else { slots.push({ dayOfWeek: day, startPeriod: start, span }) }
  formData.value.unavailableSlots = slots.length > 0 ? JSON.stringify(slots) : ''
}

// actionRender - shared by all tabs
const actionRender = (row: any) => {
  const buttons = [
    h(NButton, { size: 'tiny', text: true, onClick: () => openEdit(row) }, { default: () => '编辑' }),
  ]
  // 拆班：仅对含多个班级的教学任务显示
  if (resourceStore.activeTab === 'teachingTask' && (row.classes?.length ?? 0) > 1) {
    buttons.push(h(NButton, { size: 'tiny', text: true, type: 'warning', onClick: () => splitMerged(row) }, { default: () => '拆班' }))
  }
  buttons.push(h(NButton, { size: 'tiny', text: true, type: 'error', onClick: () => deleteItem(row) }, { default: () => '删除' }))
  return h(NSpace, { size: 'small' }, { default: () => buttons })
}

// ===== 教学任务专用列 =====
const teachingTaskCols = [
  { key: 'courseName', width: 120, render: (row: any) => row.course?.name || '-' },
  { key: 'teacherName', width: 90, render: (row: any) => row.teacher?.name || '-' },
  { key: 'classes', width: 160, render: (row: any) => {
    const names = (row.classes || []).map((c: any) => c.classGroup?.name || c.classGroup?.code || '').filter(Boolean)
    return h('div', { style: 'display:flex;flex-wrap:wrap;gap:4px' }, names.map((n: string) => h(NTag, { size: 'small', bordered: false }, { default: () => n })))
  }},
  { key: 'totalHours', width: 60, title: '学时' },
  { key: 'weeks', width: 80, render: (row: any) => `${row.startWeek || 1}-${row.endWeek || 16}周` },
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
      totalHours: firstTask.totalHours || 0, startWeek: firstTask.startWeek || 1,
      endWeek: firstTask.endWeek || 16, maxHoursPerWeek: firstTask.maxHoursPerWeek || 0,
    } as any, allClassIds)
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

// 拆班：将合班教学任务拆分为多个单班任务
async function splitMerged(row: any) {
  if (!row || !row.ID) return
  try {
    const created = await TS.SplitMergedTeachingTask(row.ID)
    message.success(`拆班成功，已拆分为 ${created} 个单班教学任务`)
    if (activeSemester.value) resourceStore.loadTeachingTasks(activeSemester.value.ID)
  } catch (e) {
    message.error('拆班失败：' + (e as any).message)
  }
}

// 教学任务表单选项（组件级 ref，替代 window 全局变量）
const ttCourseOptions = ref<{ label: string; value: number }[]>([])
const ttTeacherOptions = ref<{ label: string; value: number }[]>([])
const ttClassOptions = ref<{ label: string; value: number }[]>([])

// 打开教学任务编辑时，加载课程/教师/班级选项
function openTeachingTaskEdit(row?: any) {
  ttCourseOptions.value = resourceStore.courses.map(c => ({ label: `${c.code} ${c.name}`, value: c.ID }))
  ttTeacherOptions.value = resourceStore.teachers.map(t => ({ label: `${t.code} ${t.name}`, value: t.ID }))
  ttClassOptions.value = resourceStore.classGroups.map(c => ({ label: `${c.code} ${c.name}`, value: c.ID }))
  
  if (row) {
    editingItem.value = row
    formData.value = {
      courseId: row.courseId,
      teacherId: row.teacherId,
      semesterId: row.semesterId,
      _classIds: (row.classes || []).map((c: any) => c.classGroupId || c.ClassGroupID),
      totalHours: row.totalHours || row.course?.hours || 0,
      startWeek: row.startWeek || 1,
      endWeek: row.endWeek || 16,
      maxHoursPerWeek: row.maxHoursPerWeek || 0,
    }
  } else {
    editingItem.value = null
    formData.value = { semesterId: activeSemester.value?.ID || 0, _classIds: [], startWeek: 1, endWeek: 16, totalHours: 0, maxHoursPerWeek: 0 }
  }
  showModal.value = true
}

function resolveOptions(field: any): any[] {
  if (field.options === 'courses') return ttCourseOptions.value
  if (field.options === 'teachers') return ttTeacherOptions.value
  if (field.options === 'classGroups') return ttClassOptions.value
  return field.options || []
}

const teacherCols = [...teacherColumns.slice(0, -1), { key: 'actions', width: 140, render: actionRender }]
const classroomCols = [...classroomColumns.slice(0, -1), { key: 'actions', width: 140, render: actionRender }]
const courseCols = [...courseColumns.slice(0, -1), { key: 'actions', width: 140, render: actionRender }]
const classCols = [...classColumns.slice(0, -1), { key: 'actions', width: 140, render: actionRender }]

// ===== Excel Import / Export =====
const importFileRef = ref<HTMLInputElement>()

function triggerImport() { importFileRef.value?.click() }

// ===== 导入导出模板定义 =====
interface TemplateColumn {
  header: string     // 中文列名（不含选项提示）
  field: string      // 内部字段名
  required: boolean  // 是否必填
  hint?: string      // 可选值提示，如 "是/否"、"普通教室/多媒体教室/实验室"
}

// 每个标签的列定义 + 示例数据
const TEMPLATE_COLUMNS: Record<string, { columns: TemplateColumn[]; examples: string[] }> = {
  teacher: {
    columns: [
      { header: '工号', field: 'code', required: false, hint: '留空自动生成' },
      { header: '姓名', field: 'name', required: true },
      { header: '院系', field: 'dept', required: true },
      { header: '避免早课', field: 'preferNoEarly', required: false, hint: '是/否' },
      { header: '避免晚课', field: 'preferNoLate', required: false, hint: '是/否' },
      { header: '最大到校天数', field: 'maxDaysPerWeek', required: false, hint: '1-7' },
      { header: '优先低楼层', field: 'preferLowFloor', required: false, hint: '是/否' },
    ],
    examples: ['', '张三', '理学院', '是', '否', '3', '是'],
  },
  classroom: {
    columns: [
      { header: '教室编号', field: 'code', required: false, hint: '留空自动生成' },
      { header: '名称', field: 'name', required: true },
      { header: '楼栋', field: 'building', required: true },
      { header: '楼层', field: 'floor', required: false, hint: '数字' },
      { header: '容量', field: 'capacity', required: false, hint: '人数' },
      { header: '类型', field: 'type', required: false, hint: '普通教室/多媒体教室/实验室/机房' },
    ],
    examples: ['', 'A999教室', 'A栋', '1', '100', '普通教室'],
  },
  course: {
    columns: [
      { header: '课程代码', field: 'code', required: false, hint: '留空自动生成' },
      { header: '名称', field: 'name', required: true },
      { header: '院系', field: 'dept', required: true, hint: '如：计算机学院' },
      { header: '学时', field: 'hours', required: true, hint: '如：64（影响每周排课次数）' },
      { header: '学分', field: 'credit', required: false, hint: '数字' },
      { header: '类型', field: 'type', required: false, hint: '专业必修/专业选修/通识必修/通识选修' },
    ],
    examples: ['', '新课程', '计算机学院', '64', '3.0', '专业选修'],
  },
  class: {
    columns: [
      { header: '班级代码', field: 'code', required: false, hint: '留空自动生成' },
      { header: '名称', field: 'name', required: true },
      { header: '院系', field: 'dept', required: true },
      { header: '人数', field: 'students', required: false, hint: '人数' },
      { header: '年级', field: 'grade', required: false, hint: '如：2023' },
    ],
    examples: ['', '班级名', '计算机学院', '60', '2023'],
  },
  teachingTask: {
    columns: [
      { header: '课程代码', field: 'courseCode', required: true, hint: '如：CS301' },
      { header: '工号', field: 'teacherCode', required: true, hint: '如：T007' },
      { header: '班级代码', field: 'classGroupIds', required: false, hint: '逗号分隔，如：CS2301,CS2302' },
      { header: '总学时', field: 'totalHours', required: false, hint: '如：64，留空取课程默认' },
      { header: '起始周', field: 'startWeek', required: false, hint: '如：1' },
      { header: '结束周', field: 'endWeek', required: false, hint: '如：16' },
      { header: '周最大学时', field: 'maxHoursPerWeek', required: false, hint: '如：8，留空不限' },
    ],
    examples: ['CS301', 'T007', 'CS2301,CS2302', '64', '1', '16', ''],
  },
}

/** 从 TEMPLATE_COLUMNS 构建输出用的行和映射表 */
function buildSchema(tab: string) {
  const def = TEMPLATE_COLUMNS[tab]
  // 表头行：必填加 *，有选项提示的追加到列名
  const headerRow = def.columns.map(c => {
    let text = c.required ? `*${c.header}` : c.header
    if (c.hint) text += `（${c.hint}）`
    return text
  })
  // 标识行：必填/选填
  const indicatorRow = def.columns.map(c => c.required ? '必填' : '选填')
  // 示例行：首列加【示例】标记
  const exampleRow = [...def.examples]
  if (exampleRow.length > 0) exampleRow[0] = `【示例】${exampleRow[0]}`
  // 字段映射：纯列名 → 字段名
  const fieldMap: Record<string, string> = {}
  def.columns.forEach(c => { fieldMap[c.header] = c.field })
  return { headerRow, indicatorRow, exampleRow, fieldMap }
}

/** 把中文表头行映射为内部字段名 */
function mapRow(fieldMap: Record<string, string>, headers: string[], row: any[]): Record<string, any> {
  const item: Record<string, any> = {}
  headers.forEach((h, j) => {
    // 去掉 * 前缀和（选项提示）后缀，得到纯列名
    const key = h.trim().replace(/^\*/, '').replace(/[（(][^)）]*[)）]/, '').trim()
    const field = fieldMap[key] || key
    let val: any = row[j] ?? ''
    if (typeof val === 'string') {
      val = val.trim()
      if (val === '是' || val === 'TRUE' || val === 'true') val = true
      else if (val === '否' || val === 'FALSE' || val === 'false') val = false
    }
    if (val !== '') item[field] = val
  })
  return item
}

/** 检测某行是否为「必填/选填」标识行或示例行 */
function isMetaRow(row: any[]): boolean {
  if (row.length === 0) return true
  const first = String(row[0] || '').trim()
  if (first.includes('必填') || first.includes('选填')) return true
  if (first.startsWith('【示例】') || first.startsWith('示例')) return true
  return false
}

async function handleFileChange(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (!file) return
  const reader = new FileReader()
  reader.onload = async (ev) => {
    try {
      const tab = resourceStore.activeTab
      const schema = buildSchema(tab)
      const data = new Uint8Array(ev.target?.result as ArrayBuffer)
      const wb = XLSX.read(data, { type: 'array' })
      const ws = wb.Sheets[wb.SheetNames[0]]
      const rows = XLSX.utils.sheet_to_json<any>(ws, { header: 1 })
      if (rows.length < 2) { message.warning('文件为空或格式不正确'); return }
      const headers = rows[0] as string[]
      // 过滤掉标识行和示例行
      const dataRows = rows.slice(1).filter((r: any) => !isMetaRow(r) && r.length > 0 && String(r[0]).trim())
      let count = 0
      const errors: string[] = []
      for (let i = 0; i < dataRows.length; i++) {
        const item = mapRow(schema.fieldMap, headers, dataRows[i])
        try {
          if (tab === 'teachingTask') {
            const courseCode = String(item.courseCode || '').trim()
            const teacherCode = String(item.teacherCode || '').trim()
            const classCodes = String(item.classGroupIds || '').split(',').map((s: string) => s.trim()).filter(Boolean)
            const totalHours = String(item.totalHours || '').trim()
            const startWeek = String(item.startWeek || '').trim()
            const endWeek = String(item.endWeek || '').trim()
            const maxHoursPerWeek = String(item.maxHoursPerWeek || '').trim()
            if (!courseCode || !teacherCode) { errors.push(`第${i + 1}行: 课程代码或工号为空`); continue }
            try {
              const row = [courseCode, teacherCode, classCodes.join(','), totalHours, startWeek, endWeek, maxHoursPerWeek]
              const imported = await TS.ImportTeachingTasks(activeSemester.value?.ID || 0, [row])
              if (imported[0] > 0) count += imported[0]
              if (imported[1] && imported[1].length > 0) errors.push(...imported[1])
            } catch { errors.push(`第${i + 1}行: 后端导入接口调用失败`) }
          } else {
            await callCreate(tab, item)
            count++
          }
        } catch (e) {
          errors.push(`第${i + 1}行: ${(e as any)?.message || '未知错误'}`)
        }
      }
      if (count > 0) message.success(`成功导入 ${count} 条记录`)
      if (errors.length > 0) message.warning(`导入完成，${errors.length} 行失败：\n${errors.slice(0, 5).join('\n')}${errors.length > 5 ? `\n...还有 ${errors.length - 5} 行错误` : ''}`)
      resourceStore.loadAll()
    } catch (err) {
      message.error('导入失败：' + (err as any).message)
    }
  }
  reader.readAsArrayBuffer(file)
  if (e.target) (e.target as HTMLInputElement).value = ''
}

function downloadTemplate() {
  const tab = resourceStore.activeTab
  const schema = buildSchema(tab)
  const rows = [schema.headerRow, schema.indicatorRow, schema.exampleRow]
  const ws = XLSX.utils.aoa_to_sheet(rows)
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
        placeholder="选择学院"
        filterable
        :filter="fuzzyFilterFn"
        clearable
        size="small"
        style="width: 160px"
      />
      <div class="spacer"></div>
      <n-button size="small" type="primary" @click="openCreate()">+ 新增</n-button>
      <n-button size="small" @click="triggerImport()">导入Excel</n-button>
      <n-button size="small" @click="downloadTemplate()">下载模板</n-button>
      <n-button v-if="resourceStore.activeTab === 'teachingTask'" size="small" type="warning" @click="handleDetectMerge()">智能检测合班</n-button>
      <n-button size="small" type="error" :loading="clearing" @click="clearAll()">一键清空</n-button>
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
	          <n-select v-else-if="f.type === 'select'" v-model:value="formData[f.key]" :options="resolveOptions(f)" :filterable="f.filterable" :filter="f.filterable ? fuzzyFilterFn : undefined" :clearable="true" :placeholder="'请选择' + f.label" />
          <n-select v-else-if="f.type === 'multiSelect'" v-model:value="formData[f.key]" :options="resolveOptions(f)" :filterable="f.filterable" :filter="f.filterable ? fuzzyFilterFn : undefined" :multiple="true" :clearable="true" :placeholder="'请选择' + f.label" />
          <div v-else-if="f.type === 'timegrid'" class="time-grid">
            <div class="tg-header">
              <span class="tg-label"></span>
              <span v-for="d in 7" :key="d" class="tg-day">{{ DAY_NAMES[d-1] }}</span>
            </div>
            <div v-for="row in timeGridRows" :key="row.start" class="tg-row">
              <span class="tg-label">
                <span class="tg-period">{{ row.label }}</span>
                <span class="tg-time">{{ row.time }}</span>
              </span>
              <span v-for="d in 7" :key="d"
                class="tg-cell"
                :class="{ active: isSlotUnavailable(d-1, row.start) }"
                @click="toggleSlot(d-1, row.start, row.span)"
              ></span>
            </div>
          </div>
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

/* ===== 不可用时间网格 ===== */
.time-grid {
  width: 100%;
}
.tg-header, .tg-row {
  display: flex;
  align-items: center;
  gap: 2px;
  margin-bottom: 2px;
}
.tg-label {
  width: 68px;
  font-size: 12px;
  color: var(--n-text-color-2, #666);
  text-align: right;
  padding-right: 6px;
  flex-shrink: 0;
  line-height: 1.35;
  display: flex;
  flex-direction: column;
  align-items: flex-end;
}
.tg-period {
  color: var(--n-text-color, #333);
}
.tg-time {
  font-size: 10px;
  color: var(--n-text-color-3, #999);
  white-space: nowrap;
}
.tg-day {
  flex: 1;
  text-align: center;
  font-size: 11px;
  color: var(--n-text-color-2, #666);
  min-width: 0;
}
.tg-cell {
  flex: 1;
  aspect-ratio: 1;
  border-radius: 3px;
  border: 1.5px solid var(--n-border-color, #d0d0d0);
  background: var(--n-color, #fafafa);
  cursor: pointer;
  transition: all 0.15s;
  min-width: 0;
}
.tg-cell:hover {
  border-color: var(--n-primary-color, #3575f0);
  background: var(--n-primary-color-suppl, #e8f0fe);
}
.tg-cell.active {
  background: #e88080;
  border-color: #d04040;
}
.tg-cell.active:hover {
  background: #d06060;
}
</style>
