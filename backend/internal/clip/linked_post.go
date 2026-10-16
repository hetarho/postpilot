package clip

import (
	"context"
	"time"
)

// LinkedPosts is an owner-scoped read port; the clip context never reads post tables.
type LinkedPosts interface {
	Get(ctx context.Context, user, slug string) (LinkedPost, error)
}

type LinkedPost struct {
	Slug, Title          string
	CreatedAt, UpdatedAt time.Time
	ContentReady         bool
}

// A slug can be reused after deletion. Both values identify the original post.
type LinkedPostIdentity struct {
	Slug      string
	CreatedAt time.Time
}
