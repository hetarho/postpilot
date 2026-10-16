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

func (a clipLinkedPosts) Text(ctx context.Context, user, slug string) (clip.LinkedPostDocument, error) {
	p, err := a.service.Get(ctx, user, slug)
	if errors.Is(err, post.ErrNotFound) || errors.Is(err, post.ErrForbidden) {
		return clip.LinkedPostDocument{}, clip.ErrNotFound
	}
	if err != nil {
		return clip.LinkedPostDocument{}, err
	}
	return clip.LinkedPostDocument{CreatedAt: p.CreatedAt, Text: linkedPostCanonicalText(p.Slug, p.Content)}, nil
}

func linkedPostCanonicalText(slug string, content *post.PostContent) *clip.LinkedPostText {
	if content == nil || len(content.Blocks) == 0 {
		return nil
	}
	text := &clip.LinkedPostText{Slug: slug, Title: content.Title, Sections: []clip.LinkedPostSection{}}
	var current *clip.LinkedPostSection
	for _, block := range content.Blocks {
		if block.Type == post.BlockHeading {
			text.Sections = append(text.Sections, clip.LinkedPostSection{Heading: block.Content})
			current = &text.Sections[len(text.Sections)-1]
			continue
		}
		var value string
		switch block.Type {
		case post.BlockText, post.BlockQuote:
			value = block.Content
		case post.BlockList:
			value = strings.Join(block.Items, "\n")
		default:
			continue
		}
		if value == "" {
			continue
		}
		if current == nil {
			text.Sections = append(text.Sections, clip.LinkedPostSection{})
			current = &text.Sections[len(text.Sections)-1]
		}
		if current.Text != "" {
			current.Text += "\n"
		}
		current.Text += value
	}
	return text
}
