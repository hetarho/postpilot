package modelcatalog

import (
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/postpilot/backend/internal/llm"
)

// The paste protocol (MODEL-51) is one plain-text document the operator hand-edits: a
// version line, `[purpose]` section headers, and one model id per line. It exists because
// curating a recommended set one checkbox at a time is dozens of round trips, and because
// the list an operator arrives with is already a list.
//
// The format is deliberately forgiving about whitespace and unforgiving about everything
// else: a stray space must not change what gets registered, while a pasted table row must
// not be mistaken for a model id.
const DocumentVersionLine = "# postpilot models v1"

// Causes a document line can be rejected for. They cross the wire as strings and are
// rendered by the operator surface's own copy — this is master-only admin detail, not one
// of the normalized user-facing failure reasons the frontend catalog carries.
const (
	// Parse-time causes: the text alone is wrong.
	IssueBadVersion       = "bad_version"
	IssueUnknownPurpose   = "unknown_purpose"
	IssueDuplicateSection = "duplicate_section"
	IssueOrphanID         = "id_before_section"
	IssueMalformedLine    = "malformed_line"
	IssueDuplicateID      = "duplicate_id"
	IssueUnknownLevel     = "unknown_level"
	// The `[recommendations]` section's own grammar (MODEL-72, MODEL-73).
	IssueSetLabel       = "set_label_invalid"
	IssueDuplicateSet   = "duplicate_set"
	IssueSetLimit       = "set_limit"
	IssueOrphanStage    = "stage_before_set"
	IssueDuplicateStage = "duplicate_stage"
	IssueMissingStage   = "missing_stage"
	// Validation causes: the text parsed, but the catalog refuses it.
	IssueUnknownModel   = "unknown_model"
	IssueUnlisted       = "unlisted_model"
	IssueIneligible     = "purpose_ineligible"
	IssueFreeIneligible = "free_path_ineligible"
	// A set slot the document's own registrations do not support (MODEL-73).
	IssueSlotUnregistered = "slot_unregistered"
	IssueSlotUnclassified = "slot_unclassified"
	IssueSlotDuplicate    = "slot_duplicate"
)

// RecommendationsHeader names the one section that is not a purpose (MODEL-72).
const RecommendationsHeader = "recommendations"

// The bounds a document's sets share with every other save (MODEL-69). They mirror the
// provider context's constants: this context does not import that one.
const (
	MaxDocumentSets          = 10
	MaxDocumentSetLabelRunes = 60
)

// setStages are the stage lines a set carries, in the order they are written and exported,
// with the number of ids each takes: the active model, then the A/B pair where the stage
// keeps one (MODEL-23).
var setStages = []struct {
	name string
	ids  int
}{
	{llm.StageNameObserve, 3},
	{llm.StageNameAnalyze, 1},
	{llm.StageNameWrite, 3},
}

func setStageIDs(stage string) (int, bool) {
	for _, candidate := range setStages {
		if candidate.name == stage {
			return candidate.ids, true
		}
	}
	return 0, false
}

// DocumentIssue is one rejected line. The line number is 1-based over the document as
// pasted, so the operator can find it without counting the lines the parser ignored.
type DocumentIssue struct {
	Line  int
	Text  string
	Cause string
}

// DocumentEntry is one id line: the model, the level it named, and where it sat.
type DocumentEntry struct {
	ModelID string
	// Level is what the line's second token said, or "" when it carried none. An id-only
	// line is a real instruction to leave the registration unlevelled (MODEL-59), not a
	// missing value to be filled in from what is stored.
	Level Level
	// Line is the 1-based document line, so an issue about this entry can point at it.
	Line int
}

// DocumentSection is one purpose and the entries listed under it, in the order they appear.
type DocumentSection struct {
	Purpose Purpose
	// Line is where the header sits, so a validation issue about the section itself can
	// point at something.
	Line    int
	Entries []DocumentEntry
}

// DocumentStage is one stage line of a set: the stage and its ids in slot order.
type DocumentStage struct {
	Stage string
	IDs   []string
	Line  int
}

// DocumentSet is one `set <label>` block and the stage lines under it.
type DocumentSet struct {
	Label  string
	Line   int
	Stages []DocumentStage
}

// Stage returns the set's line for one stage, if it has one.
func (s DocumentSet) Stage(stage string) (DocumentStage, bool) {
	for _, line := range s.Stages {
		if line.Stage == stage {
			return line, true
		}
	}
	return DocumentStage{}, false
}

// Document is a parsed paste. Only the purposes it actually names are present: a purpose
// with no section is not "empty", it is untouched (MODEL-52). Recommendations is nil when the
// document has no `[recommendations]` section — the sets are untouched — and a non-nil empty
// list when the section is there with no set in it, which removes every set (MODEL-72).
type Document struct {
	Sections        []DocumentSection
	Recommendations *[]DocumentSet
}

// ParseDocument reads the protocol. It returns everything it could parse together with
// every line it refused, because the operator fixes a paste by seeing all of its problems
// at once rather than one per attempt. A document with any issue is never applied
// (MODEL-53) — that decision belongs to the caller, which also has the catalog to check
// against.
func ParseDocument(text string) (Document, []DocumentIssue) {
	var (
		doc    Document
		issues []DocumentIssue
	)
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")

	versioned := false
	current := -1
	seenPurpose := map[Purpose]bool{}
	seenID := []map[string]bool{}
	// Inside the recommendations section, `set` opens the block that stage lines belong to.
	// A set the parser refused (bad label, repeated label, past the limit) still opens a
	// block, detached from the document, so its own stage lines are not reported a second
	// time as orphans.
	inRecommendations := false
	var openSet *DocumentSet
	openSetKept := false
	seenLabel := map[string]bool{}
	closeSet := func() {
		if openSet == nil {
			return
		}
		if openSetKept {
			for _, stage := range setStages {
				if _, ok := openSet.Stage(stage.name); !ok {
					issues = append(issues, DocumentIssue{Line: openSet.Line, Text: stage.name, Cause: IssueMissingStage})
				}
			}
			*doc.Recommendations = append(*doc.Recommendations, *openSet)
		}
		openSet, openSetKept = nil, false
	}

	for i, raw := range lines {
		number := i + 1
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if !versioned {
			// The version line must be the first thing that is not blank — a document whose
			// header the operator dropped is refused whole rather than half-read.
			if line != DocumentVersionLine {
				issues = append(issues, DocumentIssue{Line: number, Text: line, Cause: IssueBadVersion})
				return doc, issues
			}
			versioned = true
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			name, ok := strings.CutSuffix(strings.TrimPrefix(line, "["), "]")
			if !ok {
				issues = append(issues, DocumentIssue{Line: number, Text: line, Cause: IssueMalformedLine})
				continue
			}
			closeSet()
			if strings.TrimSpace(name) == RecommendationsHeader {
				if doc.Recommendations != nil {
					issues = append(issues, DocumentIssue{Line: number, Text: line, Cause: IssueDuplicateSection})
					continue
				}
				doc.Recommendations = &[]DocumentSet{}
				inRecommendations, current = true, -1
				continue
			}
			inRecommendations = false
			purpose, err := ParsePurpose(strings.TrimSpace(name))
			if err != nil {
				issues = append(issues, DocumentIssue{Line: number, Text: line, Cause: IssueUnknownPurpose})
				continue
			}
			if seenPurpose[purpose] {
				// Two sections for one purpose have no meaning under MODEL-52: each is
				// supposed to be that purpose's complete membership.
				issues = append(issues, DocumentIssue{Line: number, Text: line, Cause: IssueDuplicateSection})
				continue
			}
			seenPurpose[purpose] = true
			doc.Sections = append(doc.Sections, DocumentSection{Purpose: purpose, Line: number})
			seenID = append(seenID, map[string]bool{})
			current = len(doc.Sections) - 1
			continue
		}
		if inRecommendations {
			fields := strings.Fields(line)
			switch {
			case fields[0] == "set":
				closeSet()
				label := strings.TrimSpace(strings.TrimPrefix(line, "set"))
				openSet = &DocumentSet{Label: label, Line: number}
				switch count := utf8.RuneCountInString(label); {
				case count == 0 || count > MaxDocumentSetLabelRunes:
					issues = append(issues, DocumentIssue{Line: number, Text: line, Cause: IssueSetLabel})
				case seenLabel[label]:
					issues = append(issues, DocumentIssue{Line: number, Text: line, Cause: IssueDuplicateSet})
				case len(*doc.Recommendations) >= MaxDocumentSets:
					issues = append(issues, DocumentIssue{Line: number, Text: line, Cause: IssueSetLimit})
				default:
					seenLabel[label] = true
					openSetKept = true
				}
			default:
				want, isStage := setStageIDs(fields[0])
				ids := fields[1:]
				malformed := !isStage || len(ids) != want
				for _, id := range ids {
					malformed = malformed || !looksLikeModelID(id)
				}
				switch {
				case malformed:
					issues = append(issues, DocumentIssue{Line: number, Text: line, Cause: IssueMalformedLine})
				case openSet == nil:
					issues = append(issues, DocumentIssue{Line: number, Text: line, Cause: IssueOrphanStage})
				default:
					if _, repeated := openSet.Stage(fields[0]); repeated {
						issues = append(issues, DocumentIssue{Line: number, Text: line, Cause: IssueDuplicateStage})
						continue
					}
					openSet.Stages = append(openSet.Stages, DocumentStage{Stage: fields[0], IDs: ids, Line: number})
				}
			}
			continue
		}
		// An id line is one or two tokens: the model, and optionally its level (MODEL-59).
		// Fields() rather than Cut() so any run of spaces or a tab separates them — the
		// format stays forgiving about whitespace and strict about everything else.
		fields := strings.Fields(line)
		if len(fields) > 2 || !looksLikeModelID(fields[0]) {
			// A pasted table row, a bullet, a quoted id, a third token: anything that is not
			// the grammar is refused rather than guessed at.
			issues = append(issues, DocumentIssue{Line: number, Text: line, Cause: IssueMalformedLine})
			continue
		}
		modelID := fields[0]
		var level Level
		if len(fields) == 2 {
			parsed, err := ParseLevel(fields[1])
			if err != nil {
				// The id is fine and only the grade is wrong, so the operator is told which
				// half to fix rather than being sent to look at the whole line.
				issues = append(issues, DocumentIssue{Line: number, Text: line, Cause: IssueUnknownLevel})
				continue
			}
			level = parsed
		}
		if current < 0 {
			issues = append(issues, DocumentIssue{Line: number, Text: line, Cause: IssueOrphanID})
			continue
		}
		if seenID[current][modelID] {
			// Listing one model twice is ambiguous whatever the levels say — two lines
			// disagreeing about the grade is exactly the case a "last wins" rule would hide.
			issues = append(issues, DocumentIssue{Line: number, Text: line, Cause: IssueDuplicateID})
			continue
		}
		seenID[current][modelID] = true
		doc.Sections[current].Entries = append(doc.Sections[current].Entries, DocumentEntry{
			ModelID: modelID, Level: level, Line: number,
		})
	}

	closeSet()
	if !versioned {
		// An empty or all-blank paste is the same mistake as a wrong header.
		issues = append(issues, DocumentIssue{Line: 1, Cause: IssueBadVersion})
	}
	return doc, issues
}

// looksLikeModelID keeps the parser strict: a source id is one bare `vendor/model` token.
// The caller has already split the line on whitespace, so what is left to refuse is the
// punctuation a table cell or quoted id drags along — and anything with no slug in it.
//
// The slash requirement earns its keep now that a line may carry two tokens (MODEL-59):
// without it a markdown bullet reads as the id `-` followed by an unknown level, which
// sends the operator to fix the wrong half of a line whose real problem is the bullet.
// Every id the source publishes is `vendor/model` (MODEL-18 takes the segment before the
// slash as the grouping key), so nothing legitimate is turned away.
func looksLikeModelID(token string) bool {
	if token == "" || strings.ContainsAny(token, " \t|`\"',") || strings.ContainsAny(token, "[]") {
		return false
	}
	slug, rest, ok := strings.Cut(token, "/")
	return ok && slug != "" && rest != ""
}

// RenderDocument writes the protocol for the registrations and recommendation sets given
// (MODEL-55, MODEL-72). Every purpose gets a section, including the empty ones, and the
// recommendations section comes last — empty when there is no set — so what comes out is a
// complete statement of the catalog rather than a partial edit, and pasting it straight back
// is no change.
//
// The output is byte-stable: fixed purpose order, ids sorted within a section, the sets in
// their stored order, one trailing newline, and no comment lines beyond the version. Nothing
// derived from prices or labels is written, because a comment that goes stale is worse than
// no comment.
func RenderDocument(registrations map[Purpose][]DocumentEntry, sets []StoredSet) string {
	var b strings.Builder
	b.WriteString(DocumentVersionLine)
	b.WriteString("\n")
	for _, purpose := range Purposes {
		b.WriteString("\n[")
		b.WriteString(string(purpose))
		b.WriteString("]\n")
		entries := slices.Clone(registrations[purpose])
		slices.SortFunc(entries, func(a, b DocumentEntry) int { return strings.Compare(a.ModelID, b.ModelID) })
		for _, entry := range entries {
			b.WriteString(entry.ModelID)
			// An unset level writes NO second token rather than an empty one, so the line an
			// operator reads back is the line they would have written themselves.
			if entry.Level != "" {
				b.WriteString(" ")
				b.WriteString(string(entry.Level))
			}
			b.WriteString("\n")
		}
	}
	b.WriteString("\n[")
	b.WriteString(RecommendationsHeader)
	b.WriteString("]\n")
	for index, set := range sets {
		if index > 0 {
			b.WriteString("\n")
		}
		b.WriteString("set ")
		b.WriteString(set.Label)
		b.WriteString("\n")
		for _, line := range [][]string{
			{llm.StageNameObserve, set.Observe[0], set.Observe[1], set.Observe[2]},
			{llm.StageNameAnalyze, set.Analyze},
			{llm.StageNameWrite, set.Write[0], set.Write[1], set.Write[2]},
		} {
			b.WriteString(strings.Join(line, " "))
			b.WriteString("\n")
		}
	}
	return b.String()
}
