// ===== 核心数据类型 =====

/** 课程 */
export interface Course {
  id: number
  code: string
  name: string
  dept: DeptCode
  credit: number
  type: string  // 专业必修 | 全校选修 | ...
  hours: number
}

/** 教师 */
export interface Teacher {
  id: number
  code: string
  name: string
  dept: string
  title: string   // 教授 | 副教授 | 讲师
  status: 'active' | 'inactive'
  weeklyHours?: number
}

/** 教室 */
export interface Classroom {
  id: number
  code: string
  name: string
  building: string
  capacity: number
  type: string   // 普通教室 | 实验室 | 体育馆
  status: string
}

/** 班级 */
export interface ClassGroup {
  id: number
  code: string
  name: string
  dept: string
  grade: number
  students: number
}

/** 学期 */
export interface Semester {
  id: number
  name: string
  isActive: boolean
}

/** 排课条目 */
export interface ScheduleEntry {
  id: number
  courseId: number
  teacherId: number
  classroomId: number
  semester: string
  dayOfWeek: number    // 0=Mon ... 6=Sun
  startPeriod: number  // 0-9
  span: number         // 连续节数
  weeks: string        // "1-16"
  course?: Course
  teacher?: Teacher
  classroom?: Classroom
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
export type PageId = 'schedule' | 'resource' | 'scheduling' | 'conflict' | 'settings'

/** 课表视图 */
export type ScheduleView = 'week' | 'timeline' | 'month'

/** 排课配置 */
export interface SchedulingConfig {
  scope: string
  semester: string
  strategy: string
  iterations: number
  constraints: string[]
}

/** 排课结果 */
export interface SchedulingResult {
  totalCourses: number
  scheduled: number
  conflicts: number
  utilization: number
  logs: string[]
}

/** 冲突 */
export interface Conflict {
  id: number
  type: string
  description: string
  severity: 'error' | 'warning'
  details: Record<string, any>
}
