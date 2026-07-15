/**
 * 教学周日期派生工具。
 * 从学期开学日期（Semester.StartDate）计算指定教学周的日期。
 */

const MS_PER_DAY = 86400000

/**
 * 解析学期第一天 ISO 字符串；解析失败时兜底当年 1 月 1 日。
 */
function parseStartDate(startDate: string): Date {
  if (startDate) {
    const d = new Date(startDate)
    if (!isNaN(d.getTime())) return d
  }
  return new Date(new Date().getFullYear(), 0, 1)
}

/**
 * 从学期开学日期派生指定教学周的 7 天日期。
 * 学期第一天所在周为第 1 周；周一为每周第一天（ISO 8601）。
 *
 * @param startDate - 学期第一天，ISO 字符串（如 "2026-09-01T00:00:00Z"）
 * @param weekNumber - 教学周序号（1-based）
 * @returns 长度为 7 的 Date 数组，周一到周日
 */
export function getWeekDates(startDate: string, weekNumber: number): Date[] {
  const anchor = parseStartDate(startDate)
  // 对齐 anchor 到当周周一 (getDay: Sun=0..Sat=6)
  const anchorDow = anchor.getDay() === 0 ? 6 : anchor.getDay() - 1
  const anchorMonday = new Date(anchor.getTime() - anchorDow * MS_PER_DAY)
  // 从学期第一周周一 + (weekNumber - 1) * 7 天派生目标周周一
  const monday = new Date(anchorMonday.getTime() + (weekNumber - 1) * 7 * MS_PER_DAY)
  return Array.from({ length: 7 }, (_, i) => new Date(monday.getTime() + i * MS_PER_DAY))
}

/**
 * 格式化为短日期 "M/D"，用于周视图表头。
 */
export function formatShortDate(date: Date): string {
  return `${date.getMonth() + 1}/${date.getDate()}`
}
