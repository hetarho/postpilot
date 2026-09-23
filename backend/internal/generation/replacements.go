package generation

import (
	"log/slog"
	"strings"
)

// ReplacementSurface is where a replacement candidate stands (GEN-53).
type ReplacementSurface string

const (
	ReplacementTitle ReplacementSurface = "title"
	ReplacementTag   ReplacementSurface = "tag"
	ReplacementBody  ReplacementSurface = "body"
)

// Replacement is one span the write pass offered listed 분야 phrases for: its surface, the tag
// or block index (0 for the title), the exact text written there, and the phrases offered.
type Replacement struct {
	Surface ReplacementSurface
	Index   int
	Source  string
	Phrases []string
}

// ValidateReplacements keeps the candidates the FINAL content still bears out, which is the
// content that is stored: an index names the tag or block at that position after the attachment
// filter and the slot pass, and is never re-pointed to follow a block that moved (GEN-53 —
// blocks carry no identity). A candidate survives only while its source stands at its place, by
// exact substring: a title containing it; `tags[index]` containing it; `blocks[index]` a TEXT that
// is not an unfilled slot, a HEADING or a QUOTE whose content contains it, or a LIST with an item
// containing it. An offered phrase must be one of the frozen list, differ from the source, and
// appear once; an entry left with none is dropped, and so is one repeating an earlier survivor's
// surface, index and source. Then the first ReplacementSpansMax entries and each one's first
// ReplacementPhrasesMax phrases survive, in model order (GEN-54), so an invalid entry never
// spends a place.
//
// These rules are the contract the browser mirrors to mark spans over the stored content: a
// change here is a change on both sides.
func ValidateReplacements(candidates []Replacement, content PostContent, phrases []string) []Replacement {
	listed := make(map[string]bool, len(phrases))
	for _, phrase := range phrases {
		listed[phrase] = true
	}
	type span struct {
		surface ReplacementSurface
		index   int
		source  string
	}
	seen := map[span]bool{}
	var kept []Replacement
	for _, candidate := range candidates {
		candidate.Source = strings.TrimSpace(candidate.Source)
		if candidate.Surface == ReplacementTitle {
			candidate.Index = 0
		}
		if reason := sourceMismatch(candidate, content); reason != "" {
			dropReplacement(candidate, reason)
			continue
		}
		candidate.Phrases = offeredPhrases(candidate, listed)
		if len(candidate.Phrases) == 0 {
			dropReplacement(candidate, "phrases")
			continue
		}
		key := span{candidate.Surface, candidate.Index, candidate.Source}
		if seen[key] {
			dropReplacement(candidate, "duplicate")
			continue
		}
		seen[key] = true
		if len(kept) == ReplacementSpansMax {
			dropReplacement(candidate, "cap")
			continue
		}
		if len(candidate.Phrases) > ReplacementPhrasesMax {
			candidate.Phrases = candidate.Phrases[:ReplacementPhrasesMax]
		}
		kept = append(kept, candidate)
	}
	return kept
}

// sourceMismatch names why a candidate's source does not stand at its place, or "" when it does.
func sourceMismatch(candidate Replacement, content PostContent) string {
	if candidate.Source == "" {
		return "source"
	}
	switch candidate.Surface {
	case ReplacementTitle:
		if !strings.Contains(content.Title, candidate.Source) {
			return "source"
		}
	case ReplacementTag:
		if candidate.Index < 0 || candidate.Index >= len(content.Tags) {
			return "index"
		}
		if !strings.Contains(content.Tags[candidate.Index], candidate.Source) {
			return "source"
		}
	case ReplacementBody:
		if candidate.Index < 0 || candidate.Index >= len(content.Blocks) {
			return "index"
		}
		block := content.Blocks[candidate.Index]
		switch {
		case block.Type == BlockText && block.Slot == nil, block.Type == BlockHeading, block.Type == BlockQuote:
			if !strings.Contains(block.Content, candidate.Source) {
				return "source"
			}
		case block.Type == BlockList:
			for _, item := range block.Items {
				if strings.Contains(item, candidate.Source) {
					return ""
				}
			}
			return "source"
		default:
			// A photo, a clip and an unfilled slot hold no prose to replace.
			return "block_type"
		}
	default:
		return "surface"
	}
	return ""
}

// offeredPhrases keeps the phrases that are listed, differ from the source and appear once.
func offeredPhrases(candidate Replacement, listed map[string]bool) []string {
	var out []string
	kept := map[string]bool{}
	for _, phrase := range candidate.Phrases {
		phrase = strings.TrimSpace(phrase)
		switch {
		case phrase == "" || !listed[phrase]:
			dropReplacementPhrase(candidate, "not_listed")
		case phrase == candidate.Source:
			dropReplacementPhrase(candidate, "is_source")
		case kept[phrase]:
			dropReplacementPhrase(candidate, "duplicate")
		default:
			kept[phrase] = true
			out = append(out, phrase)
		}
	}
	return out
}

// Neither log carries user text, as ValidateBlocks' does not.
func dropReplacement(candidate Replacement, reason string) {
	slog.Warn("dropping generated replacement candidate", "surface", candidate.Surface, "index", candidate.Index, "reason", reason)
}

func dropReplacementPhrase(candidate Replacement, reason string) {
	slog.Warn("dropping generated replacement phrase", "surface", candidate.Surface, "index", candidate.Index, "reason", reason)
}
