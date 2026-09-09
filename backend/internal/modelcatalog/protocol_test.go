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
	want := []string{"z-ai/glm-5.3-flash", "google/gemini-3.8-flash"}
	if !reflect.DeepEqual(doc.Sections[0].ModelIDs, want) {
		t.Errorf("ids = %v, want %v (surrounding whitespace is trimmed)", doc.Sections[0].ModelIDs, want)
	}
	// Line numbers are over the document as pasted, blank and comment lines included.
	if got := doc.Sections[0].Lines; !reflect.DeepEqual(got, []int{6, 7}) {
		t.Errorf("lines = %v, want [6 7]", got)
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
	doc := modelcatalog.RenderDocument(map[modelcatalog.Purpose][]string{
		modelcatalog.PurposeWriting:       {"z-ai/glm-5.3", "anthropic/claude-sonnet-5"},
		modelcatalog.PurposePhotoAnalysis: {"google/gemini-3.8-flash"},
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
	if doc != modelcatalog.RenderDocument(map[modelcatalog.Purpose][]string{
		modelcatalog.PurposeWriting:       {"anthropic/claude-sonnet-5", "z-ai/glm-5.3"},
		modelcatalog.PurposePhotoAnalysis: {"google/gemini-3.8-flash"},
	}) {
		t.Error("input order must not change the rendered bytes")
	}
}

func TestRenderedDocumentParsesBackToWhatWentIn(t *testing.T) {
	in := map[modelcatalog.Purpose][]string{
		modelcatalog.PurposeWriting:       {"z-ai/glm-5.3", "anthropic/claude-sonnet-5"},
		modelcatalog.PurposePhotoAnalysis: {"google/gemini-3.8-flash"},
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
		slices.Sort(want)
		got := slices.Clone(section.ModelIDs)
		if len(want) == 0 && len(got) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s = %v, want %v", section.Purpose, got, want)
		}
	}
}
