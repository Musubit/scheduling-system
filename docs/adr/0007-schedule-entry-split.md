# ADR-0007: ScheduleEntry 拆分为 TimeAssignment + ScheduleEntry

**Status**: ✅ 已采纳
**Date**: 2026-07-20
**Version**: v0.6.0

## Context

原 `ScheduleEntry` 模型混合了时间分配和教室分配两个职责：一个条目同时包含 `DayOfWeek/StartPeriod/Span`（时间）和 `ClassroomID`（资源）。这导致：

1. TIME_ONLY 模式下必须创建"虚拟教室"和 `IsVirtual` 标记来绕过教室唯一约束
2. 时间和教室更新耦合在一起（MoveService 一次修改两个维度）
3. v0.7 多方案排课需要同一时间分配对应多个教室方案

## Decision

将 `ScheduleEntry` 拆分为两个独立表：

- **TimeAssignment**: 时间事实（`TeachingTaskID`, `DayOfWeek`, `StartPeriod`, `Span`）
- **ScheduleEntry**: 资源分配（`TimeAssignmentID` FK → TimeAssignment, `ClassroomID`）

TIME_ONLY 模式只写 `TimeAssignment`，不写 `ScheduleEntry`。

## Consequences

### 正面
- **INV-E1**: TIME_ONLY 模式下 `schedule_entries` 表零行（不再需要虚拟教室）
- **时间/教室解耦**: MoveService 改时间只动 TA，改教室只动 SE
- **多方案基础**: 同一 TimeAssignment 可对应多个 ScheduleEntry（v0.7）

### 负面
- 读路径需要 JOIN（`ScheduleQueryService.GetEnrichedScheduleEntries`）
- 数据库 migration（v0.5.7 数据备份到 `_bak_schedule_entries_v057`）
- MoveService 需要适配双表操作

## Migration

```sql
-- 备份旧表
ALTER TABLE schedule_entries RENAME TO _bak_schedule_entries_v057;
-- 新建 time_assignments 和 schedule_entries（通过 GORM AutoMigrate）
```

## Alternatives Considered

- **方案 A**: 保留原模型，加 `EntryType` 标记 — 拒绝，不解决根本问题
- **方案 C**: 三表拆分（TimeAssignment + RoomAssignment + ScheduleEntry） — 拒绝，过度设计
