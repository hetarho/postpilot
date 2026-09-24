package quality

import "context"

// Measurements is the per-revision store (QUAL-4), declared here by its consumer (ARCH-6).
type Measurements interface {
	// Measurement reads a post's row; false means it has none.
	Measurement(ctx context.Context, userID, slug string) (StoredMeasurement, bool, error)
	// SaveMeasurement writes the row, replacing whatever revision it held.
	SaveMeasurement(ctx context.Context, m StoredMeasurement) error
}

// PhraseLists is the per-field phrase store. The service only reads it; the daily batch reads
// a field's row and replaces it whole, which is all it needs.
type PhraseLists interface {
	// PhraseList reads one field's row; false means the field has none yet.
	PhraseList(ctx context.Context, field string) (PhraseList, bool, error)
	// ReplacePhraseList writes a field's whole row, inserting it when the field has none.
	ReplacePhraseList(ctx context.Context, list PhraseList) error
}

// PostSource is the post context as this one reads it. It names only what the measurements
// need: a post's content and languages, and the account's published window. Published-ness
// arrives through Published alone, so no status is read.
type PostSource interface {
	// Post reads one owned post, ErrPostNotFound for an unknown or foreign slug.
	Post(ctx context.Context, userID, slug string) (PostSnapshot, error)
	// Published is the account's published posts, newest publication first, at most limit.
	Published(ctx context.Context, userID string, limit int) ([]PublishedPost, error)
}

// BlogSearch is the Naver blog search as the phrase batch reads it: one page of display plain
// titles and descriptions from start (1-based). The port is this context's, so the wrapper that
// implements it never imports quality (ARCH-6).
type BlogSearch interface {
	SearchBlog(ctx context.Context, query string, start, display int) ([]SearchItem, error)
}
