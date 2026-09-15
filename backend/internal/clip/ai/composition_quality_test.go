package ai_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/platform/config"
)

func TestCompositionQualityEvidenceCorpus(t *testing.T) {
	data, err := os.ReadFile("testdata/composition-quality.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct{ Name, Mutation, FirstItem, FirstText, FirstFallback string }
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			in, response := nativeInput(), nativePlan()
			if in.Analyses[0].Source.Info.HasAudio {
				t.Fatal("fixture is not silent")
			}
			for _, observed := range in.Analyses[0].Segments {
				if observed.Speech != "" {
					t.Fatal("invented speech")
				}
			}
			switch tc.Mutation {
			case "none":
			case "lookalike":
				in.Analyses[0].Segments[0].Subjects = []string{"접시", "라면"}
				in.Analyses[0].Segments[0].Event = "비슷한 면 요리를 담는다"
			case "missing_price":
				delete(in.Composition.Inputs.Items["menu"][0].Values, "price")
			case "experience":
				nativeGenerated(response, 0)["text"] = "먹어보니 맛있어요"
			case "global_price":
				nativeGenerated(response, 0)["fact_refs"] = []any{nativeFact("fee", "", "")}
			default:
				t.Fatal("unhandled corpus mutation", tc.Mutation)
			}
			writer, models, _ := newService(t, raw(response), true)
			plan, _, err := writer.Plan(t.Context(), testRef(), in)
			if err != nil {
				t.Fatal(err)
			}
			if len(models.calls) != 1 {
				t.Fatal("fixture used an extra repair call")
			}
			if plan.Portable.Cuts[0].ItemID != tc.FirstItem {
				t.Fatal("wrong subject association", plan.Portable.Cuts[0])
			}
			first := findNativeCopy(t, plan, "cut-sea")
			if tc.FirstText == "" {
				if first != nil || !hasFallback(plan, "cut-sea", tc.FirstFallback) {
					t.Fatal("unsupported copy survived", first, plan.Portable.Fallbacks)
				}
			} else if first == nil || first.Resolved.Text != tc.FirstText {
				t.Fatal("grounded copy changed", first)
			}
			second := findNativeCopy(t, plan, "cut-cheese")
			if second == nil || second.Resolved.Text != "치즈라면 $12 per serving" {
				t.Fatal("other dish changed", second)
			}
			for _, text := range plan.Portable.Elements {
				if text.Resolved.Element.ID == "optional" {
					t.Fatal("blank optional fact produced a placeholder")
				}
				for _, fact := range text.Resolved.Facts {
					if fact.GroupID != "menu" || fact.ItemID != text.Resolved.ItemID {
						t.Fatal("fact escaped its visible item", text)
					}
					matched := false
					for _, item := range in.Composition.Inputs.Items["menu"] {
						if item.ID == fact.ItemID && item.Values[fact.FieldID] == fact.Value && fact.Value != "" {
							matched = true
						}
					}
					if !matched {
						t.Fatal("fact value differs from its own authored item", fact)
					}
				}
				if text.Resolved.Element.Kind == "ai" {
					if len(text.Evidence) != 1 {
						t.Fatal("missing scene evidence", text)
					}
					index := 0
					if text.Resolved.ItemID == "cheese" {
						index = 1
					}
					e := text.Evidence[0]
					if e.SourceID != "source" || e.Fingerprint != in.Analyses[0].Source.Fingerprint || e.StartMS != index*7500 || e.EndMS != (index+1)*7500 {
						t.Fatal("caption points to the wrong scene", text)
					}
				}
			}
			if err := clip.ValidateCompositionEvidence(plan); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCompositionQualityManualReorderKeepsFrozenFactsAndExactText(t *testing.T) {
	in := nativeInput()
	writer, _, _ := newService(t, raw(nativePlan()), true)
	plan, _, err := writer.Plan(t.Context(), testRef(), in)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := clip.EncodeEditPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	analysis, _ := json.Marshal(in.Analyses)
	p := clip.Project{Ratio: plan.Ratio, EditPlan: encoded, Analysis: string(analysis), Composition: in.Composition, EditPlanRevision: 1}
	draft := clip.CorrectionFromPlan(plan)
	slices.Reverse(draft.Cuts)
	before := map[string]clip.CorrectionText{}
	for _, text := range draft.Elements {
		before[text.InstanceID] = text
	}
	corrected, err := clip.ApplyCorrection(config.ClipRender(&config.Config{}), p, draft)
	if err != nil {
		t.Fatal(err)
	}
	if err := clip.ValidateCompositionEvidence(corrected); err != nil {
		t.Fatal(err)
	}
	for _, text := range corrected.Portable.Elements {
		previous := before[text.Resolved.InstanceID]
		if text.Resolved.Text != previous.Text || text.Resolved.ItemID != previous.ItemID || !reflect.DeepEqual(text.Evidence, previous.Evidence) {
			t.Fatal("reordering changed caption identity or authored text", text)
		}
		if text.Resolved.Element.Kind == "ai" {
			want := 0
			if text.Resolved.CutID == "cut-sea" {
				want = 7500
			}
			if text.Resolved.StartMS < want || text.Resolved.EndMS > want+7500 {
				t.Fatal("caption did not follow its cut", text)
			}
		}
	}
	stored, err := clip.EncodeEditPlan(corrected)
	if err != nil {
		t.Fatal(err)
	}
	// Editing the reusable source after persistence cannot rewrite this project's snapshot.
	in.Template.CompositionBody = `<clip version="1"/>`
	decoded, err := clip.DecodeEditPlan(stored)
	if err != nil || decoded.Portable.Snapshot.Body != nativeBody || !strings.Contains(stored, "12,000원") {
		t.Fatal("frozen content changed", err)
	}
}

// The same recorded corpus is handed to the production media review and browser
// compositor through an optional artifact directory, never through live AI.
func TestCompositionQualityAssemblyCorpus(t *testing.T) {
	for _, ratio := range []string{"vertical", "horizontal", "square"} {
		for _, pace := range []string{"steady", "rapid"} {
			t.Run(ratio+"-"+pace, func(t *testing.T) {
				in := nativeInput()
				in.Ratio, in.TargetDurationMS = ratio, 18000
				in.Composition.Inputs.Values = nil
				body := `<clip version="1" intro="b" caption="bold" outro="e" pace="` + pace + `"><group id="menu"><field id="name" label="이름" required="true"/><field id="price" label="가격"/></group><repeat for="menu"><scene id="dish" scope="item"><text id="copy" kind="ai" role="caption" position="bottom" basis="cut">관찰한 <value field="menu.name"/></text><text id="price" kind="fixed" role="info" position="top" basis="cut"><value field="menu.price"/></text></scene></repeat><text id="exact" kind="fixed" role="badge" position="header" basis="output-start" start="1" end="2">직접 작성</text><text id="intro" kind="fixed" role="hook" basis="output-start"/><text id="outro" kind="fixed" role="ending" basis="output-end"/></clip>`
				setNativeBody(&in, body)
				in.Analyses = nil
				writer, models, _ := newService(t, "", true)
				var recorded []json.RawMessage
				for i, id := range []string{"a", "b"} {
					source := clip.AnalysisSource{RenderSource: clip.RenderSource{ID: id, Fingerprint: strings.Repeat(id, 64), Info: clip.MediaInfo{DurationMS: 16000, Width: 1280, Height: 720, HasAudio: true, FrameRateNumerator: 60, FrameRateDenominator: 1, CadenceVerified: true, DecodedFrames: 960, DecodedDurationMS: 16000}}, Filename: id + ".mp4"}
					response := observation()
					response["source_id"], response["chunk_index"] = id, 0
					name := []string{"해물라면", "치즈라면"}[i]
					segments := []any{}
					for j, end := range []int{15000, 15500, 16000} {
						segment := firstSegment(observation())
						segment["start_ms"], segment["end_ms"] = []int{0, 15000, 15500}[j], end
						segment["subjects"], segment["event"], segment["speech"] = []string{name}, name+"을 담는다", "합성 음성"
						if j == 1 {
							segment["certainty"] = "uncertain"
						}
						if j == 2 {
							segment["certainty"], segment["usability"], segment["quality"] = "unknown", "unusable", "fully obscured"
							segment["subjects"], segment["event"], segment["speech"], segment["action"], segment["motion"] = []string{}, "", "", "", ""
						}
						segments = append(segments, segment)
					}
					response["segments"] = segments
					models.response.Text = raw(response)
					recorded = append(recorded, json.RawMessage(models.response.Text))
					chunkInput := chunk()
					chunkInput.Source, chunkInput.Index, chunkInput.OffsetMS, chunkInput.DurationMS = source, 0, 0, 16000
					chunkInput.Video.DurationMS = 16000
					observed, _, err := writer.ObserveChunk(t.Context(), testRef(), chunkInput)
					if err != nil {
						d, _ := clip.DiagnosticFromError(err)
						t.Fatalf("%v: %+v", err, d)
					}
					in.Analyses = append(in.Analyses, clip.SourceAnalysis{Source: source, Segments: observed.Segments})
				}
				cuts, generated := []any{}, []any{}
				start := 0
				for i, rate := range clip.PlaybackRates() {
					if i == 3 {
						start = 0
					}
					source, item, name := "a", "sea", "해물라면"
					if i >= 3 {
						source, item, name = "b", "cheese", "치즈라면"
					}
					id, end := fmt.Sprintf("cut-%d", i), start+3*rate
					refs := []string{clip.ObservationID(source, 0)}
					cut := map[string]any{"id": id, "source_id": source, "template_section_id": "dish", "group_id": "menu", "item_id": item, "start_ms": start, "end_ms": end, "rate_permille": rate, "volume": 1, "focal": map[string]float64{"x": []float64{.25, .75}[i/3], "y": .5}, "observation_refs": refs}
					cuts = append(cuts, cut)
					generated = append(generated, map[string]any{"element_id": "copy", "cut_id": id, "text": name + []string{"을 담는다", "의 모습", "이 보인다"}[i%3], "short_text": "", "keyword": "", "rows": []string{}, "short_rows": []string{}, "observation_refs": refs, "fact_refs": []any{nativeFact("name", "menu", item)}})
					start = end
				}
				response := map[string]any{"ratio": ratio, "duration_ms": 18000, "cuts": cuts, "generated": generated}
				models.response.Text = raw(response)
				recorded = append(recorded, json.RawMessage(models.response.Text))
				plan, _, err := writer.Plan(t.Context(), testRef(), in)
				if err != nil {
					d, _ := clip.DiagnosticFromError(err)
					t.Fatalf("%v: %+v", err, d)
				}
				if len(models.calls) != 3 || len(plan.Cuts) != 6 || plan.DurationMS != 18000 || len(plan.Portable.Fallbacks) != 0 {
					t.Fatalf("corpus silently changed: calls=%d cuts=%d duration=%d fallback=%+v", len(models.calls), len(plan.Cuts), plan.DurationMS, plan.Portable.Fallbacks)
				}
				plan.SourceAudio = &clip.SourceAudioSettings{Values: []clip.SourceAudioSetting{{SourceID: "a", Fingerprint: strings.Repeat("a", 64)}, {SourceID: "b", Fingerprint: strings.Repeat("b", 64)}}}
				encoded, err := clip.EncodeEditPlan(plan)
				if err != nil {
					d, _ := clip.DiagnosticFromError(err)
					t.Fatalf("%v: %+v", err, d)
				}
				analysis, _ := json.Marshal(in.Analyses)
				project := clip.Project{Ratio: ratio, EditPlan: encoded, Analysis: string(analysis), Composition: in.Composition, EditPlanRevision: 1}
				draft := clip.CorrectionFromPlan(plan)
				draft.Cuts[3].TransitionMS = 200
				draft.DurationMS = 17800
				// Reorder adjacent same-scene splits without changing ranges, ids or facts.
				draft.Cuts[0], draft.Cuts[1] = draft.Cuts[1], draft.Cuts[0]
				corrected, err := clip.ApplyCorrection(config.ClipRender(&config.Config{}), project, draft)
				if err != nil {
					d, _ := clip.DiagnosticFromError(err)
					t.Fatalf("%v: %+v", err, d)
				}
				if err = clip.ValidateCompositionEvidence(corrected); err != nil {
					d, _ := clip.DiagnosticFromError(err)
					t.Fatalf("%v: %+v", err, d)
				}
				if corrected.Cuts[0].ID != "cut-1" || corrected.Cuts[1].ID != "cut-0" || len(corrected.Portable.Elements) != 13 {
					t.Fatal("correction lost assembly or text")
				}
				encoded, err = clip.EncodeEditPlan(corrected)
				if err != nil {
					d, _ := clip.DiagnosticFromError(err)
					t.Fatalf("%v: %+v", err, d)
				}
				if root := os.Getenv("CLIP_ASSEMBLY_CORPUS_DIR"); root != "" {
					artifact := struct {
						Plan      string
						Analyses  []clip.SourceAnalysis
						Responses []json.RawMessage
					}{encoded, in.Analyses, recorded}
					data, _ := json.MarshalIndent(artifact, "", "  ")
					if err := os.MkdirAll(root, 0755); err != nil {
						d, _ := clip.DiagnosticFromError(err)
						t.Fatalf("%v: %+v", err, d)
					}
					if err := os.WriteFile(filepath.Join(root, ratio+"-"+pace+".json"), data, 0644); err != nil {
						d, _ := clip.DiagnosticFromError(err)
						t.Fatalf("%v: %+v", err, d)
					}
				}
			})
		}
	}
}
