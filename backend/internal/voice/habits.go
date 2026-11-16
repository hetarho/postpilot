package voice

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// HabitKind names the countable habit a found phrase shows (VOICE-77).
type HabitKind string

const (
	HabitEnding    HabitKind = "ending"
	HabitAdverb    HabitKind = "adverb"
	HabitOpening   HabitKind = "opening"
	HabitClosing   HabitKind = "closing"
	HabitSignature HabitKind = "signature"
)

// HabitPhrase is one phrase of a post that shows a habit of its voice. Start and End are
// half-open Unicode scalar offsets into the text of the block (ItemIndex -1) or, for a LIST, of
// the item at ItemIndex, the way an origin span is located. VoicePerHundred is 0 where the habit
// has no rate; Example is empty when no 학습 글 sentence is left to show.
type HabitPhrase struct {
	BlockIndex       int
	ItemIndex        int
	Start, End       int
	Quote            string
	Kind             HabitKind
	Term             string
	VoicePerHundred  float64
	TextCount        int
	Example          string
	ExampleFictional bool
}

// habitField is one searched string of a post: a TEXT or QUOTE block's content, or one LIST item.
type habitField struct {
	block, item int
	runes       []rune
}

// habitLine is a prose line of a field, by rune offsets.
type habitLine struct {
	field      *habitField
	start, end int
	text       string
}

// FindHabitPhrases returns every phrase of blocks that shows a habit the analysis counted or the
// AI part named (VOICE-77), by matching in code alone. Only TEXT and QUOTE content and LIST items
// are searched, and only their prose lines (VOICE-61). Spans never overlap: the longer wins, then
// the earlier. present holds the ids of the voice's current 학습 글, which gate each example.
func FindHabitPhrases(analysis Analysis, blocks []Block, present map[string]bool) []HabitPhrase {
	lines := habitLines(blocks)
	var found []HabitPhrase
	found = append(found, signaturePhrases(analysis, lines)...)
	found = append(found, openClosePhrases(analysis, lines)...)
	found = append(found, endingPhrases(analysis, lines)...)
	found = append(found, adverbPhrases(analysis, lines)...)

	counts := map[string]int{}
	for _, phrase := range found {
		counts[habitKey(phrase.Kind, phrase.Term)]++
	}
	found = withoutOverlaps(found)
	examples := map[string]habitExample{}
	for i := range found {
		key := habitKey(found[i].Kind, found[i].Term)
		found[i].TextCount = counts[key]
		example, ok := examples[key]
		if !ok {
			example = exampleFor(analysis, found[i].Kind, found[i].Term, present)
			examples[key] = example
		}
		found[i].Example, found[i].ExampleFictional = example.sentence, example.fictional
	}
	sort.SliceStable(found, func(i, j int) bool {
		a, b := found[i], found[j]
		if a.BlockIndex != b.BlockIndex {
			return a.BlockIndex < b.BlockIndex
		}
		if a.ItemIndex != b.ItemIndex {
			return a.ItemIndex < b.ItemIndex
		}
		return a.Start < b.Start
	})
	return found
}

func habitKey(kind HabitKind, term string) string { return string(kind) + "\x00" + term }

// habitLines reads a post's searched fields as prose lines, in block order.
func habitLines(blocks []Block) []habitLine {
	var out []habitLine
	add := func(field *habitField) {
		start := 0
		for i := 0; i <= len(field.runes); i++ {
			if i < len(field.runes) && field.runes[i] != '\n' {
				continue
			}
			from, to := start, i
			start = i + 1
			for from < to && unicode.IsSpace(field.runes[from]) {
				from++
			}
			for to > from && unicode.IsSpace(field.runes[to-1]) {
				to--
			}
			if text := string(field.runes[from:to]); isProse(text) {
				out = append(out, habitLine{field: field, start: from, end: to, text: text})
			}
		}
	}
	for _, block := range blocks {
		switch block.Type {
		case BlockText, BlockQuote:
			add(&habitField{block: block.Index, item: -1, runes: []rune(block.Content)})
		case BlockList:
			for item, text := range block.Items {
				add(&habitField{block: block.Index, item: item, runes: []rune(text)})
			}
		}
	}
	return out
}

func (l habitLine) phrase(start, end int, kind HabitKind, term string, rate float64) HabitPhrase {
	return HabitPhrase{
		BlockIndex: l.field.block, ItemIndex: l.field.item, Start: start, End: end,
		Quote: string(l.field.runes[start:end]), Kind: kind, Term: term, VoicePerHundred: rate,
	}
}

// runeOffset converts a byte offset into the line's text to a rune offset in its field.
func (l habitLine) runeOffset(byteOffset int) int {
	return l.start + utf8.RuneCountInString(l.text[:byteOffset])
}

// sentenceSpans locates each of the line's sentences, by rune offsets into its field.
func (l habitLine) sentenceSpans() [][2]int {
	var spans [][2]int
	cursor := 0
	for _, sentence := range SegmentSentences(l.text) {
		at := strings.Index(l.text[cursor:], sentence)
		if at < 0 {
			continue
		}
		byteStart := cursor + at
		cursor = byteStart + len(sentence)
		spans = append(spans, [2]int{l.runeOffset(byteStart), l.runeOffset(cursor)})
	}
	return spans
}

func signaturePhrases(analysis Analysis, lines []habitLine) []HabitPhrase {
	var out []HabitPhrase
	for _, term := range analysis.AI.SignaturePhrases {
		if term == "" {
			continue
		}
		for _, l := range lines {
			for from := 0; from < len(l.text); {
				at := strings.Index(l.text[from:], term)
				if at < 0 {
					break
				}
				start := l.runeOffset(from + at)
				out = append(out, l.phrase(start, start+utf8.RuneCountInString(term), HabitSignature, term, 0))
				from += at + len(term)
			}
		}
	}
	return out
}

// openClosePhrases is ⑤: the post's first and last prose line, when it begins like one of the
// voice's recurring lines, spans that line's first and last sentence.
func openClosePhrases(analysis Analysis, lines []habitLine) []HabitPhrase {
	if len(lines) == 0 {
		return nil
	}
	var out []HabitPhrase
	match := func(l habitLine, voiceLines []string, last bool, kind HabitKind) {
		want := firstSyllables(l.text)
		spans := l.sentenceSpans()
		if want == "" || len(spans) == 0 {
			return
		}
		span := spans[0]
		if last {
			span = spans[len(spans)-1]
		}
		for _, line := range voiceLines {
			if firstSyllables(line) == want {
				out = append(out, l.phrase(span[0], span[1], kind, line, 0))
				return
			}
		}
	}
	match(lines[0], analysis.Counted.OpenClose.Openings, false, HabitOpening)
	match(lines[len(lines)-1], analysis.Counted.OpenClose.Closings, true, HabitClosing)
	return out
}

// endingPhrases is ①: a sentence whose stripped text ends with one of the voice's frequent
// sentence-final strings, longest first, spans that string.
func endingPhrases(analysis Analysis, lines []habitLine) []HabitPhrase {
	suffixes := append([]Suffix(nil), analysis.Counted.Endings.Suffixes...)
	sort.SliceStable(suffixes, func(i, j int) bool {
		return utf8.RuneCountInString(suffixes[i].Text) > utf8.RuneCountInString(suffixes[j].Text)
	})
	var out []HabitPhrase
	for _, l := range lines {
		for _, span := range l.sentenceSpans() {
			stripped := stripTail(string(l.field.runes[span[0]:span[1]]))
			for _, suffix := range suffixes {
				if suffix.Text == "" || !strings.HasSuffix(stripped, suffix.Text) {
					continue
				}
				end := span[0] + utf8.RuneCountInString(stripped)
				rate := 0.0
				if analysis.Counted.Sentences > 0 {
					rate = float64(suffix.Count) * 100 / float64(analysis.Counted.Sentences)
				}
				out = append(out, l.phrase(end-utf8.RuneCountInString(suffix.Text), end, HabitEnding, suffix.Text, rate))
				break
			}
		}
	}
	return out
}

// adverbPhrases is ⑥: a whitespace token that is a lexicon word, alone or with one particle,
// spans the word inside the token.
func adverbPhrases(analysis Analysis, lines []habitLine) []HabitPhrase {
	rates := map[string]float64{}
	for _, word := range analysis.Counted.Adverbs.Words {
		rates[word.Word] = word.PerHundred
	}
	var out []HabitPhrase
	for _, l := range lines {
		runes := l.field.runes
		for i := l.start; i < l.end; {
			if unicode.IsSpace(runes[i]) {
				i++
				continue
			}
			j := i
			for j < l.end && !unicode.IsSpace(runes[j]) {
				j++
			}
			token := string(runes[i:j])
			if word := lexiconWord(stripToken(token)); word != "" {
				if rate, ok := rates[word]; ok {
					start := i + utf8.RuneCountInString(token[:strings.Index(token, word)])
					out = append(out, l.phrase(start, start+utf8.RuneCountInString(word), HabitAdverb, word, rate))
				}
			}
			i = j
		}
	}
	return out
}

// withoutOverlaps keeps the longer of two overlapping spans of one field, the earlier on a tie.
func withoutOverlaps(found []HabitPhrase) []HabitPhrase {
	order := append([]HabitPhrase(nil), found...)
	sort.SliceStable(order, func(i, j int) bool {
		a, b := order[i], order[j]
		if la, lb := a.End-a.Start, b.End-b.Start; la != lb {
			return la > lb
		}
		if a.BlockIndex != b.BlockIndex {
			return a.BlockIndex < b.BlockIndex
		}
		if a.ItemIndex != b.ItemIndex {
			return a.ItemIndex < b.ItemIndex
		}
		return a.Start < b.Start
	})
	var kept []HabitPhrase
	for _, candidate := range order {
		clear := true
		for _, have := range kept {
			if have.BlockIndex == candidate.BlockIndex && have.ItemIndex == candidate.ItemIndex &&
				candidate.Start < have.End && have.Start < candidate.End {
				clear = false
				break
			}
		}
		if clear {
			kept = append(kept, candidate)
		}
	}
	return kept
}

type habitExample struct {
	sentence  string
	fictional bool
}

// exampleFor is the one sentence a habit is shown with (VOICE-77): the first one holding the
// term among the analysis's own examples whose 학습 글 still exists (VOICE-21), or, for an AI
// 말투, its fictional sample (VOICE-69).
func exampleFor(analysis Analysis, kind HabitKind, term string, present map[string]bool) habitExample {
	holds := func(sentence string) bool {
		if kind == HabitOpening || kind == HabitClosing {
			return firstSyllables(sentence) != "" && firstSyllables(sentence) == firstSyllables(term)
		}
		return strings.Contains(sentence, term)
	}
	if analysis.Origin == OriginSynthetic {
		for _, sentence := range SegmentSentences(analysis.SyntheticSample) {
			if holds(sentence) {
				return habitExample{sentence: sentence, fictional: true}
			}
		}
		return habitExample{}
	}
	counted := analysis.Counted
	for _, example := range []Example{
		counted.Endings.Example, counted.Marks.Example, counted.Emoji.Example, counted.Shape.Example,
		counted.OpenClose.Example, counted.Adverbs.Example, counted.Person.Example, counted.Headings.Example,
	} {
		if example.Sentence != "" && present[example.MaterialID] && holds(example.Sentence) {
			return habitExample{sentence: example.Sentence}
		}
	}
	for _, example := range analysis.AI.Examples {
		if example.Sentence != "" && present[example.MaterialID] && holds(example.Sentence) {
			return habitExample{sentence: example.Sentence}
		}
	}
	return habitExample{}
}
