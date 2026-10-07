package experiment

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// TestOutput is a complete candidate answer. These plain domain types retain the
// same canonical block fields as an ordinary post, without importing its context.
type TestOutput struct {
	Content         TestOutputContent
	Storyline       *TestOutputStoryline
	Nouns           []string
	ContentLanguage string
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
}

type TestOutputParagraph struct {
	Text  string
	Files []string
}

// The private storage edge has an explicit version. It contains no model
// identity, accounting, provider diagnostics or supplier cost.
type testOutputWire struct {
	Version         int                      `json:"version"`
	Content         testOutputContentWire    `json:"content"`
	Storyline       *testOutputStorylineWire `json:"storyline"`
	Nouns           []string                 `json:"nouns"`
	ContentLanguage string                   `json:"content_language"`
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
}

type testOutputParagraphWire struct {
	Text  string   `json:"text"`
	Files []string `json:"files"`
}

const maxTestOutputBytes = 4 << 20

func EncodeTestOutput(value TestOutput) ([]byte, error) {
	wire := testOutputWire{Version: 1, ContentLanguage: value.ContentLanguage, Nouns: value.Nouns}
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

func DecodeTestOutput(raw []byte) (TestOutput, error) {
	if len(raw) == 0 || len(raw) > maxTestOutputBytes {
		return TestOutput{}, ErrTestOutputIncompatible
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
		if wire.Storyline.Paragraphs != nil {
			value.Storyline.Paragraphs = make([]TestOutputParagraph, len(wire.Storyline.Paragraphs))
		}
		for index, p := range wire.Storyline.Paragraphs {
			value.Storyline.Paragraphs[index] = TestOutputParagraph{Text: p.Text, Files: p.Files}
		}
	}
	return value, nil
}

func validateTestOutputWire(value testOutputWire) error {
	if value.Version != 1 || (value.ContentLanguage != "ko" && value.ContentLanguage != "en") {
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
