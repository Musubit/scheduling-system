# Triage Hardening: 从对抗审查中筛选出的实用修复

**Goal:** 从对抗性审查报告中筛选出对单机离线单用户应用真正有价值的缺陷并修复。
**Architecture:** 仅修改 Go 后端 + 少量前端。不涉及 Flask 鉴权、编译加固等单用户场景不适用的项目。
**Spec:** 对抗性审查报告 V-01 ~ V-26（筛选后 12 项）
**Tech:** Go (Wails v3 services), SQLite (GORM v2)
**QA:** `go test ./backend/services/ -v -count=1` 全部通过；手动验证 MoveEntry 边界、双击保护

## 筛选标准

单机离线单用户 → 排除：网络鉴权(V-05)、多用户竞争(V-07)、迁移标记篡改(V-02)、路径穿越(V-04)、编译加固(V-26)、日志脱敏(V-23)、种子不可预测(V-16)等。

保留的核心逻辑：**panic 会杀进程、数据校验缺失会写坏 DB、UI 行为与后端不一致会导致困惑。**

---

## Task 1: [P0] RunScheduling 并发保护 + panic 恢复
**Files:** `backend/services/scheduling_service.go`
**Depends:** none
**Details:**
1. 加 `var runMu sync.Mutex` 和 `var schedulingRunning atomic.Bool`：
   ```go
   func (s *SchedulingService) RunScheduling(...) SchedulingResult {
       if !s.schedulingRunning.CompareAndSwap(false, true) {
           return SchedulingResult{Error: "排课任务已在运行"}
       }
       defer s.schedulingRunning.Store(false)
       s.runMu.Lock()
       defer s.runMu.Unlock()
       // ... existing logic
   }
   ```
2. 在 `RunScheduling` 入口加 panic 恢复：
   ```go
   defer func() {
       if r := recover(); r != nil {
           result.Error = fmt.Sprintf("排课引擎异常: %v", r)
           log.Printf("PANIC recovered in RunScheduling: %v\n%s", r, debug.Stack())
       }
   }()
   ```
3. 同样在 `MoveService.MoveEntry`、`MoveService.MoveEntryAndScore` 入口加 panic 恢复。

**QA:** `go test ./backend/services/ -run TestScheduling -v`；手动快速双击"开始排课"验证不重复执行。

---

## Task 2: [P0] MoveEntry 强制校验 + 事务保护
**Files:** `backend/services/move_service.go`
**Depends:** none
**Details:**
1. 在 `MoveEntry` 开头内联参数校验（不依赖前端调 CheckMove）：
   ```go
   if req.NewDay < 0 || req.NewDay > 6 {
       return MoveResult{Error: "无效星期"}
   }
   if req.NewPeriod < 0 || req.NewPeriod > 10 {
       return MoveResult{Error: "无效节次"}
   }
   if req.NewSpan < 1 || req.NewSpan > 3 {
       return MoveResult{Error: "无效跨度"}
   }
   if req.NewPeriod + req.NewSpan > 11 {
       return MoveResult{Error: "节次+跨度超出范围"}
   }
   ```
2. 将 `db.Save(&entry)` + `computeScore` 包裹在 `db.Transaction` 中，失败自动回滚。
3. 同样校验 `MoveEntryAndScore`。

**QA:** `go test ./backend/services/ -run TestMove -v`；尝试传入 NewDay=99 验证被拒绝。

---

## Task 3: [P0] TIME_ONLY 合成教室 ID 改为不落库
**Files:** `backend/services/scheduling_service.go`, `backend/models/schedule_entry.go`
**Depends:** none
**Details:**
1. 在 `ScheduleEntry` 模型加 `IsVirtual bool` 字段：
   ```go
   IsVirtual bool `gorm:"default:false" json:"isVirtual"`
   ```
2. 修改 `applySyntheticClassroomIDsForTimeOnly`：不再生成 1,000,000+ 的假 ID，而是设置 `IsVirtual=true` + 保留原始 ClassroomID（或 0）。
3. 修改前端展示逻辑：对 `IsVirtual` 条目显示"虚拟教室"而非空白。
4. 修改评分逻辑：`IsVirtual` 条目跳过 LowFloorPref 等教室相关评分维度。

**QA:** `go test ./backend/services/ -run TestTimeOnly -v`；前端排课后检查课表不显示空白教室。

---

## Task 4: [P1] SolveMultiRun 真正取消
**Files:** `backend/services/sa_solver.go`
**Depends:** none
**Details:**
```go
outer:
for i := 0; i < runs; i++ {
    select {
    case <-cancelCh:
        break outer
    default:
    }
    // ...
}
if bestResult == nil {
    return &SAResult{Entries: []models.ScheduleEntry{}}
}
```

**QA:** `go test ./backend/services/ -run TestLocked -v`；手动在排课过程中点取消。

---

## Task 5: [P1] UnavailableSlots + ConstraintWeights 值域校验
**Files:** `backend/services/teaching_task_service.go`, `backend/services/scheduling_service.go`
**Depends:** none
**Details:**
1. `UpdateTeacher` / `CreateTeacher` 中校验 `UnavailableSlots` JSON：
   ```go
   if t.UnavailableSlots != "" {
       var slots []LockedTimeSlot
       if err := json.Unmarshal([]byte(t.UnavailableSlots), &slots); err != nil {
           return fmt.Errorf("UnavailableSlots 格式错误: %w", err)
       }
       for _, s := range slots {
           if s.DayOfWeek > 6 || s.StartPeriod > 10 || s.Span < 1 || s.Span > 3 ||
               int(s.StartPeriod)+int(s.Span) > 11 {
               return fmt.Errorf("非法时段: day=%d start=%d span=%d", s.DayOfWeek, s.StartPeriod, s.Span)
           }
       }
   }
   ```
2. `RunScheduling` 中校验 `ConstraintWeights`：
   ```go
   for k, v := range config.ConstraintWeights {
       if v < 0 { config.ConstraintWeights[k] = 0 }
       if v > 100 { config.ConstraintWeights[k] = 100 }
   }
   ```

**QA:** `go test ./backend/services/ -v`；手动传入负权重验证被 clamp。

---

## Task 6: [P1] ImportTeachingTasks 行数/列长限制
**Files:** `backend/services/teaching_task_service.go`
**Depends:** none
**Details:**
1. 导入开头加：
   ```go
   if len(rows) > 5000 {
       return ImportResult{Error: "导入行数超过上限 5000"}
   }
   ```
2. 每个单元格校验 `len(cell) > 256` 则截断并记录警告。
3. `fmt.Sscanf` 错误不再忽略：
   ```go
   if _, err := fmt.Sscanf(v, "%d", &totalHours); err != nil {
       totalHours = 0 // will fallback to course.Hours
   }
   ```

**QA:** 构造 >5000 行 CSV 验证被拒绝。

---

## Task 7: [P1] innerHTML 改 textContent
**Files:** `frontend/src/views/SchedulePage.vue`, `frontend/src/views/ReportPage.vue`
**Depends:** none
**Details:**
将 PDF 导出中的 `container.innerHTML = headerHtml` 改为 DOM API 构建：
```js
// SchedulePage.vue 和 ReportPage.vue
const titleEl = document.createElement('div')
titleEl.className = 'export-header-title'
titleEl.textContent = title
container.appendChild(titleEl)
```

**QA:** 手动导出 PDF，教师名含 `<script>` 标签时不执行。

---

## Task 8: [P1] HTTP 超时对齐 + OR-Tools 结果二次校验
**Files:** `backend/services/ortools_client.go`, `backend/services/scheduling_service.go`
**Depends:** none
**Details:**
1. `ortools_client.go` 动态超时：
   ```go
   timeout := time.Duration(input.TimeLimitSeconds+30) * time.Second
   if timeout < 150*time.Second { timeout = 150 * time.Second }
   client := &http.Client{Timeout: timeout}
   ```
2. OR-Tools 结果校验（在 `tryORTools` 写库前）：
   ```go
   for _, e := range output.Entries {
       if e.DayOfWeek > 6 || e.StartPeriod > 10 || e.Span < 1 || e.Span > 3 ||
           int(e.StartPeriod)+int(e.Span) > 11 {
           return nil, fmt.Errorf("OR-Tools 返回非法 entry: day=%d start=%d span=%d",
               e.DayOfWeek, e.StartPeriod, e.Span)
       }
   }
   ```

**QA:** `go test ./backend/services/ -v`。

---

## Task 9: [P1] SA span=0 兜底 + PostOptimize nil 指针保护
**Files:** `backend/services/sa_solver.go`, `backend/services/sa_postopt.go`
**Depends:** none
**Details:**
1. SA 加载 entry 后校验 span：
   ```go
   for i := range entries {
       if entries[i].Span < 1 || entries[i].Span > 3 {
           entries[i].Span = 2
       }
   }
   ```
2. PostOptimize 中 `buildOcc` 加 nil 保护：
   ```go
   if e.TeachingTaskID == nil {
       continue
   }
   ```
3. 清理 `sa_solver.go` 中 `rebuildFromEntries` 的死代码调用（V-22）。

**QA:** `go test ./backend/services/ -run TestDelta -v`。

---

## Task 10: [P1] MoveEntry 事务包裹
**Files:** `backend/services/move_service.go`
**Depends:** Task 2
**Details:**
将 `MoveEntryAndScore` 中的 Save + computeScore 放入事务：
```go
err := db.Transaction(func(tx *gorm.DB) error {
    if err := tx.Save(&entry).Error; err != nil { return err }
    // computeScore within transaction context
    return nil
})
if err != nil {
    return MoveResult{Error: "移动失败: " + err.Error()}
}
```

**QA:** `go test ./backend/services/ -run TestMove -v`。

---

## Final Checklist

- [ ] `go test ./backend/services/ -v -count=1` — 全部通过
- [ ] `go build ./...` — 无编译错误
- [ ] 手动：快速双击"开始排课" → 不重复执行
- [ ] 手动：拖拽到无效位置 → 被拒绝
- [ ] 手动：TIME_ONLY 排课 → 教室显示正常
- [ ] 手动：导入 5001 行 CSV → 被拒绝
