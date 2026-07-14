# v0.5.5 Dual-Mode Scheduling — Batch Execution Specification

> **Execution Specification**
>
> This document supplements `2026-07-13-scheduling-dual-mode-design.md` and defines
> the implementation waves, PR boundaries, verification criteria, migration gates,
> and release procedure. It does not replace or modify the architecture design
> specification.
>
> - Architecture Design (why): `docs/superpowers/specs/2026-07-13-scheduling-dual-mode-design.md`
> - Execution Spec (how):     this document

- **Version**: 1.0
- **Date**: 2026-07-14
- **Owner**: musubi
- **Status**: Approved — ready for implementation planning

---

## 0. 上下文

**目标**:交付原设计 §5.2 中 PR-03 → PR-12 共 10 个 PR,达到 v0.5.5 Dual-Mode
(FULL_SCHEDULING + TIME_ONLY_SCHEDULING)可发布状态。

**当前起点**:
- `main` HEAD = `21771f4`(Linux baseline)
- 远程分支 `origin/refactor/v0.5.5-p0-p3-batch` 领先 main 7 个 commit,已就地推进:
  - PR-02 骨架(`backend/dto/schedule_snapshot.go` + 测试)
  - PR-03 骨架(`models/schema_migration.go` + `models/time_assignment.go` + AutoMigrate 注册)
  - PR-10 部分(前端 mode-aware SchedulingPage、SettingsPage 结构化字段弹窗、
    P2 兼容层 `semesterSelectOptions`)
  - P3 部分(SchedulingPage 保存快照 + `dirtyMoveCount`)

**决策记录**:
| # | 决策 | 值 |
|---|---|---|
| 1 | "10 个 PR"指哪一组 | 原设计 §5.2 的 PR-03 → PR-12 |
| 2 | refactor 分支如何归位 | Squash 合入 main 作为 pre-step,重新按 PR-03~12 推进 |
| 3 | 合入方式 | `git merge --squash` 后单 commit |
| 4 | 交付节奏 | Subagent-driven,按依赖图分波并行 |
| 5 | PR-09 INV-E1 修复方案 | 方案 (b) 彻底拆分,`schedule_entries` 不含教室 |
| 6 | PR-11 Cleanup Gate | Release Qualification Gate(checklist,非时间) |
| 7 | PR-12 Scope | 仅 Mode UI 适配,不动导出 |
| 8 | CI 依赖检查 | `go list -deps` / grep 脚本,不写 vet analyzer |

---

## 1. 架构不变量(新增)

原设计已定义 INV-A1~A3 / INV-E1~E3 / INV-M1~M4 / INV-S1~S3 / INV-SN1~SN2 /
INV-MI1~MI3。本执行 spec 追加两条:

### 1.1 INV-A4 — 调度内核依赖边界

`backend/scheduling/*` 只允许依赖以下路径:
- `backend/models/`
- `backend/scheduling/types/`
- `backend/scheduling/matcher/`
- `backend/scheduling/score/`
- `backend/scheduling/room/`
- `backend/scheduling/time/`
- Go 标准库

**禁止依赖**:
- `backend/services/*`
- `backend/database/*`
- `backend/wails/*`
- `backend/app.go`

违反由 CI 拒绝(见 §7)。

### 1.2 Compatibility First 原则

> Legacy 代码仅承担迁移职责,不允许新增业务继续依赖 Legacy;所有新增能力必须
> 依赖 `backend/scheduling/*`,最终形成统一调度内核。

**执行含义**:
- PR-04 ~ PR-08 只添加代码到 `backend/scheduling/*`,legacy 保留 shim
- PR-10 之后,任何新功能开发的 imports 只能来自 `backend/scheduling/*`
- PR-11 之后,legacy 目录被清空,原则不再需要显式检查

---

## 2. Migration 版本链

`schema_migrations` 表以 `EnsureMigrationApplied(name string)` 幂等 API 写入:

| 版本 | 引入 | 含义 |
|---|---|---|
| `v0.5.5-room` | refactor squash(pre-step) | Stage B Room 域完成 |
| `v0.5.5-prep` | PR-03 | DTO / 元表 / SchedulingMode 字段就绪,行为不变 |
| `v0.5.5-dbgate` | PR-09 | `time_assignments` 建表 + `schedule_entries.ClassroomID` 移除 |
| `v0.5.5-release` | PR-10 | 主链路切至 Orchestrator |

**API 契约**:
```go
// EnsureMigrationApplied 幂等地记录一个 migration 已应用。
// 已存在同名 version 时不重复插入,不报错。
func EnsureMigrationApplied(db DB, name string) error
```

**测试要求**:每个 migration 引入者(PR-03 / PR-09 / PR-10)都必须包含
"重复调用 EnsureMigrationApplied 幂等" 单测。

---

## 3. Wave 计划总览

```
Wave 0 (pre)  : squash refactor → main                    [1 step]
Wave 1        : PR-03 EnsureMigrationApplied + 字段       [1 agent, 串行]
Wave 2        : PR-04, 05, 06, 07, 08 unwired 领域包       [5 agents 并行]
Wave 3        : PR-09 time_assignments + 拆分 + shim      [1 agent, 高风险]
Wave 4        : PR-10 Orchestrator 主链路 + UI mode       [1 agent, 中风险]
Wave 5        : PR-11 legacy 删除, PR-12 前端 Mode UI      [2 agents 并行]
收尾          : 分支清理 + CHANGELOG + tag v0.5.5
```

**关键路径**:Wave 0 → 1 → 2(最长) → 3 → 4 → 5(最长)

**波与波之间的 gate**:每一波所有 PR 合入 main 且 CI 全绿后,才启动下一波。

---

## 4. Wave 0 — Squash Pre-step

### 4.1 操作

```bash
git checkout main
git pull --ff-only
git merge --squash origin/refactor/v0.5.5-p0-p3-batch
git commit -m "chore(v0.5.5): P0/M2 骨架 + P2 前端兼容层 + P3 快照 hookup(pre-step)

Squash of refactor/v0.5.5-p0-p3-batch (6 commits).
Content will be absorbed / superseded by PR-02 through PR-12.

No behavior change intended.
Preparation only."
git push origin main
```

### 4.2 Verify(先本地跑,再 push)

- `go test ./...` 全绿
- `cd frontend && npm run build` 通过
- `git diff origin/main...HEAD --stat`(push 前用 `HEAD^..HEAD`) 确认变更文件与
  refactor 分支列出的 39 个文件一致,无临时文件误入
- `git log --oneline -1` 确认 HEAD 就是 squash commit

### 4.3 明确不做

- 不删除 `origin/refactor/v0.5.5-p0-p3-batch` 远程分支(等 PR-12 合并后统一清理)
- 不 tag(v0.5.5 tag 在 PR-12 合并后打)

---

## 5. Wave 1 — PR-03: schema_migrations 元表 + SchedulingMode + MaxRetries

### 5.1 Scope

**新增**:
- `backend/database/migrations.go`:
  ```go
  func EnsureMigrationApplied(db DB, name string) error
  ```
- 在应用启动路径中调用 `EnsureMigrationApplied(adapter, "v0.5.5-prep")`,
  紧接在 AutoMigrate 之后

**确认字段就位**(refactor squash 里已有部分,PR-03 补齐):
- `SchedulingConfig.Mode`(string,默认 `FULL_SCHEDULING`)—— 已在,验证即可
- `SchedulingConfig.MaxRetries`(int,默认从设计 §3.2 取)—— 需检查是否已有,缺则补
- `ScheduleVersion.Mode`、`ScheduleSnapshot.Mode` —— refactor squash 里已加

**Wails bindings 重生**:`wails3 generate bindings` 后 commit 前端 bindings。

### 5.2 Verify

- 新增 `backend/database/migrations_test.go`:
  - 空库首次调用 `EnsureMigrationApplied` → 表出现一行
  - 重复调用同名 version → 仍只有一行
- **启动幂等验证**(手工):
  - 删本地 `scheduling.db`
  - 启动应用一次 → 关闭 → `select * from schema_migrations` 应见
    `v0.5.5-room` 一行 + `v0.5.5-prep` 一行
  - 再启动一次 → 仍只有那两行
- `go test ./...` 全绿
- `cd frontend && npm run build` 通过

### 5.3 明确不做

- 不建 `time_assignments` migration 逻辑(PR-09)
- 不改 `schedule_entries` schema(PR-09)
- 不写 `time_assignments` service 代码(PR-09)
- 不修改 `SchedulingService.RunScheduling` 的 mode 分支
  (refactor squash 里就地做的,PR-10 才清理)

### 5.4 风险

极低。纯元数据 + 字段确认。

---

## 6. Wave 2 — PR-04~08 五个 unwired 领域包(5 agents 并行)

### 6.1 全局约束(适用所有 5 个 PR)

- 每个 PR 只在 `backend/scheduling/<package>/` 新建代码,不修改 `backend/services/*`
  主逻辑(除必要的 shim 引用)
- 每个 PR 提供**等价性验证**:新代码 vs 老代码,给定同一 fixture,输出**逐字段**
  `cmp.Diff() == ""`
- 每个 PR 有 unit test,依赖用 fake 替代,禁止在 unit test 中调用真实的下游服务

### 6.2 PR-04: `backend/scheduling/matcher/`(agent A)

**Scope**:
- 新包,导出 `matcher.Match(task, classroom) MatchResult`
- 从 `services/resource_matcher.go` 复制纯函数逻辑
- 老 `resource_matcher.go` 保留,内部改为转调 `matcher.Match`(shim)

**Verify**(三层):
1. **Golden**:5 个人工维护 fixture(`testdata/matcher/basic.json` /
   `lab.json` / `computer.json` / `sports.json` / `art.json`)
2. **Random Property**:100 组随机 fixture,老 `ResourceMatcher` 输出 vs
   新 `matcher.Match` 输出,`cmp.Diff() == ""`
3. **CI 边界**:`go list -deps ./backend/scheduling/matcher/...` 不含
   `backend/services/*` 或 `backend/database/*`

**不做**:老 `resource_matcher.go` 的所有 caller 迁移(PR-11 才做)

### 6.3 PR-05: `backend/scheduling/score/`(agent B)

**Scope**:
- 新包,导出 `Scorer` 接口 + 4 bucket(Time / Teacher / Room / Group)
- 实现 `ScoreBreakdown` 结构,支持 nullable(设计 §3.2 INV-S2)
- 老 `scoring_service.go` 保留(PR-11 前不动)

**Verify**:
- Golden fixture 5 组 + Random 100 组
- **逐字段** `cmp.Diff()`,禁止只比 `Total`(评审反馈:防止分桶漂移被
  Total 相等掩盖)
- Fake dependency test:score 内部只依赖 `matcher` 和 `types`,用 fake matcher
  跑 unit test
- CI 边界:`go list -deps` 检查

**不做**:老 `scoring_service.go` caller 迁移(PR-11)

### 6.4 PR-06: `backend/scheduling/room/`(agent C)

**Scope**:
- 新包,导出 `RoomScheduler` + `Greedy` 实现
- 消费 PR-04 的 `matcher`

**Verify**:
- Golden fixture 3 组
- **依赖边界证明**(评审反馈):用 fake matcher 跑 unit test,证明
  `RoomScheduler` 只需要 `matcher.Match` 接口即可工作
- CI 边界:`go list -deps ./backend/scheduling/room/...` 只允许出现
  `matcher` / `types` / 标准库

**不做**:与主链路挂接(PR-10)

### 6.5 PR-07: `backend/scheduling/time/`(agent D)

**Scope**:
- 新包,含 `Driver` 接口 + `SASolver` 变体 + `ORToolsClient`
- 从 `services/sa_solver.go` 和 `services/ortools_client.go` 复制
- 老文件不 delete(PR-11)

**Verify**:
- 给定同一 fixture(固定随机 seed),新旧 SA 输出:
  - Snapshot 结构
  - Assignments(order-normalized)
  - ConflictCount
  - Score(逐字段)
- 使用 `cmp.Equal(old, new)`(评审反馈:**不用 hash**,map 顺序影响)
- Fake dependency test:time 只依赖 `matcher` / `score` / `types`

**不做**:主链路 caller 迁移

### 6.6 PR-08: `backend/scheduling/orchestrator/`(agent E)

**Scope**:
- 新包,导出 `Orchestrator.Run(ctx, cfg) (*Snapshot, error)`
- **按 Mode 装配**:
  - `FULL_SCHEDULING`: time 阶段 → room 阶段 → score 汇总
  - `TIME_ONLY_SCHEDULING`: time 阶段 → score 汇总(room 阶段跳过)
- 组合 PR-05/06/07 的成品

**Verify**:
- **纯 unit test**(评审反馈):用 fake scorer / fake room / fake time,
  不依赖任何真实实现或 DB
- 覆盖:
  - FULL 成功路径
  - TIME_ONLY 成功路径
  - time 失败 → 整体失败
  - FULL 模式下 room 失败 → 整体失败
  - TIME_ONLY 模式下 room 阶段被跳过(用 fake room 断言未被调用)

**不做**:与 `SchedulingService.RunScheduling` 集成(PR-10)

### 6.7 Wave 2 汇合验证

所有 5 个 PR 合并 main 后,一次性跑:
- `go test ./...` 全绿
- **CI 边界脚本**(见 §7):`backend/scheduling/*` 不含 forbidden imports
- 老 `services/*` 主流程测试全绿(证明 shim 无回归)

---

## 7. CI 依赖边界检查

不写 Go vet analyzer,不引入额外依赖。

**推荐方案**(build script `scripts/check-deps.sh`):

```bash
#!/usr/bin/env bash
# 检查 backend/scheduling/* 不依赖 forbidden 包
set -e

FORBIDDEN=(
  "scheduling-system/backend/services"
  "scheduling-system/backend/database"
  "scheduling-system/backend/wails"
)

for pkg in $(go list ./backend/scheduling/... 2>/dev/null); do
  deps=$(go list -deps -f '{{.ImportPath}}' "$pkg")
  for forbidden in "${FORBIDDEN[@]}"; do
    if echo "$deps" | grep -q "^$forbidden"; then
      echo "❌ $pkg imports $forbidden"
      exit 1
    fi
  done
done
echo "✅ backend/scheduling/* 依赖边界正常"
```

Wave 2 首个 PR 引入该脚本,后续 PR 都在 PR checklist 中调用它。

---

## 8. Wave 3 — PR-09: time_assignments + INV-E1 修复(方案 b)

### 8.1 采用方案

**方案 (b)**:彻底拆分教室分配模型,`schedule_entries` **不再包含教室**。

- `schedule_entries.ClassroomID` **完全删除**(不改成 nullable)
- 教室分配数据完全由 `time_assignments` 承载
- FULL 模式:time 阶段完成后,room 阶段向 `time_assignments` 写入
- TIME_ONLY 模式:不产生 `time_assignments` 记录

避免 Nullable Pollution(`if entry.ClassroomID != nil {...}` 蔓延)。

### 8.2 Scope

#### 8.2.1 DB 迁移

新增 `MigrateV055DBGate()`(单事务):

```sql
BEGIN TX
  -- 1. 建 time_assignments
  CREATE TABLE IF NOT EXISTS time_assignments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    ...(见原设计 §2.4)
  );

  -- 2. 重建 schedule_entries(SQLite 标准做法),移除 ClassroomID
  CREATE TABLE schedule_entries_new (
    ...(不含 classroom_id 字段)
  );
  INSERT INTO schedule_entries_new (id, ...) SELECT id, ... FROM schedule_entries;
  DROP TABLE schedule_entries;
  ALTER TABLE schedule_entries_new RENAME TO schedule_entries;

  -- 3. Snapshot/Version schema_version 字段
  ALTER TABLE schedule_snapshots ADD COLUMN schema_version TEXT DEFAULT 'v0.5.4';
  ALTER TABLE schedule_versions  ADD COLUMN schema_version TEXT DEFAULT 'v0.5.4';

  -- 4. 记录 migration 完成
  -- EnsureMigrationApplied(db, "v0.5.5-dbgate")
COMMIT
```

#### 8.2.2 Model 变更

- `models/schedule_entry.go`:删除 `ClassroomID uint` 字段(不改 `*uint`)
- 新表 `models/time_assignment.go` 已在 refactor squash 里,PR-09 只完善字段

#### 8.2.3 Write-side Shim(编译保持适配层)

> **PR-09 的 Shim 仅用于保持主链路可编译(Compile-preserving Write Adapter),
> 不是 Orchestrator 集成,不属于主链路切换。**

**职责**:
- 替代所有 `entry.ClassroomID = X` 写入点(约 8-10 处 caller)
- 提供统一写接口:
  ```go
  // AssignmentWriter 是 PR-09 引入的编译保持适配器。
  // 唯一目的:在 schedule_entries.ClassroomID 被移除后,
  // 让主链路继续可编译。将在 PR-10 被 Orchestrator 集成替换。
  type AssignmentWriter interface {
      Record(ctx context.Context, entryID uint, classroomID uint) error
  }
  ```
- 内部仅向 `time_assignments` 写数据

**严格限制**(评审反馈,强约束):
- ✅ 仅允许修改 Write Path
- ❌ **不允许**修改 SchedulingService 的编排流程
- ❌ **不允许**新增 Mode 分支(refactor squash 里现有的 mode 分支保持不变)
- ❌ **不允许**修改读取逻辑(Read Path)
- ❌ **不允许**提前依赖 Orchestrator

### 8.3 前置准备(在写代码前完成)

- 备份 checklist 归档 `docs/superpowers/plans/pr-09-deploy-checklist.md`:
  - `cp scheduling.db pre_v0.5.5_dbgate_backup.db`
  - `sqlite3 scheduling.db ".dump" > pre_v0.5.5_dbgate.sql`
  - 备份文件保留 30 天
- 回滚脚本 `backend/database/rollback/roll_back_v0.5.5_dbgate.sql`:
  写好并归档,不集成到应用

### 8.4 Verify

**功能测试**:
1. Fixture 1(空库):首次跑 → `schema_migrations` 有 `v0.5.5-dbgate` +
   `time_assignments` 存在 + `schedule_entries` 无 `classroom_id` 列
2. Fixture 2(v0.5.4 db,20 条老数据):跑 → 20 条老数据全部保留、字段值不变
   (除 classroom_id 已删除)、`time_assignments` 空表
3. Fixture 3(重复跑):无变化
4. **Migration Benchmark**(评审反馈):
   - 生成 10,000 条 schedule_entries + 关联数据
   - Migration 成功、数据一致、耗时 < 3 秒

**回滚验证**:
- 在 Fixture 2 状态下手工跑 `roll_back_v0.5.5_dbgate.sql`(sqlite CLI)
- 验证:还原到 v0.5.4 schema、`schema_migrations` 里 `v0.5.5-dbgate` 行被删

**Shim 集成测试**:
- FULL 模式排课一次 → `schedule_entries` 无教室字段、`time_assignments` 有对应行
- TIME_ONLY 模式排课一次 → `schedule_entries` 有条目、`time_assignments` 空

### 8.5 风险与回滚

- 🔴 **风险等级**:高(唯一 DB 结构破坏性变更)
- **回滚路径**:`git revert PR-09` + 手工跑 `roll_back_v0.5.5_dbgate.sql` +
  从 `pre_v0.5.5_dbgate_backup.db` 恢复
- **回滚窗口**:PR-09 部署后 30 天内,备份不删除

### 8.6 明确不做

- 不切换 `SchedulingService.RunScheduling` 到 Orchestrator(PR-10)
- 不删 legacy(PR-11)
- 不动前端 UI(PR-10 / PR-12)

---

## 9. Wave 4 — PR-10: 主链路切换 + UI Mode

### 9.1 Scope

#### 后端
- `backend/services/scheduling_service.go` `RunScheduling`:
  - 删除 refactor squash 里散在 9 处的 `isTimeOnly` 分支
  - 改为**单次调用** `orchestrator.Orchestrator.Run(ctx, cfg)`
  - Orchestrator 返回结果 → 现有 DB 落库层(通过 PR-09 的 shim)
- `EnsureMigrationApplied(db, "v0.5.5-release")`

#### 前端
- `frontend/src/views/SchedulingPage.vue`:
  - "关闭教室分配"checkbox → **Radio 组**
    (FULL_SCHEDULING / TIME_ONLY_SCHEDULING)
  - UI 文案按设计 §3.4
- Wails bindings 重生

### 9.2 Verify

**HBUT Golden Regression Baseline**(评审反馈,作为长期回归基线):
- Fixture:`testdata/hbut-golden/`(HBUT 22 教学任务真实数据)
- 验证:
  - Snapshot 结构逐字段
  - Score 逐 bucket
  - Assignment Count
  - Conflict Count
- 用 `cmp.Equal()`,基线文件签入 git

**自动化测试**:
- 原设计 §3.10 I1/I2/I3/I5/I7 五项
- `go test ./...` + `npm run build`

**手工回归**:
- FULL / TIME_ONLY 各跑一遍,截图存
  `docs/superpowers/plans/pr-10-manual-regression.md`

### 9.3 明确不做

- 不删 legacy(PR-11)
- 前端 mode-aware 结果面板不重做(refactor squash 里已有)
- 不动导出功能(PR-12 也不动)

### 9.4 回滚

`git revert PR-10` → 主链路回到 `SchedulingService` 内联逻辑,shim 仍在,
数据不受影响。

---

## 10. Wave 5 — PR-11 + PR-12 并行

### 10.1 PR-11 Release Qualification Gate(评审反馈,取代"one work day")

**Gate 由 Checklist 决定,不由时间决定。**PR-10 合并后,以下**全部满足**方可开 PR-11:

- [ ] FULL 模式连续验证通过(至少 3 次连跑)
- [ ] TIME_ONLY 模式连续验证通过(至少 3 次连跑)
- [ ] 无 Crash
- [ ] 无 P0 Bug
- [ ] 无 P1 Bug
- [ ] 无 Migration Bug(schema_migrations 一致)
- [ ] 无 Snapshot Corruption(现有快照可正常反序列化)
- [ ] HBUT 真实数据验证完成(与 §9.2 Golden 一致)

**Gate 通过后 checklist 归档到 `docs/superpowers/plans/pr-11-gate-checklist.md`。**

### 10.2 PR-11: 删除 Legacy(agent F)

**Scope**(spec §5.2 明列 + solver.py):
- `backend/services/sa_solver.go` 及关联文件
- `backend/services/ortools_client.go`
- `backend/services/scoring_service.go`
- `backend/services/resource_matcher.go`
- `backend/services/solver_orchestrator*.go`
- `backend/python/solver.py` 里 `solve_full` 函数(保留 socket 骨架)

**删除前置 Verify**(评审反馈):
```bash
# 对每个待删符号,确认 0 caller
for sym in ResourceMatcher SAsolver ORToolsClient ScoringService SolverOrchestrator; do
  echo "=== $sym ==="
  git grep "$sym" -- '*.go' | grep -v "^backend/services/" || echo "  0 caller ✓"
done
```
不允许"看编译过就删",必须 grep 证据链。

**删除后 Verify**:
- `go build ./...` + `go test ./...` 全绿
- CI 边界脚本(§7)通过
- `npm run build` 通过

**限制**:纯删除。若删除中遇到"这个符号被 X 引用了",**停下来报告**,不允许顺手改 X。

### 10.3 PR-12: 前端 Mode UI 深度适配(agent G)

**Scope**(评审反馈,已缩减):
- `SchedulingPage.vue`:schedule 表格 mode-aware
  (TIME_ONLY 收窄、隐藏教室列)
- `score-card` 组件:TIME_ONLY 下 Room bucket 显示 "Disabled" 而非 0
- `HistoryComparePage.vue`:对比两个不同 mode 的 snapshot 时显示警告

**明确不做**(评审反馈):
- ❌ **不修改导出功能(CSV / Excel / PDF)** —— 单独版本处理

**Verify**:
- `npm run build` 通过
- 手工:切换 FULL/TIME_ONLY,确认 UI 差异符合设计 §3.4
- 手工:对比 FULL snapshot vs TIME_ONLY snapshot,确认警告出现

### 10.4 并行安全性

- PR-11 只删 backend,PR-12 只改 frontend
- 无共享文件冲突,可真并行

---

## 11. 收尾(Wave 5 全部合并后)

- [ ] 删除远程分支:`git push origin --delete refactor/v0.5.5-p0-p3-batch`
- [ ] `git remote prune origin`
- [ ] 删除所有 Wave 中间产生的 feature 分支(本地 + 远程)
- [ ] `CHANGELOG.md` 落笔 v0.5.5 release notes
- [ ] `docs/ACTIVE_TASK.md` / `docs/ROADMAP.md` 同步
- [ ] 打 tag:`git tag -a v0.5.5 -m "..."` + push

**收尾产物 checklist**:
- [ ] `main` 分支 CI 全绿
- [ ] `git branch -r` 只剩 `origin/main`
- [ ] `git tag` 包含 `v0.5.5`

---

## 12. 风险与缓解总表

| PR | 风险 | 缓解 |
|---|---|---|
| PR-03 | 极低 | 幂等 API + 启动两次验证 |
| PR-04~08 | 低 | 三层 verify(golden / random / dep 边界),不 wire 到主链路 |
| PR-09 | 🔴 高 | 预备份 + 回滚脚本 + 30 天备份保留 + 10k benchmark |
| PR-10 | 🟡 中 | Golden regression baseline + 手工回归 checklist |
| PR-11 | 🟡 中 | Release Qualification Gate + grep 0 caller 前置验证 |
| PR-12 | 低 | UI-only,scope 缩减,不动导出 |

---

## 13. 完成定义(DoD)

10 个 PR 全部合并且以下满足:

1. ✅ 用户可在 UI 通过 Radio 一键切换 FULL / TIME_ONLY
2. ✅ TIME_ONLY 下不因教室不足导致整体排课失败
3. ✅ TIME_ONLY 评分与展示语义自洽,不出现伪资源分
4. ✅ FULL 路径关键行为无回归(HBUT Golden ±5%)
5. ✅ Legacy `services/*` 内 solver/matcher/scoring 全部清空
6. ✅ `backend/scheduling/*` 依赖边界 CI 通过
7. ✅ 文档 / CHANGELOG / tag 全部同步
8. ✅ 远程只剩 `main` 分支

---

## 附录 A — 依赖关系图

```
[Wave 0] refactor squash
    ↓
[Wave 1] PR-03 (schema_migrations + fields)
    ↓
[Wave 2] PR-04  PR-05  PR-06  PR-07  PR-08   (并行,unwired)
    ↓
[Wave 3] PR-09 (DB gate + write shim, 高风险)
    ↓
[Wave 4] PR-10 (Orchestrator 集成 + UI radio)
    ↓
[Wave 5] PR-11 (删 legacy) ‖ PR-12 (前端 UI mode)
    ↓
[收尾] 分支清理 + tag v0.5.5
```

## 附录 B — 与原设计的对应关系

| 原设计 §5.2 | 本执行 spec |
|---|---|
| PR-01 | 已 merge main,不重做 |
| PR-02 | refactor squash 已含骨架,PR-03 补齐 |
| PR-03 | §5 |
| PR-04 | §6.2 |
| PR-05 | §6.3 |
| PR-06 | §6.4 |
| PR-07 | §6.5 |
| PR-08 | §6.6 |
| PR-09 | §8(方案 b) |
| PR-10 | §9 |
| PR-11 | §10.1 + §10.2 |
| PR-12 | §10.3(scope 缩减) |
