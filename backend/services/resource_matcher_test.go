package services

import (
	"testing"

	"scheduling-system/backend/models"
)

// ============ Match() 核心匹配 ============

// --- 1.1 教室类型匹配 ---

func TestMatch_PE_ToGymnasium(t *testing.T) {
	task := models.TeachingTask{}
	course := models.Course{Category: models.CategoryPE}
	room := models.Classroom{RoomType: models.RoomTypeGym}
	r := Match(task, course, room)
	if !r.OK {
		t.Errorf("expected OK, got %+v", r)
	}
}

func TestMatch_Lab_ToLab(t *testing.T) {
	task := models.TeachingTask{}
	course := models.Course{Category: models.CategoryLab}
	room := models.Classroom{RoomType: models.RoomTypeLab}
	r := Match(task, course, room)
	if !r.OK {
		t.Errorf("expected OK, got %+v", r)
	}
}

func TestMatch_Computer_ToComputer(t *testing.T) {
	task := models.TeachingTask{}
	course := models.Course{Category: models.CategoryComputer}
	room := models.Classroom{RoomType: models.RoomTypeComputer}
	r := Match(task, course, room)
	if !r.OK {
		t.Errorf("expected OK, got %+v", r)
	}
}

func TestMatch_Theory_ToStandard(t *testing.T) {
	task := models.TeachingTask{}
	course := models.Course{Category: models.CategoryTheory}
	room := models.Classroom{RoomType: models.RoomTypeNormal}
	r := Match(task, course, room)
	if !r.OK {
		t.Errorf("expected OK, got %+v", r)
	}
}

func TestMatch_Seminar_ToMultimedia(t *testing.T) {
	task := models.TeachingTask{}
	course := models.Course{Category: models.CategorySeminar}
	room := models.Classroom{RoomType: models.RoomTypeMultimedia}
	r := Match(task, course, room)
	if !r.OK {
		t.Errorf("expected OK, got %+v", r)
	}
}

func TestMatch_Lab_ToStandard_Fail(t *testing.T) {
	task := models.TeachingTask{}
	course := models.Course{Category: models.CategoryLab}
	room := models.Classroom{RoomType: models.RoomTypeNormal}
	r := Match(task, course, room)
	if r.OK {
		t.Error("expected mismatch")
	}
	if r.Code != CodeRoomTypeMismatch {
		t.Errorf("expected CodeRoomTypeMismatch, got %d", r.Code)
	}
	if r.RequiredType != models.RoomTypeLab {
		t.Errorf("expected RequiredType=%s, got %s", models.RoomTypeLab, r.RequiredType)
	}
}

func TestMatch_PE_ToStandard_Fail(t *testing.T) {
	task := models.TeachingTask{}
	course := models.Course{Category: models.CategoryPE}
	room := models.Classroom{RoomType: models.RoomTypeNormal}
	r := Match(task, course, room)
	if r.OK {
		t.Error("expected mismatch")
	}
	if r.Code != CodeRoomTypeMismatch {
		t.Errorf("expected CodeRoomTypeMismatch, got %d", r.Code)
	}
}

func TestMatch_Computer_ToLab_Fail(t *testing.T) {
	task := models.TeachingTask{}
	course := models.Course{Category: models.CategoryComputer}
	room := models.Classroom{RoomType: models.RoomTypeLab}
	r := Match(task, course, room)
	if r.OK {
		t.Error("expected mismatch")
	}
	if r.Code != CodeRoomTypeMismatch {
		t.Errorf("expected CodeRoomTypeMismatch, got %d", r.Code)
	}
}

// --- 1.2 排他教室检查 ---

func TestMatch_Theory_ToLab_ExclusionFail(t *testing.T) {
	task := models.TeachingTask{}
	course := models.Course{Category: models.CategoryTheory}
	room := models.Classroom{RoomType: models.RoomTypeLab}
	r := Match(task, course, room)
	if r.OK {
		t.Error("expected mismatch")
	}
	if r.Code != CodeSpecialtyExclusion {
		t.Errorf("expected CodeSpecialtyExclusion, got %d", r.Code)
	}
}

func TestMatch_Theory_ToComputer_ExclusionFail(t *testing.T) {
	task := models.TeachingTask{}
	course := models.Course{Category: models.CategoryTheory}
	room := models.Classroom{RoomType: models.RoomTypeComputer}
	r := Match(task, course, room)
	if r.OK {
		t.Error("expected mismatch")
	}
	if r.Code != CodeSpecialtyExclusion {
		t.Errorf("expected CodeSpecialtyExclusion, got %d", r.Code)
	}
}

func TestMatch_Theory_ToGym_ExclusionFail(t *testing.T) {
	task := models.TeachingTask{}
	course := models.Course{Category: models.CategoryTheory}
	room := models.Classroom{RoomType: models.RoomTypeGym}
	r := Match(task, course, room)
	if r.OK {
		t.Error("expected mismatch")
	}
	if r.Code != CodeSpecialtyExclusion {
		t.Errorf("expected CodeSpecialtyExclusion, got %d", r.Code)
	}
}

func TestMatch_Theory_ToStandard_OK(t *testing.T) {
	task := models.TeachingTask{}
	course := models.Course{Category: models.CategoryTheory}
	room := models.Classroom{RoomType: models.RoomTypeNormal}
	r := Match(task, course, room)
	if !r.OK {
		t.Errorf("expected OK, got %+v", r)
	}
}

func TestMatch_Theory_ToMultimedia_OK(t *testing.T) {
	task := models.TeachingTask{}
	course := models.Course{Category: models.CategoryTheory}
	room := models.Classroom{RoomType: models.RoomTypeMultimedia}
	r := Match(task, course, room)
	if !r.OK {
		t.Errorf("expected OK, got %+v", r)
	}
}

func TestMatch_Theory_ToLectureHall_OK(t *testing.T) {
	task := models.TeachingTask{}
	course := models.Course{Category: models.CategoryTheory}
	room := models.Classroom{RoomType: models.RoomTypeLecture}
	r := Match(task, course, room)
	if !r.OK {
		t.Errorf("expected OK, got %+v", r)
	}
}

// --- 1.3 设备匹配 ---

func TestMatch_Equipment_Has_OK(t *testing.T) {
	task := models.TeachingTask{RequiredEquipment: `["projector"]`}
	course := models.Course{Category: models.CategoryTheory}
	room := models.Classroom{RoomType: models.RoomTypeMultimedia, Equipment: `["projector","smartboard"]`}
	r := Match(task, course, room)
	if !r.OK {
		t.Errorf("expected OK, got %+v", r)
	}
}

func TestMatch_Equipment_NoEquip_Fail(t *testing.T) {
	task := models.TeachingTask{RequiredEquipment: `["projector"]`}
	course := models.Course{Category: models.CategoryTheory}
	room := models.Classroom{RoomType: models.RoomTypeNormal, Equipment: ""}
	r := Match(task, course, room)
	if r.OK {
		t.Error("expected mismatch")
	}
	if r.Code != CodeEquipmentMissing {
		t.Errorf("expected CodeEquipmentMissing, got %d", r.Code)
	}
	if len(r.MissingEquipment) != 1 || r.MissingEquipment[0] != "projector" {
		t.Errorf("expected missing [projector], got %v", r.MissingEquipment)
	}
}

func TestMatch_Equipment_WrongEquip_Fail(t *testing.T) {
	task := models.TeachingTask{RequiredEquipment: `["projector"]`}
	course := models.Course{Category: models.CategoryTheory}
	room := models.Classroom{RoomType: models.RoomTypeNormal, Equipment: `["smartboard"]`}
	r := Match(task, course, room)
	if r.OK {
		t.Error("expected mismatch")
	}
	if r.Code != CodeEquipmentMissing {
		t.Errorf("expected CodeEquipmentMissing, got %d", r.Code)
	}
}

func TestMatch_Equipment_NoRequirement_OK(t *testing.T) {
	task := models.TeachingTask{}
	course := models.Course{Category: models.CategoryTheory}
	room := models.Classroom{RoomType: models.RoomTypeNormal, Equipment: `["projector"]`}
	r := Match(task, course, room)
	if !r.OK {
		t.Errorf("expected OK, got %+v", r)
	}
}

func TestMatch_Equipment_PartialMissing_Fail(t *testing.T) {
	task := models.TeachingTask{RequiredEquipment: `["projector","camera"]`}
	course := models.Course{Category: models.CategoryTheory}
	room := models.Classroom{RoomType: models.RoomTypeNormal, Equipment: `["projector"]`}
	r := Match(task, course, room)
	if r.OK {
		t.Error("expected mismatch")
	}
	if len(r.MissingEquipment) != 1 || r.MissingEquipment[0] != "camera" {
		t.Errorf("expected missing [camera], got %v", r.MissingEquipment)
	}
}

func TestMatch_Equipment_AllPresent_OK(t *testing.T) {
	task := models.TeachingTask{RequiredEquipment: `["projector","camera"]`}
	course := models.Course{Category: models.CategoryTheory}
	room := models.Classroom{RoomType: models.RoomTypeNormal, Equipment: `["projector","camera","aircon"]`}
	r := Match(task, course, room)
	if !r.OK {
		t.Errorf("expected OK, got %+v", r)
	}
}

func TestMatch_Equipment_MalformedJSON_OK(t *testing.T) {
	task := models.TeachingTask{RequiredEquipment: "broken json"}
	course := models.Course{Category: models.CategoryTheory}
	room := models.Classroom{RoomType: models.RoomTypeNormal, Equipment: ""}
	r := Match(task, course, room)
	if !r.OK {
		t.Errorf("expected OK (malformed=empty=no requirement), got %+v", r)
	}
}

func TestMatch_Equipment_EmptyString_OK(t *testing.T) {
	task := models.TeachingTask{RequiredEquipment: ""}
	course := models.Course{Category: models.CategoryTheory}
	room := models.Classroom{RoomType: models.RoomTypeNormal, Equipment: ""}
	r := Match(task, course, room)
	if !r.OK {
		t.Errorf("expected OK, got %+v", r)
	}
}

// --- 1.4 优先级测试 ---

func TestMatch_Priority_ExplicitOverCategory(t *testing.T) {
	task := models.TeachingTask{RequiredRoomType: models.RoomTypeLecture}
	course := models.Course{Category: models.CategoryLab}
	room := models.Classroom{RoomType: models.RoomTypeLecture}
	r := Match(task, course, room)
	if !r.OK {
		t.Errorf("expected OK (explicit overrides category), got %+v", r)
	}
}

func TestMatch_Priority_CategoryOverName(t *testing.T) {
	task := models.TeachingTask{}
	course := models.Course{Category: models.CategoryLab, Name: "体育"}
	room := models.Classroom{RoomType: models.RoomTypeLab}
	r := Match(task, course, room)
	if !r.OK {
		t.Errorf("expected OK (category overrides name), got %+v", r)
	}
	if r.RequiredType != "" && r.Code == CodeRoomTypeMismatch {
		t.Errorf("category should override name inference")
	}
}

func TestMatch_Priority_NameFallback_Lab(t *testing.T) {
	task := models.TeachingTask{}
	course := models.Course{Name: "电路实验"}
	room := models.Classroom{RoomType: models.RoomTypeLab}
	r := Match(task, course, room)
	if !r.OK {
		t.Errorf("expected OK (name fallback), got %+v", r)
	}
}

func TestMatch_Priority_NameFallback_PE(t *testing.T) {
	task := models.TeachingTask{}
	course := models.Course{Name: "体育"}
	room := models.Classroom{RoomType: models.RoomTypeGym}
	r := Match(task, course, room)
	if !r.OK {
		t.Errorf("expected OK (name fallback), got %+v", r)
	}
}

func TestMatch_Priority_NameFallback_Computer(t *testing.T) {
	task := models.TeachingTask{}
	course := models.Course{Name: "上机实训"}
	room := models.Classroom{RoomType: models.RoomTypeComputer}
	r := Match(task, course, room)
	if !r.OK {
		t.Errorf("expected OK (name fallback), got %+v", r)
	}
}

func TestMatch_Priority_NoRequirement(t *testing.T) {
	task := models.TeachingTask{}
	course := models.Course{Name: "高等数学"}
	room := models.Classroom{RoomType: models.RoomTypeNormal}
	r := Match(task, course, room)
	if !r.OK {
		t.Errorf("expected OK, got %+v", r)
	}
}

// --- 1.5 组合场景 ---

func TestMatch_TypeAndEquipment_OK(t *testing.T) {
	task := models.TeachingTask{RequiredEquipment: `["microscope"]`}
	course := models.Course{Category: models.CategoryLab}
	room := models.Classroom{RoomType: models.RoomTypeLab, Equipment: `["microscope"]`}
	r := Match(task, course, room)
	if !r.OK {
		t.Errorf("expected OK, got %+v", r)
	}
}

func TestMatch_TypeOK_EquipmentFail(t *testing.T) {
	task := models.TeachingTask{RequiredEquipment: `["microscope"]`}
	course := models.Course{Category: models.CategoryLab}
	room := models.Classroom{RoomType: models.RoomTypeLab, Equipment: ""}
	r := Match(task, course, room)
	if r.OK {
		t.Error("expected equipment mismatch")
	}
	if r.Code != CodeEquipmentMissing {
		t.Errorf("expected CodeEquipmentMissing, got %d", r.Code)
	}
}

func TestMatch_TypeFail_EquipmentNotChecked(t *testing.T) {
	task := models.TeachingTask{RequiredEquipment: `["microscope"]`}
	course := models.Course{Category: models.CategoryLab}
	room := models.Classroom{RoomType: models.RoomTypeNormal, Equipment: ""}
	r := Match(task, course, room)
	if r.OK {
		t.Error("expected type mismatch")
	}
	if r.Code != CodeRoomTypeMismatch {
		t.Errorf("expected CodeRoomTypeMismatch, got %d", r.Code)
	}
}

func TestMatch_Exclusion_NoEquipReq(t *testing.T) {
	task := models.TeachingTask{}
	course := models.Course{Category: models.CategoryTheory}
	room := models.Classroom{RoomType: models.RoomTypeLab}
	r := Match(task, course, room)
	if r.OK {
		t.Error("expected exclusion")
	}
	if r.Code != CodeSpecialtyExclusion {
		t.Errorf("expected CodeSpecialtyExclusion, got %d", r.Code)
	}
}
