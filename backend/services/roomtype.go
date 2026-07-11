package services

import "strings"

// IsLabCourse returns true if the course name indicates a lab course (contains "实验").
// Case-insensitive matching so "大学物理实验" and "大学物理實驗" both match.
func IsLabCourse(courseName string) bool {
	return strings.Contains(strings.ToLower(courseName), "实验")
}

// IsComputerCourse returns true if the course name indicates a computer lab course (contains "上机").
// Case-insensitive matching.
func IsComputerCourse(courseName string) bool {
	return strings.Contains(strings.ToLower(courseName), "上机")
}
