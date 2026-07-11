# v0.3.0 — 排课质量感知与用户体验增强版

## 版本定位

继 v0.2.0 稳定化发布后，v0.3.0 聚焦于 **排课质量的可见性、可解释性、可优化性** 三个维度，
不做大规模重构，不引入新的求解器复杂度。

## 范围

### ✅ 保留

| 模块 | 内容 | 状态 |
|------|------|------|
| Solver 稳定性 | `lastNeighbor` 从包级变量迁移到 `schedulingContext` | ✅ 完成 |
| 排课质量报告 | ReportPage 增加质量分析展示 | 待开发 |
| 排课质量优化 | `course_dispersed` 增加周内均衡维度 | 待开发 |
| 教师负载分析 | 后处理评分展示，**不影响求解** | 待开发 |
| 过程反馈 | 排课进度增加阶段化标签 | 待开发 |
| 资源一致性 | 统一教室类型判断逻辑 | 待开发 |

### ❌ 推迟到 v0.4.0

- Backup/Restore 重构
- 前端导航状态统一（AppSidebar vs app.ts）
- DEFAULT_LOCKED 配置统一
- fuzzyFilter 类型优化
- ScoringService 构造统一
- 包级状态进一步隔离

---

## 任务拆分

### Task 1: 统一教室类型判断 ⭐⭐⭐

**文件：**
- `backend/services/sa_initial.go`（行 46-53，内联判断）
- `backend/services/sa_solver.go`（行 423-434，`getRequiredRoomType`）
- `backend/services/sa_postopt.go`（行 293-304，`roomTypeForCourse`）
- `backend/services/scheduling_service.go`（行 452-459，`containsKeyword` 内联）

**问题：** 4 处教室类型判断逻辑重复，且 `scheduling_service.go` 使用 `containsKeyword`（大小写不敏感），SA 文件使用 `strings.Contains`（大小写敏感）。

**方案：**
1. 新建 `backend/services/roomtype.go`，定义包级函数：
   ```go
   // IsSportsRoom 判断是否为体育课专用教室（体育馆/操场）
   func IsSportsRoom(roomName string) bool
   // IsLabRoom 判断课程名是否含"实验" → 需要实验室
   func IsLabCourse(courseName string) bool
   // IsComputerRoom 判断课程名是否含"上机" → 需要机房
   func IsComputerCourse(courseName string) bool
   ```
2. 三个函数各自内部使用 `strings.Contains(strings.ToLower(s), keyword)` 做大小写不敏感匹配
3. 调用方按各自上下文组合调用（体育用 `IsSportsRoom` 判断教室，实验/上机用 `IsLabCourse`/`IsComputerCourse` 判断课程名）
4. **不修改教室分配规则**（体育→体育馆，含"实验"→实验室，含"上机"→机房）

**验收：** `go build` 通过，SA 求解行为不变

---

### Task 2: ReportPage 质量分析展示 ⭐⭐⭐⭐

**文件：**
- `frontend/src/views/ReportPage.vue`
- `frontend/src/types/index.ts`（ScoreBreakdown 类型）

**目标：** 将现有分项评分数据可视化，增加"总分 + 分项星级 + 问题提示"三层展示。

**方案：**
1. 在已有 `ScoreCard` 基础上，增加星级评分行（每个分项 5 星，0.0-1.0 映射）
2. 计算方法：
   - 每个分类的 `starRating = Math.round((categoryScore / perCategoryMax) * 5 * 2) / 2`
   - 渲染为 ★★★★☆ 形式
3. 增加问题提示区：当某分类分数低于阈值（如 <60%）时，显示改善建议
4. 不修改后端数据结构，纯前端渲染增强

**验收：** `npm run build` 通过，快照详情正确展示星级和问题提示

---

### Task 3: course_dispersed 周内均衡增强 ⭐⭐⭐⭐⭐

**文件：**
- `scheduler/solver.py`（SC4 约束块，行约 408-448）
- `backend/services/scoring_service.go`（`scoreCourseSpacing` 方法，行约 189-267）

**目标：** 在现有日期间隔评分基础上，增加周内每日课程数量均衡度惩罚。

**方案（solver.py）：**
1. 在 SC4 约束块中，在现有 `consecutive` 惩罚之后，新增周内均衡惩罚：
   ```python
   # Daily balance: penalize when max daily sessions far exceeds average
   avg_sessions = total_sessions / len(occupied_days)  # float
   excess = max_daily_sessions - ceil(avg_sessions)
   if excess > 0:
       objective_terms.append(excess * (-w_balance))
   ```
2. 权重 `w_balance = int(w * 0.2)`（约为 course_dispersed 总权的 20%，低于日期间隔权重）
3. constraint key 保持 `course_dispersed` 不变

**方案（scoring_service.go）：**
1. 在现有 `gapScore * (1.0 - concentrationPenalty)` 之后，增加 `dailyBalancePenalty`
2. 计算：`avgSessions = totalSessions / len(days)`，`excess = maxDailySessions - ceil(avgSessions)`
3. 惩罚系数：`balancePenalty = float64(excess) * 0.15`
4. 最终 `courseScore = gapScore * (1.0 - concentrationPenalty - balancePenalty)`，下限截断为 0
5. 保持 `scoreCourseSpacing` 的整体结构不变

**验收：**
- OR-Tools 和 SA 求解器均产生更均衡的排课结果
- `go build` + Python 语法验证通过
- 前端约束配置无需变化（key 不变）

---

### Task 4: 教师负载分析（后处理） ⭐⭐⭐

**文件：**
- `backend/services/scoring_service.go`（新增方法）
- `backend/models/score_breakdown.go` 或扩展 `ScoreBreakdown`
- `frontend/src/views/ReportPage.vue`
- `frontend/src/types/index.ts`

**目标：** 在 ReportPage 展示教师工作量分析，**不参与优化**（仅后处理评分）。

**方案（Go）：**
1. 新增方法 `AnalyzeTeacherWorkload(entries, teachers, classGroups)` 返回 `[]TeacherWorkloadInfo`
2. `TeacherWorkloadInfo` 结构：
   ```go
   type TeacherWorkloadInfo struct {
       TeacherID    uint
       TeacherName  string
       TotalSessions int
       MaxDaily     int    // 最高单日课时
       MinDaily     int    // 最低单日课时（排除0课日）
       BalanceScore float64 // 0-100 均衡评分
       BusyDays     int    // 有课天数
       Suggestion   string // 改善建议
   }
   ```
3. 均衡评分算法：`(1 - (maxDaily - minDaily) / max(maxDaily, 1)) * 100`
4. 在 `CreateSnapshot` 时调用，结果存入 `SnapshotDetail` 的 summary 字段或新增 teacher_workload 类型
5. **不调用 `scoreTeacherWorkload()` 影响 `Total` 分**

**方案（前端）：**
1. ReportPage 新增"教师负载分析"卡片
2. 展示表格：教师名、总课时、最高单日、最低单日、均衡评分、建议
3. 均衡评分低（<70）的行高亮警告色

**验收：**
- ReportPage 正确展示教师负载数据
- 排课结果（solver 输出）与不开启此分析时**完全一致**

---

### Task 5: 排课过程阶段反馈 ⭐⭐⭐⭐

**文件：**
- `backend/services/scheduling_service.go`（`RunScheduling` + 新增 `ScheduleProgress` 类型）
- `frontend/src/stores/scheduling.ts`（`startScheduling` 状态监听）
- `frontend/src/views/SchedulingPage.vue`（进度区域模板）

**目标：** 让用户看到排课当前处于哪个阶段，使用结构化数据而非日志解析。

**方案（后端）：**
1. 新增 `ScheduleProgress` 类型：
   ```go
   type ScheduleProgress struct {
       Progress int    `json:"progress"` // 0-100
       Stage    string `json:"stage"`    // 阶段标签
   }
   ```
2. 在 `SchedulingResult` 中嵌入 `ProgressHistory []ScheduleProgress` 字段
3. 在 `RunScheduling` 的关键节点追加阶段记录：
   ```go
   result.ProgressHistory = append(result.ProgressHistory, ScheduleProgress{10, "加载教学任务"})
   result.ProgressHistory = append(result.ProgressHistory, ScheduleProgress{20, "初始化求解器"})
   result.ProgressHistory = append(result.ProgressHistory, ScheduleProgress{40, "生成排课方案"})
   result.ProgressHistory = append(result.ProgressHistory, ScheduleProgress{70, "优化冲突"})
   result.ProgressHistory = append(result.ProgressHistory, ScheduleProgress{90, "保存结果"})
   result.ProgressHistory = append(result.ProgressHistory, ScheduleProgress{100, "排课完成"})
   ```
4. **不修改求解算法内部逻辑**

**方案（前端）：**
1. 调度 store 的 `startScheduling` 中，从返回的 `goResult.progressHistory` 驱动进度更新
2. 在 SchedulingPage 进度区域，使用 `NSteps` 展示当前阶段（current step = progress/20 取整）
3. 阶段标签从 `progressHistory` 最后一项的 `stage` 字段读取
4. 保留现有 `NProgress` 进度条和进度百分比

**验收：**
- 排课过程中 NSteps 展示 5 个阶段依次推进
- 进度条和阶段标签同步更新
- `npm run build` + `go build` 通过

---

## 依赖关系

```
Task 1 (教室类型统一)
  │
  │  无依赖，可并行
  │
Task 2 (ReportPage 展示)
  │
  ├──→ Task 4 (教师负载分析) — 依赖 Task 2 的 ReportPage 结构
  │
Task 3 (周内均衡) — 无依赖，可并行
  │
Task 5 (过程反馈) — 无依赖，可并行
```

## 建议执行顺序

```
第一轮（并行）
├── Task 1: 教室类型统一
└── Task 3: 周内均衡增强

第二轮
└── Task 2: ReportPage 质量展示

第三轮
├── Task 4: 教师负载分析
└── Task 5: 过程反馈

第四轮
└── 集成测试 + 版本发布
```

## 风险矩阵

| 任务 | 风险 | 缓解 |
|------|------|------|
| Task 1 | 大小写敏感性变更可能影响教室分配 | 统一为小写比较，回归测试 |
| Task 2 | 星级计算除零风险 | `perCategoryMax > 0` 守卫 |
| Task 3 | OR-Tools 新增惩罚项可能降低其他维度得分 | w=30% 的低权重，可调 |
| Task 4 | 新 ScoreBreakdown 字段破坏前端序列化 | Go JSON tag 保持兼容 |
| Task 5 | 日志格式变更影响现有日志展示 | 仅增加，不修改现有格式 |
