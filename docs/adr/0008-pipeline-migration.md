# ADR-0008: Strangler Fig Pipeline Migration

**Status**: ✅ 已采纳
**Date**: 2026-07-20
**Version**: v0.6.0–v0.6.1

## Context

v0.5.x 的排课引擎是单体 SA 求解器（`sa_solver.go`），混合了时间和教室分配。v0.6.x 的目标是将引擎重构为两阶段流水线：

1. **Stage 1** (TimeScheduler): 纯时间排课，实现 `ITimeScheduler`
2. **Stage 2** (RoomScheduler): 教室分配，实现 `IRoomScheduler`
3. **Stage 3** (Scorer): 4-bucket 评分，实现 `IScorer`

三个阶段由 `ISchedulingOrchestrator` 组合调度。

## Decision

采用 **Strangler Fig（绞杀榕）渐进迁移策略**：

- **v0.6.0**: 创建 Adapter（`LegacySASolverAdapter`, `LegacyRoomAllocatorAdapter`, `LegacyScorerAdapter`），桥接旧实现到新接口。`SchedulingOrchestrator` 接管 `RunScheduling()`，但内部委托给 Adapter。
- **v0.6.1**: 实现纯版本 `scheduling/time/`（SA TimeScheduler）、`scheduling/room/`（Greedy Scheduler）、`scheduling/score/`（4-bucket Scorer）。在 `app.go` 替换 Adapter 为纯实现。删除旧 SA solver 和 Adapter 文件（~1900 行）。
- **v0.6.2**: 删除残留旧代码，冻结核心模型。

## Rationale

- **风险可控**: 每阶段独立 merge + 回滚
- **生产不中断**: 每阶段经过完整编译验证
- **接口先行**: Adapter 建立了明确的接口契约，新实现只需满足接口

## Consequences

### 正面
- 旧 SA solver 已完全删除，代码库减 ~1900 行
- 新 `scheduling/time/` 包无 models/DB 依赖（纯函数）
- 新 `scheduling/room/` 和 `scheduling/score/` 包仅依赖 types 接口

### 负面
- v0.6.0 → v0.6.1 过渡期间 Adapter 是空壳（stub）
- OR-Tools 集成路径尚未完全迁移（solver.py 保留）

## Alternatives Considered

- **方案 A** (Big Bang 重写): 一次性替换所有引擎 — 拒绝，风险太高
- **方案 C** (特性开关): 通过配置切换新旧引擎 — 拒绝，增加不必要的复杂度
