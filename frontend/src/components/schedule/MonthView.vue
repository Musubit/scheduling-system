<script setup lang="ts">
import { computed } from 'vue'
import { useScheduleStore } from '../../stores/schedule'
import type { DeptCode } from '../../types'

const scheduleStore = useScheduleStore()

const dayNames = ['一', '二', '三', '四', '五', '六', '日']

// Mock courses for month view display
interface MockCourse {
  day: number
  name: string
  dept: DeptCode
}
const mockCourses: MockCourse[] = [
  { day: 0, name: '高等数学', dept: 'math' },
  { day: 0, name: '数据结构', dept: 'cs' },
  { day: 0, name: '大学英语', dept: 'eng' },
  { day: 1, name: '线性代数', dept: 'math' },
  { day: 1, name: '计组', dept: 'cs' },
  { day: 2, name: '操作系统', dept: 'cs' },
  { day: 2, name: '大学物理', dept: 'phys' },
  { day: 3, name: '算法设计', dept: 'cs' },
  { day: 3, name: '离散数学', dept: 'math' },
  { day: 4, name: '编译原理', dept: 'cs' },
  { day: 4, name: '英语听说', dept: 'eng' },
]

// Calendar calculations
const year = computed(() => scheduleStore.currentYear)
const month = computed(() => scheduleStore.currentMonth)

const daysInMonth = computed(() => new Date(year.value, month.value, 0).getDate())
const firstDayOfWeek = computed(() => {
  // getDay() returns 0=Sun, we want 0=Mon
  const d = new Date(year.value, month.value - 1, 1).getDay()
  return d === 0 ? 6 : d - 1
})

const today = new Date().getDate()

function getCoursesForDayOfWeek(dow: number): MockCourse[] {
  return mockCourses.filter(c => c.day === dow).slice(0, 3)
}

// Generate calendar days array
const calendarDays = computed(() => {
  const days: { date: number; isCurrentMonth: boolean; isToday: boolean; dow: number }[] = []
  const prevMonthDays = new Date(year.value, month.value - 1, 0).getDate()

  // Previous month padding
  for (let i = firstDayOfWeek.value - 1; i >= 0; i--) {
    days.push({ date: prevMonthDays - i, isCurrentMonth: false, isToday: false, dow: -1 })
  }

  // Current month
  for (let d = 1; d <= daysInMonth.value; d++) {
    const dow = (firstDayOfWeek.value + d - 1) % 7
    const dt = new Date(year.value, month.value - 1, d)
    days.push({
      date: d,
      isCurrentMonth: true,
      isToday: dt.toDateString() === new Date().toDateString(),
      dow,
    })
  }

  // Next month padding
  const remaining = 7 - (days.length % 7)
  if (remaining < 7) {
    for (let i = 1; i <= remaining; i++) {
      days.push({ date: i, isCurrentMonth: false, isToday: false, dow: -1 })
    }
  }

  return days
})
</script>

<template>
  <div class="month-view">
    <div class="month-grid">
      <div v-for="d in dayNames" :key="d" class="month-header-cell">周{{ d }}</div>
      <div
        v-for="(day, idx) in calendarDays"
        :key="idx"
        class="month-cell"
        :class="{
          'other-month': !day.isCurrentMonth,
          'is-today': day.isToday,
        }"
      >
        <div class="date-num">{{ day.date }}</div>
        <div v-if="day.isCurrentMonth" class="month-events">
          <div
            v-for="(course, ci) in getCoursesForDayOfWeek(day.dow)"
            :key="ci"
            class="month-event"
            :class="'mev-' + course.dept"
          >
            {{ course.name }}
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.month-view {
  flex: 1;
  overflow-y: auto;
  min-height: 0;
}

.month-grid {
  display: grid;
  grid-template-columns: repeat(7, 1fr);
  gap: 1px;
  background: var(--b3-border-color);
  border: 1px solid var(--b3-border-color);
  border-radius: var(--b3-border-radius);
  overflow: hidden;
  min-height: 100%;
}

.month-header-cell {
  background: var(--b3-theme-surface);
  padding: 8px;
  text-align: center;
  font-size: 12px;
  font-weight: 500;
  color: var(--b3-theme-on-surface);
}

.month-cell {
  background: var(--b3-theme-background);
  min-height: 80px;
  padding: 4px 6px;
}

.month-cell.other-month {
  background: var(--b3-theme-surface);
  opacity: 0.4;
}

.month-cell.is-today {
  background: var(--b3-theme-primary-lightest);
}

.month-cell.is-today .date-num {
  color: var(--b3-theme-primary);
  font-weight: 700;
}

.date-num {
  font-size: 12px;
  color: var(--b3-theme-on-surface);
  margin-bottom: 4px;
}

.month-events {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.month-event {
  font-size: 10px;
  padding: 1px 4px;
  border-radius: 2px;
  border-left: 2px solid;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  line-height: 1.4;
}

.mev-cs { background: var(--course-cs-bg); color: var(--course-cs-border); border-left-color: var(--course-cs-border); }
.mev-math { background: var(--course-math-bg); color: var(--course-math-border); border-left-color: var(--course-math-border); }
.mev-phys { background: var(--course-phys-bg); color: var(--course-phys-border); border-left-color: var(--course-phys-border); }
.mev-eng { background: var(--course-eng-bg); color: var(--course-eng-border); border-left-color: var(--course-eng-border); }
.mev-art { background: var(--course-art-bg); color: var(--course-art-border); border-left-color: var(--course-art-border); }
.mev-eco { background: var(--course-eco-bg); color: var(--course-eco-border); border-left-color: var(--course-eco-border); }
.mev-law { background: var(--course-law-bg); color: var(--course-law-border); border-left-color: var(--course-law-border); }
.mev-edu { background: var(--course-edu-bg); color: var(--course-edu-border); border-left-color: var(--course-edu-border); }
</style>
