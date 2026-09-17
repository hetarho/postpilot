package clip_test

import (
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/clip/design"
	"github.com/postpilot/backend/internal/platform/config"
)

func captionElement(instance, style string, startMS, endMS int) clip.PortableText {
	return clip.PortableText{Owner: clip.OwnerCaption{Style: style}, Resolved: composition.ResolvedElement{
		InstanceID: instance, Text: "여기 진짜 좋아요", StartMS: startMS, EndMS: endMS,
		Element: composition.Element{ID: instance, Kind: "ai", Role: "caption", Basis: "cut"}}}
}

// CDS-81: a static style is rasterised once however long it is on screen, so
// only the sequence-rendered captions are counted, and what they add is their
// own frames times the measured per-frame cost — not a guess from the count.
func TestTheQuoteCountsSequenceCaptionsAndWhatTheirFramesAdd(t *testing.T) {
	cfg := config.ClipRender(&config.Config{})
	project := clip.Project{CaptionStyles: []string{design.DefaultCaptionStyle, "neon", "word-pop"}}
	plan := clip.EditPlan{Portable: &clip.PortablePlan{Elements: []clip.PortableText{
		captionElement("static", design.DefaultCaptionStyle, 0, 3000),
		captionElement("sequence", "neon", 1000, 3000),
		// No owner style: the caption takes the project's first allowed style,
		// exactly as the layout resolves it — here the static default.
		captionElement("default", "", 3000, 4000),
	}}}
	// One rapid caption is drawn over its phrases, each with its own interval.
	rapid := captionElement("rapid", "word-pop", 4000, 8000)
	rapid.Pace = "rapid"
	rapid.Phrases = []clip.EditablePhrase{{Text: "여기", StartMS: 4000, EndMS: 4500}, {Text: "진짜 좋아요", StartMS: 4500, EndMS: 5000}}
	plan.Portable.Elements = append(plan.Portable.Elements, rapid)
	cost := clip.SequenceCostOf(project, plan, true, cfg)
	if !cost.FromPlan || cost.Captions != 2 {
		t.Fatalf("counted %d sequence captions from a plan holding two: %+v", cost.Captions, cost)
	}
	// 2 s of neon plus two half-second phrases, at 30 fps.
	if cost.Frames != 60+15+15 {
		t.Fatalf("frames %d, want %d", cost.Frames, 90)
	}
	if cost.AddedRenderMS != cost.Frames*cfg.SequenceFrameCostMS {
		t.Fatalf("added %d ms for %d frames", cost.AddedRenderMS, cost.Frames)
	}
	if cost.SelectedStyles != 2 {
		t.Fatalf("the selection holds two sequence styles, counted %d", cost.SelectedStyles)
	}
}

// A project using only static styles has nothing to state, before a plan exists
// and after: zero captions and no added time, never an estimate of one.
func TestOnlyStaticStylesCostNothingExtra(t *testing.T) {
	cfg := config.ClipRender(&config.Config{})
	project := clip.Project{CaptionStyles: []string{design.DefaultCaptionStyle}}
	plan := clip.EditPlan{Portable: &clip.PortablePlan{Elements: []clip.PortableText{
		captionElement("a", design.DefaultCaptionStyle, 0, 3000)}}}
	for _, cost := range []clip.SequenceCaptionCost{
		clip.SequenceCostOf(project, plan, true, cfg),
		clip.SequenceCostOf(project, clip.EditPlan{}, false, cfg),
	} {
		if cost.Captions != 0 || cost.Frames != 0 || cost.AddedRenderMS != 0 || cost.SelectedStyles != 0 {
			t.Fatalf("a static-only clip was quoted sequence work: %+v", cost)
		}
	}
	// A clip that has not been generated yet is counted from the selection
	// alone: the styles are known, the captions are not.
	pending := clip.Project{CaptionStyles: []string{"neon", "serif"}}
	cost := clip.SequenceCostOf(pending, clip.EditPlan{}, false, cfg)
	if cost.FromPlan || cost.SelectedStyles != 2 || cost.Captions != 0 || cost.AddedRenderMS != 0 {
		t.Fatalf("a project with no plan was quoted as if it had one: %+v", cost)
	}
	// An empty selection is the default style alone (CLIP-142), which is static.
	if clip.SequenceCostOf(clip.Project{}, clip.EditPlan{}, false, cfg).SelectedStyles != 0 {
		t.Fatal("the default style was counted as sequence-rendered")
	}
}
