<script setup lang="ts">
import { ref, computed, h } from 'vue'
import { useScheduleStore } from '../stores/schedule'
import { useResourceStore } from '../stores/resource'
import WeekView from '../components/schedule/WeekView.vue'
import TimelineView from '../components/schedule/TimelineView.vue'
import MonthView from '../components/schedule/MonthView.vue'
import { NButton, NDropdown, NSelect } from 'naive-ui'
import * as XLSX from 'xlsx'
import { jsPDF } from 'jspdf'
import html2canvas from 'html2canvas'
import { DAY_NAMES, DEPARTMENTS } from '../types'

const scheduleStore = useScheduleStore()
const resourceStore = useResourceStore()
const exporting = ref(false)

// Perspective state — three dimensions
const perspectives = [
  { label: '教师', value: 'teacher' as const },
  { label: '教室', value: 'classroom' as const },
  { label: '班级', value: 'class' as const },
]
const filterDept = ref<string | null>(null)
const filterTeacherId = ref<number | null>(null)
const filterClassroomId = ref<number | null>(null)
const filterClassId = ref<number | null>(null)

// Sync to store
function selectPerspective(p: 'teacher' | 'classroom' | 'class') {
  scheduleStore.setPerspective(p)
  filterDept.value = null
  filterTeacherId.value = null
  filterClassroomId.value = null
  filterClassId.value = null
}

function syncTeacher() {
  scheduleStore.selectedTeacherId = filterTeacherId.value
  scheduleStore.selectedClassroomId = null
  scheduleStore.selectedClassId = null
}
function syncClassroom() {
  scheduleStore.selectedClassroomId = filterClassroomId.value
  scheduleStore.selectedTeacherId = null
  scheduleStore.selectedClassId = null
}
function syncClass() {
  scheduleStore.selectedClassId = filterClassId.value
  scheduleStore.selectedTeacherId = null
  scheduleStore.selectedClassroomId = null
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

// Classroom options
const classroomOptions = computed(() => {
  return resourceStore.classrooms.map(c => ({ label: `${c.name} (${c.building})`, value: c.ID }))
})

import { fuzzyFilter } from '../utils/fuzzyFilter'

const fuzzyFilterFn = fuzzyFilter as any

// Whether to show the schedule
const showSchedule = computed(() => {
  if (scheduleStore.perspective === 'teacher' && scheduleStore.selectedTeacherId) return true
  if (scheduleStore.perspective === 'classroom' && scheduleStore.selectedClassroomId) return true
  if (scheduleStore.perspective === 'class' && scheduleStore.selectedClassId) return true
  return false
})

const hintText = computed(() => {
  if (scheduleStore.perspective === 'teacher' && !scheduleStore.selectedTeacherId) {
    return '请选择学院和教师查看课表'
  }
  if (scheduleStore.perspective === 'classroom' && !scheduleStore.selectedClassroomId) {
    return '请选择教室查看课表'
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

async function exportSchedule(mode: 'teacher' | 'classroom' | 'class') {
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
    const labelMap = { teacher: '按教师', classroom: '按教室', class: '按班级' } as const
    const rows: any[] = []
    for (const e of all) {
      const classNames = entryClassNames(e)
      const classLabel = classNames.join('、')
      let groups: string[]
      if (mode === 'teacher') groups = [e.teacher?.name || '未知教师']
      else if (mode === 'classroom') groups = [e.classroom?.name || '未知教室']
      else groups = classNames.length ? classNames : ['未知班级']
      for (const g of groups) {
        rows.push({
          '分组': g,
          '课程名称': e.course?.name || '',
          '课程编号': e.course?.code || '',
          '教师': e.teacher?.name || '',
          '教室': e.classroom?.name || '',
          '星期': DAY_NAMES[e.dayOfWeek] || '',
          '节次': `第${e.startPeriod + 1}-${e.startPeriod + e.span}节`,
          '教学周': e.weeks || '',
          '班级': classLabel,
          '学分': e.course?.credit ?? '',
        })
      }
    }
    rows.sort((a, b) => String(a['分组']).localeCompare(String(b['分组'])))

    const ws = XLSX.utils.json_to_sheet(rows)
    const wb = XLSX.utils.book_new()
    XLSX.utils.book_append_sheet(wb, ws, '课表')
    XLSX.writeFile(wb, `排课表_${labelMap[mode]}_${new Date().toISOString().slice(0, 10)}.xlsx`)
  } catch (err: any) {
    window.alert('导出失败：' + (err?.message || err))
  } finally {
    exporting.value = false
  }
}

async function exportSchedulePDF(mode: 'teacher' | 'classroom' | 'class') {
  exporting.value = true
  try {
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
    const labelMap = { teacher: '按教师', classroom: '按教室', class: '按班级' } as const
    const headers = ['分组', '课程名称', '课程编号', '教师', '教室', '星期', '节次', '教学周', '班级', '学分']
    const rows: string[][] = []
    for (const e of all) {
      const classNames = entryClassNames(e)
      const classLabel = classNames.join('、')
      let groups: string[]
      if (mode === 'teacher') groups = [e.teacher?.name || '未知教师']
      else if (mode === 'classroom') groups = [e.classroom?.name || '未知教室']
      else groups = classNames.length ? classNames : ['未知班级']
      for (const g of groups) {
        rows.push([
          g,
          e.course?.name || '',
          e.course?.code || '',
          e.teacher?.name || '',
          e.classroom?.name || '',
          DAY_NAMES[e.dayOfWeek] || '',
          `第${e.startPeriod + 1}-${e.startPeriod + e.span}节`,
          e.weeks || '',
          classLabel,
          String(e.course?.credit ?? ''),
        ])
      }
    }
    rows.sort((a, b) => a[0].localeCompare(b[0]))

    // Render an offscreen HTML table (Chinese-safe via browser fonts),
    // rasterize with html2canvas, then paginate into a PDF.
    const container = document.createElement('div')
    container.style.cssText = 'position:fixed;left:-10000px;top:0;background:#fff;padding:16px;width:1120px;font-family:"Microsoft YaHei",sans-serif;color:#000;'
    const escapeHtml = (s: string) => s.replace(/[&<>\"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c] as string))
    const bodyRows = rows.map((r) => '<tr>' + r.map((c) => `<td style="border:1px solid #ccc;padding:4px 6px;font-size:12px;white-space:nowrap;">${escapeHtml(c)}</td>`).join('') + '</tr>').join('')
    const headRow = '<tr>' + headers.map((hh) => `<th style="border:1px solid #333;padding:4px 6px;font-size:12px;background:#f0f0f0;">${hh}</th>`).join('') + '</tr>'
    container.innerHTML = `<h3 style="font-family:inherit;margin:0 0 8px;">排课表（${labelMap[mode]}）　生成时间：${new Date().toLocaleString()}</h3><table style="border-collapse:collapse;width:100%;"><thead>${headRow}</thead><tbody>${bodyRows}</tbody></table>`
    document.body.appendChild(container)
    const canvas = await html2canvas(container, { scale: 2, backgroundColor: '#ffffff' })
    document.body.removeChild(container)

    const pdf = new jsPDF('p', 'mm', 'a4')
    const pageW = pdf.internal.pageSize.getWidth()
    const pageH = pdf.internal.pageSize.getHeight()
    const imgW = pageW
    const imgH = (canvas.height * imgW) / canvas.width
    const imgData = canvas.toDataURL('image/png')
    let heightLeft = imgH
    let position = 0
    pdf.addImage(imgData, 'PNG', 0, position, imgW, imgH)
    heightLeft -= pageH
    while (heightLeft > 0) {
      position -= pageH
      pdf.addPage()
      pdf.addImage(imgData, 'PNG', 0, position, imgW, imgH)
      heightLeft -= pageH
    }
    const dateStr = new Date().toISOString().slice(0, 10)
    pdf.save(`排课表_${labelMap[mode]}_${dateStr}.pdf`)
  } catch (err: any) {
    window.alert('导出PDF失败：' + (err?.message || err))
  } finally {
    exporting.value = false
  }
}

const exportOptions = [
  { label: '按教师导出', key: 'teacher' as const },
  { label: '按教室导出', key: 'classroom' as const },
  { label: '按班级导出', key: 'class' as const },
]

const exportPdfOptions = [
  { label: '按教师导出', key: 'teacher' as const },
  { label: '按教室导出', key: 'classroom' as const },
  { label: '按班级导出', key: 'class' as const },
]
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
        <n-dropdown trigger="click" :options="exportOptions" @select="exportSchedule">
          <n-button size="small" :loading="exporting">导出课表</n-button>
        </n-dropdown>
        <n-dropdown trigger="click" :options="exportPdfOptions" @select="exportSchedulePDF">
          <n-button size="small">导出PDF</n-button>
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
          @update:value="syncTeacher(); syncClassroom(); syncClass()"
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
          style="width: 120px"
          @update:value="syncTeacher()"
        />
        <n-select
          v-if="scheduleStore.perspective === 'classroom'"
          v-model:value="filterClassroomId"
          :options="classroomOptions"
          placeholder="选择教室"
          filterable
          :filter="fuzzyFilterFn"
          clearable
          size="small"
          style="width: 160px"
          @update:value="syncClassroom()"
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
