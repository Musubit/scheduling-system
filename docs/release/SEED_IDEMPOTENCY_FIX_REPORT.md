# Seed 幂等性修复报告

> **日期**：2026-07-13
> **基线**：v0.5.4 TeachingTask Domain Stabilization
> **修改范围**：仅 `backend/database/database.go` + `backend/database/seed.go`

---

## 一、问题原因

### 根因：`Count` + `Create` 两步操作 + GORM error state 传播

`seedIfAbsent` 原实现：

```go
var cnt int64
db.Model(new(T)).Where(keyField+" = ?", keyVal(it)).Count(&cnt)
if cnt == 0 {
    db.Create(&it)
}
```

**问题链路**：

1. 某一行 `Create` 因 UNIQUE 约束失败（如重复 seed 调用）
2. 错误虽然通过 `log.Printf` 打印，但**未检查 `Error()`**，`db` 的底层 `*gorm.DB` 保持 error state
3. 下一轮迭代的 `Count` 因 error state 存在而**不执行查询**，`cnt` 保持 0
4. `cnt == 0` 为 true → 再次 `Create` → 再次失败
5. 连锁反应直到所有剩余行全部失败（典型表现为报文中最后几行 UNIQUE 约束错误）

### 是否为新引入的 bug？

否。该问题自 `seedIfAbsent` 引入以来一直存在，仅在重复 seed 时触发。v0.5.4 之前测试流程未暴露，因为开发数据库经常被删除重建。

---

## 二、修改文件

| 文件 | 变更 |
|---|---|
| `backend/database/database.go` | `DB` 接口增加 `FirstOrCreate` 方法；`GormAdapter` 实现（支持带条件和不带条件两种调用） |
| `backend/database/seed.go` | `seedIfAbsent` 重写为 `FirstOrCreate`；`seedTeachingTasks` 重写为 `FirstOrCreate`；`seedDemoEntries` 加 error 处理 |

### 2.1 `database.go` — DB 接口扩展

```go
// 新增接口方法
FirstOrCreate(dest interface{}, conds ...interface{}) DB

// 实现（兼容两种调用模式）
func (g *GormAdapter) FirstOrCreate(dest interface{}, conds ...interface{}) DB {
    if len(conds) > 0 {
        return &GormAdapter{db: g.db.Where(conds[0], conds[1:]...).FirstOrCreate(dest)}
    }
    return &GormAdapter{db: g.db.FirstOrCreate(dest)}
}
```

### 2.2 `seed.go` — seedIfAbsent

**前**：`Count(&cnt)` + `if cnt == 0 { Create(&it) }`
**后**：`db.FirstOrCreate(&it, keyField+" = ?", keyVal(it))`

`FirstOrCreate` 是 GORM 内置的原子操作：先按条件查找，存在则填充到 `it`，不存在则创建新行。单次调用完成，无 error state 传播风险。

### 2.3 `seed.go` — seedTeachingTasks

**前**：`Count(&existing)` + `if existing > 0 { continue }` else `Create(&task)`
**后**：`db.Where("course_id = ? AND teacher_id = ? AND semester_id = ?", ...).FirstOrCreate(&task)`

TeachingTaskClass 同理改为 `FirstOrCreate("teaching_task_id = ? AND class_group_id = ?")`。

### 2.4 `seed.go` — seedDemoEntries

**前**：`db.Create(&entries)`
**后**：`if err := db.Create(&entries).Error(); err != nil { log.Printf(...) }`

加 error 检查，避免静默失败。

---

## 三、验证结果

### 3.1 编译

```
> go build ./...
无输出 — 编译通过
```

### 3.2 单测

```
> go test ./...
ok  scheduling-system/backend/models    (cached)
ok  scheduling-system/backend/services  (cached)
全通过
```

### 3.3 幂等性运行时验证

**首次创建**：

```
InitDB #1: OK
teachers=19  courses=35  groups=13  tasks=34
```

**同库二次加载**：

```
InitDB #2: OK (idempotent)
teachers=19  courses=35  groups=13  tasks=34
```

无 UNIQUE constraint 错误，数据一致。

### 3.4 前端构建

```
> npm run build
✓ built in 5.58s
零错误
```

### 3.5 seedcheck 工具两轮独立运行

```
$ go run backend/seedcheck/main.go
teachers=19  colleges=19  courses=35  groups=13  teachingTasks=34
$ go run backend/seedcheck/main.go
teachers=19  colleges=19  courses=35  groups=13  teachingTasks=34
```

两轮均成功，结果一致。

---

## 四、是否影响 v0.5.4

| 维度 | 影响 | 说明 |
|---|---|---|
| TeachingTask 模型 | ❌ 无影响 | 模型字段未变，`FirstOrCreate` 仅为"查找或创建" |
| Solver | ❌ 无影响 | 无任何 solver 代码被修改 |
| ScheduleEntry 结构 | ❌ 无影响 | 仅加了 error 检查，逻辑相同 |
| 前端界面 | ❌ 无影响 | 无前端代码修改 |
| 数据库 Schema | ❌ 无影响 | 无迁移，无表结构变化 |
| 业务语义 | ❌ 无影响 | "同一课程+教师+学期"唯一性规则不变 |

**结论**：v0.5.4 所有已验收功能不受影响。

---

## 五、修改汇总

```
backend/database/database.go
  └─ DB interface: +FirstOrCreate
  └─ GormAdapter:  +FirstOrCreate (两种调用模式)

backend/database/seed.go
  └─ seedIfAbsent: Count+Create → FirstOrCreate
  └─ seedTeachingTasks: Count+Create → FirstOrCreate (含 TeachingTaskClass)
  └─ seedDemoEntries: Create → Create + error check
```

**不改的文件**：`models/`、`services/`、`scheduler/`、`frontend/`（除 bindings 自动生成外零变动）。
