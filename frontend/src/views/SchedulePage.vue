<script setup lang="ts">
import { ref, computed } from 'vue'
import { useScheduleStore } from '../stores/schedule'
import { useResourceStore } from '../stores/resource'
import { useAppStore } from '../stores/app'
import WeekView from '../components/schedule/WeekView.vue'
import TimelineView from '../components/schedule/TimelineView.vue'
import MonthView from '../components/schedule/MonthView.vue'
import { NButton, NDropdown, NSelect, NModal, NInput, NSpace, useMessage } from 'naive-ui'
import * as XLSX from 'xlsx'
import { jsPDF } from 'jspdf'
import html2canvas from 'html2canvas'
import { DAY_NAMES, DEPARTMENTS } from '../types'

const scheduleStore = useScheduleStore()
const resourceStore = useResourceStore()
const appStore = useAppStore()
const exporting = ref(false)
const message = useMessage()

// Save-as-version modal
const showSaveModal = ref(false)
const versionName = ref('')
const savingVersion = ref(false)

const defaultVersionName = computed(() => {
  const now = new Date()
  const pad = (n: number) => n.toString().padStart(2, '0')
  return `手动方案 ${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())} ${pad(now.getHours())}:${pad(now.getMinutes())}`
})

async function handleSaveVersion() {
  savingVersion.value = true
  try {
    const { CreateManualVersion } = await import('../../bindings/scheduling-system/backend/services/versionservice')
    await CreateManualVersion(appStore.currentSemesterName, versionName.value.trim())
    message.success('课表方案已保存')
    showSaveModal.value = false
    versionName.value = ''
  } catch (err: any) {
    message.error('保存失败：' + (err?.message || err))
  } finally {
    savingVersion.value = false
  }
}

// Perspective state
const perspectives = [
  { label: '教师', value: 'teacher' as const },
  { label: '班级', value: 'class' as const },
]
const filterDept = ref<string | null>(null)
const filterTeacherId = ref<number | null>(null)
const filterClassId = ref<number | null>(null)

// Sync to store
function selectPerspective(p: 'teacher' | 'class') {
  scheduleStore.setPerspective(p)
  filterDept.value = null
  filterTeacherId.value = null
  filterClassId.value = null
}

function syncTeacher() {
  scheduleStore.selectedTeacherId = filterTeacherId.value
  scheduleStore.selectedClassId = null
}
function syncClass() {
  scheduleStore.selectedClassId = filterClassId.value
  scheduleStore.selectedTeacherId = null
}

// Department options
const deptOptions = DEPARTMENTS.map(d => ({ label: d.name, value: d.name }))

// Teacher options filtered by dept
const teacherOptions = computed(() => {
  let list = resourceStore.teachers
  if (filterDept.value) {
    list = list.filter(t => t.dept === filterDept.value)
  }
  return list.map(t => ({ label: t.name, value: t.ID }))
})

// Class options filtered by dept
const classOptions = computed(() => {
  let list = resourceStore.classGroups
  if (filterDept.value) {
    list = list.filter(c => c.dept === filterDept.value)
  }
  return list.map(c => ({ label: c.name, value: c.ID }))
})

import { fuzzyFilterFn } from '../utils/fuzzyFilter'

// Whether to show the schedule
const showSchedule = computed(() => {
  if (scheduleStore.perspective === 'teacher' && scheduleStore.selectedTeacherId) return true
  if (scheduleStore.perspective === 'class' && scheduleStore.selectedClassId) return true
  return false
})

const hintText = computed(() => {
  if (scheduleStore.perspective === 'teacher' && !scheduleStore.selectedTeacherId) {
    return '请选择学院和教师查看课表'
  }
  if (scheduleStore.perspective === 'class' && !scheduleStore.selectedClassId) {
    return '请选择学院和班级查看课表'
  }
  return ''
})

function entryClassNames(e: any): string[] {
  if (e.teachingTask?.classes?.length) {
    return e.teachingTask.classes.map((c: any) => c.classGroup?.name || '').filter(Boolean)
  }
  if (e.classGroup?.name) return [e.classGroup.name]
  return []
}

async function exportSchedule(mode: 'teacher' | 'class') {
  exporting.value = true
  try {
    // Export the FULL timetable (all currently loaded entries), grouped by the
    // chosen dimension — independent of the current perspective filter.
    // Use a temporary load so we don't clobber the store's current entries.
    const { GetScheduleEntries } = await import('../../bindings/scheduling-system/backend/services/resourceservice')
    let all: any[]
    try {
      all = (await GetScheduleEntries('')) || []
    } catch {
      all = scheduleStore.entries
    }
    if (!all.length) {
      window.alert('暂无排课数据，请先运行自动排课')
      return
    }
    const labelMap = { teacher: '按教师', class: '按班级' } as const
    const rows: any[] = []
    for (const e of all) {
      const classNames = entryClassNames(e)
      const classLabel = classNames.join('、')
      let groups: string[]
      if (mode === 'teacher') groups = [e.teacher?.name || '未知教师']
      else groups = classNames.length ? classNames : ['未知班级']
      for (const g of groups) {
        rows.push({
          '分组': g,
          '课程名称': e.course?.name || '',
          '课程编号': e.course?.code || '',
          '教师': e.teacher?.name || '',
          '教室': e.classroom ? `${e.classroom.building?.name || e.classroom.code} ${e.classroom.name}` : '',
          '星期': DAY_NAMES[e.dayOfWeek] || '',
          '节次': `第${e.startPeriod + 1}-${e.startPeriod + e.span}节`,
          '教学周': e.weeks || '',
          '班级': classLabel,
          '学分': e.course?.credit ?? '',
        })
      }
    }
    // 按模式去除冗余列
    const dropCol = mode === 'teacher' ? '教师' : null
    const filteredRows = dropCol
      ? rows.map(r => { const { [dropCol]: _, ...rest } = r; return rest })
      : rows
    filteredRows.sort((a, b) => {
      const g = String(a['分组']).localeCompare(String(b['分组']))
      if (g !== 0) return g
      const dow = DAY_NAMES.indexOf(a['星期']) - DAY_NAMES.indexOf(b['星期'])
      if (dow !== 0) return dow
      return String(a['节次']).localeCompare(String(b['节次']))
    })

    const ws = XLSX.utils.json_to_sheet(filteredRows)
    // 基础格式
    const range = XLSX.utils.decode_range(ws['!ref'] || 'A1')
    for (let C = range.s.c; C <= range.e.c; C++) {
      const addr = XLSX.utils.encode_col(C) + '1'
      if (ws[addr]) ws[addr].s = { font: { bold: true } }
    }
    ws['!autofilter'] = { ref: ws['!ref'] || 'A1' }
    // 自动列宽（估算）
    const colWidths: { wch: number }[] = []
    for (let C = range.s.c; C <= range.e.c; C++) {
      let maxLen = 8
      for (let R = range.s.r; R <= range.e.r; R++) {
        const cell = ws[XLSX.utils.encode_cell({ r: R, c: C })]
        if (cell?.v != null) maxLen = Math.max(maxLen, String(cell.v).length)
      }
      colWidths.push({ wch: Math.min(maxLen + 4, 40) })
    }
    ws['!cols'] = colWidths

    const wb = XLSX.utils.book_new()
    XLSX.utils.book_append_sheet(wb, ws, '课表')
    XLSX.writeFile(wb, `排课表_${labelMap[mode]}_${new Date().toISOString().slice(0, 10)}_${Math.random().toString(16).slice(2, 8)}.xlsx`)
  } catch (err: any) {
    window.alert('导出失败：' + (err?.message || err))
  } finally {
    exporting.value = false
  }
}

async function exportSchedulePDF() {
  exporting.value = true
  try {
    // 空数据保护：检查是否有可导出的课程
    if (scheduleStore.displayEntries.length === 0) {
      const hasSelection = scheduleStore.selectedTeacherId || scheduleStore.selectedClassId
      window.alert(hasSelection ? '当前对象暂无课程' : '当前没有可导出的课表，请先选择教师或班级')
      return
    }

    // 截图当前课表视图（非数据表格）
    const scheduleEl = document.querySelector('.schedule-content') as HTMLElement
    if (!scheduleEl) {
      window.alert('课表视图未加载，请刷新后重试')
      return
    }

    // 确定标题信息：当前视图 + 选中的对象
    let title = '课表'
    if (scheduleStore.perspective === 'teacher' && scheduleStore.selectedTeacherId) {
      const t = scheduleStore.displayEntries[0]?.teacher
      title = `${t?.name || '教师'} 课表`
    } else if (scheduleStore.perspective === 'class' && scheduleStore.selectedClassId) {
      title = '班级课表'
    }

    // 构建专用导出容器：克隆课表网格 + DOM标题（避免jsPDF中文乱码）
    const grid = document.querySelector('.schedule-grid') as HTMLElement
    if (!grid) {
      window.alert('课表网格未加载，请刷新后重试')
      return
    }

    const EXPORT_W = 1400
    const container = document.createElement('div')
    // B3 主题变量（保证克隆grid的样式继承）
    const b3Vars: Record<string, string> = {
      '--b3-theme-background': '#ffffff',
      '--b3-theme-surface': '#f6f6f6',
      '--b3-theme-on-surface': '#333333',
      '--b3-theme-on-background': '#222222',
      '--b3-theme-on-surface-light': '#999999',
      '--b3-border-color': '#e0e0e0',
      '--b3-border-radius': '6px',
      '--b3-border-radius-s': '4px',
      '--b3-theme-primary': '#3575f0',
      '--b3-theme-primary-light': '#5b8af7',
      '--b3-theme-primary-lightest': '#e8f0fe',
      '--b3-theme-error': '#e53935',
    }
    container.style.cssText = `position:fixed;left:-30000px;top:0;width:${EXPORT_W}px;background:#fff;padding:14px 18px 10px;font-family:"Microsoft YaHei","PingFang SC",sans-serif;color:#222;`
    for (const [k, v] of Object.entries(b3Vars)) container.style.setProperty(k, v)

    // 标题（DOM API 构建，防止 XSS）
    const dateStr = new Date().toLocaleString()
    const titleEl = document.createElement('div')
    titleEl.style.cssText = 'font-size:20px;font-weight:700;margin-bottom:4px;line-height:1.4;'
    titleEl.textContent = title
    container.appendChild(titleEl)
    const subEl = document.createElement('div')
    subEl.style.cssText = 'font-size:12px;color:#888;margin-bottom:12px;'
    subEl.textContent = `第${scheduleStore.currentWeek}周　生成时间：${dateStr}`
    container.appendChild(subEl)

    // 克隆课表网格
    const gridClone = grid.cloneNode(true) as HTMLElement
    gridClone.style.setProperty('overflow', 'visible')
    gridClone.style.setProperty('height', 'auto')
    gridClone.style.setProperty('flex', 'none')
    // 移除工具栏（如果被clone进来了——WeekView的工具栏是grid的兄弟，cloneNode只clone grid本身所以不会带）
    gridClone.querySelector('.week-toolbar')?.remove()
    container.appendChild(gridClone)
    document.body.appendChild(container)

    try {
      const canvas = await html2canvas(container, { scale: 3, backgroundColor: '#ffffff' })

      const pdf = new jsPDF('l', 'mm', 'a4')
      const pageW = pdf.internal.pageSize.getWidth()
      const pageH = pdf.internal.pageSize.getHeight()
      const imgW = pageW - 14
      const imgH = (canvas.height * imgW) / canvas.width
      const imgData = canvas.toDataURL('image/png')

      let heightLeft = imgH
      let position = 7
      pdf.addImage(imgData, 'PNG', 7, position, imgW, imgH)
      heightLeft -= (pageH - position)
      while (heightLeft > 0) {
        position -= pageH
        pdf.addPage()
        pdf.addImage(imgData, 'PNG', 7, position, imgW, imgH)
        heightLeft -= pageH
      }

      const fileDate = new Date().toISOString().slice(0, 10)
      const hash6 = Math.random().toString(16).slice(2, 8)
      pdf.save(`${title}_${fileDate}_${hash6}.pdf`)
    } finally {
      document.body.removeChild(container)
    }
  } catch (err: any) {
    window.alert('导出PDF失败：' + (err?.message || err))
  } finally {
    exporting.value = false
  }
}

const exportOptions = [
  { label: '按教师导出', key: 'teacher' as const },
  { label: '按班级导出', key: 'class' as const },
]

// 合并导出菜单：Excel数据 + PDF打印
const combinedExportOptions: any[] = [
  { type: 'group', key: 'excel-header', label: 'Excel（数据）' },
  ...exportOptions.map(o => ({ key: 'excel:' + o.key, label: '　' + o.label })),
  { type: 'divider', key: 'div1' },
  { key: 'pdf', label: 'PDF（当前课表）' },
]

function handleExportSelect(key: string) {
  if (key === 'pdf') {
    exportSchedulePDF()
  } else if (key.startsWith('excel:')) {
    exportSchedule(key.slice(6) as 'teacher' | 'class')
  }
}
</script>

<template>
  <div class="schedule-page">
    <!-- Header row 1: title + navigation -->
    <div class="schedule-header">
      <div class="schedule-title">
        <template v-if="scheduleStore.currentView === 'week'">
          第 {{ scheduleStore.currentWeek }} 周
        </template>
        <template v-else-if="scheduleStore.currentView === 'timeline'">
          时间线视图 · 第 {{ scheduleStore.currentWeek }} 周
        </template>
        <template v-else>
          {{ scheduleStore.currentYear }} 年 {{ scheduleStore.currentMonth }} 月
        </template>
      </div>
      <div class="schedule-meta">
        <div class="nav-btns">
          <button class="nav-arrow-btn" title="上一周/月" @click="scheduleStore.currentView === 'month' ? scheduleStore.prevMonth() : scheduleStore.prevWeek()">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="15 18 9 12 15 6"/></svg>
          </button>
          <span class="nav-label" v-if="scheduleStore.currentView !== 'month'">第 {{ scheduleStore.currentWeek }} 周</span>
          <span class="nav-label" v-else>{{ scheduleStore.currentYear }}年{{ scheduleStore.currentMonth }}月</span>
          <button class="nav-arrow-btn" title="下一周/月" @click="scheduleStore.currentView === 'month' ? scheduleStore.nextMonth() : scheduleStore.nextWeek()">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="9 18 15 12 9 6"/></svg>
          </button>
        </div>
        <span class="stat-badge">已排 {{ scheduleStore.filteredCount }} 门课</span>
        <n-button size="small" @click="showSaveModal = true" :disabled="scheduleStore.viewMode === 'version'">另存为方案</n-button>
        <n-dropdown trigger="click" :options="combinedExportOptions" @select="handleExportSelect">
          <n-button size="small" :loading="exporting">导出</n-button>
        </n-dropdown>
      </div>
    </div>

    <!-- Header row 2: perspective tabs -->
    <div class="perspective-bar">
      <div class="perspective-tabs">
        <button
          v-for="p in perspectives" :key="p.value"
          class="perspective-tab"
          :class="{ active: scheduleStore.perspective === p.value }"
          @click="selectPerspective(p.value)"
        >{{ p.label }}</button>
      </div>
      <div class="perspective-filters">
        <n-select
          v-model:value="filterDept"
          :options="deptOptions"
          placeholder="选择学院"
          filterable
          :filter="fuzzyFilterFn"
          clearable
          size="small"
          style="width: 150px"
          @update:value="syncTeacher(); syncClass()"
        />
        <n-select
          v-if="scheduleStore.perspective === 'teacher'"
          v-model:value="filterTeacherId"
          :options="teacherOptions"
          placeholder="选择教师"
          filterable
          :filter="fuzzyFilterFn"
          clearable
          size="small"
          style="width: 140px"
          @update:value="syncTeacher()"
        />
        <n-select
          v-if="scheduleStore.perspective === 'class'"
          v-model:value="filterClassId"
          :options="classOptions"
          placeholder="选择班级"
          filterable
          :filter="fuzzyFilterFn"
          clearable
          size="small"
          style="width: 140px"
          @update:value="syncClass()"
        />
      </div>
    </div>

    <!-- Content -->
    <div v-if="showSchedule" class="schedule-content">
      <WeekView v-if="scheduleStore.currentView === 'week'" />
      <TimelineView v-else-if="scheduleStore.currentView === 'timeline'" />
      <MonthView v-else />
    </div>
    <div v-else-if="hintText" class="perspective-hint">{{ hintText }}</div>
  </div>

  <!-- Save-as-version modal -->
  <n-modal v-model:show="showSaveModal" preset="card" title="保存当前课表方案" style="width: 420px;">
    <n-form label-placement="top">
      <n-form-item label="方案名称">
        <n-input
          v-model:value="versionName"
          :placeholder="defaultVersionName"
          clearable
          @keyup.enter="handleSaveVersion"
        />
      </n-form-item>
    </n-form>
    <template #footer>
      <n-space justify="end">
        <n-button :loading="savingVersion" @click="showSaveModal = false">取消</n-button>
        <n-button type="primary" :loading="savingVersion" @click="handleSaveVersion">保存</n-button>
      </n-space>
    </template>
  </n-modal>
</template>

<style scoped>
.schedule-page {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.schedule-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
  flex-shrink: 0;
}

.schedule-title {
  font-size: 18px;
  font-weight: 600;
  color: var(--b3-theme-on-background);
}

.schedule-meta {
  display: flex;
  align-items: center;
  gap: 10px;
}

.nav-btns {
  display: flex;
  align-items: center;
  gap: 4px;
}

.nav-arrow-btn {
  width: 28px;
  height: 28px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: var(--b3-border-radius-s);
  color: var(--b3-theme-on-surface);
  cursor: pointer;
  transition: all 0.15s;
  border: none;
  background: var(--b3-theme-surface);
}

.nav-arrow-btn:hover:not(:disabled) {
  background: var(--b3-list-hover);
  color: var(--b3-theme-primary);
}

.nav-label {
  font-size: 13px;
  font-weight: 500;
  color: var(--b3-theme-on-background);
  min-width: 60px;
  text-align: center;
}

.stat-badge {
  font-size: 12px;
  color: var(--b3-theme-success);
  background: var(--b3-card-success-background);
  padding: 2px 10px;
  border-radius: var(--b3-border-radius-s);
}

.perspective-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
  flex-shrink: 0;
}

.perspective-tabs {
  display: flex;
  gap: 4px;
  background: var(--b3-theme-surface);
  padding: 3px;
  border-radius: 6px;
  border: 1px solid var(--b3-border-color);
}

.perspective-tab {
  padding: 4px 16px;
  border-radius: 4px;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  border: none;
  background: transparent;
  color: var(--b3-theme-on-surface);
  transition: all 0.15s;
}

.perspective-tab:hover {
  background: var(--b3-list-hover);
}

.perspective-tab.active {
  background: var(--b3-theme-primary);
  color: #fff;
}

.perspective-filters {
  display: flex;
  align-items: center;
  gap: 8px;
}

.perspective-hint {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--b3-theme-on-surface-light);
  font-size: 14px;
}

.schedule-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
}
</style>
