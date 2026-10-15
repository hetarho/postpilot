// Package campaign is the advertiser guide a 체험단 post or clip carries (POST-118, CLIP-210):
// what the campaign asks of it, as pasted text plus the items the product can count. It is pure
// — no store, no context — so the post and clip contexts share one set of bounds without
// importing each other.
package campaign

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// Bounds on a guide (POST-118). Code constants, mirrored on the client.
const (
	// GuideMaxChars bounds the pasted text, counted in Unicode scalar values after trimming.
	GuideMaxChars = 3000
	// KeywordMax bounds the required keywords; HashtagMax the required hashtags, which join a
	// post's tags and so follow the tag ceiling (GEN-46).
	KeywordMax = 10
	HashtagMax = 30
	// ItemMaxChars bounds one keyword or hashtag.
	ItemMaxChars = 40
	// MinCharsFloor and MinCharsCeiling are the post's target-length bounds (POST-20).
	MinCharsFloor   = 100
	MinCharsCeiling = 10000
	// MinPhotosCeiling is a post's photo ceiling.
	MinPhotosCeiling = 30
)

// Field names a guide part in a refusal.
type Field string

const (
	FieldText      Field = "text"
	FieldKeywords  Field = "keywords"
	FieldHashtags  Field = "hashtags"
	FieldMinChars  Field = "min_chars"
	FieldMinPhotos Field = "min_photos"
)

// Guide is one advertiser guide. A nil MinChars or MinPhotos is "not asked".
type Guide struct {
	Text      string
	Keywords  []string
	MinChars  *int
	MinPhotos *int
	Hashtags  []string
}

// Empty reports whether the guide asks for nothing at all.
func (g Guide) Empty() bool {
	return strings.TrimSpace(g.Text) == "" && len(g.Keywords) == 0 && len(g.Hashtags) == 0 && g.MinChars == nil && g.MinPhotos == nil
}

// TooLongError is a part over its bound, with both counts so the caller reports the limit hit.
type TooLongError struct {
	Field Field
	Chars int
	Max   int
}

func (e *TooLongError) Error() string {
	return fmt.Sprintf("advertiser guide %s is %d long, at most %d", e.Field, e.Chars, e.Max)
}

// ErrItemInvalid is a counted item outside its bounds or a part the kind does not take.
var ErrItemInvalid = errors.New("advertiser guide item is invalid")

// ItemInvalidError names the part that was refused.
type ItemInvalidError struct{ Field Field }

func (e *ItemInvalidError) Error() string {
	return "advertiser guide " + string(e.Field) + " is invalid"
}
func (e *ItemInvalidError) Unwrap() error { return ErrItemInvalid }

// Options say which counted items a kind takes: a clip has no character or photo minimum
// (CLIP-210).
type Options struct {
	CountsAllowed bool
}

// PostOptions and ClipOptions are the two kinds' options.
var (
	PostOptions = Options{CountsAllowed: true}
	ClipOptions = Options{CountsAllowed: false}
)

// Normalize trims the text, trims and de-duplicates the keywords and hashtags — a hashtag loses
// one leading '#', and two items equal ignoring case and spaces keep the first — and checks every
// bound. A refusal names its part and changes nothing.
func Normalize(g Guide, options Options) (Guide, error) {
	text := strings.TrimSpace(g.Text)
	if chars := utf8.RuneCountInString(text); chars > GuideMaxChars {
		return Guide{}, &TooLongError{Field: FieldText, Chars: chars, Max: GuideMaxChars}
	}
	keywords, err := items(g.Keywords, FieldKeywords, KeywordMax, false)
	if err != nil {
		return Guide{}, err
	}
	hashtags, err := items(g.Hashtags, FieldHashtags, HashtagMax, true)
	if err != nil {
		return Guide{}, err
	}
	out := Guide{Text: text, Keywords: keywords, Hashtags: hashtags}
	if g.MinChars != nil {
		if !options.CountsAllowed || *g.MinChars < MinCharsFloor || *g.MinChars > MinCharsCeiling {
			return Guide{}, &ItemInvalidError{Field: FieldMinChars}
		}
		value := *g.MinChars
		out.MinChars = &value
	}
	if g.MinPhotos != nil {
		if !options.CountsAllowed || *g.MinPhotos < 1 || *g.MinPhotos > MinPhotosCeiling {
			return Guide{}, &ItemInvalidError{Field: FieldMinPhotos}
		}
		value := *g.MinPhotos
		out.MinPhotos = &value
	}
	return out, nil
}

func items(raw []string, field Field, max int, hashtag bool) ([]string, error) {
	out := make([]string, 0, len(raw))
	seen := make(map[string]bool, len(raw))
	for _, item := range raw {
		value := strings.TrimSpace(item)
		if hashtag {
			value = strings.TrimSpace(strings.TrimPrefix(value, "#"))
		}
		if value == "" {
			return nil, &ItemInvalidError{Field: field}
		}
		if chars := utf8.RuneCountInString(value); chars > ItemMaxChars {
			return nil, &TooLongError{Field: field, Chars: chars, Max: ItemMaxChars}
		}
		key := strings.ToLower(strings.Join(strings.Fields(value), ""))
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, value)
	}
	if len(out) > max {
		return nil, &ItemInvalidError{Field: field}
	}
	return out, nil
}

// Ticks normalizes a checklist's ticked lines (POST-119): trimmed, distinct, at most
// TickMax lines of at most TickMaxChars each.
const (
	TickMax      = 200
	TickMaxChars = 500
)

// NormalizeTicks trims and de-duplicates ticked lines and refuses an over-long list or line.
func NormalizeTicks(lines []string) ([]string, error) {
	out := make([]string, 0, len(lines))
	seen := make(map[string]bool, len(lines))
	for _, line := range lines {
		value := strings.TrimSpace(line)
		if value == "" || seen[value] {
			continue
		}
		if utf8.RuneCountInString(value) > TickMaxChars {
			return nil, &ItemInvalidError{Field: "checklist_ticks"}
		}
		seen[value] = true
		out = append(out, value)
	}
	if len(out) > TickMax {
		return nil, &ItemInvalidError{Field: "checklist_ticks"}
	}
	return out, nil
}
