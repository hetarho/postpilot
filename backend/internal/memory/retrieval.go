package memory

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// Retrieval is deterministic and lexical, using the Go standard library alone: no vector, no
// embedding, no similarity and no third-party dependency (MEM-8). At this corpus size — an
// account holds at most MemoryMaxPerAccount facts, each with at most MemoryTagsMax tags —
// tag overlap answers the only question asked of it, and an embedding call per generation
// would cost credits for no measurable gain.
//
// TextsForPost is the context's published behaviour for prompt builders: the selected texts
// in injection order. The caller hands over the post's own words — its memo, its 가제, its
// template answers and the `objects` and `visible_text` of its observations (MEM-7) — and
// receives texts. A memory row never leaves this package.
func (s *Service) TextsForPost(ctx context.Context, userID string, keyParts []string) ([]string, error) {
	memories, err := s.store.List(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("load memories for retrieval: %w", err)
	}
	selected := Select(memories, keyParts, s.limits.InjectMax)
	texts := make([]string, 0, len(selected))
	for _, m := range selected {
		texts = append(texts, m.Text)
	}
	return texts, nil
}

// Select is the retrieval rule itself, over memories the caller already holds in the order
// List returns them — most recently used first, then most recently created. It is a pure
// function so the rule is testable without a database, and so the two tie-breaks are the
// list's own order rather than a second sort that could disagree with it.
//
// MEM-6's split: `preference` and `persona` are candidates for every post regardless of
// tags, because a standing fact about the author is what lets a post leave the frame;
// `place`, `person` and `history` need a tag to match, because an unfiltered place or
// person fact staples an unrelated shop to the next post.
//
// The score is the number of DISTINCT tags that matched — nothing is weighted, nothing is
// scored by a model (MEM-4). Ties fall back to the incoming order, which is last use then
// creation, both descending (MEM-7).
func Select(memories []Memory, keyParts []string, max int) []Memory {
	if max <= 0 || len(memories) == 0 {
		return nil
	}
	key := foldKey(keyParts)
	type scored struct {
		memory Memory
		score  int
		order  int
	}
	candidates := make([]scored, 0, len(memories))
	for i, m := range memories {
		matches := 0
		seen := make(map[string]struct{}, len(m.Tags))
		for _, tag := range m.Tags {
			folded := fold(tag)
			if folded == "" {
				continue
			}
			if _, already := seen[folded]; already {
				continue
			}
			seen[folded] = struct{}{}
			if strings.Contains(key, folded) {
				matches++
			}
		}
		if matches == 0 && !m.Kind.AlwaysCandidate() {
			continue
		}
		candidates = append(candidates, scored{memory: m, score: matches, order: i})
	}
	// Stable on the incoming order by construction: the comparison falls through to it.
	sort.Slice(candidates, func(a, b int) bool {
		if candidates[a].score != candidates[b].score {
			return candidates[a].score > candidates[b].score
		}
		return candidates[a].order < candidates[b].order
	})
	if len(candidates) > max {
		candidates = candidates[:max]
	}
	out := make([]Memory, 0, len(candidates))
	for _, c := range candidates {
		out = append(out, c.memory)
	}
	return out
}

// foldKey joins the post's own words into one folded string to match tags against.
func foldKey(parts []string) string {
	folded := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := fold(part); value != "" {
			folded = append(folded, value)
		}
	}
	// A separator between the parts, so a tag cannot match across the seam between the memo
	// and a template answer and claim a word neither of them contains.
	return strings.Join(folded, " ")
}

// fold is the whole of the matching's normalization: Unicode lower-casing, and every run of
// whitespace or punctuation collapsed to a single space. Nothing else — no stemming, no
// morphological analysis, no transliteration (MEM-8).
//
// A tag matches when it occurs as a SUBSTRING of the folded key rather than as a whole
// token. That is deliberate and it is the reason there is no analyser here: Korean is
// agglutinative, so `연남동` has to match `연남동에서` and token equality would miss every
// inflected occurrence. A short tag can over-match in return — `물` inside `물건` — which is
// the accepted cost, and the reason MemoryTagsMax is small and the user writes the tags.
func fold(value string) string {
	var out strings.Builder
	out.Grow(len(value))
	space := false
	for _, r := range value {
		switch {
		case unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r):
			space = true
		default:
			if space && out.Len() > 0 {
				out.WriteRune(' ')
			}
			space = false
			out.WriteRune(unicode.ToLower(r))
		}
	}
	return out.String()
}
