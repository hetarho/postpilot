package quality

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Tokens splits text into 어절: NFC first, so a decomposed Hangul input compares equal to its
// composed noun, then Unicode whitespace, then each 어절 loses what stands at its edges without
// being part of the word — punctuation, symbols, and the invisible parts of an emoji (variation
// selectors, ZWJ and other format runes, enclosing marks), with a keycap (a digit, # or *, then
// U+FE0F, then U+20E3) dropped whole so its digit does not stay behind. A trailing ~, ^^, # or
// emoji therefore never blocks a match, while inner punctuation stays (1,000원, 3.5km). An 어절
// that trims to nothing is dropped.
func Tokens(text string) []string {
	fields := strings.Fields(norm.NFC.String(text))
	tokens := make([]string, 0, len(fields))
	for _, field := range fields {
		if token := trimEdges(field); token != "" {
			tokens = append(tokens, token)
		}
	}
	return tokens
}

// edgeRune is a rune that is never part of a word at an 어절's edge. None of these classes holds
// a letter, a digit or a Hangul rune, and other combining marks stay, so an accent NFC could not
// compose is never cut.
func edgeRune(r rune) bool {
	return unicode.IsPunct(r) || unicode.IsSymbol(r) ||
		unicode.Is(unicode.Variation_Selector, r) || unicode.Is(unicode.Cf, r) || unicode.Is(unicode.Me, r)
}

const (
	variationSelector16 = '\ufe0f'
	combiningKeycap     = '\u20e3'
)

func keycapBase(r rune) bool { return (r >= '0' && r <= '9') || r == '#' || r == '*' }

// trimEdges drops edge runes from both ends, a whole keycap sequence (base, optional U+FE0F,
// U+20E3) at a time before a single rune: trimming rune by rune would leave a keycap's digit
// behind (추천 + keycap 1 would become 추천1).
func trimEdges(field string) string {
	runes := []rune(field)
	start, end := 0, len(runes)
	for start < end {
		if n := keycapAt(runes[start:end]); n > 0 {
			start += n
			continue
		}
		if !edgeRune(runes[start]) {
			break
		}
		start++
	}
	for start < end {
		if n := keycapBefore(runes[start:end]); n > 0 {
			end -= n
			continue
		}
		if !edgeRune(runes[end-1]) {
			break
		}
		end--
	}
	return string(runes[start:end])
}

// keycapAt is the length of the keycap sequence opening runes, or 0.
func keycapAt(runes []rune) int {
	if len(runes) < 2 || !keycapBase(runes[0]) {
		return 0
	}
	i := 1
	if runes[i] == variationSelector16 {
		i++
	}
	if i < len(runes) && runes[i] == combiningKeycap {
		return i + 1
	}
	return 0
}

// keycapBefore is the length of the keycap sequence closing runes, or 0.
func keycapBefore(runes []rune) int {
	n := len(runes)
	if n < 2 || runes[n-1] != combiningKeycap {
		return 0
	}
	i := n - 2
	if runes[i] == variationSelector16 {
		i--
	}
	if i >= 0 && keycapBase(runes[i]) {
		return n - i
	}
	return 0
}

// slotToken is an unfilled template slot's marker inside a TEXT block. A post block carries no
// slot field of its own, so the token in its content is how one is recognized.
var slotToken = regexp.MustCompile(`\{\{slot:[0-9]+\}\}`)

// Unit is one stretch of text a run may not cross: a block's text, or one LIST item.
type Unit struct {
	Type BlockType
	Text string
}

// Units is a document's measured text in block order: the content of each TEXT, HEADING and
// QUOTE block and each LIST item. Photos and clips carry no text, and an unfilled slot's token
// is not prose, so a TEXT block holding only one gives nothing; a unit left blank is dropped.
func Units(doc Document) []Unit {
	var units []Unit
	add := func(kind BlockType, text string) {
		if strings.TrimSpace(text) != "" {
			units = append(units, Unit{Type: kind, Text: text})
		}
	}
	for _, block := range doc.Blocks {
		switch block.Type {
		case BlockText:
			// A space, not nothing, so a token set between two words cannot join them into one.
			add(block.Type, slotToken.ReplaceAllString(block.Content, " "))
		case BlockHeading, BlockQuote:
			add(block.Type, block.Content)
		case BlockList:
			for _, item := range block.Items {
				add(block.Type, item)
			}
		}
	}
	return units
}
