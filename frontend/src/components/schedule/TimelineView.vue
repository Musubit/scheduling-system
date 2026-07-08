<script setup lang="ts">
import { computed } from 'vue'
import { DAY_NAMES } from '../../types'
import type { ScheduleEntry } from '../../types'
import { useScheduleStore } from '../../stores/schedule'

const scheduleStore = useScheduleStore()

const hours = ['08:00', '09:00', '10:00', '11:00', '12:00', '13:00', '14:00', '15:00', '16:00', '17:00', '18:00', '19:00', '20:00', '21:00']

function periodToMinute(p: number): number {
  // Map period index to minutes from 8:00
  const map: Record<number, number> = {
    0: 20, 1: 70, 2: 135, 3: 185,
    4: 360, 5: 410, 6: 475, 7: 525,
    8: 630, 9: 680, 10: 730,
  }
  return map[p] ?? (p * 50)
}

function getCoursesForDay(day: number): Array<ScheduleEntry & { lane: number; totalLanes: number }> {
	  const entries = scheduleStore.displayEntries.filter(e => e.dayOfWeek === day)
  if (entries.length <= 1) return entries.map(e => ({ ...e, lane: 0, totalLanes: 1 }))

  // Sort by start time
  const sorted = [...entries].sort((a, b) => a.startPeriod - b.startPeriod)

  // Lane assignment: detect overlapping courses, assign different lanes
  const lanes: number[] = [] // lane -> endPeriod
  const result: Array<ScheduleEntry & { lane: number; totalLanes: number }> = []

  for (const e of sorted) {
    let assignedLane = -1
    for (let l = 0; l < lanes.length; l++) {
      if (lanes[l] <= e.startPeriod) {
        assignedLane = l
        lanes[l] = e.startPeriod + e.span
        break
      }
    }
    if (assignedLane < 0) {
      assignedLane = lanes.length
      lanes.push(e.startPeriod + e.span)
    }
    result.push({ ...e, lane: assignedLane, totalLanes: 0 })
  }

  const totalLanes = lanes.length
  for (const r of result) {
    (r as any).totalLanes = totalLanes
  }

  return result
}

function eventStyle(e: ScheduleEntry, lane: number, totalLanes: number) {
  const startMin = periodToMinute(e.startPeriod)
  const endMin = periodToMinute(e.startPeriod + e.span)
  const totalMin = 14 * 60 // 8:00 - 22:00 = 14 hours

  const left = (startMin / totalMin * 100)
  const width = ((endMin - startMin) / totalMin * 100)

  const laneHeight = 100 / totalLanes
  const top = lane * laneHeight

  return {
    left: left + '%',
    width: width + '%',
    top: top + '%',
    height: (laneHeight - 2) + '%',
  }
}
</script>

<template>
  <div class="timeline-view">
    <div v-if="scheduleStore.displayEntries.length === 0" class="empty-state">暂无排课数据</div>
    <template v-else>
      <div class="tl-header">
        <div class="tl-day-label"></div>
        <div class="tl-hours">
          <div v-for="h in hours" :key="h" class="tl-hour">{{ h }}</div>
        </div>
      </div>
      <div v-for="(day, di) in DAY_NAMES" :key="di" class="tl-row">
        <div class="tl-day-name">{{ day }}</div>
        <div class="tl-track">
          <div
            v-for="(e, ei) in getCoursesForDay(di)"
            :key="ei"
            class="tl-event"
            :class="'tl-' + (e.course?.dept || 'cs')"
            :style="eventStyle(e, e.lane, e.totalLanes)"
            :title="`${e.course?.name} · ${e.classroom?.name} · ${e.teacher?.name}`"
          >
            <span class="tl-event-name">{{ e.course?.name }}</span>
            <span class="tl-event-room">{{ e.classroom?.name }}</span>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.timeline-view { flex: 1; display: flex; flex-direction: column; min-height: 0; gap: 2px; }
.empty-state { display: flex; align-items: center; justify-content: center; height: 200px; color: var(--b3-theme-on-surface-light); font-size: 14px; }
.tl-header { display: flex; margin-bottom: 4px; flex-shrink: 0; }
.tl-day-label { width: 48px; flex-shrink: 0; }
.tl-hours { display: grid; grid-template-columns: repeat(14, 1fr); flex: 1; }
.tl-hour { font-size: 10px; color: var(--b3-theme-on-surface-light); text-align: center; }
.tl-row { display: flex; align-items: stretch; flex: 1; min-height: 0; }
.tl-day-name { width: 48px; flex-shrink: 0; font-size: 12px; font-weight: 500; color: var(--b3-theme-on-surface); display: flex; align-items: center; padding: 0 4px; }
.tl-track { flex: 1; position: relative; background: var(--b3-theme-background); border: 1px solid var(--b3-border-color); border-radius: var(--b3-border-radius-s); min-height: 0; }
.tl-event { position: absolute; border-radius: 3px; border-left: 3px solid; padding: 2px 6px; display: flex; align-items: center; gap: 6px; font-size: 11px; overflow: hidden; white-space: nowrap; min-width: 0; }
.tl-event-name { font-weight: 600; color: var(--b3-theme-on-background); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; flex-shrink: 1; min-width: 0; }
.tl-event-room { opacity: 0.6; color: var(--b3-theme-on-surface); font-size: 10px; flex-shrink: 0; }
</style>
