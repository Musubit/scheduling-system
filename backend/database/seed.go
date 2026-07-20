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

// hbutBuildings 代表性教学楼 seed（Stage B 最终闭环 B5-Final）。
//
// 覆盖 3 种 Building.Category：
//   - teaching：1 教 / 2 教 / 3 教 / 5 教 A 区 / 5 教 B 区 / 7 教 A 区 / 7 教 B 区
//   - lab     ：6 教实验楼 A 区 / 6 教实验楼 B 区
//   - sports  ：中区操场 / 西区操场
//
// 不追求完整录入 —— 仅提供演示排课所需的最小教学楼多样性。
// 编号 "1" / "5A" / "6A" 等格式是 HBUT 约定，解析规则位于 backend/adapters/hbut。
var hbutBuildings = []models.Building{
	{Code: "1", Name: "1 教", Category: models.BuildingCategoryTeaching, Status: models.BuildingStatusActive},
	{Code: "2", Name: "2 教", Category: models.BuildingCategoryTeaching, Status: models.BuildingStatusActive},
	{Code: "3", Name: "3 教", Category: models.BuildingCategoryTeaching, Status: models.BuildingStatusActive},
	{Code: "5A", Name: "5 教 A 区", Category: models.BuildingCategoryTeaching, Status: models.BuildingStatusActive},
	{Code: "5B", Name: "5 教 B 区", Category: models.BuildingCategoryTeaching, Status: models.BuildingStatusActive},
	{Code: "6A", Name: "6 教实验楼 A 区", Category: models.BuildingCategoryLab, Status: models.BuildingStatusActive},
	{Code: "6B", Name: "6 教实验楼 B 区", Category: models.BuildingCategoryLab, Status: models.BuildingStatusActive},
	{Code: "7A", Name: "7 教 A 区", Category: models.BuildingCategoryTeaching, Status: models.BuildingStatusActive},
	{Code: "7B", Name: "7 教 B 区", Category: models.BuildingCategoryTeaching, Status: models.BuildingStatusActive},
	{Code: "FIELD_M", Name: "中区操场", Category: models.BuildingCategorySports, Status: models.BuildingStatusActive},
	{Code: "FIELD_W", Name: "西区操场", Category: models.BuildingCategorySports, Status: models.BuildingStatusActive},
}

// seedBuildings 首次将代表性教学楼写入 buildings 表，FirstOrCreate by code 保证幂等。
// 必须在 seedBaseData 中先于 Classrooms seed 调用，供 Classroom BuildingID FK 查找。
//
// Stage B B5-Final：先清理历史遗留的虚拟 "GYM" 教学楼（若存在），
// 该 code 在早期开发 seed 中曾用于承载"体育馆"教室，B5-Final 后统一使用
// 独立 sports Building（FIELD_M / FIELD_W）+ 场地本身作为 Classroom。
func seedBuildings(db DB) {
	// 清理旧 dev 数据库残留：GYM building 与其名下的 GYM_MAIN classroom。
	// 幂等：不存在时无副作用。ScheduleEntry 中引用 GYM_MAIN 的行由 seedDemoEntries
	// 的 code→ID 名称查找负责跳过（PE101 目标已切换到 MIDDLE_FIELD）。
	_ = db.Where("code = ?", "GYM_MAIN").Delete(&models.Classroom{})
	_ = db.Where("code = ?", "GYM").Delete(&models.Building{})

	for _, b := range hbutBuildings {
		_ = db.FirstOrCreate(&b, "code = ?", b.Code)
	}
}

// seedBaseData seeds semesters, departments, teachers, classrooms, courses and class groups.
// 幂等：所有 seed 使用 FirstOrCreate / seedIfAbsent。
func seedBaseData(db DB) {
	// ===== Semesters =====
	// v0.5.5 修订：只 seed 一个"下一个即将开学"的学期（Status=planned），
	// 由 nextUpcomingSemester(time.Now()) 动态推算 —— 用户可在设置里编辑或
	// 直接启用。避免历史 seed 数据造成的"2 月有两个第一天"混淆。
	sem := nextUpcomingSemester(time.Now())
	_ = db.FirstOrCreate(&sem, "academic_year = ? AND term = ?", sem.AcademicYear, sem.Term)

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

	// ===== Buildings =====
	// v0.5.5 Stage B: 首次将代表性教学楼写入 buildings 表；后续 Classrooms
	// 通过 BuildingCode 查找 BuildingID FK。
	seedBuildings(db)

	// ===== Classrooms =====
	// v0.5.5 Stage B: 教室编号使用 HBUT 真实格式 "<BuildingCode>-<RoomNumber>"。
	// 编号解析规则位于 backend/adapters/hbut，本处 seed 明文指定所有字段，
	// **不调用 Parser**（保持 seed → Core 单向依赖）。
	//
	// 通过 buildingByCode 查找 BuildingID —— 禁止硬编码 FK。
	buildingByCode := loadBuildingByCode(db)

	type classroomSpec struct {
		Code         string
		Name         string
		BuildingCode string
		Floor        int
		Number       string
		Capacity     int
		RoomType     string
	}
	classroomSpecs := []classroomSpec{
		// 1 教（单区教学楼代表）
		{"1-201", "1-201", "1", 2, "201", 90, models.RoomTypeNormal},
		{"1-302", "1-302", "1", 3, "302", 80, models.RoomTypeNormal},
		{"1-001", "1-001", "1", 0, "001", 200, models.RoomTypeLecture}, // floor=0 阶梯教室
		// 2 教 / 3 教（单区教学楼补充演示；每栋 1 间代表性教室）
		{"2-302", "2-302", "2", 3, "302", 80, models.RoomTypeNormal},
		{"3-203", "3-203", "3", 2, "203", 90, models.RoomTypeMultimedia},
		// 5A 教（分区楼 A 侧）
		{"5A-102", "5A-102", "5A", 1, "102", 100, models.RoomTypeMultimedia},
		{"5A-203", "5A-203", "5A", 2, "203", 60, models.RoomTypeNormal},
		// 5B 教（分区楼 B 侧）
		{"5B-301", "5B-301", "5B", 3, "301", 100, models.RoomTypeMultimedia},
		{"5B-303", "5B-303", "5B", 3, "303", 70, models.RoomTypeNormal},
		// 6A / 6B 教（实验楼分区；Building.Category=lab 但内部教室 RoomType 不受硬约束）
		{"6A-213", "6A-213", "6A", 2, "213", 50, models.RoomTypeLab},
		{"6A-422", "6A-422", "6A", 4, "422", 50, models.RoomTypeComputer}, // 实验楼里的机房：验证 Category 不硬约束 RoomType
		{"6B-315", "6B-315", "6B", 3, "315", 50, models.RoomTypeLab},
		// 7A / 7B 教（多分区教学楼补充演示）
		{"7A-102", "7A-102", "7A", 1, "102", 80, models.RoomTypeNormal},
		{"7B-502", "7B-502", "7B", 5, "502", 120, models.RoomTypeMultimedia},
		// 体育教学资源（HBUT 户外操场：中区操场 + 西区操场；RoomType=GYM）
		//
		// Stage B B5-Final：不再引入 "GYM" 虚拟教学楼与 GYM_MAIN 教室 ——
		// 体育资源统一为独立 Building（sports category）+ 场地本身作为 Classroom。
		{"MIDDLE_FIELD", "中区操场", "FIELD_M", 1, "", 500, models.RoomTypeGym},
		{"WEST_FIELD", "西区操场", "FIELD_W", 1, "", 500, models.RoomTypeGym},
	}

	classrooms := make([]models.Classroom, 0, len(classroomSpecs))
	for _, s := range classroomSpecs {
		building, ok := buildingByCode[s.BuildingCode]
		if !ok {
			log.Printf("Seed: building %q not found for classroom %q — skip", s.BuildingCode, s.Code)
			continue
		}
		classrooms = append(classrooms, models.Classroom{
			Code:       s.Code,
			Name:       s.Name,
			BuildingID: building.ID,
			Floor:      s.Floor,
			Number:     s.Number,
			Capacity:   s.Capacity,
			RoomType:   s.RoomType,
			Status:     "available",
		})
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
		{"CS301", "T006", "7A-102", 0, 0, 2}, // 1-2 节 · 计算机 数据结构 · 7A 普通
		{"ME201", "T001", "1-001", 0, 2, 2},  // 3-4 节 · 机械设计 · 1 教阶梯（大班）
		{"EE201", "T002", "1-201", 0, 4, 2},  // 5-6 节 · 电路原理 · 1 教普通
		{"SC201", "T013", "1-001", 0, 6, 2},  // 7-8 节 · 高等数学 · 1 教阶梯（合班）
		// ---- Tuesday ----
		{"CE201", "T005", "5B-301", 1, 0, 2}, // 1-2 节 · 结构力学 · 5B 多媒体
		{"EM201", "T010", "1-302", 1, 2, 2},  // 3-4 节 · 西方经济学 · 1 教普通
		{"AD201", "T007", "7B-502", 1, 4, 2}, // 5-6 节 · 设计素描 · 7B 多媒体
		// ---- Wednesday ----
		{"LS201", "T004", "6A-213", 2, 0, 2}, // 1-2 节 · 生物化学 · 6A 实验室
		{"ID201", "T009", "5A-203", 2, 2, 2}, // 3-4 节 · 产品设计 · 5A 普通
		{"FL101", "T012", "5A-102", 2, 4, 2}, // 5-6 节 · 大学英语 · 5A 多媒体
		// ---- Thursday ----
		{"ET201", "T019", "6A-422", 3, 0, 2}, // 1-2 节 · 应用型工程实践 · 6A 机房
		{"PE101", "T014", "MIDDLE_FIELD", 3, 2, 2}, // 3-4 节 · 体育(篮球) · 中区操场（合班）
	}

	// TODO(v0.6.0): Rebuild seed demo entries using TimeAssignment + ScheduleEntry split model.
	// ScheduleEntry now requires TimeAssignmentID (not standalone CourseID/TeacherID/DayOfWeek/etc).
	// Seed data must first create TimeAssignment rows, then ScheduleEntry rows linking to them.
	_ = entries
	_ = semesterID
	_ = courseByCode
	_ = teacherByCode
	_ = roomByCode
	log.Println("Seed: v0.6.0 — demo schedule entries temporarily disabled (TA+SE split requires TimeAssignments first)")
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

func loadBuildingByCode(db DB) map[string]models.Building {
	var buildings []models.Building
	db.Find(&buildings)
	out := make(map[string]models.Building, len(buildings))
	for _, b := range buildings {
		out[b.Code] = b
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

// nextUpcomingSemester 根据当前时间返回下一个即将开学的学期，Status=planned。
// 规则（中国大陆高校常见节奏）：
//   - 每年 3 月 1 日到 8 月 31 日：下一个学期 = 当年秋季学期（AY = 当年-次年，Term=FIRST，默认 9 月 1 日开学）。
//   - 每年 9 月 1 日到次年 2 月末：下一个学期 = 次年春季学期（AY = 上一年-当年，Term=SECOND，
//     默认 2 月第三个周一开学 —— 避开春节假期）。
//
// 内置默认第一天只是"合理起点"，用户可在设置里编辑；用户改过之后
// 幂等 seed 不会覆盖（FirstOrCreate 只在同 AY+Term 缺席时插入）。
func nextUpcomingSemester(now time.Time) models.Semester {
	y := now.Year()
	m := now.Month()

	if m >= time.March && m <= time.August {
		// 下一个：当年 9 月 1 日开学的秋季学期
		start := time.Date(y, time.September, 1, 0, 0, 0, 0, time.UTC)
		return models.Semester{
			AcademicYear: firstTermAcademicYear(y),
			Term:         models.SemesterTermFirst,
			StartDate:    start,
			EndDate:      start.AddDate(0, 0, 18*7-1),
			Status:       models.SemesterStatusPlanned,
		}
	}
	// 9 月-次年 2 月：下一个 = 次年春季（Term=SECOND）
	springYear := y
	if m >= time.September {
		springYear = y + 1
	}
	start := nthMondayOfMonth(springYear, time.February, 3)
	return models.Semester{
		AcademicYear: secondTermAcademicYear(springYear),
		Term:         models.SemesterTermSecond,
		StartDate:    start,
		EndDate:      start.AddDate(0, 0, 18*7-1),
		Status:       models.SemesterStatusPlanned,
	}
}

// firstTermAcademicYear: 秋季学期学年 = 当年-次年，如 y=2026 → "2026-2027"。
func firstTermAcademicYear(y int) string {
	return itoa4(y) + "-" + itoa4(y+1)
}

// secondTermAcademicYear: 春季学期学年 = 上一年-当年，如 springYear=2027 → "2026-2027"。
func secondTermAcademicYear(springYear int) string {
	return itoa4(springYear-1) + "-" + itoa4(springYear)
}

// nthMondayOfMonth: 返回给定年月的第 n 个周一（1-based，UTC）。
func nthMondayOfMonth(y int, m time.Month, n int) time.Time {
	first := time.Date(y, m, 1, 0, 0, 0, 0, time.UTC)
	// Weekday(): Sun=0, Mon=1, …, Sat=6
	offset := (int(time.Monday) - int(first.Weekday()) + 7) % 7
	return first.AddDate(0, 0, offset+(n-1)*7)
}

// itoa4: 4 位年份格式化。避免引入 strconv 依赖爬到文件顶部（仅本文件内部使用）。
func itoa4(y int) string {
	// 年份始终 4 位；保守起见做一次 fmt-free 转换。
	buf := [4]byte{'0', '0', '0', '0'}
	i := 3
	for y > 0 && i >= 0 {
		buf[i] = byte('0' + y%10)
		y /= 10
		i--
	}
	return string(buf[:])
}
