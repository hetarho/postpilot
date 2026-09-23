package quality

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Sentences splits text into the sentences M4's average length is taken over (QUAL-10). A
// boundary follows any run of . ! ? …, so ..., ?! and …… each end one sentence, and every line
// break is one. A . between two ASCII digits is a decimal point (3.5km, 1.5배), not a boundary.
// Each sentence keeps its terminators and is trimmed; a piece with no letter or digit is
// dropped. Abbreviations get no special case.
func Sentences(text string) []string {
	runes := []rune(norm.NFC.String(text))
	var sentences []string
	var current []rune
	flush := func() {
		piece := strings.TrimSpace(string(current))
		current = current[:0]
		if strings.IndexFunc(piece, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) >= 0 {
			sentences = append(sentences, piece)
		}
	}
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case r == '\n' || r == '\r':
			flush()
		case r == '.' && i > 0 && i+1 < len(runes) && isASCIIDigit(runes[i-1]) && isASCIIDigit(runes[i+1]):
			current = append(current, r)
		case isTerminator(r):
			for i < len(runes) && isTerminator(runes[i]) {
				current = append(current, runes[i])
				i++
			}
			i--
			flush()
		default:
			current = append(current, r)
		}
	}
	flush()
	return sentences
}

func isTerminator(r rune) bool { return r == '.' || r == '!' || r == '?' || r == '…' }

func isASCIIDigit(r rune) bool { return r >= '0' && r <= '9' }
