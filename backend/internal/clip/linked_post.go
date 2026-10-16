package clip

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

// LinkedPosts is an owner-scoped read port; the clip context never reads post tables.
type LinkedPosts interface {
	Get(ctx context.Context, user, slug string) (LinkedPost, error)
	Text(ctx context.Context, user, slug string) (LinkedPostDocument, error)
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

// LinkedPostDocument fences canonical text with the identity from that same read.
type LinkedPostDocument struct {
	CreatedAt time.Time
	Text      *LinkedPostText
}

type LinkedPostText struct {
	CaptureNeighbors map[string][]int    `json:"capture_neighbors,omitempty"`
	Slug             string              `json:"slug"`
	Title            string              `json:"title"`
	Sections         []LinkedPostSection `json:"sections"`
}
type LinkedPhoto struct {
	File       string     `json:"file"`
	CapturedAt *time.Time `json:"captured_at,omitempty"`
}
type LinkedPostSection struct {
	Photos  []LinkedPhoto `json:"photos,omitempty"`
	Heading string        `json:"heading"`
	Text    string        `json:"text"`
}

func (p *LinkedPostText) Digest() string {
	if p == nil {
		return ""
	}
	raw, _ := json.Marshal(p)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
