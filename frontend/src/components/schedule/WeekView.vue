<script setup lang="ts">
import { inject, computed } from 'vue'
import { PERIODS, DAY_NAMES } from '../../types'
import type { DeptCode, ScheduleEntry } from '../../types'
import { useScheduleStore } from '../../stores/schedule'

const scheduleStore = useScheduleStore()
const drawerRef = inject<any>('drawerRef')

// Use store entries if available, otherwise fall back to mock data
const displayEntries = computed(() => {
  if (scheduleStore.entries.length > 0) return scheduleStore.entries
  return mockEntriesFromCourses()
})

function mockEntriesFromCourses(): ScheduleEntry[] {
  return mockCourses.map(c => ({
    id: 0,
    courseId: 0, teacherId: 0, classroomId: 0,
    semester: '',
    dayOfWeek: c.day,
    startPeriod: c.period,
    span: c.span,
    weeks: '1-16',
    course: { id: 0, code: '', name: c.name, dept: c.dept, credit: 0, type: '', hours: 0 },
    teacher: { id: 0, code: '', name: c.teacher, dept: '', title: '', status: 'active' },
    classroom: { id: 0, code: '', name: c.room, building: '', capacity: 0, type: '', status: '' },
  }))
}

// 打开课程详情抽屉
function getCourseAt(day: number, period: number): ScheduleEntry | undefined {
  return displayEntries.value.find(e => e.dayOfWeek === day && e.startPeriod === period)
}

function openCourseDetail(entry: ScheduleEntry) {
  if (!drawerRef?.value) return
  drawerRef.value.openDrawer(entry)
}

// 模拟数据（阶段3替换为真实API数据）
interface MockCourse {
  day: number
  period: number
  span: number
  name: string
  teacher: string
  room: string
  dept: DeptCode
  classes: string
  conflict?: boolean
}

const mockCourses: MockCourse[] = [
  { day: 0, period: 0, span: 2, name: '高等数学', teacher: '王建国', room: 'A301', dept: 'math', classes: '数学2301' },
  { day: 0, period: 2, span: 2, name: '数据结构', teacher: '张明远', room: 'C502', dept: 'cs', classes: '计算机2301' },
  { day: 0, period: 4, span: 2, name: '大学英语', teacher: '刘芳', room: 'B108', dept: 'eng', classes: '多专业合班' },
  { day: 0, period: 8, span: 2, name: '体育(篮球)', teacher: '陈刚', room: '体育馆', dept: 'edu', classes: '体育选修' },
  { day: 4, period: 8, span: 3, name: '晚课实验(三连上)', teacher: '周海', room: 'C502', dept: 'cs', classes: '计算机2301' },
  { day: 1, period: 0, span: 2, name: '线性代数', teacher: '王建国', room: 'B205', dept: 'math', classes: '计算机2301' },
  { day: 1, period: 2, span: 2, name: '计算机组成原理', teacher: '李伟', room: 'A301', dept: 'cs', classes: '计算机2301', conflict: true },
  { day: 1, period: 5, span: 2, name: '概率论', teacher: '赵秀英', room: 'B301', dept: 'math', classes: '经济2301' },
  { day: 2, period: 0, span: 2, name: '操作系统', teacher: '周海', room: 'C502', dept: 'cs', classes: '计算机2302' },
  { day: 2, period: 2, span: 2, name: '大学物理', teacher: '钱学森', room: 'A201', dept: 'phys', classes: '物理2301' },
  { day: 2, period: 4, span: 2, name: '马克思主义基本原理', teacher: '吴芳', room: 'D401', dept: 'law', classes: '多专业合班' },
  { day: 3, period: 0, span: 2, name: '算法设计', teacher: '张明远', room: 'C301', dept: 'cs', classes: '计算机2301' },
  { day: 3, period: 2, span: 2, name: '离散数学', teacher: '赵秀英', room: 'B205', dept: 'math', classes: '计算机2301' },
  { day: 4, period: 0, span: 2, name: '编译原理', teacher: '李伟', room: 'C301', dept: 'cs', classes: '计算机2301' },
  { day: 4, period: 2, span: 2, name: '英语听说', teacher: '刘芳', room: 'B108', dept: 'eng', classes: '计算机2301' },
  { day: 5, period: 2, span: 2, name: '数学建模', teacher: '钱学森', room: 'A201', dept: 'math', classes: '数学2302' },
]

</script>

<template>
  <div class="week-view">
    <div class="schedule-grid">
      <!-- Header row -->
      <div class="grid-corner">节次</div>
      <div
        v-for="(name, di) in DAY_NAMES"
        :key="di"
        class="grid-header"
        :class="{ today: di === 1 }"
      >
        <span class="day-name">{{ name }}</span>
        <span class="day-date">3/{{ 24 + di }}</span>
      </div>

      <!-- Grid cells -->
      <template v-for="(period, pi) in PERIODS" :key="pi">
        <div class="time-label">
          <span class="period-num">{{ period.num }}</span>
          <span class="period-time">{{ period.time.replace('\n', ' ') }}</span>
        </div>
        <div
          v-for="(_, di) in DAY_NAMES"
          :key="di"
          class="grid-cell"
        >
          <template v-if="getCourseAt(di, pi)">
            <div
              class="course-card"
              :class="[
                'course-' + (getCourseAt(di, pi)!.course?.dept || 'cs'),
                { 'course-conflict': false }
              ]"
              :style="{ gridRow: 'span ' + getCourseAt(di, pi)!.span }"
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
.week-view {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.schedule-grid {
  flex: 1;
  display: grid;
  grid-template-columns: 60px repeat(7, 1fr);
  grid-template-rows: auto repeat(11, minmax(36px, 1fr));
  gap: 1px;
  background: var(--b3-border-color);
  border: 1px solid var(--b3-border-color);
  border-radius: var(--b3-border-radius);
  overflow: hidden;
}

.grid-corner,
.grid-header,
.time-label {
  background: var(--b3-theme-surface);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
  font-weight: 500;
  color: var(--b3-theme-on-surface);
}

.grid-header {
  flex-direction: column;
  gap: 1px;
}

.grid-header.today {
  background: var(--b3-theme-primary-lightest);
  color: var(--b3-theme-primary);
}

.day-name { font-size: 12px; }
.day-date { font-size: 10px; opacity: 0.7; }

.time-label {
  flex-direction: column;
  gap: 1px;
  font-size: 11px;
}

.period-num { font-weight: 600; color: var(--b3-theme-on-background); }
.period-time { font-size: 9px; color: var(--b3-theme-on-surface-light); }

.grid-cell {
  background: var(--b3-theme-background);
  min-height: 48px;
  overflow: hidden;
}

.course-card {
  height: 100%;
  padding: 4px 6px;
  font-size: 11px;
  cursor: pointer;
  transition: box-shadow 0.15s;
  border-left: 3px solid;
  overflow: hidden;
}

.course-card:hover {
  box-shadow: var(--b3-point-shadow);
}

.course-name {
  font-weight: 600;
  color: var(--b3-theme-on-background);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.course-detail {
  font-size: 10px;
  color: var(--b3-theme-on-surface-light);
  margin-top: 1px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.course-conflict {
  outline: 2px solid var(--b3-theme-error);
  outline-offset: -2px;
}

/* Department colors */
.course-cs { background: var(--course-cs-bg); border-left-color: var(--course-cs-border); }
.course-math { background: var(--course-math-bg); border-left-color: var(--course-math-border); }
.course-phys { background: var(--course-phys-bg); border-left-color: var(--course-phys-border); }
.course-eng { background: var(--course-eng-bg); border-left-color: var(--course-eng-border); }
.course-art { background: var(--course-art-bg); border-left-color: var(--course-art-border); }
.course-eco { background: var(--course-eco-bg); border-left-color: var(--course-eco-border); }
.course-law { background: var(--course-law-bg); border-left-color: var(--course-law-border); }
.course-edu { background: var(--course-edu-bg); border-left-color: var(--course-edu-border); }
</style>
