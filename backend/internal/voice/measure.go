package voice

import (
	"math"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type Measurements struct {
	Sentences                  []string
	AverageSentenceChars       float64
	EndingDistribution         []EndingRatio
	ParagraphMin, ParagraphMax int
}

// SegmentSentences is deterministic and dependency-free. It recognizes Korean/Latin
// terminal punctuation and keeps punctuation with the sentence so endings remain
// measurable. Newlines end an otherwise unpunctuated sentence.
func SegmentSentences(text string) []string {
	var out []string
	start := 0
	for i, r := range text {
		if r != '.' && r != '!' && r != '?' && r != '。' && r != '\n' {
			continue
		}
		end := i + utf8.RuneLen(r)
		if value := strings.TrimSpace(text[start:end]); value != "" {
			out = append(out, value)
		}
		start = end
	}
	if value := strings.TrimSpace(text[start:]); value != "" {
		out = append(out, value)
	}
	return out
}

func Measure(text string) Measurements {
	sentences := SegmentSentences(text)
	counts := map[string]int{"다": 0, "해요": 0, "습니다": 0, "기타": 0}
	totalChars := 0
	for _, sentence := range sentences {
		plain := strings.TrimRightFunc(strings.TrimSpace(sentence), func(r rune) bool { return unicode.IsPunct(r) || unicode.IsSpace(r) })
		totalChars += utf8.RuneCountInString(plain)
		switch {
		case strings.HasSuffix(plain, "습니다") || strings.HasSuffix(plain, "ㅂ니다"):
			counts["습니다"]++
		case strings.HasSuffix(plain, "해요") || strings.HasSuffix(plain, "어요") || strings.HasSuffix(plain, "아요") || strings.HasSuffix(plain, "요"):
			counts["해요"]++
		case strings.HasSuffix(plain, "다"):
			counts["다"]++
		default:
			counts["기타"]++
		}
	}
	distribution := make([]EndingRatio, 0, 4)
	for _, ending := range []string{"다", "해요", "습니다", "기타"} {
		ratio := 0.0
		if len(sentences) > 0 {
			ratio = math.Round(float64(counts[ending])*10000/float64(len(sentences))) / 10000
		}
		distribution = append(distribution, EndingRatio{Ending: ending, Ratio: ratio})
	}
	avg := 0.0
	if len(sentences) > 0 {
		avg = math.Round(float64(totalChars)*100/float64(len(sentences))) / 100
	}
	minP, maxP := 0, 0
	for _, paragraph := range strings.Split(text, "\n") {
		n := len(SegmentSentences(paragraph))
		if n == 0 {
			continue
		}
		if minP == 0 || n < minP {
			minP = n
		}
		if n > maxP {
			maxP = n
		}
	}
	return Measurements{Sentences: sentences, AverageSentenceChars: avg, EndingDistribution: distribution, ParagraphMin: minP, ParagraphMax: maxP}
}

func MeasuredProfile(text string, nowTime func() time.Time) StructuredProfile {
	m := Measure(text)
	register := VoiceValue{Unknown: true, Source: SourceUnknown}
	best := EndingRatio{}
	for _, item := range m.EndingDistribution {
		if item.Ending != "기타" && item.Ratio > best.Ratio {
			best = item
		}
	}
	if best.Ratio > 0 {
		register = VoiceValue{Value: best.Ending, Source: SourceMeasured}
	}
	unknown := VoiceValue{Unknown: true, Source: SourceUnknown}
	return StructuredProfile{UpdatedAt: nowTime(), Lexical: LexicalProfile{Description: unknown}, Endings: EndingsProfile{BaseRegister: register, Distribution: m.EndingDistribution}, Syntax: SyntaxProfile{AverageSentenceChars: m.AverageSentenceChars, SentenceLength: VoiceValue{Value: formatMeasurement(m.AverageSentenceChars), Source: SourceMeasured}, ConnectiveStyle: unknown, Nominalization: unknown, PassiveTendency: unknown}, Structure: StructureProfile{IntroPattern: unknown, ClosingPattern: unknown, ParagraphSentencesMin: m.ParagraphMin, ParagraphSentencesMax: m.ParagraphMax, HeadingHabit: unknown, ListHabit: unknown, EmojiUse: unknown}}
}

func formatMeasurement(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) + "자" }
