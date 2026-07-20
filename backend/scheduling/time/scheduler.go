// Package time 提供纯时间排课的 SA 求解器。
//
// TimeScheduler 实现 types.ITimeScheduler，是两阶段流水线的 Stage 1。
// 它只关心"什么课在什么时候上"——不涉及教室分配（INV-P1/P2）。
//
// 求解器内部使用增量评分缓存（scoreCache），在 SA 热路径上以 O(1)
// 更新评分维度，避免每次迭代全量扫描。
package time

import (
	"context"
	"math"
	"math/rand"
	"time"

	schedtypes "scheduling-system/backend/scheduling/types"
)

// Config 是 SA 求解器的超参数。零值字段会被 DefaultConfig 填充。
type Config struct {
	InitialTemp       float64 // 起始温度 (default 1000.0)
	CoolingRate       float64 // 每步降温倍率 (default 0.95)
	IterationsPerTemp int     // 每温度层的邻域移动次数 (default 5000)
	MinTemp           float64 // 终止温度 (default 0.01)
	MaxTimeSeconds    float64 // 最大求解时间 (0 = unlimited, default 60)
}

// DefaultConfig 返回推荐的 SA 超参数。
func DefaultConfig() Config {
	return Config{
		InitialTemp:       1000.0,
		CoolingRate:       0.95,
		IterationsPerTemp: 5000,
		MinTemp:           0.01,
		MaxTimeSeconds:    60,
	}
}

// TimeScheduler 是纯时间排课的 SA 求解器。
type TimeScheduler struct {
	config Config
}

// New 创建一个 TimeScheduler。若 cfg 为零值则使用 DefaultConfig。
func New(cfg Config) *TimeScheduler {
	if cfg.IterationsPerTemp <= 0 {
		cfg = DefaultConfig()
	}
	return &TimeScheduler{config: cfg}
}

// 编译期接口检查。
var _ schedtypes.ITimeScheduler = (*TimeScheduler)(nil)

// Solve 实现 types.ITimeScheduler。
func (ts *TimeScheduler) Solve(
	ctx context.Context,
	input schedtypes.TimeSchedulingInput,
	progress schedtypes.ProgressReporter,
) (schedtypes.TimeSchedulingOutput, error) {
	// 防御: nil reporter
	if progress == nil {
		progress = schedtypes.NoopReporter{}
	}

	start := time.Now()
	rng := rand.New(rand.NewSource(input.Seed))

	// 构建内部上下文
	sctx := newTimeContext(input, rng)

	// Phase 1: 贪心构造初始解
	progress.Stage("build_initial", 10)
	sctx.buildInitial()

	// Phase 2: 播种评分缓存
	sctx.sCache.rebuildFromEntries(sctx.entries, sctx.taskByID, sctx.sportsCourseIDs)

	// 初始评分
	currentScore := sctx.scoreFromCache()
	bestEntries := make([]timeEntry, len(sctx.entries))
	copy(bestEntries, sctx.entries)
	bestScore := currentScore

	// SA 主循环
	temp := ts.config.InitialTemp
	iter := 0
	deadline := start.Add(time.Duration(ts.config.MaxTimeSeconds) * time.Second)
	if ts.config.MaxTimeSeconds <= 0 {
		deadline = start.Add(60 * time.Second) // 默认 60s
	}

	progress.Stage("anneal", 20)

	for temp > ts.config.MinTemp {
		select {
		case <-ctx.Done():
			goto done
		default:
		}

		if time.Now().After(deadline) {
			break
		}

		for i := 0; i < ts.config.IterationsPerTemp; i++ {
			iter++

			// 尝试邻域移动
			newScore := sctx.tryNeighbor(currentScore)
			delta := newScore - currentScore

			if delta > 0 || (temp > 0 && rng.Float64() < math.Exp(delta/temp)) {
				currentScore = newScore
				if newScore > bestScore {
					bestScore = newScore
					bestEntries = make([]timeEntry, len(sctx.entries))
					copy(bestEntries, sctx.entries)
				}
			} else {
				sctx.undoNeighbor()
			}

			// 定期检查取消
			if i%100 == 0 {
				select {
				case <-ctx.Done():
					goto done
				default:
				}
			}
		}

		temp *= ts.config.CoolingRate

		// 定期报告进度
		if iter%1000 == 0 {
			progress.Iteration(iter, 0, currentScore, bestScore, temp)
		}
	}

done:
	elapsed := time.Since(start).Milliseconds()

	// 将最优解转为 TimeAssignmentDraft
	assignments := entriesToDrafts(bestEntries)

	// 计算 TimeScoreDetail（用最优解的条目重建缓存评分）
	finalCache := newScoreCache(sctx.teacherByID, sctx.taskByID)
	finalCache.rebuildFromEntries(bestEntries, sctx.taskByID, sctx.sportsCourseIDs)
	detail := finalCache.scoreDetail(sctx.enabledMap, sctx.sportsCourseIDs, sctx.expectedTotalSessions)

	progress.Stage("done", 100)

	return schedtypes.TimeSchedulingOutput{
		Assignments: assignments,
		ScoreDetail: detail.toTimeScoreDetail(),
		Iterations:  iter,
		ElapsedMs:   elapsed,
	}, nil
}
