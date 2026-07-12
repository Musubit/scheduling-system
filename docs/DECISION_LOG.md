# DECISION_LOG.md — 关键决策记录

> **最后更新**: 2026-07-13

---

## 2026-07-10 — v0.5.2 评分统一语义

**决策**: 引入 `Completeness` + `FinalTotal` 字段，排课完成度作为评分惩罚因子

**理由**: 防止"少排少错"——未排满的课表因冲突少而得分虚高

**公式**: `factor = ratio × (0.5 + 0.5 × ratio)`，ratio=0.5 → factor=0.375

**兼容性**: 新字段可选，旧代码不读不写无影响；`Total` 保留原 7 项之和语义

---

## 2026-07-10 — v0.5.2 SA delta-score 缓存

**决策**: 在 SA 热路径维护增量评分缓存，避免每次邻域评估全量重扫

**实现**: `sa_scorecache.go` + `sa_scorecache_apply.go` + `sa_scorecache_reduce.go`
- `applyEntry(sign, entry, isSports)` 对称 ±1 更新所有计数器
- `scoreFromCache()` 从缓存还原完整 ScoreBreakdown
- 正确性由 `TestDeltaScoreMatchesFullScore` 黄金测试保障

**结果**: SA 迭代速度 29,830 → 236,000+ iter/s（8x 加速）

---

## 2026-07-10 — v0.5.2 uint64 occupancy keys

**决策**: roomOcc/teacherOcc/classOcc 从 `map[string]bool` 改为 `map[uint64]bool`

**实现**: `occKey(day, period, id)` 将三个坐标打包为单个 uint64

**理由**: uint64 键的哈希和比较比字符串快，减少 GC 压力

---

## 2026-07-10 — v0.5.1 灵活课时跨度

**决策**: 支持课时跨度 1/2/3 节（原先仅支持 2 节）

**实现**: `session_plan.go` 作为单一真实来源，`solver.py` 镜像同步

**优先级**: task.PreferredSpan > courseHours 派生 > MaxHoursPerWeek 钳制

---

## 2026-07-10 — v0.4.0 Stable Core 冻结

**决策**: 冻结评分核心（ScoreSchedule / ScoringContext / ScoreBreakdown / ScheduleSnapshot / ScheduleVersion / TeachingTaskClass）

**规则**: 可追加字段，不可修改已有字段语义

**影响**: v0.5.2 的扩展均以可选字段方式追加，保持向后兼容

---

## 2026-07-09 — v0.4.0 全局学期状态

**决策**: 建立 `appStore.currentSemesterId` 为全局唯一学期来源（SSOT）

**影响**: SchedulePage / ResourcePage / SchedulingPage / ReportPage / HistoryComparePage 统一消费

---

## 2026-07-08 — v0.4.0 DB 接口化 (ADR-0003)

**决策**: 定义 `database.DB` 接口，构造函数注入到每个服务

**替代方案**: 包级全局 `*gorm.DB` 变量（已废弃）

---

## 2026-07-08 — v0.4.0 TeachingTask 实体 (ADR-0004)

**决策**: 引入 TeachingTask + TeachingTaskClass 显式关联，替代院系模糊匹配

**影响**: 消除 `deptMap` / `reverseDeptMap` 硬编码，支持合班和跨院系排课

---

## 2026-07-07 — 多引擎架构 (ADR-0005)

**决策**: SA 为主引擎，OR-Tools 为可选增强，uv 管理 Python 依赖

**降级策略**: OR-Tools 不可用/超时 → 无缝切换 SA，用户无感

---

## 2026-07-07 — 快照模式 (ADR-0002)

**决策**: 排课/微调后生成持久化快照，支持历史对比

**触发**: 自动（排课后）+ 手动（微调后用户主动）

---

## 2026-07-07 — SA 作为主求解器 (ADR-0001)

**决策**: 采用模拟退火，放弃 OR-Tools 作为主引擎

**理由**: OR-Tools 体积 150MB+、MSVC/MinGW ABI 不兼容、Go 绑定不成熟
