# ACTIVE_TASK.md — 当前活跃任务

> **最后更新**: 2026-07-13
> **当前版本**: v0.5.5 Phase 1 completed（已 merge main `93e0d2f`）
> **下一阶段**: v0.5.5 Phase 2 — Academic Calendar Extension

---

## 当前状态

v0.5.5 Phase 1（Semester Domain Stabilization）已完成并合并到 main：
- Semester 模型重构：`AcademicYear/Term/StartDate:time.Time/EndDate/Status`
- `ScheduleEntry.Semester` / `ScheduleSnapshot.Semester` 由 `string` 迁移为 `SemesterID uint` FK
- Services / SASolver / Seed / Wails Bindings 已同步
- `go test ./...` + `npm run build` 通过
- 分支 `release/v0.5.5-academic-calendar` 已删除

历史发布轨迹（v0.5.3 URMF、v0.5.4 TeachingTask 稳定化、v0.5.5 Phase 1）见 `docs/PROJECT_STATE.md § 7`、`CHANGELOG.md`。

---

## 下一步行动

### 🔴 P0: v0.5.5 Phase 2 — Academic Calendar Extension（待启动）

**前置条件**（✅ 全部满足）：
- [x] Phase 1 Semester 基础模型已稳定
- [x] Seed 幂等验证通过
- [x] Bindings 与前端 build 通过
- [x] docs 状态已同步（chore/v0.5.5-repository-sync）

**范围**（等待用户下发正式设计前，暂列骨架）：

| Item | 内容 |
|------|------|
| 新表 | `academic_terms`（SemesterID FK + Season(AUTUMN/WINTER/SPRING/SUMMER) + TermType(TEACHING/EXAM/PRACTICE) + StartWeek/EndWeek） |
| 领域包 | `backend/services/academic_calendar/`：CurrentSemester / CurrentWeek / WeekView 派生（不建 TeachingWeek 表） |
| Seed | 每学期 3 段（秋季常规/秋季考试/冬季实践 等） |
| 前端 | 暂不动（Phase 3 承接） |

**明确不做**：
- Holiday 表（另开 Epic）
- Solver 感知 TermType（Phase 3）
- 前端 `s.name`/`s.isActive` 清理（Phase 3）

### 🟡 P1: v0.5.5 Phase 3 — 前端清理 + Solver 感知

Phase 2 完成后处理：
- `stores/app.ts` / `AppToolbar.vue` / `SettingsPage.vue` / `SchedulingPage.vue` / `HistoryComparePage.vue` 中 10+ 处 `s.name`/`s.isActive` 引用清理
- `WeekView.vue` 硬编码"当年 1 月 1 日 + week×7"删除，改用 Semester.StartDate 派生
- Solver 可选感知 TermType

### 🟢 P2: H3 调整后保存快照（v0.4.0 遗留）

Phase 3 完成后处理。

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
