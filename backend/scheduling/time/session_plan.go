package time

// resolveSessionPlan 根据教学任务参数计算每周 session 的 span 列表。
// 返回的 []int 每个元素是一个 session 的 span（1/2/3），len = sessions per week。
//
// 规则优先级:
//  1. PreferredSpan 优先（若合法）
//  2. 学时驱动的默认映射
//  3. MaxHoursPerWeek 封顶
func resolveSessionPlan(courseHours, startWeek, endWeek, maxHoursPerWeek, preferredSpan int) []int {
	weeks := endWeek - startWeek + 1
	if weeks < 1 {
		weeks = 1
	}
	if courseHours <= 0 {
		return []int{2} // 默认单 session, 2节
	}

	weeklyHoursR := (courseHours + weeks/2) / weeks
	if weeklyHoursR < 1 {
		weeklyHoursR = 1
	}
	if maxHoursPerWeek > 0 && weeklyHoursR > maxHoursPerWeek {
		weeklyHoursR = maxHoursPerWeek
	}

	// Path A: PreferredSpan
	if preferredSpan >= 1 && preferredSpan <= 3 {
		return planFromPreferredSpan(weeklyHoursR, preferredSpan)
	}

	// Path B: 学时驱动默认
	return planFromWeeklyHours(weeklyHoursR)
}

func planFromWeeklyHours(weeklyHours int) []int {
	if weeklyHours <= 0 {
		return []int{2}
	}
	switch weeklyHours {
	case 1:
		return []int{1}
	case 2:
		return []int{2}
	case 3:
		return []int{3}
	case 4:
		return []int{2, 2}
	case 5:
		return []int{2, 2, 1}
	case 6:
		return []int{2, 2, 2}
	case 7:
		return []int{2, 2, 2, 1}
	default:
		return []int{2, 2, 2, 2}
	}
}

func planFromPreferredSpan(weeklyHours, span int) []int {
	if weeklyHours <= 0 {
		return []int{span}
	}
	spans := make([]int, 0, 4)
	remaining := weeklyHours
	for remaining >= span && len(spans) < 4 {
		spans = append(spans, span)
		remaining -= span
	}
	if remaining > 0 && remaining <= 3 && len(spans) < 4 {
		spans = append(spans, remaining)
	}
	if len(spans) == 0 {
		s := weeklyHours
		if s > 3 {
			s = 3
		}
		spans = []int{s}
	}
	return spans
}
