# ACTIVE_TASK.md — 当前活跃任务

> **最后更新**: 2026-07-14
> **当前版本**: v0.5.5 Phase 1 completed（已 merge main `93e0d2f`）+ v0.5.5 P0 M3 前端 / P2 兼容层 / P3 SchedulingPage 保存快照 已就地推进
> **下一阶段**: v0.5.5 P0 剩余(INV-E1 修复 + Orchestrator 装配重构 + M4 门禁)

---

## 当前状态

v0.5.5 Phase 1（Semester Domain Stabilization）已完成并合并到 main。此后在同一分支就地推进的增量:

**已合入待 commit(本地工作树):**
- P0 M3 前端: SchedulingPage 结果面板 mode-aware(隐藏 TIME_ONLY 教室冲突行/硬约束验证行 + mode-badge + 约束禁用提示),TIME_ONLY 下 `low_floor_preference` checkbox 显式 disabled
- P2 前端清理(Step 23-25):
  - `appStore.semesterSelectOptions` computed 引入结构化 label + 状态标记(●当前/○预排/□已归档)
  - AppToolbar 全局学期下拉、SchedulingPage 学期下拉都改为消费 `semesterSelectOptions`,不再直接读 `s.name`/`s.isActive`
  - WeekView 周日期从"当年 1 月 1 日 + week×7"改为从 `appStore.currentSemester.startDate` 派生,修复跨年学期日期错误
- P3 SchedulingPage 保存快照按钮 + dirtyMoveCount(WeekView 拖拽后累加,快照保存后清零)+ 快照命名弹窗
- 后端 `snapshot_service_test.go` 新增两个 test 锁 CreateManualSnapshot 契约(空学期报错 / 有 entries happy path)
- 之前会话遗留: seed 精简到 1 planned 学期、SettingsPage 结构化字段弹窗、`resource_service.go` normalizeSemester+demote 互斥
- Linux 构建修复: `build/linux/Taskfile.yml` CGO_ENABLED 默认 "1"(Wails v3 alpha Linux 后端强需 CGO)

历史发布轨迹（v0.5.3 URMF、v0.5.4 TeachingTask 稳定化、v0.5.5 Phase 1）见 `docs/PROJECT_STATE.md § 7`、`CHANGELOG.md`。

---

## 下一步行动

### 🔴 P0: v0.5.5 核心引擎剩余项(高风险,需签字)

| Item | 内容 | 状态 |
|------|------|------|
| M1 Mode 主链路 | `SchedulingConfig.Mode` 已进 `RunScheduling`,已按 mode 分支 | ✅ 完成 |
| M1 Orchestrator 装配 | 目前 9 处 `isTimeOnly` 分支散落在 `RunScheduling`,未收敛到 `SolverOrchestrator.Run` | ⏳ 未做(~10h,中风险) |
| M2 INV-E1(TIME_ONLY 零教室行) | 目前仍写合成 `ClassroomID` 到 `schedule_entries` 满足 not-null 约束,违反规范 | ⏳ 未做(~8h,**高风险**,需签字选方案) |
| M2 EnabledScoreDimensions 结构化门控 | TIME_ONLY 资源维度目前靠 `classrooms=nil` 短路,不是显式 dimension 门控 | ⏳ 未做(~4h) |
| M2 Score bucket nullable | `ScoreBreakdown` 各分桶都是 float64 default 0,禁用与实评 0 无法区分 | ⏳ 未做(~6h) |
| M2 Snapshot/Version 存 Mode 字段 | 历史行不带 mode,前端无法区分 TIME_ONLY 快照的资源分是禁用还是真 0 | ⏳ 未做(~3h) |
| M3 前端 mode-aware 结果面板 | 冲突行/验证行/badge/禁用提示 | ✅ 完成 |
| M4 门禁 | 双模式回归 checklist + CHANGELOG + release notes | ⏳ 未做(~2h) |

**决策阻塞项(需明确后再动手):**
1. **INV-E1 修复方案** — (a) `ClassroomID` 改 `*uint` nullable + 从 `idx_schedule_room` 移出 + 迁移 vs (b) 新增 `time_assignments` 表拆分。规范倾向 (b),但改动巨大。
2. **是否引入 `time_assignments` 新表** — 影响 Snapshot/Version JSON 语义、前端 EnrichedEntry 服务、Wails bindings 全量重生。

### 🟠 P1: v0.5.5 Phase 2 — Academic Calendar Extension（顺延）

**前置条件**（✅ 全部满足）:
- [x] Phase 1 Semester 基础模型已稳定
- [x] Seed 幂等验证通过
- [x] Bindings 与前端 build 通过

**范围**:

| Item | 内容 |
|------|------|
| 新表 | `academic_terms`（SemesterID FK + Season(AUTUMN/WINTER/SPRING/SUMMER) + TermType(TEACHING/EXAM/PRACTICE) + StartWeek/EndWeek） |
| 领域包 | `backend/services/academic_calendar/`：CurrentSemester / CurrentWeek / WeekView 派生（不建 TeachingWeek 表） |
| Seed | 每学期 3 段（秋季常规/秋季考试/冬季实践 等） |
| 前端 | 暂不动（Phase 3 承接） |

**明确不做**：
- Holiday 表（另开 Epic）
- Solver 感知 TermType（Phase 3,且经 P2 Step 27 决策后可能整体划出 v0.5.5 范围）

### 🟡 P2: v0.5.5 Phase 3 — 前端清理 + Solver 感知(部分已就地完成)

| Item | 状态 |
|---|---|
| AppToolbar / SchedulingPage 消费 `semesterSelectOptions` | ✅ 完成 |
| WeekView 硬编码 1 月 1 日删除,改用 Semester.StartDate | ✅ 完成 |
| appStore `name`/`isActive` 兼容层最终删除(等所有消费端确认迁移完) | ⏳ 观察一版后 |
| HistoryComparePage `snapshot.name` 依赖(是 snapshot 自带 name,非 semester,不需要动) | ⚠️ 已确认不需要动 |
| Solver TermType 感知 | ⚠️ P2 Step 27 决策待定 — 若产品明确不要,建议整体划出 v0.5.5 |

### 🟢 P3: H3 调整后保存快照 — SchedulingPage hookup(已就地完成)

| Item | 状态 |
|---|---|
| Backend `CreateManualSnapshot` | ✅ 早已存在 |
| ReportPage 生成报告按钮 | ✅ 早已接通 |
| SchedulingPage 保存快照按钮 + 命名弹窗 | ✅ 完成(本次) |
| Schedule store `dirtyMoveCount` | ✅ 完成(本次) |
| WeekView 拖拽后调 `markDirty()` | ✅ 完成(本次) |
| Backend 单元测试 | ✅ 完成(本次) |

**H3 现已可宣告 done-with-caveat**:前端两处入口(SchedulingPage + ReportPage)都能命名保存快照。

---

## 设计文档索引

### v0.5.3 URMF（已实施完成）
| 文档 | 路径 | 状态 |
|------|------|------|
| URMF 设计 R3 | `docs/design/v0.5.3-resource-matching-framework.md` | ✅ 已实施 |
| 实施计划 | `docs/design/v0.5.3-implementation-plan.md` | ✅ 已实施 |
| 测试矩阵 | `docs/testing/v0.5.3-resource-tests.md` | ✅ 已实施 |
| ADR-0006 | `docs/adr/0006-unified-resource-matching-framework.md` | ✅ 已采纳 |
| P6 前端 review | `docs/review/v0.5.3-p6-frontend-review.md` | ✅ 已归档（`b2d722b` 消化） |

### v0.5.4 TeachingTask 稳定化（已实施完成）
| 文档 | 路径 |
|------|------|
| 实施报告 | `docs/release/V0.5.4_IMPLEMENT_REPORT.md` |
| Release Review | `docs/release/V0.5.4_RELEASE_REVIEW.md` |
| 最终发布报告 | `docs/release/V0.5.4_FINAL_RELEASE_REPORT.md` |
| Seed 幂等修复 | `docs/release/SEED_IDEMPOTENCY_FIX_REPORT.md` |

### v0.5.5 Phase 1 Semester Domain（已实施完成）
| 文档 | 路径 |
|------|------|
| Epic B 架构审查 | `docs/release/V0.5.5_EPIC_B_ARCHITECTURE_REVIEW.md` |
| Epic B 日历基础设计 | `docs/release/V0.5.5_EPIC_B_ACADEMIC_CALENDAR_DESIGN.md` |
| Seed 演进对齐审查 | `docs/release/V0.5.5_SEED_ALIGNMENT_REVIEW.md` |
| Phase 1 实施报告 | `docs/release/V0.5.5_PHASE1_IMPLEMENT_REPORT.md` |

### v0.5.5 Dual-Mode Scheduling(P0 SSoT)
| 文档 | 路径 | 状态 |
|------|------|------|
| Fast-track 计划 | `docs/superpowers/plans/2026-07-14-v0.5.5-core-engine-generalization-fast-track.md` | ⏳ M3 完成,M1/M2 剩余,M4 待做 |
| 双模式设计 spec(SSoT) | `docs/superpowers/specs/2026-07-13-scheduling-dual-mode-design.md` | 已冻结 |
