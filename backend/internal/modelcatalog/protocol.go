package modelcatalog

import (
	"slices"
	"strings"
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
	// Validation causes: the text parsed, but the catalog refuses it.
	IssueUnknownModel = "unknown_model"
	IssueUnlisted     = "unlisted_model"
	IssueIneligible   = "purpose_ineligible"
)

// DocumentIssue is one rejected line. The line number is 1-based over the document as
// pasted, so the operator can find it without counting the lines the parser ignored.
type DocumentIssue struct {
	Line  int
	Text  string
	Cause string
}

// DocumentSection is one purpose and the ids listed under it, in the order they appear.
type DocumentSection struct {
	Purpose Purpose
	// Line is where the header sits, so a validation issue about the section itself can
	// point at something.
	Line     int
	ModelIDs []string
	// Lines[i] is the document line ModelIDs[i] came from.
	Lines []int
}

// Document is a parsed paste. Only the purposes it actually names are present: a purpose
// with no section is not "empty", it is untouched (MODEL-52).
type Document struct {
	Sections []DocumentSection
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
		if !looksLikeModelID(line) {
			// A pasted table row, a bullet, a quoted id: anything that is not bare is
			// refused rather than guessed at.
			issues = append(issues, DocumentIssue{Line: number, Text: line, Cause: IssueMalformedLine})
			continue
		}
		if current < 0 {
			issues = append(issues, DocumentIssue{Line: number, Text: line, Cause: IssueOrphanID})
			continue
		}
		if seenID[current][line] {
			issues = append(issues, DocumentIssue{Line: number, Text: line, Cause: IssueDuplicateID})
			continue
		}
		seenID[current][line] = true
		doc.Sections[current].ModelIDs = append(doc.Sections[current].ModelIDs, line)
		doc.Sections[current].Lines = append(doc.Sections[current].Lines, number)
	}

	if !versioned {
		// An empty or all-blank paste is the same mistake as a wrong header.
		issues = append(issues, DocumentIssue{Line: 1, Cause: IssueBadVersion})
	}
	return doc, issues
}

// looksLikeModelID keeps the parser strict: a source id is one bare token. Whitespace is
// what separates a real id from the table cell, bullet or numbered line somebody pasted
// around it.
func looksLikeModelID(line string) bool {
	if line == "" || strings.ContainsAny(line, " \t|`\"',") {
		return false
	}
	return !strings.ContainsAny(line, "[]")
}

// RenderDocument writes the protocol for the registrations given (MODEL-55). Every purpose
// gets a section, including the empty ones, so what comes out is a complete statement of
// the catalog rather than a partial edit — and pasting it straight back is no change.
//
// The output is byte-stable: fixed purpose order, ids sorted within a section, one trailing
// newline, and no comment lines beyond the version. Nothing derived from prices or labels
// is written, because a comment that goes stale is worse than no comment.
func RenderDocument(registrations map[Purpose][]string) string {
	var b strings.Builder
	b.WriteString(DocumentVersionLine)
	b.WriteString("\n")
	for _, purpose := range Purposes {
		b.WriteString("\n[")
		b.WriteString(string(purpose))
		b.WriteString("]\n")
		ids := slices.Clone(registrations[purpose])
		slices.Sort(ids)
		for _, id := range ids {
			b.WriteString(id)
			b.WriteString("\n")
		}
	}
	return b.String()
}
