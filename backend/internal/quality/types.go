package quality

import (
	"errors"
	"time"
)

// ErrPostNotFound covers an unknown slug and another account's alike: the two must not be
// distinguishable.
var ErrPostNotFound = errors.New("quality: post not found")

// StoredMeasurement is one revision's self-measurement as it is stored (QUAL-4): M3 and M4,
// stamped with the content revision and the MeasureVersion that produced them, so a row with
// either stamp different is stale.
type StoredMeasurement struct {
	PostSlug, UserID string
	Revision         int64
	MeasureVersion   int
	Self             Self
	ComputedAt       time.Time
}

// PhraseList is one field's phrases in rank order, as the daily batch last wrote them
// (QUAL-41). RefreshedAt is nil for a field whose first fetch failed.
type PhraseList struct {
	Field         string
	Phrases       []string
	CorpusSize    int
	RefreshedAt   *time.Time
	NextRefreshAt time.Time
}

// PostSnapshot is one post as this context reads it. Content nil means the post has none yet.
type PostSnapshot struct {
	Slug            string
	Revision        int64
	Content         *Document
	ContentLanguage *Language
	TargetLanguage  Language
	Nouns           []string
}

// PublishedPost is one post of the account's published window, which is read newest
// publication first (QUAL-39).
type PublishedPost struct {
	Slug            string
	Revision        int64
	Content         Document
	ContentLanguage *Language
	Nouns           []string
	PublishedAt     time.Time
}

// Language is the language a post's content is measured in. It decides the containment rule
// (QUAL-7) and nothing else.
type Language string

const (
	LanguageKorean  Language = "ko"
	LanguageEnglish Language = "en"
)

// LanguageOf is the language a post measures in. A post with no content language predates the
// field, and legacy content was Korean, so it measures as Korean (QUAL-26).
func LanguageOf(content *Language) Language {
	if content == nil {
		return LanguageKorean
	}
	return *content
}

// BlockType is a block's kind in the post context's own spelling. The adapter that builds a
// Document maps each kind with a closed switch.
type BlockType string

const (
	BlockText    BlockType = "TEXT"
	BlockHeading BlockType = "HEADING"
	BlockImage   BlockType = "IMAGE"
	BlockVideo   BlockType = "VIDEO"
	BlockQuote   BlockType = "QUOTE"
	BlockList    BlockType = "LIST"
)

// Block carries only what a metric reads: the text of a TEXT, HEADING or QUOTE block, a LIST's
// items, and a file, which is what makes an IMAGE block a photo (QUAL-10). Level, alt text and
// captions are never measured.
type Block struct {
	Type    BlockType
	Content string
	File    string
	Items   []string
}

// Document is one post's content as QUAL measures it. Title is the content's title, never the
// post's working title, because the published title is what a visitor sees (QUAL-7).
type Document struct {
	Title  string
	Blocks []Block
}
