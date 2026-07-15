# 稳定最优排课 — 四项优化实现方案

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让排课结果更稳定、更可预期——跨次运行只升不降、自适应收敛、后优化确定性、评分无浮点噪声。

**Architecture:** 四个独立优化分别改动四个不同文件，零依赖，可完全并行。每个优化自含测试。

**Tech Stack:** Go 1.22, GORM v2, SQLite, `go test`

## Global Constraints

- 不破坏 Stable Core 冻结（`ScoreSchedule` 签名不变）
- 所有新增字段向后兼容（zero-value = 旧行为）
- `go test ./backend/services/ -v -count=1` 全部通过
- `go build ./...` 无编译错误

---

## 文件结构

| 文件 | 改动 |
|------|------|
| `backend/services/scheduling_service.go` | Task 1: 跨次最优缓存 |
| `backend/services/sa_solver.go` | Task 2: 自适应轮数 |
| `backend/services/sa_postopt.go` | Task 3: 确定性 PostOptimize |
| `backend/services/scoring_service.go` | Task 4: 评分 epsilon |
| `backend/services/stable_scheduling_test.go` | 新建：四项优化的测试 |

---

## Task 1: 跨次运行保留最优解

**Files:**
- Modify: `backend/services/scheduling_service.go`
- Test: `backend/services/stable_scheduling_test.go`

**Interfaces:**
- Produces: `SchedulingService.bestCachedScore float64`, `SchedulingService.bestCachedEntries []models.ScheduleEntry`, `SchedulingService.bestCachedResult *SchedulingResult`
- Consumes: `SAResult.Score`, `SAResult.Entries`

**原理:** 在 `SchedulingService` 上缓存上次最优结果。新运行的分数 <= 缓存分数时，返回缓存结果。分数更高时更新缓存。

- [ ] **Step 1: 添加缓存字段**

在 `backend/services/scheduling_service.go` 的 `SchedulingService` 结构体中添加：

```go
type SchedulingService struct {
    db                database.DB
    // ... 现有字段 ...

    // 跨次最优缓存
    bestCachedScore   float64
    bestCachedEntries []models.ScheduleEntry
    bestCachedResult  *SchedulingResult
}
```

- [ ] **Step 2: 在 RunScheduling 中实现缓存逻辑**

在 `RunScheduling` 中，`ScoreSchedule` 计算完成后、写入数据库之前，添加缓存判断：

```go
// 统一评分后
breakdown := scorer.ScoreSchedule(saResult.Entries, teachers, scoreClassrooms, scoringCtx)
saResult.Score = breakdown.FinalTotal
result.Score = breakdown.FinalTotal
result.ScoreDetail = &breakdown

// === 跨次最优缓存 ===
if s.bestCachedResult != nil && breakdown.FinalTotal <= s.bestCachedScore {
    // 本次没有超越缓存，返回缓存结果
    *result = *s.bestCachedResult
    result.Logs = append(result.Logs, fmt.Sprintf("本次评分 %.1f ≤ 缓存最优 %.1f，返回缓存结果", breakdown.FinalTotal, s.bestCachedScore))
    return *result
}
// 本次更优，更新缓存
s.bestCachedScore = breakdown.FinalTotal
s.bestCachedEntries = make([]models.ScheduleEntry, len(saResult.Entries))
copy(s.bestCachedEntries, saResult.Entries)
// result 会在最终赋值后缓存
```

在 `RunScheduling` 的 return 之前，缓存最终 result：

```go
// 缓存最终结果（深拷贝 ScoreDetail）
cached := *result
if result.ScoreDetail != nil {
    sd := *result.ScoreDetail
    cached.ScoreDetail = &sd
}
s.bestCachedResult = &cached
```

- [ ] **Step 3: 写测试**

在 `backend/services/stable_scheduling_test.go` 中：

```go
func TestCrossRunCachePreservesBest(t *testing.T) {
    // 构造两个不同分数的 SchedulingResult
    // 第一次调用 RunScheduling → 缓存分数 600
    // 第二次调用 RunScheduling，模拟分数 580 → 应返回缓存的 600
    // 第三次调用 RunScheduling，模拟分数 620 → 应更新缓存为 620
}
```

- [ ] **Step 4: 运行测试**

Run: `go test ./backend/services/ -run TestCrossRunCache -v -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/services/scheduling_service.go backend/services/stable_scheduling_test.go
git commit -m "feat: cross-run best result caching — results only improve"
```

---

## Task 2: 自适应轮数（收敛检测）

**Files:**
- Modify: `backend/services/sa_solver.go`
- Test: `backend/services/stable_scheduling_test.go`

**Interfaces:**
- Produces: `SolveMultiRun` 签名不变，内部行为变化
- Consumes: `SAResult.Score`

**原理:** 在 `SolveMultiRun` 循环中，连续 K 轮（默认 2）无改善则提前停止。不改变函数签名，只改变内部循环退出条件。

- [ ] **Step 1: 添加常量**

在 `backend/services/sa_solver.go` 顶部添加：

```go
const (
    defaultPatienceRuns = 2 // 连续无改善轮数阈值
)
```

- [ ] **Step 2: 修改 SolveMultiRun 循环**

在 `SolveMultiRun` 的 outer for 循环中，跟踪 bestScore 和无改善计数：

```go
func (s *SASolver) SolveMultiRun(
    // ... 参数不变 ...
) *SAResult {
    if runs <= 0 {
        runs = 3
    }
    timePerRun := config.MaxTimeSeconds / float64(runs)

    var bestResult *SAResult
    var bestScore float64
    noImproveCount := 0

    for i := 0; i < runs; i++ {
        select {
        case <-cancelCh:
            if bestResult != nil {
                return bestResult
            }
            return &SAResult{Entries: []models.ScheduleEntry{}}
        default:
        }

        runConfig := config
        runConfig.MaxTimeSeconds = timePerRun
        runConfig.Seed = time.Now().UnixNano() + int64(i*31337)

        result := s.Solve(teachingTasks, teachers, classrooms, classGroups,
            lockedSlots, constraints, semesterID, runConfig, nil, nil)

        if result == nil {
            continue
        }

        if bestResult == nil || result.Score > bestScore {
            bestResult = result
            bestScore = result.Score
            noImproveCount = 0
        } else {
            noImproveCount++
            if noImproveCount >= defaultPatienceRuns {
                break // 连续无改善，提前停止
            }
        }
    }

    if bestResult == nil {
        return &SAResult{Entries: []models.ScheduleEntry{}}
    }
    return bestResult
}
```

- [ ] **Step 3: 写测试**

```go
func TestAdaptiveRunStopsEarly(t *testing.T) {
    // Mock Solve 返回固定分数（模拟收敛）
    // 调用 SolveMultiRun(runs=10)
    // 验证实际调用 Solve 的次数 < 10（提前停止了）
}
```

- [ ] **Step 4: 运行测试**

Run: `go test ./backend/services/ -run TestAdaptiveRun -v -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/services/sa_solver.go backend/services/stable_scheduling_test.go
git commit -m "feat: adaptive run count — stop early on convergence"
```

---

## Task 3: 确定性 PostOptimize

**Files:**
- Modify: `backend/services/sa_postopt.go`
- Test: `backend/services/stable_scheduling_test.go`

**Interfaces:**
- Produces: `PostOptimize` 签名不变
- Consumes: 无外部依赖

**原理:** 当前 PostOptimize 用 `rng.Perm(7)` 随机打乱星期顺序遍历。改为顺序遍历 0-6，使结果完全确定性。同一个输入 → 同一个输出。

- [ ] **Step 1: 修改星期遍历**

在 `backend/services/sa_postopt.go` 中找到 `rng.Perm(7)` 的使用（约第 250 行），替换为顺序遍历：

```go
// 之前:
// dayOrder := rng.Perm(7)
// for _, day := range dayOrder {

// 之后:
for day := 0; day < 7; day++ {
```

- [ ] **Step 2: 检查 rng 的其他用途**

确认 `rng` 在 PostOptimize 中是否还有其他随机用途。如果有，同样改为确定性逻辑。如果 `rng` 不再使用，删除相关变量声明。

- [ ] **Step 3: 写测试**

```go
func TestPostOptimizeDeterministic(t *testing.T) {
    // 构造固定的 entries 列表
    // 调用 PostOptimize 两次，相同输入
    // 验证两次输出完全一致（逐 entry 比较 day/period/room）
}
```

- [ ] **Step 4: 运行测试**

Run: `go test ./backend/services/ -run TestPostOptimizeDeterministic -v -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/services/sa_postopt.go backend/services/stable_scheduling_test.go
git commit -m "feat: deterministic PostOptimize — same input always produces same output"
```

---

## Task 4: 评分 Epsilon（浮点噪声消除）

**Files:**
- Modify: `backend/services/scoring_service.go`
- Test: `backend/services/stable_scheduling_test.go`

**Interfaces:**
- Produces: `ScoreEpsilon float64` 常量，`ScoreEquals(a, b float64) bool` 函数
- Consumes: 被 Task 1/2/3 的比较逻辑使用

**原理:** 当前所有评分比较用严格 `>` / `<`。浮点累加误差可能导致 587.9999999 和 588.0000001 被视为不同。添加 epsilon 阈值，差值 < epsilon 视为相等。

- [ ] **Step 1: 添加常量和工具函数**

在 `backend/services/scoring_service.go` 顶部添加：

```go
const ScoreEpsilon = 0.01

// ScoreEquals 判断两个分数是否在 epsilon 范围内相等
func ScoreEquals(a, b float64) bool {
    diff := a - b
    if diff < 0 {
        diff = -diff
    }
    return diff < ScoreEpsilon
}

// ScoreGreater 判断 a 是否显著大于 b
func ScoreGreater(a, b float64) bool {
    return a > b+ScoreEpsilon
}
```

- [ ] **Step 2: 替换 sa_solver.go 中的比较**

在 `sa_solver.go` 中，将 `result.Score > bestResult.Score` 替换为 `scoring.ScoreGreater(result.Score, bestResult.Score)`。

SA 主循环中的 `currentScore > bestScore` 同样替换。

注意：`delta > 0` 的比较保持不变（这是 SA 接受准则，不是分数比较）。

- [ ] **Step 3: 替换 sa_postopt.go 中的比较**

将 `finalBreakdown.FinalTotal < baselineScore` 替换为 `scoring.ScoreGreater(baselineScore, finalBreakdown.FinalTotal)`。

- [ ] **Step 4: 替换 scheduling_service.go 中的比较**

将 `breakdown.FinalTotal <= s.bestCachedScore` 替换为 `!scoring.ScoreGreater(breakdown.FinalTotal, s.bestCachedScore)`。

- [ ] **Step 5: 写测试**

```go
func TestScoreEpsilon(t *testing.T) {
    assert.True(t, ScoreEquals(588.0, 588.005))
    assert.True(t, ScoreEquals(588.0, 587.995))
    assert.False(t, ScoreEquals(588.0, 588.02))

    assert.True(t, ScoreGreater(588.02, 588.0))
    assert.False(t, ScoreGreater(588.005, 588.0))
    assert.False(t, ScoreGreater(587.0, 588.0))
}
```

- [ ] **Step 6: 运行全部测试**

Run: `go test ./backend/services/ -v -count=1`
Expected: 全部 PASS

- [ ] **Step 7: Commit**

```bash
git add backend/services/scoring_service.go backend/services/sa_solver.go backend/services/sa_postopt.go backend/services/scheduling_service.go backend/services/stable_scheduling_test.go
git commit -m "feat: score epsilon — eliminate floating-point noise in comparisons"
```

---

## 验收标准

| # | 检查项 | 命令 / 方法 |
|---|--------|-------------|
| 1 | Go 编译通过 | `go build ./...` |
| 2 | 全部测试通过 | `go test ./backend/services/ -v -count=1` |
| 3 | 跨次缓存：第二次排课分数不低于第一次 | 手动：连续点两次排课，第二次评分 ≥ 第一次 |
| 4 | 自适应轮数：收敛后提前停止 | 日志中应显示实际运行轮数 < 配置轮数 |
| 5 | 确定性 PostOptimize：相同输入相同输出 | 测试 `TestPostOptimizeDeterministic` 通过 |
| 6 | Epsilon 消除噪声 | 测试 `TestScoreEpsilon` 通过 |
| 7 | 前端 build 通过 | `cd frontend && npm run build` |
