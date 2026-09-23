package quality

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
