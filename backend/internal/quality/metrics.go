package quality

import (
	"sort"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// Metric is one of the four measurements (QUAL-5), by its canonical ASCII id.
type Metric string

const (
	MetricTitleSaturation  Metric = "title_saturation"   // M1 제목 도배율
	MetricCrossPostPhrases Metric = "cross_post_phrases" // M2 글 간 고정 문구
	MetricInPostRepetition Metric = "in_post_repetition" // M3 글 안 반복과 제목 관련성
	MetricComposition      Metric = "composition"        // M4 분량·구성
)

// Metrics lists the four in QUAL's order.
func Metrics() []Metric {
	return []Metric{MetricTitleSaturation, MetricCrossPostPhrases, MetricInPostRepetition, MetricComposition}
}

// ParseMetric reads a canonical id; anything else is not a metric.
func ParseMetric(id string) (Metric, bool) {
	for _, m := range Metrics() {
		if string(m) == id {
			return m, true
		}
	}
	return "", false
}

// Sample is one post as the metrics read it: its content, the language it is measured in
// (already resolved, QUAL-26) and the nouns the write pass returned for it (GEN-55), nil or
// empty meaning none.
type Sample struct {
	Slug     string
	Doc      Document
	Language Language
	Nouns    []string
}

// normalizedNouns trims and composes each noun, drops the blank ones and keeps the first of
// each exact spelling, in the order the write pass returned them.
func normalizedNouns(nouns []string) []string {
	var out []string
	seen := make(map[string]bool, len(nouns))
	for _, noun := range nouns {
		noun = norm.NFC.String(strings.TrimSpace(noun))
		if noun == "" || seen[noun] {
			continue
		}
		seen[noun] = true
		out = append(out, noun)
	}
	return out
}

// preferred is the one tie-break every "most frequent noun" uses: the higher count, then the
// longer noun in runes, then the smaller string.
func preferred(count int, noun string, bestCount int, best string) bool {
	if count != bestCount {
		return count > bestCount
	}
	if runes, bestRunes := utf8.RuneCountInString(noun), utf8.RuneCountInString(best); runes != bestRunes {
		return runes > bestRunes
	}
	return noun < best
}

// TitleSaturation is M1's account value: the share of the window's titles that contain the most
// frequent noun, and that noun, "" when no title contains any.
type TitleSaturation struct {
	Value *float64
	Noun  string
}

// MeasureTitleSaturation reads at most the TitleWindow most recent titles (QUAL-7). The candidates
// are every noun the write pass returned for those posts, each title is judged under its own
// post's language, and the noun contained in the most titles wins. With no candidate at all the
// value cannot be computed (QUAL-40).
func MeasureTitleSaturation(window []Sample) TitleSaturation {
	if len(window) > TitleWindow {
		window = window[:TitleWindow]
	}
	var candidates []string
	seen := map[string]bool{}
	for _, sample := range window {
		for _, noun := range normalizedNouns(sample.Nouns) {
			if !seen[noun] {
				seen[noun] = true
				candidates = append(candidates, noun)
			}
		}
	}
	if len(candidates) == 0 {
		return TitleSaturation{}
	}
	titles := make([][]string, len(window))
	for i, sample := range window {
		titles[i] = Tokens(sample.Doc.Title)
	}
	best, bestCount := "", -1
	for _, noun := range candidates {
		tokens := Tokens(noun)
		count := 0
		for i, sample := range window {
			if Contains(titles[i], tokens, sample.Language) {
				count++
			}
		}
		if preferred(count, noun, bestCount, best) {
			best, bestCount = noun, count
		}
	}
	value := float64(bestCount) / float64(len(window))
	result := TitleSaturation{Value: &value}
	if bestCount > 0 {
		result.Noun = best
	}
	return result
}

// OthersOf is the window a post is compared with: the PostWindow most recent published posts
// other than itself (QUAL-8). published is newest first.
func OthersOf(published []Sample, slug string) []Sample {
	var out []Sample
	for _, sample := range published {
		if sample.Slug == slug {
			continue
		}
		out = append(out, sample)
		if len(out) == PostWindow {
			break
		}
	}
	return out
}

// MeasureCrossPost is M2 for one post: its share of runes standing in a RunLength-어절 run
// shared with any of others, and the runs (QUAL-8).
func MeasureCrossPost(post Sample, others []Sample) (Overlap, bool) {
	docs := make([]Document, len(others))
	for i, other := range others {
		docs[i] = other.Doc
	}
	return MatchRuns(post.Doc, docs, RunLength)
}

// namedRun is the run M2's rule text names (QUAL-14): among the PostWindow most recent posts, the
// run standing in the most of them, then the one with more tokens, then the earliest — the run
// whose oldest standing post was published least recently, then the one starting earliest in that
// post — and finally the smaller text. It is "" when no run stands anywhere.
func namedRun(published []Sample) string {
	window := published[:min(PostWindow, len(published))]
	windowIndex := make(map[string]int, len(window))
	for i, sample := range window {
		windowIndex[sample.Slug] = i
	}
	type standing struct {
		tokens []string
		posts  map[int]bool
	}
	runs := map[string]*standing{}
	var order []string
	for p, post := range window {
		others := OthersOf(published, post.Slug)
		overlap, ok := MeasureCrossPost(post, others)
		if !ok {
			continue
		}
		for _, run := range overlap.Runs {
			key := strings.Join(run.Tokens, " ")
			entry, found := runs[key]
			if !found {
				entry = &standing{tokens: run.Tokens, posts: map[int]bool{}}
				runs[key] = entry
				order = append(order, key)
			}
			entry.posts[p] = true
			for _, i := range run.In {
				if index, inWindow := windowIndex[others[i].Slug]; inWindow {
					entry.posts[index] = true
				}
			}
		}
	}
	var best *rankedRun
	for _, key := range order {
		entry := runs[key]
		oldest := -1
		for index := range entry.posts {
			oldest = max(oldest, index)
		}
		candidate := rankedRun{
			text: key, standing: len(entry.posts), tokens: len(entry.tokens),
			oldest: oldest, start: startIn(window[oldest].Doc, entry.tokens),
		}
		if best == nil || candidate.before(*best) {
			best = &candidate
		}
	}
	if best == nil {
		return ""
	}
	return best.text
}

// rankedRun is what namedRun orders a run by.
type rankedRun struct {
	text          string
	standing      int
	tokens        int
	oldest, start int
}

// before orders two runs for namedRun: more standing, more tokens, an older oldest post (a larger
// window index), an earlier start in it, then the smaller text.
func (r rankedRun) before(other rankedRun) bool {
	switch {
	case r.standing != other.standing:
		return r.standing > other.standing
	case r.tokens != other.tokens:
		return r.tokens > other.tokens
	case r.oldest != other.oldest:
		return r.oldest > other.oldest
	case r.start != other.start:
		return r.start < other.start
	default:
		return r.text < other.text
	}
}

// startIn is where a run's tokens first stand whole inside one unit of doc.
func startIn(doc Document, tokens []string) int {
	seq := indexSequence(doc, len(tokens))
	if at := seq.windows[windowKey(tokens)]; len(at) > 0 {
		return at[0]
	}
	return -1
}

// Repetition is M3 for one post: the share of the body's noun occurrences taken by the most
// frequent noun, and the share of the nouns the title contains that the body also contains
// (QUAL-9). Each half is nil when it cannot be computed. M3 names no noun (QUAL-43).
type Repetition struct {
	Share          *float64
	TitleRelevance *float64
}

// MeasureRepetition counts the post's returned nouns by the containment rule of QUAL-7, one unit
// at a time. A post with no returned nouns has no M3 at all.
func MeasureRepetition(s Sample) Repetition {
	nouns := normalizedNouns(s.Nouns)
	if len(nouns) == 0 {
		return Repetition{}
	}
	units := Units(s.Doc)
	unitTokens := make([][]string, len(units))
	for i, unit := range units {
		unitTokens[i] = Tokens(unit.Text)
	}
	occurrences := make(map[string]int, len(nouns))
	total := 0
	for _, noun := range nouns {
		tokens := Tokens(noun)
		for _, unit := range unitTokens {
			occurrences[noun] += Occurrences(unit, tokens, s.Language)
		}
		total += occurrences[noun]
	}
	var result Repetition
	if total > 0 {
		topCount := 0
		for _, noun := range nouns {
			topCount = max(topCount, occurrences[noun])
		}
		share := float64(topCount) / float64(total)
		result.Share = &share
	}
	title := Tokens(s.Doc.Title)
	inTitle, covered := 0, 0
	for _, noun := range nouns {
		if !Contains(title, Tokens(noun), s.Language) {
			continue
		}
		inTitle++
		if occurrences[noun] > 0 {
			covered++
		}
	}
	if inTitle > 0 {
		relevance := float64(covered) / float64(inTitle)
		result.TitleRelevance = &relevance
	}
	return result
}

// Composition is M4 for one post (QUAL-10): its character and photo counts, how many of the six
// block types it uses, and its average sentence length, nil when it has no sentence. Only the
// type count carries a band; the rest is context shown beside it.
type Composition struct {
	CharCount, PhotoCount, DistinctBlockTypes int
	AvgSentenceLength                         *float64
}

// MeasureComposition counts the text of TEXT, HEADING, QUOTE and LIST in runes, the photos as IMAGE
// blocks carrying a file (a clip is not a photo), a type only where one of its blocks carries
// something (an unfilled template position is not composition), and sentence length over TEXT
// alone — in runes for Korean and in words for English.
func MeasureComposition(s Sample) Composition {
	var c Composition
	used := map[BlockType]bool{}
	units := Units(s.Doc)
	for _, unit := range units {
		c.CharCount += utf8.RuneCountInString(norm.NFC.String(strings.TrimSpace(unit.Text)))
		used[unit.Type] = true
	}
	for _, block := range s.Doc.Blocks {
		if (block.Type == BlockImage || block.Type == BlockVideo) && strings.TrimSpace(block.File) != "" {
			used[block.Type] = true
			if block.Type == BlockImage {
				c.PhotoCount++
			}
		}
	}
	c.DistinctBlockTypes = len(used)
	total, sentences := 0, 0
	for _, unit := range units {
		if unit.Type != BlockText {
			continue
		}
		for _, sentence := range Sentences(unit.Text) {
			if s.Language == LanguageEnglish {
				total += len(Tokens(sentence))
			} else {
				total += utf8.RuneCountInString(norm.NFC.String(strings.TrimSpace(sentence)))
			}
			sentences++
		}
	}
	if sentences > 0 {
		average := float64(total) / float64(sentences)
		c.AvgSentenceLength = &average
	}
	return c
}

// Self is what one revision of one post measures on its own, and what is stored against that
// revision (QUAL-4): M3 and M4. M2 depends on the published window and is measured at read.
type Self struct {
	Repetition  Repetition
	Composition Composition
}

// MeasureSelf measures one post against itself.
func MeasureSelf(s Sample) Self {
	return Self{Repetition: MeasureRepetition(s), Composition: MeasureComposition(s)}
}

// Median is the middle of the present values, the mean of the two middle ones for an even count,
// and nil when none is present: an absent value neither pulls a median nor counts as zero
// (QUAL-40).
func Median(values []*float64) *float64 {
	var present []float64
	for _, value := range values {
		if value != nil {
			present = append(present, *value)
		}
	}
	if len(present) == 0 {
		return nil
	}
	sort.Float64s(present)
	mid := len(present) / 2
	median := present[mid]
	if len(present)%2 == 0 {
		median = (present[mid-1] + present[mid]) / 2
	}
	return &median
}
