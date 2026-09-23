package quality

import (
	"sort"
	"strings"
)

// Phrase is one recurring run of words in a field's top results, with how many titles or
// descriptions carry it.
type Phrase struct {
	Text   string
	Tokens int
	Count  int
}

// ExtractPhrases is the one implementation of QUAL-38's phrase list: every run of
// PhraseMinTokens to PhraseMaxTokens words, counted once per unit (one title or one
// description), with a run made only of stopwords never counted. A run is dropped when a longer
// run containing it has the same count, since the longer one already says it. The list is
// ranked by count, then by fewer words, then lexicographically, and cut at PhraseListMax.
func ExtractPhrases(units []string) []Phrase {
	counts := map[string]int{}
	lengths := map[string]int{}
	for _, unit := range units {
		seen := map[string]bool{}
		for _, segment := range segments(unit) {
			for start := range segment {
				for n := PhraseMinTokens; n <= PhraseMaxTokens && start+n <= len(segment); n++ {
					run := segment[start : start+n]
					if allStopwords(run) {
						continue
					}
					key := strings.Join(run, " ")
					if seen[key] {
						continue
					}
					seen[key] = true
					counts[key]++
					lengths[key] = n
				}
			}
		}
	}

	dropped := map[string]bool{}
	for key, count := range counts {
		// Tokens hold no whitespace, so splitting the key recovers them exactly.
		tokens := strings.Split(key, " ")
		for n := PhraseMinTokens; n < len(tokens); n++ {
			for start := 0; start+n <= len(tokens); start++ {
				if sub := strings.Join(tokens[start:start+n], " "); counts[sub] == count {
					dropped[sub] = true
				}
			}
		}
	}

	phrases := make([]Phrase, 0, len(counts))
	for key, count := range counts {
		if !dropped[key] {
			phrases = append(phrases, Phrase{Text: key, Tokens: lengths[key], Count: count})
		}
	}
	sort.Slice(phrases, func(i, j int) bool {
		a, b := phrases[i], phrases[j]
		switch {
		case a.Count != b.Count:
			return a.Count > b.Count
		case a.Tokens != b.Tokens:
			return a.Tokens < b.Tokens
		default:
			return a.Text < b.Text
		}
	})
	if len(phrases) > PhraseListMax {
		phrases = phrases[:PhraseListMax]
	}
	return phrases
}

// segments splits one unit into the stretches a run may span. Each whitespace field goes through
// the 어절 tokenizer, and one that trims to nothing — a lone |, - or … — ends the stretch, so
// "성수 카페 | 분위기 좋은" never yields "카페 분위기".
func segments(unit string) [][]string {
	var out [][]string
	var current []string
	for _, field := range strings.Fields(unit) {
		tokens := Tokens(field)
		if len(tokens) == 0 {
			if len(current) > 0 {
				out = append(out, current)
			}
			current = nil
			continue
		}
		current = append(current, tokens...)
	}
	if len(current) > 0 {
		out = append(out, current)
	}
	return out
}

func allStopwords(run []string) bool {
	for _, token := range run {
		if !isStopword(token) {
			return false
		}
	}
	return true
}
