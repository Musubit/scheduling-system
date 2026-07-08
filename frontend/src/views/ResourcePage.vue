<script setup lang="ts">
import { useResourceStore } from '../stores/resource'
import { NButton, NInput, NModal, NSelect, NDataTable, NSpace, NTag } from 'naive-ui'
import { DEPARTMENTS } from '../types'
import { ref, computed, h, onMounted } from 'vue'
import * as RS from '../../bindings/scheduling-system/services/resourceservice'
import * as XLSX from 'xlsx'

const resourceStore = useResourceStore()

onMounted(() => { resourceStore.loadAll() })

const tabOptions = [
  { key: 'teacher' as const, label: '教师' },
  { key: 'classroom' as const, label: '教室' },
  { key: 'course' as const, label: '课程' },
  { key: 'class' as const, label: '班级' },
]

// ===== 模拟数据 =====
const mockTeachers = [
  { id: 1, code: 'T001', name: '张建国', dept: '机械工程学院', title: '教授', status: 'active', weeklyHours: 12 },
  { id: 2, code: 'T002', name: '李明远', dept: '电气与电子工程学院', title: '教授', status: 'active', weeklyHours: 10 },
  { id: 3, code: 'T003', name: '王伟', dept: '材料与化学工程学院', title: '副教授', status: 'active', weeklyHours: 14 },
  { id: 4, code: 'T004', name: '刘芳', dept: '外国语学院', title: '讲师', status: 'active', weeklyHours: 16 },
  { id: 5, code: 'T005', name: '赵秀英', dept: '理学院', title: '副教授', status: 'active', weeklyHours: 8 },
  { id: 6, code: 'T006', name: '孙志强', dept: '经济与管理学院', title: '教授', status: 'active', weeklyHours: 10 },
  { id: 7, code: 'T007', name: '周海', dept: '计算机学院', title: '副教授', status: 'inactive', weeklyHours: 0 },
  { id: 8, code: 'T008', name: '钱学林', dept: '生物工程与食品学院', title: '教授', status: 'active', weeklyHours: 6 },
]

const mockClassrooms = [
  { id: 1, code: 'A301', name: 'A301', building: 'A栋', capacity: 80, type: '普通教室', status: 'available' },
  { id: 2, code: 'B205', name: 'B205', building: 'B栋', capacity: 60, type: '普通教室', status: 'available' },
  { id: 3, code: 'C502', name: 'C502', building: 'C栋', capacity: 120, type: '多媒体教室', status: 'available' },
  { id: 4, code: 'D401', name: 'D401', building: 'D栋', capacity: 200, type: '阶梯教室', status: 'maintenance' },
  { id: 5, code: 'A201', name: 'A201', building: 'A栋', capacity: 90, type: '普通教室', status: 'available' },
  { id: 6, code: 'E101', name: 'E101', building: 'E栋', capacity: 50, type: '实验室', status: 'available' },
  { id: 7, code: 'C301', name: 'C301', building: 'C栋', capacity: 100, type: '多媒体教室', status: 'available' },
]

const mockCourses = [
  { id: 1, code: 'CS301', name: '数据结构', dept: '计算机学院', credit: 4.0, type: '专业必修', hours: 64 },
  { id: 2, code: 'SC201', name: '高等数学', dept: '理学院', credit: 5.0, type: '公共必修', hours: 80 },
  { id: 3, code: 'EN101', name: '大学英语', dept: '外国语学院', credit: 3.0, type: '公共必修', hours: 48 },
  { id: 4, code: 'SC203', name: '大学物理', dept: '理学院', credit: 4.0, type: '公共必修', hours: 64 },
  { id: 5, code: 'CS302', name: '操作系统', dept: '计算机学院', credit: 4.0, type: '专业必修', hours: 64 },
  { id: 6, code: 'MX101', name: '马克思主义基本原理', dept: '马克思主义学院', credit: 2.0, type: '公共必修', hours: 32 },
  { id: 7, code: 'AD201', name: '设计素描', dept: '艺术设计学院', credit: 2.0, type: '专业必修', hours: 32 },
]

const mockClasses = [
  { id: 1, code: 'CS2301', name: '计算机2301', dept: '计算机学院', grade: 2023, students: 86 },
  { id: 2, code: 'CS2302', name: '计算机2302', dept: '计算机学院', grade: 2023, students: 82 },
  { id: 3, code: 'ME2301', name: '机械2301', dept: '机械工程学院', grade: 2023, students: 72 },
  { id: 4, code: 'EE2301', name: '电气2301', dept: '电气与电子工程学院', grade: 2023, students: 68 },
  { id: 5, code: 'CE2301', name: '土木2301', dept: '土木建筑与环境学院', grade: 2023, students: 55 },
  { id: 6, code: 'EM2301', name: '经管2301', dept: '经济与管理学院', grade: 2023, students: 78 },
  { id: 7, code: 'AD2301', name: '艺设2301', dept: '艺术设计学院', grade: 2023, students: 40 },
]

// ===== 列定义 =====
const teacherColumns = [
  { title: '工号', key: 'code', width: 80 },
  { title: '姓名', key: 'name', width: 100 },
  { title: '院系', key: 'dept', width: 160 },
  { title: '职称', key: 'title', width: 80 },
  { title: '课时', key: 'weeklyHours', width: 70 },
  { title: '状态', key: 'status', width: 80, render: (row: any) => h(NTag, { type: row.status === 'active' ? 'success' : 'default', size: 'small' }, { default: () => row.status === 'active' ? '在职' : '休假' }) },
  { title: '操作', key: 'actions', width: 140, render: () => h(NSpace, { size: 'small' }, { default: () => [h(NButton, { size: 'tiny', text: true }, { default: () => '编辑' }), h(NButton, { size: 'tiny', text: true, type: 'error' }, { default: () => '删除' }) ] }) },
]

const classroomColumns = [
  { title: '编号', key: 'code', width: 80 },
  { title: '教室名', key: 'name', width: 100 },
  { title: '教学楼', key: 'building', width: 80 },
  { title: '容量', key: 'capacity', width: 70 },
  { title: '类型', key: 'type', width: 100 },
  { title: '状态', key: 'status', width: 80, render: (row: any) => h(NTag, { type: row.status === 'available' ? 'success' : 'warning', size: 'small' }, { default: () => row.status === 'available' ? '可用' : '维护' }) },
  { title: '操作', key: 'actions', width: 140, render: () => h(NSpace, { size: 'small' }, { default: () => [h(NButton, { size: 'tiny', text: true }, { default: () => '编辑' }), h(NButton, { size: 'tiny', text: true, type: 'error' }, { default: () => '删除' }) ] }) },
]

const courseColumns = [
  { title: '编号', key: 'code', width: 80 },
  { title: '课程名', key: 'name', width: 140 },
  { title: '院系', key: 'dept', width: 140 },
  { title: '学分', key: 'credit', width: 60 },
  { title: '类型', key: 'type', width: 100 },
  { title: '课时', key: 'hours', width: 60 },
  { title: '操作', key: 'actions', width: 140, render: () => h(NSpace, { size: 'small' }, { default: () => [h(NButton, { size: 'tiny', text: true }, { default: () => '编辑' }), h(NButton, { size: 'tiny', text: true, type: 'error' }, { default: () => '删除' }) ] }) },
]

const classColumns = [
  { title: '编号', key: 'code', width: 100 },
  { title: '班级名', key: 'name', width: 140 },
  { title: '院系', key: 'dept', width: 160 },
  { title: '年级', key: 'grade', width: 70 },
  { title: '人数', key: 'students', width: 70 },
  { title: '操作', key: 'actions', width: 140, render: () => h(NSpace, { size: 'small' }, { default: () => [h(NButton, { size: 'tiny', text: true }, { default: () => '编辑' }), h(NButton, { size: 'tiny', text: true, type: 'error' }, { default: () => '删除' }) ] }) },
]

const deptOptions = [
  { label: '全部院系', value: '全部院系' },
  ...DEPARTMENTS.map(d => ({ label: d.name, value: d.name })),
]

// ===== Modal state =====
const showModal = ref(false)
const editingItem = ref<any>(null)
const formData = ref<Record<string, any>>({})

function openCreate() { editingItem.value = null; formData.value = {}; showModal.value = true }
function openEdit(row: any) { editingItem.value = row; formData.value = { ...row }; showModal.value = true }
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
      const idx = data.findIndex((i: any) => i.id === editingItem.value.id)
      if (idx >= 0) Object.assign(data[idx], formData.value)
    } else {
      data.push({ ...formData.value, id: Date.now() })
    }
  }
  closeModal()
}

async function deleteItem(row: any) {
  if (!confirm('确定要删除这条记录吗？')) return
  const data = getMockData(resourceStore.activeTab)
  try {
    await callDelete(resourceStore.activeTab, row.id)
  } catch (e) { console.warn('Delete via Go failed, local fallback:', e) }
  const idx = data.findIndex((i: any) => i.id === row.id)
  if (idx >= 0) data.splice(idx, 1)
  resourceStore.loadAll()
}

async function callCreate(tab: string, item: any) {
  const m = toModel(tab, item)
  switch (tab) {
    case 'teacher': await RS.CreateTeacher(m); break
    case 'classroom': await RS.CreateClassroom(m); break
    case 'course': await RS.CreateCourse(m); break
    case 'class': await RS.CreateClassGroup(m); break
  }
}
async function callUpdate(tab: string, id: number, item: any) {
  const m = toModel(tab, item)
  switch (tab) {
    case 'teacher': await RS.UpdateTeacher(m); break
    case 'classroom': await RS.UpdateClassroom(m); break
    case 'course': await RS.UpdateCourse(m); break
    case 'class': await RS.UpdateClassGroup(m); break
  }
}
async function callDelete(tab: string, id: number) {
  switch (tab) {
    case 'teacher': await RS.DeleteTeacher(id); break
    case 'classroom': await RS.DeleteClassroom(id); break
    case 'course': await RS.DeleteCourse(id); break
    case 'class': await RS.DeleteClassGroup(id); break
  }
}
function toModel(_tab: string, item: any): any {
  return { ...item, id: item.id || 0 }
}

function getMockData(tab: string): any[] {
  const map: Record<string, any[]> = { teacher: mockTeachers, classroom: mockClassrooms, course: mockCourses, class: mockClasses }
  return map[tab] || []
}

const formFields = computed(() => {
  const fields: Record<string, { key: string; label: string; type?: string }[]> = {
    teacher: [{ key: 'code', label: '工号' }, { key: 'name', label: '姓名' }, { key: 'dept', label: '院系' }, { key: 'title', label: '职称' }],
    classroom: [{ key: 'code', label: '编号' }, { key: 'name', label: '教室名' }, { key: 'building', label: '教学楼' }, { key: 'capacity', label: '容量', type: 'number' }, { key: 'type', label: '类型' }],
    course: [{ key: 'code', label: '编号' }, { key: 'name', label: '课程名' }, { key: 'dept', label: '院系' }, { key: 'credit', label: '学分', type: 'number' }, { key: 'type', label: '类型' }, { key: 'hours', label: '课时', type: 'number' }],
    class: [{ key: 'code', label: '编号' }, { key: 'name', label: '班级名' }, { key: 'dept', label: '院系' }, { key: 'grade', label: '年级', type: 'number' }, { key: 'students', label: '人数', type: 'number' }],
  }
  return fields[resourceStore.activeTab] || []
})

const actionRender = (row: any) => h(NSpace, { size: 'small' }, { default: () => [
  h(NButton, { size: 'tiny', text: true, onClick: () => openEdit(row) }, { default: () => '编辑' }),
  h(NButton, { size: 'tiny', text: true, type: 'error', onClick: () => deleteItem(row) }, { default: () => '删除' }),
]})

const teacherCols = [...teacherColumns.slice(0, -1), { title: '操作', key: 'actions', width: 140, render: actionRender }]
const classroomCols = [...classroomColumns.slice(0, -1), { title: '操作', key: 'actions', width: 140, render: actionRender }]
const courseCols = [...courseColumns.slice(0, -1), { title: '操作', key: 'actions', width: 140, render: actionRender }]
const classCols = [...classColumns.slice(0, -1), { title: '操作', key: 'actions', width: 140, render: actionRender }]

// ===== Excel Import / Export =====
const importFileRef = ref<HTMLInputElement>()

function triggerImport() { importFileRef.value?.click() }

async function handleFileChange(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (!file) return
  const reader = new FileReader()
  reader.onload = async (ev) => {
    try {
      const wb = XLSX.read(ev.target?.result, { type: 'binary' })
      const ws = wb.Sheets[wb.SheetNames[0]]
      const rows = XLSX.utils.sheet_to_json<any>(ws, { header: 1 })
      if (rows.length < 2) { alert('文件为空或格式不正确'); return }
      const headers = rows[0] as string[]
      const data = rows.slice(1).filter((r: any) => r.length > 0 && r[0])
      let count = 0
      for (const row of data) {
        const item: any = {}
        headers.forEach((h, i) => { item[h] = row[i] ?? '' })
        try { await callCreate(resourceStore.activeTab, item); count++ } catch {}
      }
      alert(`成功导入 ${count} 条记录`)
      resourceStore.loadAll()
    } catch (err) {
      alert('导入失败：' + (err as any).message)
    }
  }
  reader.readAsBinaryString(file)
  // Reset input
  if (e.target) (e.target as HTMLInputElement).value = ''
}

function downloadTemplate() {
  const tab = resourceStore.activeTab
  let headers: string[] = []
  let example: any[] = []
  switch (tab) {
    case 'teacher':
      headers = ['code', 'name', 'dept', 'title']
      example = ['T099', '张三', '理学院', '讲师']
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
  }
  const ws = XLSX.utils.aoa_to_sheet([headers, example])
  const wb = XLSX.utils.book_new()
  XLSX.utils.book_append_sheet(wb, ws, 'Sheet1')
  XLSX.writeFile(wb, `${tab}-template.xlsx`)
}
</script>

<template>
  <div class="resource-page">
    <div class="resource-tabs">
      <n-button
        v-for="tab in tabOptions"
        :key="tab.key"
        :type="resourceStore.activeTab === tab.key ? 'primary' : 'default'"
        size="small"
        @click="resourceStore.switchTab(tab.key)"
      >
        {{ tab.label }}
      </n-button>
    </div>

    <div class="resource-toolbar">
      <n-input
        :placeholder="`搜索${tabOptions.find(t => t.key === resourceStore.activeTab)?.label || ''}...`"
        clearable
        size="small"
        style="width: 240px"
      />
      <n-select
        :options="deptOptions"
        size="small"
        style="width: 160px"
        default-value="全部院系"
      />
      <div class="spacer"></div>
      <n-button size="small" type="primary" @click="openCreate()">+ 新增</n-button>
      <n-button size="small" @click="triggerImport()">导入Excel</n-button>
      <n-button size="small" @click="downloadTemplate()">下载模板</n-button>
      <input ref="importFileRef" type="file" accept=".xlsx,.xls" style="display:none" @change="handleFileChange" />
    </div>

    <div class="resource-table">
      <n-data-table v-if="resourceStore.activeTab === 'teacher'" :columns="teacherCols" :data="resourceStore.filteredTeachers.length ? resourceStore.filteredTeachers : mockTeachers" :single-line="false" size="small" />
      <n-data-table v-else-if="resourceStore.activeTab === 'classroom'" :columns="classroomCols" :data="resourceStore.filteredClassrooms.length ? resourceStore.filteredClassrooms : mockClassrooms" :single-line="false" size="small" />
      <n-data-table v-else-if="resourceStore.activeTab === 'course'" :columns="courseCols" :data="resourceStore.filteredCourses.length ? resourceStore.filteredCourses : mockCourses" :single-line="false" size="small" />
      <n-data-table v-else-if="resourceStore.activeTab === 'class'" :columns="classCols" :data="resourceStore.filteredClasses.length ? resourceStore.filteredClasses : mockClasses" :single-line="false" size="small" />
    </div>

    <!-- Form Modal -->
    <n-modal v-model:show="showModal" :title="(editingItem ? '编辑' : '新增') + (tabOptions.find(t => t.key === resourceStore.activeTab)?.label || '')">
      <div style="padding: 12px 0; display: flex; flex-direction: column; gap: 12px;">
        <div v-for="f in formFields" :key="f.key" style="display: flex; align-items: center; gap: 8px;">
          <label style="width: 60px; font-size: 13px; color: var(--b3-theme-on-surface); flex-shrink: 0;">{{ f.label }}</label>
          <n-input v-model:value="formData[f.key]" size="small" :placeholder="f.label" :type="f.type === 'number' ? 'number' : 'text'" style="flex: 1;" />
        </div>
      </div>
      <template #footer>
        <n-space justify="end">
          <n-button size="small" @click="closeModal()">取消</n-button>
          <n-button size="small" type="primary" @click="saveItem()">保存</n-button>
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

.resource-tabs {
  display: flex;
  gap: 8px;
  margin-bottom: 16px;
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
</style>
