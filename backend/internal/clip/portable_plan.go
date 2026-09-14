package clip

import (
	"encoding/json"
	"fmt"
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
	Version     int
	Ratio       string
	Plan        storedCorrectionPlan
	Focals      map[string]Point
	CopyStyles  []string
	Composition *PortablePlan
	Rates       map[string]int
	SourceAudio []storedSourceAudio
}
type storedSourceAudio struct {
	SourceID, Fingerprint string
	RetainOriginal        bool
}

func encodeAssemblyPlan(p EditPlan, styles []string) (string, error) {
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
	envelope := storedAssemblyPlan{CompositionPlanVersion, p.Ratio, storedCorrection(CorrectionFromPlan(plain)), focals, slices.Clone(styles), p.Portable, rates, audio}
	b, err := json.Marshal(envelope)
	return string(b), err
}

// decodeAssemblyPlan reads the version-6 envelope. The version-5 and earlier
// readers stay untouched, so a plan written before rates existed is still read
// by the exact code that wrote it.
func decodeAssemblyPlan(raw string) (EditPlan, []string, error) {
	var s storedAssemblyPlan
	if strictJSON(raw, &s) != nil || s.Version != CompositionPlanVersion {
		return EditPlan{}, nil, ErrInvalid
	}
	p, styles, err := portableFromStored(storedPortablePlan{s.Version, s.Ratio, s.Plan, s.Focals, s.CopyStyles, PortablePlan{}})
	if err != nil {
		return p, styles, err
	}
	p.Portable = s.Composition
	// A version-6 plan states every rate explicitly. An absent or zero entry is
	// not a 1x default here: this envelope was written after rates existed.
	if len(s.Rates) != len(p.Cuts) {
		return EditPlan{}, nil, ErrInvalid
	}
	for i := range p.Cuts {
		rate, ok := s.Rates[p.Cuts[i].ID]
		if !ok || !ValidPlaybackRate(rate) {
			return EditPlan{}, nil, ErrInvalid
		}
		p.Cuts[i].PlaybackRatePermille = rate
	}
	settings := &SourceAudioSettings{}
	for _, v := range s.SourceAudio {
		settings.Values = append(settings.Values, SourceAudioSetting{v.SourceID, v.Fingerprint, v.RetainOriginal})
	}
	p.SourceAudio = settings
	if err := ValidateSourceAudioSettings(p); err != nil {
		return EditPlan{}, nil, err
	}
	if p.Portable == nil {
		return p, styles, nil
	}
	return p, styles, validatePortablePlan(p)
}

func decodePortablePlan(raw string) (EditPlan, []string, error) {
	var s storedPortablePlan
	if strictJSON(raw, &s) != nil || s.Version != portablePlanVersion {
		return EditPlan{}, nil, ErrInvalid
	}
	p, styles, err := portableFromStored(s)
	if err != nil {
		return p, styles, err
	}
	p.Portable = &s.Composition
	// Nothing in a version-5 plan ever chose a rate or a source-audio setting,
	// so it reads at 1x with its original audio meaning made explicit.
	upgradeLegacyAssembly(&p)
	return p, styles, validatePortablePlan(p)
}

func portableFromStored(s storedPortablePlan) (EditPlan, []string, error) {
	// Reuse existing correction geometry validation without widening its reader.
	b, err := json.Marshal(storedEditPlan{storedPlanVersion, s.Ratio, s.Plan, s.Focals, s.CopyStyles})
	if err != nil {
		return EditPlan{}, nil, err
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
	return nil
}

// LegacyPortablePlan is a read projection. No retained MP4, revision or stored
// plan changes until an accepted correction/generation explicitly writes it.
func LegacyPortablePlan(p Project, plan EditPlan, limits composition.Limits) (*PortablePlan, error) {
	if plan.Portable != nil {
		return plan.Portable, nil
	}
	c := p.Composition
	if c == nil {
		value := LegacyProjectComposition(p, Recipe{CopyStyles: []string{"clean"}})
		c = &value
	}
	out := &PortablePlan{Snapshot: c.Snapshot, Inputs: c.Inputs}
	if c.Snapshot.Legacy {
		limits = LegacyCompositionLimits(limits)
	}
	doc, problem := composition.Parse(c.Snapshot.Body, limits)
	if problem != nil {
		return nil, problem
	}
	for _, element := range doc.Elements {
		if element.Kind != "fixed" {
			continue
		}
		r := composition.ResolvedElement{InstanceID: element.ID, Element: element, StartMS: 0, EndMS: plan.DurationMS, AuthoredTiming: true}
		for _, part := range element.Parts {
			r.Text += part.Literal
		}
		for _, row := range element.Rows {
			text := ""
			for _, part := range row.Parts {
				text += part.Literal
			}
			r.Rows = append(r.Rows, composition.ResolvedRow{Role: row.Role, Text: text})
		}
		switch element.Basis {
		case "output-start":
			r.StartMS, r.EndMS = *element.StartMS, *element.EndMS
		case "output-end":
			r.StartMS, r.EndMS = plan.DurationMS+*element.StartMS, plan.DurationMS+*element.EndMS
		}
		out.Elements = append(out.Elements, PortableText{Resolved: r, Scope: "context", Accent: doc.Accent, Pace: doc.Pace})
	}
	values := map[string]string{}
	for _, a := range p.Answers {
		values[a.Label] = a.Text
	}
	legacyRecipe := Recipe{}
	if c.Snapshot.LegacyRecipe != nil {
		legacyRecipe = *c.Snapshot.LegacyRecipe
	}
	factsPlan := plan.WithFacts(p.Disclosure, p.Answers, legacyRecipe.Preset, p.CTA, legacyRecipe.Accent, p.HideDisclosure)
	offset := 0
	for _, cut := range plan.Cuts {
		offset -= cut.TransitionMS
		out.Cuts = append(out.Cuts, composition.Cut{ID: cut.ID, SectionID: "footage", SourceID: cut.SourceID, StartMS: cut.StartMS, EndMS: cut.EndMS, TransitionMS: cut.TransitionMS, PlaybackRatePermille: cut.Rate()})
		for index, copy := range cut.Copies {
			if copy.Text == "" {
				continue
			}
			start, end := cut.CaptionWindow(index)
			id := fmt.Sprintf("legacy-copy-%s-%d", cut.ID, index)
			e := composition.Element{ID: id, Kind: "fixed", Role: "caption", Style: copy.Style, Position: copy.Anchor, Align: copy.Align, Basis: "cut", StartMS: &start, EndMS: &end, Parts: []composition.Part{{Literal: copy.Text}}}
			if copy.StartMS == 0 && copy.EndMS == 0 {
				e.StartMS, e.EndMS = nil, nil
			}
			out.Elements = append(out.Elements, PortableText{Accent: copy.Accent, Keyword: copy.Keyword, Pace: copy.Pace, Resolved: composition.ResolvedElement{InstanceID: id, CutID: cut.ID, Element: e, Text: copy.Text, StartMS: offset + start, EndMS: offset + end, AuthoredTiming: copy.StartMS != 0 || copy.EndMS != 0}, Scope: "scene", Evidence: []SourceEvidence{{cut.SourceID, cut.Fingerprint, cut.StartMS, cut.EndMS}}})
		}
		for _, label := range factsPlan.ChipLabels(cut) {
			start, end := 0, cut.OutputDurationMS()
			id := "legacy-info-" + cut.ID + "-" + strings.TrimPrefix(LegacyFieldID(label), "field-")
			text := label + " " + values[label]
			element := composition.Element{ID: id, Kind: "fixed", Role: "info", Style: "auto", Position: "header", Align: "center", Basis: "cut", StartMS: &start, EndMS: &end, Parts: []composition.Part{{Literal: text}}}
			out.Elements = append(out.Elements, PortableText{Resolved: composition.ResolvedElement{InstanceID: id, CutID: cut.ID, Element: element, Text: text, StartMS: offset, EndMS: offset + end, AuthoredTiming: true, Facts: []composition.Fact{{FieldID: LegacyFieldID(label), Value: values[label]}}}, Scope: "scene", Accent: legacyRecipe.Accent, Evidence: []SourceEvidence{{cut.SourceID, cut.Fingerprint, cut.StartMS, cut.EndMS}}})
		}
		offset += cut.OutputDurationMS()
	}
	return out, nil
}

func FreezeLegacyPlan(p Project, plan EditPlan, recipe Recipe, limits composition.Limits) (*PortablePlan, error) {
	plain := plan
	plain.Portable = nil
	raw, err := EncodeEditPlan(plain, recipe.CopyStyles)
	if err != nil {
		return nil, err
	}
	p.EditPlan = raw
	c := LegacyProjectComposition(p, recipe)
	p.Composition = &c
	return LegacyPortablePlan(p, plain, limits)
}
