//go:build !production

package database

import (
	"log"
	"time"

	"scheduling-system/backend/models"
)

// SeedData initializes the database with representative demo data for development.
//
// v0.5.5 Stage A: 净化 seed —— 对齐湖工大官方 19 学院（本部 18 + 工程技术学院 1）：
//   - Departments 19 条首次入库（Code + Name 短名）
//   - Teachers 19 位（每院 1 位，Dept 字符串对齐官方名单）
//   - Courses 26 门（6 公共必修 + 19 学院代表课 + 1 计算机额外课）
//   - Classrooms 12 间（保留虚构 A栋/B栋/... 结构，Stage B 才升级到真实编号）
//   - ClassGroups 12 班（含 CS 双班演示合班场景）
//   - TeachingTasks 12 条（10 单班 + 2 合班演示 SC201/PE101）
//   - ScheduleEntries 12 条（对应 12 TT 的初始课表）
//
// 生产 build 使用 seed_production.go 的空实现 —— 用户通过 UI/导入自行录入。
//
// It is designed to be idempotent and resilient:
//   - Base data (semesters/departments/teachers/classrooms/courses/class-groups) is seeded
//     only when absent, so repeated calls never clobber user data or fail on UNIQUE constraints.
//   - Teaching tasks are what drives the scheduling engine; they are always
//     (re)created when missing, as long as the base data exists.
//   - Demo schedule entries are seeded only on a fresh database.
func SeedData(db DB) {
	if db == nil {
		return
	}

	var entryCount int64
	db.Model(&models.ScheduleEntry{}).Count(&entryCount)

	// Base data is seeded idempotently (FirstOrCreate), so it is always safe
	// to run — it adds any missing rows without clobbering existing data.
	seedBaseData(db)

	// Teaching tasks drive the engine — always ensure they exist (idempotent).
	seedTeachingTasks(db)

	// Demo schedule entries only on a fresh database.
	if entryCount == 0 {
		seedDemoEntries(db)
	}
}

// seedIfAbsent creates each item only if a row with the same key does not
// already exist, making the seed idempotent without clobbering user data.
// Uses FirstOrCreate for atomic "find or create" semantics — unlike Count+Create,
// this does not suffer from GORM error-state propagation on repeated calls.
func seedIfAbsent[T any](db DB, items []T, keyField string, keyVal func(T) interface{}) {
	for _, it := range items {
		_ = db.FirstOrCreate(&it, keyField+" = ?", keyVal(it))
	}
}

// hbutDepartments 湖工大官方 19 学院 SSOT（本部 18 + 工程技术学院独立学院 1）。
// Name 使用短名（不含产业学院后缀），与 Teacher.Dept / Course.Dept 冗余字符串保持一致。
// 权威来源：用户 2026-07-13 确认，交叉印证 2024 部门预算与官方招生页面。
var hbutDepartments = []models.Department{
	{Code: "ME", Name: "机械工程学院"},
	{Code: "EE", Name: "电气与电子工程学院"},
	{Code: "MC", Name: "材料与化学工程学院"},
	{Code: "LS", Name: "生命科学与健康工程学院"},
	{Code: "CE", Name: "土木建筑与环境学院"},
	{Code: "CS", Name: "计算机学院"},
	{Code: "AD", Name: "艺术设计学院"},
	{Code: "DA", Name: "数字艺术产业学院"},
	{Code: "ID", Name: "工业设计学院"},
	{Code: "EM", Name: "经济与管理学院"},
	{Code: "MX", Name: "马克思主义学院"},
	{Code: "FL", Name: "外国语学院"},
	{Code: "SC", Name: "理学院"},
	{Code: "PE", Name: "体育学院"},
	{Code: "VE", Name: "职业技术师范学院"},
	{Code: "DT", Name: "底特律绿色工业学院"},
	{Code: "IN", Name: "国际学院"},
	{Code: "IV", Name: "创新创业学院"},
	{Code: "ET", Name: "工程技术学院"},
}

// seedDepartments 首次将官方 19 学院写入 departments 表，FirstOrCreate by code 保证幂等。
func seedDepartments(db DB) {
	for _, d := range hbutDepartments {
		_ = db.FirstOrCreate(&d, "code = ?", d.Code)
	}
}

// seedBaseData seeds semesters, departments, teachers, classrooms, courses and class groups.
// 幂等：所有 seed 使用 FirstOrCreate / seedIfAbsent。
func seedBaseData(db DB) {
	// ===== Semesters =====
	// v0.5.5: 使用 AcademicYear + Term 复合唯一键，删除 Name/IsActive
	semesters := []models.Semester{
		{AcademicYear: "2025-2026", Term: models.SemesterTermSecond, StartDate: time.Date(2026, 2, 23, 0, 0, 0, 0, time.UTC), EndDate: time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC), Status: models.SemesterStatusActive},
		{AcademicYear: "2025-2026", Term: models.SemesterTermFirst, StartDate: time.Date(2025, 9, 1, 0, 0, 0, 0, time.UTC), EndDate: time.Date(2026, 1, 25, 0, 0, 0, 0, time.UTC), Status: models.SemesterStatusArchived},
		{AcademicYear: "2024-2025", Term: models.SemesterTermSecond, StartDate: time.Date(2025, 2, 24, 0, 0, 0, 0, time.UTC), EndDate: time.Date(2025, 7, 27, 0, 0, 0, 0, time.UTC), Status: models.SemesterStatusArchived},
	}
	for _, s := range semesters {
		_ = db.FirstOrCreate(&s, "academic_year = ? AND term = ?", s.AcademicYear, s.Term)
	}

	// ===== Departments =====
	// v0.5.5 Stage A: 首次将官方 19 学院写入 SSOT。
	seedDepartments(db)

	// ===== Teachers =====
	// 每院 1 位教师，Dept 字符串严格对齐 hbutDepartments 的 Name。
	teachers := []models.Teacher{
		{Code: "T001", Name: "张建国", Dept: "机械工程学院", Status: "active", PreferNoEarly: true, PreferLowFloor: true, MaxDaysPerWeek: 3},
		{Code: "T002", Name: "李明远", Dept: "电气与电子工程学院", Status: "active", PreferNoEarly: true, PreferLowFloor: true, MaxDaysPerWeek: 3},
		{Code: "T003", Name: "王伟", Dept: "材料与化学工程学院", Status: "active", MaxDaysPerWeek: 3},
		{Code: "T004", Name: "钱学林", Dept: "生命科学与健康工程学院", Status: "active", PreferNoLate: true, PreferLowFloor: true, MaxDaysPerWeek: 3},
		{Code: "T005", Name: "杨华", Dept: "土木建筑与环境学院", Status: "active", PreferNoEarly: true, PreferLowFloor: true, MaxDaysPerWeek: 3},
		{Code: "T006", Name: "周海", Dept: "计算机学院", Status: "active", MaxDaysPerWeek: 3},
		{Code: "T007", Name: "郑美", Dept: "艺术设计学院", Status: "active", MaxDaysPerWeek: 3},
		{Code: "T008", Name: "刘颖", Dept: "数字艺术产业学院", Status: "active", MaxDaysPerWeek: 3},
		{Code: "T009", Name: "黄蕾", Dept: "工业设计学院", Status: "active", MaxDaysPerWeek: 3},
		{Code: "T010", Name: "孙志强", Dept: "经济与管理学院", Status: "active", PreferLowFloor: true, MaxDaysPerWeek: 3},
		{Code: "T011", Name: "吴芳", Dept: "马克思主义学院", Status: "active", MaxDaysPerWeek: 3},
		{Code: "T012", Name: "刘芳", Dept: "外国语学院", Status: "active", PreferNoEarly: true, MaxDaysPerWeek: 3},
		{Code: "T013", Name: "赵秀英", Dept: "理学院", Status: "active", MaxDaysPerWeek: 3},
		{Code: "T014", Name: "陈刚", Dept: "体育学院", Status: "active", MaxDaysPerWeek: 3},
		{Code: "T015", Name: "周敏", Dept: "职业技术师范学院", Status: "active", MaxDaysPerWeek: 3},
		{Code: "T016", Name: "张伟", Dept: "底特律绿色工业学院", Status: "active", MaxDaysPerWeek: 3},
		{Code: "T017", Name: "李娜", Dept: "国际学院", Status: "active", MaxDaysPerWeek: 3},
		{Code: "T018", Name: "陈晨", Dept: "创新创业学院", Status: "active", MaxDaysPerWeek: 3},
		{Code: "T019", Name: "刘强", Dept: "工程技术学院", Status: "active", MaxDaysPerWeek: 3},
	}
	seedIfAbsent(db, teachers, "code", func(t models.Teacher) interface{} { return t.Code })

	// ===== Classrooms =====
	// Stage A 保留虚构编号 + 中文 RoomType（延后至 Stage B 升级为真实教学楼 + 英文枚举）。
	// F201 机房是本 Stage A 唯一新增（覆盖计算机课程需要）。
	classrooms := []models.Classroom{
		{Code: "A301", Name: "A301", Building: "A栋", Floor: 3, Capacity: 80, Type: "普通教室", Status: "available"},
		{Code: "A201", Name: "A201", Building: "A栋", Floor: 2, Capacity: 90, Type: "普通教室", Status: "available"},
		{Code: "B205", Name: "B205", Building: "B栋", Floor: 2, Capacity: 60, Type: "普通教室", Status: "available"},
		{Code: "B108", Name: "B108", Building: "B栋", Floor: 1, Capacity: 100, Type: "多媒体教室", Status: "available"},
		{Code: "B301", Name: "B301", Building: "B栋", Floor: 3, Capacity: 70, Type: "普通教室", Status: "available"},
		{Code: "C301", Name: "C301", Building: "C栋", Floor: 3, Capacity: 100, Type: "多媒体教室", Status: "available"},
		{Code: "C502", Name: "C502", Building: "C栋", Floor: 5, Capacity: 120, Type: "多媒体教室", Status: "available"},
		{Code: "D102", Name: "D102", Building: "D栋", Floor: 1, Capacity: 80, Type: "普通教室", Status: "available"},
		{Code: "D401", Name: "D401", Building: "D栋", Floor: 4, Capacity: 200, Type: "阶梯教室", Status: "available"},
		{Code: "E101", Name: "E101", Building: "E栋", Floor: 1, Capacity: 50, Type: "实验室", Status: "available"},
		{Code: "F201", Name: "F201", Building: "F栋", Floor: 2, Capacity: 50, Type: "机房", Status: "available"},
		{Code: "GYM01", Name: "体育馆", Building: "体育馆", Floor: 1, Capacity: 300, Type: "体育馆", Status: "available"},
	}
	seedIfAbsent(db, classrooms, "code", func(c models.Classroom) interface{} { return c.Code })

	// ===== Courses =====
	// 6 公共必修 + 19 学院代表课 + 1 计算机额外课 = 26 门。
	// 全部 Dept 字符串严格对齐 hbutDepartments 的 Name。
	courses := []models.Course{
		// ---- 公共必修课 × 6 ----
		{Code: "SC201", Name: "高等数学", Dept: "理学院", Credit: 5.0, Type: "公共必修", Hours: 80, Status: "active"},
		{Code: "SC202", Name: "线性代数", Dept: "理学院", Credit: 3.0, Type: "公共必修", Hours: 48, Status: "active"},
		{Code: "SC203", Name: "大学物理", Dept: "理学院", Credit: 4.0, Type: "公共必修", Hours: 64, Status: "active"},
		{Code: "FL101", Name: "大学英语", Dept: "外国语学院", Credit: 3.0, Type: "公共必修", Hours: 48, Status: "active"},
		{Code: "MX101", Name: "马克思主义基本原理", Dept: "马克思主义学院", Credit: 2.0, Type: "公共必修", Hours: 32, Status: "active"},
		{Code: "PE101", Name: "体育(篮球)", Dept: "体育学院", Credit: 1.0, Type: "公共必修", Hours: 32, Status: "active"},
		// ---- 学院代表课 × 19 ----
		{Code: "ME201", Name: "机械设计基础", Dept: "机械工程学院", Credit: 4.0, Type: "专业必修", Hours: 64, Status: "active"},
		{Code: "EE201", Name: "电路原理", Dept: "电气与电子工程学院", Credit: 4.0, Type: "专业必修", Hours: 64, Status: "active"},
		{Code: "MC201", Name: "有机化学", Dept: "材料与化学工程学院", Credit: 4.0, Type: "专业必修", Hours: 64, Status: "active", Category: models.CategoryLab},
		{Code: "LS201", Name: "生物化学", Dept: "生命科学与健康工程学院", Credit: 4.0, Type: "专业必修", Hours: 64, Status: "active", Category: models.CategoryLab},
		{Code: "CE201", Name: "结构力学", Dept: "土木建筑与环境学院", Credit: 4.0, Type: "专业必修", Hours: 64, Status: "active"},
		{Code: "CS301", Name: "数据结构", Dept: "计算机学院", Credit: 4.0, Type: "专业必修", Hours: 64, Status: "active", Category: models.CategoryComputer},
		{Code: "AD201", Name: "设计素描", Dept: "艺术设计学院", Credit: 3.0, Type: "专业必修", Hours: 48, Status: "active", Category: models.CategoryArt},
		{Code: "DA201", Name: "数字媒体艺术导论", Dept: "数字艺术产业学院", Credit: 3.0, Type: "专业必修", Hours: 48, Status: "active", Category: models.CategoryArt},
		{Code: "ID201", Name: "产品设计", Dept: "工业设计学院", Credit: 3.0, Type: "专业必修", Hours: 48, Status: "active", Category: models.CategoryArt},
		{Code: "EM201", Name: "西方经济学", Dept: "经济与管理学院", Credit: 3.0, Type: "专业必修", Hours: 48, Status: "active"},
		{Code: "MX201", Name: "中国近现代史纲要", Dept: "马克思主义学院", Credit: 2.0, Type: "公共必修", Hours: 32, Status: "active"},
		{Code: "FL201", Name: "英美文学", Dept: "外国语学院", Credit: 3.0, Type: "专业必修", Hours: 48, Status: "active"},
		{Code: "SC301", Name: "概率论与数理统计", Dept: "理学院", Credit: 3.0, Type: "专业必修", Hours: 48, Status: "active"},
		{Code: "PE201", Name: "体育教育学", Dept: "体育学院", Credit: 3.0, Type: "专业必修", Hours: 48, Status: "active"},
		{Code: "VE201", Name: "职业教育学", Dept: "职业技术师范学院", Credit: 3.0, Type: "专业必修", Hours: 48, Status: "active"},
		{Code: "DT201", Name: "绿色制造", Dept: "底特律绿色工业学院", Credit: 3.0, Type: "专业必修", Hours: 48, Status: "active"},
		{Code: "IN201", Name: "跨文化交际", Dept: "国际学院", Credit: 2.0, Type: "专业必修", Hours: 32, Status: "active"},
		{Code: "IV201", Name: "创新创业基础", Dept: "创新创业学院", Credit: 2.0, Type: "公共必修", Hours: 32, Status: "active"},
		{Code: "ET201", Name: "应用型工程实践", Dept: "工程技术学院", Credit: 3.0, Type: "专业必修", Hours: 48, Status: "active", Category: models.CategoryLab},
		// ---- 计算机学院额外课（演示同院多课程） × 1 ----
		{Code: "CS302", Name: "操作系统", Dept: "计算机学院", Credit: 4.0, Type: "专业必修", Hours: 64, Status: "active", Category: models.CategoryComputer},
	}
	seedIfAbsent(db, courses, "code", func(c models.Course) interface{} { return c.Code })

	// ===== Class Groups =====
	// 12 班：10 学院各 1 班 + 计算机双班（合班演示需要）+ 工程技术 1 班。
	groups := []models.ClassGroup{
		{Code: "CS2301", Name: "计算机2301", Dept: "计算机学院", Grade: 2023, Students: 86, Status: "active"},
		{Code: "CS2302", Name: "计算机2302", Dept: "计算机学院", Grade: 2023, Students: 82, Status: "active"},
		{Code: "ME2301", Name: "机械2301", Dept: "机械工程学院", Grade: 2023, Students: 72, Status: "active"},
		{Code: "EE2301", Name: "电气2301", Dept: "电气与电子工程学院", Grade: 2023, Students: 68, Status: "active"},
		{Code: "LS2301", Name: "生命2301", Dept: "生命科学与健康工程学院", Grade: 2023, Students: 70, Status: "active"},
		{Code: "CE2301", Name: "土木2301", Dept: "土木建筑与环境学院", Grade: 2023, Students: 55, Status: "active"},
		{Code: "EM2301", Name: "经管2301", Dept: "经济与管理学院", Grade: 2023, Students: 78, Status: "active"},
		{Code: "AD2301", Name: "艺设2301", Dept: "艺术设计学院", Grade: 2023, Students: 40, Status: "active"},
		{Code: "ID2301", Name: "工设2301", Dept: "工业设计学院", Grade: 2023, Students: 45, Status: "active"},
		{Code: "FL2301", Name: "外语2301", Dept: "外国语学院", Grade: 2023, Students: 50, Status: "active"},
		{Code: "SC2301", Name: "理学2301", Dept: "理学院", Grade: 2023, Students: 60, Status: "active"},
		{Code: "ET2301", Name: "工程2301", Dept: "工程技术学院", Grade: 2023, Students: 55, Status: "active"},
	}
	seedIfAbsent(db, groups, "code", func(g models.ClassGroup) interface{} { return g.Code })
}

// seedTeachingTasks 创建 12 个演示教学任务（10 单班 + 2 合班演示）。
// 使用按 Code 查询而非下标引用，避免 base data 顺序变化时错乱。
// FirstOrCreate 幂等：(course_id, teacher_id, semester_id) 复合键。
func seedTeachingTasks(db DB) {
	// 加载基础数据（按 code 索引，避免顺序脆弱依赖）
	var semesters []models.Semester
	db.Order("id asc").Find(&semesters)
	if len(semesters) == 0 {
		log.Println("Seed: skip teaching tasks (no semesters)")
		return
	}
	activeSemesterID := semesters[0].ID

	courseByCode := loadCourseByCode(db)
	teacherByCode := loadTeacherByCode(db)
	groupByCode := loadGroupByCode(db)

	if len(courseByCode) == 0 || len(teacherByCode) == 0 || len(groupByCode) == 0 {
		log.Println("Seed: skip teaching tasks (base data missing)")
		return
	}

	// seedTask 描述一个教学任务：(课程, 教师, 学时, 关联班级列表)。
	// ClassCodes 长度 > 1 表示合班场景（一个教学任务关联多个班级）。
	type seedTask struct {
		CourseCode  string
		TeacherCode string
		Hours       int
		ClassCodes  []string
	}

	tasksSpec := []seedTask{
		// ---- 10 单班任务 ----
		{"CS301", "T006", 64, []string{"CS2301"}}, // 数据结构 · 周海 · CS2301
		{"ME201", "T001", 64, []string{"ME2301"}}, // 机械设计基础 · 张建国 · ME2301
		{"EE201", "T002", 64, []string{"EE2301"}}, // 电路原理 · 李明远 · EE2301
		{"CE201", "T005", 64, []string{"CE2301"}}, // 结构力学 · 杨华 · CE2301
		{"EM201", "T010", 48, []string{"EM2301"}}, // 西方经济学 · 孙志强 · EM2301
		{"AD201", "T007", 48, []string{"AD2301"}}, // 设计素描 · 郑美 · AD2301
		{"LS201", "T004", 64, []string{"LS2301"}}, // 生物化学 · 钱学林 · LS2301
		{"ID201", "T009", 48, []string{"ID2301"}}, // 产品设计 · 黄蕾 · ID2301
		{"FL101", "T012", 48, []string{"FL2301"}}, // 大学英语 · 刘芳 · FL2301
		{"ET201", "T019", 48, []string{"ET2301"}}, // 应用型工程实践 · 刘强 · ET2301
		// ---- 2 合班演示 ----
		{"SC201", "T013", 80, []string{"CS2301", "CS2302"}}, // 高等数学 · 赵秀英 · CS 双班合上
		{"PE101", "T014", 32, []string{"CS2301", "ME2301"}}, // 体育(篮球) · 陈刚 · CS+ME 合班
	}

	for _, spec := range tasksSpec {
		course, ok := courseByCode[spec.CourseCode]
		if !ok {
			log.Printf("Seed: course %s not found, skip task", spec.CourseCode)
			continue
		}
		teacher, ok := teacherByCode[spec.TeacherCode]
		if !ok {
			log.Printf("Seed: teacher %s not found, skip task", spec.TeacherCode)
			continue
		}

		task := models.TeachingTask{
			CourseID:        course.ID,
			TeacherID:       teacher.ID,
			SemesterID:      activeSemesterID,
			Status:          "active",
			TotalHours:      spec.Hours,
			StartWeek:       1,
			EndWeek:         16,
			MaxHoursPerWeek: 0,
		}
		result := db.Where("course_id = ? AND teacher_id = ? AND semester_id = ?",
			course.ID, teacher.ID, activeSemesterID).FirstOrCreate(&task)
		if result.Error() != nil {
			log.Printf("Seed: create teaching task failed (%s/%s): %v", spec.CourseCode, spec.TeacherCode, result.Error())
			continue
		}

		// TeachingTaskClass 关联表 — 合班场景需插入多条
		for _, classCode := range spec.ClassCodes {
			group, ok := groupByCode[classCode]
			if !ok {
				log.Printf("Seed: class group %s not found, skip association", classCode)
				continue
			}
			assoc := models.TeachingTaskClass{
				TeachingTaskID: task.ID,
				ClassGroupID:   group.ID,
			}
			_ = db.Where("teaching_task_id = ? AND class_group_id = ?",
				task.ID, group.ID).FirstOrCreate(&assoc)
		}
	}
}

// seedDemoEntries 灌入 12 条初始排课条目（对应 seedTeachingTasks 的 12 个 TT）。
// 按 Code 查询避免下标脆弱依赖。周一至周四分布，教室按 RoomType 匹配。
// 仅当 schedule_entries 表为空时执行（RunScheduling 会清空重排）。
func seedDemoEntries(db DB) {
	var semesters []models.Semester
	db.Order("id asc").Find(&semesters)
	if len(semesters) == 0 {
		log.Println("Seed: skip demo entries (no semesters)")
		return
	}
	semesterID := semesters[0].ID

	courseByCode := loadCourseByCode(db)
	teacherByCode := loadTeacherByCode(db)
	roomByCode := loadClassroomByCode(db)

	// entrySpec 描述一条演示排课，通过 code 查询目标资源。
	type entrySpec struct {
		CourseCode  string
		TeacherCode string
		RoomCode    string
		DayOfWeek   models.DayOfWeek
		StartPeriod models.Period
		Span        int
	}

	entries := []entrySpec{
		// ---- Monday ----
		{"CS301", "T006", "D102", 0, 0, 2},   // 1-2 节
		{"ME201", "T001", "D401", 0, 2, 2},   // 3-4 节（大班优先阶梯）
		{"EE201", "T002", "A201", 0, 4, 2},   // 5-6 节
		{"SC201", "T013", "D401", 0, 6, 2},   // 7-8 节（合班）
		// ---- Tuesday ----
		{"CE201", "T005", "C301", 1, 0, 2},   // 1-2 节
		{"EM201", "T010", "A301", 1, 2, 2},   // 3-4 节
		{"AD201", "T007", "C502", 1, 4, 2},   // 5-6 节
		// ---- Wednesday ----
		{"LS201", "T004", "E101", 2, 0, 2},   // 1-2 节（实验室）
		{"ID201", "T009", "B205", 2, 2, 2},   // 3-4 节
		{"FL101", "T012", "B108", 2, 4, 2},   // 5-6 节
		// ---- Thursday ----
		{"ET201", "T019", "F201", 3, 0, 2},   // 1-2 节（机房，工程实践）
		{"PE101", "T014", "GYM01", 3, 2, 2},  // 3-4 节（合班，体育馆）
	}

	rows := make([]models.ScheduleEntry, 0, len(entries))
	for _, spec := range entries {
		course, ok := courseByCode[spec.CourseCode]
		if !ok {
			continue
		}
		teacher, ok := teacherByCode[spec.TeacherCode]
		if !ok {
			continue
		}
		room, ok := roomByCode[spec.RoomCode]
		if !ok {
			continue
		}
		rows = append(rows, models.ScheduleEntry{
			CourseID:    course.ID,
			TeacherID:   teacher.ID,
			ClassroomID: room.ID,
			SemesterID:  semesterID,
			DayOfWeek:   spec.DayOfWeek,
			StartPeriod: spec.StartPeriod,
			Span:        spec.Span,
			Weeks:       "1-16",
		})
	}

	if len(rows) == 0 {
		return
	}
	if err := db.Create(&rows).Error(); err != nil {
		log.Printf("Seed: demo schedule entries already exist or creation failed: %v", err)
	}
}

// ===== Helper: Code → Entity 查询工具 =====
// 将 base data 按 code 建索引，避免下标脆弱依赖。

func loadCourseByCode(db DB) map[string]models.Course {
	var courses []models.Course
	db.Find(&courses)
	out := make(map[string]models.Course, len(courses))
	for _, c := range courses {
		out[c.Code] = c
	}
	return out
}

func loadTeacherByCode(db DB) map[string]models.Teacher {
	var teachers []models.Teacher
	db.Find(&teachers)
	out := make(map[string]models.Teacher, len(teachers))
	for _, t := range teachers {
		out[t.Code] = t
	}
	return out
}

func loadGroupByCode(db DB) map[string]models.ClassGroup {
	var groups []models.ClassGroup
	db.Find(&groups)
	out := make(map[string]models.ClassGroup, len(groups))
	for _, g := range groups {
		out[g.Code] = g
	}
	return out
}

func loadClassroomByCode(db DB) map[string]models.Classroom {
	var rooms []models.Classroom
	db.Find(&rooms)
	out := make(map[string]models.Classroom, len(rooms))
	for _, r := range rooms {
		out[r.Code] = r
	}
	return out
}
