<script setup lang="ts">
import { inject, computed, ref } from 'vue'
import { PERIODS, DAY_NAMES } from '../../types'
import type { ScheduleEntry } from '../../types'
import { useScheduleStore } from '../../stores/schedule'
import { useMessage } from 'naive-ui'

const scheduleStore = useScheduleStore()
const drawerRef = inject<any>('drawerRef')
const message = useMessage()

const displayEntries = computed(() => scheduleStore.entries)

// Drag state
const dragEntry = ref<ScheduleEntry | null>(null)
const dragOverDay = ref(-1)
const dragOverPeriod = ref(-1)
const conflictFlash = ref<{ day: number; period: number } | null>(null)

// Dynamic today
const todayDow = computed(() => {
  const d = new Date().getDay()
  return d === 0 ? 6 : d - 1
})

// Dynamic dates based on current week
const weekDates = computed(() => {
  const now = new Date()
  const startOfYear = new Date(now.getFullYear(), 0, 1)
  const daysSinceStart = (scheduleStore.currentWeek - 1) * 7
  const base = new Date(startOfYear.getTime() + daysSinceStart * 86400000)
  const monday = new Date(base.getTime() - (base.getDay() === 0 ? 6 : base.getDay() - 1) * 86400000)
  return DAY_NAMES.map((_, i) => {
    const d = new Date(monday.getTime() + i * 86400000)
    return `${d.getMonth() + 1}/${d.getDate()}`
  })
})

function getCourseAt(day: number, period: number): ScheduleEntry | undefined {
  return displayEntries.value.find(e => e.dayOfWeek === day && period >= e.startPeriod && period < e.startPeriod + e.span)
}

function isFirstCell(entry: ScheduleEntry, period: number): boolean {
  return entry.startPeriod === period
}

function openCourseDetail(entry: ScheduleEntry) {
  if (!drawerRef?.value) return
  drawerRef.value.openDrawer(entry)
}

// ---- Drag & Drop ----
function onDragStart(e: DragEvent, entry: ScheduleEntry) {
  dragEntry.value = entry
  if (e.dataTransfer) {
    e.dataTransfer.effectAllowed = 'move'
    e.dataTransfer.setData('text/plain', String(entry.ID))
  }
}

function onDragEnd() {
  dragEntry.value = null
  dragOverDay.value = -1
  dragOverPeriod.value = -1
}

function onDragOver(e: DragEvent, day: number, period: number) {
  e.preventDefault()
  if (e.dataTransfer) {
    e.dataTransfer.dropEffect = 'move'
  }
  dragOverDay.value = day
  dragOverPeriod.value = period
}

function onDragLeave() {
  dragOverDay.value = -1
  dragOverPeriod.value = -1
}

async function onDrop(e: DragEvent, day: number, period: number) {
  e.preventDefault()
  dragOverDay.value = -1
  dragOverPeriod.value = -1

  if (!dragEntry.value) return
  const entry = dragEntry.value

  // Don't drop on same position
  if (entry.dayOfWeek === day && entry.startPeriod === period) {
    dragEntry.value = null
    return
  }

  try {
    // Call CheckMove API
    const { CheckMove } = await import('../../../bindings/scheduling-system/services/moveservice')
    const result = await CheckMove({
      entryId: entry.ID,
      newDay: day,
      newPeriod: period,
      newSpan: entry.span,
    })

    if (!result.valid) {
      // Show conflict feedback
      conflictFlash.value = { day, period }
      const conflictDesc = result.conflicts?.[0]?.description || '冲突'
      message.error(`无法移动：${conflictDesc}`)
      setTimeout(() => { conflictFlash.value = null }, 1500)
      dragEntry.value = null
      return
    }

    // Execute the move
    const { MoveEntry } = await import('../../../bindings/scheduling-system/services/moveservice')
    await MoveEntry({
      entryId: entry.ID,
      newDay: day,
      newPeriod: period,
      newSpan: entry.span,
    })

    message.success('课表已调整')
    // Reload schedule
    await scheduleStore.loadSchedule(scheduleStore.currentSemester)
  } catch (err: any) {
    message.error('调整失败：' + (err?.message || err))
  }

  dragEntry.value = null
}

// Check if a cell is a valid drop target (not occupied by another course, not locked)
function isDropTarget(day: number, period: number): boolean {
  const existing = getCourseAt(day, period)
  if (!existing) return true
  // Can drop on cells occupied by the dragged entry itself
  if (dragEntry.value && existing.ID === dragEntry.value.ID) return true
  return false
}
</script>

<template>
  <div class="week-view">
    <div v-if="displayEntries.length === 0" class="empty-state">暂无排课数据，请先运行自动排课</div>
    <div v-else class="schedule-grid">
      <div class="grid-corner">节次</div>
      <div v-for="(name, di) in DAY_NAMES" :key="di" class="grid-header" :class="{ today: di === todayDow }">
        <span class="day-name">{{ name }}</span>
        <span class="day-date">{{ weekDates[di] }}</span>
      </div>
      <template v-for="(period, pi) in PERIODS" :key="pi">
        <div class="time-label">
          <span class="period-num">{{ period.num }}</span>
          <span class="period-time">{{ period.time.replace('\n', ' ') }}</span>
        </div>
        <div
          v-for="(_, di) in DAY_NAMES"
          :key="di"
          class="grid-cell"
          :class="{
            'drag-over': dragOverDay === di && dragOverPeriod === pi && isDropTarget(di, pi),
            'conflict-flash': conflictFlash?.day === di && conflictFlash?.period === pi,
            'drop-target': dragEntry && isDropTarget(di, pi),
          }"
          @dragover="onDragOver($event, di, pi)"
          @dragleave="onDragLeave"
          @drop="onDrop($event, di, pi)"
        >
          <template v-if="getCourseAt(di, pi) && isFirstCell(getCourseAt(di, pi)!, pi)">
            <div
              class="course-card"
              :class="['course-' + (getCourseAt(di, pi)!.course?.dept || 'cs')]"
              :style="{ gridRow: 'span ' + getCourseAt(di, pi)!.span }"
              draggable="true"
              @dragstart="onDragStart($event, getCourseAt(di, pi)!)"
              @dragend="onDragEnd"
              @click="openCourseDetail(getCourseAt(di, pi)!)"
            >
              <div class="course-name">{{ getCourseAt(di, pi)!.course?.name || '' }}</div>
              <div class="course-detail">{{ getCourseAt(di, pi)!.classroom?.name || '' }} · {{ getCourseAt(di, pi)!.teacher?.name || '' }}</div>
            </div>
          </template>
        </div>
      </template>
    </div>
  </div>
</template>

<style scoped>
.week-view { flex: 1; display: flex; flex-direction: column; min-height: 0; }
.empty-state { display: flex; align-items: center; justify-content: center; flex: 1; color: var(--b3-theme-on-surface-light); font-size: 14px; }
.schedule-grid { flex: 1; display: grid; grid-template-columns: 60px repeat(7, 1fr); grid-template-rows: auto repeat(11, minmax(36px, 1fr)); gap: 1px; background: var(--b3-border-color); border: 1px solid var(--b3-border-color); border-radius: var(--b3-border-radius); overflow: hidden; }
.grid-corner, .grid-header, .time-label { background: var(--b3-theme-surface); display: flex; align-items: center; justify-content: center; font-size: 12px; font-weight: 500; color: var(--b3-theme-on-surface); }
.grid-header { flex-direction: column; gap: 1px; }
.grid-header.today { background: var(--b3-theme-primary-lightest); color: var(--b3-theme-primary); }
.day-name { font-size: 12px; }
.day-date { font-size: 10px; opacity: 0.7; }
.time-label { flex-direction: column; gap: 1px; font-size: 11px; }
.period-num { font-weight: 600; color: var(--b3-theme-on-background); }
.period-time { font-size: 9px; color: var(--b3-theme-on-surface-light); }
.grid-cell { background: var(--b3-theme-background); min-height: 48px; overflow: hidden; position: relative; transition: background 0.15s; }
.grid-cell.drop-target { background: var(--b3-theme-primary-lightest); opacity: 0.85; }
.grid-cell.drag-over { background: var(--b3-theme-primary-light); outline: 2px dashed var(--b3-theme-primary); outline-offset: -2px; z-index: 1; }
.grid-cell.conflict-flash { animation: conflictPulse 0.3s ease 3; background: var(--b3-theme-error-lightest) !important; }
@keyframes conflictPulse {
  0%, 100% { background: var(--b3-theme-error-lightest); }
  50% { background: var(--b3-theme-error); }
}
.course-card { height: 100%; padding: 4px 6px; font-size: 11px; cursor: grab; transition: box-shadow 0.15s, opacity 0.15s; border-left: 3px solid; overflow: hidden; }
.course-card:active { cursor: grabbing; }
.course-card:hover { box-shadow: var(--b3-point-shadow); }
.course-name { font-weight: 600; color: var(--b3-theme-on-background); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.course-detail { font-size: 10px; color: var(--b3-theme-on-surface-light); margin-top: 1px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.course-mech { background: var(--course-mech-bg); border-left-color: var(--course-mech-border); }
.course-elec { background: var(--course-elec-bg); border-left-color: var(--course-elec-border); }
.course-mate { background: var(--course-mate-bg); border-left-color: var(--course-mate-border); }
.course-bio { background: var(--course-bio-bg); border-left-color: var(--course-bio-border); }
.course-civil { background: var(--course-civil-bg); border-left-color: var(--course-civil-border); }
.course-cs { background: var(--course-cs-bg); border-left-color: var(--course-cs-border); }
.course-art { background: var(--course-art-bg); border-left-color: var(--course-art-border); }
.course-design { background: var(--course-design-bg); border-left-color: var(--course-design-border); }
.course-econ { background: var(--course-econ-bg); border-left-color: var(--course-econ-border); }
.course-eng { background: var(--course-eng-bg); border-left-color: var(--course-eng-border); }
.course-sci { background: var(--course-sci-bg); border-left-color: var(--course-sci-border); }
.course-marx { background: var(--course-marx-bg); border-left-color: var(--course-marx-border); }
.course-voc { background: var(--course-voc-bg); border-left-color: var(--course-voc-border); }
.course-intl { background: var(--course-intl-bg); border-left-color: var(--course-intl-border); }
.course-pe { background: var(--course-pe-bg); border-left-color: var(--course-pe-border); }
.course-cont { background: var(--course-cont-bg); border-left-color: var(--course-cont-border); }
.course-innov { background: var(--course-innov-bg); border-left-color: var(--course-innov-border); }
.course-engtech { background: var(--course-engtech-bg); border-left-color: var(--course-engtech-border); }
.course-detroit { background: var(--course-detroit-bg); border-left-color: var(--course-detroit-border); }
</style>
