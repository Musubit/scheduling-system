package database

import (
	"log"

	"scheduling-system/backend/models"
)

// SeedData initializes the database with sample data.
//
// It is designed to be idempotent and resilient:
//   - Base data (semesters/teachers/classrooms/courses/class-groups) is seeded
//     only when absent, so repeated calls never clobber user data or fail on
//     UNIQUE constraints.
//   - Teaching tasks are what drives the scheduling engine; they are always
//     (re)created when missing, as long as the base data exists.
//   - Demo schedule entries are seeded only on a fresh database.
func SeedData(db DB) {
	if db == nil {
		return
	}

	var teacherCount, entryCount int64
	db.Model(&models.Teacher{}).Count(&teacherCount)
	db.Model(&models.ScheduleEntry{}).Count(&entryCount)

	// Base data is seeded idempotently (FirstOrCreate), so it is always safe
	// to run — it adds any missing rows (e.g. newly added colleges) without
	// clobbering existing data.
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
func seedIfAbsent[T any](db DB, items []T, keyField string, keyVal func(T) interface{}) {
	for _, it := range items {
		var cnt int64
		db.Model(new(T)).Where(keyField+" = ?", keyVal(it)).Count(&cnt)
		if cnt == 0 {
			db.Create(&it)
		}
	}
}

// seedBaseData seeds semesters, teachers, classrooms, courses and class groups.
// Safe to call only when these tables are empty.
func seedBaseData(db DB) {
	// ===== Semesters =====
	semesters := []models.Semester{
		{Name: "2025-2026 第二学期", IsActive: true},
		{Name: "2025-2026 第一学期", IsActive: false},
		{Name: "2024-2025 第二学期", IsActive: false},
	}
	seedIfAbsent(db, semesters, "name", func(s models.Semester) interface{} { return s.Name })

	// ===== Teachers =====
	teachers := []models.Teacher{
		{Code: "T001", Name: "张建国", Dept: "机械工程学院", Status: "active", PreferNoEarly: true, PreferLowFloor: true, MaxDaysPerWeek: 3},
		{Code: "T002", Name: "李明远", Dept: "电气与电子工程学院", Status: "active", PreferNoEarly: true, PreferLowFloor: true, MaxDaysPerWeek: 3},
		{Code: "T003", Name: "王伟", Dept: "材料与化学工程学院", Status: "active", MaxDaysPerWeek: 3},
		{Code: "T004", Name: "刘芳", Dept: "外国语学院", Status: "active", PreferNoEarly: true, MaxDaysPerWeek: 3},
		{Code: "T005", Name: "赵秀英", Dept: "理学院", Status: "active", MaxDaysPerWeek: 3},
		{Code: "T006", Name: "孙志强", Dept: "经济与管理学院", Status: "active", PreferLowFloor: true, MaxDaysPerWeek: 3},
		{Code: "T007", Name: "周海", Dept: "计算机学院", Status: "active", MaxDaysPerWeek: 3},
		{Code: "T008", Name: "钱学林", Dept: "生物工程与食品学院", Status: "active", PreferNoLate: true, PreferLowFloor: true, MaxDaysPerWeek: 3},
		{Code: "T009", Name: "吴芳", Dept: "马克思主义学院", Status: "active", MaxDaysPerWeek: 3},
		{Code: "T010", Name: "郑美", Dept: "艺术设计学院", Status: "active", MaxDaysPerWeek: 3},
		{Code: "T011", Name: "陈刚", Dept: "体育学院", Status: "active", MaxDaysPerWeek: 3},
		{Code: "T012", Name: "杨华", Dept: "土木建筑与环境学院", Status: "active", PreferNoEarly: true, PreferLowFloor: true, MaxDaysPerWeek: 3},
		{Code: "T013", Name: "黄蕾", Dept: "工业设计学院", Status: "active", MaxDaysPerWeek: 3},
		{Code: "T014", Name: "周敏", Dept: "职业技术师范学院", Status: "active", MaxDaysPerWeek: 3},
		{Code: "T015", Name: "李娜", Dept: "国际学院", Status: "active", MaxDaysPerWeek: 3},
		{Code: "T016", Name: "王芳", Dept: "继续教育学院", Status: "active", MaxDaysPerWeek: 3},
		{Code: "T017", Name: "陈晨", Dept: "创新创业学院", Status: "active", MaxDaysPerWeek: 3},
		{Code: "T018", Name: "刘强", Dept: "工程技术学院", Status: "active", MaxDaysPerWeek: 3},
		{Code: "T019", Name: "张伟", Dept: "底特律绿色工业学院", Status: "active", MaxDaysPerWeek: 3},
	}
	seedIfAbsent(db, teachers, "code", func(t models.Teacher) interface{} { return t.Code })

	// ===== Classrooms =====
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
		{Code: "GYM01", Name: "体育馆", Building: "体育馆", Floor: 1, Capacity: 300, Type: "体育馆", Status: "available"},
	}
	seedIfAbsent(db, classrooms, "code", func(c models.Classroom) interface{} { return c.Code })

	// ===== Courses =====
	courses := []models.Course{
		{Code: "ME201", Name: "机械设计基础", Dept: "机械工程学院", Credit: 4.0, Type: "专业必修", Hours: 64, Status: "active"},
		{Code: "ME301", Name: "数控技术", Dept: "机械工程学院", Credit: 3.0, Type: "专业必修", Hours: 48, Status: "active"},
		{Code: "EE201", Name: "电路原理", Dept: "电气与电子工程学院", Credit: 4.0, Type: "专业必修", Hours: 64, Status: "active"},
		{Code: "EE301", Name: "电力系统分析", Dept: "电气与电子工程学院", Credit: 3.0, Type: "专业必修", Hours: 48, Status: "active"},
		{Code: "MC201", Name: "有机化学", Dept: "材料与化学工程学院", Credit: 4.0, Type: "专业必修", Hours: 64, Status: "active"},
		{Code: "BF201", Name: "生物化学", Dept: "生物工程与食品学院", Credit: 4.0, Type: "专业必修", Hours: 64, Status: "active"},
		{Code: "CE201", Name: "结构力学", Dept: "土木建筑与环境学院", Credit: 4.0, Type: "专业必修", Hours: 64, Status: "active"},
		{Code: "CE301", Name: "工程制图", Dept: "土木建筑与环境学院", Credit: 3.0, Type: "专业必修", Hours: 48, Status: "active"},
		{Code: "CS301", Name: "数据结构", Dept: "计算机学院", Credit: 4.0, Type: "专业必修", Hours: 64, Status: "active"},
		{Code: "CS302", Name: "操作系统", Dept: "计算机学院", Credit: 4.0, Type: "专业必修", Hours: 64, Status: "active"},
		{Code: "CS303", Name: "计算机网络", Dept: "计算机学院", Credit: 3.0, Type: "专业必修", Hours: 48, Status: "active"},
		{Code: "AD201", Name: "设计素描", Dept: "艺术设计学院", Credit: 3.0, Type: "专业必修", Hours: 48, Status: "active"},
		{Code: "ID201", Name: "产品设计", Dept: "工业设计学院", Credit: 3.0, Type: "专业必修", Hours: 48, Status: "active"},
		{Code: "EM201", Name: "西方经济学", Dept: "经济与管理学院", Credit: 3.0, Type: "专业必修", Hours: 48, Status: "active"},
		{Code: "EM202", Name: "财务管理", Dept: "经济与管理学院", Credit: 3.0, Type: "专业必修", Hours: 48, Status: "active"},
		{Code: "EN101", Name: "大学英语", Dept: "外国语学院", Credit: 3.0, Type: "公共必修", Hours: 48, Status: "active"},
		{Code: "EN102", Name: "英语听说", Dept: "外国语学院", Credit: 2.0, Type: "公共必修", Hours: 32, Status: "active"},
		{Code: "SC201", Name: "高等数学", Dept: "理学院", Credit: 5.0, Type: "公共必修", Hours: 80, Status: "active"},
		{Code: "SC202", Name: "线性代数", Dept: "理学院", Credit: 3.0, Type: "公共必修", Hours: 48, Status: "active"},
		{Code: "SC203", Name: "大学物理", Dept: "理学院", Credit: 4.0, Type: "公共必修", Hours: 64, Status: "active"},
		{Code: "MX101", Name: "马克思主义基本原理", Dept: "马克思主义学院", Credit: 2.0, Type: "公共必修", Hours: 32, Status: "active"},
		{Code: "MX102", Name: "形势与政策", Dept: "马克思主义学院", Credit: 1.0, Type: "公共必修", Hours: 16, Status: "active"},
		{Code: "PE101", Name: "体育(篮球)", Dept: "体育学院", Credit: 1.0, Type: "公共必修", Hours: 32, Status: "active"},
		{Code: "VE201", Name: "职业教育学", Dept: "职业技术师范学院", Credit: 3.0, Type: "专业必修", Hours: 48, Status: "active"},
		{Code: "VE301", Name: "课程设计与开发", Dept: "职业技术师范学院", Credit: 3.0, Type: "专业必修", Hours: 48, Status: "active"},
		{Code: "IN201", Name: "跨文化交际", Dept: "国际学院", Credit: 2.0, Type: "公共必修", Hours: 32, Status: "active"},
		{Code: "IN301", Name: "国际商务", Dept: "国际学院", Credit: 3.0, Type: "专业必修", Hours: 48, Status: "active"},
		{Code: "CO201", Name: "成人教育学", Dept: "继续教育学院", Credit: 3.0, Type: "专业必修", Hours: 48, Status: "active"},
		{Code: "CO301", Name: "继续教育管理", Dept: "继续教育学院", Credit: 2.0, Type: "专业必修", Hours: 32, Status: "active"},
		{Code: "IV201", Name: "创新创业基础", Dept: "创新创业学院", Credit: 2.0, Type: "公共必修", Hours: 32, Status: "active"},
		{Code: "IV301", Name: "创业实践", Dept: "创新创业学院", Credit: 3.0, Type: "专业必修", Hours: 48, Status: "active"},
		{Code: "ET201", Name: "工程训练", Dept: "工程技术学院", Credit: 3.0, Type: "专业必修", Hours: 48, Status: "active"},
		{Code: "ET301", Name: "智能制造概论", Dept: "工程技术学院", Credit: 3.0, Type: "专业必修", Hours: 48, Status: "active"},
		{Code: "DT201", Name: "绿色制造", Dept: "底特律绿色工业学院", Credit: 3.0, Type: "专业必修", Hours: 48, Status: "active"},
		{Code: "DT301", Name: "新能源汽车技术", Dept: "底特律绿色工业学院", Credit: 3.0, Type: "专业必修", Hours: 48, Status: "active"},
	}
	seedIfAbsent(db, courses, "code", func(c models.Course) interface{} { return c.Code })

	// ===== Class Groups =====
	groups := []models.ClassGroup{
		{Code: "CS2301", Name: "计算机2301", Dept: "计算机学院", Grade: 2023, Students: 86, Status: "active"},
		{Code: "CS2302", Name: "计算机2302", Dept: "计算机学院", Grade: 2023, Students: 82, Status: "active"},
		{Code: "ME2301", Name: "机械2301", Dept: "机械工程学院", Grade: 2023, Students: 72, Status: "active"},
		{Code: "EE2301", Name: "电气2301", Dept: "电气与电子工程学院", Grade: 2023, Students: 68, Status: "active"},
		{Code: "CE2301", Name: "土木2301", Dept: "土木建筑与环境学院", Grade: 2023, Students: 55, Status: "active"},
		{Code: "EM2301", Name: "经管2301", Dept: "经济与管理学院", Grade: 2023, Students: 78, Status: "active"},
		{Code: "AD2301", Name: "艺设2301", Dept: "艺术设计学院", Grade: 2023, Students: 40, Status: "active"},
		{Code: "VE2301", Name: "职师2301", Dept: "职业技术师范学院", Grade: 2023, Students: 50, Status: "active"},
		{Code: "IN2301", Name: "国际2301", Dept: "国际学院", Grade: 2023, Students: 45, Status: "active"},
		{Code: "CO2301", Name: "继教2301", Dept: "继续教育学院", Grade: 2023, Students: 60, Status: "active"},
		{Code: "IV2301", Name: "双创2301", Dept: "创新创业学院", Grade: 2023, Students: 40, Status: "active"},
		{Code: "ET2301", Name: "工程2301", Dept: "工程技术学院", Grade: 2023, Students: 55, Status: "active"},
		{Code: "DT2301", Name: "底特律2301", Dept: "底特律绿色工业学院", Grade: 2023, Students: 45, Status: "active"},
	}
	seedIfAbsent(db, groups, "code", func(g models.ClassGroup) interface{} { return g.Code })
}

// seedTeachingTasks creates the demo teaching tasks that drive the scheduling
// engine. It loads the base data from the database (by ID order, which matches
// the seed order) so it works whether or not seedBaseData just ran in the same
// call. Each task is assigned one class group.
func seedTeachingTasks(db DB) {
	var courses []models.Course
	var teachers []models.Teacher
	var groups []models.ClassGroup
	var semesters []models.Semester
	db.Order("id asc").Find(&courses)
	db.Order("id asc").Find(&teachers)
	db.Order("id asc").Find(&groups)
	db.Order("id asc").Find(&semesters)

	if len(courses) == 0 || len(teachers) == 0 || len(groups) == 0 {
		log.Println("Seed: skip teaching tasks (base data missing)")
		return
	}

	activeSemesterID := uint(0)
	for _, s := range semesters {
		if s.IsActive {
			activeSemesterID = s.ID
			break
		}
	}
	if activeSemesterID == 0 && len(semesters) > 0 {
		activeSemesterID = semesters[0].ID
	}

	// (课程, 教师, 班级) 组合，对应演示课表；Hours 决定 SA 每周节数。
	type seedTask struct {
		CourseIdx  int
		TeacherIdx int
		GroupIdx   int
		Hours      int
	}
	tasksSpec := []seedTask{
		{18, 5, 1, 80},  // 高等数学 赵秀英 CS2301
		{9, 7, 1, 64},   // 数据结构 周海 CS2301
		{16, 4, 6, 48},  // 大学英语 刘芳 EM2301
		{23, 11, 1, 32}, // 体育(篮球) 陈刚 CS2301
		{19, 5, 2, 48},  // 线性代数 赵秀英 CS2302
		{3, 2, 4, 64},   // 电路原理 李明远 EE2301
		{20, 8, 4, 64},  // 大学物理 钱学林 EE2301
		{14, 6, 6, 48},  // 西方经济学 孙志强 EM2301
		{10, 7, 2, 64},  // 操作系统 周海 CS2302
		{6, 8, 5, 64},   // 生物化学 钱学林 CE2301
		{21, 9, 7, 32},  // 马克思主义基本原理 吴芳 AD2301
		{12, 10, 7, 48}, // 设计素描 郑美 AD2301
		{1, 1, 3, 64},   // 机械设计基础 张建国 ME2301
		{7, 12, 5, 64},  // 结构力学 杨华 CE2301
		{11, 7, 1, 48},  // 计算机网络 周海 CS2301
		{15, 6, 6, 48},  // 财务管理 孙志强 EM2301
		{4, 2, 4, 48},   // 电力系统分析 李明远 EE2301
		{17, 4, 7, 32},  // 英语听说 刘芳 AD2301
		{5, 3, 3, 64},   // 有机化学 王伟 ME2301
		{22, 9, 7, 16},  // 形势与政策 吴芳 AD2301
		{13, 13, 7, 48}, // 产品设计 黄蕾 AD2301
		{2, 1, 3, 48},   // 数控技术 张建国 ME2301
		// ===== 新增学院（补齐湖工大 19 学院）=====
		{24, 14, 8, 48},  // 职业教育学 周敏 VE2301
		{25, 14, 8, 48},  // 课程设计与开发 周敏 VE2301
		{26, 15, 9, 32},  // 跨文化交际 李娜 IN2301
		{27, 15, 9, 48},  // 国际商务 李娜 IN2301
		{28, 16, 10, 48}, // 成人教育学 王芳 CO2301
		{29, 16, 10, 32}, // 继续教育管理 王芳 CO2301
		{30, 17, 11, 32}, // 创新创业基础 陈晨 IV2301
		{31, 17, 11, 48}, // 创业实践 陈晨 IV2301
		{32, 18, 12, 48}, // 工程训练 刘强 ET2301
		{33, 18, 12, 48}, // 智能制造概论 刘强 ET2301
		{34, 19, 13, 48}, // 绿色制造 张伟 DT2301
		{35, 19, 13, 48}, // 新能源汽车技术 张伟 DT2301
	}
	for _, spec := range tasksSpec {
		courseID := courses[spec.CourseIdx-1].ID
		teacherID := teachers[spec.TeacherIdx-1].ID
		var existing int64
		db.Model(&models.TeachingTask{}).Where("course_id = ? AND teacher_id = ? AND semester_id = ?", courseID, teacherID, activeSemesterID).Count(&existing)
		if existing > 0 {
			continue
		}
		task := models.TeachingTask{
			CourseID:        courses[spec.CourseIdx-1].ID,
			TeacherID:       teachers[spec.TeacherIdx-1].ID,
			SemesterID:      activeSemesterID,
			Status:          "active",
			TotalHours:      spec.Hours,
			StartWeek:       1,
			EndWeek:         16,
			MaxHoursPerWeek: 0,
		}
		if err := db.Create(&task).Error(); err != nil {
			log.Printf("Seed: create teaching task failed: %v", err)
			continue
		}
		class := models.TeachingTaskClass{
			TeachingTaskID: task.ID,
			ClassGroupID:   groups[spec.GroupIdx-1].ID,
		}
		if err := db.Create(&class).Error(); err != nil {
			log.Printf("Seed: create teaching task class failed: %v", err)
		}
	}
}

// seedDemoEntries seeds sample schedule entries directly. These mirror the
// teaching tasks above. When the user clicks "开始自动排课", RunScheduling
// clears this semester's entries and writes the engine output instead.
func seedDemoEntries(db DB) {
	// Monday: 1-2=0, 3-4=2, 5-6=4, 7-8=6, 9-10=8
	entries := []models.ScheduleEntry{
		{CourseID: 18, TeacherID: 5, ClassroomID: 1, Semester: "2025-2026 第二学期", DayOfWeek: 0, StartPeriod: 0, Span: 2, Weeks: "1-16"}, // 高等数学 赵秀英
		{CourseID: 9, TeacherID: 7, ClassroomID: 7, Semester: "2025-2026 第二学期", DayOfWeek: 0, StartPeriod: 2, Span: 2, Weeks: "1-16"},  // 数据结构 周海
		{CourseID: 16, TeacherID: 4, ClassroomID: 4, Semester: "2025-2026 第二学期", DayOfWeek: 0, StartPeriod: 4, Span: 2, Weeks: "1-16"}, // 大学英语 刘芳
		{CourseID: 23, TeacherID: 11, ClassroomID: 11, Semester: "2025-2026 第二学期", DayOfWeek: 0, StartPeriod: 8, Span: 2, Weeks: "1-16"}, // 体育 陈刚
		// Tuesday
		{CourseID: 19, TeacherID: 5, ClassroomID: 3, Semester: "2025-2026 第二学期", DayOfWeek: 1, StartPeriod: 0, Span: 2, Weeks: "1-16"}, // 线性代数 赵秀英
		{CourseID: 3, TeacherID: 2, ClassroomID: 2, Semester: "2025-2026 第二学期", DayOfWeek: 1, StartPeriod: 2, Span: 2, Weeks: "1-16"},  // 电路原理 李明远
		{CourseID: 20, TeacherID: 8, ClassroomID: 6, Semester: "2025-2026 第二学期", DayOfWeek: 1, StartPeriod: 4, Span: 2, Weeks: "1-16"}, // 大学物理 钱学林
		{CourseID: 14, TeacherID: 6, ClassroomID: 9, Semester: "2025-2026 第二学期", DayOfWeek: 1, StartPeriod: 6, Span: 2, Weeks: "1-16"}, // 西方经济学 孙志强
		// Wednesday
		{CourseID: 10, TeacherID: 7, ClassroomID: 7, Semester: "2025-2026 第二学期", DayOfWeek: 2, StartPeriod: 0, Span: 2, Weeks: "1-16"}, // 操作系统 周海
		{CourseID: 6, TeacherID: 8, ClassroomID: 6, Semester: "2025-2026 第二学期", DayOfWeek: 2, StartPeriod: 2, Span: 2, Weeks: "1-16"},  // 生物化学 钱学林
		{CourseID: 21, TeacherID: 9, ClassroomID: 9, Semester: "2025-2026 第二学期", DayOfWeek: 2, StartPeriod: 4, Span: 2, Weeks: "1-16"}, // 马原 吴芳
		{CourseID: 12, TeacherID: 10, ClassroomID: 10, Semester: "2025-2026 第二学期", DayOfWeek: 2, StartPeriod: 6, Span: 2, Weeks: "1-16"}, // 设计素描 郑美
		// Thursday
		{CourseID: 1, TeacherID: 1, ClassroomID: 5, Semester: "2025-2026 第二学期", DayOfWeek: 3, StartPeriod: 0, Span: 2, Weeks: "1-16"},  // 机械设计 张建国
		{CourseID: 7, TeacherID: 12, ClassroomID: 6, Semester: "2025-2026 第二学期", DayOfWeek: 3, StartPeriod: 2, Span: 2, Weeks: "1-16"}, // 结构力学 杨华
		{CourseID: 11, TeacherID: 7, ClassroomID: 7, Semester: "2025-2026 第二学期", DayOfWeek: 3, StartPeriod: 4, Span: 2, Weeks: "1-16"}, // 计算机网络 周海
		{CourseID: 15, TeacherID: 6, ClassroomID: 8, Semester: "2025-2026 第二学期", DayOfWeek: 3, StartPeriod: 6, Span: 2, Weeks: "1-16"}, // 财务管理 孙志强
		// Friday
		{CourseID: 4, TeacherID: 2, ClassroomID: 2, Semester: "2025-2026 第二学期", DayOfWeek: 4, StartPeriod: 0, Span: 2, Weeks: "1-16"},  // 电力系统 李明远
		{CourseID: 17, TeacherID: 4, ClassroomID: 4, Semester: "2025-2026 第二学期", DayOfWeek: 4, StartPeriod: 2, Span: 2, Weeks: "1-16"}, // 英语听说 刘芳
		{CourseID: 5, TeacherID: 3, ClassroomID: 10, Semester: "2025-2026 第二学期", DayOfWeek: 4, StartPeriod: 4, Span: 2, Weeks: "1-16"},  // 有机化学 王伟
		{CourseID: 22, TeacherID: 9, ClassroomID: 9, Semester: "2025-2026 第二学期", DayOfWeek: 4, StartPeriod: 6, Span: 1, Weeks: "1-16"}, // 形势与政策 吴芳
		// Saturday
		{CourseID: 13, TeacherID: 13, ClassroomID: 10, Semester: "2025-2026 第二学期", DayOfWeek: 5, StartPeriod: 2, Span: 2, Weeks: "1-16"}, // 产品设计 黄蕾
		{CourseID: 2, TeacherID: 1, ClassroomID: 5, Semester: "2025-2026 第二学期", DayOfWeek: 5, StartPeriod: 4, Span: 2, Weeks: "1-16"},  // 数控技术 张建国
	}
	db.Create(&entries)
}
