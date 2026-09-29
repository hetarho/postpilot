package media

import (
	"slices"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/clip/design"
)

// editedPlan is a clip as ② leaves it after region and caption edits: four cuts, the second of
// which no caption covers; one caption on the first cut in an owner style outside the AI set,
// at the owner's size and place; one crossing from the third cut into the fourth in the style
// the writer gave it; an intro with a deliberately blank middle slot and an outro, both drawn
// from the project's own slots.
func editedPlan(t *testing.T, ratio, pace string) (clip.EditPlan, clip.ProjectRegions) {
	t.Helper()
	cut := func(id string, start, end int) (clip.Cut, composition.Cut) {
		return clip.Cut{ID: id, SourceID: "source", Fingerprint: "fp", StartMS: start, EndMS: end, Focal: clip.Point{X: .5, Y: .5}},
			composition.Cut{ID: id, SourceID: "source", StartMS: start, EndMS: end, PlaybackRatePermille: 1000}
	}
	plan := clip.EditPlan{Ratio: ratio, DurationMS: 15000, CaptionPace: pace, IntroPreset: "cover", OutroPreset: "b",
		CaptionStyles: []string{design.DefaultCaptionStyle}, Portable: &clip.PortablePlan{
			Snapshot: clip.CompositionSnapshot{Version: 1, Body: `<clip version="1"/>`},
			Observations: []clip.SourceAnalysis{{
				Source:   clip.AnalysisSource{RenderSource: clip.RenderSource{ID: "source", Fingerprint: "fp", Info: clip.MediaInfo{DurationMS: 15000, Width: 1080, Height: 1920}}},
				Segments: []clip.Segment{{StartMS: 0, EndMS: 15000, Event: "먹는다", Quality: "clear", Scene: "food", Certainty: clip.CertaintyCertain, Usability: clip.UsabilityUsable}},
			}},
		}}
	for _, c := range [][3]any{{"one", 0, 4000}, {"two", 4000, 7500}, {"three", 7500, 11000}, {"four", 11000, 15000}} {
		edit, portable := cut(c[0].(string), c[1].(int), c[2].(int))
		plan.Cuts, plan.Portable.Cuts = append(plan.Cuts, edit), append(plan.Portable.Cuts, portable)
	}
	safe, _ := design.Safe(ratio)
	owned := clip.NarrationCaption("narration-1", "여기 좋아요", 500, 3500)
	at := clip.CaptionPlacement{X: int(safe.X) + 24, Y: int(safe.Y+safe.Height) - 400}
	owned.Owner = clip.OwnerCaption{Style: "neon", Size: 72, Position: &at}
	owned.OwnerEdited = true
	crossing := clip.NarrationCaption("narration-2", "두 컷을 지나요", 9500, 12500)
	crossing.Resolved.Element.Style = "film"
	if pace == "rapid" {
		owned.Pace, crossing.Pace = "rapid", "rapid"
	}
	plan.Portable.Elements = []clip.PortableText{owned, crossing}
	regions := clip.ProjectRegions{
		Intro: clip.ProjectRegion{Enabled: true, Slots: []clip.RegionSlot{
			{ID: "project-intro-1", Text: "성수 로컬", OwnerFixed: true},
			{ID: "project-intro-2", OwnerFixed: true},
			{ID: "project-intro-3", Text: "저녁 영업", OwnerFixed: true},
		}},
		Outro: clip.ProjectRegion{Enabled: true, Slots: []clip.RegionSlot{
			{ID: "project-outro-1", Text: "다시 만나요", OwnerFixed: true},
			{ID: "project-outro-2"},
		}},
	}
	projected, _, err := clip.ProjectPlanRegions(plan, regions, plan.Design().RegionPresets())
	if err != nil {
		t.Fatal(err)
	}
	return projected, regions
}

// CDS-52 V17 and V20 across both paths, on every ratio and at both caption paces: the export
// layout and the draft preview show the same texts over the same output intervals; the regions
// draw the project's exact slot words in order in their presets' geometry; the owner's caption
// keeps its outside-set style, its size and its place; the writer's keeps its own style; and at
// the clip's start, middle and end the same elements stand in both.
func TestEditedRegionsAndCaptionsReachPreviewAndExportAlike(t *testing.T) {
	cfg := clip.PreviewConfig{MaxAssets: 16, MaxAssetBytes: 512 << 10, MaxResponseBytes: 8 << 20, Timeout: 10 * time.Second}
	sources := []clip.RenderSource{{ID: "source", Fingerprint: "fp", Info: clip.MediaInfo{DurationMS: 15000, Width: 1080, Height: 1920}}}
	for _, ratio := range []string{"vertical", "horizontal", "square"} {
		for _, pace := range []string{"steady", "rapid"} {
			t.Run(ratio+"/"+pace, func(t *testing.T) {
				plan, regions := editedPlan(t, ratio, pace)
				layout := measuredDeclared(t, plan)
				elements := layout.elements()
				if err := clip.VerifyCompositionManifest(layout.plan, elements, clip.DefaultCompositionLimits()); err != nil {
					t.Fatal("the verifier refused the edited clip:", err)
				}
				byID := map[string]clip.CompositionElement{}
				for _, e := range elements {
					byID[e.InstanceID] = e
				}
				// V20: each region draws exactly its active slots' words, in order, in its preset.
				for kind, preset := range map[string]string{"intro": "cover", "outro": "b"} {
					region := byID["project-"+kind]
					spec, _ := design.Region(kind, preset)
					rows := []string{}
					for _, slot := range regions.Region(kind).Slots[:len(spec.Slots())] {
						rows = append(rows, slot.Text)
					}
					if err := design.VerifyRegion(kind, preset, ratio, rows, 0, len(rows), true, region.Parts); err != nil {
						t.Fatalf("%s left its preset or its words: %v", kind, err)
					}
				}
				owner, writer := byID["narration-1"], byID["narration-2"]
				if owner.Style != "neon" || writer.Style != "film" {
					t.Fatalf("the captions were restyled: %q %q", owner.Style, writer.Style)
				}
				if owner.Text != "여기 좋아요" || owner.StartMS != 500 || owner.EndMS != 3500 || writer.StartMS != 9500 || writer.EndMS != 12500 {
					t.Fatalf("the captions moved: %+v %+v", owner, writer)
				}
				safe, _ := design.Safe(ratio)
				if !owner.OwnerPlaced || owner.Region.X != safe.X+24 || owner.Region.X+owner.Region.Width > safe.X+safe.Width+0.01 || owner.Region.Y+owner.Region.Height > safe.Y+safe.Height+0.01 {
					t.Fatalf("the owner's place was not kept inside the safe area: %v", owner.Region)
				}
				for _, part := range owner.Parts {
					if part.Kind == "copy" && part.FontSize != 72 {
						t.Fatalf("the owner's size was not kept: %v", part.FontSize)
					}
				}
				// V17: the preview prepares the same texts over the same intervals, the
				// sequence style as one representative frame of its motion.
				_, r, _ := previewMeasured(t, ratio)
				preview, err := r.PreparePreview(t.Context(), plan, sources, nil, 0, cfg)
				if err != nil {
					t.Fatal(err)
				}
				// A rapid caption is shown phrase by phrase: its windows are its cues.
				windows := func(e clip.CompositionElement) [][2]int {
					if len(e.Cues) == 0 {
						return [][2]int{{e.StartMS, e.EndMS}}
					}
					out := [][2]int{}
					for _, cue := range e.Cues {
						out = append(out, [2]int{cue.StartMS, cue.EndMS})
					}
					return out
				}
				assets := map[string][][2]int{}
				moving := map[string]bool{}
				for _, asset := range preview.Assets {
					assets[asset.InstanceID] = append(assets[asset.InstanceID], [2]int{asset.StartMS, asset.EndMS})
					moving[asset.InstanceID] = moving[asset.InstanceID] || asset.RepresentativeFrame
				}
				for _, id := range []string{"project-intro", "project-outro", "narration-1", "narration-2"} {
					if !slices.Equal(assets[id], windows(byID[id])) {
						t.Fatalf("%s: the preview shows %v, the export %v", id, assets[id], windows(byID[id]))
					}
				}
				// The owner's sequence style moves at the steady pace; a rapid
				// phrase is one frame of it whatever its style (CDS-4).
				if moving["narration-1"] != (pace == "steady") || moving["narration-2"] {
					t.Fatal("the preview does not state which drawing moves")
				}
				// Start, middle and end: the second cut holds no caption.
				for _, at := range []int{1000, 5500, 14000} {
					shown := func(start, end int) bool { return start <= at && at < end }
					exported, previewed := []string{}, []string{}
					for _, e := range elements {
						if slices.ContainsFunc(windows(e), func(w [2]int) bool { return shown(w[0], w[1]) }) {
							exported = append(exported, e.InstanceID)
						}
					}
					for _, asset := range preview.Assets {
						if shown(asset.StartMS, asset.EndMS) {
							previewed = append(previewed, asset.InstanceID)
						}
					}
					slices.Sort(exported)
					slices.Sort(previewed)
					if !slices.Equal(exported, previewed) {
						t.Fatalf("at %d ms the export shows %v and the preview %v", at, exported, previewed)
					}
					if at == 5500 && len(exported) != 0 {
						t.Fatalf("the uncaptioned cut shows %v", exported)
					}
				}
			})
		}
	}
}
