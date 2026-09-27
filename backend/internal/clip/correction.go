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
// Source identity remains frozen. Native drafts may supply a normalized focal
// point; legacy corrections retain their saved framing.
type CorrectionCut struct {
	ID, SourceID, Fingerprint string
	StartMS, EndMS            int
	// The transition INTO this cut (CDS-36). The owner may change it on step ②,
	// which is why it rides the editable representation and the stored plan.
	TransitionMS int
	// One copy, or CDS-43's two in sequence. The owner may add and remove the
	// second one on step ②, which is why it is a list here too.
	Copies         []Caption
	VolumePermille int
	Focal          *Point
	// The cut's ONE fixed playback rate, as permille (CLIP-98). Zero is a draft
	// from a client that predates rates and reads as 1x; an explicit value must
	// be a supported rate.
	PlaybackRatePermille int
	// Request-only provenance for a cut the plan does not yet contain. It is
	// never stored and never projected back: a cut is created exactly once.
	Creation *CutCreation
}
type CorrectionPlan struct {
	// The owner's per-source original-sound snapshot, as a read projection. It
	// is changed through its own owner-scoped action, never by saving a plan, so
	// a draft that carries it back must carry it back unchanged (CLIP-100).
	SourceAudio       []SourceAudioSetting
	Associations      *[]SourceAssociation
	NativeComposition bool
	Elements          []CorrectionText
	DurationMS        int
	Cuts              []CorrectionCut
}
type CorrectionState struct {
	Plan                                                        CorrectionPlan
	Sources                                                     []AnalysisSource
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
	VolumePermille            int
}

type legacyCorrectionPlan struct {
	DurationMS int
	Cuts       []legacyCorrectionCut
}
type legacyStoredEditPlan struct {
	Version    int
	Ratio      string
	Plan       legacyCorrectionPlan
	Focals     map[string]Point
	CopyStyles []string
}

func (p legacyCorrectionPlan) upgrade() CorrectionPlan {
	out := CorrectionPlan{DurationMS: p.DurationMS}
	for _, c := range p.Cuts {
		out.Cuts = append(out.Cuts, CorrectionCut{ID: c.ID, SourceID: c.SourceID, Fingerprint: c.Fingerprint,
			StartMS: c.StartMS, EndMS: c.EndMS, TransitionMS: c.TransitionMS,
			Copies: []Caption{c.Copy}, VolumePermille: c.VolumePermille})
	}
	return out
}

// Versions 3–4 keep their original strict JSON shape. Editing-only fields
// (portable text projections and focal controls) never widen that envelope.
type storedCorrectionCut struct {
	ID, SourceID, Fingerprint string
	StartMS, EndMS            int
	TransitionMS              int
	Copies                    []Caption
	VolumePermille            int
}
type storedCorrectionPlan struct {
	DurationMS int
	Cuts       []storedCorrectionCut
}

func storedCorrection(p CorrectionPlan) storedCorrectionPlan {
	out := storedCorrectionPlan{DurationMS: p.DurationMS}
	for _, c := range p.Cuts {
		out.Cuts = append(out.Cuts, storedCorrectionCut{c.ID, c.SourceID, c.Fingerprint, c.StartMS, c.EndMS, c.TransitionMS, c.Copies, c.VolumePermille})
	}
	return out
}

type storedEditPlan struct {
	Version    int
	Ratio      string
	Plan       storedCorrectionPlan
	Focals     map[string]Point
	CopyStyles []string
}

func CorrectionFromPlan(p EditPlan) CorrectionPlan {
	out := CorrectionPlan{DurationMS: p.DurationMS, Cuts: make([]CorrectionCut, 0, len(p.Cuts))}
	for _, c := range p.Cuts {
		out.Cuts = append(out.Cuts, CorrectionCut{ID: c.ID, SourceID: c.SourceID, Fingerprint: c.Fingerprint, StartMS: c.StartMS, EndMS: c.EndMS, TransitionMS: c.TransitionMS, Copies: slices.Clone(c.Copies), VolumePermille: int(math.Round(c.OriginalVolume() * 1000)), PlaybackRatePermille: c.Rate()})
	}
	for i, c := range p.Cuts {
		focal := c.Focal
		out.Cuts[i].Focal = &focal
	}
	if p.SourceAudio != nil {
		out.SourceAudio = slices.Clone(p.SourceAudio.Values)
	}
	if p.Portable != nil {
		out.NativeComposition = true
		associations := slices.Clone(p.Portable.Inputs.Associations)
		out.Associations = &associations
		for _, t := range p.Portable.Elements {
			out.Elements = append(out.Elements, correctionText(t))
		}
	}
	return out
}

// Every plan this build writes is a version-6 assembly envelope, composition or
// not. The version-4 writer below it is gone; its READER stays exactly as it was.
func EncodeEditPlan(p EditPlan) (string, error) {
	return encodeAssemblyPlan(p)
}
func StrictJSON(raw string, out any) error {
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
func DecodeEditPlan(raw string) (EditPlan, error) {
	var marker struct{ Version int }
	if json.Unmarshal([]byte(raw), &marker) != nil {
		return EditPlan{}, ErrInvalid
	}
	// Dispatch on the EXACT stored version. Version 5 is a portable envelope in
	// its own right and version 6 is the current one, so a numeric comparison
	// against CompositionPlanVersion would read one of them with the wrong reader.
	switch marker.Version {
	case CompositionPlanVersion:
		return decodeAssemblyPlan(raw)
	case portablePlanVersion:
		return decodePortablePlan(raw)
	}
	raw = migrateStoredPlan(raw)
	var s storedEditPlan
	if marker.Version < 3 {
		var legacy legacyStoredEditPlan
		if err := StrictJSON(raw, &legacy); err != nil {
			return EditPlan{}, ErrInvalid
		}
		s = storedEditPlan{legacy.Version, legacy.Ratio, storedCorrection(legacy.Plan.upgrade()), legacy.Focals, legacy.CopyStyles}
	} else if err := StrictJSON(raw, &s); err != nil {
		return EditPlan{}, ErrInvalid
	}
	if s.Version < 1 || s.Version > storedPlanVersion {
		return EditPlan{}, ErrInvalid
	}
	if s.Version < 2 {
		for i := range s.Plan.Cuts {
			if i > 0 {
				s.Plan.Cuts[i].TransitionMS = design.Transition.FadeMS
			}
		}
	}
	if len(s.Plan.Cuts) == 0 || s.Plan.DurationMS <= 0 {
		return EditPlan{}, ErrInvalid
	}
	if _, err := ClipCanvas(s.Ratio); err != nil {
		return EditPlan{}, err
	}
	p := EditPlan{Ratio: s.Ratio, DurationMS: s.Plan.DurationMS}
	for _, c := range s.Plan.Cuts {
		f, ok := s.Focals[c.ID]
		if !ok || c.VolumePermille < 0 || c.VolumePermille > 1000 {
			return EditPlan{}, ErrInvalid
		}
		v := float64(c.VolumePermille) / 1000
		p.Cuts = append(p.Cuts, Cut{ID: c.ID, SourceID: c.SourceID, Fingerprint: c.Fingerprint, StartMS: c.StartMS, EndMS: c.EndMS, TransitionMS: c.TransitionMS, Focal: f, Copies: c.Copies, Volume: &v})
	}
	upgradeLegacyAssembly(&p)
	return p, nil
}

// RetainedObservations reads the same recorded evidence used by correction.
// Old projects without analysis have no evidence, rather than a decode failure.
func RetainedObservations(p Project) ([]SourceAnalysis, error) {
	if strings.TrimSpace(p.Analysis) == "" {
		return nil, nil
	}
	var analyses []SourceAnalysis
	if err := StrictJSON(p.Analysis, &analyses); err != nil {
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
	plan, err := DecodeEditPlan(p.EditPlan)
	if err != nil {
		return nil, err
	}
	sources, err := RetainedSources(p)
	if err != nil {
		return nil, err
	}
	return &CorrectionState{CorrectionFromPlan(plan), sources, cfg.FadeMS, cfg.MaxCuts, cfg.MaxCopyRunes, cfg.MinDurationMS, cfg.MaxDurationMS}, nil
}
func ApplyCorrection(cfg RenderConfig, p Project, input CorrectionPlan) (EditPlan, error) {
	// Reject an unsupported explicit rate before interval resolution turns its
	// zero transformed length into an unrelated duration error (CLIP-99).
	for _, cut := range input.Cuts {
		if !ValidPlaybackRate(cut.Rate()) {
			return EditPlan{}, cutRefusal(cut.ID, "plan_cut_rate")
		}
	}
	old, err := DecodeEditPlan(p.EditPlan)
	if err != nil {
		return EditPlan{}, err
	}
	sources, err := RetainedSources(p)
	if err != nil {
		return EditPlan{}, err
	}
	if old.Portable != nil {
		return applyNativeCorrection(cfg, p, old, sources, input)
	}
	if input.NativeComposition || len(input.Elements) != 0 {
		return EditPlan{}, ErrInvalid
	}
	known := map[string]Cut{}
	for _, c := range old.Cuts {
		known[c.ID] = c
	}
	next := EditPlan{Ratio: p.Ratio, DurationMS: input.DurationMS, SourceAudio: old.SourceAudio}
	if err := matchOwnerAudio(old, input); err != nil {
		return EditPlan{}, err
	}
	for _, c := range input.Cuts {
		prior, ok := known[c.ID]
		// Cut creation needs frozen observations and a template binding to check
		// against; a plan that predates the composition has neither, so it
		// remains a trim-and-reorder editor.
		if c.Creation != nil {
			return EditPlan{}, ErrCompositionUnavailable
		}
		if !ok || c.SourceID != prior.SourceID || c.Fingerprint != prior.Fingerprint || c.VolumePermille < 0 || c.VolumePermille > 1000 {
			return EditPlan{}, ErrInvalid
		}
		v := float64(c.VolumePermille) / 1000
		prior.StartMS, prior.EndMS, prior.TransitionMS, prior.Copies, prior.Volume = c.StartMS, c.EndMS, c.TransitionMS, c.Copies, &v
		// Per-cut volume is a gain, never a permission: it cannot turn a source's
		// original sound on, and the rate cannot change it either (CLIP-18).
		prior.PlaybackRatePermille = c.Rate()
		next.Cuts = append(next.Cuts, prior)
	}
	// A reorder carries each cut's own transition with it (CDS-36), so whichever
	// cut the owner moved to the front leads in from nothing.
	if len(next.Cuts) > 0 {
		next.Cuts[0].TransitionMS = 0
	}
	// Deleting or restoring a cut changes which sources the plan draws on, so
	// the snapshot is rebuilt around the new set with every existing owner
	// choice kept exactly as it was.
	next.SourceAudio = ReconcileSourceAudio(old.SourceAudio, next.Cuts)
	refs := make([]RenderSource, 0, len(sources))
	for _, s := range sources {
		refs = append(refs, s.RenderSource)
	}
	if err = ValidateEditPlan(cfg, next, refs); err != nil {
		return EditPlan{}, err
	}
	if err = ValidateSourceRanges(next, SourceOverlaps(old.Cuts)); err != nil {
		return EditPlan{}, err
	}
	trackNoticeCutEdits(old, &next)
	return next, nil
}

// matchOwnerAudio keeps the per-source original-sound snapshot out of the plan
// save. The editing projection carries it so a draft can show it, so a draft
// may hand it back — but only as the same answer. Every source the draft and the
// saved plan both name must agree; stating anything else is a contradiction, not
// an edit, and is refused (CLIP-100).
//
// A source only one side names is NOT a contradiction: deleting a cut, undoing
// that deletion or adding footage legitimately changes which sources the plan
// draws on, and the server rebuilds the complete snapshot itself afterwards.
func matchOwnerAudio(old EditPlan, in CorrectionPlan) error {
	if len(in.SourceAudio) == 0 || old.SourceAudio == nil {
		return nil
	}
	seen := map[sourceKey]bool{}
	for _, v := range in.SourceAudio {
		key := sourceKey{v.SourceID, v.Fingerprint}
		if seen[key] {
			return planViolation("plan_source_audio")
		}
		seen[key] = true
		for _, saved := range old.SourceAudio.Values {
			if saved.SourceID == v.SourceID && saved.Fingerprint == v.Fingerprint && saved.RetainOriginal != v.RetainOriginal {
				return planViolation("plan_source_audio")
			}
		}
	}
	return nil
}

// Every remaining source must match. A retained manifest may also hold unused originals.
func MatchRenderBatch(plan EditPlan, batch SourceBatch) error {
	required := map[string]bool{}
	for _, c := range plan.Cuts {
		required[c.Fingerprint] = true
	}
	seen := map[string]bool{}
	for _, s := range batch.Sources {
		if seen[s.Fingerprint] {
			return ErrSourceState
		}
		seen[s.Fingerprint] = true
		delete(required, s.Fingerprint)
	}
	if len(required) != 0 {
		return ErrSourceState
	}
	return nil
}
