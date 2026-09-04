package generation

import (
	"encoding/json"
	"fmt"
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
	if err == nil || response.FinishReason != "length" {
		return err
	}
	if response.Usage.ReasoningTokens > 0 {
		return &llm.TruncatedError{
			ReasoningTokens:  response.Usage.ReasoningTokens,
			CompletionTokens: response.Usage.CompletionTokens,
		}
	}
	return llm.ErrOutputTruncated
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
}

type observationsJSON struct {
	Observations []observationJSON `json:"observations"`
}

func ParseContent(raw string) (*PostContent, error) {
	candidate, ok := jsonCandidate(raw)
	if !ok {
		return nil, badOutput(raw)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(candidate), &fields); err != nil || !hasFields(fields, "title", "summary", "tags", "blocks") {
		return nil, badOutput(raw)
	}
	var wire contentJSON
	if err := json.Unmarshal([]byte(candidate), &wire); err != nil {
		return nil, badOutput(raw)
	}
	if len(wire.Tags) < TagsMin || len(wire.Tags) > TagsMax {
		return nil, badOutput(raw)
	}
	content := &PostContent{Title: wire.Title, Summary: wire.Summary, Tags: wire.Tags}
	for _, block := range wire.Blocks {
		content.Blocks = append(content.Blocks, Block{
			Type: BlockType(block.Type), Content: block.Content, Level: block.Level,
			File: block.File, Alt: block.Alt, Caption: block.Caption, Items: block.Items,
			Slot: fromSlotJSON(block.Slot),
		})
	}
	return content, nil
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
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "```") {
		if newline := strings.IndexByte(trimmed, '\n'); newline >= 0 {
			trimmed = trimmed[newline+1:]
		}
		trimmed = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(trimmed), "```"))
	}
	if json.Valid([]byte(trimmed)) && strings.HasPrefix(trimmed, "{") {
		return trimmed, true
	}
	start := strings.IndexByte(trimmed, '{')
	if start < 0 {
		return "", false
	}
	depth, inString, escaped := 0, false, false
	for i := start; i < len(trimmed); i++ {
		c := trimmed[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}
		if c == '"' {
			inString = true
			continue
		}
		switch c {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				candidate := trimmed[start : i+1]
				return candidate, json.Valid([]byte(candidate))
			}
		}
	}
	return "", false
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
