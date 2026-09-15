package ai_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

func hasNotice(plan clip.EditPlan, code string) bool {
	return slices.ContainsFunc(clip.ActivePlanNotices(plan), func(n clip.PlanNotice) bool { return n.Reason == code })
}

func TestDeliveredNativePlanNoticesRoundTripAndNoRetries(t *testing.T) {
	for _, code := range []string{"composition_cut_evidence", "plan_focal", "plan_volume", "plan_cut_rate", "composition_generated_identity", "composition_generated_bounds", "composition_generated_rows"} {
		t.Run(code, func(t *testing.T) {
			in, wire := nativeInput(), nativePlan()
			cut, g := firstCut(wire), nativeGenerated(wire, 0)
			switch code {
			case "composition_cut_evidence":
				cut["observation_refs"] = []string{"invented"}
			case "plan_focal":
				cut["focal"] = map[string]any{"x": 2, "y": -1}
			case "plan_volume":
				cut["volume"] = 3
			case "plan_cut_rate":
				cut["rate_permille"] = 1234
			case "composition_generated_identity":
				g["element_id"] = "absent"
			case "composition_generated_bounds":
				g["keyword"] = string(make([]byte, 500))
			case "composition_generated_rows":
				g["rows"] = []string{"unexpected row"}
			}
			in.Policy.ResponseRetries = 3
			s, models, _ := newService(t, raw(wire), true)
			plan, _, err := s.Plan(t.Context(), testRef(), in)
			if err != nil || len(models.calls) != 1 || !hasNotice(plan, code) {
				t.Fatalf("notice/retry contract: %v %+v", err, plan.Notices)
			}
			encoded, err := clip.EncodeEditPlan(plan)
			if err != nil {
				t.Fatal(err)
			}
			restored, err := clip.DecodeEditPlan(encoded)
			if err != nil || !reflect.DeepEqual(clip.ActivePlanNotices(plan), clip.ActivePlanNotices(restored)) {
				t.Fatal("notices lost in assembly round trip", err)
			}
			if plan.Portable.Snapshot.Body != in.Composition.Snapshot.Body {
				t.Fatal("repaired owner template")
			}
			for _, c := range plan.Cuts {
				if _, ok := clip.ContainedScene(in.Analyses, c); !ok {
					t.Fatal("invented observation")
				}
			}
		})
	}
}

func TestBackwardSectionRemovesOnlyOffendingCut(t *testing.T) {
	in, wire := nativeInput(), nativePlan()
	body := `<clip version="1" intro="b" caption="bold" outro="e"><guide>Keep selected lengths.</guide><scene id="arrival" scope="context"/><scene id="closing" scope="context"/><text id="empty-hook" kind="fixed" role="hook" basis="output-start"/><text id="empty-ending" kind="fixed" role="ending" basis="output-end"/></clip>`
	setNativeBody(&in, body)
	in.Composition.Inputs = clip.CompositionInputs{}
	in.Analyses[0].Source.Info.DurationMS = 30000
	for i := range in.Analyses[0].Segments {
		in.Analyses[0].Segments[i].StartMS = i * 15000
		in.Analyses[0].Segments[i].EndMS = (i + 1) * 15000
	}
	original := wire["cuts"].([]any)
	cuts := []any{}
	for i, section := range []string{"arrival", "arrival", "closing", "arrival"} {
		c := map[string]any{}
		for k, v := range original[0].(map[string]any) {
			c[k] = v
		}
		c["id"] = []string{"first", "second", "third", "backward"}[i]
		c["template_section_id"], c["start_ms"], c["end_ms"] = section, i*7500, (i+1)*7500
		cuts = append(cuts, c)
	}
	wire["cuts"], wire["generated"] = cuts, []any{}
	in.TargetDurationMS = 22500
	s, models, _ := newService(t, raw(wire), true)
	plan, _, err := s.Plan(t.Context(), testRef(), in)
	if err != nil || len(models.calls) != 1 || len(plan.Cuts) != 3 || plan.Cuts[0].ID != "first" || plan.Cuts[1].ID != "second" || plan.Cuts[2].ID != "third" || !hasNotice(plan, "composition_section_order") {
		t.Fatalf("bad ordered remainder: %v %+v", err, plan)
	}
}
