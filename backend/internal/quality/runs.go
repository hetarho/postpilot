package quality

import (
	"sort"
	"strings"
	"unicode/utf8"
)

// Run is one stretch of a post's tokens that also stands verbatim, inside one text unit, in
// other documents: its tokens, where it starts in the post's token sequence, and the index into
// `others` of every document that holds it whole, ascending.
type Run struct {
	Tokens []string
	Start  int
	In     []int
}

// Overlap is a post's M2 reading against the documents it was compared with (QUAL-8): the
// share of its runes standing inside a matched window, and the maximal runs those windows form.
type Overlap struct {
	Share float64
	Runs  []Run
}

// MatchRuns finds where post repeats `length` or more consecutive 어절 of other documents.
//
// A document is its Units' tokens in order, and a window is `length` consecutive tokens inside
// ONE unit: a stretch that straddles two blocks or two LIST items is not text that appears
// verbatim (QUAL-8). Each document is indexed once by window key; the key is the window's exact
// tokens, so the map is the hash and no collision can reach the answer.
//
// The share counts the runes of every post token some matched window covers, over the runes of
// all its tokens — trimmed 어절, so whitespace and edge punctuation never count. A run chains
// matched windows that follow each other in both documents; runs are deduplicated by their
// tokens, keeping the earliest, and each names every other document holding it whole.
//
// It answers false with no other document or fewer than `length` post tokens, where the share
// cannot be computed (QUAL-40); a post with enough tokens and no match is a real zero.
func MatchRuns(post Document, others []Document, length int) (Overlap, bool) {
	if length < 1 || len(others) == 0 {
		return Overlap{}, false
	}
	own := indexSequence(post, length)
	if len(own.tokens) < length {
		return Overlap{}, false
	}
	indexed := make([]sequence, len(others))
	for i, other := range others {
		indexed[i] = indexSequence(other, length)
	}

	covered := make([]bool, len(own.tokens))
	for w, key := range own.keys {
		for _, other := range indexed {
			if _, ok := other.windows[key]; ok {
				for t := own.starts[w]; t < own.starts[w]+length; t++ {
					covered[t] = true
				}
				break
			}
		}
	}
	var coveredRunes, allRunes int
	for t, token := range own.tokens {
		runes := utf8.RuneCountInString(token)
		allRunes += runes
		if covered[t] {
			coveredRunes += runes
		}
	}

	found := map[string]Run{}
	for _, other := range indexed {
		for _, run := range chainedRuns(own, other, length) {
			key := windowKey(run.Tokens)
			if earlier, ok := found[key]; !ok || run.Start < earlier.Start {
				found[key] = run
			}
		}
	}
	runs := make([]Run, 0, len(found))
	for _, run := range found {
		for i, other := range indexed {
			if other.holds(run.Tokens, length) {
				run.In = append(run.In, i)
			}
		}
		runs = append(runs, run)
	}
	sort.Slice(runs, func(a, b int) bool {
		if runs[a].Start != runs[b].Start {
			return runs[a].Start < runs[b].Start
		}
		return len(runs[a].Tokens) > len(runs[b].Tokens)
	})
	return Overlap{Share: float64(coveredRunes) / float64(allRunes), Runs: runs}, true
}

// sequence is one document's tokens with its windows: starts and keys in position order, the
// same windows by key, and the unit each token belongs to.
type sequence struct {
	tokens  []string
	unit    []int
	starts  []int
	keys    []string
	windows map[string][]int
}

func indexSequence(doc Document, length int) sequence {
	seq := sequence{windows: map[string][]int{}}
	for u, unit := range Units(doc) {
		tokens := Tokens(unit.Text)
		offset := len(seq.tokens)
		for range tokens {
			seq.unit = append(seq.unit, u)
		}
		seq.tokens = append(seq.tokens, tokens...)
		for i := 0; i+length <= len(tokens); i++ {
			key := windowKey(tokens[i : i+length])
			seq.starts = append(seq.starts, offset+i)
			seq.keys = append(seq.keys, key)
			seq.windows[key] = append(seq.windows[key], offset+i)
		}
	}
	return seq
}

// chainedRuns pairs each post window with every equal window of other, then walks each chain of
// pairs that advance together in both documents. A chain covers tokens [j, j+m+length) of the
// post and is the verbatim common run the two share there.
func chainedRuns(post, other sequence, length int) []Run {
	type pair struct{ post, other int }
	matched := map[pair]bool{}
	var pairs []pair
	for w, key := range post.keys {
		for _, at := range other.windows[key] {
			p := pair{post.starts[w], at}
			matched[p] = true
			pairs = append(pairs, p)
		}
	}
	var runs []Run
	for _, p := range pairs {
		if matched[pair{p.post - 1, p.other - 1}] {
			continue // not the start of its chain
		}
		m := 0
		for matched[pair{p.post + m + 1, p.other + m + 1}] {
			m++
		}
		tokens := append([]string(nil), post.tokens[p.post:p.post+m+length]...)
		runs = append(runs, Run{Tokens: tokens, Start: p.post})
	}
	return runs
}

// holds reports whether the sequence carries tokens whole inside one unit, looked up from its
// index of their first window.
func (s sequence) holds(tokens []string, length int) bool {
	for _, at := range s.windows[windowKey(tokens[:length])] {
		end := at + len(tokens)
		if end > len(s.tokens) || s.unit[at] != s.unit[end-1] {
			continue
		}
		if windowKey(s.tokens[at:end]) == windowKey(tokens) {
			return true
		}
	}
	return false
}

func windowKey(tokens []string) string { return strings.Join(tokens, "\x00") }
