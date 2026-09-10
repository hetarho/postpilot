package modelcatalog_test

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/modelcatalog"
)

func TestParseDocumentReadsSectionsAndIgnoresNoise(t *testing.T) {
	text := "\n" + modelcatalog.DocumentVersionLine + "\n" +
		"# a comment the operator left\n\n" +
		"[photo-analysis]\n" +
		"  z-ai/glm-5.3-flash  \n" +
		"google/gemini-3.8-flash\n\n" +
		"[writing]\n" +
		"anthropic/claude-sonnet-5\n"

	doc, issues := modelcatalog.ParseDocument(text)
	if len(issues) != 0 {
		t.Fatalf("issues = %v, want none", issues)
	}
	if len(doc.Sections) != 2 {
		t.Fatalf("sections = %d, want 2", len(doc.Sections))
	}
	if doc.Sections[0].Purpose != modelcatalog.PurposePhotoAnalysis {
		t.Errorf("first section = %q", doc.Sections[0].Purpose)
	}
	want := []modelcatalog.DocumentEntry{
		// Line numbers are over the document as pasted, blank and comment lines included.
		{ModelID: "z-ai/glm-5.3-flash", Line: 6},
		{ModelID: "google/gemini-3.8-flash", Line: 7},
	}
	if !reflect.DeepEqual(doc.Sections[0].Entries, want) {
		t.Errorf("entries = %v, want %v (surrounding whitespace is trimmed)", doc.Sections[0].Entries, want)
	}
}

func TestParseDocumentRejections(t *testing.T) {
	version := modelcatalog.DocumentVersionLine + "\n"
	cases := map[string]struct {
		text  string
		cause string
		line  int
	}{
		"missing version": {
			text: "[writing]\nanthropic/claude-sonnet-5\n", cause: modelcatalog.IssueBadVersion, line: 1,
		},
		"wrong version": {
			text: "# postpilot models v2\n[writing]\n", cause: modelcatalog.IssueBadVersion, line: 1,
		},
		"empty document": {
			text: "\n\n  \n", cause: modelcatalog.IssueBadVersion, line: 1,
		},
		"unknown purpose": {
			text: version + "[audio-analysis]\n", cause: modelcatalog.IssueUnknownPurpose, line: 2,
		},
		"duplicate section": {
			text: version + "[writing]\n[writing]\n", cause: modelcatalog.IssueDuplicateSection, line: 3,
		},
		"id before any section": {
			text: version + "anthropic/claude-sonnet-5\n", cause: modelcatalog.IssueOrphanID, line: 2,
		},
		"duplicate id in one section": {
			text:  version + "[writing]\nanthropic/claude-sonnet-5\nanthropic/claude-sonnet-5\n",
			cause: modelcatalog.IssueDuplicateID, line: 4,
		},
		"pasted table row": {
			text:  version + "[writing]\n| 무료 | anthropic/claude-sonnet-5 | 2 / 10 |\n",
			cause: modelcatalog.IssueMalformedLine, line: 3,
		},
		"backticked id": {
			text: version + "[writing]\n`anthropic/claude-sonnet-5`\n", cause: modelcatalog.IssueMalformedLine, line: 3,
		},
		"unterminated header": {
			text: version + "[writing\n", cause: modelcatalog.IssueMalformedLine, line: 2,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, issues := modelcatalog.ParseDocument(tc.text)
			if len(issues) == 0 {
				t.Fatal("want an issue, got none")
			}
			if issues[0].Cause != tc.cause {
				t.Errorf("cause = %q, want %q", issues[0].Cause, tc.cause)
			}
			if issues[0].Line != tc.line {
				t.Errorf("line = %d, want %d", issues[0].Line, tc.line)
			}
		})
	}
}

func TestParseDocumentReportsEveryBadLineAtOnce(t *testing.T) {
	text := modelcatalog.DocumentVersionLine + "\n[writing]\n" +
		"anthropic/claude-sonnet-5\n" +
		"anthropic/claude-sonnet-5\n" +
		"[audio-analysis]\n" +
		"| pasted | row |\n"
	_, issues := modelcatalog.ParseDocument(text)
	if len(issues) != 3 {
		t.Fatalf("issues = %v, want three — a paste is fixed by seeing all of its problems", issues)
	}
}

func TestRenderDocumentIsCompleteSortedAndStable(t *testing.T) {
	doc := modelcatalog.RenderDocument(map[modelcatalog.Purpose][]modelcatalog.DocumentEntry{
		modelcatalog.PurposeWriting:       {{ModelID: "z-ai/glm-5.3"}, {ModelID: "anthropic/claude-sonnet-5"}},
		modelcatalog.PurposePhotoAnalysis: {{ModelID: "google/gemini-3.8-flash"}},
	})
	if !strings.HasPrefix(doc, modelcatalog.DocumentVersionLine+"\n") {
		t.Fatalf("document must open with the version line:\n%s", doc)
	}
	for _, purpose := range modelcatalog.Purposes {
		if !strings.Contains(doc, "["+string(purpose)+"]\n") {
			t.Errorf("every purpose gets a section, %q is missing", purpose)
		}
	}
	writing := doc[strings.Index(doc, "[writing]"):]
	if strings.Index(writing, "anthropic/claude-sonnet-5") > strings.Index(writing, "z-ai/glm-5.3") {
		t.Error("ids are sorted within a section so the output is byte-stable")
	}
	if doc != modelcatalog.RenderDocument(map[modelcatalog.Purpose][]modelcatalog.DocumentEntry{
		modelcatalog.PurposeWriting:       {{ModelID: "anthropic/claude-sonnet-5"}, {ModelID: "z-ai/glm-5.3"}},
		modelcatalog.PurposePhotoAnalysis: {{ModelID: "google/gemini-3.8-flash"}},
	}) {
		t.Error("input order must not change the rendered bytes")
	}
}

func TestRenderedDocumentParsesBackToWhatWentIn(t *testing.T) {
	in := map[modelcatalog.Purpose][]modelcatalog.DocumentEntry{
		// Levels ride the round trip too: an exported document must parse back to exactly
		// the registrations AND grades it was rendered from (MODEL-55, MODEL-59).
		modelcatalog.PurposeWriting: {
			{ModelID: "z-ai/glm-5.3", Level: modelcatalog.LevelBalanced},
			{ModelID: "anthropic/claude-sonnet-5"},
		},
		modelcatalog.PurposePhotoAnalysis: {{ModelID: "google/gemini-3.8-flash", Level: modelcatalog.LevelValue}},
	}
	doc, issues := modelcatalog.ParseDocument(modelcatalog.RenderDocument(in))
	if len(issues) != 0 {
		t.Fatalf("a rendered document must parse cleanly, got %v", issues)
	}
	if len(doc.Sections) != len(modelcatalog.Purposes) {
		t.Fatalf("sections = %d, want all five", len(doc.Sections))
	}
	for _, section := range doc.Sections {
		want := slices.Clone(in[section.Purpose])
		slices.SortFunc(want, func(a, b modelcatalog.DocumentEntry) int {
			return strings.Compare(a.ModelID, b.ModelID)
		})
		// The rendered document has no line numbers to compare against, so the round trip
		// is over the two fields it actually carries.
		got := make([]modelcatalog.DocumentEntry, 0, len(section.Entries))
		for _, entry := range section.Entries {
			got = append(got, modelcatalog.DocumentEntry{ModelID: entry.ModelID, Level: entry.Level})
		}
		if len(want) == 0 && len(got) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s = %v, want %v", section.Purpose, got, want)
		}
	}
}

// T093/MODEL-59: an id line may name the registration's level. Every document that was
// valid before this change still parses to the same registrations, with every level unset.
func TestParseDocument_IDLineMayCarryALevel(t *testing.T) {
	text := modelcatalog.DocumentVersionLine + "\n" +
		"[writing]\n" +
		"anthropic/claude-sonnet-5 top\n" +
		// Any run of whitespace separates the tokens, tabs included — the format stays
		// forgiving about spacing and strict about everything else.
		"z-ai/glm-5.3\tbalanced\n" +
		"deepseek/deepseek-v4-flash-0731   value\n" +
		// An id-only line is a real instruction: leave this registration unlevelled.
		"google/gemini-3.8-flash\n"

	doc, issues := modelcatalog.ParseDocument(text)
	if len(issues) != 0 {
		t.Fatalf("issues = %v, want none", issues)
	}
	want := []modelcatalog.DocumentEntry{
		{ModelID: "anthropic/claude-sonnet-5", Level: modelcatalog.LevelTop, Line: 3},
		{ModelID: "z-ai/glm-5.3", Level: modelcatalog.LevelBalanced, Line: 4},
		{ModelID: "deepseek/deepseek-v4-flash-0731", Level: modelcatalog.LevelValue, Line: 5},
		{ModelID: "google/gemini-3.8-flash", Line: 6},
	}
	if !reflect.DeepEqual(doc.Sections[0].Entries, want) {
		t.Errorf("entries = %v, want %v", doc.Sections[0].Entries, want)
	}
}

// The two ways an id line can be wrong beyond the id itself: a grade that is not one of the
// four, and a third token. They are separate causes because the operator's fix differs.
func TestParseDocument_RejectsABadLevelAndAThirdToken(t *testing.T) {
	for name, tc := range map[string]struct {
		line  string
		cause string
	}{
		"unknown level":     {"openai/gpt-x legendary", modelcatalog.IssueUnknownLevel},
		"level cased wrong": {"openai/gpt-x Top", modelcatalog.IssueUnknownLevel},
		"three tokens":      {"openai/gpt-x top extra", modelcatalog.IssueMalformedLine},
		"bullet kept":       {"- openai/gpt-x", modelcatalog.IssueMalformedLine},
	} {
		t.Run(name, func(t *testing.T) {
			text := modelcatalog.DocumentVersionLine + "\n[writing]\n" + tc.line + "\n"
			_, issues := modelcatalog.ParseDocument(text)
			if len(issues) != 1 || issues[0].Cause != tc.cause {
				t.Fatalf("issues = %v, want one %s", issues, tc.cause)
			}
			if issues[0].Line != 3 || issues[0].Text != tc.line {
				t.Errorf("issue = %+v, want line 3 with the offending text", issues[0])
			}
		})
	}
}

// Listing one model twice stays ambiguous whatever the levels say: two lines disagreeing
// about the grade is exactly what a "last one wins" rule would hide.
func TestParseDocument_DuplicateIDIsRefusedEvenWithDifferentLevels(t *testing.T) {
	text := modelcatalog.DocumentVersionLine + "\n[writing]\n" +
		"openai/gpt-x top\nopenai/gpt-x value\n"
	_, issues := modelcatalog.ParseDocument(text)
	if len(issues) != 1 || issues[0].Cause != modelcatalog.IssueDuplicateID {
		t.Fatalf("issues = %v, want one duplicate_id", issues)
	}
}

// A set level renders as a second token, an unset one renders as nothing at all — the line
// an operator reads back is the line they would have written.
func TestRenderDocument_WritesTheLevelOnlyWhenSet(t *testing.T) {
	doc := modelcatalog.RenderDocument(map[modelcatalog.Purpose][]modelcatalog.DocumentEntry{
		modelcatalog.PurposeWriting: {
			{ModelID: "z-ai/glm-5.3", Level: modelcatalog.LevelBalanced},
			{ModelID: "anthropic/claude-sonnet-5"},
		},
	})
	if !strings.Contains(doc, "\nz-ai/glm-5.3 balanced\n") {
		t.Errorf("a levelled id renders as `<id> <level>`:\n%s", doc)
	}
	if !strings.Contains(doc, "\nanthropic/claude-sonnet-5\n") {
		t.Errorf("an unlevelled id renders bare, with no trailing token:\n%s", doc)
	}
}
