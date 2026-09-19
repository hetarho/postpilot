package app

import "github.com/postpilot/backend/internal/clip"

func renderBatchSources(plan clip.EditPlan, batch clip.SourceBatch) clip.SourceBatch {
	needed := map[string]bool{}
	for _, c := range plan.Cuts {
		needed[c.Fingerprint] = true
	}
	out := batch
	out.Sources = nil
	for _, v := range batch.Sources {
		if needed[v.Fingerprint] {
			out.Sources = append(out.Sources, v)
		}
	}
	return out
}
