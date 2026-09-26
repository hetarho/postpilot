package quality

import "context"

// Measurements is the per-revision store (QUAL-4), declared here by its consumer (ARCH-6).
type Measurements interface {
	// Measurement reads a post's row; false means it has none.
	Measurement(ctx context.Context, userID, slug string) (StoredMeasurement, bool, error)
	// SaveMeasurement writes the row, replacing whatever revision it held.
	SaveMeasurement(ctx context.Context, m StoredMeasurement) error
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
