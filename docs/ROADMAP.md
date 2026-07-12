# ROADMAP.md — 高校智能排课系统

> **最后更新**: 2026-07-13
> **当前版本**: v0.4.0 (main) / v0.5.2 (feat/v0.5.2-score-solver, 待合并)

---

## 版本总览

```
v0.3.x  ████████████  已发布 — 基础功能
v0.4.0  ████████████  已发布 — 体验与可扩展性 (Stable Core 冻结)
v0.5.x  ████████░░░░  开发中 — 智能排课基础
v0.6.x  ░░░░░░░░░░░░  规划中 — 教务流程
v0.7.x  ░░░░░░░░░░░░  规划中 — 工程稳定性
v1.0    ░░░░░░░░░░░░  规划中 — 生产发布
```

---

## v0.5.x — Intelligent Scheduling Foundation

**主题**: 在 Stable Core 之上建立高性能、语义统一的智能排课能力。

### v0.5.2 — 评分统一 + SA 性能优化 ✅ (待合并)

| Goal | 内容 | 状态 |
|------|------|------|
| Goal 1 | 灵活课时跨度 (1/2/3 节, HBUT 适配) | ✅ |
| Goal 2 | ScoreSchedule 完成度评分 (FinalTotal + Completeness) | ✅ |
| Goal 3 | SA delta-score cache + uint64 keys (8x 加速) | ✅ |

**Stable Core 兼容性**: 全部新增字段可选 (zero-value = legacy 行为)
- `ScoreBreakdown`: +PlacedSessions/ExpectedSessions/Completeness/FinalTotal
- `ScoringContext`: +ExpectedTotalSessions (0 = legacy)
- `ScheduleSnapshot`: +FinalScore/PlacedSessions/ExpectedSessions/Completeness
- `TeachingTask`: +PreferredSpan (0 = 不指定)
- ScoringContext.Version: 1 → 2 (v1 调用路径保留)

### v0.5.3 — 教室资源约束增强 (规划中)

**目标**: 支持课程类别 × 教室类型 × 设备约束，最小侵入 Stable Core。

| Item | 说明 |
|------|------|
| 课程类别 | Course 模型新增 Category 字段 (theory/lab/pe/seminar) |
| 教室类型 | Classroom 模型新增 RoomType 字段 (普通/实验室/机房/体育馆/多媒体) |
| 设备约束 | Classroom 模型新增 Equipment 字段 (JSON array: projector/smartboard/aircon...) |
| 匹配规则 | 教学任务可声明 RequiredRoomType + RequiredEquipment，求解器在硬约束中检查 |
| 评分影响 | 可选软约束: 同类课程优先集中教室 (减少教室切换) |

**设计原则**: 全部为模型字段追加 + 求解器约束增强，不修改 ScoreSchedule 签名和 7 个软约束公式。

### v0.5.4 — H3: 调整后保存快照 (v0.4.0 遗留)

手动调课后用户主动生成快照。后端 `CreateManualSnapshot` 已存在，需前端对接。

---

## v0.6.x — Academic Workflow

**主题**: 将排课系统融入教务日常工作流。

| Epic | 说明 |
|------|------|
| A — 学期日历 | 学期起止日期 + 教学周计算引擎 |
| B — 教学任务导入增强 | Excel 模板标准化 + 校验规则 + 导入预览 |
| C — 课表导出 | Excel/PDF 导出，按教师/班级/教室/课程多维视图 |
| D — 调课工作流 | 调课申请 → 冲突检测 → 执行 → 快照 |
| E — 教师工作量统计 | 周课时/学期课时/跨校区统计 |

---

## v0.7.x — Engineering Stability

**主题**: 工程化加固，为生产发布做准备。

| Epic | 说明 |
|------|------|
| A — 日志与可观测性 | 结构化日志 + 排课过程 tracing |
| B — 错误处理统一 | 全局错误码 + 用户友好提示 |
| C — 数据备份与恢复 | 一键导出/导入全量数据 |
| D — CI/CD | GitHub Actions: lint + test + build + release |
| E — 性能基准 | benchmark/ 数据集 + 回归测试门禁 |
| F — 安全加固 | 数据库加密 + 敏感信息保护 |

---

## v1.0 — Production Release

**主题**: 面向湖北工业大学教务处的生产级交付。

| 门槛 | 标准 |
|------|------|
| 功能完整 | v0.5.x + v0.6.x 全部 Epic 完成 |
| 稳定性 | 连续 3 个版本无 P0 bug |
| 性能 | 22 教学任务 SA < 3s, 50 任务 < 10s |
| 文档 | ADR 完整、用户手册、管理员指南 |
| 部署 | 一键安装包 (Wails + uv 自包含) |
