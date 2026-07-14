# ROADMAP.md — 高校智能排课系统

> **最后更新**: 2026-07-14
> **当前版本**: v0.5.5 Phase 1 completed (main `93e0d2f`) / 下一阶段 v0.5.5 P0（核心引擎通用能力）待启动

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

### v0.5.2 — 评分统一 + SA 性能优化 ✅ (tag `v0.5.2`)

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

### v0.5.3 — Unified Resource Matching Framework (URMF) ✅ (已合并 main, 无独立 tag)

**目标**: 统一 SA / OR-Tools / MoveService 三处教室类型判断为单一纯函数决策。

| Goal | 内容 | 状态 |
|------|------|------|
| Goal 1 | ResourceMatcher V1 纯函数 + `MatchResult` + `Explain` (`ee93fb4` P2) | ✅ |
| Goal 2 | SA 求解器接入 `AllowedRooms` (`01ef918` P3) | ✅ |
| Goal 3 | OR-Tools + MoveService 接入，OR-Tools payload `AllowedRoomIDs` 由 Go 侧计算 (`86d6fbf` P4) | ✅ |
| Goal 4 | `Classroom.RoomType` 冲突修复，复用现有 `Classroom.Type` (`b2d722b`) | ✅ |
| Goal 5 | ADR-0006 URMF 采纳 | ✅ |

### v0.5.4 — TeachingTask Domain Stabilization ✅ (tag `v0.5.4`)

**目标**: 消除 TeachingTask 自动合班推断包袱，稳定教学任务领域模型。

| Goal | 内容 | 状态 |
|------|------|------|
| Goal 1 | 删除 `teaching_task_service.go` 自动合并推断（-139 行）(`4958e20`) | ✅ |
| Goal 2 | `ResourcePage.vue` 相关 UI 移除（-94 行） | ✅ |
| Goal 3 | Seed 幂等性修复 Count+Create → FirstOrCreate (`a69cb1a`) | ✅ |

### v0.5.5 — Academic Calendar Foundation（先 P0 后 Phase 推进）

#### v0.5.5 Phase 1 — Semester Domain Stabilization ✅ (已合并 `93e0d2f`, 未独立打 tag)

| Goal | 内容 | 状态 |
|------|------|------|
| Goal 1 | Semester 模型重构：AcademicYear/Term/StartDate:time.Time/EndDate/Status | ✅ |
| Goal 2 | ScheduleEntry.Semester / ScheduleSnapshot.Semester → SemesterID FK | ✅ |
| Goal 3 | Services + SASolver + Seed + Bindings 级联同步 | ✅ |
| Goal 4 | Seed 复合唯一键 (academic_year, term) | ✅ |

#### v0.5.5 P0 — Core Engine Generalization ⏳ (最高优先级,部分完成)

| Goal | 内容 | 状态 |
|------|------|------|
| Goal 1 | 排课模式主链路打通：`FULL_SCHEDULING` / `TIME_ONLY_SCHEDULING` | ✅ M1 完成 |
| Goal 2 | 支持关闭教室分配后仍可完成课程时间安排 | ⚠️ 功能可用,但 INV-E1(TIME_ONLY 零教室行)未合规 |
| Goal 3 | TIME_ONLY 评分维度对齐（资源维度禁用而非伪 0） | ⚠️ 靠 classrooms=nil 短路,非结构化 dimension 门控 |
| Goal 4 | 前端模式开关与后端透传闭环 | ✅ M3 完成(结果面板 mode-aware badge + 冲突行/验证行隐藏 + 约束禁用提示) |
| Goal 5 | 双模式最小回归门禁（go test / 前端 build / 核心流程） | ⏳ M4 未做 |

**决策阻塞**: INV-E1 修复方案(ClassroomID nullable vs 拆表 time_assignments)、Orchestrator 装配是否本 P0 内做,均需签字。见 `docs/ACTIVE_TASK.md § 决策阻塞项`。

#### v0.5.5 Phase 2 — Academic Calendar Extension ⏳ (待启动)

| Item | 说明 |
|------|------|
| AcademicTerm 表 | 每学期 3 段（秋季常规/秋季考试/冬季实践 等），Season + TermType |
| `services/academic_calendar/` 领域包 | CurrentSemester / CurrentWeek / WeekView 派生（TeachingWeek 不建表） |
| Seed | 每学期 3 条 AcademicTerm 记录 |

#### v0.5.5 Phase 3 — 前端清理 + Solver 感知 ⚙️ (部分就地完成)

| Item | 说明 | 状态 |
|------|------|------|
| 前端字段清理 | `stores/app.ts` 派生 `semesterSelectOptions`；`AppToolbar` / `SchedulingPage` / `SettingsPage` 已迁移；`HistoryComparePage` 用的是 snapshot.name(与 semester 无关,无需迁) | ✅ 消费端已切换 |
| WeekView 日期派生 | 从 `appStore.currentSemester.startDate` 派生周日期 | ✅ 完成 |
| Solver TermType 感知 | 需产品决策是否属于 v0.5.5 范围;当前 solver 完全 term-agnostic | ⏳ 决策待定 |

### H3 — 调整后保存快照 (v0.4.0 遗留) ✅

手动调课后用户主动生成快照:
- Backend `CreateManualSnapshot` 已存在
- ReportPage 生成报告按钮已接
- **SchedulingPage 保存快照按钮 + dirtyMoveCount + 命名弹窗** 已就地完成
- 后端 `snapshot_service_test.go` 契约 test 已补

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
