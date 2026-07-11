/**
 * 子序列模糊匹配 — 搜"职师"能匹配"职业技术师范学院"
 * 用于 n-select 的 :filter 属性
 */
export function fuzzyFilter(pattern: string, option: { label: string; value: any }): boolean {
  if (!pattern) return true
  const p = pattern.toLowerCase()
  const l = option.label.toLowerCase()
  let pi = 0
  for (let li = 0; li < l.length && pi < p.length; li++) {
    if (l[li] === p[pi]) pi++
  }
  return pi === p.length
}
