package voice

import (
	"strings"
	"unicode"
)

// SegmentSentences is deterministic and dependency-free. It recognizes Korean/Latin
// terminal punctuation and keeps punctuation with the sentence so endings remain
// measurable. A run of end marks (`!!`, `?!`, `.~`) and the emoji or ㅎㅎ·ㅠㅠ written right
// after it stay with the sentence they close. Newlines end an otherwise unpunctuated sentence.
func SegmentSentences(text string) []string {
	var out []string
	runes := []rune(text)
	start := 0
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r != '.' && r != '!' && r != '?' && r != '。' && r != '\n' {
			continue
		}
		end := i + 1
		if r != '\n' {
			for end < len(runes) && isTrailingMark(runes[end]) {
				end++
			}
			for end < len(runes) && (isEmojiRune(runes[end]) || isJamo(runes[end])) {
				end++
			}
		}
		if value := strings.TrimSpace(string(runes[start:end])); value != "" {
			out = append(out, value)
		}
		start = end
		i = end - 1
	}
	if value := strings.TrimSpace(string(runes[start:])); value != "" {
		out = append(out, value)
	}
	return out
}

// isTrailingMark is a rune that continues a sentence's final run of marks.
func isTrailingMark(r rune) bool {
	switch r {
	case '.', '!', '?', '。', '~', '～', '…':
		return true
	}
	return false
}

// endingOf classes a sentence by its ending: 습니다, 해요, 다 or 기타.
func endingOf(sentence string) string {
	v := strings.TrimRightFunc(strings.TrimSpace(sentence), func(r rune) bool { return unicode.IsPunct(r) || unicode.IsSpace(r) })
	if strings.HasSuffix(v, "습니다") || strings.HasSuffix(v, "니다") || strings.HasSuffix(v, "ㅂ니다") {
		return "습니다"
	}
	if strings.HasSuffix(v, "해요") || strings.HasSuffix(v, "어요") || strings.HasSuffix(v, "아요") || strings.HasSuffix(v, "요") {
		return "해요"
	}
	if strings.HasSuffix(v, "다") {
		return "다"
	}
	return "기타"
}
