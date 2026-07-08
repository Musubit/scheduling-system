<script setup lang="ts">
import { DAY_NAMES } from '../../types'
import type { DeptCode } from '../../types'

interface MockCourse {
  day: number
  period: number
  span: number
  name: string
  teacher: string
  room: string
  dept: DeptCode
}

// Shared mock data matching WeekView
const mockCourses: MockCourse[] = [
  { day: 0, period: 0, span: 2, name: '高等数学', teacher: '赵秀英', room: 'A301', dept: 'sci' },
  { day: 0, period: 2, span: 2, name: '数据结构', teacher: '周海', room: 'C502', dept: 'cs' },
  { day: 0, period: 4, span: 2, name: '大学英语', teacher: '刘芳', room: 'B108', dept: 'eng' },
  { day: 0, period: 8, span: 2, name: '体育(篮球)', teacher: '陈刚', room: '体育馆', dept: 'pe' },
  { day: 1, period: 0, span: 2, name: '线性代数', teacher: '赵秀英', room: 'B205', dept: 'sci' },
  { day: 1, period: 2, span: 2, name: '电路原理', teacher: '李明远', room: 'A201', dept: 'elec' },
  { day: 1, period: 4, span: 2, name: '大学物理', teacher: '钱学林', room: 'C301', dept: 'sci' },
  { day: 2, period: 0, span: 2, name: '操作系统', teacher: '周海', room: 'C502', dept: 'cs' },
  { day: 2, period: 2, span: 2, name: '生物化学', teacher: '钱学林', room: 'C301', dept: 'bio' },
  { day: 2, period: 4, span: 2, name: '马克思主义基本原理', teacher: '吴芳', room: 'D401', dept: 'marx' },
  { day: 3, period: 0, span: 2, name: '机械设计基础', teacher: '张建国', room: 'B301', dept: 'mech' },
  { day: 3, period: 2, span: 2, name: '结构力学', teacher: '杨华', room: 'C301', dept: 'civil' },
  { day: 4, period: 0, span: 2, name: '电力系统分析', teacher: '李明远', room: 'A201', dept: 'elec' },
  { day: 4, period: 8, span: 3, name: '晚课实验(三连上)', teacher: '周海', room: 'C502', dept: 'cs' },
]

const hours = ['08:00', '09:00', '10:00', '11:00', '12:00', '13:00', '14:00', '15:00', '16:00', '17:00', '18:00', '19:00', '20:00', '21:00']

// Map period index → start hour for timeline positioning
function periodToHour(p: number): number {
  // Period 0 → 08:00, period 1 → 09:00 etc, but with gaps for breaks
  const map: Record<number, number> = {
    0: 8.33, 1: 9.17,   // 1-2节
    2: 10.25, 3: 11.08, // 3-4节
    4: 14, 5: 14.83,    // 5-6节
    6: 15.92, 7: 16.75, // 7-8节
    8: 18.5, 9: 19.33,  // 9-10节
    10: 20.17,           // 11节
  }
  return map[p] ?? 8 + p
}

function getCoursesForDay(day: number): MockCourse[] {
  return mockCourses.filter(c => c.day === day)
}
</script>

<template>
  <div class="timeline-view">
    <!-- Hour header -->
    <div class="tl-header">
      <div class="tl-day-label"></div>
      <div class="tl-hours">
        <div v-for="h in hours" :key="h" class="tl-hour">{{ h }}</div>
      </div>
    </div>

    <!-- Day rows -->
    <div v-for="(day, di) in DAY_NAMES" :key="di" class="tl-row">
      <div class="tl-day-name">{{ day }}</div>
      <div class="tl-track">
        <div
          v-for="(course, ci) in getCoursesForDay(di)"
          :key="ci"
          class="tl-event"
          :class="'tl-' + course.dept"
          :style="{
            left: ((periodToHour(course.period) - 8) / 14 * 100) + '%',
            width: (course.span / 14 * 100) + '%',
          }"
        >
          <span class="tl-event-name">{{ course.name }}</span>
          <span class="tl-event-room">{{ course.room }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.timeline-view {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow-y: auto;
  min-height: 0;
  gap: 2px;
}

.tl-header {
  display: flex;
  margin-bottom: 4px;
  flex-shrink: 0;
}

.tl-day-label {
  width: 48px;
  flex-shrink: 0;
}

.tl-hours {
  display: grid;
  grid-template-columns: repeat(14, 1fr);
  flex: 1;
}

.tl-hour {
  font-size: 10px;
  color: var(--b3-theme-on-surface-light);
  text-align: center;
}

.tl-row {
  display: flex;
  align-items: stretch;
  min-height: 36px;
}

.tl-day-name {
  width: 48px;
  flex-shrink: 0;
  font-size: 12px;
  font-weight: 500;
  color: var(--b3-theme-on-surface);
  display: flex;
  align-items: center;
  padding: 0 4px;
}

.tl-track {
  flex: 1;
  position: relative;
  background: var(--b3-theme-background);
  border: 1px solid var(--b3-border-color);
  border-radius: var(--b3-border-radius-s);
  min-height: 36px;
}

.tl-event {
  position: absolute;
  top: 2px;
  bottom: 2px;
  border-radius: 3px;
  border-left: 3px solid;
  padding: 2px 6px;
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  overflow: hidden;
  white-space: nowrap;
}

.tl-event-name {
  font-weight: 600;
  color: var(--b3-theme-on-background);
}

.tl-event-room {
  opacity: 0.6;
  color: var(--b3-theme-on-surface);
  font-size: 10px;
}

/* Department colors */
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
