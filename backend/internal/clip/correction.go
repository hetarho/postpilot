package clip

import (
	"encoding/json"
	"errors"
	"io"
	"math"
	"slices"
	"strings"
)

var ErrPlanConflict = errors.New("clip edit plan revision conflict")

// The editable representation uses integer thousandths, including its durable JSON.
// Framing is deliberately not editable: source identity and focal points stay frozen.
type CorrectionCut struct {
	ID, SourceID, Fingerprint string
	StartMS, EndMS            int
	Copy                      Caption
	VolumePermille            int
}
type CorrectionPlan struct {
	DurationMS int
	Cuts       []CorrectionCut
}
type CorrectionState struct {
	Plan                                                        CorrectionPlan
	Sources                                                     []AnalysisSource
	CopyStyles                                                  []string
	FadeMS, MaxCuts, MaxCopyRunes, MinDurationMS, MaxDurationMS int
}
type storedEditPlan struct {
	Version    int
	Ratio      string
	Plan       CorrectionPlan
	Focals     map[string]Point
	CopyStyles []string
}

func CorrectionFromPlan(p EditPlan) CorrectionPlan {
	out := CorrectionPlan{DurationMS: p.DurationMS, Cuts: make([]CorrectionCut, 0, len(p.Cuts))}
	for _, c := range p.Cuts {
		out.Cuts = append(out.Cuts, CorrectionCut{c.ID, c.SourceID, c.Fingerprint, c.StartMS, c.EndMS, c.Copy, int(math.Round(c.OriginalVolume() * 1000))})
	}
	return out
}
func EncodeEditPlan(p EditPlan, styles []string) (string, error) {
	focals := map[string]Point{}
	for _, c := range p.Cuts {
		focals[c.ID] = c.Focal
	}
	b, err := json.Marshal(storedEditPlan{1, p.Ratio, CorrectionFromPlan(p), focals, slices.Clone(styles)})
	return string(b), err
}
func strictJSON(raw string, out any) error {
	d := json.NewDecoder(strings.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return ErrInvalid
	}
	if d.Decode(new(any)) != io.EOF {
		return ErrInvalid
	}
	return nil
}
func DecodeEditPlan(raw string) (EditPlan, []string, error) {
	var marker struct{ Version int }
	if json.Unmarshal([]byte(raw), &marker) != nil {
		return EditPlan{}, nil, ErrInvalid
	}
	if marker.Version == 0 {
		// T076's original representation. Do not invent template permissions lost in
		// that format: only styles already present in the retained plan are approved.
		var p EditPlan
		if err := strictJSON(raw, &p); err != nil {
			return p, nil, err
		}
		if len(p.Cuts) == 0 || p.DurationMS <= 0 {
			return EditPlan{}, nil, ErrInvalid
		}
		if _, err := ClipCanvas(p.Ratio); err != nil {
			return EditPlan{}, nil, err
		}
		styles := []string{}
		for _, c := range p.Cuts {
			if !slices.Contains(styles, c.Copy.Style) {
				styles = append(styles, c.Copy.Style)
			}
		}
		return p, styles, nil
	}
	var s storedEditPlan
	if err := strictJSON(raw, &s); err != nil || s.Version != 1 {
		return EditPlan{}, nil, ErrInvalid
	}
	if len(s.Plan.Cuts) == 0 || s.Plan.DurationMS <= 0 {
		return EditPlan{}, nil, ErrInvalid
	}
	if _, err := ClipCanvas(s.Ratio); err != nil {
		return EditPlan{}, nil, err
	}
	p := EditPlan{Ratio: s.Ratio, DurationMS: s.Plan.DurationMS}
	for _, c := range s.Plan.Cuts {
		f, ok := s.Focals[c.ID]
		if !ok || c.VolumePermille < 0 || c.VolumePermille > 1000 {
			return EditPlan{}, nil, ErrInvalid
		}
		v := float64(c.VolumePermille) / 1000
		p.Cuts = append(p.Cuts, Cut{ID: c.ID, SourceID: c.SourceID, Fingerprint: c.Fingerprint, StartMS: c.StartMS, EndMS: c.EndMS, Focal: f, Copy: c.Copy, Volume: &v})
	}
	return p, s.CopyStyles, nil
}
func RetainedSources(p Project) ([]AnalysisSource, error) {
	var analyses []SourceAnalysis
	if err := strictJSON(p.Analysis, &analyses); err != nil {
		return nil, err
	}
	sources := make([]AnalysisSource, 0, len(analyses))
	for _, a := range analyses {
		sources = append(sources, a.Source)
	}
	return sources, nil
}
func EditingState(p Project, cfg RenderConfig) (*CorrectionState, error) {
	if p.EditPlan == "" {
		return nil, nil
	}
	plan, styles, err := DecodeEditPlan(p.EditPlan)
	if err != nil {
		return nil, err
	}
	sources, err := RetainedSources(p)
	if err != nil {
		return nil, err
	}
	return &CorrectionState{CorrectionFromPlan(plan), sources, styles, cfg.FadeMS, cfg.MaxCuts, cfg.MaxCopyRunes, cfg.MinDurationMS, cfg.MaxDurationMS}, nil
}
func ApplyCorrection(cfg RenderConfig, p Project, input CorrectionPlan) (EditPlan, []string, error) {
	old, styles, err := DecodeEditPlan(p.EditPlan)
	if err != nil {
		return EditPlan{}, nil, err
	}
	sources, err := RetainedSources(p)
	if err != nil {
		return EditPlan{}, nil, err
	}
	known := map[string]Cut{}
	for _, c := range old.Cuts {
		known[c.ID] = c
	}
	next := EditPlan{Ratio: p.Ratio, DurationMS: input.DurationMS}
	for _, c := range input.Cuts {
		prior, ok := known[c.ID]
		if !ok || c.SourceID != prior.SourceID || c.Fingerprint != prior.Fingerprint || c.VolumePermille < 0 || c.VolumePermille > 1000 || !slices.Contains(styles, c.Copy.Style) {
			return EditPlan{}, nil, ErrInvalid
		}
		v := float64(c.VolumePermille) / 1000
		prior.StartMS, prior.EndMS, prior.Copy, prior.Volume = c.StartMS, c.EndMS, c.Copy, &v
		next.Cuts = append(next.Cuts, prior)
	}
	refs := make([]RenderSource, 0, len(sources))
	for _, s := range sources {
		refs = append(refs, s.RenderSource)
	}
	if err = ValidateEditPlan(cfg, next, refs); err != nil {
		return EditPlan{}, nil, err
	}
	return next, styles, nil
}

// Exactly the remaining source subset, independent of new transient lease ids.
func MatchRenderBatch(plan EditPlan, batch SourceBatch) error {
	required := map[string]bool{}
	for _, c := range plan.Cuts {
		required[c.Fingerprint] = true
	}
	if len(required) != len(batch.Sources) {
		return ErrSourceState
	}
	for _, s := range batch.Sources {
		if !required[s.Fingerprint] {
			return ErrSourceState
		}
		delete(required, s.Fingerprint)
	}
	return nil
}
