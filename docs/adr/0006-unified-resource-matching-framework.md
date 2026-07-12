# ADR-0006: 统一资源匹配框架 (Unified Resource Matching Framework)

## 状态

已采纳

## 上下文

v0.5.2 及之前，教室类型匹配逻辑散落在 7 个文件、5 处重复的 if-else 链中，全部基于课程名关键字推断（"实验"→实验室、"上机"→机房、"体育"→体育馆）。存在以下问题：

1. **5 处重复** — `sa_initial.go`、`sa_neighbors.go`、`sa_postopt.go`、`sa_solver.go`、`scheduling_service.go` 各有一份相同的 if-else 链
2. **MoveService 遗漏** — `CheckMove` 不校验教室类型，调课时可将实验课拖到普通教室
3. **Go/Python 不同步** — Go 侧和 Python `solver.py` 各自维护一套匹配规则
4. **不可扩展** — 新增教室类型需修改多处代码

v0.5.3 需要建立统一的资源约束能力（What can be scheduled），作为排课系统能力边界的升级。

## 决策

**采用统一资源匹配框架 (URMF)，以包级纯函数形式实现，Go 侧为唯一匹配真相源。**

### 核心决策

| # | 决策 | 理由 |
|---|------|------|
| 1 | ResourceMatcher 为包级纯函数集 | 无状态 = 可并发 = 可测试 = 无副作用；SA / OR-Tools / MoveService / 诊断系统均可直接调用 |
| 2 | Match() 返回 MatchResult 而非 bool | 诊断信息一次生成（Code + Reason + MissingEquipment），调课失败提示、排课诊断、日志直接复用，避免 bool→再判断的二次开销 |
| 3 | Go 侧通过 AllowedRooms() 预计算可用教室列表 | 单一真相源；Python 只读 `allowedRoomIds`，不做任何匹配/推断；彻底消除 Go/Python 两端不同步风险 |
| 4 | ResourceMatchCode 用 int 枚举 | switch 友好，日志友好，避免 string 拼写错误 |
| 5 | RoomType / CourseCategory 常量集中定义在 `room_types.go` | 新增类型只改一处，不碰匹配逻辑 |
| 6 | EquipmentSet 封装设备集合 | `Has()` / `ContainsAll()` 语义化查询，避免散落的 for 循环字符串比较 |
| 7 | 名称推断保留为 fallback 但标记 Deprecated | 旧数据零迁移成本；v0.6.0 Migration 补全 Category 后移除 |
| 8 | ExplainRequirement() 返回纯结构化数据 | 不含展示文案；调用方自行组装；为排课诊断、调课提示、日志预留结构化接口 |
| 9 | MoveService 直接调 Match() | 不自建任何判断逻辑，避免第三个匹配实现 |
| 10 | ResourceMatcher 标记为 V1 | 未来扩展（校区、教师资质等）可演进为 V2，文档和诊断中一眼可知基于哪套规则 |

### 架构

```
┌──────────────────────────────────────────────────────────┐
│              resource_matcher.go (无状态, V1)              │
│                                                          │
│  Match(task, course, room) → MatchResult                 │
│  InferRoomType(task, course) → string                    │
│  ExplainRequirement(task, course) → ResourceRequirement  │
│  ExplainMismatch(result) → string                        │
│  IsSharedVenue(room) → bool                              │
│  AllowedRooms(task, course, rooms) → []Classroom         │
│                                                          │
│  不查 DB · 不评分 · 不记日志 · 不修改数据 · 不持有状态     │
├──────────────────────────────────────────────────────────┤
│  调用方                                                    │
│  ├─ SA: Match() 过滤候选教室 + IsSharedVenue()            │
│  ├─ SchedulingService: AllowedRooms() 预计算 → JSON       │
│  ├─ OR-Tools (Python): 只读 allowedRoomIds，不做匹配      │
│  ├─ MoveService: Match() 校验 + Reason 作为冲突描述        │
│  └─ 未来: 排课诊断系统 / 调课失败提示 / 日志输出            │
└──────────────────────────────────────────────────────────┘
```

### 为什么不直接在 solver.py 判断？

1. **单一真相源** — 如果 Python 侧也做匹配，就有两套规则需要同步。Go 侧改了 Category 映射，Python 侧不知道，就会产生不一致的解
2. **Python 不需要推断** — Go 已经知道哪些教室可用，只需传递 ID 列表。Python 只做 CP-SAT 建模，不做业务匹配
3. **消除历史问题** — v0.5.2 之前 Python 侧有 `SPECIALTY_ROOM_TYPES = {"体育馆", "实验室", "机房"}` 硬编码，Go 侧改了 Python 不会自动跟

### 为什么 Match() 不返回 bool？

```go
// ❌ 旧方式：bool 返回后，调用方需要再写一套判断来生成提示
if !matchRoom(task, room) {
    // 为什么不匹配？再判断一次...
    if room.Type != "实验室" { reason = "需要实验室" }
    ...
}

// ✅ 新方式：MatchResult 一次生成所有诊断信息
result := Match(task, course, room)
if !result.OK {
    log.Warn(result.Code, result.Reason)  // 直接用
    conflict.Description = result.Reason   // 直接用
}
```

### 为什么 ResourceMatcher 标记 V1？

v0.5.3 覆盖教室资源匹配。未来可能扩展：
- V2: 校区、建筑维度
- V3: 教师资质、多教室联合

标记 V1 使得诊断系统、日志、文档可以明确引用当前规则版本。

## 影响

### 正面
- 消除 5 处重复代码（~60 行）
- 修复 MoveService 教室类型校验遗漏
- Go/Python 匹配规则一致性从"约定"变为"架构保障"
- 为 v0.6 排课诊断系统奠定结构化基础

### 负面
- 新增 6 个文件（~770 行）
- SA 求解器需要适配（5 处改动）
- OR-Tools payload 增大（每个 task 携带 allowedRoomIds）

### 风险
- SA 行为变化导致评分波动 → 缓解：InferRoomType 对旧数据返回值与旧逻辑完全一致
- OR-Tools 解空间变化 → 缓解：AllowedRooms 对旧数据返回与旧 match_room 一致

## 关联

- [ADR-0001: 模拟退火算法](0001-simulated-annealing.md)
- [ADR-0005: SA + OR-Tools 多引擎架构](0005-multi-solver-architecture.md)
- [v0.5.3 URMF 设计文档](../design/v0.5.3-resource-matching-framework.md)
