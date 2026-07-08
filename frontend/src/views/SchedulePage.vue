<script setup lang="ts">
import { ref, computed, h } from 'vue'
import { useScheduleStore } from '../stores/schedule'
import { useResourceStore } from '../stores/resource'
import WeekView from '../components/schedule/WeekView.vue'
import TimelineView from '../components/schedule/TimelineView.vue'
import MonthView from '../components/schedule/MonthView.vue'
import { NButton, NDropdown, NSelect } from 'naive-ui'
import * as XLSX from 'xlsx'
import { DAY_NAMES, DEPARTMENTS } from '../types'

const scheduleStore = useScheduleStore()
const resourceStore = useResourceStore()
const exporting = ref(false)

// Perspective state
const perspectives = [
  { label: '全部', value: 'all' as const },
  { label: '教师', value: 'teacher' as const },
  { label: '班级', value: 'class' as const },
]
const filterDept = ref<string | null>(null)
const filterTeacherId = ref<number | null>(null)
const filterClassId = ref<number | null>(null)

// Sync to store
function selectPerspective(p: 'all' | 'teacher' | 'class') {
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

// Whether to show the schedule
const showSchedule = computed(() => {
  if (scheduleStore.perspective === 'all') return true
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

function exportSchedule(mode: 'teacher' | 'class' | 'dept') {
  exporting.value = true
  try {
    const entries = scheduleStore.displayEntries
    if (!entries.length) return

    const rows: any[] = []
    entries.forEach(e => {
      let groupKey = ''
      if (mode === 'teacher') groupKey = e.teacher?.name || '未知教师'
      else if (mode === 'class') groupKey = e.classGroup?.name || e.course?.name || '未知'
      else groupKey = e.course?.dept || '未知系所'

      rows.push({
        '分组': groupKey,
        '课程名称': e.course?.name || '',
        '课程编号': e.course?.code || '',
        '教师': e.teacher?.name || '',
        '教室': e.classroom?.name || '',
        '星期': DAY_NAMES[e.dayOfWeek] || '',
        '节次': `第${e.startPeriod + 1}-${e.startPeriod + e.span}节`,
        '教学周': e.weeks || '',
        '班级': e.classGroup?.name || '',
        '学分': e.course?.credit || '',
      })
    })

    rows.sort((a, b) => a['分组'].localeCompare(b['分组']))

    const ws = XLSX.utils.json_to_sheet(rows)
    const wb = XLSX.utils.book_new()
    XLSX.utils.book_append_sheet(wb, ws, '课表')
    XLSX.writeFile(wb, `排课表_${mode === 'teacher' ? '按教师' : mode === 'class' ? '按班级' : '按系所'}_${new Date().toISOString().slice(0, 10)}.xlsx`)
  } finally {
    exporting.value = false
  }
}

const exportOptions = [
  { label: '按教师导出', key: 'teacher' as const },
  { label: '按班级导出', key: 'class' as const },
  { label: '按系所导出', key: 'dept' as const },
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
      <div class="perspective-filters" v-if="scheduleStore.perspective !== 'all'">
        <n-select
          v-model:value="filterDept"
          :options="deptOptions"
          placeholder="选择学院"
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
          clearable
          size="small"
          style="width: 120px"
          @update:value="syncTeacher()"
        />
        <n-select
          v-if="scheduleStore.perspective === 'class'"
          v-model:value="filterClassId"
          :options="classOptions"
          placeholder="选择班级"
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
