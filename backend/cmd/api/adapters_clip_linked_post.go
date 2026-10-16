package main

import (
	"context"
	"errors"
	"strings"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/post"
)

type clipLinkedPosts struct{ service *post.Service }

func (a clipLinkedPosts) Get(ctx context.Context, user, slug string) (clip.LinkedPost, error) {
	p, err := a.service.Get(ctx, user, slug)
	if errors.Is(err, post.ErrNotFound) || errors.Is(err, post.ErrForbidden) {
		return clip.LinkedPost{}, clip.ErrNotFound
	}
	if err != nil {
		return clip.LinkedPost{}, err
	}
	title := p.Title
	if strings.TrimSpace(title) == "" && p.Content != nil {
		title = p.Content.Title
	}
	return clip.LinkedPost{Slug: p.Slug, Title: title, CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt, ContentReady: p.Content != nil && len(p.Content.Blocks) > 0}, nil
}
