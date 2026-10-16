package ai

import "github.com/postpilot/backend/internal/clip"

func addLinkedPost(payload map[string]any, in clip.PlanningInput) {
	if in.LinkedPost != nil {
		payload["linked_post"] = in.LinkedPost
	}
}
