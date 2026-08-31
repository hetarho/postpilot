package experiment

import "fmt"

// Language is the experiment context's canonical frozen stage language. It is present
// only for write comparisons; adapters map it to post/voice/proto values at boundaries.
type Language string

const (
	LanguageKorean  Language = "ko"
	LanguageEnglish Language = "en"
)

func ParseLanguage(value string) (Language, error) {
	language := Language(value)
	if !language.Valid() {
		return "", fmt.Errorf("%w: %q", ErrLanguageRequired, value)
	}
	return language, nil
}

func (l Language) Valid() bool {
	return l == LanguageKorean || l == LanguageEnglish
}

func (l Language) String() string { return string(l) }

func cloneLanguage(value *Language) *Language {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
