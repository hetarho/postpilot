package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/postpilot/backend/internal/naversearch"
	"github.com/postpilot/backend/internal/platform/config"
	"github.com/postpilot/backend/internal/post"
	"github.com/postpilot/backend/internal/quality"
)

// qualityPosts hands the quality context the post context as it reads it: one owned post's
// content and languages, and the account's published window. Post never learns quality exists,
// and quality never imports post (ARCH-7).
type qualityPosts struct{ service *post.Service }

func (a qualityPosts) Post(ctx context.Context, userID, slug string) (quality.PostSnapshot, error) {
	found, err := a.service.Get(ctx, userID, slug)
	switch {
	case errors.Is(err, post.ErrNotFound), errors.Is(err, post.ErrForbidden):
		return quality.PostSnapshot{}, quality.ErrPostNotFound
	case err != nil:
		return quality.PostSnapshot{}, err
	}
	target, ok := qualityLanguage(found.TargetLanguage)
	if !ok {
		return quality.PostSnapshot{}, fmt.Errorf("post %s has an unknown target language %q", slug, found.TargetLanguage)
	}
	snapshot := quality.PostSnapshot{
		Slug: found.Slug, Revision: found.ContentRevision, TargetLanguage: target, Nouns: found.ContentNouns,
	}
	if snapshot.ContentLanguage, err = qualityContentLanguage(found.ContentLanguage); err != nil {
		return quality.PostSnapshot{}, err
	}
	if found.Content != nil {
		doc, err := qualityDocument(*found.Content)
		if err != nil {
			return quality.PostSnapshot{}, err
		}
		snapshot.Content = &doc
	}
	return snapshot, nil
}

func (a qualityPosts) Published(ctx context.Context, userID string, limit int) ([]quality.PublishedPost, error) {
	rows, err := a.service.PublishedPosts(ctx, userID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]quality.PublishedPost, 0, len(rows))
	for _, row := range rows {
		doc, err := qualityDocument(row.Content)
		if err != nil {
			return nil, err
		}
		language, err := qualityContentLanguage(row.ContentLanguage)
		if err != nil {
			return nil, err
		}
		out = append(out, quality.PublishedPost{
			Slug: row.Slug, Revision: row.ContentRevision, Content: doc, ContentLanguage: language,
			Nouns: row.Nouns, PublishedAt: row.PublishedAt,
		})
	}
	return out, nil
}

// qualityDocument copies what a metric reads. A block type this does not know is an error rather
// than a skip: a silently dropped block would change every count it took part in.
func qualityDocument(content post.PostContent) (quality.Document, error) {
	doc := quality.Document{Title: content.Title, Blocks: make([]quality.Block, 0, len(content.Blocks))}
	for _, block := range content.Blocks {
		kind, ok := qualityBlockType(block.Type)
		if !ok {
			return quality.Document{}, fmt.Errorf("unknown block type %q", block.Type)
		}
		doc.Blocks = append(doc.Blocks, quality.Block{
			Type: kind, Content: block.Content, File: block.File, Items: append([]string(nil), block.Items...),
		})
	}
	return doc, nil
}

func qualityBlockType(kind post.BlockType) (quality.BlockType, bool) {
	switch kind {
	case post.BlockText:
		return quality.BlockText, true
	case post.BlockHeading:
		return quality.BlockHeading, true
	case post.BlockImage:
		return quality.BlockImage, true
	case post.BlockVideo:
		return quality.BlockVideo, true
	case post.BlockQuote:
		return quality.BlockQuote, true
	case post.BlockList:
		return quality.BlockList, true
	}
	return "", false
}

func qualityLanguage(language post.Language) (quality.Language, bool) {
	switch language {
	case post.LanguageKorean:
		return quality.LanguageKorean, true
	case post.LanguageEnglish:
		return quality.LanguageEnglish, true
	}
	return "", false
}

// qualityContentLanguage keeps nil nil: a post whose content predates the column measures as the
// quality context decides, not as this adapter guesses.
func qualityContentLanguage(language *post.Language) (*quality.Language, error) {
	if language == nil {
		return nil, nil
	}
	mapped, ok := qualityLanguage(*language)
	if !ok {
		return nil, fmt.Errorf("unknown content language %q", *language)
	}
	return &mapped, nil
}

// naverBlog is the search client as the adapter below uses it, so a test can stand one in.
type naverBlog interface {
	SearchBlog(ctx context.Context, query string, start int) ([]naversearch.Item, error)
}

// qualityBlogSearch hands the phrase batch the Naver blog search, item for item in its order.
type qualityBlogSearch struct{ client naverBlog }

func (a qualityBlogSearch) SearchBlog(ctx context.Context, query string, start int) ([]quality.SearchItem, error) {
	items, err := a.client.SearchBlog(ctx, query, start)
	if err != nil {
		return nil, err
	}
	out := make([]quality.SearchItem, len(items))
	for i, item := range items {
		out[i] = quality.SearchItem{Title: item.Title, Description: item.Description}
	}
	return out, nil
}

// phraseSearch is the batch's search, or nil without both Naver keys (QUAL-42). It returns the
// interface so that disabled is a true nil, never a typed nil that would read as enabled.
func phraseSearch(cfg *config.Config) quality.BlogSearch {
	if !cfg.NaverSearchEnabled {
		return nil
	}
	return qualityBlogSearch{client: naversearch.New(cfg.NaverSearchClientID, cfg.NaverSearchClientSecret, http.DefaultClient)}
}
