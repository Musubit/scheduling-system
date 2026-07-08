/**
 * Wails v3 runtime 封装
 * Wails v3 在运行时自动注入 window.wails 对象，
 * Go 方法通过 window.go.main.App.MethodName() 调用
 *
 * 开发模式下如果 Go 后端未启动，方法调用会失败 —
 * 各 view 应使用 try/catch 或检测 isWailsEnv()
 */

// Wails v3 运行时类型声明
declare global {
  interface Window {
    wails?: {
      Call: (method: string, ...args: any[]) => Promise<any>
    }
    go?: {
      main: {
        App: Record<string, (...args: any[]) => Promise<any>>
      }
    }
  }
}

/**
 * 检查是否在 Wails 环境中运行
 */
export function isWailsEnv(): boolean {
  return !!(window.go?.main?.App)
}

/**
 * 调用 Go 后端方法
 */
async function callGo<T>(method: string, ...args: any[]): Promise<T> {
  if (!window.go?.main?.App) {
    console.warn(`[useWails] Wails runtime not available, method "${method}" will use mock data`)
    throw new Error('Wails runtime not available')
  }
  return (window.go.main.App as any)[method](...args)
}

// ===== 导出的 API 方法 =====

export async function getCourses() {
  return callGo<any[]>('GetCourses')
}

export async function createCourse(course: any) {
  return callGo<void>('CreateCourse', course)
}

export async function updateCourse(course: any) {
  return callGo<void>('UpdateCourse', course)
}

export async function deleteCourse(id: number) {
  return callGo<void>('DeleteCourse', id)
}

export async function getTeachers() {
  return callGo<any[]>('GetTeachers')
}

export async function createTeacher(teacher: any) {
  return callGo<void>('CreateTeacher', teacher)
}

export async function updateTeacher(teacher: any) {
  return callGo<void>('UpdateTeacher', teacher)
}

export async function deleteTeacher(id: number) {
  return callGo<void>('DeleteTeacher', id)
}

export async function getClassrooms() {
  return callGo<any[]>('GetClassrooms')
}

export async function getScheduleEntries(semester: string) {
  return callGo<any[]>('GetScheduleEntries', semester)
}

export async function runScheduling(config: any) {
  return callGo<any>('RunScheduling', config)
}

export async function detectConflicts(semester: string) {
  return callGo<any[]>('DetectConflicts', semester)
}
