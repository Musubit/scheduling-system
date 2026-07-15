# Remove Classroom Perspective & Verify Time-Only Engine

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 删除无用的教室视角（前端），并确保 TIME_ONLY_SCHEDULING 模式在排课引擎全链路（SA/OR-Tools 双通道降级、评分、快照、日志）中正常工作。

**Architecture:** 两个独立子任务：(1) 前端教室视角删除——移除 SchedulePage 中的 classroom perspective tab、filter、store 引用、导出选项；(2) TIME_ONLY 模式端到端验证与修复——补充前端 ScoreBreakdown 类型定义（buckets/enabledDimensions），确认 OR-Tools 降级路径在 TIME_ONLY 下正常，修复 ReportPage 对 TIME_ONLY 快照的评分显示。

**Tech Stack:** Vue 3 + Pinia (frontend), Go + GORM (backend), Wails v3 bindings

## Global Constraints

- 不破坏 `ScoreSchedule()` / `ScoreBreakdown` / `ScoringContext` 的 Stable Core 接口（ROADMAP.md v0.3.x 冻结）
- 前端不重新计算评分，Single Source of Truth 在后端
- 教室视角删除仅影响前端，不修改任何 Go 后端代码
- TIME_ONLY 模式下 `resource` 维度评分为 Disabled（nil bucket），前端应隐藏而非显示 0

---

## Part A: 删除教室视角

### Task 1: 移除 SchedulePage 教室视角 tab 和 filter UI

**Files:**
- Modify: `frontend/src/views/SchedulePage.vue:48-81,105-107,389-444`

**Interfaces:**
- Consumes: `scheduleStore.perspective`, `scheduleStore.selectedClassroomId`, `scheduleStore.setPerspective()`
- Produces: 教室视角 UI 完全移除，教师/班级视角不受影响

- [ ] **Step 1: 从 perspectives 数组移除 classroom**

在 `SchedulePage.vue` 中，将 perspectives 数组从三项改为两项：

```typescript
// SchedulePage.vue:48-52
const perspectives = [
  { label: '教师', value: 'teacher' as const },
  { label: '班级', value: 'class' as const },
]
```

- [ ] **Step 2: 删除教室 filter 相关的响应式变量和函数**

删除以下代码（SchedulePage.vue）：
- `filterClassroomId` ref 声明（约第55行）
- `syncClassroom()` 函数（约第69-72行）
- `classroomOptions` computed（约第105-107行）
- `selectPerspective` 函数中对 `filterClassroomId` 的重置（约第63行的 `filterClassroomId.value = null`）

- [ ] **Step 3: 删除教室 filter 的模板 UI**

删除 `SchedulePage.vue` 模板中 `v-if="scheduleStore.perspective === 'classroom'"` 的 `<n-select>` 块（约第421-430行）。

- [ ] **Step 4: 更新 hintText computed**

移除教室视角的 hint 分支（约第124-126行）：
```typescript
// 删除:
if (scheduleStore.perspective === 'classroom' && !scheduleStore.selectedClassroomId) {
  return '请选择教室查看课表'
}
```

- [ ] **Step 5: 更新 showSchedule computed**

移除教室视角的显示分支（约第113-114行）：
```typescript
// 删除:
if (scheduleStore.perspective === 'classroom' && scheduleStore.selectedClassroomId) return true
```

- [ ] **Step 6: 验证教师和班级视角仍正常工作**

运行前端开发服务器，确认：
- 只显示"教师"和"班级"两个 tab
- 选择教师后课表正常显示
- 选择班级后课表正常显示
- 无控制台错误

- [ ] **Step 7: Commit**

```bash
git add frontend/src/views/SchedulePage.vue
git commit -m "feat(frontend): remove classroom perspective from schedule view"
```

---

### Task 2: 移除 SchedulePage 教室导出选项

**Files:**
- Modify: `frontend/src/views/SchedulePage.vue:140-222,329-349`

**Interfaces:**
- Consumes: `exportSchedule()` 函数, `combinedExportOptions` 数组
- Produces: 教室导出选项从 Excel 和 PDF 导出菜单中移除

- [ ] **Step 1: 从 exportOptions 移除 classroom**

```typescript
// SchedulePage.vue:329-333
const exportOptions = [
  { label: '按教师导出', key: 'teacher' as const },
  { label: '按班级导出', key: 'class' as const },
]
```

- [ ] **Step 2: 更新 exportSchedule 函数签名和 labelMap**

将 `exportSchedule` 的 `mode` 参数类型从 `'teacher' | 'classroom' | 'class'` 改为 `'teacher' | 'class'`，并从 `labelMap` 中移除 classroom 条目：

```typescript
async function exportSchedule(mode: 'teacher' | 'class') {
  // ...
  const labelMap = { teacher: '按教师', class: '按班级' } as const
  // ...
  const dropCol = mode === 'teacher' ? '教师' : null
  // ...（其余逻辑不变）
}
```

- [ ] **Step 3: 更新 handleExportSelect 类型**

将 `handleExportSelect` 中的类型转换更新为 `'teacher' | 'class'`。

- [ ] **Step 4: 验证导出功能**

- 确认 Excel 导出菜单只显示"按教师导出"和"按班级导出"
- 确认 PDF 导出仍正常工作
- 确认无控制台错误

- [ ] **Step 5: Commit**

```bash
git add frontend/src/views/SchedulePage.vue
git commit -m "feat(frontend): remove classroom export options"
```

---

### Task 3: 清理 schedule store 中的教室视角状态

**Files:**
- Modify: `frontend/src/stores/schedule.ts:39-72`

**Interfaces:**
- Consumes: `perspective`, `selectedClassroomId`, `setPerspective()`, `displayEntries`
- Produces: store 中不再有教室视角相关的响应式状态

- [ ] **Step 1: 更新 perspective 类型**

将 perspective ref 的类型从 `'teacher' | 'classroom' | 'class'` 改为 `'teacher' | 'class'`：

```typescript
// schedule.ts:39
const perspective = ref<'teacher' | 'class'>('teacher')
```

- [ ] **Step 2: 删除 selectedClassroomId ref**

删除 `schedule.ts` 中的 `selectedClassroomId` ref 声明（约第42行）。

- [ ] **Step 3: 删除 displayEntries 中的教室分支**

删除 `displayEntries` computed 中 `perspective.value === 'classroom'` 的分支（约第48-50行）。

- [ ] **Step 4: 更新 setPerspective 函数签名和实现**

```typescript
function setPerspective(p: 'teacher' | 'class') {
  perspective.value = p
  selectedTeacherId.value = null
  selectedClassId.value = null
}
```

- [ ] **Step 5: 从 return 对象中移除 selectedClassroomId**

从 store 的 return 语句中移除 `selectedClassroomId`。

- [ ] **Step 6: 搜索所有引用 selectedClassroomId 的文件并更新**

```bash
cd /c/Users/musubi/Desktop/scheduling-system
grep -r "selectedClassroomId" frontend/src/ --include="*.ts" --include="*.vue"
```

更新所有引用点（应只有 SchedulePage.vue 和 schedule.ts，已在前面步骤中处理）。

- [ ] **Step 7: 验证**

- 确认 TypeScript 编译无错误
- 确认前端页面正常加载
- 确认教师和班级视角切换正常

- [ ] **Step 8: Commit**

```bash
git add frontend/src/stores/schedule.ts
git commit -m "refactor(frontend): remove classroom perspective state from schedule store"
```

---

## Part B: TIME_ONLY 模式引擎验证与修复

### Task 4: 补充前端 ScoreBreakdown 类型定义（buckets + enabledDimensions）

**Files:**
- Modify: `frontend/src/types/index.ts:253-265`

**Interfaces:**
- Consumes: Go backend `ScoreBreakdown` JSON（含 `buckets`、`enabledDimensions`、`finalTotal`、`placedSessions`、`expectedSessions`、`completeness`）
- Produces: 前端 TypeScript 类型完整匹配后端返回结构

- [ ] **Step 1: 添加 ScoreBucket 和 ScoreBuckets 类型定义**

在 `frontend/src/types/index.ts` 中，在 `ScoreBreakdown` 接口之前添加：

```typescript
/** 评分桶（单个维度） */
export interface ScoreBucket {
  value: number
  max: number
  details?: Record<string, number>
}

/** 四桶评分结构（time/teacher/student/resource） */
export interface ScoreBuckets {
  time?: ScoreBucket | null
  teacher?: ScoreBucket | null
  student?: ScoreBucket | null
  resource?: ScoreBucket | null  // TIME_ONLY 下为 null
}
```

- [ ] **Step 2: 补充 ScoreBreakdown 缺失字段**

更新 `ScoreBreakdown` 接口，添加 Go 后端已返回但前端未定义的字段：

```typescript
export interface ScoreBreakdown {
  total: number
  teacherPref: number
  courseSpacing: number
  teacherDays: number
  lowFloorPref: number
  weekendAvoid: number
  pePeriodPref?: number
  studentFatigue?: number
  perCategoryMax: number
  enabledCategoryCount: number
  // v0.5.2: completeness scaling
  placedSessions?: number
  expectedSessions?: number
  completeness?: number
  finalTotal?: number
  // v0.5.5: structured buckets
  buckets?: ScoreBuckets
  enabledDimensions?: string[]
}
```

- [ ] **Step 3: 更新 SchedulingResult 类型添加 mode 字段**

确认 `SchedulingResult` 接口中的 `mode` 字段类型正确（已有，验证类型为 `SchedulingMode`）。

- [ ] **Step 4: 验证 TypeScript 编译**

```bash
cd /c/Users/musubi/Desktop/scheduling-system/frontend
npx vue-tsc --noEmit 2>&1 | head -20
```

预期：无新增类型错误。

- [ ] **Step 5: Commit**

```bash
git add frontend/src/types/index.ts
git commit -m "fix(frontend): add missing ScoreBreakdown fields (buckets, enabledDimensions, completeness)"
```

---

### Task 5: 验证 SchedulingPage 评分显示在 TIME_ONLY 模式下的正确性

**Files:**
- Modify: `frontend/src/views/SchedulingPage.vue`（如需修复）

**Interfaces:**
- Consumes: `store.result.scoreDetail`（含 `buckets`、`enabledDimensions`）
- Produces: TIME_ONLY 模式下 resource 维度评分不显示，其他维度正常

- [ ] **Step 1: 审查 SchedulingPage 中 isConstraintEnabled 函数**

确认 `isConstraintEnabled` 在 `TIME_ONLY_SCHEDULING` 模式下正确禁用 `low_floor_preference`（约第46-50行）：

```typescript
const isConstraintEnabled = (key: string) => {
  if (store.config.mode === 'TIME_ONLY_SCHEDULING' && key === 'low_floor_preference') {
    return false
  }
  return store.config.constraints.includes(key)
}
```

此逻辑已存在且正确。

- [ ] **Step 2: 审查冲突显示逻辑**

确认 `isTimeOnlyMode` computed 和 `totalConflicts` computed 正确处理 TIME_ONLY（约第54-60行）：

```typescript
const isTimeOnlyMode = computed(() => store.config.mode === 'TIME_ONLY_SCHEDULING')
const totalConflicts = computed(() => {
  if (!store.result) return 0
  const roomC = isTimeOnlyMode.value ? 0 : store.result.roomConflicts
  return store.result.teacherConflicts + roomC + store.result.classConflicts
})
```

此逻辑已存在且正确。

- [ ] **Step 3: 审查硬约束验证区域**

确认教室占用冲突在 TIME_ONLY 下不显示（约第397-402行）：

```html
<div class="verify-item" v-if="!isTimeOnlyMode">
  <!-- 教室占用冲突 -->
</div>
```

此逻辑已存在且正确。

- [ ] **Step 4: 审查评分明细显示**

确认评分明细区域在 TIME_ONLY 下正确隐藏 `low_floor_preference` 项（约第348-352行）：

```html
<div class="breakdown-item" v-if="isConstraintEnabled('low_floor_preference')">
```

此逻辑已存在且正确。

- [ ] **Step 5: 如发现任何不一致，修复并 commit**

如果审查发现任何问题，修复后：
```bash
git add frontend/src/views/SchedulingPage.vue
git commit -m "fix(frontend): ensure TIME_ONLY mode score display correctness"
```

如果一切正确，跳过此 commit。

---

### Task 6: 验证 OR-Tools 在 TIME_ONLY 模式下的降级路径

**Files:**
- Verify: `backend/services/scheduling_service.go:539-758`（tryORTools）
- Verify: `backend/services/solver_orchestrator.go`（健康检查 + 降级）

**Interfaces:**
- Consumes: `SolverOrchestrator.IsORToolsAvailable()`, `ORToolsClient.Solve()`
- Produces: OR-Tools 不可用时自动降级到 SA，TIME_ONLY 模式下传递虚拟教室

- [ ] **Step 1: 审查 tryORTools 中的教室传递逻辑**

在 `scheduling_service.go` 的 `RunScheduling` 中，确认 `solverClassrooms` 在 TIME_ONLY 下是虚拟教室，且传递给 `tryORTools`：

```go
// scheduling_service.go:164-168
solverClassrooms := classrooms
if isTimeOnly {
    solverClassrooms = buildVirtualClassroomsForTimeOnly(teachingTasks, classGroups)
    addLog(fmt.Sprintf("TIME_ONLY 使用 %d 个虚拟教室执行排课", len(solverClassrooms)))
}
```

然后在第233行：
```go
if ortoolsResult := s.tryORTools(teachingTasks, teachers, solverClassrooms, classGroups, ...); ortoolsResult != nil {
```

确认 `solverClassrooms`（虚拟教室）被传递给 OR-Tools。

- [ ] **Step 2: 审查 tryORTools 内部的教室映射**

在 `tryORTools` 中（约第573-577行），确认虚拟教室被正确映射到 `ORToolsRoom`：

```go
for _, c := range classrooms {
    input.Classrooms = append(input.Classrooms, ORToolsRoom{
        ID: c.ID, Floor: c.Floor, Capacity: c.Capacity, Type: c.RoomType, Equipment: c.Equipment,
    })
}
```

虚拟教室的 ID/Capacity/Type 都在 `buildVirtualClassroomsForTimeOnly` 中正确设置。

- [ ] **Step 3: 审查 OR-Tools 降级到 SA 的路径**

确认以下降级场景在 `tryORTools` 中都有处理：
1. `orchestrator == nil` → 返回 nil（第549行）
2. `!orchestrator.IsORToolsAvailable()` → 返回 nil（第549行）
3. `client == nil` → 返回 nil（第555行）
4. `err != nil` → 返回 nil + 日志（第666行）
5. `Status == "error"` → 返回 nil + 日志（第670行）
6. `Status == "infeasible"` → 返回 nil + 日志（第679行）
7. `len(output.Entries) == 0` → 返回 nil + 日志（第670行）

所有场景都已处理。当 `tryORTools` 返回 nil 时，`RunScheduling` 继续执行 SA 路径（第239-259行）。

- [ ] **Step 4: 审查 SA 路径在 TIME_ONLY 下的行为**

确认 SA solver 的 `SolveMultiRun` 接收 `solverClassrooms`（虚拟教室）：

```go
// scheduling_service.go:242-249
result := solver.SolveMultiRun(
    teachingTasks, teachers, solverClassrooms, classGroups,
    lockedSlots, effectiveConstraints, config.SemesterID,
    saConfig, 3, nil, nil,
)
```

SA solver 内部会使用虚拟教室进行教室占用冲突检测——这在 TIME_ONLY 下是正确的（虚拟教室容量足够大，不会产生容量冲突）。

- [ ] **Step 5: 审查评分在 TIME_ONLY 下的行为**

确认评分时 `scoreClassrooms` 在 TIME_ONLY 下为 nil：

```go
// scheduling_service.go:287-289
scoreClassrooms := classrooms
if isTimeOnly {
    scoreClassrooms = nil
}
breakdown := scorer.ScoreSchedule(saResult.Entries, teachers, scoreClassrooms, scoringCtx)
```

`ScoreSchedule` 在 `scoreClassrooms` 为 nil 时，`lowFloorPref` 评分会因 `classroomMap` 为空而返回 `maxScore`（无惩罚）。同时 `scoringCtx` 的 Mode 为 `TIME_ONLY_SCHEDULING`，`EnabledScoreDimensions()` 不包含 "resource"，所以 `low_floor_preference` 被禁用。

- [ ] **Step 6: 如发现任何不一致，记录并修复**

如果发现任何问题，修复后 commit。如果一切正确，无需 commit。

---

### Task 7: 验证快照和版本在 TIME_ONLY 模式下的正确性

**Files:**
- Verify: `backend/services/snapshot_service.go:22-65`
- Verify: `backend/services/scheduling_service.go:357-383`
- Verify: `frontend/src/views/ReportPage.vue`

**Interfaces:**
- Consumes: `ScoringContext.WithMode()`, `SnapshotService.CreateSnapshot()`
- Produces: TIME_ONLY 快照记录正确的 Mode，ReportPage 正确显示

- [ ] **Step 1: 审查快照创建时的 Mode 记录**

在 `scheduling_service.go` 中，确认 `scoringCtx` 携带正确的 Mode：

```go
// scheduling_service.go:276
scoringCtx := NewScoringContextWithExpected(effectiveConstraints, sportsCourseIDs, teachingTasks, expectedTotalSessions).WithMode(mode)
```

然后在快照创建时传递：
```go
// scheduling_service.go:358-361
_, snapErr := s.snapshots.CreateSnapshot(
    config.SemesterID, config.Scope, models.TriggerAuto, "simulated_annealing",
    saResult.Entries, teachers, classrooms, scoringCtx, saResult.ElapsedMs, result.Conflicts,
)
```

`CreateSnapshot` 内部（snapshot_service.go:39）记录 Mode：
```go
Mode: string(scoringCtx.EffectiveMode()),
```

- [ ] **Step 2: 审查 ReportPage 对 TIME_ONLY 快照的显示**

在 `ReportPage.vue` 中，`categoryDefs` 列出了所有 7 个评分维度。对于 TIME_ONLY 快照，`lowFloorPref` 字段的值会是 `maxScore`（因为评分时禁用了该约束，返回满分）。但这不会造成误导，因为：
- 快照的 `perCategoryMax` 正确反映了实际启用的约束数
- `lowFloorPref` 在 TIME_ONLY 下不会被评分（`enabled` map 中为 false）

确认 ReportPage 不需要特殊处理 TIME_ONLY 模式——评分数据本身已经是正确的。

- [ ] **Step 3: 审查快照列表中的 Mode 显示**

确认快照列表（ReportPage 的 snapshot-list）是否显示 Mode 信息。当前实现不显示 Mode，但这不是必须的——用户可以通过评分维度判断。如果需要，可以后续增强。

- [ ] **Step 4: 如发现任何不一致，修复并 commit**

如果发现任何问题，修复后 commit。如果一切正确，无需 commit。

---

### Task 8: 端到端验证 — 运行 TIME_ONLY 排课并检查全链路

**Files:** 无代码修改，纯验证

**Interfaces:**
- 验证目标：TIME_ONLY 模式下 OR-Tools 降级到 SA → 排课成功 → 评分正确 → 快照正确 → 日志完整

- [ ] **Step 1: 启动应用并进入排课页面**

```bash
cd /c/Users/musubi/Desktop/scheduling-system
wails dev
```

- [ ] **Step 2: 配置 TIME_ONLY 排课**

1. 选择排课学期
2. 勾选"关闭教室场地分配（仅排上课时间）"
3. 确认约束列表中"优先低楼层"显示为禁用状态
4. 点击"开始自动排课"

- [ ] **Step 3: 验证排课日志**

确认日志包含：
- `排课模式: TIME_ONLY（关闭教室场地分配）`
- `TIME_ONLY 使用 N 个虚拟教室执行时间排课`
- OR-Tools 降级日志（如适用）或 OR-Tools 成功日志
- `排课完成` 且无错误

- [ ] **Step 4: 验证评分显示**

确认结果面板：
- 显示"⏱ 仅时间模式"badge
- 综合评分为有效数值（0-100）
- 评分明细中不显示"优先低楼层"项
- 冲突数中教室冲突为 0
- 硬约束验证中不显示"教室占用冲突"项

- [ ] **Step 5: 验证课表显示**

1. 点击"查看课表"跳转到 SchedulePage
2. 确认教师视角正常显示课表
3. 确认班级视角正常显示课表
4. 确认教室视角 tab 不存在

- [ ] **Step 6: 验证快照和报告**

1. 进入验证报告页面
2. 确认自动生成的快照存在
3. 确认快照评分明细正确
4. 确认 PDF 导出正常

- [ ] **Step 7: 验证版本保存**

1. 回到排课页面，点击"另存为方案"
2. 进入课表中心，确认版本列表中显示新版本
3. 点击"查看"确认版本数据正确

- [ ] **Step 8: 记录验证结果**

如果所有验证通过，在此记录：
```
✅ TIME_ONLY 模式端到端验证通过
- OR-Tools 降级: [正常/已降级]
- SA 求解: [正常]
- 评分显示: [正确]
- 快照: [正确]
- 版本: [正确]
- 教室视角: [已移除]
```

如有失败项，创建对应的修复 task。

---

## Task Dependency Graph

```
Task 1 (删除教室视角 UI)
  └─→ Task 2 (删除教室导出)
       └─→ Task 3 (清理 store)

Task 4 (补充 TS 类型)
  └─→ Task 5 (验证 SchedulingPage 评分)
       └─→ Task 8 (端到端验证)

Task 6 (验证 OR-Tools 降级) ─→ Task 8
Task 7 (验证快照/版本) ─→ Task 8
```

Task 1-3 (Part A) 和 Task 4-7 (Part B) 可以并行执行。Task 8 需要所有前置 task 完成。
