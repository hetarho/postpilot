package voice

import (
	"math"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// The fingerprint is what the product counts of a voice's surface habits (VOICE-24): eight
// items measured over its 학습 글, over a plain text or over a post's blocks, each with one
// example sentence and an unknown state, and the comparison VOICE-62 draws from two of them.
// Everything here is pure: no I/O, no clock, no model (VOICE-27, VOICE-51).

// Material is one 학습 글 as the fingerprint reads it.
type Material struct {
	ID        string
	Kind      SampleKind
	Part      PromptPart
	CreatedAt time.Time
	Text      string
}

// Block is one piece of a post's content as the fingerprint reads it.
type Block struct {
	Type    string
	Content string
	Items   []string
}

// Block types the fingerprint reads; IMAGE and VIDEO, captions included, are skipped.
const (
	BlockText    = "TEXT"
	BlockHeading = "HEADING"
	BlockList    = "LIST"
	BlockQuote   = "QUOTE"
)

// Item names one of the eight counted items, in VOICE-24's order.
type Item string

const (
	ItemEndings  Item = "endings"
	ItemMarks    Item = "marks"
	ItemEmoji    Item = "emoji"
	ItemShape    Item = "shape"
	ItemOpenings Item = "openings"
	ItemAdverbs  Item = "adverbs"
	ItemPerson   Item = "person"
	ItemHeadings Item = "headings"
)

// Items lists the eight items in order.
func Items() []Item {
	return []Item{ItemEndings, ItemMarks, ItemEmoji, ItemShape, ItemOpenings, ItemAdverbs, ItemPerson, ItemHeadings}
}

// Example is the sentence an item is shown with; MaterialID is empty for a measured text.
type Example struct {
	Sentence   string
	MaterialID string
}

// Suffix is a frequent sentence-final string (`더라구요`).
type Suffix struct {
	Text  string
	Count int
}

// Endings is item ①: the ending-class mix and the frequent sentence-final strings.
type Endings struct {
	Unknown                    bool
	Da, Haeyo, Seumnida, Other float64
	Suffixes                   []Suffix
	Example                    Example
}

// Marks is item ②: the share of sentences ending in each mark, and the repeat share.
type Marks struct {
	Unknown                                          bool
	Exclaim, Question, Tilde, Ellipsis, Period, None float64
	Repeat                                           float64
	Example                                          Example
}

// Emoji is item ③: emoji clusters and ㅎㅎ · ㅋㅋ · ㅠㅠ runs per 100 sentences.
type Emoji struct {
	Unknown              bool
	Emoji, Hh, Kk, Tears float64
	Example              Example
}

// Shape is item ④: sentence length, paragraph size and one sentence per line.
type Shape struct {
	Unknown                    bool
	AverageChars               float64
	ParagraphAverage           float64
	ParagraphMin, ParagraphMax int
	LineBreakShare             float64
	OwnLine                    bool
	Example                    Example
}

// OpenClose is item ⑤: the recurring first and last lines.
type OpenClose struct {
	Unknown            bool
	Openings, Closings []string
	Example            Example
}

// WordRate is one lexicon word per 100 sentences.
type WordRate struct {
	Word       string
	PerHundred float64
}

// Adverbs is item ⑥: the frequent adverbs and short phrases. None is a real answer.
type Adverbs struct {
	Unknown bool
	None    bool
	Words   []WordRate
	// Rates holds every lexicon word the text uses at all, for the comparison.
	Rates   map[string]float64
	Example Example
}

// First-person forms, item ⑦.
const (
	PersonJeo = "저"
	PersonUri = "우리"
	PersonNa  = "나"
)

// Person is item ⑦: 저 · 우리 · 나 per 100 sentences and the dominant form ("" is none).
type Person struct {
	Unknown      bool
	Jeo, Uri, Na float64
	Dominant     string
	Example      Example
}

// Headings is item ⑧: how headings and lists are written — the share of headings that start
// with an emoji, that ask a question and that are numbered, and the list lines' share and marker.
type Headings struct {
	Unknown                                  bool
	Count                                    int
	EmojiShare, QuestionShare, NumberedShare float64
	ListShare                                float64
	Marker                                   string
	Example                                  Example
}

// Fingerprint is the eight counted items of a voice or a text.
type Fingerprint struct {
	Sentences int
	Endings   Endings
	Marks     Marks
	Emoji     Emoji
	Shape     Shape
	OpenClose OpenClose
	Adverbs   Adverbs
	Person    Person
	Headings  Headings
}

// Unknown reports whether an item is unknown.
func (f Fingerprint) Unknown(item Item) bool {
	switch item {
	case ItemEndings:
		return f.Endings.Unknown
	case ItemMarks:
		return f.Marks.Unknown
	case ItemEmoji:
		return f.Emoji.Unknown
	case ItemShape:
		return f.Shape.Unknown
	case ItemOpenings:
		return f.OpenClose.Unknown
	case ItemAdverbs:
		return f.Adverbs.Unknown
	case ItemPerson:
		return f.Person.Unknown
	case ItemHeadings:
		return f.Headings.Unknown
	}
	return true
}

// The thresholds below which an item is unknown rather than 0 (VOICE-26).
const (
	fingerprintMinSentences     = 10
	fingerprintShapeMinSentence = 5
	fingerprintNoneMinSentences = 30
	openingMaxRunes             = 60
	headingMaxRunes             = 30
)

// adverbLexicon is ⑥'s code-owned lexicon, in its tie-breaking order.
var adverbLexicon = []string{
	"진짜", "정말", "너무", "완전", "되게", "엄청", "무척", "아주", "매우", "약간", "살짝", "조금", "좀", "꽤", "제법", "은근", "참", "딱", "막",
	"그냥", "확실히", "특히", "역시", "일단", "드디어", "벌써", "이미", "다시", "또", "사실", "솔직히", "개인적으로", "무엇보다", "근데", "그런데",
	"그래서", "그리고", "그러니까", "아무튼", "어쨌든", "게다가",
}

var adverbParticles = []string{"로", "도", "는", "은", "요"}

var personForms = map[string]string{
	"저": PersonJeo, "저는": PersonJeo, "저도": PersonJeo, "저를": PersonJeo, "저의": PersonJeo, "저한테": PersonJeo, "저에게": PersonJeo, "제가": PersonJeo, "제": PersonJeo,
	"저희": PersonUri, "저희는": PersonUri, "저희도": PersonUri, "저희가": PersonUri, "저희의": PersonUri, "우리": PersonUri, "우리는": PersonUri, "우리도": PersonUri, "우리가": PersonUri, "우리의": PersonUri,
	"나": PersonNa, "나는": PersonNa, "나도": PersonNa, "나를": PersonNa, "나의": PersonNa, "나한테": PersonNa, "나에게": PersonNa, "내가": PersonNa, "내": PersonNa,
}

var listMarkers = []string{"-", "•", "·", "*", "▶", "▷", "✔", "✅", "☑", "✓"}

// --- the reading ---

// line is one prose line of a paragraph.
type line struct {
	text       string
	heading    bool   // a HEADING block, whatever it says
	listMarker string // a LIST block item's marker
}

// source is one 학습 글 or text reduced to its paragraphs of prose lines.
type source struct {
	id         string
	kind       SampleKind
	part       PromptPart
	paragraphs [][]line
	// blocks is a post's content: only its HEADING and LIST blocks are headings and lists.
	blocks bool
}

type sentence struct {
	text string
	id   string
}

// FingerprintOf counts a voice's 학습 글, newest first for examples and ties.
func FingerprintOf(materials []Material) Fingerprint {
	ordered := slices.Clone(materials)
	sort.SliceStable(ordered, func(i, j int) bool {
		if !ordered[i].CreatedAt.Equal(ordered[j].CreatedAt) {
			return ordered[i].CreatedAt.After(ordered[j].CreatedAt)
		}
		return ordered[i].ID > ordered[j].ID
	})
	sources := make([]source, 0, len(ordered))
	for _, material := range ordered {
		sources = append(sources, source{id: material.ID, kind: material.Kind, part: material.Part, paragraphs: textParagraphs(material.Text)})
	}
	return measure(sources, false)
}

// MeasureText counts one plain text, a post as the owner or the AI wrote it.
func MeasureText(text string) Fingerprint {
	return measure([]source{{kind: SampleKindPost, paragraphs: textParagraphs(text)}}, true)
}

// MeasureBlocks counts a post's content: TEXT and QUOTE blocks are prose paragraphs, a HEADING
// is a heading, a LIST's items are list lines, and IMAGE and VIDEO are skipped.
func MeasureBlocks(blocks []Block) Fingerprint {
	var paragraphs [][]line
	for _, block := range blocks {
		switch block.Type {
		case BlockText, BlockQuote:
			for _, paragraph := range textParagraphs(block.Content) {
				paragraphs = append(paragraphs, paragraph)
			}
		case BlockHeading:
			if text := strings.TrimSpace(block.Content); isProse(text) {
				paragraphs = append(paragraphs, []line{{text: text, heading: true}})
			}
		case BlockList:
			var items []line
			for _, item := range block.Items {
				if text := strings.TrimSpace(item); isProse(text) {
					items = append(items, line{text: text, listMarker: "-"})
				}
			}
			if len(items) > 0 {
				paragraphs = append(paragraphs, items)
			}
		}
	}
	return measure([]source{{kind: SampleKindPost, paragraphs: paragraphs, blocks: true}}, true)
}

// textParagraphs splits a text on blank lines and keeps each paragraph's prose lines (VOICE-61).
func textParagraphs(text string) [][]line {
	var paragraphs [][]line
	var current []line
	flush := func() {
		if len(current) > 0 {
			paragraphs = append(paragraphs, current)
			current = nil
		}
	}
	for _, raw := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if strings.TrimSpace(raw) == "" {
			flush()
			continue
		}
		if isProse(raw) {
			current = append(current, line{text: strings.TrimSpace(raw)})
		}
	}
	flush()
	return paragraphs
}

func measure(sources []source, text bool) Fingerprint {
	var sentences []sentence
	var paragraphSizes []int
	lines, listLines := 0, 0
	var headings []line
	var headingIDs []string
	var firstList *sentence
	markers := map[string]int{}
	for _, src := range sources {
		for _, paragraph := range src.paragraphs {
			size := 0
			for _, ln := range paragraph {
				lines++
				marker := ln.listMarker
				if marker == "" && !src.blocks {
					marker = listMarkerOf(ln.text)
				}
				if marker != "" {
					listLines++
					markers[marker]++
					if firstList == nil {
						firstList = &sentence{text: ln.text, id: src.id}
					}
				} else if ln.heading || (!src.blocks && isPlainHeading(ln.text, len(paragraph))) {
					headings = append(headings, ln)
					headingIDs = append(headingIDs, src.id)
				}
				for _, s := range SegmentSentences(ln.text) {
					sentences = append(sentences, sentence{text: s, id: src.id})
					size++
				}
			}
			if size > 0 {
				paragraphSizes = append(paragraphSizes, size)
			}
		}
	}
	f := Fingerprint{Sentences: len(sentences)}
	f.Endings = endingsOf(sentences)
	f.Marks = marksOf(sentences)
	f.Emoji = emojiOf(sentences)
	f.Shape = shapeOf(sentences, paragraphSizes, lines)
	f.OpenClose = openCloseOf(sources, text)
	f.Adverbs = adverbsOf(sentences)
	f.Person = personOf(sentences)
	f.Headings = headingsOf(headings, headingIDs, lines, listLines, markers, firstList)
	return f
}

// --- ① endings ---

func endingsOf(sentences []sentence) Endings {
	out := Endings{Unknown: len(sentences) < fingerprintMinSentences}
	if len(sentences) == 0 {
		return out
	}
	counts := map[string]int{}
	candidates := map[string]int{}
	for _, s := range sentences {
		stripped := stripTail(s.text)
		counts[endingOf(stripped)]++
		for _, suffix := range syllableSuffixes(stripped) {
			candidates[suffix]++
		}
	}
	n := float64(len(sentences))
	out.Da, out.Haeyo, out.Seumnida, out.Other = float64(counts["다"])/n, float64(counts["해요"])/n, float64(counts["습니다"])/n, float64(counts["기타"])/n
	out.Suffixes = topSuffixes(candidates, len(sentences))
	dominant := "다"
	for _, class := range []string{"해요", "습니다", "기타"} {
		if counts[class] > counts[dominant] {
			dominant = class
		}
	}
	for _, s := range sentences {
		stripped := stripTail(s.text)
		if endingOf(stripped) != dominant {
			continue
		}
		if len(out.Suffixes) == 0 || strings.HasSuffix(stripped, out.Suffixes[0].Text) {
			out.Example = Example{Sentence: s.text, MaterialID: s.id}
			break
		}
	}
	if out.Example.Sentence == "" {
		for _, s := range sentences {
			if endingOf(stripTail(s.text)) == dominant {
				out.Example = Example{Sentence: s.text, MaterialID: s.id}
				break
			}
		}
	}
	return out
}

// syllableSuffixes are the last 2, 3 and 4 Hangul syllables of a stripped sentence.
func syllableSuffixes(stripped string) []string {
	runes := []rune(stripped)
	run := 0
	for i := len(runes) - 1; i >= 0 && isSyllable(runes[i]); i-- {
		run++
	}
	var out []string
	for n := 2; n <= 4 && n <= run; n++ {
		out = append(out, string(runes[len(runes)-n:]))
	}
	return out
}

func topSuffixes(candidates map[string]int, sentences int) []Suffix {
	var kept []Suffix
	for text, count := range candidates {
		if count >= 3 && float64(count)/float64(sentences) >= 0.05 {
			kept = append(kept, Suffix{Text: text, Count: count})
		}
	}
	// A shorter suffix gives way to a longer one that holds most of its count.
	var survivors []Suffix
	for _, short := range kept {
		covered := false
		for _, long := range kept {
			if utf8.RuneCountInString(long.Text) > utf8.RuneCountInString(short.Text) && strings.HasSuffix(long.Text, short.Text) && float64(long.Count) >= 0.8*float64(short.Count) {
				covered = true
				break
			}
		}
		if !covered {
			survivors = append(survivors, short)
		}
	}
	sort.Slice(survivors, func(i, j int) bool {
		if survivors[i].Count != survivors[j].Count {
			return survivors[i].Count > survivors[j].Count
		}
		li, lj := utf8.RuneCountInString(survivors[i].Text), utf8.RuneCountInString(survivors[j].Text)
		if li != lj {
			return li > lj
		}
		return survivors[i].Text < survivors[j].Text
	})
	if len(survivors) > 5 {
		survivors = survivors[:5]
	}
	return survivors
}

// --- ② sentence-final marks ---

const (
	markExclaim  = "exclaim"
	markQuestion = "question"
	markTilde    = "tilde"
	markEllipsis = "ellipsis"
	markPeriod   = "period"
	markNone     = "none"
)

func marksOf(sentences []sentence) Marks {
	out := Marks{Unknown: len(sentences) < fingerprintMinSentences}
	if len(sentences) == 0 {
		return out
	}
	counts := map[string]int{}
	firstOf := map[string]sentence{}
	repeats := 0
	for _, s := range sentences {
		mark, repeated := finalMark(s.text)
		counts[mark]++
		if _, seen := firstOf[mark]; !seen {
			firstOf[mark] = s
		}
		if repeated {
			repeats++
		}
	}
	n := float64(len(sentences))
	out.Exclaim, out.Question, out.Tilde = float64(counts[markExclaim])/n, float64(counts[markQuestion])/n, float64(counts[markTilde])/n
	out.Ellipsis, out.Period, out.None = float64(counts[markEllipsis])/n, float64(counts[markPeriod])/n, float64(counts[markNone])/n
	out.Repeat = float64(repeats) / n
	best := ""
	for _, mark := range []string{markExclaim, markQuestion, markTilde, markEllipsis, markNone} {
		if counts[mark] > 0 && (best == "" || counts[mark] > counts[best]) {
			best = mark
		}
	}
	if best == "" {
		best = markPeriod
	}
	if s, ok := firstOf[best]; ok {
		out.Example = Example{Sentence: s.text, MaterialID: s.id}
	}
	return out
}

// finalMark classifies a sentence by its last mark after emoji and jamo, and reports a final
// run holding two or more of the same `!`, `?` or `~`.
func finalMark(text string) (string, bool) {
	runes := []rune(strings.TrimRightFunc(text, func(r rune) bool {
		return unicode.IsSpace(r) || isEmojiRune(r) || isJamo(r)
	}))
	if len(runes) == 0 {
		return markNone, false
	}
	run := 0
	for i := len(runes) - 1; i >= 0 && isTrailingMark(runes[i]); i-- {
		run++
	}
	tail := runes[len(runes)-run:]
	repeated := false
	for _, mark := range []rune{'!', '?', '~'} {
		same := 0
		for _, r := range tail {
			if r == mark || (mark == '~' && r == '～') {
				same++
			}
		}
		if same >= 2 {
			repeated = true
		}
	}
	last := runes[len(runes)-1]
	switch {
	case last == '!':
		return markExclaim, repeated
	case last == '?':
		return markQuestion, repeated
	case last == '~' || last == '～':
		return markTilde, repeated
	case last == '…':
		return markEllipsis, repeated
	case last == '.' || last == '。':
		dots := 0
		for _, r := range tail {
			if r == '.' || r == '。' {
				dots++
			}
		}
		if dots >= 2 {
			return markEllipsis, repeated
		}
		return markPeriod, repeated
	}
	return markNone, repeated
}

// --- ③ emoji and jamo ---

func emojiOf(sentences []sentence) Emoji {
	out := Emoji{Unknown: len(sentences) < fingerprintMinSentences}
	if len(sentences) == 0 {
		return out
	}
	emoji, hh, kk, tears := 0, 0, 0, 0
	for _, s := range sentences {
		e, h, k, t := countEmojiAndJamo(s.text)
		emoji, hh, kk, tears = emoji+e, hh+h, kk+k, tears+t
		if out.Example.Sentence == "" && e+h+k+t > 0 {
			out.Example = Example{Sentence: s.text, MaterialID: s.id}
		}
	}
	per := func(count int) float64 { return float64(count) * 100 / float64(len(sentences)) }
	out.Emoji, out.Hh, out.Kk, out.Tears = per(emoji), per(hh), per(kk), per(tears)
	return out
}

// countEmojiAndJamo counts emoji clusters and runs of two or more ㅎ, ㅋ and ㅠ/ㅜ.
func countEmojiAndJamo(text string) (emoji, hh, kk, tears int) {
	runes := []rune(text)
	for i := 0; i < len(runes); {
		r := runes[i]
		switch {
		case isEmojiBase(r):
			for i < len(runes) && isEmojiRune(runes[i]) {
				i++
			}
			emoji++
			continue
		case r == 'ㅎ' || r == 'ㅋ' || r == 'ㅠ' || r == 'ㅜ':
			j := i
			for j < len(runes) && sameJamoFamily(runes[j], r) {
				j++
			}
			if j-i >= 2 {
				switch r {
				case 'ㅎ':
					hh++
				case 'ㅋ':
					kk++
				default:
					tears++
				}
			}
			i = j
			continue
		}
		i++
	}
	return
}

func sameJamoFamily(r, family rune) bool {
	if family == 'ㅠ' || family == 'ㅜ' {
		return r == 'ㅠ' || r == 'ㅜ'
	}
	return r == family
}

func isEmojiBase(r rune) bool {
	return (r >= 0x1F000 && r <= 0x1FAFF) || (r >= 0x2600 && r <= 0x27BF) || (r >= 0x2B00 && r <= 0x2BFF) || r == 0x203C || r == 0x2049
}

// isEmojiRune is an emoji rune or one that joins or modifies a cluster.
func isEmojiRune(r rune) bool { return isEmojiBase(r) || r == 0x200D || r == 0xFE0F }

func isJamo(r rune) bool { return r >= 0x3131 && r <= 0x318E }

func isSyllable(r rune) bool { return r >= 0xAC00 && r <= 0xD7A3 }

// stripTail drops trailing whitespace, punctuation, emoji and compatibility jamo.
func stripTail(text string) string {
	return strings.TrimRightFunc(text, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) || isEmojiRune(r) || isJamo(r)
	})
}

// --- ④ shape ---

func shapeOf(sentences []sentence, paragraphSizes []int, lines int) Shape {
	out := Shape{Unknown: len(sentences) < fingerprintShapeMinSentence}
	if len(sentences) == 0 {
		return out
	}
	total := 0
	lengths := make([]int, len(sentences))
	for i, s := range sentences {
		lengths[i] = utf8.RuneCountInString(strings.TrimRightFunc(strings.TrimSpace(s.text), func(r rune) bool { return unicode.IsPunct(r) || unicode.IsSpace(r) }))
		total += lengths[i]
	}
	out.AverageChars = math.Round(float64(total)*10/float64(len(sentences))) / 10
	if len(paragraphSizes) > 0 {
		sum := 0
		out.ParagraphMin, out.ParagraphMax = paragraphSizes[0], paragraphSizes[0]
		for _, size := range paragraphSizes {
			sum += size
			out.ParagraphMin, out.ParagraphMax = min(out.ParagraphMin, size), max(out.ParagraphMax, size)
		}
		out.ParagraphAverage = math.Round(float64(sum)*10/float64(len(paragraphSizes))) / 10
	}
	out.LineBreakShare = math.Min(1, float64(lines)/float64(len(sentences)))
	out.OwnLine = out.LineBreakShare >= 0.8
	closest := 0
	for i, length := range lengths {
		if math.Abs(float64(length)-out.AverageChars) < math.Abs(float64(lengths[closest])-out.AverageChars) {
			closest = i
		}
	}
	out.Example = Example{Sentence: sentences[closest].text, MaterialID: sentences[closest].id}
	return out
}

// --- ⑤ opening and closing ---

type lineCount struct {
	text, id string
	count    int
	order    int
}

func openCloseOf(sources []source, text bool) OpenClose {
	openings := map[string]*lineCount{}
	closings := map[string]*lineCount{}
	order := 0
	add := func(into map[string]*lineCount, value, id string) {
		value = cutRunes(strings.TrimSpace(value), openingMaxRunes)
		if value == "" {
			return
		}
		key := strings.Join(strings.Fields(value), " ")
		if found, ok := into[key]; ok {
			found.count++
			return
		}
		order++
		into[key] = &lineCount{text: key, id: id, count: 1, order: order}
	}
	for _, src := range sources {
		first, last := firstAndLast(src.paragraphs)
		switch {
		case text || src.kind == SampleKindPost:
			add(openings, first, src.id)
			add(closings, last, src.id)
		case src.part == PartOpening:
			if sentences := SegmentSentences(joinLines(src.paragraphs)); len(sentences) > 0 {
				add(openings, sentences[0], src.id)
			}
		case src.part == PartClosing:
			if sentences := SegmentSentences(joinLines(src.paragraphs)); len(sentences) > 0 {
				add(closings, sentences[len(sentences)-1], src.id)
			}
		}
	}
	out := OpenClose{Openings: rankLines(openings), Closings: rankLines(closings)}
	out.Unknown = len(out.Openings) == 0 && len(out.Closings) == 0
	if len(out.Openings) > 0 {
		out.Example = Example{Sentence: out.Openings[0], MaterialID: openings[out.Openings[0]].id}
	} else if len(out.Closings) > 0 {
		out.Example = Example{Sentence: out.Closings[0], MaterialID: closings[out.Closings[0]].id}
	}
	return out
}

func rankLines(counts map[string]*lineCount) []string {
	ranked := make([]*lineCount, 0, len(counts))
	for _, found := range counts {
		ranked = append(ranked, found)
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].count != ranked[j].count {
			return ranked[i].count > ranked[j].count
		}
		return ranked[i].order < ranked[j].order
	})
	out := make([]string, 0, min(3, len(ranked)))
	for _, found := range ranked {
		if len(out) == 3 {
			break
		}
		out = append(out, found.text)
	}
	return out
}

func firstAndLast(paragraphs [][]line) (string, string) {
	if len(paragraphs) == 0 {
		return "", ""
	}
	lastParagraph := paragraphs[len(paragraphs)-1]
	return paragraphs[0][0].text, lastParagraph[len(lastParagraph)-1].text
}

func joinLines(paragraphs [][]line) string {
	var parts []string
	for _, paragraph := range paragraphs {
		for _, ln := range paragraph {
			parts = append(parts, ln.text)
		}
	}
	return strings.Join(parts, "\n")
}

func cutRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) > limit {
		return strings.TrimSpace(string(runes[:limit]))
	}
	return value
}

// --- ⑥ adverbs and phrases ---

func adverbsOf(sentences []sentence) Adverbs {
	out := Adverbs{Rates: map[string]float64{}}
	counts := map[string]int{}
	firstOf := map[string]sentence{}
	for _, s := range sentences {
		for _, token := range strings.Fields(s.text) {
			if word := lexiconWord(stripToken(token)); word != "" {
				counts[word]++
				if _, seen := firstOf[word]; !seen {
					firstOf[word] = s
				}
			}
		}
	}
	for word, count := range counts {
		out.Rates[word] = float64(count) * 100 / float64(max(len(sentences), 1))
	}
	for _, word := range adverbLexicon {
		if counts[word] >= 2 {
			out.Words = append(out.Words, WordRate{Word: word, PerHundred: out.Rates[word]})
		}
	}
	sort.SliceStable(out.Words, func(i, j int) bool { return counts[out.Words[i].Word] > counts[out.Words[j].Word] })
	if len(out.Words) > 6 {
		out.Words = out.Words[:6]
	}
	switch {
	case len(sentences) < fingerprintMinSentences:
		out.Unknown = true
	case len(out.Words) == 0:
		out.None = len(sentences) >= fingerprintNoneMinSentences
		out.Unknown = !out.None
	}
	if len(out.Words) > 0 {
		s := firstOf[out.Words[0].Word]
		out.Example = Example{Sentence: s.text, MaterialID: s.id}
	}
	return out
}

// lexiconWord is the lexicon word a token is, alone or with one particle, or "".
func lexiconWord(token string) string {
	for _, word := range adverbLexicon {
		if token == word {
			return word
		}
		for _, particle := range adverbParticles {
			if token == word+particle {
				return word
			}
		}
	}
	return ""
}

// stripToken drops punctuation, quotes, emoji and jamo around a whitespace token.
func stripToken(token string) string {
	return strings.TrimFunc(token, func(r rune) bool {
		return unicode.IsPunct(r) || unicode.IsSymbol(r) || isEmojiRune(r) || isJamo(r) || strings.ContainsRune("\"'“”‘’「」『』", r)
	})
}

// --- ⑦ first person ---

func personOf(sentences []sentence) Person {
	out := Person{Unknown: len(sentences) < fingerprintMinSentences}
	if len(sentences) == 0 {
		return out
	}
	counts := map[string]int{}
	firstOf := map[string]sentence{}
	for _, s := range sentences {
		for _, token := range strings.Fields(s.text) {
			if form, ok := personForms[stripToken(token)]; ok {
				counts[form]++
				if _, seen := firstOf[form]; !seen {
					firstOf[form] = s
				}
			}
		}
	}
	per := func(count int) float64 { return float64(count) * 100 / float64(len(sentences)) }
	out.Jeo, out.Uri, out.Na = per(counts[PersonJeo]), per(counts[PersonUri]), per(counts[PersonNa])
	if out.Jeo+out.Uri+out.Na >= 1 {
		out.Dominant = PersonJeo
		for _, form := range []string{PersonUri, PersonNa} {
			if counts[form] > counts[out.Dominant] {
				out.Dominant = form
			}
		}
		s := firstOf[out.Dominant]
		out.Example = Example{Sentence: s.text, MaterialID: s.id}
	}
	return out
}

// --- ⑧ headings and lists ---

// isPlainHeading is a plain-text prose line alone in its paragraph, short, and not a sentence.
func isPlainHeading(text string, paragraphLines int) bool {
	if paragraphLines != 1 || utf8.RuneCountInString(text) > headingMaxRunes {
		return false
	}
	trimmed := strings.TrimRightFunc(text, func(r rune) bool { return unicode.IsSpace(r) || isEmojiRune(r) })
	return endingOf(stripTail(text)) == "기타" || strings.HasSuffix(trimmed, "?")
}

// listMarkerOf is a list line's marker: a bullet, a number with `.` or `)` (as `1.`), or ①–⑳
// (as `①`); "" for a line that is not a list line.
func listMarkerOf(text string) string {
	for _, marker := range listMarkers {
		if strings.HasPrefix(text, marker) {
			return marker
		}
	}
	runes := []rune(text)
	if len(runes) > 0 && runes[0] >= '①' && runes[0] <= '⑳' {
		return "①"
	}
	digits := 0
	for digits < len(runes) && runes[digits] >= '0' && runes[digits] <= '9' {
		digits++
	}
	if digits > 0 && digits < len(runes) && (runes[digits] == '.' || runes[digits] == ')') {
		return "1."
	}
	return ""
}

func headingsOf(headings []line, ids []string, lines, listLines int, markers map[string]int, firstList *sentence) Headings {
	out := Headings{Count: len(headings), Unknown: len(headings) == 0 && listLines == 0}
	if len(headings) > 0 {
		withEmoji, questions, numbered := 0, 0, 0
		for _, heading := range headings {
			if first, _ := utf8.DecodeRuneInString(heading.text); isEmojiBase(first) {
				withEmoji++
			}
			if strings.HasSuffix(strings.TrimRightFunc(heading.text, func(r rune) bool { return unicode.IsSpace(r) || isEmojiRune(r) }), "?") {
				questions++
			}
			if marker := listMarkerOf(heading.text); marker == "1." || marker == "①" {
				numbered++
			}
		}
		n := float64(len(headings))
		out.EmojiShare, out.QuestionShare, out.NumberedShare = float64(withEmoji)/n, float64(questions)/n, float64(numbered)/n
		out.Example = Example{Sentence: headings[0].text, MaterialID: ids[0]}
	} else if firstList != nil {
		out.Example = Example{Sentence: firstList.text, MaterialID: firstList.id}
	}
	if lines > 0 {
		out.ListShare = float64(listLines) / float64(lines)
	}
	for _, marker := range append(append([]string(nil), listMarkers...), "1.", "①") {
		if markers[marker] > 0 && (out.Marker == "" || markers[marker] > markers[out.Marker]) {
			out.Marker = marker
		}
	}
	return out
}

// --- the comparison (VOICE-62) ---

// FacetUnit is how a facet's two values read: a 0…1 share, a rate per 100 sentences, characters,
// sentences, or a list of terms.
type FacetUnit string

const (
	UnitShare      FacetUnit = "share"
	UnitPerHundred FacetUnit = "per_hundred"
	UnitChars      FacetUnit = "chars"
	UnitSentences  FacetUnit = "sentences"
	UnitText       FacetUnit = "text"
)

// FacetUnits lists every unit.
func FacetUnits() []FacetUnit {
	return []FacetUnit{UnitShare, UnitPerHundred, UnitChars, UnitSentences, UnitText}
}

// Facet is one value of an item on both sides, numeric or (UnitText) a list of terms, in the
// item's own unit. Wording belongs to the widget and the projection.
type Facet struct {
	Key                   string
	Unit                  FacetUnit
	Voice, Text           float64
	VoiceTerms, TextTerms []string
}

// ItemComparison is one counted item measured on the voice and on a text.
type ItemComparison struct {
	Item     Item
	Unknown  bool
	Facets   []Facet
	Headline string
	// Distance is 0…1, relative to the voice's own value.
	Distance float64
}

// Compare measures a text against a voice: one comparison per item, the farthest first and
// the unknown ones last.
func Compare(voice, text Fingerprint) []ItemComparison {
	out := make([]ItemComparison, 0, len(Items()))
	for _, item := range Items() {
		comparison := ItemComparison{Item: item}
		comparison.Facets, comparison.Distance = compareItem(item, voice, text)
		comparison.Distance = math.Max(0, math.Min(1, comparison.Distance))
		comparison.Headline = headline(comparison.Facets)
		if voice.Unknown(item) || text.Unknown(item) {
			comparison.Unknown, comparison.Distance = true, 0
		}
		out = append(out, comparison)
	}
	order := map[Item]int{}
	for i, item := range Items() {
		order[item] = i
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Unknown != out[j].Unknown {
			return !out[i].Unknown
		}
		if !out[i].Unknown && out[i].Distance != out[j].Distance {
			return out[i].Distance > out[j].Distance
		}
		return order[out[i].Item] < order[out[j].Item]
	})
	return out
}

func compareItem(item Item, v, t Fingerprint) ([]Facet, float64) {
	share := func(key string, a, b float64) Facet { return Facet{Key: key, Unit: UnitShare, Voice: a, Text: b} }
	rate := func(key string, a, b float64) Facet { return Facet{Key: key, Unit: UnitPerHundred, Voice: a, Text: b} }
	terms := func(key string, a, b []string) Facet {
		return Facet{Key: key, Unit: UnitText, VoiceTerms: a, TextTerms: b}
	}
	switch item {
	case ItemEndings:
		facets := []Facet{share("다", v.Endings.Da, t.Endings.Da), share("해요", v.Endings.Haeyo, t.Endings.Haeyo), share("습니다", v.Endings.Seumnida, t.Endings.Seumnida), share("기타", v.Endings.Other, t.Endings.Other)}
		facets = append(facets, terms("suffixes", suffixTexts(v.Endings.Suffixes), suffixTexts(t.Endings.Suffixes)))
		return facets, halfL1(facets[:4])
	case ItemMarks:
		facets := []Facet{
			share(markExclaim, v.Marks.Exclaim, t.Marks.Exclaim), share(markQuestion, v.Marks.Question, t.Marks.Question),
			share(markTilde, v.Marks.Tilde, t.Marks.Tilde), share(markEllipsis, v.Marks.Ellipsis, t.Marks.Ellipsis),
			share(markPeriod, v.Marks.Period, t.Marks.Period), share(markNone, v.Marks.None, t.Marks.None),
			share("repeat", v.Marks.Repeat, t.Marks.Repeat),
		}
		return facets, halfL1(facets[:6])
	case ItemEmoji:
		facets := []Facet{rate("emoji", v.Emoji.Emoji, t.Emoji.Emoji), rate("ㅎㅎ", v.Emoji.Hh, t.Emoji.Hh), rate("ㅋㅋ", v.Emoji.Kk, t.Emoji.Kk), rate("ㅠㅠ", v.Emoji.Tears, t.Emoji.Tears)}
		distance := 0.0
		for _, facet := range facets {
			distance = math.Max(distance, math.Abs(facet.Voice-facet.Text)/math.Max(math.Max(facet.Voice, facet.Text), 1))
		}
		return facets, distance
	case ItemShape:
		facets := []Facet{
			{Key: "average", Unit: UnitChars, Voice: v.Shape.AverageChars, Text: t.Shape.AverageChars},
			{Key: "paragraph", Unit: UnitSentences, Voice: v.Shape.ParagraphAverage, Text: t.Shape.ParagraphAverage},
			share("line_break", v.Shape.LineBreakShare, t.Shape.LineBreakShare),
		}
		length := math.Min(1, math.Abs(v.Shape.AverageChars-t.Shape.AverageChars)/math.Max(v.Shape.AverageChars, 1))
		return facets, (length + math.Abs(v.Shape.LineBreakShare-t.Shape.LineBreakShare)) / 2
	case ItemOpenings:
		facets := []Facet{terms("openings", v.OpenClose.Openings, t.OpenClose.Openings), terms("closings", v.OpenClose.Closings, t.OpenClose.Closings)}
		distance := 0.0
		if !sharesLine(v.OpenClose.Openings, t.OpenClose.Openings) {
			distance += 0.5
		}
		if !sharesLine(v.OpenClose.Closings, t.OpenClose.Closings) {
			distance += 0.5
		}
		return facets, distance
	case ItemAdverbs:
		facets := make([]Facet, 0, len(v.Adverbs.Words))
		missing := 0
		for _, word := range v.Adverbs.Words {
			used := t.Adverbs.Rates[word.Word]
			facets = append(facets, rate(word.Word, word.PerHundred, used))
			if used == 0 {
				missing++
			}
		}
		if len(v.Adverbs.Words) == 0 {
			return facets, 0
		}
		return facets, float64(missing) / float64(len(v.Adverbs.Words))
	case ItemPerson:
		facets := []Facet{rate(PersonJeo, v.Person.Jeo, t.Person.Jeo), rate(PersonUri, v.Person.Uri, t.Person.Uri), rate(PersonNa, v.Person.Na, t.Person.Na)}
		facets = append(facets, terms("dominant", nonEmpty(v.Person.Dominant), nonEmpty(t.Person.Dominant)))
		if v.Person.Dominant != t.Person.Dominant {
			return facets, 1
		}
		if v.Person.Dominant == "" {
			return facets, 0
		}
		a, b := personRate(v.Person), personRate(t.Person)
		return facets, math.Abs(a-b) / math.Max(math.Max(a, b), 1e-9)
	case ItemHeadings:
		facets := []Facet{
			share("emoji", v.Headings.EmojiShare, t.Headings.EmojiShare), share("question", v.Headings.QuestionShare, t.Headings.QuestionShare),
			share("numbered", v.Headings.NumberedShare, t.Headings.NumberedShare), share("list", v.Headings.ListShare, t.Headings.ListShare),
			terms("marker", nonEmpty(v.Headings.Marker), nonEmpty(t.Headings.Marker)),
		}
		return facets, (math.Abs(v.Headings.EmojiShare-t.Headings.EmojiShare) + math.Abs(v.Headings.QuestionShare-t.Headings.QuestionShare)) / 2
	}
	return nil, 0
}

func halfL1(facets []Facet) float64 {
	sum := 0.0
	for _, facet := range facets {
		sum += math.Abs(facet.Voice - facet.Text)
	}
	return sum / 2
}

// headline is the facet with the largest relative gap: a numeric one by |a−b| / max(a, b), a
// list by the share of the voice's terms the text lacks.
func headline(facets []Facet) string {
	best, bestGap := "", -1.0
	for _, facet := range facets {
		gap := 0.0
		if facet.Unit == UnitText {
			if len(facet.VoiceTerms) > 0 {
				missing := 0
				for _, term := range facet.VoiceTerms {
					if !slices.Contains(facet.TextTerms, term) {
						missing++
					}
				}
				gap = float64(missing) / float64(len(facet.VoiceTerms))
			}
		} else if top := math.Max(math.Abs(facet.Voice), math.Abs(facet.Text)); top > 0 {
			gap = math.Abs(facet.Voice-facet.Text) / top
		}
		if gap > bestGap {
			best, bestGap = facet.Key, gap
		}
	}
	return best
}

// sharesLine reports a text line that begins like one of the voice's lines: the same first three
// syllables once whitespace, punctuation and emoji are gone.
func sharesLine(voice, text []string) bool {
	for _, candidate := range text {
		want := firstSyllables(candidate)
		for _, line := range voice {
			if want != "" && firstSyllables(line) == want {
				return true
			}
		}
	}
	return false
}

func firstSyllables(value string) string {
	var out []rune
	for _, r := range value {
		if isSyllable(r) {
			out = append(out, r)
			if len(out) == 3 {
				break
			}
		}
	}
	return string(out)
}

func suffixTexts(suffixes []Suffix) []string {
	out := make([]string, 0, len(suffixes))
	for _, suffix := range suffixes {
		out = append(out, suffix.Text)
	}
	return out
}

func nonEmpty(value string) []string {
	if value == "" {
		return []string{}
	}
	return []string{value}
}

func personRate(p Person) float64 {
	switch p.Dominant {
	case PersonJeo:
		return p.Jeo
	case PersonUri:
		return p.Uri
	case PersonNa:
		return p.Na
	}
	return 0
}
