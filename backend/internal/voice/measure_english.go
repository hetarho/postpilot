package voice

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var englishStopWords = map[string]struct{}{
	"a": {}, "an": {}, "and": {}, "are": {}, "as": {}, "at": {}, "be": {}, "but": {}, "by": {}, "for": {}, "from": {},
	"has": {}, "have": {}, "he": {}, "her": {}, "his": {}, "i": {}, "in": {}, "is": {}, "it": {}, "its": {}, "of": {}, "on": {},
	"or": {}, "our": {}, "she": {}, "that": {}, "the": {}, "their": {}, "they": {}, "this": {}, "to": {}, "was": {}, "we": {},
	"were": {}, "with": {}, "you": {}, "your": {},
}

var englishConnectives = []string{
	"however", "therefore", "moreover", "meanwhile", "instead", "otherwise", "because", "although", "though", "while", "then", "also", "finally", "first", "second",
}

func MeasuredProfileForLanguage(text string, language Language, nowTime func() time.Time) StructuredProfile {
	if language != LanguageEnglish {
		return MeasuredProfile(text, nowTime)
	}
	return measuredEnglishProfile(text, nowTime)
}

func englishWords(text string) []string {
	parts := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && r != '\'' && r != '’'
	})
	out := parts[:0]
	for _, part := range parts {
		part = strings.Trim(part, "'’")
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func measuredEnglishProfile(text string, nowTime func() time.Time) StructuredProfile {
	base := Measure(text)
	sentences := base.Sentences
	words := englishWords(text)
	unknown := VoiceValue{Unknown: true, Source: SourceUnknown}
	measured := func(value string) VoiceValue { return VoiceValue{Value: value, Source: SourceMeasured} }

	wordTotal := len(words)
	averageWords := 0.0
	if len(sentences) > 0 {
		averageWords = roundHundredths(float64(wordTotal) / float64(len(sentences)))
	}

	contractions := 0
	for _, word := range words {
		if englishContraction(word) {
			contractions++
		}
	}
	register := unknown
	if wordTotal > 0 {
		if contractions > 0 {
			register = measured("conversational (contractions present)")
		} else {
			register = measured("formal (uncontracted)")
		}
	}

	cadenceCounts := map[string]int{"statement": 0, "question": 0, "exclamation": 0, "fragment": 0}
	for _, sentence := range sentences {
		switch lastNonSpaceRune(sentence) {
		case '?':
			cadenceCounts["question"]++
		case '!':
			cadenceCounts["exclamation"]++
		case '.', '。':
			cadenceCounts["statement"]++
		default:
			cadenceCounts["fragment"]++
		}
	}
	cadence := make([]EndingRatio, 0, 4)
	for _, kind := range []string{"statement", "question", "exclamation", "fragment"} {
		ratio := 0.0
		if len(sentences) > 0 {
			ratio = roundFour(float64(cadenceCounts[kind]) / float64(len(sentences)))
		}
		cadence = append(cadence, EndingRatio{Ending: kind, Ratio: ratio})
	}

	connectiveCounts := map[string]int{}
	for _, word := range words {
		for _, connective := range englishConnectives {
			if word == connective {
				connectiveCounts[connective]++
			}
		}
	}
	preferredConnectives := sortedTermsByCount(connectiveCounts, 5)
	connectiveStyle := unknown
	if wordTotal > 0 {
		connectiveStyle = measured(fmt.Sprintf("%d explicit connectives per 100 words", roundedRate(sumCounts(connectiveCounts), wordTotal)))
	}

	passiveCount := englishPassiveSentences(sentences)
	nominalCount := englishNominalizations(words)
	passive := unknown
	nominal := unknown
	if len(sentences) > 0 {
		passive = measured(fmt.Sprintf("%d of %d sentences", passiveCount, len(sentences)))
	}
	if wordTotal > 0 {
		nominal = measured(fmt.Sprintf("%d nominalizations per 100 words", roundedRate(nominalCount, wordTotal)))
	}

	lexicalCounts := map[string]int{}
	letterTotal := 0
	for _, word := range words {
		letterTotal += utf8.RuneCountInString(word)
		if _, stop := englishStopWords[word]; !stop && utf8.RuneCountInString(word) > 2 {
			lexicalCounts[word]++
		}
	}
	lexical := unknown
	preferredWords := make([]WeightedWord, 0, 5)
	if wordTotal > 0 {
		unique := map[string]struct{}{}
		for _, word := range words {
			unique[word] = struct{}{}
		}
		lexical = measured(fmt.Sprintf("%.2f average word characters; %.2f type-token ratio; %d contractions per 100 words", float64(letterTotal)/float64(wordTotal), float64(len(unique))/float64(wordTotal), roundedRate(contractions, wordTotal)))
		for _, word := range sortedTermsByCount(lexicalCounts, 5) {
			preferredWords = append(preferredWords, WeightedWord{Word: word, Weight: lexicalCounts[word]})
		}
	}

	intro, closing := unknown, unknown
	if len(sentences) > 0 {
		intro = measured(englishCadenceKind(sentences[0]) + " opening")
		closing = measured(englishCadenceKind(sentences[len(sentences)-1]) + " closing")
	}
	heading, list, emoji := measured("absent"), measured("absent"), measured("absent")
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") || englishHeadingLine(trimmed) {
			heading = measured("present")
		}
		if englishListLine(trimmed) {
			list = measured("present")
		}
	}
	if containsEmojiLikeSymbol(text) {
		emoji = measured("present")
	}

	axes := englishAxes(words, sentences, nominalCount)
	return StructuredProfile{
		UpdatedAt: nowTime(),
		Lexical:   LexicalProfile{Description: lexical, PreferredWords: preferredWords},
		Endings:   EndingsProfile{BaseRegister: register, Distribution: cadence},
		Syntax: SyntaxProfile{
			AverageSentenceChars: base.AverageSentenceChars, AverageSentenceWords: &averageWords,
			SentenceLength: measured(fmt.Sprintf("%.2f words", averageWords)), ConnectiveStyle: connectiveStyle,
			PreferredConnectives: preferredConnectives, Nominalization: nominal, PassiveTendency: passive,
		},
		Structure: StructureProfile{
			IntroPattern: intro, ClosingPattern: closing, ParagraphSentencesMin: base.ParagraphMin, ParagraphSentencesMax: base.ParagraphMax,
			HeadingHabit: heading, ListHabit: list, EmojiUse: emoji,
		},
		Axes: axes,
	}
}

func englishContraction(word string) bool {
	word = strings.ReplaceAll(word, "’", "'")
	for _, suffix := range []string{"n't", "'re", "'ve", "'ll", "'d", "'m"} {
		if strings.HasSuffix(word, suffix) && len(word) > len(suffix) {
			return true
		}
	}
	// 's is ambiguous, so only closed-class forms that cannot be a person's or
	// object's possessive count as contractions.
	switch word {
	case "it's", "he's", "she's", "that's", "there's", "here's", "what's", "who's", "where's", "when's", "why's", "how's", "let's":
		return true
	default:
		return false
	}
}

func englishPassiveSentences(sentences []string) int {
	be := map[string]struct{}{"am": {}, "are": {}, "be": {}, "been": {}, "being": {}, "is": {}, "was": {}, "were": {}, "get": {}, "gets": {}, "got": {}}
	irregular := map[string]struct{}{"built": {}, "done": {}, "driven": {}, "given": {}, "known": {}, "made": {}, "seen": {}, "shown": {}, "taken": {}, "told": {}, "written": {}}
	count := 0
	for _, sentence := range sentences {
		words := englishWords(sentence)
		matched := false
		for i := 0; i+1 < len(words) && !matched; i++ {
			if _, ok := be[words[i]]; !ok {
				continue
			}
			for j := i + 1; j < min(i+4, len(words)); j++ {
				_, special := irregular[words[j]]
				if special || strings.HasSuffix(words[j], "ed") || strings.HasSuffix(words[j], "en") {
					matched = true
					break
				}
			}
		}
		if matched {
			count++
		}
	}
	return count
}

func englishNominalizations(words []string) int {
	count := 0
	for _, word := range words {
		for _, suffix := range []string{"tion", "sion", "ment", "ness", "ity", "ance", "ence", "ism"} {
			if len(word) > len(suffix)+2 && strings.HasSuffix(word, suffix) {
				count++
				break
			}
		}
	}
	return count
}

func englishAxes(words, sentences []string, nominalCount int) AxesProfile {
	if len(words) == 0 {
		return AxesProfile{}
	}
	counts := map[string]int{}
	for _, word := range words {
		counts[word]++
	}
	axis := func(value int) *int {
		value = max(-3, min(3, value))
		return &value
	}
	firstPerson := counts["i"] + counts["we"] + counts["my"] + counts["our"]
	secondPerson := counts["you"] + counts["your"]
	narrative := counts["then"] + counts["when"] + counts["yesterday"] + counts["today"] + counts["later"]
	persuasion := counts["should"] + counts["must"] + counts["recommend"] + counts["because"]
	humor := counts["haha"] + counts["lol"] + counts["funny"] + counts["joke"]
	questions, exclamations := 0, 0
	for _, sentence := range sentences {
		switch lastNonSpaceRune(sentence) {
		case '?':
			questions++
		case '!':
			exclamations++
		}
	}
	return AxesProfile{
		Involvement:         axis(scorePresence(firstPerson+exclamations) - scorePresence(nominalCount)),
		Narrativity:         axis(scorePresence(narrative+counts["was"]+counts["were"]) - scorePresence(nominalCount)),
		PersuasionOvertness: axis(scorePresence(persuasion)),
		Abstractness:        axis(scorePresence(nominalCount) - scorePresence(narrative)),
		AddresseeFocus:      axis(scorePresence(secondPerson + questions)),
		Humor:               axis(scorePresence(humor)),
	}
}

func scorePresence(value int) int {
	switch {
	case value >= 6:
		return 3
	case value >= 3:
		return 2
	case value > 0:
		return 1
	default:
		return 0
	}
}

func englishCadenceKind(sentence string) string {
	switch lastNonSpaceRune(sentence) {
	case '?':
		return "question"
	case '!':
		return "exclamation"
	case '.', '。':
		return "statement"
	default:
		return "fragment"
	}
}

func lastNonSpaceRune(value string) rune {
	var last rune
	for _, current := range value {
		if !unicode.IsSpace(current) {
			last = current
		}
	}
	return last
}

func englishHeadingLine(line string) bool {
	words := strings.Fields(line)
	if len(words) == 0 || len(words) > 8 || strings.ContainsAny(line, ".!?") {
		return false
	}
	for _, word := range words {
		first, _ := utf8.DecodeRuneInString(word)
		if unicode.IsLetter(first) && !unicode.IsUpper(first) {
			return false
		}
	}
	return true
}

func englishListLine(line string) bool {
	if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") || strings.HasPrefix(line, "• ") {
		return true
	}
	first, rest, ok := strings.Cut(line, ". ")
	if !ok || first == "" {
		return false
	}
	for _, r := range first {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return strings.TrimSpace(rest) != ""
}

func containsEmojiLikeSymbol(value string) bool {
	for _, r := range value {
		if unicode.Is(unicode.So, r) && r > 127 {
			return true
		}
	}
	return false
}

func sortedTermsByCount(values map[string]int, limit int) []string {
	terms := make([]string, 0, len(values))
	for term := range values {
		terms = append(terms, term)
	}
	sort.Slice(terms, func(i, j int) bool {
		if values[terms[i]] != values[terms[j]] {
			return values[terms[i]] > values[terms[j]]
		}
		return terms[i] < terms[j]
	})
	if len(terms) > limit {
		terms = terms[:limit]
	}
	return terms
}

func sumCounts(values map[string]int) int {
	total := 0
	for _, count := range values {
		total += count
	}
	return total
}

func roundedRate(count, total int) int {
	if total == 0 {
		return 0
	}
	return int(math.Round(float64(count) * 100 / float64(total)))
}

func roundHundredths(value float64) float64 { return math.Round(value*100) / 100 }
func roundFour(value float64) float64       { return math.Round(value*10000) / 10000 }
