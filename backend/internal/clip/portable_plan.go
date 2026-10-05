package clip

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/postpilot/backend/internal/clip/composition"
)

// Version 5 retains correction geometry beside portable content. Keeping the
// previous envelope separate preserves exact strict readers for versions 1–4.
type storedPortablePlan struct {
	Version     int
	Ratio       string
	Plan        storedCorrectionPlan
	Focals      map[string]Point
	CopyStyles  []string
	Composition PortablePlan
}

// Version 6 is the assembly envelope every new plan is written in. It keeps
// version 5's fields and adds the two the assembly contract made explicit: each
// cut's fixed playback rate and the complete owner-owned source-audio snapshot.
// The rates ride their own map beside the focals, exactly so the version-5
// reader's structs stay byte-compatible and keep reading what they wrote.
//
// The composition is a POINTER here, unlike version 5's value: a project old
// enough to have no composition at all still gets a version-6 plan rather than
// a sixth shape nobody reads.
type storedAssemblyPlan struct {
	Notices            []PlanNotice
	NoticeCutRevisions map[string]int
	Version            int
	Ratio              string
	Plan               storedCorrectionPlan
	Focals             map[string]Point
	Composition        *PortablePlan
	Rates              map[string]int
	SourceAudio        []storedSourceAudio
}
type storedSourceAudio struct {
	SourceID, Fingerprint string
	RetainOriginal        bool
}

func encodeAssemblyPlan(p EditPlan) (string, error) { return encodeSpokenPlan(p) }
func encodeVersionSixPlan(p EditPlan) (string, error) {
	if p.Portable != nil {
		if err := validatePortablePlan(p); err != nil {
			return "", err
		}
	}
	focals := map[string]Point{}
	rates := map[string]int{}
	for _, cut := range p.Cuts {
		focals[cut.ID] = cut.Focal
		rates[cut.ID] = cut.Rate()
	}
	plain := p
	plain.Portable = nil
	audio := []storedSourceAudio{}
	settings := p.SourceAudio
	if settings == nil {
		// Nothing has authorized any audio. A legacy plan reaches here with its
		// snapshot already derived on decode (CLIP-101); a plan built today has
		// simply not been given one, and CLIP-18's default is off.
		settings = ReconcileSourceAudio(nil, p.Cuts)
	}
	for _, v := range settings.Values {
		audio = append(audio, storedSourceAudio{v.SourceID, v.Fingerprint, v.RetainOriginal})
	}
	envelope := storedAssemblyPlan{Version: assemblyPlanVersion, Ratio: p.Ratio, Plan: storedCorrection(CorrectionFromPlan(plain)), Focals: focals, Composition: p.Portable, Rates: rates, SourceAudio: audio, Notices: p.Notices, NoticeCutRevisions: p.NoticeCutRevisions}
	b, err := json.Marshal(envelope)
	return string(b), err
}

// decodeAssemblyPlan reads the version-6 envelope. The version-5 and earlier
// readers stay untouched, so a plan written before rates existed is still read
// by the exact code that wrote it.
func decodeAssemblyPlan(raw string) (EditPlan, error) {
	var legacy map[string]json.RawMessage
	if json.Unmarshal([]byte(raw), &legacy) != nil {
		return EditPlan{}, ErrInvalid
	}
	delete(legacy, "CopyStyles")
	delete(legacy, "Styles")
	clean, err := json.Marshal(legacy)
	if err != nil {
		return EditPlan{}, ErrInvalid
	}
	raw = string(clean)
	var s storedAssemblyPlan
	if StrictJSON(raw, &s) != nil || s.Version != assemblyPlanVersion {
		return EditPlan{}, ErrInvalid
	}
	p, err := portableFromStored(storedPortablePlan{s.Version, s.Ratio, s.Plan, s.Focals, nil, PortablePlan{}})
	if err != nil {
		return p, err
	}
	p.Portable = s.Composition
	p.Notices, p.NoticeCutRevisions = s.Notices, s.NoticeCutRevisions
	// A version-6 plan states every rate explicitly. An absent or zero entry is
	// not a 1x default here: this envelope was written after rates existed.
	if len(s.Rates) != len(p.Cuts) {
		return EditPlan{}, ErrInvalid
	}
	for i := range p.Cuts {
		rate, ok := s.Rates[p.Cuts[i].ID]
		if !ok || !ValidPlaybackRate(rate) {
			return EditPlan{}, ErrInvalid
		}
		p.Cuts[i].PlaybackRatePermille = rate
	}
	settings := &SourceAudioSettings{}
	for _, v := range s.SourceAudio {
		settings.Values = append(settings.Values, SourceAudioSetting{v.SourceID, v.Fingerprint, v.RetainOriginal})
	}
	p.SourceAudio = settings
	if err := ValidateSourceAudioSettings(p); err != nil {
		return EditPlan{}, err
	}
	if p.Portable == nil {
		return p, nil
	}
	return p, validatePortablePlan(p)
}

func decodePortablePlan(raw string) (EditPlan, error) {
	var s storedPortablePlan
	if StrictJSON(raw, &s) != nil || s.Version != portablePlanVersion {
		return EditPlan{}, ErrInvalid
	}
	p, err := portableFromStored(s)
	if err != nil {
		return p, err
	}
	p.Portable = &s.Composition
	// Nothing in a version-5 plan ever chose a rate or a source-audio setting,
	// so it reads at 1x with its original audio meaning made explicit.
	upgradeLegacyAssembly(&p)
	return p, validatePortablePlan(p)
}

func portableFromStored(s storedPortablePlan) (EditPlan, error) {
	// Reuse existing correction geometry validation without widening its reader.
	b, err := json.Marshal(storedEditPlan{storedPlanVersion, s.Ratio, s.Plan, s.Focals, s.CopyStyles})
	if err != nil {
		return EditPlan{}, err
	}
	return DecodeEditPlan(string(b))
}

// upgradeLegacyAssembly is the ONE compatibility path that reads an absent rate
// as 1x and derives the audio snapshot from saved per-cut volume (CLIP-101).
func upgradeLegacyAssembly(p *EditPlan) {
	for i := range p.Cuts {
		p.Cuts[i].PlaybackRatePermille = RateUnitPermille
	}
	if p.Portable != nil {
		for i := range p.Portable.Cuts {
			p.Portable.Cuts[i].PlaybackRatePermille = RateUnitPermille
		}
		for i := range p.Portable.RetiredCuts {
			p.Portable.RetiredCuts[i].PlaybackRatePermille = RateUnitPermille
		}
		for i := range p.Portable.RetiredBindings {
			p.Portable.RetiredBindings[i].PlaybackRatePermille = RateUnitPermille
		}
	}
	p.SourceAudio = LegacySourceAudio(p.Cuts)
}

func validatePortablePlan(p EditPlan) error {
	v := p.Portable
	if v == nil || v.Snapshot.Version != CompositionVersion || strings.TrimSpace(v.Snapshot.Body) == "" || len(v.Cuts) != len(p.Cuts) {
		return ErrInvalid
	}
	cuts := map[string]bool{}
	for i, c := range v.Cuts {
		old := p.Cuts[i]
		if c.ID == "" || cuts[c.ID] || c.ID != old.ID || c.SourceID != old.SourceID || c.StartMS != old.StartMS || c.EndMS != old.EndMS || c.TransitionMS != old.TransitionMS || c.Rate() != old.Rate() {
			return ErrInvalid
		}
		cuts[c.ID] = true
	}
	ids := map[string]bool{}
	for _, text := range v.Elements {
		r := text.Resolved
		if r.InstanceID == "" || ids[r.InstanceID] || r.Element.ID == "" || r.StartMS < 0 || r.EndMS <= r.StartMS || r.EndMS > p.DurationMS || (r.Element.Basis == "cut" && r.CutID != "" && !cuts[r.CutID]) {
			return ErrInvalid
		}
		if r.Element.Kind != "fixed" && r.Element.Kind != "ai" {
			return ErrInvalid
		}
		// A narration caption is admitted on its own shape alone: the snapshot
		// declares the fixed regions and nothing else, so nothing here may ask
		// the document whether this caption exists (CLIP-134).
		if text.Scope == NarrationScope && (!ValidNarrationShape(text) || r.StartMS != *r.Element.StartMS || r.EndMS != *r.Element.EndMS) {
			return ErrInvalid
		}
		ids[r.InstanceID] = true
		for _, evidence := range text.Evidence {
			valid := false
			for _, c := range p.Cuts {
				if c.SourceID == evidence.SourceID && c.Fingerprint == evidence.Fingerprint && evidence.StartMS >= c.StartMS && evidence.EndMS <= c.EndMS && evidence.StartMS < evidence.EndMS {
					valid = true
				}
			}
			// Trimming changes the selected footage, not its original evidence.
			// Frozen observations keep the owner/source boundary verifiable even
			// when an earlier observed range is no longer in a selected cut.
			for _, observed := range v.Observations {
				source := observed.Source
				if source.ID == evidence.SourceID && source.Fingerprint == evidence.Fingerprint && evidence.StartMS >= 0 && evidence.StartMS < evidence.EndMS && evidence.EndMS <= source.Info.DurationMS {
					valid = true
				}
			}
			if !valid {
				return ErrInvalid
			}
		}
	}
	return ValidateNarrationIntervals(v.Elements, p.DurationMS)
}

// ValidNarrationShape reports whether a caption carries the one shape narration
// takes: a server-minted identity, no cut, and an absolute output interval.
func ValidNarrationShape(text PortableText) bool {
	r := text.Resolved
	e := r.Element
	return text.Scope == NarrationScope && r.CutID == "" && r.GroupID == "" && r.ItemID == "" &&
		r.InstanceID == e.ID && narrationInstanceID.MatchString(e.ID) &&
		e.Kind == "ai" && e.Role == "caption" && e.Basis == "output-start" && e.StartMS != nil && e.EndMS != nil
}

// ValidateNarrationIntervals holds the narration to CLIP-66's two rules against
// the output it will be rendered onto: every interval lies inside that output
// and no two of them overlap. Both refusals name the caption's own instance,
// because a caption the edited output no longer holds is corrected by the owner
// rather than moved, retimed or dropped for them (CLIP-67).
func ValidateNarrationIntervals(elements []PortableText, durationMS int) error {
	type window struct {
		id         string
		start, end int
	}
	var windows []window
	for _, text := range elements {
		if text.Scope != NarrationScope {
			continue
		}
		if !ValidNarrationShape(text) {
			return ErrInvalid
		}
		// The DECLARED interval, not the resolved one: the declaration is what
		// every later resolution reads, so it is what has to fit the output.
		e := text.Resolved.Element
		if *e.StartMS < 0 || *e.EndMS <= *e.StartMS || *e.EndMS > durationMS {
			return narrationRefusal(e.ID, NoticeCaptionOutsideOutput)
		}
		windows = append(windows, window{e.ID, *e.StartMS, *e.EndMS})
	}
	slices.SortStableFunc(windows, func(a, b window) int { return a.start - b.start })
	for i := 1; i < len(windows); i++ {
		if windows[i].start < windows[i-1].end {
			return narrationRefusal(windows[i].id, NoticeCaptionOverlap)
		}
	}
	return nil
}

func narrationRefusal(id, reason string) error {
	return &composition.Problem{ElementID: id, Line: 1, Reason: reason}
}
