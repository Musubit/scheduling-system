<script setup lang="ts">
import { inject, computed, ref, onMounted } from 'vue'
import { PERIODS, DAY_NAMES } from '../../types'
import type { ScheduleEntry, LockedTimeSlot } from '../../types'
import { useScheduleStore } from '../../stores/schedule'
import { useMessage } from 'naive-ui'
import { courseColorStyle } from '../../utils/courseColor'

const scheduleStore = useScheduleStore()
const drawerRef = inject<any>('drawerRef')
const message = useMessage()

const displayEntries = computed(() => scheduleStore.displayEntries)

// ---- Locked Time Slot Editing Mode ----
const editMode = ref(false)
const lockedSlots = ref<LockedTimeSlot[]>([])

const DEFAULT_LOCKED: LockedTimeSlot[] = [
  { dayOfWeek: 3, startPeriod: 4, span: 4 },
]

function loadLockedSlots() {
  try {
    const saved = localStorage.getItem('locked-time-slots')
    if (saved) {
      lockedSlots.value = JSON.parse(saved)
    } else {
      lockedSlots.value = [...DEFAULT_LOCKED]
      saveLockedSlots()
    }
  } catch {
    lockedSlots.value = [...DEFAULT_LOCKED]
  }
}

function saveLockedSlots() {
  localStorage.setItem('locked-time-slots', JSON.stringify(lockedSlots.value))
  import('../../../bindings/scheduling-system/backend/services/resourceservice').then(({ SaveSetting }) => {
    return SaveSetting('locked_time_slots', JSON.stringify(lockedSlots.value))
  }).catch((err: any) => {
    console.warn('[WeekView] 锁定时段保存到数据库失败:', err)
  })
}

function isLocked(day: number, period: number): boolean {
  return lockedSlots.value.some(ls =>
    ls.dayOfWeek === day &&
    period >= ls.startPeriod &&
    period < ls.startPeriod + ls.span
  )
}

function toggleLockCell(day: number, period: number) {
  if (!editMode.value) return
  const startPeriod = period % 2 === 0 ? period : period - 1
  const span = 2
  const existingIdx = lockedSlots.value.findIndex(ls =>
    ls.dayOfWeek === day && ls.startPeriod === startPeriod && ls.span === span
  )
  if (existingIdx >= 0) {
    lockedSlots.value.splice(existingIdx, 1)
  } else {
    lockedSlots.value.push({ dayOfWeek: day, startPeriod, span })
  }
  saveLockedSlots()
}

onMounted(() => loadLockedSlots())

// ---- Drag & Drop ----
const dragEntry = ref<ScheduleEntry | null>(null)
const dragOverDay = ref(-1)
const dragOverPeriod = ref(-1)
const conflictFlash = ref<{ day: number; period: number } | null>(null)

// Popover state
const popoverCell = ref<{ day: number; period: number } | null>(null)

// Dynamic today
const todayDow = computed(() => {
  const d = new Date().getDay()
  return d === 0 ? 6 : d - 1
})

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

// Returns ALL courses at a given cell
function getCoursesAt(day: number, period: number): ScheduleEntry[] {
  return displayEntries.value.filter(e => e.dayOfWeek === day && period >= e.startPeriod && period < e.startPeriod + e.span)
}

// Returns the first course (for main card rendering)
function getCourseAt(day: number, period: number): ScheduleEntry | undefined {
  return getCoursesAt(day, period)[0]
}

function isFirstCell(entry: ScheduleEntry, period: number): boolean {
  return entry.startPeriod === period
}

function cellStyle(day: number, period: number): Record<string, string> {
  const courses = getCoursesAt(day, period)
  if (courses.length === 0) return {}
  const first = courses[0]
  if (isFirstCell(first, period)) {
    return { gridRow: `span ${first.span}` }
  }
  return { display: 'none' }
}

function openCourseDetail(entry: ScheduleEntry) {
  if (!drawerRef?.value) return
  drawerRef.value.openDrawer(entry)
}

// Toggle popover
function togglePopover(day: number, period: number) {
  const courses = getCoursesAt(day, period)
  if (courses.length <= 1) return
  popoverCell.value = { day, period }
}

// Drag handlers
const dragSourceEl = ref<HTMLElement | null>(null)

function onDragStart(e: DragEvent, entry: ScheduleEntry) {
  if (editMode.value) return
  dragEntry.value = entry
  popoverCell.value = null

  // Add dragging class to source element for visual feedback
  const el = e.target as HTMLElement
  dragSourceEl.value = (el.closest('.course-card') || el.closest('.overflow-item')) as HTMLElement
  if (dragSourceEl.value) {
    dragSourceEl.value.classList.add('dragging-source')
  }

  if (e.dataTransfer) {
    e.dataTransfer.effectAllowed = 'move'
    e.dataTransfer.setData('text/plain', String(entry.ID))
    // Create a semi-transparent drag preview image
    const ghost = (el.closest('.course-card') as HTMLElement)?.cloneNode(true) as HTMLElement
    if (ghost) {
      ghost.style.opacity = '0.7'
      ghost.style.position = 'absolute'
      ghost.style.top = '-9999px'
      ghost.style.width = '160px'
      document.body.appendChild(ghost)
      e.dataTransfer.setDragImage(ghost, 80, 20)
      requestAnimationFrame(() => document.body.removeChild(ghost))
    }
  }
}

function onDragEnd() {
  if (dragSourceEl.value) {
    dragSourceEl.value.classList.remove('dragging-source')
    dragSourceEl.value = null
  }
  dragEntry.value = null
  dragOverDay.value = -1
  dragOverPeriod.value = -1
}

function onDragOver(e: DragEvent, day: number, period: number) {
  if (editMode.value) return
  e.preventDefault()
  if (e.dataTransfer) {
    e.dataTransfer.dropEffect = isDropAllowed(day, period) ? 'move' : 'none'
  }
  dragOverDay.value = day
  dragOverPeriod.value = period
}

function isDropAllowed(day: number, period: number): boolean {
  if (!dragEntry.value) return false
  // Block if locked
  if (isDropBlockedByLock(day, period, dragEntry.value.span)) return false
  return true
}

function onDragLeave() {
  dragOverDay.value = -1
  dragOverPeriod.value = -1
}

async function onDrop(e: DragEvent, day: number, period: number) {
  if (editMode.value) return
  e.preventDefault()
  dragOverDay.value = -1
  dragOverPeriod.value = -1

  if (!dragEntry.value) return
  const entry = dragEntry.value

  if (!isDropTarget(day, period)) {
    dragEntry.value = null
    return
  }

  if (entry.dayOfWeek === day && entry.startPeriod === period) {
    dragEntry.value = null
    return
  }

  if (isDropBlockedByLock(day, period, entry.span)) {
    message.error('该时段为全校锁定时间，无法排课')
    dragEntry.value = null
    return
  }

  try {
    const { CheckMove } = await import('../../../bindings/scheduling-system/backend/services/moveservice')
    const result = await CheckMove({
      entryId: entry.ID,
      newDay: day,
      newPeriod: period,
      newSpan: entry.span,
    } as any)

    if (!result?.valid) {
      conflictFlash.value = { day, period }
      const conflictDesc = result?.conflicts?.[0]?.description || '冲突'
      message.error(`无法移动：${conflictDesc}`)
      setTimeout(() => { conflictFlash.value = null }, 1500)
      dragEntry.value = null
      return
    }

    const { MoveEntry } = await import('../../../bindings/scheduling-system/backend/services/moveservice')
    await MoveEntry({
      entryId: entry.ID,
      newDay: day,
      newPeriod: period,
      newSpan: entry.span,
    } as any)

    message.success('课表已调整')
    await scheduleStore.loadSchedule('')
  } catch (err: any) {
    message.error('调整失败：' + (err?.message || err))
  }

  dragEntry.value = null
}

function isDropTarget(day: number, period: number): boolean {
  if (editMode.value) return false
  return true // Allow stacking: multiple courses can share a cell
}

function isDropBlockedByLock(day: number, period: number, span: number): boolean {
  for (let p = period; p < period + span; p++) {
    if (isLocked(day, p)) return true
  }
  return false
}
</script>

<template>
  <div class="week-view">
    <div class="week-toolbar" v-if="displayEntries.length > 0 || editMode">
      <div class="toolbar-left">
        <span class="mode-label" v-if="editMode">🔒 锁定时段编辑模式 — 点击格子锁定/解锁时段</span>
        <span class="mode-label" v-else>💡 拖拽课程卡片即可调整课表位置</span>
      </div>
      <button class="mode-toggle-btn" @click="editMode = !editMode">
        {{ editMode ? '返回查看' : '编辑锁定时段' }}
      </button>
    </div>

    <div v-if="displayEntries.length === 0 && !editMode" class="empty-state">暂无排课数据，请先运行自动排课</div>
    <div v-else class="schedule-grid" :class="{ 'edit-mode': editMode }">
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
          :style="editMode ? {} : cellStyle(di, pi)"
          :class="{
            'cell-locked': isLocked(di, pi),
            'cell-edit-locked': editMode && isLocked(di, pi),
            'cell-edit-free': editMode && !isLocked(di, pi),
            'drag-over': !editMode && dragOverDay === di && dragOverPeriod === pi && isDropAllowed(di, pi),
            'drag-blocked': !editMode && dragOverDay === di && dragOverPeriod === pi && !isDropAllowed(di, pi),
            'conflict-flash': !editMode && conflictFlash?.day === di && conflictFlash?.period === pi,
            'drop-target': !editMode && dragEntry && isDropAllowed(di, pi),
            'has-overflow': !editMode && getCoursesAt(di, pi).length > 1,
          }"
          @click="editMode ? toggleLockCell(di, pi) : togglePopover(di, pi)"
          @dragover="!editMode ? onDragOver($event, di, pi) : undefined"
          @dragleave="!editMode ? onDragLeave : undefined"
          @drop="!editMode ? onDrop($event, di, pi) : undefined"
        >
          <!-- +N badge -->
          <span
            v-if="!editMode && getCoursesAt(di, pi).length > 1 && isFirstCell(getCourseAt(di, pi)!, pi)"
            class="overflow-badge"
            @click.stop="togglePopover(di, pi)"
          >+{{ getCoursesAt(di, pi).length - 1 }}</span>

          <!-- Main course card -->
          <template v-if="!editMode && getCourseAt(di, pi) && isFirstCell(getCourseAt(di, pi)!, pi)">
            <div
              class="course-card"
              :style="courseColorStyle(getCourseAt(di, pi)!.course?.ID ?? 0)"
              draggable="true"
              @dragstart="onDragStart($event, getCourseAt(di, pi)!)"
              @dragend="onDragEnd"
              @click.stop="openCourseDetail(getCourseAt(di, pi)!)"
            >
              <div class="course-name">{{ getCourseAt(di, pi)!.course?.name || '' }}</div>
              <div class="course-detail">{{ getCourseAt(di, pi)!.classroom?.name || '' }} · {{ getCourseAt(di, pi)!.teacher?.name || '' }}</div>
            </div>
          </template>

          <!-- Lock icon in edit mode -->
          <template v-if="editMode && isLocked(di, pi)">
            <span class="lock-icon">🔒</span>
          </template>
        </div>
      </template>
    </div>

    <!-- Popover for overlapping courses -->
    <Teleport to="body">
      <div
        v-if="popoverCell && !editMode"
        class="overflow-popover-backdrop"
        @click="popoverCell = null"
      >
        <div class="overflow-popover" @click.stop>
          <div class="overflow-popover-title">
            {{ DAY_NAMES[popoverCell.day] }} 第{{ popoverCell.period + 1 }}-{{ popoverCell.period + 3 }}节（{{ getCoursesAt(popoverCell.day, popoverCell.period).length }}门课）
          </div>
          <div
            v-for="e in getCoursesAt(popoverCell.day, popoverCell.period)"
            :key="e.ID"
            class="overflow-item"
            :style="courseColorStyle(e.course?.ID ?? 0)"
            draggable="true"
            @dragstart="onDragStart($event, e)"
            @click="openCourseDetail(e); popoverCell = null"
          >
            <div class="oi-name">{{ e.course?.name }}</div>
            <div class="oi-meta">{{ e.teacher?.name }} · {{ e.classroom?.name }}</div>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<style scoped>
.week-view { flex: 1; display: flex; flex-direction: column; min-height: 0; }

.week-toolbar {
  display: flex; align-items: center; justify-content: space-between;
  margin-bottom: 8px; padding: 6px 12px;
  background: var(--b3-theme-surface); border: 1px solid var(--b3-border-color);
  border-radius: var(--b3-border-radius-s);
}
.toolbar-left { display: flex; align-items: center; gap: 8px; }
.mode-label { font-size: 13px; font-weight: 500; color: var(--b3-theme-on-surface); }
.mode-toggle-btn {
  font-size: 12px; padding: 4px 12px; border: 1px solid var(--b3-theme-primary);
  background: var(--b3-theme-primary-lightest); color: var(--b3-theme-primary);
  border-radius: 4px; cursor: pointer; font-weight: 500;
}
.mode-toggle-btn:hover { background: var(--b3-theme-primary-light); }

.empty-state { display: flex; align-items: center; justify-content: center; flex: 1; color: var(--b3-theme-on-surface-light); font-size: 14px; }

.schedule-grid { flex: 1; display: grid; grid-template-columns: 60px repeat(7, 1fr); grid-template-rows: auto repeat(11, minmax(48px, 1fr)); gap: 1px; background: var(--b3-border-color); border: 1px solid var(--b3-border-color); border-radius: var(--b3-border-radius); overflow: hidden; }

.grid-corner, .grid-header, .time-label { background: var(--b3-theme-surface); display: flex; align-items: center; justify-content: center; font-size: 12px; font-weight: 500; color: var(--b3-theme-on-surface); }
.grid-header { flex-direction: column; gap: 1px; }
.grid-header.today { background: var(--b3-theme-primary-lightest); color: var(--b3-theme-primary); }
.day-name { font-size: 12px; }
.day-date { font-size: 10px; opacity: 0.7; }
.time-label { flex-direction: column; gap: 1px; font-size: 11px; }
.period-num { font-weight: 600; color: var(--b3-theme-on-background); }
.period-time { font-size: 9px; color: var(--b3-theme-on-surface-light); }

.grid-cell { background: var(--b3-theme-background); min-height: 48px; overflow: hidden; position: relative; transition: background 0.15s; }

.edit-mode .grid-cell { cursor: pointer; display: flex; align-items: center; justify-content: center; }
.cell-locked:not(.cell-edit-locked) { background: repeating-linear-gradient(135deg, var(--b3-theme-surface), var(--b3-theme-surface) 3px, transparent 3px, transparent 6px); cursor: not-allowed; }
.cell-edit-locked { background: rgba(244, 67, 54, 0.25) !important; }
.cell-edit-free { background: var(--b3-theme-background); }
.cell-edit-free:hover { background: var(--b3-theme-primary-lightest); }
.lock-icon { font-size: 14px; opacity: 0.7; pointer-events: none; }

.grid-cell.drop-target { background: var(--b3-theme-primary-lightest); opacity: 0.85; transition: background 0.12s; }
.grid-cell.drag-over { background: var(--b3-theme-primary-light); outline: 2px dashed var(--b3-theme-primary); outline-offset: -2px; z-index: 1; box-shadow: inset 0 0 12px rgba(24, 160, 88, 0.15); }
.grid-cell.drag-blocked { background: rgba(244, 67, 54, 0.12) !important; outline: 2px dashed #f44336; outline-offset: -2px; z-index: 1; cursor: not-allowed; }
.grid-cell.conflict-flash { animation: conflictPulse 0.3s ease 3; background: var(--b3-theme-error-lightest) !important; }
@keyframes conflictPulse {
  0%, 100% { background: var(--b3-theme-error-lightest); }
  50% { background: var(--b3-theme-error); }
}

/* +N badge */
.overflow-badge {
  position: absolute; top: 2px; right: 2px; z-index: 2;
  background: #d03050; color: #fff;
  font-size: 10px; font-weight: 700; padding: 1px 5px;
  border-radius: 8px; cursor: pointer; line-height: 1.4;
}

/* Popover */
.overflow-popover-backdrop {
  position: fixed; inset: 0; z-index: 9999;
  background: rgba(0,0,0,0.2); display: flex;
  align-items: center; justify-content: center;
}
.overflow-popover {
  background: var(--b3-theme-surface); border: 1px solid var(--b3-border-color);
  border-radius: 8px; padding: 16px; min-width: 280px; max-width: 360px;
  box-shadow: 0 8px 30px rgba(0,0,0,0.2);
}
.overflow-popover-title {
  font-size: 13px; font-weight: 600; color: var(--b3-theme-on-background);
  margin-bottom: 12px; padding-bottom: 8px;
  border-bottom: 1px solid var(--b3-border-color);
}
.overflow-item {
  padding: 8px 10px; border-radius: 4px; border-left: 3px solid;
  cursor: grab; margin-bottom: 6px; font-size: 12px;
}
.overflow-item:last-child { margin-bottom: 0; }
.overflow-item:active { cursor: grabbing; }
.overflow-item:hover { filter: brightness(0.95); }
.overflow-item.dragging-source { opacity: 0.35; }
.oi-name { font-weight: 600; color: var(--b3-theme-on-background); }
.oi-meta { font-size: 11px; color: var(--b3-theme-on-surface-light); margin-top: 2px; }

.course-card { height: 100%; padding: 4px 6px; font-size: 11px; cursor: grab; transition: box-shadow 0.15s, opacity 0.15s; border-left: 3px solid; overflow: hidden; display: flex; flex-direction: column; user-select: none; }
.course-card:active { cursor: grabbing; }
.course-card:hover { box-shadow: var(--b3-point-shadow); }
.course-card.dragging-source { opacity: 0.35; box-shadow: none; }
.course-name { font-weight: 600; color: var(--b3-theme-on-background); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.course-detail { font-size: 10px; color: var(--b3-theme-on-surface-light); margin-top: 1px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }

</style>
