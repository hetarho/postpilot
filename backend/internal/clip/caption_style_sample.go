package clip

import (
	"fmt"
	"strings"

	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/clip/design"
)

// The words every style is sampled with, and the length of the cut each sample
// caption owns. The words are the server's because the sample is an offer, not
// a clip: ① shows the style, never this project's own copy, which does not
// exist until a generation has written it.
const (
	captionSampleText = "오늘의 한 장면"
	// The sample clip is as long as a clip may be short, so a set of any size
	// produces a plan the render rules already admit.
	captionSampleMinMS = 15000
)

// CaptionStyleSamplePlan is the plan one sample caption per style is drawn from
// (CLIP-142, CDS-83): each style on a cut of its own so no two samples share a
// moment, each caption placed by the owner so the anchor walk never moves one,
// and every style allowed so each caption may actually take the one it samples.
// Nothing here is stored or rendered — it exists for the length of one drawing.
func CaptionStyleSamplePlan(ratio string, styles []string) (EditPlan, []RenderSource, error) {
	if len(styles) == 0 {
		return EditPlan{}, nil, ErrInvalid
	}
	safe, ok := design.Safe(ratio)
	if !ok {
		return EditPlan{}, nil, ErrInvalid
	}
	cutMS := MinExposureMS(captionSampleText) + design.Timing.SubMinBaseMS
	if len(styles)*cutMS < captionSampleMinMS {
		cutMS = (captionSampleMinMS + len(styles) - 1) / len(styles)
	}
	var body strings.Builder
	body.WriteString(`<clip version="1">`)
	for i := range styles {
		body.WriteString(fmt.Sprintf(`<scene id="sample-%d"><text id="caption-%d" kind="ai" role="caption" basis="cut">sample</text></scene>`, i, i))
	}
	body.WriteString(`</clip>`)
	limits := composition.Limits{SourceChars: 1 << 15, Nodes: 4 * len(styles), Fields: 1, Items: 1, Cuts: len(styles),
		Cues: 4 * len(styles), Stages: 1, LabelChars: 40, PromptChars: 200, AnswerChars: 500, CopyChars: 500,
		GuideChars: 4000, MaxDurationMS: len(styles) * cutMS, AutoInsetMS: 0}
	doc, problem := composition.ReadStored(body.String(), limits)
	if problem != nil {
		return EditPlan{}, nil, problem
	}
	bindings := make([]composition.Cut, 0, len(styles))
	cuts := make([]EditCut, 0, len(styles))
	for i := range styles {
		start, end := i*cutMS, (i+1)*cutMS
		bindings = append(bindings, composition.Cut{ID: fmt.Sprintf("cut-%d", i), SectionID: fmt.Sprintf("sample-%d", i),
			SourceID: "sample", StartMS: start, EndMS: end, PlaybackRatePermille: composition.RateUnitPermille})
		cuts = append(cuts, EditCut{ID: fmt.Sprintf("cut-%d", i), SourceID: "sample", Fingerprint: "sample",
			StartMS: start, EndMS: end, Focal: Point{X: .5, Y: .5}})
	}
	resolved, problem := composition.Resolve(doc, composition.Inputs{Cuts: bindings}, limits, 1<<20)
	if problem != nil {
		return EditPlan{}, nil, problem
	}
	duration := len(styles) * cutMS
	plan := EditPlan{Ratio: ratio, DurationMS: duration, Cuts: cuts, CaptionStyles: append([]string{}, styles...),
		Portable: &PortablePlan{Snapshot: CompositionSnapshot{Version: 1, Body: body.String()}, Cuts: bindings, TargetDurationMS: duration}}
	at := CaptionPlacement{X: int(safe.X), Y: int(safe.Y)}
	for i, element := range resolved.Elements {
		if i >= len(styles) {
			break
		}
		position := at
		plan.Portable.Elements = append(plan.Portable.Elements, PortableText{Resolved: element,
			Owner: OwnerCaption{Position: &position, Style: styles[i]}, OwnerEdited: true})
		plan.Portable.Elements[i].Resolved.Text = captionSampleText
	}
	// One source, with metadata only: the drawing measures with the bundled
	// faces and reads no footage, so nothing here opens a file.
	source := RenderSource{ID: "sample", Fingerprint: "sample", Info: MediaInfo{DurationMS: duration, Width: 1920, Height: 1080}}
	return plan, []RenderSource{source}, nil
}
