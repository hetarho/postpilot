package voice

import (
	"regexp"
	"strings"
	"unicode"
)

// A line that opens with one of these labels is a place card's info line (VOICE-61).
var infoLine = regexp.MustCompile(`^(주소|영업시간|운영시간|전화|휴무|주차|가격|위치)(\s*[:：]|\s)`)

// The 시·도 an address line starts with, full and short forms.
var addressProvinces = []string{
	"서울특별시", "부산광역시", "대구광역시", "인천광역시", "광주광역시", "대전광역시", "울산광역시", "세종특별자치시",
	"경기도", "강원특별자치도", "강원도", "충청북도", "충청남도", "전북특별자치도", "전라북도", "전라남도", "경상북도", "경상남도", "제주특별자치도",
	"서울", "부산", "대구", "인천", "광주", "대전", "울산", "세종", "경기", "강원", "충북", "충남", "전북", "전남", "경북", "경남", "제주",
}

// ProseLines keeps the lines of a 학습 글 that are the owner's own prose. A hashtag-only line, a
// `[출처]` line, an info line, a Korean address line, an empty line and a line with no Hangul
// feed no sentence and no item (VOICE-61) ← a pasted Naver post carries its place card and
// hashtags.
func ProseLines(text string) []string {
	var kept []string
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if isProse(line) {
			kept = append(kept, strings.TrimSpace(line))
		}
	}
	return kept
}

// ProseText is the prose lines joined back into one text, one line per line.
func ProseText(text string) string { return strings.Join(ProseLines(text), "\n") }

// ProseSentences segments only the prose lines, the way every count of a voice does.
func ProseSentences(text string) []string { return SegmentSentences(ProseText(text)) }

func isProse(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" || !hasHangul(line) {
		return false
	}
	if hashtagOnly(line) || strings.HasPrefix(line, "[출처]") || infoLine.MatchString(line) {
		return false
	}
	for _, marker := range []string{"📍", "⏰", "☎"} {
		if strings.HasPrefix(line, marker) {
			return false
		}
	}
	return !addressLine(line)
}

func hashtagOnly(line string) bool {
	for _, token := range strings.Fields(line) {
		if !strings.HasPrefix(token, "#") {
			return false
		}
	}
	return true
}

// addressLine is a line that starts with a 시·도 and holds a 시·군·구 token and a digit.
func addressLine(line string) bool {
	starts := false
	for _, province := range addressProvinces {
		if strings.HasPrefix(line, province) {
			starts = true
			break
		}
	}
	if !starts || !strings.ContainsFunc(line, unicode.IsDigit) {
		return false
	}
	for _, token := range strings.Fields(line) {
		if strings.HasSuffix(token, "시") || strings.HasSuffix(token, "군") || strings.HasSuffix(token, "구") {
			return true
		}
	}
	return false
}

// hasHangul reports a Hangul syllable or compatibility jamo (ㅎㅎ, ㅠㅠ count).
func hasHangul(line string) bool {
	for _, r := range line {
		if (r >= 0xAC00 && r <= 0xD7A3) || (r >= 0x3131 && r <= 0x318E) {
			return true
		}
	}
	return false
}
