// ===== 核心数据类型 =====
// Note: field names match Go GORM binding conventions (ID uppercase)

/** 课程 */
export interface Course {
  ID: number
  code: string
  name: string
  dept: DeptCode
  credit: number
  type: string
  hours: number
  status?: 'active' | 'inactive'
}

/** 教师 */
export interface Teacher {
  ID: number
  code: string
  name: string
  dept: string
  status: 'active' | 'inactive'
  weeklyHours?: number
  preferNoEarly?: boolean
  preferNoLate?: boolean
  maxDaysPerWeek?: number
  preferLowFloor?: boolean
}

/** 教室 */
export interface Classroom {
  ID: number
  code: string
  name: string
  building: string
  floor?: number
  capacity: number
  type: string
  status: string
}

/** 班级 */
export interface ClassGroup {
  ID: number
  code: string
  name: string
  dept: string
  grade: number
  students: number
  status?: 'active' | 'inactive'
}

/** 学期 */
export interface Semester {
  ID: number
  name: string
  isActive: boolean
}

/** 排课条目 */
export interface ScheduleEntry {
  ID: number
  courseId: number
  teacherId: number
  classroomId: number
  classGroupId?: number
  semester: string
  dayOfWeek: number
  startPeriod: number
  span: number
  weeks: string
  course?: Course
  teacher?: Teacher
  classroom?: Classroom
  classGroup?: ClassGroup
}

/** 院系代码 */
export type DeptCode = 'mech' | 'elec' | 'mate' | 'bio' | 'civil' | 'cs' | 'art' | 'design' | 'econ' | 'eng' | 'sci' | 'marx' | 'voc' | 'intl' | 'pe' | 'cont' | 'innov' | 'engtech' | 'detroit'

/** 院系信息 */
export interface Department {
  code: DeptCode
  name: string
}

/** 院系列表 - 湖北工业大学19个学院 */
export const DEPARTMENTS: Department[] = [
  { code: 'mech', name: '机械工程学院' },
  { code: 'elec', name: '电气与电子工程学院' },
  { code: 'mate', name: '材料与化学工程学院' },
  { code: 'bio', name: '生物工程与食品学院' },
  { code: 'civil', name: '土木建筑与环境学院' },
  { code: 'cs', name: '计算机学院' },
  { code: 'art', name: '艺术设计学院' },
  { code: 'design', name: '工业设计学院' },
  { code: 'econ', name: '经济与管理学院' },
  { code: 'eng', name: '外国语学院' },
  { code: 'sci', name: '理学院' },
  { code: 'marx', name: '马克思主义学院' },
  { code: 'voc', name: '职业技术师范学院' },
  { code: 'intl', name: '国际学院' },
  { code: 'pe', name: '体育学院' },
  { code: 'cont', name: '继续教育学院' },
  { code: 'innov', name: '创新创业学院' },
  { code: 'engtech', name: '工程技术学院' },
  { code: 'detroit', name: '底特律绿色工业学院' },
]

/** Code→Chinese name lookup */
export const DEPT_NAME_MAP: Record<string, string> = {}
DEPARTMENTS.forEach(d => { DEPT_NAME_MAP[d.code] = d.name })

/** Chinese name→code lookup */
export const DEPT_CODE_MAP: Record<string, string> = {}
DEPARTMENTS.forEach(d => { DEPT_CODE_MAP[d.name] = d.code })

/** 时间段 */
export interface Period {
  num: number
  time: string
}

/** 11节课的时间段定义（高校标准作息） */
export const PERIODS: Period[] = [
  { num: 1,  time: '08:20\n09:05' },
  { num: 2,  time: '09:10\n09:55' },
  { num: 3,  time: '10:15\n11:00' },
  { num: 4,  time: '11:05\n11:50' },
  { num: 5,  time: '14:00\n14:45' },
  { num: 6,  time: '14:50\n15:35' },
  { num: 7,  time: '15:55\n16:40' },
  { num: 8,  time: '16:45\n17:30' },
  { num: 9,  time: '18:30\n19:15' },
  { num: 10, time: '19:20\n20:05' },
  { num: 11, time: '20:10\n20:55' },
]

/** 星期映射 */
export const DAY_NAMES = ['周一', '周二', '周三', '周四', '周五', '周六', '周日']

/** 页面标识 */
export type PageId = 'schedule' | 'resource' | 'scheduling' | 'report' | 'settings'

/** 课表视图 */
export type ScheduleView = 'week' | 'timeline' | 'month'

/** 排课配置 */
export interface SchedulingConfig {
  scope: string
  semester: string
  strategy: string
  iterations: number
  constraints: string[]
  lockedSlots?: LockedTimeSlot[]
}

/** 锁定时间段 */
export interface LockedTimeSlot {
  dayOfWeek: number
  startPeriod: number
  span: number
}

/** 排课结果 */
export interface SchedulingResult {
  totalCourses: number
  scheduled: number
  conflicts: number
  utilization: number
  score?: number
  scoreDetail?: ScoreBreakdown
  logs: string[]
}

/** 评分明细 */
export interface ScoreBreakdown {
  total: number
  teacherPref: number
  courseSpacing: number
  teacherDays: number
  lowFloorPref: number
  weekendAvoid: number
}

/** 冲突 */
export interface Conflict {
  id: number
  type: string
  description: string
  severity: 'error' | 'warning'
  details: Record<string, any>
}
