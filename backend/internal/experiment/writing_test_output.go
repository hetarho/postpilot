package experiment

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/post"
)

// TestOutput is a complete candidate answer with its private result-local origin
// and request evidence. Public projections keep only the canonical block fields.
type TestOutput struct {
	Content            TestOutputContent
	Storyline          *TestOutputStoryline
	Nouns              []string
	ContentLanguage    string
	Origins            *post.OriginReview
	RequestInspections []llm.RequestInspection
	// Observations is a photo observation test's whole answer (MODEL-97): one entry per
	// frozen photo and no post. An output holding them carries no content or storyline.
	Observations []TestOutputObservation
}

// TestOutputObservation is what one observer recorded for one frozen photo. Model names the
// observer and is dropped from every public projection; reveal names it from the identity.
type TestOutputObservation struct {
	File, Scene, Mood, VisibleText string
	Objects                        []string
	PeoplePresent                  bool
	Model                          string
}

type TestOutputContent struct {
	Title, Summary string
	Tags           []string
	Blocks         []TestOutputBlock
}

type TestOutputBlock struct {
	Type, Content      string
	Level              int32
	File, Alt, Caption string
	Items, Files       []string
	Layout             string
}

type TestOutputStoryline struct {
	Paragraphs []TestOutputParagraph
	MadeWith   []string
	Origins    *post.PlanOriginReview
}

type TestOutputParagraph struct {
	Text  string
	Files []string
}

// The private storage edge has an explicit version. Its optional evidence uses
// only the bounded product projections, never accounting or supplier payloads.
type testOutputWire struct {
	Version            int                      `json:"version"`
	Content            testOutputContentWire    `json:"content"`
	Storyline          *testOutputStorylineWire `json:"storyline"`
	Nouns              []string                 `json:"nouns"`
	ContentLanguage    string                   `json:"content_language"`
	Origins            json.RawMessage          `json:"origins,omitempty"`
	RequestInspections json.RawMessage          `json:"request_inspections,omitempty"`
}

type testOutputContentWire struct {
	Title   string                `json:"title"`
	Summary string                `json:"summary"`
	Tags    []string              `json:"tags"`
	Blocks  []testOutputBlockWire `json:"blocks"`
}

type testOutputBlockWire struct {
	Type    string   `json:"type"`
	Content string   `json:"content"`
	Level   int32    `json:"level"`
	File    string   `json:"file"`
	Alt     string   `json:"alt"`
	Caption string   `json:"caption"`
	Items   []string `json:"items"`
	Files   []string `json:"files"`
	Layout  string   `json:"layout"`
}

type testOutputStorylineWire struct {
	Paragraphs []testOutputParagraphWire `json:"paragraphs"`
	MadeWith   []string                  `json:"made_with"`
	Origins    json.RawMessage           `json:"origins,omitempty"`
}

type testOutputParagraphWire struct {
	Text  string   `json:"text"`
	Files []string `json:"files"`
}

// Version 2 is a photo observation answer: observations and no content, so a reader of a
// post output can never mistake one for the other.
type testObservationOutputWire struct {
	Version            int                         `json:"version"`
	Observations       []testOutputObservationWire `json:"observations"`
	RequestInspections json.RawMessage             `json:"request_inspections,omitempty"`
}

type testOutputObservationWire struct {
	File          string   `json:"file"`
	Scene         string   `json:"scene"`
	Mood          string   `json:"mood"`
	VisibleText   string   `json:"visible_text"`
	Objects       []string `json:"objects"`
	PeoplePresent bool     `json:"people_present"`
	Model         string   `json:"model,omitempty"`
}

const (
	maxTestOutputBytes           = 4 << 20
	testPostOutputVersion        = 1
	testObservationOutputVersion = 2
)

func EncodeTestOutput(value TestOutput) ([]byte, error) {
	if value.Observations != nil {
		return encodeTestObservationOutput(value)
	}
	wire := testOutputWire{Version: testPostOutputVersion, ContentLanguage: value.ContentLanguage, Nouns: value.Nouns}
	wire.Origins, _ = json.Marshal(value.Origins)
	if value.Origins == nil {
		wire.Origins = nil
	}
	if len(value.RequestInspections) > 0 {
		wire.RequestInspections, _ = json.Marshal(validTestInspections(value.RequestInspections))
	}
	wire.Content = testOutputContentWire{Title: value.Content.Title, Summary: value.Content.Summary, Tags: value.Content.Tags}
	if value.Content.Blocks != nil {
		wire.Content.Blocks = make([]testOutputBlockWire, len(value.Content.Blocks))
	}
	for index, b := range value.Content.Blocks {
		wire.Content.Blocks[index] = testOutputBlockWire{
			Type: b.Type, Content: b.Content, Level: b.Level, File: b.File, Alt: b.Alt,
			Caption: b.Caption, Items: b.Items, Files: b.Files, Layout: b.Layout,
		}
	}
	if value.Storyline != nil {
		wire.Storyline = &testOutputStorylineWire{MadeWith: value.Storyline.MadeWith}
		if value.Storyline.Origins != nil {
			wire.Storyline.Origins, _ = json.Marshal(value.Storyline.Origins)
		}
		if value.Storyline.Paragraphs != nil {
			wire.Storyline.Paragraphs = make([]testOutputParagraphWire, len(value.Storyline.Paragraphs))
		}
		for index, p := range value.Storyline.Paragraphs {
			wire.Storyline.Paragraphs[index] = testOutputParagraphWire{Text: p.Text, Files: p.Files}
		}
	}
	if err := validateTestOutputWire(wire); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(wire)
	if err != nil {
		return nil, err
	}
	if len(raw) > maxTestOutputBytes {
		return nil, fmt.Errorf("%w: output exceeds storage limit", ErrTestOutputIncompatible)
	}
	return raw, nil
}

func encodeTestObservationOutput(value TestOutput) ([]byte, error) {
	if value.Content.Title != "" || value.Content.Summary != "" || len(value.Content.Tags) > 0 || len(value.Content.Blocks) > 0 || value.Storyline != nil || len(value.Nouns) > 0 || value.ContentLanguage != "" || value.Origins != nil {
		return nil, fmt.Errorf("%w: an observation output carries no post", ErrTestOutputIncompatible)
	}
	wire := testObservationOutputWire{Version: testObservationOutputVersion, Observations: make([]testOutputObservationWire, len(value.Observations))}
	for index, o := range value.Observations {
		wire.Observations[index] = testOutputObservationWire{File: o.File, Scene: o.Scene, Mood: o.Mood, VisibleText: o.VisibleText, Objects: o.Objects, PeoplePresent: o.PeoplePresent, Model: o.Model}
	}
	if len(value.RequestInspections) > 0 {
		wire.RequestInspections, _ = json.Marshal(validTestInspections(value.RequestInspections))
	}
	if err := validateTestObservationWire(wire); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(wire)
	if err != nil {
		return nil, err
	}
	if len(raw) > maxTestOutputBytes {
		return nil, fmt.Errorf("%w: output exceeds storage limit", ErrTestOutputIncompatible)
	}
	return raw, nil
}

func DecodeTestOutput(raw []byte) (TestOutput, error) {
	if len(raw) == 0 || len(raw) > maxTestOutputBytes {
		return TestOutput{}, ErrTestOutputIncompatible
	}
	var head struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(raw, &head); err != nil {
		return TestOutput{}, fmt.Errorf("%w: invalid output encoding", ErrTestOutputIncompatible)
	}
	if head.Version == testObservationOutputVersion {
		return decodeTestObservationOutput(raw)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var wire testOutputWire
	if err := decoder.Decode(&wire); err != nil {
		return TestOutput{}, fmt.Errorf("%w: invalid output encoding", ErrTestOutputIncompatible)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return TestOutput{}, fmt.Errorf("%w: trailing output data", ErrTestOutputIncompatible)
	}
	if err := validateTestOutputWire(wire); err != nil {
		return TestOutput{}, err
	}
	value := TestOutput{ContentLanguage: wire.ContentLanguage, Nouns: wire.Nouns}
	value.Origins = decodeOptionalTestEvidence[post.OriginReview](wire.Origins)
	if requests := decodeOptionalTestEvidence[[]llm.RequestInspection](wire.RequestInspections); requests != nil {
		value.RequestInspections = validTestInspections(*requests)
	}
	value.Content = TestOutputContent{Title: wire.Content.Title, Summary: wire.Content.Summary, Tags: wire.Content.Tags}
	if wire.Content.Blocks != nil {
		value.Content.Blocks = make([]TestOutputBlock, len(wire.Content.Blocks))
	}
	for index, b := range wire.Content.Blocks {
		value.Content.Blocks[index] = TestOutputBlock{
			Type: b.Type, Content: b.Content, Level: b.Level, File: b.File, Alt: b.Alt,
			Caption: b.Caption, Items: b.Items, Files: b.Files, Layout: b.Layout,
		}
	}
	if wire.Storyline != nil {
		value.Storyline = &TestOutputStoryline{MadeWith: wire.Storyline.MadeWith}
		value.Storyline.Origins = decodeOptionalTestEvidence[post.PlanOriginReview](wire.Storyline.Origins)
		if wire.Storyline.Paragraphs != nil {
			value.Storyline.Paragraphs = make([]TestOutputParagraph, len(wire.Storyline.Paragraphs))
		}
		for index, p := range wire.Storyline.Paragraphs {
			value.Storyline.Paragraphs[index] = TestOutputParagraph{Text: p.Text, Files: p.Files}
		}
	}
	return value, nil
}

func decodeTestObservationOutput(raw []byte) (TestOutput, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var wire testObservationOutputWire
	if err := decoder.Decode(&wire); err != nil {
		return TestOutput{}, fmt.Errorf("%w: invalid output encoding", ErrTestOutputIncompatible)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return TestOutput{}, fmt.Errorf("%w: trailing output data", ErrTestOutputIncompatible)
	}
	if err := validateTestObservationWire(wire); err != nil {
		return TestOutput{}, err
	}
	value := TestOutput{Observations: make([]TestOutputObservation, len(wire.Observations))}
	for index, o := range wire.Observations {
		value.Observations[index] = TestOutputObservation{File: o.File, Scene: o.Scene, Mood: o.Mood, VisibleText: o.VisibleText, Objects: o.Objects, PeoplePresent: o.PeoplePresent, Model: o.Model}
	}
	if requests := decodeOptionalTestEvidence[[]llm.RequestInspection](wire.RequestInspections); requests != nil {
		value.RequestInspections = validTestInspections(*requests)
	}
	return value, nil
}

// Every observation names the frozen photo it describes, once.
func validateTestObservationWire(value testObservationOutputWire) error {
	if value.Version != testObservationOutputVersion || len(value.Observations) == 0 {
		return ErrTestOutputIncompatible
	}
	files := make(map[string]bool, len(value.Observations))
	for _, o := range value.Observations {
		if o.File == "" || files[o.File] {
			return ErrTestOutputIncompatible
		}
		files[o.File] = true
	}
	return nil
}

// Optional private evidence never makes an otherwise usable canonical answer fail.
// Unknown fields, malformed metadata and unissued previews are discarded rather
// than being returned as captured execution or interpreted as an origin category.
func decodeOptionalTestEvidence[T any](raw []byte) *T {
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return nil
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	var value T
	if d.Decode(&value) != nil || d.Decode(new(any)) != io.EOF {
		return nil
	}
	return &value
}

func validTestInspections(values []llm.RequestInspection) []llm.RequestInspection {
	var out []llm.RequestInspection
	for _, value := range values {
		if value.Status == llm.InspectionCaptured && value.Validate() == nil {
			out = append(out, value)
		}
	}
	return out
}

// PublicTestOutput retains only the complete post/plan. Frozen source evidence
// and issued prompts have a separate owner/reveal-scoped read, even after reveal.
// Invalid stored output is absent instead of forwarding an opaque JSON payload.
func PublicTestOutput(raw []byte) []byte {
	value, err := DecodeTestOutput(raw)
	if err != nil {
		return nil
	}
	value.Origins, value.RequestInspections = nil, nil
	for index := range value.Observations {
		value.Observations[index].Model = ""
	}
	if value.Storyline != nil {
		value.Storyline.Origins = nil
	}
	out, err := EncodeTestOutput(value)
	if err != nil {
		return nil
	}
	return out
}

func validateTestOutputWire(value testOutputWire) error {
	if value.Version != testPostOutputVersion || (value.ContentLanguage != "ko" && value.ContentLanguage != "en") {
		return ErrTestOutputIncompatible
	}
	// The generation parser already validates and bounds field combinations.
	// This edge preserves that accepted answer, including an empty storyline,
	// while refusing unknown canonical block kinds instead of losing them in RPC.
	for _, b := range value.Content.Blocks {
		switch b.Type {
		case "TEXT", "HEADING", "IMAGE", "QUOTE", "LIST", "VIDEO", "GALLERY":
		default:
			return ErrTestOutputIncompatible
		}
	}
	return nil
}
