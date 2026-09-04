package generation

import (
	"encoding/json"
	"fmt"
)

type templatePayload struct {
	Name string `json:"name"`
	// The expanded, rendered body — not the authored source. What the worker prompts with
	// must be exactly what enqueue decided.
	Body  string             `json:"body"`
	Slots []templateSlotJSON `json:"slots,omitempty"`
}

type templateSlotJSON struct {
	Kind  string `json:"kind"`
	Label string `json:"label,omitempty"`
}

type observationPayload struct {
	File          string   `json:"file"`
	Scene         string   `json:"scene,omitempty"`
	Mood          string   `json:"mood,omitempty"`
	VisibleText   string   `json:"visible_text,omitempty"`
	Objects       []string `json:"objects,omitempty"`
	PeoplePresent bool     `json:"people_present,omitempty"`
	Model         string   `json:"model,omitempty"`
}

type generationPayload struct {
	TargetLanguage string           `json:"target_language"`
	TargetLength   *int             `json:"target_length,omitempty"`
	Template       *templatePayload `json:"template,omitempty"`
	// The applicable guideline texts in injection order, frozen exactly as Template is. A
	// payload written before guidelines existed decodes with this absent, which is "none".
	Guidelines []string `json:"guidelines,omitempty"`
	// The photos this run observes, frozen at enqueue. A POINTER, and deliberately without
	// `omitempty`: the three states are absent (a job queued before this contract — observe
	// everything), present-and-empty (observe nothing), and present-with-names. A plain
	// []string with omitempty would encode "reuse everything" as absent and decode it back
	// as "re-observe everything" — the exact silent double-spend this contract prevents.
	ObserveFiles *[]string `json:"observe_files"`
	// The reusable snapshot as it stood at enqueue, from the same read the selection came
	// from. ObserveFiles alone decides which of these entries the run replaces.
	Observations []observationPayload `json:"observations,omitempty"`
}

// GenerationOptions is what a durable generate job froze at enqueue. Every field is an
// option of that one run: a later edit of the post's target length, of the template row, or
// of any guideline must not change the prompt of work already waiting in the queue.
type GenerationOptions struct {
	TargetLanguage Language
	TargetLength   *int
	Template       *TemplateBrief
	Guidelines     []string
	// ObserveFiles carries presence: nil observes every attached photo, non-nil-but-empty
	// observes nothing. See generationPayload.ObserveFiles for why the distinction matters.
	ObserveFiles *[]string
	Observations []Observation
}

// EncodeGenerationPayload freezes generation-only options in the durable job.
func EncodeGenerationPayload(options GenerationOptions) ([]byte, error) {
	if !options.TargetLanguage.Valid() {
		return nil, ErrLanguageRequired
	}
	return json.Marshal(generationPayload{
		TargetLanguage: options.TargetLanguage.String(),
		TargetLength:   cloneOptionalInt(options.TargetLength),
		Template:       encodeTemplate(options.Template),
		Guidelines:     cloneTexts(options.Guidelines),
		ObserveFiles:   cloneOptionalTexts(options.ObserveFiles),
		Observations:   encodeObservations(options.Observations),
	})
}

// DecodeGenerationPayload accepts an empty payload for jobs queued before this
// contract existed; those jobs intentionally carry no target length and no template.
// A payload written before templates existed simply decodes with the field absent.
func DecodeGenerationPayload(raw []byte) (GenerationOptions, error) {
	if len(raw) == 0 {
		return GenerationOptions{TargetLanguage: LanguageKorean}, nil
	}
	var payload generationPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return GenerationOptions{}, fmt.Errorf("decode generation payload: %w", err)
	}
	if payload.TargetLength != nil && *payload.TargetLength <= 0 {
		return GenerationOptions{}, fmt.Errorf("decode generation payload: target length must be positive")
	}
	// Payloads queued before language support did not carry this field. Migration and
	// compatibility both preserve their established Korean behavior.
	language := LanguageKorean
	if payload.TargetLanguage != "" {
		var err error
		language, err = ParseLanguage(payload.TargetLanguage)
		if err != nil {
			return GenerationOptions{}, fmt.Errorf("decode generation payload: %w", err)
		}
	}
	return GenerationOptions{
		TargetLanguage: language,
		TargetLength:   cloneOptionalInt(payload.TargetLength),
		Template:       decodeTemplate(payload.Template),
		Guidelines:     cloneTexts(payload.Guidelines),
		ObserveFiles:   cloneOptionalTexts(payload.ObserveFiles),
		Observations:   decodeObservations(payload.Observations),
	}, nil
}

// cloneOptionalTexts keeps the frozen set frozen while PRESERVING presence: a non-nil empty
// slice must stay non-nil and empty through both the encode and the decode, because that is
// the whole "observe nothing" state.
func cloneOptionalTexts(values *[]string) *[]string {
	if values == nil {
		return nil
	}
	copied := append([]string{}, *values...)
	return &copied
}

func encodeObservations(observations []Observation) []observationPayload {
	if len(observations) == 0 {
		return nil
	}
	wire := make([]observationPayload, 0, len(observations))
	for _, observation := range observations {
		wire = append(wire, observationPayload{
			File: observation.File, Scene: observation.Scene, Mood: observation.Mood,
			VisibleText: observation.VisibleText, Objects: cloneTexts(observation.Objects),
			PeoplePresent: observation.PeoplePresent, Model: observation.Model,
		})
	}
	return wire
}

func decodeObservations(wire []observationPayload) []Observation {
	if len(wire) == 0 {
		return nil
	}
	out := make([]Observation, 0, len(wire))
	for _, observation := range wire {
		out = append(out, Observation{
			File: observation.File, Scene: observation.Scene, Mood: observation.Mood,
			VisibleText: observation.VisibleText, Objects: cloneTexts(observation.Objects),
			PeoplePresent: observation.PeoplePresent, Model: observation.Model,
		})
	}
	return out
}

// cloneTexts keeps a frozen slice frozen: the payload and the job must not share backing
// storage with whatever the caller does next to its own slice.
func cloneTexts(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	return append([]string(nil), values...)
}

func encodeTemplate(brief *TemplateBrief) *templatePayload {
	if brief == nil {
		return nil
	}
	slots := make([]templateSlotJSON, 0, len(brief.Slots))
	for _, slot := range brief.Slots {
		slots = append(slots, templateSlotJSON{Kind: slot.Kind, Label: slot.Label})
	}
	return &templatePayload{Name: brief.Name, Body: brief.Body, Slots: slots}
}

// decodeTemplate reads a payload written before templates existed as "no template" rather
// than failing: a resumable job must not become unresumable because the contract moved.
func decodeTemplate(payload *templatePayload) *TemplateBrief {
	if payload == nil {
		return nil
	}
	slots := make([]TemplateSlot, 0, len(payload.Slots))
	for _, slot := range payload.Slots {
		slots = append(slots, TemplateSlot{Kind: slot.Kind, Label: slot.Label})
	}
	return &TemplateBrief{Name: payload.Name, Body: payload.Body, Slots: slots}
}
