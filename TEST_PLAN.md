# TEST_PLAN.md

高校智能排课系统测试计划。

- **适用范围**：v0.5.0 起所有 Theme / Maintenance 版本。
- **约束**：本文件为测试计划，不修改任何生产代码；仅约定"测什么、怎么算过、数据集是什么、阈值是几"。
- **上游依赖**：`ROADMAP.md`（Stable Core 冻结策略）、`CONTEXT.md`（领域术语）、`docs/adr/*`（架构决策）。
- **下游产物**：CI 脚本、`benchmark/` 数据集、`.golden` 快照、发布门禁清单。

---

## 0. 顶层结构

| 测试块 | 目的 | 主要工具 | CI 阻塞? |
|---|---|---|---|
| **§1 Model 测试** | 领域模型不变式（Span / Week / Conflict） | `go test` | 是 |
| **§2 Solver 测试** | SA / OR-Tools 正确性、无解、超时、边界 | `go test` + `pytest` | 是 |
| **§3 Score 测试** | 评分一致性（Stable Core 冻结护栏） | `go test` + Golden | 是 |
| **§4 数据测试** | Excel 导入 / 空字段 / 异常字段 | `go test` + 前端 `vitest` | 是 |
| **§5 Benchmark** | 三档规模的求解时间 / 成功率 / 评分 | `go test -bench` | 阈值报警 |

所有测试**禁止依赖网络、禁止依赖本机文件路径**，输入数据必须落 `testdata/` 或 `benchmark/`。