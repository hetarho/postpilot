package media

import (
	"errors"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
)

// narrationPlan is a two-cut flow of 7.5 s each with the narration written over
// it: captions that belong to no cut and may cross the boundary between them.
func narrationPlan(t *testing.T, captions ...clip.PortableText) clip.EditPlan {
	t.Helper()
	body := `<clip version="1" intro="b" caption="bold" outro="e"><field id="place" label="상호" required="true">가게</field></clip>`
	plan := clip.EditPlan{Ratio: "vertical", DurationMS: 15000,
		Cuts: []clip.Cut{
			{ID: "one", SourceID: "source", Fingerprint: "fp", StartMS: 0, EndMS: 7500, Focal: clip.Point{X: .5, Y: .5}},
			{ID: "two", SourceID: "source", Fingerprint: "fp", StartMS: 7500, EndMS: 15000, Focal: clip.Point{X: .5, Y: .5}},
		},
		Portable: &clip.PortablePlan{
			Snapshot: clip.CompositionSnapshot{Version: 1, Body: body},
			Inputs:   clip.CompositionInputs{Values: map[string]string{"place": "성수 곱창"}},
			Cuts: []composition.Cut{
				{ID: "one", SourceID: "source", StartMS: 0, EndMS: 7500, PlaybackRatePermille: 1000},
				{ID: "two", SourceID: "source", StartMS: 7500, EndMS: 15000, PlaybackRatePermille: 1000},
			},
			Observations: []clip.SourceAnalysis{{
				Source: clip.AnalysisSource{RenderSource: clip.RenderSource{ID: "source", Fingerprint: "fp", Info: clip.MediaInfo{DurationMS: 15000, Width: 1080, Height: 1920}}},
				Segments: []clip.Segment{
					{StartMS: 0, EndMS: 7500, Event: "담는다", Quality: "clear", Scene: "food", Subject: clip.Region{X: .1, Y: .1, Width: .3, Height: .3}, Certainty: clip.CertaintyCertain, Usability: clip.UsabilityUsable},
					{StartMS: 7500, EndMS: 15000, Event: "먹는다", Quality: "clear", Scene: "food", Subject: clip.Region{X: .6, Y: .6, Width: .3, Height: .3}, Certainty: clip.CertaintyCertain, Usability: clip.UsabilityUsable},
				},
			}},
			Elements: captions,
		}}
	return plan
}

func narrationText(id, text string, start, end int) clip.PortableText {
	return clip.NarrationCaption(id, text, start, end)
}

func TestNarrationIsScheduledAcrossTheWholeTimeline(t *testing.T) {
	// Three captions, the last two colliding: the later one is omitted rather
	// than moved, and neither is cut-bound.
	plan := narrationPlan(t,
		narrationText("narration-1", "고기를 올렸어요", 1000, 5000),
		narrationText("narration-2", "국물도 나왔어요", 6000, 10000),
		narrationText("narration-3", "겹치는 자막", 9000, 12000),
	)
	kept, drops := scheduleDeclaredCaptions(plan)
	if len(kept) != 2 || len(drops) != 1 || drops[0].Reason != clip.NoticeCaptionOverlap || drops[0].ElementID != "narration-3" {
		t.Fatal("the overlapping caption was not the one omitted", kept, drops)
	}
	if kept[0].Resolved.StartMS != 1000 || kept[0].Resolved.EndMS != 5000 || kept[1].Resolved.StartMS != 6000 {
		t.Fatal("a surviving caption was retimed", kept[0].Resolved, kept[1].Resolved)
	}
	// The per-cut sentence limit is the legacy plan's rule: a narration may hold
	// as many captions over one cut as its own windows allow.
	crowded := narrationPlan(t,
		narrationText("narration-1", "첫 자막", 0, 2000),
		narrationText("narration-2", "둘째 자막", 2000, 4000),
		narrationText("narration-3", "셋째 자막", 4000, 6000),
	)
	kept, drops = scheduleDeclaredCaptions(crowded)
	if len(kept) != 3 || len(drops) != 0 {
		t.Fatal("the per-cut sentence limit was applied to the narration", kept, drops)
	}
}

func TestNarrationFloorTakesTheShorterTextThenTheFreeRoom(t *testing.T) {
	long := "아주 긴 문장을 하나 씁니다"
	floor := clip.MinExposureMS(long)
	// Nothing after it: the caption takes the room it needs to be read.
	plan := narrationPlan(t, narrationText("narration-1", long, 1000, 1500))
	kept, drops := scheduleDeclaredCaptions(plan)
	if len(kept) != 1 || len(drops) != 0 || kept[0].Resolved.EndMS != 1000+floor {
		t.Fatal("the caption did not take the free room", kept, drops)
	}
	// A neighbour leaves no room, but a grounded shorter sentence fits.
	short := narrationText("narration-1", long, 1000, 1000+clip.MinExposureMS("짧은 문장"))
	short.Alternatives = []clip.CopyAlternative{{Text: "짧은 문장"}}
	plan = narrationPlan(t, short, narrationText("narration-2", "다음 자막", short.Resolved.EndMS, 12000))
	kept, _ = scheduleDeclaredCaptions(plan)
	if len(kept) != 2 || kept[0].Resolved.Text != "짧은 문장" || kept[0].FallbackReason != "shorter_copy" {
		t.Fatal("the shorter sentence was not used", kept)
	}
	// Neither fits: the caption is omitted with the floor's own reason.
	plan = narrationPlan(t, narrationText("narration-1", long, 1000, 1300), narrationText("narration-2", "다음 자막", 1300, 6000))
	kept, drops = scheduleDeclaredCaptions(plan)
	if len(kept) != 1 || len(drops) != 1 || drops[0].Reason != clip.NoticeCaptionFloor {
		t.Fatal("an unreadable caption was kept", kept, drops)
	}
	// An owner-edited caption keeps its own window, whatever the floor says.
	owned := narrationText("narration-1", long, 1000, 1300)
	owned.OwnerEdited = true
	kept, drops = scheduleDeclaredCaptions(narrationPlan(t, owned))
	if len(kept) != 1 || len(drops) != 0 || kept[0].Resolved.EndMS != 1300 {
		t.Fatal("the owner's own window was rescheduled", kept, drops)
	}
}

func TestASpanningCaptionIsPlacedAgainstEveryCutItCovers(t *testing.T) {
	canvas, err := clip.ClipCanvas("vertical")
	if err != nil {
		t.Fatal(err)
	}
	plan := narrationPlan(t, narrationText("narration-1", "두 컷을 지나갑니다", 6000, 9000))
	// Footage text in the SECOND cut only: a caption that reaches into it is
	// held away from the photographed text, one placed before it is not.
	plan.Portable.Observations[0].Segments[1].ReadableText = true
	subject, readable, _ := coveredFootage(canvas, plan, plan.Portable.Elements[0])
	// The caption covers both cuts: the subject box holds both subjects and the
	// second cut's readable text reaches it.
	if !readable {
		t.Fatal("readable footage under the caption was not seen")
	}
	if subject.Width <= .4 || subject.Height <= .4 {
		t.Fatal("the subject box did not cover both cuts", subject)
	}
	// A caption inside one cut reads that cut alone.
	inside := narrationPlan(t, narrationText("narration-1", "첫 컷 안입니다", 1000, 5000))
	inside.Portable.Observations[0].Segments[1].ReadableText = true
	_, readable, _ = coveredFootage(canvas, inside, inside.Portable.Elements[0])
	if readable {
		t.Fatal("a caption inside the first cut read the second cut's footage")
	}
}

func TestNarrationPhrasesAreAbsoluteAndRenderedAcrossCuts(t *testing.T) {
	caption := narrationText("narration-1", "두 컷을 지나갑니다", 6000, 9000)
	caption.Pace = "rapid"
	plan := narrationPlan(t, caption)
	layout := measuredDeclared(t, plan)
	elements := layout.elements()
	if len(elements) != 1 || elements[0].StartMS != 6000 || elements[0].EndMS != 9000 {
		t.Fatal("the caption lost its absolute window", elements)
	}
	for _, cue := range elements[0].Cues {
		if cue.StartMS < 6000 || cue.EndMS > 9000 {
			t.Fatal("a phrase was placed against a cut instead of the output", cue)
		}
	}
	// The stored phrases come back absolute too, not relative to any cut.
	for _, text := range layout.plan.Portable.Elements {
		for _, phrase := range text.Phrases {
			if phrase.StartMS < 6000 || phrase.EndMS > 9000 {
				t.Fatal("a stored phrase was offset by a cut", phrase)
			}
		}
	}
}

func TestVerifierHoldsTheNarrationToTheTimelineAndItsFacts(t *testing.T) {
	plan := narrationPlan(t,
		narrationText("narration-1", "성수 곱창입니다", 1000, 5000),
		narrationText("narration-2", "국물도 나왔어요", 6000, 10000),
	)
	layout := measuredDeclared(t, plan)
	elements := layout.elements()
	if err := clip.VerifyCompositionManifest(layout.plan, elements, clip.DefaultCompositionLimits()); err != nil {
		t.Fatal("a valid narration was refused", err)
	}
	problem := &composition.Problem{}
	// V18: two captions claiming the same moment, whatever cut lies beneath.
	overlapping := elements
	overlapping[1].StartMS = overlapping[0].StartMS
	layout.plan.Portable.Elements[1].Resolved.StartMS = overlapping[0].StartMS
	err := clip.VerifyCompositionManifest(layout.plan, overlapping, clip.DefaultCompositionLimits())
	if !errors.As(err, &problem) || problem.Reason != clip.NoticeCaptionOverlap {
		t.Fatal("overlapping narration was rendered", err)
	}
	// V16: an interval the output does not hold.
	beyond := measuredDeclared(t, narrationPlan(t, narrationText("narration-1", "성수 곱창입니다", 1000, 5000)))
	outside := beyond.elements()
	outside[0].EndMS = beyond.plan.DurationMS + 1
	beyond.plan.Portable.Elements[0].Resolved.EndMS = outside[0].EndMS
	err = clip.VerifyCompositionManifest(beyond.plan, outside, clip.DefaultCompositionLimits())
	if !errors.As(err, &problem) || problem.Reason != "interval_outside" {
		t.Fatal("a caption past the end was rendered", err)
	}
}

func TestVerifierRefusesANumberNoCollectedFactStates(t *testing.T) {
	a, r := measured(t)
	layoutError := func(plan clip.EditPlan) error {
		return a.WithWorkspace(t.Context(), "narration-grounding", func(ws clip.MediaWorkspace) error {
			_, err := r.layoutComposition(t.Context(), ws, plan)
			return err
		})
	}
	problem := &composition.Problem{}
	err := layoutError(narrationPlan(t, narrationText("narration-1", "12,000원입니다", 1000, 5000)))
	if !errors.As(err, &problem) || problem.Reason != "unsupported_number_unit" {
		t.Fatal("an ungrounded number was rendered", err)
	}
	// The same number, collected as an item fact of a dish the caption never
	// names: a caption belongs to no item and may state any of them.
	grounded := narrationPlan(t, narrationText("narration-1", "12,000원입니다", 1000, 5000))
	grounded.Portable.Inputs.Items = map[string][]composition.Item{"menu": {{ID: "sea", Values: map[string]string{"price": "12,000원"}}}}
	if err := layoutError(grounded); err != nil {
		t.Fatal("a grounded number was refused", err)
	}
	// An owner-written caption is the owner's own claim.
	owned := narrationPlan(t, narrationText("narration-1", "9,000원입니다", 1000, 5000))
	owned.Portable.Elements[0].OwnerEdited = true
	if err := layoutError(owned); err != nil {
		t.Fatal("the owner's own sentence was ground checked", err)
	}
}

func TestPreviewShowsNarrationAtItsAbsoluteTimesAcrossCuts(t *testing.T) {
	cfg := clip.PreviewConfig{MaxAssets: 8, MaxAssetBytes: 512 << 10, MaxResponseBytes: 4 << 20, Timeout: 5 * time.Second}
	_, r, _ := previewMeasured(t, "vertical")
	plan := narrationPlan(t, narrationText("narration-1", "두 컷을 지나갑니다", 6000, 9000))
	sources := []clip.RenderSource{{ID: "source", Fingerprint: "fp", Info: clip.MediaInfo{DurationMS: 15000, Width: 1920, Height: 1080}}}
	result, err := r.PreparePreview(t.Context(), plan, sources, []string{"narration-1"}, 0, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Assets) != 1 || result.Assets[0].StartMS != 6000 || result.Assets[0].EndMS != 9000 {
		t.Fatal("the preview did not show the caption at its own output times", result.Assets)
	}
}
