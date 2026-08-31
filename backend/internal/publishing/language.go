package publishing

import "fmt"

// Language is the publishing context's canonical frozen language tag. It is copied
// from the post-owned snapshot; this context never infers or changes it.
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

func (l Language) Valid() bool { return l == LanguageKorean || l == LanguageEnglish }

func (l Language) String() string { return string(l) }
