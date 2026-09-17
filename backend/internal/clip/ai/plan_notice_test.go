package ai_test

import (
	"slices"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
)

func hasNotice(plan clip.EditPlan, code string) bool {
	return slices.ContainsFunc(clip.ActivePlanNotices(plan, composition.DefaultDesign()), func(n clip.PlanNotice) bool { return n.Reason == code })
}
