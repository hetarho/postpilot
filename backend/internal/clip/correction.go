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
	TransitionMS int
	// One copy, or CDS-43's two in sequence. The owner may add and remove the
	// second one on step ②, which is why it is a list here too.
	Copies         []Caption
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
// read back as — its stored duration was computed from it. Version 3 carries a
// LIST of copies (CDS-43); everything before it wrote exactly one, which is what
// it is read back as.
const storedPlanVersion = 4

// The cut every stored plan before version 3 wrote: one `Copy` where there is
// now a list. It is a separate type rather than a token rewrite because a
// caption's own text may contain a brace and a rewrite could not tell them apart.
type legacyCorrectionCut struct {
	ID, SourceID, Fingerprint string
	StartMS, EndMS            int
	TransitionMS              int
	Copy                      Caption
	Chips                     []string
	VolumePermille            int
}

// The whole plan T076 stored, whose cuts also carried one copy each.
type legacyEditPlan struct {
	Ratio       string
	DurationMS  int
	Cuts        []legacyEditCut
	Disclosure  string
	Facts       []Answer
	Preset      string
	Hook        string
	CTA, Accent string
	Written     []Written
	Decisions   []Composition
}
type legacyEditCut struct {
	ID, SourceID, Fingerprint string
	StartMS, EndMS            int
	TransitionMS              int
	Focal                     Point
	Copy                      Copy
	Chips                     []string
	Volume                    *float64
}

func (p legacyEditPlan) upgrade() EditPlan {
	out := EditPlan{Ratio: p.Ratio, DurationMS: p.DurationMS, Disclosure: p.Disclosure, Facts: p.Facts,
		Preset: p.Preset, Hook: p.Hook, CTA: p.CTA, Accent: p.Accent, Written: p.Written, Decisions: p.Decisions}
	for _, c := range p.Cuts {
		out.Cuts = append(out.Cuts, Cut{ID: c.ID, SourceID: c.SourceID, Fingerprint: c.Fingerprint,
			StartMS: c.StartMS, EndMS: c.EndMS, TransitionMS: c.TransitionMS, Focal: c.Focal,
			Copies: []Copy{c.Copy}, Chips: c.Chips, Volume: c.Volume})
	}
	return out
}

type legacyCorrectionPlan struct {
	DurationMS int
	Cuts       []legacyCorrectionCut
	Hook       string
}
type legacyStoredEditPlan struct {
	Version    int
	Ratio      string
	Plan       legacyCorrectionPlan
	Focals     map[string]Point
	CopyStyles []string
}

func (p legacyCorrectionPlan) upgrade() CorrectionPlan {
	out := CorrectionPlan{DurationMS: p.DurationMS, Hook: p.Hook}
	for _, c := range p.Cuts {
		out.Cuts = append(out.Cuts, CorrectionCut{ID: c.ID, SourceID: c.SourceID, Fingerprint: c.Fingerprint,
			StartMS: c.StartMS, EndMS: c.EndMS, TransitionMS: c.TransitionMS,
			Copies: []Caption{c.Copy}, Chips: c.Chips, VolumePermille: c.VolumePermille})
	}
	return out
}

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
		out.Cuts = append(out.Cuts, CorrectionCut{c.ID, c.SourceID, c.Fingerprint, c.StartMS, c.EndMS, c.TransitionMS, slices.Clone(c.Copies), slices.Clone(c.Chips), int(math.Round(c.OriginalVolume() * 1000))})
	}
	return out
}
func EncodeEditPlan(p EditPlan, styles []string) (string, error) {
	if p.Portable != nil {
		return encodePortablePlan(p, styles)
	}
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
	var marker struct{ Version int }
	if json.Unmarshal([]byte(raw), &marker) != nil {
		return EditPlan{}, nil, ErrInvalid
	}
	if marker.Version == CompositionPlanVersion {
		return decodePortablePlan(raw)
	}
	raw = migrateStoredPlan(raw)
	if marker.Version == 0 {
		// T076's original representation. Do not invent template permissions lost in
		// that format: only styles already present in the retained plan are approved.
		var legacy legacyEditPlan
		if err := strictJSON(raw, &legacy); err != nil {
			return EditPlan{}, nil, err
		}
		p := legacy.upgrade()
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
			for _, copy := range c.Copies {
				if !slices.Contains(styles, copy.Style) {
					styles = append(styles, copy.Style)
				}
			}
		}
		return p, styles, nil
	}
	var s storedEditPlan
	if marker.Version < 3 {
		var legacy legacyStoredEditPlan
		if err := strictJSON(raw, &legacy); err != nil {
			return EditPlan{}, nil, ErrInvalid
		}
		s = storedEditPlan{legacy.Version, legacy.Ratio, legacy.Plan.upgrade(), legacy.Focals, legacy.CopyStyles}
	} else if err := strictJSON(raw, &s); err != nil {
		return EditPlan{}, nil, ErrInvalid
	}
	if s.Version < 1 || s.Version > storedPlanVersion {
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
		p.Cuts = append(p.Cuts, Cut{ID: c.ID, SourceID: c.SourceID, Fingerprint: c.Fingerprint, StartMS: c.StartMS, EndMS: c.EndMS, TransitionMS: c.TransitionMS, Focal: f, Copies: c.Copies, Chips: c.Chips, Volume: &v})
	}
	return p, s.CopyStyles, nil
}

// RetainedObservations reads the same recorded evidence used by correction.
// Old projects without analysis have no evidence, rather than a decode failure.
func RetainedObservations(p Project) ([]SourceAnalysis, error) {
	if strings.TrimSpace(p.Analysis) == "" {
		return nil, nil
	}
	var analyses []SourceAnalysis
	if err := strictJSON(p.Analysis, &analyses); err != nil {
		return nil, err
	}
	return analyses, nil
}
func RetainedSources(p Project) ([]AnalysisSource, error) {
	analyses, err := RetainedObservations(p)
	if err != nil {
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
		if !ok || c.SourceID != prior.SourceID || c.Fingerprint != prior.Fingerprint || c.VolumePermille < 0 || c.VolumePermille > 1000 {
			return EditPlan{}, nil, ErrInvalid
		}
		for _, copy := range c.Copies {
			if !slices.Contains(styles, copy.Style) {
				return EditPlan{}, nil, ErrInvalid
			}
		}
		v := float64(c.VolumePermille) / 1000
		prior.StartMS, prior.EndMS, prior.TransitionMS, prior.Copies, prior.Chips, prior.Volume = c.StartMS, c.EndMS, c.TransitionMS, c.Copies, c.Chips, &v
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
