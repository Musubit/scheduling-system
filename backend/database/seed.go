package database

import "scheduling-system/backend/models"

// SeedData initializes the database with sample data.
func SeedData(db DB) {
	if db == nil {
		return
	}

	// Only seed if no data exists
	var count int64
	db.Model(&models.Teacher{}).Count(&count)
	if count > 0 {
		return
	}

	// ===== Semesters =====
	semesters := []models.Semester{
		{Name: "2025-2026 第二学期", IsActive: true},
		{Name: "2025-2026 第一学期", IsActive: false},
		{Name: "2024-2025 第二学期", IsActive: false},
	}
	db.Create(&semesters)

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
	}
	db.Create(&teachers)

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
	db.Create(&classrooms)

	// ===== Courses =====
	courses := []models.Course{
		{Code: "ME201", Name: "机械设计基础", Dept: "mech", Credit: 4.0, Type: "专业必修", Hours: 64, Status: "active"},
		{Code: "ME301", Name: "数控技术", Dept: "mech", Credit: 3.0, Type: "专业必修", Hours: 48, Status: "active"},
		{Code: "EE201", Name: "电路原理", Dept: "elec", Credit: 4.0, Type: "专业必修", Hours: 64, Status: "active"},
		{Code: "EE301", Name: "电力系统分析", Dept: "elec", Credit: 3.0, Type: "专业必修", Hours: 48, Status: "active"},
		{Code: "MC201", Name: "有机化学", Dept: "mate", Credit: 4.0, Type: "专业必修", Hours: 64, Status: "active"},
		{Code: "BF201", Name: "生物化学", Dept: "bio", Credit: 4.0, Type: "专业必修", Hours: 64, Status: "active"},
		{Code: "CE201", Name: "结构力学", Dept: "civil", Credit: 4.0, Type: "专业必修", Hours: 64, Status: "active"},
		{Code: "CE301", Name: "工程制图", Dept: "civil", Credit: 3.0, Type: "专业必修", Hours: 48, Status: "active"},
		{Code: "CS301", Name: "数据结构", Dept: "cs", Credit: 4.0, Type: "专业必修", Hours: 64, Status: "active"},
		{Code: "CS302", Name: "操作系统", Dept: "cs", Credit: 4.0, Type: "专业必修", Hours: 64, Status: "active"},
		{Code: "CS303", Name: "计算机网络", Dept: "cs", Credit: 3.0, Type: "专业必修", Hours: 48, Status: "active"},
		{Code: "AD201", Name: "设计素描", Dept: "art", Credit: 3.0, Type: "专业必修", Hours: 48, Status: "active"},
		{Code: "ID201", Name: "产品设计", Dept: "design", Credit: 3.0, Type: "专业必修", Hours: 48, Status: "active"},
		{Code: "EM201", Name: "西方经济学", Dept: "econ", Credit: 3.0, Type: "专业必修", Hours: 48, Status: "active"},
		{Code: "EM202", Name: "财务管理", Dept: "econ", Credit: 3.0, Type: "专业必修", Hours: 48, Status: "active"},
		{Code: "EN101", Name: "大学英语", Dept: "eng", Credit: 3.0, Type: "公共必修", Hours: 48, Status: "active"},
		{Code: "EN102", Name: "英语听说", Dept: "eng", Credit: 2.0, Type: "公共必修", Hours: 32, Status: "active"},
		{Code: "SC201", Name: "高等数学", Dept: "sci", Credit: 5.0, Type: "公共必修", Hours: 80, Status: "active"},
		{Code: "SC202", Name: "线性代数", Dept: "sci", Credit: 3.0, Type: "公共必修", Hours: 48, Status: "active"},
		{Code: "SC203", Name: "大学物理", Dept: "sci", Credit: 4.0, Type: "公共必修", Hours: 64, Status: "active"},
		{Code: "MX101", Name: "马克思主义基本原理", Dept: "marx", Credit: 2.0, Type: "公共必修", Hours: 32, Status: "active"},
		{Code: "MX102", Name: "形势与政策", Dept: "marx", Credit: 1.0, Type: "公共必修", Hours: 16, Status: "active"},
		{Code: "PE101", Name: "体育(篮球)", Dept: "pe", Credit: 1.0, Type: "公共必修", Hours: 32, Status: "active"},
	}
	db.Create(&courses)

	// ===== Class Groups =====
	groups := []models.ClassGroup{
		{Code: "CS2301", Name: "计算机2301", Dept: "计算机学院", Grade: 2023, Students: 86, Status: "active"},
		{Code: "CS2302", Name: "计算机2302", Dept: "计算机学院", Grade: 2023, Students: 82, Status: "active"},
		{Code: "ME2301", Name: "机械2301", Dept: "机械工程学院", Grade: 2023, Students: 72, Status: "active"},
		{Code: "EE2301", Name: "电气2301", Dept: "电气与电子工程学院", Grade: 2023, Students: 68, Status: "active"},
		{Code: "CE2301", Name: "土木2301", Dept: "土木建筑与环境学院", Grade: 2023, Students: 55, Status: "active"},
		{Code: "EM2301", Name: "经管2301", Dept: "经济与管理学院", Grade: 2023, Students: 78, Status: "active"},
		{Code: "AD2301", Name: "艺设2301", Dept: "艺术设计学院", Grade: 2023, Students: 40, Status: "active"},
	}
	db.Create(&groups)

	// ===== Schedule Entries (连上规则：startPeriod 必须是 0/2/4/6/8) =====
	entries := []models.ScheduleEntry{
		// Monday: 1-2=0, 3-4=2, 5-6=4, 7-8=6, 9-10=8
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
