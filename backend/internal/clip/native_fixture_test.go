package clip_test

import (
	"fmt"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
)

// nativePortable is the composition a plan of these cuts carries when it is
// written today: the no-template document, one binding per cut and every copy
// as a fixed caption bound to its cut, on the transformed output timeline. It is
// what a test that edits a native plan starts from.
func nativePortable(plan clip.EditPlan) *clip.PortablePlan {
	out := &clip.PortablePlan{
		Snapshot: clip.CompositionSnapshot{Version: clip.CompositionVersion, Body: clip.EmptyCompositionBody()},
		Inputs:   clip.CompositionInputs{Values: map[string]string{}, Items: map[string][]composition.Item{}},
	}
	offset := 0
	for _, cut := range plan.Cuts {
		offset -= cut.TransitionMS
		out.Cuts = append(out.Cuts, composition.Cut{ID: cut.ID, SectionID: "footage", SourceID: cut.SourceID, StartMS: cut.StartMS, EndMS: cut.EndMS, TransitionMS: cut.TransitionMS, PlaybackRatePermille: cut.Rate()})
		for index, copy := range cut.Copies {
			if copy.Text == "" {
				continue
			}
			start, end := cut.CaptionWindow(index)
			id := fmt.Sprintf("copy-%s-%d", cut.ID, index)
			e := composition.Element{ID: id, Kind: "fixed", Role: "caption", Style: copy.Style, Position: copy.Anchor, Align: copy.Align, Basis: "cut", StartMS: &start, EndMS: &end, Parts: []composition.Part{{Literal: copy.Text}}}
			if copy.StartMS == 0 && copy.EndMS == 0 {
				e.StartMS, e.EndMS = nil, nil
			}
			out.Elements = append(out.Elements, clip.PortableText{Accent: copy.Accent, Keyword: copy.Keyword, Pace: copy.Pace, Scope: "scene",
				Resolved: composition.ResolvedElement{InstanceID: id, CutID: cut.ID, Element: e, Text: copy.Text, StartMS: offset + start, EndMS: offset + end, AuthoredTiming: copy.StartMS != 0 || copy.EndMS != 0},
				Evidence: []clip.SourceEvidence{{SourceID: cut.SourceID, Fingerprint: cut.Fingerprint, StartMS: cut.StartMS, EndMS: cut.EndMS}}})
		}
		offset += cut.OutputDurationMS()
	}
	return out
}
