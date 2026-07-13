package services

import (
	"testing"

	"scheduling-system/backend/models"
)

// ============ InferRoomType() ============

func TestInferRoomType_Explicit(t *testing.T) {
	task := models.TeachingTask{RequiredRoomType: models.RoomTypeLecture}
	course := models.Course{Category: models.CategoryLab}
	rt := InferRoomType(task, course)
	if rt != models.RoomTypeLecture {
		t.Errorf("expected %s, got %s", models.RoomTypeLecture, rt)
	}
}

func TestInferRoomType_Category_PE(t *testing.T) {
	task := models.TeachingTask{}
	course := models.Course{Category: models.CategoryPE}
	rt := InferRoomType(task, course)
	if rt != models.RoomTypeGym {
		t.Errorf("expected %s, got %s", models.RoomTypeGym, rt)
	}
}

func TestInferRoomType_Category_Lab(t *testing.T) {
	task := models.TeachingTask{}
	course := models.Course{Category: models.CategoryLab}
	rt := InferRoomType(task, course)
	if rt != models.RoomTypeLab {
		t.Errorf("expected %s, got %s", models.RoomTypeLab, rt)
	}
}

func TestInferRoomType_Category_Computer(t *testing.T) {
	task := models.TeachingTask{}
	course := models.Course{Category: models.CategoryComputer}
	rt := InferRoomType(task, course)
	if rt != models.RoomTypeComputer {
		t.Errorf("expected %s, got %s", models.RoomTypeComputer, rt)
	}
}

func TestInferRoomType_Category_Theory_Empty(t *testing.T) {
	task := models.TeachingTask{}
	course := models.Course{Category: models.CategoryTheory}
	rt := InferRoomType(task, course)
	if rt != "" {
		t.Errorf("expected empty for theory, got %s", rt)
	}
}

func TestInferRoomType_Category_Seminar_Empty(t *testing.T) {
	task := models.TeachingTask{}
	course := models.Course{Category: models.CategorySeminar}
	rt := InferRoomType(task, course)
	if rt != "" {
		t.Errorf("expected empty for seminar, got %s", rt)
	}
}

func TestInferRoomType_NameFallback_PE(t *testing.T) {
	task := models.TeachingTask{}
	course := models.Course{Name: "体育"}
	rt := InferRoomType(task, course)
	if rt != models.RoomTypeGym {
		t.Errorf("expected %s, got %s", models.RoomTypeGym, rt)
	}
}

func TestInferRoomType_NameFallback_Lab(t *testing.T) {
	task := models.TeachingTask{}
	course := models.Course{Name: "计算机网络实验"}
	rt := InferRoomType(task, course)
	if rt != models.RoomTypeLab {
		t.Errorf("expected %s, got %s", models.RoomTypeLab, rt)
	}
}

func TestInferRoomType_NameFallback_Computer(t *testing.T) {
	task := models.TeachingTask{}
	course := models.Course{Name: "上机实训"}
	rt := InferRoomType(task, course)
	if rt != models.RoomTypeComputer {
		t.Errorf("expected %s, got %s", models.RoomTypeComputer, rt)
	}
}

func TestInferRoomType_NameFallback_NoMatch(t *testing.T) {
	task := models.TeachingTask{}
	course := models.Course{Name: "高等数学"}
	rt := InferRoomType(task, course)
	if rt != "" {
		t.Errorf("expected empty, got %s", rt)
	}
}

func TestInferRoomType_EmptyName(t *testing.T) {
	task := models.TeachingTask{}
	course := models.Course{Name: ""}
	rt := InferRoomType(task, course)
	if rt != "" {
		t.Errorf("expected empty, got %s", rt)
	}
}

func TestInferRoomType_Explicit_Overrides_Category(t *testing.T) {
	task := models.TeachingTask{RequiredRoomType: models.RoomTypeLecture}
	course := models.Course{Category: models.CategoryLab}
	rt := InferRoomType(task, course)
	if rt != models.RoomTypeLecture {
		t.Errorf("explicit should override category, expected %s, got %s", models.RoomTypeLecture, rt)
	}
}

// ============ IsSharedVenue() ============

func TestIsSharedVenue_Gymnasium(t *testing.T) {
	room := models.Classroom{RoomType: models.RoomTypeGym}
	if !IsSharedVenue(room) {
		t.Error("expected gymnasium to be shared")
	}
}

func TestIsSharedVenue_Standard(t *testing.T) {
	room := models.Classroom{RoomType: models.RoomTypeNormal}
	if IsSharedVenue(room) {
		t.Error("expected standard to NOT be shared")
	}
}

func TestIsSharedVenue_Lab(t *testing.T) {
	room := models.Classroom{RoomType: models.RoomTypeLab}
	if IsSharedVenue(room) {
		t.Error("expected lab to NOT be shared")
	}
}

func TestIsSharedVenue_Multimedia(t *testing.T) {
	room := models.Classroom{RoomType: models.RoomTypeMultimedia}
	if IsSharedVenue(room) {
		t.Error("expected multimedia to NOT be shared")
	}
}

func TestIsSharedVenue_Empty(t *testing.T) {
	room := models.Classroom{RoomType: ""}
	if IsSharedVenue(room) {
		t.Error("expected empty type to NOT be shared")
	}
}

// ============ AllowedRooms() ============

func TestAllowedRooms_Lab_OnlyLabMatches(t *testing.T) {
	task := models.TeachingTask{}
	course := models.Course{Category: models.CategoryLab}
	rooms := []models.Classroom{
		{RoomType: models.RoomTypeLab},
		{RoomType: models.RoomTypeNormal},
		{RoomType: models.RoomTypeGym},
		{RoomType: models.RoomTypeComputer},
	}
	out := AllowedRooms(task, course, rooms)
	if len(out) != 1 {
		t.Errorf("expected 1, got %d", len(out))
	}
	if out[0].RoomType != models.RoomTypeLab {
		t.Errorf("expected lab, got %s", out[0].RoomType)
	}
}

func TestAllowedRooms_Theory_ExcludesSpecialty(t *testing.T) {
	task := models.TeachingTask{}
	course := models.Course{Category: models.CategoryTheory}
	rooms := []models.Classroom{
		{RoomType: models.RoomTypeNormal},
		{RoomType: models.RoomTypeMultimedia},
		{RoomType: models.RoomTypeLecture},
		{RoomType: models.RoomTypeLab},
		{RoomType: models.RoomTypeGym},
		{RoomType: models.RoomTypeComputer},
	}
	out := AllowedRooms(task, course, rooms)
	if len(out) != 3 {
		t.Errorf("expected 3 (standard+multimedia+lecture), got %d", len(out))
	}
}

func TestAllowedRooms_NoRequirement_ExcludesSpecialty(t *testing.T) {
	task := models.TeachingTask{}
	course := models.Course{Name: "高等数学"}
	rooms := []models.Classroom{
		{RoomType: models.RoomTypeNormal},
		{RoomType: models.RoomTypeMultimedia},
		{RoomType: models.RoomTypeLab},
		{RoomType: models.RoomTypeGym},
		{RoomType: models.RoomTypeComputer},
	}
	out := AllowedRooms(task, course, rooms)
	if len(out) != 2 {
		t.Errorf("expected 2 (standard+multimedia), got %d", len(out))
	}
}

func TestAllowedRooms_Explicit_LectureHall(t *testing.T) {
	task := models.TeachingTask{RequiredRoomType: models.RoomTypeLecture}
	course := models.Course{Category: models.CategoryLab}
	rooms := []models.Classroom{
		{RoomType: models.RoomTypeNormal},
		{RoomType: models.RoomTypeLecture},
		{RoomType: models.RoomTypeLab},
	}
	out := AllowedRooms(task, course, rooms)
	if len(out) != 1 {
		t.Errorf("expected 1, got %d", len(out))
	}
	if out[0].RoomType != models.RoomTypeLecture {
		t.Errorf("expected lecture hall, got %s", out[0].RoomType)
	}
}

func TestAllowedRooms_EmptyRooms(t *testing.T) {
	task := models.TeachingTask{}
	course := models.Course{Category: models.CategoryTheory}
	out := AllowedRooms(task, course, []models.Classroom{})
	if len(out) != 0 {
		t.Errorf("expected 0, got %d", len(out))
	}
}

// ============ ExplainRequirement() ============

func TestExplainRequirement_Explicit(t *testing.T) {
	task := models.TeachingTask{RequiredRoomType: models.RoomTypeLab}
	course := models.Course{}
	req := ExplainRequirement(task, course)
	if req.RoomType != models.RoomTypeLab {
		t.Errorf("expected %s, got %s", models.RoomTypeLab, req.RoomType)
	}
	if req.RoomTypeSource != "explicit" {
		t.Errorf("expected explicit, got %s", req.RoomTypeSource)
	}
}

func TestExplainRequirement_Category(t *testing.T) {
	task := models.TeachingTask{}
	course := models.Course{Category: models.CategoryPE}
	req := ExplainRequirement(task, course)
	if req.RoomType != models.RoomTypeGym {
		t.Errorf("expected %s, got %s", models.RoomTypeGym, req.RoomType)
	}
	if req.RoomTypeSource != "category" {
		t.Errorf("expected category, got %s", req.RoomTypeSource)
	}
}

func TestExplainRequirement_NameFallback(t *testing.T) {
	task := models.TeachingTask{}
	course := models.Course{Name: "体育"}
	req := ExplainRequirement(task, course)
	if req.RoomType != models.RoomTypeGym {
		t.Errorf("expected %s, got %s", models.RoomTypeGym, req.RoomType)
	}
	if req.RoomTypeSource != "name_fallback" {
		t.Errorf("expected name_fallback, got %s", req.RoomTypeSource)
	}
}

func TestExplainRequirement_None(t *testing.T) {
	task := models.TeachingTask{}
	course := models.Course{Name: "高等数学"}
	req := ExplainRequirement(task, course)
	if req.RoomType != "" {
		t.Errorf("expected empty, got %s", req.RoomType)
	}
	if req.RoomTypeSource != "none" {
		t.Errorf("expected none, got %s", req.RoomTypeSource)
	}
}

func TestExplainRequirement_WithEquipment(t *testing.T) {
	task := models.TeachingTask{RequiredEquipment: `["projector"]`}
	course := models.Course{Category: models.CategoryTheory}
	req := ExplainRequirement(task, course)
	if len(req.Equipment) != 1 || req.Equipment[0] != "projector" {
		t.Errorf("expected [projector], got %v", req.Equipment)
	}
}

func TestExplainRequirement_ExplicitWithEquipment(t *testing.T) {
	task := models.TeachingTask{
		RequiredRoomType:  models.RoomTypeComputer,
		RequiredEquipment: `["computer"]`,
	}
	course := models.Course{}
	req := ExplainRequirement(task, course)
	if req.RoomType != models.RoomTypeComputer {
		t.Errorf("expected %s, got %s", models.RoomTypeComputer, req.RoomType)
	}
	if req.RoomTypeSource != "explicit" {
		t.Errorf("expected explicit, got %s", req.RoomTypeSource)
	}
	if len(req.Equipment) != 1 || req.Equipment[0] != "computer" {
		t.Errorf("expected [computer], got %v", req.Equipment)
	}
}

// ============ ExplainMismatch() ============

func TestExplainMismatch_OK_Empty(t *testing.T) {
	r := MatchResult{OK: true, Code: MatchOK}
	s := ExplainMismatch(r)
	if s != "" {
		t.Errorf("expected empty string, got %s", s)
	}
}

func TestExplainMismatch_RoomTypeMismatch(t *testing.T) {
	r := MatchResult{
		OK:           false,
		Code:         CodeRoomTypeMismatch,
		RequiredType: models.RoomTypeLab,
		ActualType:   models.RoomTypeNormal,
	}
	s := ExplainMismatch(r)
	if s == "" {
		t.Error("expected non-empty")
	}
	if !contains(s, models.RoomTypeLab) {
		t.Errorf("expected to contain %s, got %s", models.RoomTypeLab, s)
	}
	if !contains(s, models.RoomTypeNormal) {
		t.Errorf("expected to contain %s, got %s", models.RoomTypeNormal, s)
	}
}

func TestExplainMismatch_EquipmentMissing(t *testing.T) {
	r := MatchResult{
		OK:               false,
		Code:             CodeEquipmentMissing,
		MissingEquipment: []string{"projector"},
	}
	s := ExplainMismatch(r)
	if !contains(s, "projector") {
		t.Errorf("expected to contain projector, got %s", s)
	}
}

func TestExplainMismatch_SpecialtyExclusion(t *testing.T) {
	r := MatchResult{
		OK:         false,
		Code:       CodeSpecialtyExclusion,
		ActualType: models.RoomTypeLab,
	}
	s := ExplainMismatch(r)
	if !contains(s, models.RoomTypeLab) {
		t.Errorf("expected to contain %s, got %s", models.RoomTypeLab, s)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		(len(s) > 0 && len(sub) > 0 && (indexOf(s, sub) >= 0)))
}

func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
