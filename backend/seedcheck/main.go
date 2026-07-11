package main

import (
	"fmt"
	"os"
	"path/filepath"

	"scheduling-system/backend/database"
	"scheduling-system/backend/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func main() {
	dir, _ := os.MkdirTemp("", "seedcheck")
	defer os.RemoveAll(dir)

	if _, err := database.InitDB(dir); err != nil {
		fmt.Println("InitDB err:", err)
		os.Exit(1)
	}

	gdb, err := gorm.Open(sqlite.Open(filepath.Join(dir, "schedule.db")), &gorm.Config{})
	if err != nil {
		fmt.Println("open err:", err)
		os.Exit(1)
	}

	var teachers []models.Teacher
	gdb.Find(&teachers)
	depts := map[string]bool{}
	for _, t := range teachers {
		depts[t.Dept] = true
	}
	var courses, groups, tasks int64
	gdb.Model(&models.Course{}).Count(&courses)
	gdb.Model(&models.ClassGroup{}).Count(&groups)
	gdb.Model(&models.TeachingTask{}).Count(&tasks)

	fmt.Printf("teachers=%d  colleges=%d  courses=%d  groups=%d  teachingTasks=%d\n",
		len(teachers), len(depts), courses, groups, tasks)
	fmt.Println("学院列表:")
	for d := range depts {
		fmt.Println("  -", d)
	}
}
