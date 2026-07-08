package database

import "scheduling-system/models"

// SeedData initializes the database with sample data.
func SeedData() {
	if DB == nil {
		return
	}

	// Only seed if no data exists
	var count int64
	DB.Model(&models.Teacher{}).Count(&count)
	if count > 0 {
		return
	}

	// ===== Semesters =====
	semesters := []models.Semester{
		{Name: "2025-2026 第二学期", IsActive: true},
		{Name: "2025-2026 第一学期", IsActive: false},
		{Name: "2024-2025 第二学期", IsActive: false},
	}
	DB.Create(&semesters)

	// ===== Teachers =====
	teachers := []models.Teacher{
		{Code: "T001", Name: "王建国", Dept: "数学与统计学院", Title: "教授", Status: "active"},
		{Code: "T002", Name: "张明远", Dept: "计算机科学学院", Title: "教授", Status: "active"},
		{Code: "T003", Name: "李伟", Dept: "计算机科学学院", Title: "副教授", Status: "active"},
		{Code: "T004", Name: "刘芳", Dept: "外国语学院", Title: "讲师", Status: "active"},
		{Code: "T005", Name: "赵秀英", Dept: "数学与统计学院", Title: "副教授", Status: "active"},
		{Code: "T006", Name: "孙志强", Dept: "经济管理学院", Title: "教授", Status: "active"},
		{Code: "T007", Name: "周海", Dept: "计算机科学学院", Title: "副教授", Status: "active"},
		{Code: "T008", Name: "钱学森", Dept: "物理学院", Title: "教授", Status: "active"},
		{Code: "T009", Name: "吴芳", Dept: "法学院", Title: "讲师", Status: "inactive"},
		{Code: "T010", Name: "郑美", Dept: "艺术学院", Title: "副教授", Status: "active"},
		{Code: "T011", Name: "陈刚", Dept: "教育学院", Title: "讲师", Status: "active"},
	}
	DB.Create(&teachers)

	// ===== Classrooms =====
	classrooms := []models.Classroom{
		{Code: "A301", Name: "A301", Building: "A栋", Capacity: 80, Type: "普通教室", Status: "available"},
		{Code: "A201", Name: "A201", Building: "A栋", Capacity: 90, Type: "普通教室", Status: "available"},
		{Code: "B205", Name: "B205", Building: "B栋", Capacity: 60, Type: "普通教室", Status: "available"},
		{Code: "B108", Name: "B108", Building: "B栋", Capacity: 100, Type: "多媒体教室", Status: "available"},
		{Code: "B301", Name: "B301", Building: "B栋", Capacity: 70, Type: "普通教室", Status: "available"},
		{Code: "C301", Name: "C301", Building: "C栋", Capacity: 100, Type: "多媒体教室", Status: "available"},
		{Code: "C502", Name: "C502", Building: "C栋", Capacity: 120, Type: "多媒体教室", Status: "available"},
		{Code: "D102", Name: "D102", Building: "D栋", Capacity: 80, Type: "普通教室", Status: "available"},
		{Code: "D401", Name: "D401", Building: "D栋", Capacity: 200, Type: "阶梯教室", Status: "available"},
		{Code: "E101", Name: "E101", Building: "E栋", Capacity: 50, Type: "实验室", Status: "available"},
		{Code: "GYM01", Name: "体育馆", Building: "体育馆", Capacity: 300, Type: "体育馆", Status: "available"},
	}
	DB.Create(&classrooms)

	// ===== Courses =====
	courses := []models.Course{
		{Code: "CS301", Name: "数据结构", Dept: "cs", Credit: 4.0, Type: "专业必修", Hours: 64},
		{Code: "CS302", Name: "操作系统", Dept: "cs", Credit: 4.0, Type: "专业必修", Hours: 64},
		{Code: "CS303", Name: "计算机组成原理", Dept: "cs", Credit: 4.0, Type: "专业必修", Hours: 64},
		{Code: "CS304", Name: "算法设计与分析", Dept: "cs", Credit: 3.0, Type: "专业必修", Hours: 48},
		{Code: "CS305", Name: "编译原理", Dept: "cs", Credit: 4.0, Type: "专业必修", Hours: 64},
		{Code: "CS306", Name: "软件工程", Dept: "cs", Credit: 3.0, Type: "专业必修", Hours: 48},
		{Code: "CS307", Name: "程序设计实践", Dept: "cs", Credit: 2.0, Type: "专业选修", Hours: 32},
		{Code: "MATH201", Name: "高等数学", Dept: "math", Credit: 5.0, Type: "专业必修", Hours: 80},
		{Code: "MATH202", Name: "线性代数", Dept: "math", Credit: 3.0, Type: "专业必修", Hours: 48},
		{Code: "MATH203", Name: "概率论与数理统计", Dept: "math", Credit: 3.0, Type: "专业必修", Hours: 48},
		{Code: "MATH204", Name: "离散数学", Dept: "math", Credit: 3.0, Type: "专业必修", Hours: 48},
		{Code: "MATH205", Name: "数学建模", Dept: "math", Credit: 2.0, Type: "专业选修", Hours: 32},
		{Code: "PHY201", Name: "大学物理", Dept: "phys", Credit: 4.0, Type: "专业必修", Hours: 64},
		{Code: "ENG101", Name: "大学英语", Dept: "eng", Credit: 3.0, Type: "公共必修", Hours: 48},
		{Code: "ENG102", Name: "英语听说", Dept: "eng", Credit: 2.0, Type: "公共必修", Hours: 32},
		{Code: "ART201", Name: "艺术鉴赏", Dept: "art", Credit: 2.0, Type: "全校选修", Hours: 32},
		{Code: "ART202", Name: "油画基础", Dept: "art", Credit: 2.0, Type: "全校选修", Hours: 32},
		{Code: "ECO201", Name: "西方经济学", Dept: "eco", Credit: 3.0, Type: "专业必修", Hours: 48},
		{Code: "ECO202", Name: "财务管理", Dept: "eco", Credit: 3.0, Type: "专业必修", Hours: 48},
		{Code: "LAW101", Name: "马克思主义基本原理", Dept: "law", Credit: 2.0, Type: "公共必修", Hours: 32},
		{Code: "LAW102", Name: "形势与政策", Dept: "law", Credit: 1.0, Type: "公共必修", Hours: 16},
		{Code: "EDU201", Name: "体育(篮球)", Dept: "edu", Credit: 1.0, Type: "公共必修", Hours: 32},
	}
	DB.Create(&courses)

	// ===== Class Groups =====
	groups := []models.ClassGroup{
		{Code: "CS2301", Name: "计算机2301", Dept: "计算机科学学院", Grade: 2023, Students: 86},
		{Code: "CS2302", Name: "计算机2302", Dept: "计算机科学学院", Grade: 2023, Students: 82},
		{Code: "MATH2301", Name: "数学2301", Dept: "数学与统计学院", Grade: 2023, Students: 65},
		{Code: "MATH2302", Name: "数学2302", Dept: "数学与统计学院", Grade: 2023, Students: 58},
		{Code: "PHY2301", Name: "物理2301", Dept: "物理学院", Grade: 2023, Students: 45},
		{Code: "ECO2301", Name: "经济2301", Dept: "经济管理学院", Grade: 2023, Students: 72},
		{Code: "ECO2302", Name: "经济2302", Dept: "经济管理学院", Grade: 2023, Students: 68},
	}
	DB.Create(&groups)

	// ===== Schedule Entries =====
	entries := []models.ScheduleEntry{
		// Monday
		{CourseID: 8, TeacherID: 1, ClassroomID: 1, Semester: "2025-2026 第二学期", DayOfWeek: 0, StartPeriod: 0, Span: 2, Weeks: "1-16"},
		{CourseID: 1, TeacherID: 2, ClassroomID: 7, Semester: "2025-2026 第二学期", DayOfWeek: 0, StartPeriod: 2, Span: 2, Weeks: "1-16"},
		{CourseID: 14, TeacherID: 4, ClassroomID: 4, Semester: "2025-2026 第二学期", DayOfWeek: 0, StartPeriod: 4, Span: 2, Weeks: "1-16"},
		{CourseID: 22, TeacherID: 11, ClassroomID: 11, Semester: "2025-2026 第二学期", DayOfWeek: 0, StartPeriod: 8, Span: 2, Weeks: "1-16"},
		// Tuesday
		{CourseID: 9, TeacherID: 1, ClassroomID: 3, Semester: "2025-2026 第二学期", DayOfWeek: 1, StartPeriod: 0, Span: 2, Weeks: "1-16"},
		{CourseID: 3, TeacherID: 3, ClassroomID: 1, Semester: "2025-2026 第二学期", DayOfWeek: 1, StartPeriod: 2, Span: 2, Weeks: "1-16"},
		{CourseID: 10, TeacherID: 5, ClassroomID: 6, Semester: "2025-2026 第二学期", DayOfWeek: 1, StartPeriod: 5, Span: 2, Weeks: "1-16"},
		{CourseID: 18, TeacherID: 6, ClassroomID: 8, Semester: "2025-2026 第二学期", DayOfWeek: 1, StartPeriod: 8, Span: 2, Weeks: "1-16"},
		// Wednesday
		{CourseID: 2, TeacherID: 7, ClassroomID: 7, Semester: "2025-2026 第二学期", DayOfWeek: 2, StartPeriod: 0, Span: 2, Weeks: "1-16"},
		{CourseID: 13, TeacherID: 8, ClassroomID: 2, Semester: "2025-2026 第二学期", DayOfWeek: 2, StartPeriod: 2, Span: 2, Weeks: "1-16"},
		{CourseID: 20, TeacherID: 9, ClassroomID: 9, Semester: "2025-2026 第二学期", DayOfWeek: 2, StartPeriod: 4, Span: 2, Weeks: "1-16"},
		{CourseID: 16, TeacherID: 10, ClassroomID: 10, Semester: "2025-2026 第二学期", DayOfWeek: 2, StartPeriod: 6, Span: 2, Weeks: "1-16"},
		// Thursday
		{CourseID: 4, TeacherID: 2, ClassroomID: 5, Semester: "2025-2026 第二学期", DayOfWeek: 3, StartPeriod: 0, Span: 2, Weeks: "1-16"},
		{CourseID: 11, TeacherID: 5, ClassroomID: 3, Semester: "2025-2026 第二学期", DayOfWeek: 3, StartPeriod: 2, Span: 2, Weeks: "1-16"},
		{CourseID: 19, TeacherID: 6, ClassroomID: 8, Semester: "2025-2026 第二学期", DayOfWeek: 3, StartPeriod: 6, Span: 2, Weeks: "1-16"},
		// Friday
		{CourseID: 5, TeacherID: 3, ClassroomID: 5, Semester: "2025-2026 第二学期", DayOfWeek: 4, StartPeriod: 0, Span: 2, Weeks: "1-16"},
		{CourseID: 15, TeacherID: 4, ClassroomID: 4, Semester: "2025-2026 第二学期", DayOfWeek: 4, StartPeriod: 2, Span: 2, Weeks: "1-16"},
		{CourseID: 6, TeacherID: 7, ClassroomID: 7, Semester: "2025-2026 第二学期", DayOfWeek: 4, StartPeriod: 4, Span: 2, Weeks: "1-16"},
		{CourseID: 21, TeacherID: 9, ClassroomID: 9, Semester: "2025-2026 第二学期", DayOfWeek: 4, StartPeriod: 6, Span: 1, Weeks: "1-16"},
		// Saturday
		{CourseID: 12, TeacherID: 8, ClassroomID: 2, Semester: "2025-2026 第二学期", DayOfWeek: 5, StartPeriod: 2, Span: 2, Weeks: "1-16"},
		{CourseID: 17, TeacherID: 10, ClassroomID: 10, Semester: "2025-2026 第二学期", DayOfWeek: 5, StartPeriod: 4, Span: 2, Weeks: "1-16"},
		// Sunday
		{CourseID: 7, TeacherID: 3, ClassroomID: 5, Semester: "2025-2026 第二学期", DayOfWeek: 6, StartPeriod: 8, Span: 2, Weeks: "1-16"},
	}
	DB.Create(&entries)
}
