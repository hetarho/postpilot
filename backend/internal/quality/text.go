package quality

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Tokens splits text into 어절: NFC first, so a decomposed Hangul input compares equal to its
// composed noun, then Unicode whitespace, then each 어절 loses the punctuation and symbols at
// its edges. Symbols go too, so a trailing ~, ^^, # or emoji never blocks a match, while inner
// punctuation stays (1,000원, 3.5km). An 어절 that trims to nothing is dropped.
func Tokens(text string) []string {
	fields := strings.Fields(norm.NFC.String(text))
	tokens := make([]string, 0, len(fields))
	for _, field := range fields {
		if token := strings.TrimFunc(field, edgeRune); token != "" {
			tokens = append(tokens, token)
		}
	}
	return tokens
}

func edgeRune(r rune) bool { return unicode.IsPunct(r) || unicode.IsSymbol(r) }

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
