package generation

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/postpilot/backend/internal/llm"
)

const badOutputPrefix = "모델이 JSON 대신 다른 답을 돌려줬어요: "

type ErrBadOutput struct{ Head string }

func (e *ErrBadOutput) Error() string { return badOutputPrefix + e.Head }
func (e *ErrBadOutput) Unwrap() error { return llm.ErrBadOutput }

// responseParseError preserves the provider's completion-budget signal when a
// non-empty response was cut off mid-JSON. A syntactically invalid full response is
// still ErrBadOutput; a length-limited partial response has an actionable remedy.
//
// When the provider reported where the budget went, the failure carries the split so the
// remedy follows from the message: a body that filled its budget wants a larger one, while
// one the model never wrote because it reasoned through the budget wants a lower effort for
// this purpose. A provider that reported nothing keeps the bare sentinel.
func responseParseError(response llm.Response, err error) error {
	return llm.ResponseParseError(response, err)
}

type blockJSON struct {
	Type    string   `json:"type"`
	Content string   `json:"content"`
	Level   int32    `json:"level"`
	File    string   `json:"file"`
	Alt     string   `json:"alt"`
	Caption string   `json:"caption"`
	Items   []string `json:"items"`
	// Present only on an unfilled template slot. A revision receives the current content and
	// must hand untouched blocks back byte for byte, so the marker has to survive that round
	// trip — otherwise every revision would quietly turn a reserved position into prose.
	Slot *blockSlotJSON `json:"slot,omitempty"`
}

type blockSlotJSON struct {
	Kind  string `json:"kind"`
	Label string `json:"label,omitempty"`
}

type contentJSON struct {
	Title   string      `json:"title"`
	Summary string      `json:"summary"`
	Tags    []string    `json:"tags"`
	Blocks  []blockJSON `json:"blocks"`
}

type observationJSON struct {
	File          string   `json:"file"`
	Scene         string   `json:"scene"`
	Mood          string   `json:"mood"`
	VisibleText   string   `json:"visible_text"`
	Objects       []string `json:"objects"`
	PeoplePresent bool     `json:"people_present"`
	// Video-only, and OPTIONAL on the way in: the photo answer has neither, and the required
	// field list below is the photo one so a photo batch keeps parsing exactly as it did.
	// The video schema requires them of the model; this struct only has to be able to hold
	// what comes back.
	Events []string `json:"events,omitempty"`
	Speech string   `json:"speech,omitempty"`
}

type observationsJSON struct {
	Observations []observationJSON `json:"observations"`
}

// ParseContent turns the model's text into a PostContent. tagCount is the frozen per-post
// count: a longer tag list keeps its first tagCount entries in model order and a shorter one
// is accepted as written (GEN-46) — failing a paid write over a miscount is worse than a
// shorter list. A missing `tags` key is still bad output.
func ParseContent(raw string, tagCount int) (*PostContent, error) {
	content, _, err := parseContentFields(raw, tagCount)
	return content, err
}

// ParseWriteAnswer is ParseContent for the write pass, whose answer also carries `nouns`
// (GEN-55). The content comes from the same helper, so it is exactly what ParseContent would
// return. The nouns never fail a paid write: a missing, null or malformed member is none, the
// way a tag miscount is accepted rather than refused (GEN-46).
func ParseWriteAnswer(raw string, tagCount int) (*WriteAnswer, error) {
	content, fields, err := parseContentFields(raw, tagCount)
	if err != nil {
		return nil, err
	}
	return &WriteAnswer{
		Content:      *content,
		Nouns:        boundedNouns(fields["nouns"]),
		Replacements: shapedReplacements(fields["replacements"]),
	}, nil
}

// replacementJSON is one candidate as the write answer carries it (GEN-53).
type replacementJSON struct {
	Surface string   `json:"surface"`
	Index   int      `json:"index"`
	Source  string   `json:"source"`
	Phrases []string `json:"phrases"`
}

// shapedReplacements only shapes: a missing, null or non-array member is none, and an item
// lacking one of its four keys, or failing to decode, is dropped alone. What survives is
// ValidateReplacements' to judge against the final content — and like the nouns, none of it
// can fail a paid write.
func shapedReplacements(raw json.RawMessage) []Replacement {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		slog.Warn("dropping malformed generated replacements", "err", err)
		return nil
	}
	var out []Replacement
	for _, item := range items {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(item, &fields); err != nil || !hasFields(fields, "surface", "index", "source", "phrases") {
			slog.Warn("dropping a malformed generated replacement candidate")
			continue
		}
		var wire replacementJSON
		if err := json.Unmarshal(item, &wire); err != nil {
			slog.Warn("dropping a malformed generated replacement candidate", "err", err)
			continue
		}
		out = append(out, Replacement{
			Surface: ReplacementSurface(wire.Surface), Index: wire.Index, Source: wire.Source, Phrases: wire.Phrases,
		})
	}
	return out
}

// parseContentFields is what both parsers share: the candidate extraction, the four required
// members, the tag bound and the block mapping. It hands the decoded members back as well, so
// the write pass reads the one it adds without parsing the answer twice.
func parseContentFields(raw string, tagCount int) (*PostContent, map[string]json.RawMessage, error) {
	candidate, ok := jsonCandidate(raw)
	if !ok {
		return nil, nil, badOutput(raw)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(candidate), &fields); err != nil || !hasFields(fields, "title", "summary", "tags", "blocks") {
		return nil, nil, badOutput(raw)
	}
	var wire contentJSON
	if err := json.Unmarshal([]byte(candidate), &wire); err != nil {
		return nil, nil, badOutput(raw)
	}
	if tagCount > 0 && len(wire.Tags) > tagCount {
		wire.Tags = wire.Tags[:tagCount]
	}
	content := &PostContent{Title: wire.Title, Summary: wire.Summary, Tags: wire.Tags}
	for _, block := range wire.Blocks {
		content.Blocks = append(content.Blocks, Block{
			Type: BlockType(block.Type), Content: block.Content, Level: block.Level,
			File: block.File, Alt: block.Alt, Caption: block.Caption, Items: block.Items,
			Slot: fromSlotJSON(block.Slot),
		})
	}
	return content, fields, nil
}

// boundedNouns is the authoritative bound on the write answer's nouns; the schema's maxItems
// and the prompt's number only ask for it. Each is trimmed and a blank one dropped; a repeat is
// dropped by case-insensitive comparison, keeping the first spelling, because QUAL-7's English
// containment is case-insensitive; and at most WriteNounsMax survive, in model order.
func boundedNouns(raw json.RawMessage) []string {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		slog.Warn("dropping malformed generated nouns", "err", err)
		return nil
	}
	var nouns []string
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		noun := strings.TrimSpace(value)
		key := strings.ToLower(noun)
		if noun == "" || seen[key] {
			continue
		}
		seen[key] = true
		nouns = append(nouns, noun)
		if len(nouns) == WriteNounsMax {
			break
		}
	}
	return nouns
}

func parseObservations(raw string) ([]Observation, error) {
	candidate, ok := jsonCandidate(raw)
	if !ok {
		return nil, badOutput(raw)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(candidate), &fields); err != nil || !hasFields(fields, "observations") {
		return nil, badOutput(raw)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(fields["observations"], &items); err != nil {
		return nil, badOutput(raw)
	}
	wire := observationsJSON{Observations: make([]observationJSON, 0, len(items))}
	for _, item := range items {
		var itemFields map[string]json.RawMessage
		if err := json.Unmarshal(item, &itemFields); err != nil || !hasFields(itemFields, "file", "scene", "mood", "visible_text", "objects", "people_present") {
			return nil, badOutput(raw)
		}
		var observation observationJSON
		if err := json.Unmarshal(item, &observation); err != nil {
			return nil, badOutput(raw)
		}
		wire.Observations = append(wire.Observations, observation)
	}
	out := make([]Observation, 0, len(wire.Observations))
	for _, item := range wire.Observations {
		out = append(out, Observation{
			File: item.File, Scene: item.Scene, Mood: item.Mood, VisibleText: item.VisibleText,
			Objects: item.Objects, PeoplePresent: item.PeoplePresent,
			Events: item.Events, Speech: item.Speech,
		})
	}
	return out, nil
}

func hasFields(fields map[string]json.RawMessage, required ...string) bool {
	for _, name := range required {
		value, ok := fields[name]
		if !ok || string(value) == "null" {
			return false
		}
	}
	return true
}

func jsonCandidate(raw string) (string, bool) {
	return llm.JSONCandidate(raw)
}

func badOutput(raw string) error {
	runes := []rune(strings.TrimSpace(raw))
	if len(runes) > BadOutputErrorHeadChars {
		runes = runes[:BadOutputErrorHeadChars]
	}
	return &ErrBadOutput{Head: string(runes)}
}

func marshalPromptJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(data)
}

func fromSlotJSON(slot *blockSlotJSON) *BlockSlot {
	if slot == nil || strings.TrimSpace(slot.Kind) == "" {
		return nil
	}
	return &BlockSlot{Kind: slot.Kind, Label: slot.Label}
}
