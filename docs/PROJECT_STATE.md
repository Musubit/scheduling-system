# PROJECT_STATE.md — 高校智能排课系统

> **最后更新**: 2026-07-13
> **当前分支**: `main` (v0.5.2 已发布)
> **下一版本**: v0.5.3 — 统一资源匹配框架 (URMF)

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
| v0.5.3 | 📐 设计就绪 | — | 统一资源匹配框架 (URMF) |

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

### v0.5.3 规划

| Goal | 说明 | 状态 |
|------|------|------|
| Goal 1 | 数据模型扩展 (Category + Equipment + RequiredRoomType) | 📐 设计就绪 |
| Goal 2 | ResourceMatcher V1 (纯函数) | 📐 设计就绪 |
| Goal 3 | SA + OR-Tools + MoveService 统一接入 | 📐 设计就绪 |
| Goal 4 | 前端 UI 资源管理增强 | 📐 设计就绪 |

**设计文档**:
- [v0.5.3 URMF 设计文档](design/v0.5.3-resource-matching-framework.md) (R3 冻结)
- [v0.5.3 实施计划](design/v0.5.3-implementation-plan.md) (P1-P7)
- [v0.5.3 测试矩阵](testing/v0.5.3-resource-tests.md) (89 个测试)
- [ADR-0006: URMF](adr/0006-unified-resource-matching-framework.md)

---

## 8. 待办与风险

### 待办
1. **v0.5.3 实施** — 按 P1-P7 七阶段实施，每阶段验收后提交
2. **H3: 调整后保存快照** — v0.4.0 遗留 P2 任务
3. **ROADMAP.md 缺失** — 需重建

### 风险
1. **分支与发布标签不一致** — main 在 v0.4.0，开发分支领先 3 commits 未合并
2. **solver.py 同步** — session_plan.go 和 solver.py 的 span 规则需手动保持同步
3. **SA 缓存正确性依赖测试保障** — delta cache 的正确性完全依赖 `TestDeltaScoreMatchesFullScore`，需确保测试覆盖充分

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
