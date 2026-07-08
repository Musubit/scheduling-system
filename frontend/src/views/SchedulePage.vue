<script setup lang="ts">
import { ref, h } from 'vue'
import { useScheduleStore } from '../stores/schedule'
import WeekView from '../components/schedule/WeekView.vue'
import TimelineView from '../components/schedule/TimelineView.vue'
import MonthView from '../components/schedule/MonthView.vue'
import { NButton, NDropdown } from 'naive-ui'
import * as XLSX from 'xlsx'
import { DAY_NAMES } from '../types'

const scheduleStore = useScheduleStore()
const exporting = ref(false)

function exportSchedule(mode: 'teacher' | 'class' | 'dept') {
  exporting.value = true
  try {
    const entries = scheduleStore.entries
    if (!entries.length) return

    const rows: any[] = []
    const titleMap: Record<string, string> = {
      teacher: '按教师导出',
      class: '按班级导出',
      dept: '按系所导出',
    }

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

    // Sort by group
    rows.sort((a, b) => a['分组'].localeCompare(b['分组']))

    const ws = XLSX.utils.json_to_sheet(rows)
    const wb = XLSX.utils.book_new()
    XLSX.utils.book_append_sheet(wb, ws, '课表')
    XLSX.writeFile(wb, `排课表_${titleMap[mode]}_${new Date().toISOString().slice(0, 10)}.xlsx`)
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
    <div class="schedule-header">
      <div class="schedule-title">
        <template v-if="scheduleStore.currentView === 'week'">
          2025-2026 第二学期 · 第 {{ scheduleStore.currentWeek }} 周
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
        <span class="stat-badge">已排 {{ scheduleStore.totalCourses || '...' }} 门课</span>
        <n-dropdown trigger="click" :options="exportOptions" @select="exportSchedule">
          <n-button size="small" :loading="exporting">导出课表</n-button>
        </n-dropdown>
      </div>
    </div>

    <WeekView v-if="scheduleStore.currentView === 'week'" />
    <TimelineView v-else-if="scheduleStore.currentView === 'timeline'" />
    <MonthView v-else />
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
  margin-bottom: 16px;
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

.nav-arrow-btn:disabled {
  opacity: 0.3;
  cursor: default;
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
</style>
