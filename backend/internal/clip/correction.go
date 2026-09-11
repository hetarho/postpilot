package clip

import (
	"encoding/json"
	"errors"
	"io"
	"math"
	"slices"
	"strings"

	"github.com/postpilot/backend/internal/clip/design"
)

var ErrPlanConflict = errors.New("clip edit plan revision conflict")

// The editable representation uses integer thousandths, including its durable JSON.
// Framing is deliberately not editable: source identity and focal points stay frozen.
type CorrectionCut struct {
	ID, SourceID, Fingerprint string
	StartMS, EndMS            int
	// The transition INTO this cut (CDS-36). The owner may change it on step ②,
	// which is why it rides the editable representation and the stored plan.
	TransitionMS   int
	Copy           Caption
	Chips          []string
	VolumePermille int
}
type CorrectionPlan struct {
	DurationMS int
	Cuts       []CorrectionCut
	// The opening card's sentence (CDS-28). Part of the approved composition, so
	// unlike the disclosure and the facts it IS stored with the plan.
	Hook string
}
type CorrectionState struct {
	Plan                                                        CorrectionPlan
	Sources                                                     []AnalysisSource
	CopyStyles                                                  []string
	FadeMS, MaxCuts, MaxCopyRunes, MinDurationMS, MaxDurationMS int
}

// Version 2 carries each cut's own transition. A version-1 plan predates CDS-36
// and was rendered with a fade at every boundary, so that is exactly what it is
// read back as — its stored duration was computed from it.
const storedPlanVersion = 2

type storedEditPlan struct {
	Version    int
	Ratio      string
	Plan       CorrectionPlan
	Focals     map[string]Point
	CopyStyles []string
}

func CorrectionFromPlan(p EditPlan) CorrectionPlan {
	out := CorrectionPlan{DurationMS: p.DurationMS, Hook: p.Hook, Cuts: make([]CorrectionCut, 0, len(p.Cuts))}
	for _, c := range p.Cuts {
		out.Cuts = append(out.Cuts, CorrectionCut{c.ID, c.SourceID, c.Fingerprint, c.StartMS, c.EndMS, c.TransitionMS, c.Copy, slices.Clone(c.Chips), int(math.Round(c.OriginalVolume() * 1000))})
	}
	return out
}
func EncodeEditPlan(p EditPlan, styles []string) (string, error) {
	focals := map[string]Point{}
	for _, c := range p.Cuts {
		focals[c.ID] = c.Focal
	}
	b, err := json.Marshal(storedEditPlan{storedPlanVersion, p.Ratio, CorrectionFromPlan(p), focals, slices.Clone(styles)})
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

// The exact tokens migration 0041 rewrites, applied again on read. `encoding/json`
// emits no whitespace and each token carries its own quotes and key, so a caption's
// own text can never match one: a `"` inside text is escaped as `\"`. This is a belt
// for the window between the binary swap and the migration — and for a plan written
// by the old binary in it — never the authority.
var legacyPlanTokens = [][2]string{
	{`"Style":"diary"`, `"Style":"memo"`},
	{`"Style":"emphasis"`, `"Style":"bold"`},
	// The plan freezes the template's approved set as a bare JSON array, so those
	// elements carry no key. Each token holds the structural character on BOTH
	// sides instead: a `[`, `,` or `]` beside a quote cannot occur inside an
	// encoded string, while a bare `"diary"` would also match a caption whose
	// text is the word diary.
	{`["diary",`, `["memo",`},
	{`,"diary",`, `,"memo",`},
	{`,"diary"]`, `,"memo"]`},
	{`["diary"]`, `["memo"]`},
	{`["emphasis",`, `["bold",`},
	{`,"emphasis",`, `,"bold",`},
	{`,"emphasis"]`, `,"bold"]`},
	{`["emphasis"]`, `["bold"]`},
	{`"Position":"center"`, `"Position":"lower_mid"`},
	// The key rename carries the alignment the old vocabulary had no field for.
	{`"Position":"`, `"Align":"center","Anchor":"`},
}

func migrateStoredPlan(raw string) string {
	for _, t := range legacyPlanTokens {
		raw = strings.ReplaceAll(raw, t[0], t[1])
	}
	return raw
}
func DecodeEditPlan(raw string) (EditPlan, []string, error) {
	raw = migrateStoredPlan(raw)
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
		// T076 predates CDS-36 too: every boundary was a fade, and the stored
		// duration only adds up if it is read back as one.
		for i := range p.Cuts {
			if i > 0 {
				p.Cuts[i].TransitionMS = design.Transition.FadeMS
			}
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
	if err := strictJSON(raw, &s); err != nil || s.Version < 1 || s.Version > storedPlanVersion {
		return EditPlan{}, nil, ErrInvalid
	}
	if s.Version < 2 {
		for i := range s.Plan.Cuts {
			if i > 0 {
				s.Plan.Cuts[i].TransitionMS = design.Transition.FadeMS
			}
		}
	}
	if len(s.Plan.Cuts) == 0 || s.Plan.DurationMS <= 0 {
		return EditPlan{}, nil, ErrInvalid
	}
	if _, err := ClipCanvas(s.Ratio); err != nil {
		return EditPlan{}, nil, err
	}
	p := EditPlan{Ratio: s.Ratio, DurationMS: s.Plan.DurationMS, Hook: s.Plan.Hook}
	for _, c := range s.Plan.Cuts {
		f, ok := s.Focals[c.ID]
		if !ok || c.VolumePermille < 0 || c.VolumePermille > 1000 {
			return EditPlan{}, nil, ErrInvalid
		}
		v := float64(c.VolumePermille) / 1000
		p.Cuts = append(p.Cuts, Cut{ID: c.ID, SourceID: c.SourceID, Fingerprint: c.Fingerprint, StartMS: c.StartMS, EndMS: c.EndMS, TransitionMS: c.TransitionMS, Focal: f, Copy: c.Copy, Chips: c.Chips, Volume: &v})
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
	next := EditPlan{Ratio: p.Ratio, DurationMS: input.DurationMS, Hook: strings.TrimSpace(input.Hook)}
	// The hook is the one text on a corrected plan the owner wrote for the clip
	// rather than for a cut, so it answers to CDS-42 here: it may only state
	// numbers and names the owner's own answers already carry.
	if !Grounded(next.Hook, p.Answers) {
		return EditPlan{}, nil, planViolation("plan_hook")
	}
	for _, c := range input.Cuts {
		prior, ok := known[c.ID]
		if !ok || c.SourceID != prior.SourceID || c.Fingerprint != prior.Fingerprint || c.VolumePermille < 0 || c.VolumePermille > 1000 || !slices.Contains(styles, c.Copy.Style) {
			return EditPlan{}, nil, ErrInvalid
		}
		v := float64(c.VolumePermille) / 1000
		prior.StartMS, prior.EndMS, prior.TransitionMS, prior.Copy, prior.Chips, prior.Volume = c.StartMS, c.EndMS, c.TransitionMS, c.Copy, c.Chips, &v
		next.Cuts = append(next.Cuts, prior)
	}
	// A reorder carries each cut's own transition with it (CDS-36), so whichever
	// cut the owner moved to the front leads in from nothing.
	if len(next.Cuts) > 0 {
		next.Cuts[0].TransitionMS = 0
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
