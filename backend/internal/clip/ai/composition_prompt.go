package ai

import (
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/clip/design"
)

func nativeComposition(in clip.PlanningInput) bool {
	return in.Composition != nil
}

func compositionLimits(cfg Config, in clip.PlanningInput) composition.Limits {
	return cfg.Template.Composition
}

// The parser resolves row authorship; the design owns every slot's fit. Each
// generated row is described by the slot it lands in (CLIP-147): its role, size
// and floor, the lines it may take and the syllables that fit it at the floor
// (CDS-86), narrowed by the row's own declared maximum (CLIP-116).
func generatedRegionSlots(presets composition.DesignSelection, ratio, body string, limits composition.Limits) []map[string]any {
	out := []map[string]any{}
	doc, problem := composition.ReadStored(body, limits)
	if problem != nil {
		return out
	}
	taken := map[string]int{}
	for _, e := range doc.Elements {
		region, id := regionSelection(presets, e)
		if region == "" {
			continue
		}
		for i, row := range e.Rows {
			if composition.RowKind(e, row) != "ai" {
				continue
			}
			spec, width, ok := design.RegionSlotAt(region, id, ratio, taken[region]+i)
			if !ok {
				continue
			}
			budget := design.RegionSlotBudget(spec, width)
			if row.Chars > 0 && row.Chars < budget {
				budget = row.Chars
			}
			out = append(out, map[string]any{"element_id": e.ID, "region": region, "preset": id, "row_index": i, "role": spec.Role, "size": spec.Size, "floor": spec.Floor, "lines": spec.MaxLines(), "max_syllables": budget})
		}
		taken[region] += max(1, len(e.Rows))
	}
	return out
}
