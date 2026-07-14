package matcher_test

import (
	"math/rand"
	"testing"

	"scheduling-system/backend/models"
	"scheduling-system/backend/scheduling/matcher"
	"scheduling-system/backend/services"

	"github.com/google/go-cmp/cmp"
)

func TestMatch_EquivalenceRandom(t *testing.T) {
	rng := rand.New(rand.NewSource(20260714))

	roomTypes := []string{"", "NORMAL", "LAB", "COMPUTER", "GYM", "MULTIMEDIA", "LECTURE"}
	categories := []string{"", "theory", "lab", "computer", "pe"}
	requiredTypes := []string{"", "LAB", "COMPUTER", "GYM", "MULTIMEDIA"}
	equipmentOptions := []string{"", `[]`, `["projector"]`, `["projector","audio"]`, `["microscope"]`}

	for i := 0; i < 100; i++ {
		task := models.TeachingTask{
			CourseID:          uint(100 + i),
			RequiredRoomType:  requiredTypes[rng.Intn(len(requiredTypes))],
			RequiredEquipment: equipmentOptions[rng.Intn(len(equipmentOptions))],
		}
		course := models.Course{
			Name:     []string{"高等数学", "大学体育", "物理实验", "程序设计", "艺术鉴赏"}[rng.Intn(5)],
			Category: categories[rng.Intn(len(categories))],
		}
		room := models.Classroom{
			Name:      "test room",
			RoomType:  roomTypes[rng.Intn(len(roomTypes))],
			Capacity:  30 + rng.Intn(200),
			Equipment: equipmentOptions[rng.Intn(len(equipmentOptions))],
		}

		newResult := matcher.Match(task, course, room)
		oldResult := services.Match(task, course, room)

		converted := matcher.MatchResult{
			OK:               oldResult.OK,
			Code:             matcher.ResourceMatchCode(oldResult.Code),
			Reason:           oldResult.Reason,
			RequiredType:     oldResult.RequiredType,
			ActualType:       oldResult.ActualType,
			MissingEquipment: oldResult.MissingEquipment,
		}

		if diff := cmp.Diff(converted, newResult); diff != "" {
			t.Errorf("fixture %d: matcher.Match != services.Match (-old +new):\n%s\ntask=%+v\ncourse=%+v\nroom=%+v",
				i, diff, task, course, room)
		}
	}
}