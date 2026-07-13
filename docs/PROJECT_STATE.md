# PROJECT_STATE.md — 高校智能排课系统

> **最后更新**: 2026-07-13
> **当前版本**: v0.5.5 Phase 1 completed（Semester Domain Stabilization，已 merge main `93e0d2f`）
> **下一阶段**: v0.5.5 Phase 2 — Academic Calendar Extension（AcademicTerm / TeachingWeek）

---

## 1. 项目概况

湖北工业大学（HBUT）智能排课系统。Wails 桌面应用 (Go + Vue3 + Naive UI)，SA 模拟退火为主求解器，OR-Tools CP-SAT 为可选增强引擎。

**技术栈**: Go 1.22 + GORM + SQLite | Vue 3 + Pinia + Naive UI | Python (Flask + OR-Tools) | Wails v2

---

## 2. 版本状态

| 版本 | 状态 | 分支/Tag | 说明 |
|------|------|----------|------|
| v0.3.x | ✅ 已发布 | `v0.3.0` `v0.3.1` `v0.3.2` `v0.3.3` | 早期功能版 |
| v0.4.0 | ✅ 已发布 | `main` / tag `v0.4.0` | 体验与可扩展性（Stable Core 冻结） |
| v0.5.1 | 🔀 已合并到 v0.5.2 | `feat/v0.5.1-flexible-span` | 灵活课时跨度（1 commit: `4072448`） |
| v0.5.2 | ✅ 已发布 | tag `v0.5.2` (main) | 评分统一 + SA 性能优化 + 灵活课时跨度 |
| v0.5.3 | ✅ 已合并 | main（未打独立 tag） | URMF 统一资源匹配框架（P2/P3/P4/P6 系列 commit） |
| v0.5.4 | ✅ 已发布 | tag `v0.5.4` (main) | TeachingTask 领域稳定化 + Seed 幂等修复 |
| v0.5.5 Phase 1 | ✅ 已合并 | main `93e0d2f`（未打 tag，等 Phase 2/3 完成后统一打 v0.5.5） | Semester 领域稳定化 |
| v0.5.5 Phase 2 | ⏳ 待启动 | — | Academic Calendar Extension（AcademicTerm / TeachingWeek） |
| v0.5.5 Phase 3 | ⏳ 待启动 | — | 前端学期字段清理 + Solver 感知 |

### v0.5.2 发布内容（tag `v0.5.2`, 已合并到 main）

3 个 commit + 版本号更新 commit (d443109)，+2124 / -169 行：

1. **`4072448`** feat(scheduler): support flexible course span for HBUT
   - 新增 `session_plan.go` + 测试：课时跨度灵活化（1/2/3 节）
   - `models/types.go`: Span/Period 类型 + 98 行新代码
   - `models/teaching_task.go`: PreferredSpan 字段
   - `scheduler/solver.py`: OR-Tools 侧同步适配
   - `models/span_test.go`: 121 行模型测试

2. **`86ba983`** feat(scoring): unify scoring semantics + placement completeness + PostOptimize marginal
   - `scoring_service.go`: 新增 `Completeness` + `FinalTotal` 字段
     - 完成度比例 → 惩罚系数 `factor = ratio * (0.5 + 0.5*ratio)`
     - ratio=1.0 → 1.0 | ratio=0.5 → 0.375 | ratio=0.0 → 0.0
   - `scoring_context.go`: ScoringContext 扩展（+31 行）
   - `sa_postopt.go`: 后优化改为边际增量评估（+122 行重构）
   - `sa_initial.go` / `sa_neighbors.go`: 适配统一评分
   - `snapshot_service.go` / `snapshot.go`: 快照存储新增字段
   - `scheduling_service.go`: 排课入口适配
   - 新增测试: `score_completeness_test.go`, `sa_delta_parity_test.go`, `sa_postopt_test.go`
   - `frontend/src/types/index.ts`: 前端类型同步

3. **`ec07fd6`** perf(sa): delta-score cache + uint64 occupancy keys = 8x SA speedup (Goal 3)
   - **新增 5 文件**:
     - `sa_scorecache.go` (62 行): 缓存结构定义
     - `sa_scorecache_apply.go` (149 行): applyEntry ±1 增量更新
     - `sa_scorecache_reduce.go` (321 行): scoreFromCache → ScoreBreakdown 还原
     - `sa_bench_test.go` (123 行): 基准测试
     - `sa_delta_parity_test.go` (174 行): 缓存 vs 全量评分一致性验证
   - **修改**:
     - `sa_solver.go`: roomOcc/teacherOcc/classOcc 改用 `map[uint64]bool` + `occKey()` 打包
     - `sa_initial.go` / `sa_neighbors.go` / `sa_postopt.go`: 适配 uint64 键 + 缓存集成
   - **性能**: SA 迭代速度 29,830 → 236,000-241,000 iter/s（7.9-8.1x 加速）

---

## 3. Stable Core 冻结范围

以下为 v0.4.0 冻结的核心，**v0.5.2 不得破坏向后兼容**：

| 组件 | 文件 | 冻结内容 |
|------|------|----------|
| 评分入口 | `scoring_service.go` | `ScoreSchedule()` 签名 + 7 个软约束公式 |
| 评分上下文 | `scoring_context.go` | `ScoringContext` (version 1) |
| 评分结果 | `scoring_service.go` | `ScoreBreakdown` 结构 |
| 快照模型 | `models/snapshot.go` | 字段结构（可追加，不可改） |
| 版本管理 | `models/schedule_version.go` | `ScheduleVersion` + 50 条限制 |
| 教学任务 | `models/teaching_task.go` | `TeachingTask` + `TeachingTaskClass` |

**v0.5.2 对 Stable Core 的扩展**（向后兼容）：
- `ScoreBreakdown` 新增 `Completeness` + `FinalTotal` 字段（可选字段，旧代码不读不写无影响）
- `ScheduleSnapshot` 新增 `StudentFatigue` / `EnabledConstraints` / `ScoreVersion` 字段
- `TeachingTask` 新增 `PreferredSpan` 字段

---

## 4. 架构总览

```
┌──────────────────────────────────────────────┐
│  Frontend (Vue3 + Pinia + Naive UI)          │
│  ├─ SchedulingPage    ├─ SnapshotPage        │
│  ├─ ResourcePage      ├─ ReportPage          │
│  ├─ HistoryPage       ├─ SettingsPage        │
│  └─ Stores: schedule / scheduling / resource / app / ui │
├──────────────────────────────────────────────┤
│  Wails Bindings (Go ↔ JS)                    │
├──────────────────────────────────────────────┤
│  Backend Services (Go)                       │
│  ├─ SchedulingService (入口)                  │
│  │   └─ SolverOrchestrator                   │
│  │       ├─ SASolver (主引擎, 纯 Go)          │
│  │       │   ├─ sa_initial.go  (贪心初始解)    │
│  │       │   ├─ sa_neighbors.go (移动/交换)    │
│  │       │   ├─ sa_postopt.go   (后优化)      │
│  │       │   ├─ sa_scorecache*.go (增量评分)   │
│  │       │   └─ session_plan.go (课时跨度)    │
│  │       └─ ORToolsClient (可选, HTTP)        │
│  ├─ ScoringService (唯一评分入口)              │
│  │   └─ ScoringContext (version 1, 7+1 约束)  │
│  ├─ SnapshotService (快照 + 版本)             │
│  ├─ VersionService (历史版本, 50 条限制)       │
│  └─ DB Interface (ADR-0003, 依赖注入)         │
├──────────────────────────────────────────────┤
│  SQLite (GORM)                               │
├──────────────────────────────────────────────┤
│  Python Flask (可选, uv .venv)               │
│  └─ solver.py (CP-SAT 多 session X 变量模型)  │
└──────────────────────────────────────────────┘
```

### 评分维度（7 个软约束 + 1 个完成度）

| # | Key | 说明 | 满分 |
|---|-----|------|------|
| 1 | `teacherPreferences` | 教师不早课/晚课偏好 | 100 |
| 2 | `courseSpacing` | 同课程跨天分布 | 100 |
| 3 | `teacherDays` | 教师集中上课天数 | 100 |
| 4 | `lowFloorPref` | 低楼层偏好 | 100 |
| 5 | `weekendAvoid` | 周末课避免 | 100 |
| 6 | `pePeriodPref` | 体育课时段偏好 | 100 |
| 7 | `studentFatigue` | 学生疲劳度（连堂上限） | 100 |
| + | `completeness` | 排课完成度惩罚因子 | ×(0~1) |

`Total` = 7 项之和 (满分 700)
`FinalTotal` = `Total` × `completeness_factor(ratio)`

---

## 5. 架构决策记录 (ADR)

| ADR | 标题 | 状态 |
|-----|------|------|
| 0001 | SA 作为排课求解器 | ✅ 已采纳 |
| 0002 | 快照模式持久化 | ✅ 已采纳 |
| 0003 | DB 接口化（依赖注入） | ✅ 已采纳 |
| 0004 | TeachingTask 实体 | ✅ 已采纳 |
| 0005 | SA + OR-Tools 多引擎 + uv 打包 | ✅ 已采纳 |
| 0006 | 统一资源匹配框架 (URMF) | ✅ 已采纳 |

---

## 6. v0.4.0 已完成 Epic

| Epic | 优先级 | 内容 | 状态 |
|------|--------|------|------|
| A | P0 | 遗留重复代码消除 | ✅ |
| B | P0 | 全局学期状态 + 导航统一 | ✅ |
| C | P1 | 快照重命名 + Trigger 扩展 | ✅ |
| D | P1 | 约束配置 UI 优化 | ✅ |
| E | P2 | 锁定时段 + 学期日期选择器 | ✅ |
| F1 | P2 | 排课页配置面板可折叠 | ✅ |
| F2 | P0 | 评分链路统一验证 | ✅ (no-change) |
| G1 | P1 | Windows GUI 构建 | ✅ |
| G2 | P1 | WebView2 数据隔离 | ✅ |
| G3 | P1 | 应用元数据 | ✅ |
| G4 | P2 | 发布工程 | ✅ |
| H1 | P0 | 手动调课实时评分 | ✅ |
| H2-1 | P0 | 版本管理后端 | ✅ |
| H2-2 | P1 | 版本管理前端 | ✅ |
| H3 | P2 | 调整后保存快照 | 📋 Planned |

---

## 7. v0.5.2 发布状态

| Goal | 说明 | 状态 |
|------|------|------|
| Goal 1 | 灵活课时跨度 (1/2/3 节) | ✅ 已完成 |
| Goal 2 | 评分统一语义 + 完成度惩罚 | ✅ 已完成 |
| Goal 3 | SA delta-score 缓存 + uint64 键 | ✅ 已完成 (8x 加速) |
| Goal 4 | v0.5.2 发布（合并到 main + tag） | ✅ 已完成 (d443109, tag v0.5.2) |
| Goal 5 | H3: 调整后保存快照 | 📋 待做（从 v0.4.0 遗留） |

### v0.5.3 发布状态（URMF）

| Goal | 说明 | 状态 |
|------|------|------|
| Goal 1 | 数据模型扩展 (Category + Equipment + RequiredRoomType 派生) | ✅ 已完成 |
| Goal 2 | ResourceMatcher V1 (纯函数, `ee93fb4` P2) | ✅ 已完成 |
| Goal 3 | SA 接入 ResourceMatcher (`01ef918` P3) | ✅ 已完成 |
| Goal 4 | OR-Tools + MoveService 接入 (`86d6fbf` P4) | ✅ 已完成 |
| Goal 5 | `Classroom.RoomType` 冲突修复 (`b2d722b`) | ✅ 已完成 |
| Goal 6 | 独立 tag | ❌ 未打（内容随 v0.5.4 一同发布） |

### v0.5.4 发布状态（TeachingTask 稳定化）

| Goal | 说明 | 状态 |
|------|------|------|
| Goal 1 | 删除 TeachingTask 自动合并推断（`teaching_task_service.go` -139 行 / `ResourcePage.vue` -94 行） | ✅ 已完成 (`4958e20`) |
| Goal 2 | Seed 幂等性修复（Count+Create → FirstOrCreate） | ✅ 已完成 (`a69cb1a`) |
| Goal 3 | 实施报告 + 发布 review | ✅ 已完成 (`e44e60a` / `79c5493`) |
| Goal 4 | tag `v0.5.4` | ✅ 已打 |

### v0.5.5 Phase 1 状态（Semester Domain Stabilization）

| Goal | 说明 | 状态 |
|------|------|------|
| Goal 1 | Semester 结构化（AcademicYear/Term/StartDate:time.Time/EndDate/Status） | ✅ 已完成 |
| Goal 2 | ScheduleEntry.Semester → SemesterID FK | ✅ 已完成 |
| Goal 3 | ScheduleSnapshot.Semester → SemesterID FK | ✅ 已完成 |
| Goal 4 | Services 级联同步（SchedulingService/SnapshotService/VersionService/MoveService/SASolver） | ✅ 已完成 |
| Goal 5 | Seed 复合唯一键 `(academic_year, term)` | ✅ 已完成 |
| Goal 6 | Wails Bindings 重新生成 | ✅ 已完成 |
| Goal 7 | `go test ./...` + `npm run build` | ✅ 通过 |
| Goal 8 | merge main + 分支删除 | ✅ 已完成 (`93e0d2f`) |
| Goal 9 | 前端 `s.name`/`s.isActive` 引用清理 | ⏳ 明确延后至 Phase 3 |

### v0.5.5 Phase 2 规划（Academic Calendar Extension）

| Goal | 说明 | 状态 |
|------|------|------|
| Goal 1 | AcademicTerm 新表（Season + TermType，1:N 挂 Semester） | ⏳ 待启动 |
| Goal 2 | `services/academic_calendar/` 领域包（WeekView / CurrentSemester / CurrentWeek 派生） | ⏳ 待启动 |
| Goal 3 | AcademicTerm seed（每学期 3 段） | ⏳ 待启动 |

### v0.5.5 Phase 3 规划（前端 + Solver 接入）

| Goal | 说明 | 状态 |
|------|------|------|
| Goal 1 | 前端 `stores/app.ts`、`SettingsPage.vue` 等 5 文件 `s.name`/`s.isActive` 清理 | ⏳ 待启动 |
| Goal 2 | `WeekView.vue` 硬编码日期计算清除，改用后端派生 | ⏳ 待启动 |
| Goal 3 | Solver 可选感知 TermType | ⏳ 待启动 |

**Phase 1 设计文档**：
- [v0.5.5 Epic B 架构审查](release/V0.5.5_EPIC_B_ARCHITECTURE_REVIEW.md)
- [v0.5.5 Epic B 日历基础设计](release/V0.5.5_EPIC_B_ACADEMIC_CALENDAR_DESIGN.md)
- [v0.5.5 Seed 演进对齐审查](release/V0.5.5_SEED_ALIGNMENT_REVIEW.md)
- [v0.5.5 Phase 1 实施报告](release/V0.5.5_PHASE1_IMPLEMENT_REPORT.md)

---

## 8. 待办与风险

### 待办
1. **v0.5.5 Phase 2 启动** — AcademicTerm 新表 + academic_calendar 领域包（等待用户指令）
2. **v0.5.5 Phase 3** — 前端 `s.name`/`s.isActive` 清理（5 文件 10+ 处）+ WeekView 日期派生 + Solver TermType
3. **H3: 调整后保存快照** — v0.4.0 遗留 P2 任务

### 风险
1. **前端旧字段引用仍编译通过**：前端 `s.name`/`s.isActive` 引用（Phase 3 承接）在 `npm run build` 中未报错——原因待查（可能是 TS 宽松类型/隐式转换），Phase 3 启动前需复核
2. **v0.5.3/v0.5.5 tag 缺失**：v0.5.3 内容散在 P2-P6 commit，无独立 tag；v0.5.5 Phase 1 已 merge 但等 Phase 2/3 完成后统一打 tag
3. **solver.py 同步** — session_plan.go 和 solver.py 的 span 规则需手动保持同步
4. **SA 缓存正确性依赖测试保障** — delta cache 的正确性完全依赖 `TestDeltaScoreMatchesFullScore`，需确保测试覆盖充分

---

## 9. 关键文件索引

### 后端核心
| 文件 | 说明 |
|------|------|
| `backend/services/scheduling_service.go` | 排课入口 `RunScheduling()` |
| `backend/services/scoring_service.go` | 唯一评分入口 `ScoreSchedule()` |
| `backend/services/scoring_context.go` | 评分上下文 (version 1, 7+1 约束) |
| `backend/services/sa_solver.go` | SA 主逻辑 |
| `backend/services/sa_scorecache*.go` | v0.5.2 增量评分缓存（3 文件） |
| `backend/services/session_plan.go` | v0.5.2 课时跨度规划 |
| `backend/services/snapshot_service.go` | 快照创建 + 存储 |
| `backend/services/ortools_client.go` | OR-Tools HTTP 客户端 |

### 数据模型
| 文件 | 说明 |
|------|------|
| `backend/models/schedule_entry.go` | 排课条目 |
| `backend/models/snapshot.go` | 快照模型 |
| `backend/models/teaching_task.go` | 教学任务 |
| `backend/models/types.go` | DayOfWeek / Period / Span 类型 |

### 前端
| 文件 | 说明 |
|------|------|
| `frontend/src/views/SchedulingPage.vue` | 排课页 |
| `frontend/src/stores/scheduling.ts` | 排课状态 |
| `frontend/src/types/index.ts` | 类型定义 |

### Python
| 文件 | 说明 |
|------|------|
| `scheduler/solver.py` | OR-Tools CP-SAT 求解器 |

### 文档
| 文件 | 说明 |
|------|------|
| `docs/adr/0001-0005` | 5 个架构决策记录 |
| `TEST_PLAN.md` | v0.5.0 起测试计划 |
| `.scratch/v0.4.0/PLAN.md` | v0.4.0 详细计划（归档） |
