package quality

import "strings"

// Occurrences counts where noun stands in tokens (QUAL-7, QUAL-9): the positions whose tokens
// equal the noun's leading tokens and whose next token contains its last one. A Korean 어절
// contains a noun when it starts with it, so a particle never hides one (감자탕을 contains
// 감자탕); an English word contains it only when equal ignoring case. Callers pass Tokens(noun),
// so a noun gets the same normalization as the text, and count one Unit at a time, so a noun
// never spans two blocks. An empty noun stands nowhere.
//
// The prefix rule is deliberately loose — 제주도에서 contains both 제주 and 제주도 — because
// that is what keeps a hand-edited title measurable without a morphological analyzer.
func Occurrences(tokens, noun []string, lang Language) int {
	if len(noun) == 0 {
		return 0
	}
	count := 0
	for at := 0; at+len(noun) <= len(tokens); at++ {
		if standsAt(tokens, at, noun, lang) {
			count++
		}
	}
	return count
}

// Contains reports whether noun stands anywhere in tokens.
func Contains(tokens, noun []string, lang Language) bool {
	return Occurrences(tokens, noun, lang) > 0
}

func standsAt(tokens []string, at int, noun []string, lang Language) bool {
	last := len(noun) - 1
	for i := 0; i < last; i++ {
		if !sameToken(tokens[at+i], noun[i], lang) {
			return false
		}
	}
	return containsToken(tokens[at+last], noun[last], lang)
}

// sameToken compares a multi-token noun's leading tokens, which must stand as written.
func sameToken(token, noun string, lang Language) bool {
	if lang == LanguageEnglish {
		return strings.EqualFold(token, noun)
	}
	return token == noun
}

// containsToken is the rule for the token a noun ends on. Every language but English takes the
// Korean rule, as a post with no content language does (QUAL-26).
func containsToken(token, noun string, lang Language) bool {
	if lang == LanguageEnglish {
		return strings.EqualFold(token, noun)
	}
	return strings.HasPrefix(token, noun)
}
