package generation

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/post"
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
	// A photo group's own (GEN-77). omitempty so the current content a revise prompt shows names
	// them on a group alone; the schema still asks for both on every block.
	Files  []string `json:"files,omitempty"`
	Layout string   `json:"layout,omitempty"`
}

type contentJSON struct {
	Title   string      `json:"title"`
	Summary string      `json:"summary"`
	Tags    []string    `json:"tags"`
	Blocks  []blockJSON `json:"blocks"`
}

// Origin offsets are never accepted from the model. The owning consumer resolves
// exact quotes/occurrences against canonical text after explicit normalization.
type originFieldJSON struct {
	Kind           string `json:"kind"`
	TagIndex       *int   `json:"tag_index,omitempty"`
	BlockIndex     *int   `json:"block_index,omitempty"`
	ItemIndex      *int   `json:"item_index,omitempty"`
	ParagraphIndex *int   `json:"paragraph_index,omitempty"`
}

type originCandidateJSON struct {
	File       string              `json:"file,omitempty"`
	Field      originFieldJSON     `json:"field"`
	Quote      string              `json:"quote"`
	Occurrence *int                `json:"occurrence"`
	Category   post.OriginCategory `json:"category"`
	SourceRefs []string            `json:"source_refs"`
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
	// The photo's upright turn (GEN-79). Required of the photo answer by its schema, read
	// leniently here: anything but 0, 90, 180 or 270 is no turn.
	Rotation int `json:"rotation,omitempty"`
}

// uprightRotation keeps a reported turn only when it is one of the four quarter turns (GEN-79).
func uprightRotation(degrees int) int {
	switch degrees {
	case 90, 180, 270:
		return degrees
	default:
		return 0
	}
}

type observationsJSON struct {
	Observations []observationJSON `json:"observations"`
}

// ParseContent turns the model's text into a PostContent. tagCount is the frozen per-post
// upper bound: a longer tag list keeps its first tagCount entries in model order;
// fewer or zero tags are valid without padding (GEN-46). A missing `tags` key
// is still bad output.
func ParseContent(raw string, tagCount int) (*PostContent, error) {
	content, _, err := ParseContentWithOrigins(raw, tagCount)
	return content, err
}

func ParseContentWithOrigins(raw string, tagCount int) (*PostContent, []post.OriginCandidate, error) {
	content, fields, err := parseContentFields(raw, tagCount)
	if err != nil {
		return nil, nil, err
	}
	candidates, _ := parseOriginCandidates(fields["origins"])
	return content, candidates, nil
}

// ParseRevisionContent preserves an unchanged returned tag array before applying
// the requested-change upper bound. Equality covers order and exact strings;
// this boundary neither interprets prose instructions nor guesses user intent.
// The explicit requested-only rule belongs to the revision prompt (GEN-40).
func ParseRevisionContent(raw string, tagCount int, current PostContent) (*PostContent, error) {
	content, _, err := ParseRevisionContentWithOrigins(raw, tagCount, current)
	return content, err
}

func ParseRevisionContentWithOrigins(raw string, tagCount int, current PostContent) (*PostContent, []post.OriginCandidate, error) {
	content, fields, err := parseContentFields(raw, 0)
	if err != nil {
		return nil, nil, err
	}
	if slices.Equal(content.Tags, current.Tags) {
		content.Tags = slices.Clone(current.Tags)
	} else if tagCount > 0 && len(content.Tags) > tagCount {
		content.Tags = content.Tags[:tagCount]
	}
	candidates, _ := parseOriginCandidates(fields["origins"])
	return content, candidates, nil
}

// ParseWriteAnswer is ParseContent for the write pass, whose answer also carries `nouns`
// (GEN-55) and `storyline` (GEN-67). The content comes from the same helper, so it is exactly
// what ParseContent would return. Neither member fails a paid write: a missing, null or
// malformed one is none, the way a tag miscount is accepted rather than refused (GEN-46).
// attachments are the names the run was shown; a storyline file outside them is dropped.
func ParseWriteAnswer(raw string, tagCount int, attachments []string) (*WriteAnswer, error) {
	return parseWriteAnswer(raw, tagCount, attachments, []string{"title", "summary", "tags", "blocks", "nouns", "storyline"})
}

func ParseWriteAlongStorylineAnswer(raw string, tagCount int, attachments []string) (*WriteAnswer, error) {
	return parseWriteAnswer(raw, tagCount, attachments, []string{"title", "summary", "tags", "blocks", "nouns"})
}

func parseWriteAnswer(raw string, tagCount int, attachments []string, requiredTail []string) (*WriteAnswer, error) {
	content, fields, err := parseContentFieldsForStage(raw, tagCount, requiredTail)
	if err != nil {
		return nil, err
	}
	candidates, planCandidates := parseOriginCandidates(fields["origins"])
	paragraphs, rawParagraphs, mapping := boundedStorylineWithMap(fields["storyline"], attachments)
	return &WriteAnswer{Content: *content, Nouns: boundedNouns(fields["nouns"]), OriginCandidates: candidates,
		Storyline: &Storyline{Paragraphs: paragraphs, OriginCandidates: RemapPlanOriginCandidates(rawParagraphs, paragraphs, mapping, planCandidates)}}, nil
}

type storylineParagraphJSON struct {
	Text  string   `json:"text"`
	Files []string `json:"files"`
}

// boundedStoryline is the authoritative bound on the write answer's storyline. Each paragraph's
// text is trimmed and cut at StorylineTextMaxChars runes; its files keep only the run's exact
// attachment names, and a name given again keeps its first paragraph; a paragraph left with
// neither text nor a file is dropped; at most StorylineParagraphMax survive, in model order.
func boundedStoryline(raw json.RawMessage, attachments []string) []StorylineParagraph {
	paragraphs, _, _ := boundedStorylineWithMap(raw, attachments)
	return paragraphs
}

func boundedStorylineWithMap(raw json.RawMessage, attachments []string) ([]StorylineParagraph, []StorylineParagraph, []int) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil, nil
	}
	var values []storylineParagraphJSON
	if err := json.Unmarshal(raw, &values); err != nil {
		slog.Warn("dropping a malformed generated storyline", "err", err)
		return nil, nil, nil
	}
	paragraphs, mapping := boundParagraphsWithMap(values, attachments)
	rawParagraphs := make([]StorylineParagraph, len(values))
	for i, value := range values {
		rawParagraphs[i] = StorylineParagraph{Text: value.Text, Files: value.Files}
	}
	return paragraphs, rawParagraphs, mapping
}

// ParseStorylineAnswer reads a storyline job's answer (GEN-68, GEN-69): the storyline is the
// whole output, so unlike the write's member a missing, malformed or empty one is bad output —
// there is nothing else the job could keep. The paragraphs follow the write's bounds.
func ParseStorylineAnswer(raw string, attachments []string) ([]StorylineParagraph, error) {
	paragraphs, _, err := ParseStorylineAnswerWithOrigins(raw, attachments)
	return paragraphs, err
}

func ParseStorylineAnswerWithOrigins(raw string, attachments []string) ([]StorylineParagraph, []PlanOriginCandidate, error) {
	decoded, err := decodeAnswerFields(raw, []string{"storyline"}, []string{"storyline"})
	if err != nil || !hasFields(decoded.values, "storyline") {
		return nil, nil, badOutput(raw)
	}
	fields := decoded.values
	var values []storylineParagraphJSON
	if err := json.Unmarshal(fields["storyline"], &values); err != nil {
		return nil, nil, badOutput(raw)
	}
	paragraphs, mapping := boundParagraphsWithMap(values, attachments)
	if len(paragraphs) == 0 {
		return nil, nil, badOutput(raw)
	}
	rawParagraphs := make([]StorylineParagraph, len(values))
	for i, value := range values {
		rawParagraphs[i] = StorylineParagraph{Text: value.Text, Files: value.Files}
	}
	_, candidates := parseOriginCandidates(fields["origins"])
	return paragraphs, RemapPlanOriginCandidates(rawParagraphs, paragraphs, mapping, candidates), nil
}

// boundParagraphs applies the storyline's bounds to decoded paragraphs; see boundedStoryline.
func boundParagraphs(values []storylineParagraphJSON, attachments []string) []StorylineParagraph {
	paragraphs, _ := boundParagraphsWithMap(values, attachments)
	return paragraphs
}

func boundParagraphsWithMap(values []storylineParagraphJSON, attachments []string) ([]StorylineParagraph, []int) {
	attached := make(map[string]bool, len(attachments))
	for _, name := range attachments {
		attached[name] = true
	}
	placed := make(map[string]bool, len(attachments))
	var paragraphs []StorylineParagraph
	mapping := make([]int, len(values))
	for i := range mapping {
		mapping[i] = -1
	}
	for index, value := range values {
		paragraph := StorylineParagraph{Text: cutRunes(strings.TrimSpace(value.Text), StorylineTextMaxChars)}
		for _, file := range value.Files {
			if attached[file] && !placed[file] {
				placed[file] = true
				paragraph.Files = append(paragraph.Files, file)
			}
		}
		if paragraph.Text == "" && len(paragraph.Files) == 0 {
			continue
		}
		mapping[index] = len(paragraphs)
		paragraphs = append(paragraphs, paragraph)
		if len(paragraphs) == StorylineParagraphMax {
			break
		}
	}
	return paragraphs, mapping
}

// cutRunes keeps at most max runes of text, never splitting one.
func cutRunes(text string, max int) string {
	count := 0
	for i := range text {
		if count == max {
			return text[:i]
		}
		count++
	}
	return text
}

// parseContentFields is what both parsers share: the candidate extraction, the four required
// members, the tag bound and the block mapping. It hands the decoded members back as well, so
// the write pass reads the one it adds without parsing the answer twice.
func parseContentFields(raw string, tagCount int) (*PostContent, map[string]json.RawMessage, error) {
	return parseContentFieldsForStage(raw, tagCount, []string{"title", "summary", "tags", "blocks"})
}

func parseContentFieldsForStage(raw string, tagCount int, requiredTail []string) (*PostContent, map[string]json.RawMessage, error) {
	decoded, err := decodeAnswerFields(raw, requiredTail, []string{"title", "summary", "tags", "blocks", "nouns", "storyline"})
	if err != nil || !hasFields(decoded.values, "title", "summary", "tags", "blocks") {
		return nil, nil, badOutput(raw)
	}
	fields := decoded.values
	var wire contentJSON
	encoded, err := json.Marshal(fields)
	if err != nil || json.Unmarshal(encoded, &wire) != nil {
		return nil, nil, badOutput(raw)
	}
	if decoded.originTail {
		// Full legacy writes remain lenient for absent/malformed nouns/storyline.
		// Salvage needs each stage's required value to be fully decoded and typed.
		for _, name := range requiredTail {
			switch name {
			case "nouns":
				var value []string
				if json.Unmarshal(fields[name], &value) != nil {
					return nil, nil, badOutput(raw)
				}
			case "storyline":
				var value []storylineParagraphJSON
				if json.Unmarshal(fields[name], &value) != nil {
					return nil, nil, badOutput(raw)
				}
			}
		}
	}
	if tagCount > 0 && len(wire.Tags) > tagCount {
		wire.Tags = wire.Tags[:tagCount]
	}
	content := &PostContent{Title: wire.Title, Summary: wire.Summary, Tags: wire.Tags}
	for _, block := range wire.Blocks {
		content.Blocks = append(content.Blocks, Block{
			Type: BlockType(block.Type), Content: block.Content, Level: block.Level,
			File: block.File, Alt: block.Alt, Caption: block.Caption, Items: block.Items,
			Files: block.Files, Layout: block.Layout,
		})
	}
	return content, fields, nil
}

func parseOriginCandidates(raw json.RawMessage) ([]post.OriginCandidate, []PlanOriginCandidate) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var values []originCandidateJSON
	if json.Unmarshal(raw, &values) != nil {
		return nil, nil
	}
	var content []post.OriginCandidate
	var plan []PlanOriginCandidate
	for _, value := range values {
		if value.Field.Kind == "storyline_paragraph" {
			index := -1
			if value.File == "" && value.Field.ParagraphIndex != nil && value.Field.TagIndex == nil && value.Field.BlockIndex == nil && value.Field.ItemIndex == nil {
				index = *value.Field.ParagraphIndex
			}
			plan = append(plan, PlanOriginCandidate{ParagraphIndex: index, Quote: value.Quote, Occurrence: value.Occurrence, Category: value.Category, SourceRefs: value.SourceRefs})
			continue
		}
		field := post.OriginFieldLocator{Kind: post.OriginFieldKind(value.Field.Kind), TagIndex: value.Field.TagIndex, BlockIndex: value.Field.BlockIndex, ItemIndex: value.Field.ItemIndex}
		if value.File != "" || value.Field.ParagraphIndex != nil {
			field.Kind = "invalid_locator"
		}
		content = append(content, post.OriginCandidate{Field: field, Quote: value.Quote, Occurrence: value.Occurrence, Category: value.Category, SourceRefs: value.SourceRefs})
	}
	return content, plan
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
	decoded, err := decodeAnswerFields(raw, []string{"observations"}, []string{"observations"})
	if err != nil || !hasFields(decoded.values, "observations") {
		return nil, badOutput(raw)
	}
	fields := decoded.values
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
			Events: item.Events, Speech: item.Speech, Rotation: uprightRotation(item.Rotation),
		})
	}
	var origins []originCandidateJSON
	if json.Unmarshal(fields["origins"], &origins) == nil {
		files := make(map[string]int, len(out))
		for _, observation := range out {
			files[observation.File]++
		}
		for _, candidate := range origins {
			if candidate.File == "" || files[candidate.File] != 1 {
				continue
			}
			field := strings.TrimPrefix(candidate.Field.Kind, "observation_")
			if candidate.Field.TagIndex != nil || candidate.Field.BlockIndex != nil || candidate.Field.ParagraphIndex != nil {
				field = "invalid_locator"
			}
			for i := range out {
				if out[i].File == candidate.File {
					out[i].OriginCandidates = append(out[i].OriginCandidates, ObservationOriginCandidate{Field: field, ItemIndex: candidate.Field.ItemIndex, Quote: candidate.Quote, Occurrence: candidate.Occurrence, Category: candidate.Category, SourceRefs: candidate.SourceRefs})
					break
				}
			}
		}
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
