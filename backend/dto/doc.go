// Package dto 存放前后端 & 版本快照的持久化 DTO。
//
// 这里的类型是**终结序列化值** —— 不带 GORM tag、不参与 Solver / Scheduler 计算,
// 也没有反向 DTO→model 映射 API(INV-SN2)。作用是:
//
//   - Snapshot / Version 的 entries_json 内容必须严格等于 ScheduleSnapshotDTO(INV-SN1)。
//   - 前端读快照时只消费 DTO 字段,不能触发 DB 二次查询(INV-F6)。
//   - SchemaVersion 是升级门:值不匹配时进 legacy readonly 分支。
//
// 参见:docs/superpowers/specs/2026-07-13-scheduling-dual-mode-design.md §2.5
package dto
