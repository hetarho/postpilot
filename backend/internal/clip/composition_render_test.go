package clip_test

import (
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/platform/config"
)

func TestReadingExtensionStaysWithinObservedSceneTargetAndOwnerBinding(t *testing.T) {
	fixture := func() clip.EditPlan {
		p := portableTimingFixture()
		p.Cuts[0].EndMS = 1000
		p.Cuts[1].StartMS, p.Cuts[1].EndMS = 2000, 16000
		p.Portable.TargetDurationMS = 16000
		p.Portable.Elements = p.Portable.Elements[:1]
		p.Portable.Elements = append(p.Portable.Elements, clip.PortableText{Pace: "steady", Resolved: composition.ResolvedElement{InstanceID: "ai/a", CutID: "a", Text: "장면을 보여줘요", Element: composition.Element{ID: "ai", Kind: "ai", Role: "caption", Style: "auto", Position: "auto", Basis: "cut"}}})
		p.Portable.Observations = []clip.SourceAnalysis{{Source: clip.AnalysisSource{RenderSource: clip.RenderSource{ID: "source", Fingerprint: "fp", Info: clip.MediaInfo{DurationMS: 16000}}}, Segments: []clip.Segment{{StartMS: 0, EndMS: 2000}}}}
		return p
	}
	for _, tc := range []struct {
		name   string
		change func(*clip.EditPlan)
		want   bool
	}{
		{"available", func(*clip.EditPlan) {}, true},
		{"target", func(p *clip.EditPlan) { p.Portable.TargetDurationMS = 15000 }, false},
		{"observation", func(p *clip.EditPlan) { p.Portable.Observations[0].Segments[0].EndMS = 1000 }, false},
		{"next item", func(p *clip.EditPlan) {
			p.Portable.Inputs.Associations = []clip.SourceAssociation{{SourceID: "source", Fingerprint: "fp", StartMS: 0, EndMS: 1000, GroupID: "menu", ItemID: "one"}}
		}, false},
		{"explicit", func(p *clip.EditPlan) { p.Portable.Elements[1].Resolved.Element.Style = "clean" }, false},
		{"short alternative", func(p *clip.EditPlan) { p.Portable.Elements[1].Alternatives = []clip.CopyAlternative{{Text: "a"}} }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := fixture()
			tc.change(&p)
			got, err := clip.ExtendCompositionReadingWindows(p, config.ClipCompositionLimits())
			if err != nil {
				t.Fatal(err)
			}
			if (got.Cuts[0].EndMS > 1000) != tc.want || got.Cuts[0].EndMS > 2000 || got.DurationMS > 16000 {
				t.Fatalf("invalid extension: %+v", got)
			}
			if p.Cuts[0].EndMS != 1000 {
				t.Fatal("mutated retained source trim")
			}
		})
	}
}

func portableTimingFixture() clip.EditPlan {
	start, end, endingStart, endingEnd := 500, 2000, -2000, 0
	return clip.EditPlan{Ratio: "vertical", DurationMS: 15000, Cuts: []clip.Cut{
		{ID: "a", SourceID: "source", Fingerprint: "fp", StartMS: 0, EndMS: 5000},
		{ID: "b", SourceID: "source", Fingerprint: "fp", StartMS: 5000, EndMS: 15000},
	}, Portable: &clip.PortablePlan{Snapshot: clip.CompositionSnapshot{Version: 1, Body: `<clip version="1"/>`}, Cuts: []composition.Cut{
		{ID: "a", SourceID: "source", StartMS: 0, EndMS: 5000}, {ID: "b", SourceID: "source", StartMS: 5000, EndMS: 15000},
	}, Elements: []clip.PortableText{
		{Resolved: composition.ResolvedElement{InstanceID: "whole", Element: composition.Element{ID: "whole", Kind: "fixed", Basis: "whole"}, Text: "  whole unchanged  "}},
		{Resolved: composition.ResolvedElement{InstanceID: "ending", Element: composition.Element{ID: "ending", Kind: "fixed", Basis: "output-end", StartMS: &endingStart, EndMS: &endingEnd}, Text: "end"}},
		{Resolved: composition.ResolvedElement{InstanceID: "cut/a", CutID: "a", Element: composition.Element{ID: "cut", Kind: "fixed", Basis: "cut", StartMS: &start, EndMS: &end, Span: composition.Span{Line: 23}}, Text: "exact cut text"}},
	}}}
}

func TestPortableIntervalsFollowCutsAndKeepOutputScope(t *testing.T) {
	for _, tc := range []struct {
		name                       string
		edit                       func(*clip.EditPlan)
		duration, cutStart, cutEnd int
		elements                   int
	}{
		{"initial", func(*clip.EditPlan) {}, 15000, 500, 2000, 3},
		{"reorder", func(p *clip.EditPlan) { slices.Reverse(p.Cuts) }, 15000, 10500, 12000, 3},
		{"crossfade", func(p *clip.EditPlan) { p.Cuts[1].TransitionMS = 200 }, 14800, 500, 2000, 3},
		{"removed", func(p *clip.EditPlan) { p.Cuts = p.Cuts[1:] }, 10000, 0, 0, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := portableTimingFixture()
			tc.edit(&p)
			before := *p.Portable
			got, err := clip.ResolvePortableIntervals(p, config.ClipCompositionLimits())
			if err != nil {
				t.Fatal(err)
			}
			if got.DurationMS != tc.duration || len(got.Portable.Elements) != tc.elements {
				t.Fatalf("%+v", got.Portable)
			}
			whole, ending := got.Portable.Elements[0].Resolved, got.Portable.Elements[1].Resolved
			if whole.StartMS != 0 || whole.EndMS != tc.duration || whole.Text != "  whole unchanged  " || ending.StartMS != tc.duration-2000 || ending.EndMS != tc.duration {
				t.Fatal("lost output scope")
			}
			if tc.elements == 3 {
				c := got.Portable.Elements[2].Resolved
				if c.StartMS != tc.cutStart || c.EndMS != tc.cutEnd || !c.AuthoredTiming {
					t.Fatalf("%+v", c)
				}
			}
			if !reflect.DeepEqual(before, *p.Portable) {
				t.Fatal("mutated retained draft")
			}
		})
	}
}

func TestPortableInvalidAuthoredIntervalPreservesLiteralDraft(t *testing.T) {
	p := portableTimingFixture()
	p.Cuts[0].EndMS = 1500
	got, err := clip.ResolvePortableIntervals(p, config.ClipCompositionLimits())
	var problem *composition.Problem
	if !errors.As(err, &problem) || problem.ElementID != "cut" || problem.Line != 23 || problem.Reason != "interval_outside" {
		t.Fatalf("%+v %v", problem, err)
	}
	if got.Portable.Elements[2].Resolved.Text != "exact cut text" || *got.Portable.Elements[2].Resolved.Element.EndMS != 2000 {
		t.Fatal("silently repaired authored interval")
	}
}

func TestOnlyAutomaticGeneratedCompositionEntersRepair(t *testing.T) {
	base := composition.Element{Kind: "ai", Style: "auto", Position: "auto", Basis: "cut"}
	if !clip.AutomaticCompositionRepair(clip.PortableText{Resolved: composition.ResolvedElement{Element: base}}) {
		t.Fatal("automatic copy cannot repair")
	}
	for _, edit := range []func(*composition.Element){func(e *composition.Element) { e.Kind = "fixed" }, func(e *composition.Element) { e.Style = "bold" }, func(e *composition.Element) { e.Position = "top" }, func(e *composition.Element) { e.Basis = "whole" }, func(e *composition.Element) { start, end := 0, 1000; e.StartMS = &start; e.EndMS = &end }} {
		e := base
		edit(&e)
		if clip.AutomaticCompositionRepair(clip.PortableText{Resolved: composition.ResolvedElement{Element: e}}) {
			t.Fatalf("authored declaration enters repair: %+v", e)
		}
	}
}
