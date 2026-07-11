/**
 * 车道分配算法：将可能重叠的条目分配到不同车道（lane），
 * 确保同一车道内无时间重叠。返回每个条目的车道索引及总车道数。
 *
 * @param entries  已按 startPeriod 排序的条目数组
 * @param getEnd   从条目中提取结束节次的函数（默认 startPeriod + span）
 * @returns        带 lane / totalLanes 的条目数组
 */
export function allocateLanes<T extends { startPeriod: number; span: number }>(
  entries: T[],
): Array<T & { lane: number; totalLanes: number }> {
  if (entries.length <= 1) {
    return entries.map(e => ({ ...e, lane: 0, totalLanes: 1 }))
  }

  const sorted = [...entries].sort((a, b) => a.startPeriod - b.startPeriod)
  const lanes: number[] = [] // lane -> endPeriod
  const result: Array<T & { lane: number; totalLanes: number }> = []

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
