# Roadmap

高校智能排课系统长期产品路线图。

本文档按 Theme（主题）组织版本规划，作为后续开发的统一基线。
具体 Issue 和 Milestone 不在此文档中拆分，由各版本启动时另行定义。

---

## Current State

| 项目 | 值 |
|------|-----|
| Current Version | v0.3.3 |
| Project Status | Stable Core 已冻结，进入迭代增强阶段 |
| Architecture | Wails v3 (Go + Vue 3 + OR-Tools) |
| Solver | SA（模拟退火）+ OR-Tools（CP-SAT）双引擎 |

### Stable Core Summary

以下核心模块已在 v0.3.2 Final Stabilization 中冻结，后续版本不得破坏性修改：

- `ScoreSchedule()` — 评分唯一入口
- `ScoreBreakdown` — 评分数据结构（含 PerCategoryMax / EnabledCategoryCount）
- `ScoringContext` — 评分上下文封装
- `ScheduleSnapshot` — 快照核心字段（含 Name / PerCategoryMax）
- `Snapshot.Name` 机制 — 快照名称唯一来源

---

## Roadmap Principles

| 原则 | 说明 |
|------|------|
| **Stable Core** | 已冻结的核心模块仅允许修复严重 Bug，不进行破坏性修改。新功能在核心之上扩展，不修改核心接口。 |
| **Single Source of Truth** | 每一类数据只有一个权威来源。Score 来自 `ScoreSchedule()`，PerCategoryMax 来自后端计算，Snapshot.Name 来自 `ScheduleSnapshot.Name`。前端不得重新计算或拼接。 |
| **Backend First** | 业务逻辑和数据计算优先在后端完成，前端仅负责展示和交互。新增评分项、约束、分析能力时，先定义后端接口，再实现前端。 |
| **Version Theme** | 每个版本聚焦一个主题，不混合多个方向。主题内的功能可以灵活调整，但版本边界由主题决定。 |
| **Continuous Release** | 小步快跑，持续发布。每个 minor 版本可独立发布，不依赖后续版本的完成。 |

---

## Version Roadmap

### v0.4.x - Experience & Extensibility

| 项 | 内容 |
|----|------|
| Theme | 用户体验提升、可扩展性增强、约束体系完善 |
| Focus | 消除 v0.3 遗留技术债（Backup/Restore 重构、导航数据统一、DEFAULT_LOCKED 统一、fuzzyFilter 优化）；快照重命名 UI；新增 Trigger 类型支持；约束体系扩展预留 |
| Status | Planned |

### v0.5.x - Intelligent Scheduling

| 项 | 内容 |
|----|------|
| Theme | AI 辅助排课、智能分析、智能建议 |
| Focus | 排课结果智能诊断；冲突自动修复建议；教师工作量均衡分析入求解器（基于 v0.3 后处理数据积累）；排课方案多维度对比 |
| Status | Planned |

### v0.6.x - Optimization Engine

| 项 | 内容 |
|----|------|
| Theme | 排课策略优化、多算法协同、性能提升 |
| Focus | SA / OR-Tools 策略协同优化；大规模数据性能调优；约束权重自动调参；求解时间预算控制 |
| Status | Planned |

### v0.7.x - Data & Decision

| 项 | 内容 |
|----|------|
| Theme | 数据分析、统计报表、决策支持 |
| Focus | 排课历史趋势分析；教室利用率统计；教师工作量报表；院系排课质量对比；决策支持仪表盘 |
| Status | Planned |

### v0.8.x - Ecosystem

| 项 | 内容 |
|----|------|
| Theme | 导入导出增强、插件化、开放扩展能力 |
| Focus | Excel/CSV 导入导出增强；自定义约束插件接口；第三方系统集成预留；数据备份恢复完善 |
| Status | Planned |

### v0.9.x - Polish & Stabilization

| 项 | 内容 |
|----|------|
| Theme | UI 打磨、工程优化、长期稳定性提升 |
| Focus | 全局 UI 一致性审查；前端性能优化；错误处理统一；日志体系完善；国际化预留 |
| Status | Planned |

### v1.0.0 - First Stable Release (LTS)

| 项 | 内容 |
|----|------|
| Theme | 高校智能排课系统正式版 |
| Focus | 全量回归测试；文档完善；部署指南；长期支持版本锁定 |
| Status | Planned |

---

## Stable Core Policy

以下模块自 v0.3.2 起冻结。后续版本（v0.4.x 及以后）遵守以下规则：

### 冻结模块

| 模块 | 位置 | 冻结内容 |
|------|------|----------|
| ScoreSchedule | `backend/services/scoring_service.go` | 方法签名、评分算法逻辑、perCategoryMax 计算方式 |
| ScoreBreakdown | `backend/services/scoring_service.go` | 结构体字段定义（可新增字段，不修改已有字段语义） |
| ScoringContext | `backend/services/scoring_context.go` | 结构体字段、构造函数签名 |
| ScheduleSnapshot | `backend/models/snapshot.go` | 已有字段语义（可新增字段，不修改已有字段） |
| Snapshot.Name | `backend/models/snapshot.go` | Name 生成规则、DisplayName / DefaultSnapshotName 方法 |

### 修改规则

| 场景 | 允许 |
|------|------|
| 修复严重 Bug（评分错误、数据损坏、崩溃） | ✅ 允许，需在 CHANGELOG 中标注 |
| 新增字段（不破坏已有字段语义） | ✅ 允许 |
| 新增评分方法（不修改已有方法） | ✅ 允许 |
| 修改已有字段语义或类型 | ❌ 禁止 |
| 修改已有方法签名 | ❌ 禁止 |
| 重命名已有字段或方法 | ❌ 禁止 |

---

## Status Legend

| 状态 | 含义 |
|------|------|
| Completed | 已完成并发布 |
| In Progress | 正在开发中 |
| Planned | 已规划，尚未开始 |
| Stable | 已冻结，仅接受严重 Bug 修复 |
| Deprecated | 已废弃，将在未来版本移除 |
