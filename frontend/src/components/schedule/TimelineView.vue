<script setup lang="ts">
import { computed } from 'vue'
import { DAY_NAMES } from '../../types'
import { useScheduleStore } from '../../stores/schedule'

const scheduleStore = useScheduleStore()

const hours = ['08:00', '09:00', '10:00', '11:00', '12:00', '13:00', '14:00', '15:00', '16:00', '17:00', '18:00', '19:00', '20:00', '21:00']

function periodToHour(p: number): number {
  const map: Record<number, number> = {
    0: 8.33, 1: 9.17, 2: 10.25, 3: 11.08,
    4: 14, 5: 14.83, 6: 15.92, 7: 16.75,
    8: 18.5, 9: 19.33, 10: 20.17,
  }
  return map[p] ?? 8 + p
}

function getCoursesForDay(day: number) {
  return scheduleStore.entries.filter(e => e.dayOfWeek === day)
}
</script>

<template>
  <div class="timeline-view">
    <div v-if="scheduleStore.entries.length === 0" class="empty-state">暂无排课数据</div>
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
            :style="{
              left: ((periodToHour(e.startPeriod) - 8) / 14 * 100) + '%',
              width: (e.span / 7 * 100) + '%',
            }"
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
.timeline-view { flex: 1; display: flex; flex-direction: column; overflow-y: auto; min-height: 0; gap: 2px; }
.empty-state { display: flex; align-items: center; justify-content: center; height: 200px; color: var(--b3-theme-on-surface-light); font-size: 14px; }
.tl-header { display: flex; margin-bottom: 4px; flex-shrink: 0; }
.tl-day-label { width: 48px; flex-shrink: 0; }
.tl-hours { display: grid; grid-template-columns: repeat(14, 1fr); flex: 1; }
.tl-hour { font-size: 10px; color: var(--b3-theme-on-surface-light); text-align: center; }
.tl-row { display: flex; align-items: stretch; min-height: 36px; }
.tl-day-name { width: 48px; flex-shrink: 0; font-size: 12px; font-weight: 500; color: var(--b3-theme-on-surface); display: flex; align-items: center; padding: 0 4px; }
.tl-track { flex: 1; position: relative; background: var(--b3-theme-background); border: 1px solid var(--b3-border-color); border-radius: var(--b3-border-radius-s); min-height: 36px; }
.tl-event { position: absolute; top: 2px; bottom: 2px; border-radius: 3px; border-left: 3px solid; padding: 2px 6px; display: flex; align-items: center; gap: 6px; font-size: 11px; overflow: hidden; white-space: nowrap; }
.tl-event-name { font-weight: 600; color: var(--b3-theme-on-background); }
.tl-event-room { opacity: 0.6; color: var(--b3-theme-on-surface); font-size: 10px; }
.tl-mech { background: var(--course-mech-bg); border-left-color: var(--course-mech-border); }
.tl-elec { background: var(--course-elec-bg); border-left-color: var(--course-elec-border); }
.tl-mate { background: var(--course-mate-bg); border-left-color: var(--course-mate-border); }
.tl-bio { background: var(--course-bio-bg); border-left-color: var(--course-bio-border); }
.tl-civil { background: var(--course-civil-bg); border-left-color: var(--course-civil-border); }
.tl-cs { background: var(--course-cs-bg); border-left-color: var(--course-cs-border); }
.tl-art { background: var(--course-art-bg); border-left-color: var(--course-art-border); }
.tl-design { background: var(--course-design-bg); border-left-color: var(--course-design-border); }
.tl-econ { background: var(--course-econ-bg); border-left-color: var(--course-econ-border); }
.tl-eng { background: var(--course-eng-bg); border-left-color: var(--course-eng-border); }
.tl-sci { background: var(--course-sci-bg); border-left-color: var(--course-sci-border); }
.tl-marx { background: var(--course-marx-bg); border-left-color: var(--course-marx-border); }
.tl-voc { background: var(--course-voc-bg); border-left-color: var(--course-voc-border); }
.tl-intl { background: var(--course-intl-bg); border-left-color: var(--course-intl-border); }
.tl-pe { background: var(--course-pe-bg); border-left-color: var(--course-pe-border); }
.tl-cont { background: var(--course-cont-bg); border-left-color: var(--course-cont-border); }
.tl-innov { background: var(--course-innov-bg); border-left-color: var(--course-innov-border); }
.tl-engtech { background: var(--course-engtech-bg); border-left-color: var(--course-engtech-border); }
.tl-detroit { background: var(--course-detroit-bg); border-left-color: var(--course-detroit-border); }
</style>
