package ai

import (
	"encoding/json"
	"maps"
	"slices"
	"strings"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/clip/design"
)

type narrationCaptionJSON struct {
	// declaredID is the outline entry this caption is, empty for one the writer
	// chose to add; authored says its text is the template's own and is neither
	// rewritten nor ground checked (CLIP-65). Neither crosses the wire.
	declaredID   string
	authored     bool
	ID           string     `json:"id"`
	Text         string     `json:"text"`
	ShortText    string     `json:"short_text"`
	Keyword      string     `json:"keyword"`
	Style        string     `json:"style"`
	StartMS      int        `json:"start_ms"`
	EndMS        int        `json:"end_ms"`
	Observations []string   `json:"observation_refs"`
	Facts        []factJSON `json:"fact_refs"`
}
type narrationSlotJSON struct {
	ElementID    string     `json:"element_id"`
	Rows         []string   `json:"rows"`
	ShortRows    []string   `json:"short_rows"`
	Observations []string   `json:"observation_refs"`
	Facts        []factJSON `json:"fact_refs"`
}

// narrationDeclaredJSON places one caption the template's outline already
// carries: where it plays, and — for an `ai` entry alone — what it says.
type narrationDeclaredJSON struct {
	ElementID    string     `json:"element_id"`
	Text         string     `json:"text"`
	ShortText    string     `json:"short_text"`
	Keyword      string     `json:"keyword"`
	Style        string     `json:"style"`
	StartMS      int        `json:"start_ms"`
	EndMS        int        `json:"end_ms"`
	Observations []string   `json:"observation_refs"`
	Facts        []factJSON `json:"fact_refs"`
}
type narrationJSON struct {
	Captions []narrationCaptionJSON  `json:"captions"`
	Declared []narrationDeclaredJSON `json:"declared_captions"`
	Slots    []narrationSlotJSON     `json:"slots"`
	// The flow is final. A response that echoes it is admitted and ignored
	// rather than refused, so one stray key cannot cost a whole writing call.
	Cuts []json.RawMessage `json:"cuts"`
}

var narrationShape = readShape(narrationSchema)

// collectedFacts is every fact the project actually collected — global values
// and item values alike. A caption belongs to no item and may state any of them
// (CLIP-137), so this is the whole set a number is checked against.
func collectedFacts(inputs clip.CompositionInputs) []composition.Fact {
	var out []composition.Fact
	for _, id := range slices.Sorted(maps.Keys(inputs.Values)) {
		if strings.TrimSpace(inputs.Values[id]) != "" {
			out = append(out, composition.Fact{FieldID: id, Value: inputs.Values[id]})
		}
	}
	for _, group := range slices.Sorted(maps.Keys(inputs.Items)) {
		for _, item := range inputs.Items[group] {
			for _, field := range slices.Sorted(maps.Keys(item.Values)) {
				if strings.TrimSpace(item.Values[field]) != "" {
					out = append(out, composition.Fact{FieldID: field, GroupID: group, ItemID: item.ID, Value: item.Values[field]})
				}
			}
		}
	}
	return out
}

func citedFacts(refs []factJSON, collected []composition.Fact) ([]composition.Fact, bool) {
	var out []composition.Fact
	seen := map[factJSON]bool{}
	for _, ref := range refs {
		if seen[ref] {
			return nil, false
		}
		seen[ref] = true
		at := slices.IndexFunc(collected, func(f composition.Fact) bool {
			return f.FieldID == ref.FieldID && f.GroupID == ref.GroupID && f.ItemID == ref.ItemID
		})
		if at < 0 {
			return nil, false
		}
		out = append(out, collected[at])
	}
	return out, true
}

// parseNarration writes the captions and the generated slot rows onto the flow
// the server resolved, and changes nothing else about it. Every removal it
// makes is the server's own and is recorded; a moment the writer left silent, a
// fact it did not state and a source it did not use record nothing (CLIP-138).
func parseNarration(cfg Config, input clip.NarrationInput, raw string) (out clip.EditPlan, err error) {
	var wire narrationJSON
	phase := "decode"
	defer func() {
		if err != nil {
			if _, exists := clip.DiagnosticFromError(err); !exists {
				err = planFailure(err, input.PlanningInput, input.Flow, phase, 0)
			}
		}
	}()
	if err := decode(raw, cfg.MaxResponseBytes, narrationShape, &wire); err != nil {
		return clip.EditPlan{}, err
	}
	limits := compositionLimits(cfg, input.PlanningInput)
	doc, problem := composition.Parse(input.Composition.Snapshot.Body, limits)
	if problem != nil {
		return clip.EditPlan{}, problem
	}
	phase = "composition"
	plan := input.Flow
	portable := *plan.Portable
	portable.Elements = nil
	plan.Portable = &portable
	if len(wire.Cuts) > 0 {
		recordGeneratedNotice(&plan, "composition_generated_identity", "", "", "removal")
	}
	timeline, fallbacks, err := clip.ResolveSelectedComposition(doc, portable.Inputs, portable.Cuts, limits, cfg.MaxResponseBytes)
	if err != nil {
		return clip.EditPlan{}, err
	}
	portable.Fallbacks = fallbacks

	evidence := []clip.ObservedEvidence{}
	for _, cut := range plan.Cuts {
		observed, _ := clip.CutEvidence(input.Analyses, cut)
		evidence = append(evidence, observed...)
	}
	collected := collectedFacts(portable.Inputs)
	instructed := input.Instruction != ""

	slots := map[string]narrationSlotJSON{}
	for _, slot := range wire.Slots {
		if _, exists := slots[slot.ElementID]; exists {
			recordGeneratedNotice(&plan, "composition_generated_identity", "", slot.ElementID, "removal")
			continue
		}
		slots[slot.ElementID] = slot
	}
	declared := map[string]bool{}
	// The outline's caption entries are placed by this call, not resolved into
	// the plan with an interval of their own (CLIP-112).
	captionEntries := map[string]composition.ResolvedElement{}
	for _, entry := range clip.DeclaredCaptions(timeline) {
		captionEntries[entry.Element.ID] = entry
	}
	for _, resolved := range timeline.Elements {
		declared[resolved.Element.ID] = true
		if _, isCaption := captionEntries[resolved.Element.ID]; isCaption {
			continue
		}
		text := clip.PortableText{Resolved: resolved, Scope: "context", Accent: doc.Accent, Pace: doc.Pace}
		if !generatesText(resolved.Element) {
			portable.Elements = append(portable.Elements, text)
			continue
		}
		// A generated region row goes through the ladder it already had: the
		// grounded shorter row, then an empty row with its own notice.
		slot, exists := slots[resolved.Element.ID]
		entry := generatedJSON{ElementID: slot.ElementID, Rows: slot.Rows, ShortRows: slot.ShortRows, Observations: slot.Observations, Facts: slot.Facts}
		attachRegionRows(input.Design.RegionPresets(), doc, portable.Inputs, entry, exists, clip.ItemBinding{}, evidence, &text, &plan, instructed)
		if slices.ContainsFunc(text.Resolved.Rows, func(row composition.ResolvedRow) bool { return strings.TrimSpace(row.Text) != "" }) {
			portable.Elements = append(portable.Elements, text)
		}
	}
	for id := range slots {
		if !declared[id] {
			recordGeneratedNotice(&plan, "composition_generated_identity", "", id, "removal")
		}
	}

	admitNarration(cfg, input, &plan, &portable, narrationCaptions(&plan, wire, timeline), evidence, collected, doc.Pace, instructed)
	clip.RecomputePlanNotices(&plan, input.TargetDurationMS, cfg.TargetToleranceMS)
	if _, err := clip.EncodeEditPlan(plan); err != nil {
		return clip.EditPlan{}, err
	}
	return plan, nil
}

// narrationCaptions is every caption this response places: the ones the writer
// wrote, and the outline's own entries carrying the text the template already
// fixed (CLIP-112). A declared entry the response left out is shown nowhere and
// says so once (CLIP-108).
func narrationCaptions(plan *clip.EditPlan, wire narrationJSON, timeline composition.Timeline) []narrationCaptionJSON {
	placed := map[string]narrationDeclaredJSON{}
	for _, entry := range wire.Declared {
		if _, seen := placed[entry.ElementID]; seen {
			recordGeneratedNotice(plan, "composition_generated_identity", "", entry.ElementID, "removal")
			continue
		}
		placed[entry.ElementID] = entry
	}
	out := slices.Clone(wire.Captions)
	for _, declared := range clip.DeclaredCaptions(timeline) {
		answer, exists := placed[declared.Element.ID]
		if !exists {
			recordGeneratedNotice(plan, "copy_omitted", "", declared.Element.ID, "removal")
			continue
		}
		caption := narrationCaptionJSON{declaredID: declared.Element.ID, ID: declared.Element.ID,
			Text: answer.Text, ShortText: answer.ShortText, Keyword: answer.Keyword, Style: answer.Style,
			StartMS: answer.StartMS, EndMS: answer.EndMS, Observations: answer.Observations, Facts: answer.Facts}
		// A fixed entry says what the template wrote, whatever the response
		// returned in its place (CLIP-65).
		if declared.Element.Kind != "ai" {
			caption.authored = true
			caption.Text, caption.ShortText, caption.Keyword = declared.Text, "", ""
			caption.Observations, caption.Facts = nil, nil
		}
		out = append(out, caption)
	}
	return out
}

// admitNarration walks the captions in start order and admits each one against
// the output it will play on. Every refusal is the server's own removal and
// carries its reason (CLIP-138); the captions that survive hold disjoint
// absolute windows and their identities are minted here, never by the writer.
func admitNarration(cfg Config, input clip.NarrationInput, plan *clip.EditPlan, portable *clip.PortablePlan, captions []narrationCaptionJSON, evidence []clip.ObservedEvidence, collected []composition.Fact, pace string, instructed bool) {
	ordered := slices.Clone(captions)
	slices.SortStableFunc(ordered, func(a, b narrationCaptionJSON) int { return a.StartMS - b.StartMS })
	used := map[string]bool{}
	admitted := []clip.PortableText{}
	maxChars := design.Caption().Lines * design.Caption().Chars
	styles := input.Design.AllowedCaptionStyles()
	for index, caption := range ordered {
		id := clip.NarrationID(index + 1)
		if caption.declaredID != "" {
			id = caption.declaredID
		}
		drop := func(reason string) {
			plan.Portable = portable
			portable.Fallbacks = append(portable.Fallbacks, clip.CopyFallback{ElementID: id, Reason: reason})
		}
		if len(admitted) >= min(cfg.Render.MaxCuts, compositionLimits(cfg, input.PlanningInput).Cuts) {
			drop("composition_generated_bounds")
			continue
		}
		if caption.StartMS < 0 || caption.EndMS <= caption.StartMS || caption.EndMS > plan.DurationMS {
			drop(clip.NoticeCaptionOutsideOutput)
			continue
		}
		// Ordered by start, so only the caption before this one can collide.
		if len(admitted) > 0 && caption.StartMS < admitted[len(admitted)-1].Resolved.EndMS {
			drop(clip.NoticeCaptionOverlap)
			continue
		}
		cited, valid := citedFacts(caption.Facts, collected)
		observed, complete := selectedReferences(caption.Observations, evidence, false)
		if !valid || !complete {
			drop("unavailable_scoped_fact")
			continue
		}
		// An absent choice is not a repair. Freeze the selection's default so
		// a later render does not choose a different treatment for this caption.
		style, styleFallback := caption.Style, false
		if style == "" {
			style = styles[0]
		} else if !slices.Contains(styles, style) {
			style, styleFallback = styles[0], true
		}
		rule, _ := design.CaptionRule(style)
		// The writer only styled the template's words: a style too narrow for
		// them gives way to the selection's first, as a face lacking a syllable
		// does (CDS-84), rather than losing the template's caption.
		if caption.authored && !rule.Holds(caption.Text) {
			if first, _ := design.CaptionRule(styles[0]); first.Holds(caption.Text) {
				style, rule, styleFallback = styles[0], first, true
			}
		}
		// A written caption is bounded by the style it names (CLIP-118); a rapid
		// phrase carries its own bound, and the template's words keep theirs.
		bounded := func(value string) bool {
			if pace == "rapid" || caption.authored {
				return design.Chars(value) <= maxChars
			}
			return rule.Holds(value)
		}
		// The full sentence first, then the grounded shorter one: the same
		// ladder a scene-bound caption answered, minus every item rule.
		check := func(value string) string {
			if strings.TrimSpace(value) == "" {
				return "copy_omitted"
			}
			if !bounded(value) {
				return "composition_generated_bounds"
			}
			// The template's own words are the owner's claim, not the writer's,
			// and saying them once more is what the template asked for.
			if caption.authored {
				return ""
			}
			if reason := clip.GroundNarration(value, collected, instructed); reason != "" {
				return reason
			}
			if used[sentenceKey(value)] {
				return "repeated_copy"
			}
			return ""
		}
		chosen, reason, fallback := caption.Text, check(caption.Text), ""
		if reason != "" {
			if short := check(caption.ShortText); short != "" {
				drop(reason)
				continue
			}
			chosen, fallback = caption.ShortText, reason
		}
		// CDS-41: a caption owns enough time to be read. The shorter sentence
		// first, then the room the next caption leaves, then no caption at all —
		// never a sentence nobody can finish reading.
		start, end := caption.StartMS, caption.EndMS
		if end-start < clip.MinExposureMS(chosen) {
			if short := caption.ShortText; fallback == "" && check(short) == "" && end-start >= clip.MinExposureMS(short) {
				chosen, fallback = short, "short_text"
			} else {
				room := plan.DurationMS
				if index+1 < len(ordered) {
					room = min(room, ordered[index+1].StartMS)
				}
				end = min(room, start+clip.MinExposureMS(chosen))
			}
		}
		if end-start < clip.MinExposureMS(chosen) {
			drop(clip.NoticeCaptionFloor)
			continue
		}
		used[sentenceKey(chosen)] = true
		// Every caption on the timeline carries a minted narration identity,
		// declared or written: the owner edits, retimes and restyles them all
		// the same way in ② (CLIP-17). Only a refusal names the outline entry
		// it came from, because that is what the owner has to fix.
		text := clip.NarrationCaption(clip.NarrationID(len(admitted)+1), chosen, start, end)
		text.Authored = caption.authored
		if styleFallback {
			clip.AddPlanNotice(plan, "composition_caption_style", "", text.Resolved.Element.ID, "style_fallback")
		}
		text.Resolved.Element.Style = style
		text.Resolved.Facts, text.Evidence, text.Pace, text.FallbackReason = cited, observed, pace, fallback
		if caption.Keyword != "" && strings.Contains(chosen, caption.Keyword) {
			text.Keyword = caption.Keyword
		}
		if caption.ShortText != "" && caption.ShortText != chosen && check(caption.ShortText) == "" {
			text.Alternatives = []clip.CopyAlternative{{Text: caption.ShortText}}
		}
		if pace == "rapid" {
			text.Phrases = narrationPhrases(chosen, start, end)
		}
		admitted = append(admitted, text)
	}
	portable.Elements = append(portable.Elements, admitted...)
	plan.Portable = portable
}

// narrationPhrases splits a rapid caption across its own absolute window. A
// split that does not fit leaves the caption whole rather than hurrying it past
// what CDS-59 says a phrase needs.
func narrationPhrases(text string, startMS, endMS int) []clip.EditablePhrase {
	copies, ok := clip.SplitRapid(clip.Caption{Text: text}, startMS, endMS)
	if !ok {
		return nil
	}
	var out []clip.EditablePhrase
	for _, copy := range copies {
		if design.Chars(copy.Text) > design.Rapid.MaxChars {
			return nil
		}
		out = append(out, clip.EditablePhrase{Text: copy.Text, StartMS: copy.StartMS, EndMS: copy.EndMS})
	}
	return out
}
