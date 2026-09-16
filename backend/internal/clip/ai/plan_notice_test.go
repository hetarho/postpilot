package ai_test

import (
	"slices"

	"github.com/postpilot/backend/internal/clip"
)

func hasNotice(plan clip.EditPlan, code string) bool {
	return slices.ContainsFunc(clip.ActivePlanNotices(plan), func(n clip.PlanNotice) bool { return n.Reason == code })
}
