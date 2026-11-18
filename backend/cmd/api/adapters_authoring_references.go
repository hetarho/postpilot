package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/postpilot/backend/internal/authoring"
	"github.com/postpilot/backend/internal/authoring/webpage"
	"github.com/postpilot/backend/internal/post"
	"github.com/postpilot/backend/internal/template"
)

// authoringReferences reads a template request's form references for their owner: a post
// through the post context and a template through the template context, both owner-scoped,
// and a web page through the guarded fetcher. Each becomes its form alone, flattened HERE so
// neither context learns that an authoring request exists: a post's headings, photo and
// text positions — never its prose, photo files, captions, memo or tags (TMPL-64) — and a
// template's own grammar, which is form already (TMPL-57).
type authoringReferences struct {
	posts       *post.Service
	templates   *template.Service
	pages       *webpage.Fetcher
	photoRowMax int
}

func (a authoringReferences) Read(ctx context.Context, owner string, ref authoring.Reference) (authoring.ReferenceDocument, error) {
	switch ref.Kind() {
	case "post":
		found, err := a.posts.Get(ctx, owner, ref.PostSlug)
		if errors.Is(err, post.ErrNotFound) {
			return authoring.ReferenceDocument{}, &authoring.ReferenceRefusal{Code: authoring.ReferenceRefusalMissing}
		}
		if err != nil {
			return authoring.ReferenceDocument{}, err
		}
		structure := postForm(found.Content, a.photoRowMax)
		if structure == "" {
			return authoring.ReferenceDocument{}, &authoring.ReferenceRefusal{Code: authoring.ReferenceRefusalEmpty}
		}
		title := found.Title
		if found.Content != nil && strings.TrimSpace(found.Content.Title) != "" {
			title = found.Content.Title
		}
		return authoring.ReferenceDocument{Kind: "post", Title: strings.TrimSpace(title), Structure: structure}, nil
	case "template":
		held, err := a.templates.List(ctx, owner)
		if err != nil {
			return authoring.ReferenceDocument{}, err
		}
		for _, found := range held {
			if found.ID != ref.TemplateID {
				continue
			}
			structure := strings.TrimSpace(found.Body)
			if area := strings.TrimSpace(found.TitleArea); area != "" {
				structure = "[제목 영역] " + area + "\n" + structure
			}
			if structure == "" {
				return authoring.ReferenceDocument{}, &authoring.ReferenceRefusal{Code: authoring.ReferenceRefusalEmpty}
			}
			return authoring.ReferenceDocument{Kind: "template", Title: found.Name, Structure: structure}, nil
		}
		return authoring.ReferenceDocument{}, &authoring.ReferenceRefusal{Code: authoring.ReferenceRefusalMissing}
	case "url":
		page, err := a.pages.Fetch(ctx, ref.URL)
		var refused *webpage.Refusal
		if errors.As(err, &refused) {
			return authoring.ReferenceDocument{}, &authoring.ReferenceRefusal{Code: refused.Code}
		}
		if err != nil {
			return authoring.ReferenceDocument{}, err
		}
		return authoring.ReferenceDocument{Kind: "url", Title: page.Title, Structure: page.Structure}, nil
	}
	return authoring.ReferenceDocument{}, &authoring.ReferenceRefusal{Code: authoring.ReferenceRefusalInvalid}
}

// postForm is a post's shape without its words: headings as written, and every other block
// as its position — `[본문]`, `[인용]`, `[목록 n개]`, `[사진]`, `[사진 n장 묶음]` — so the
// order, the sections and the photo places survive while the prose stays with the post.
func postForm(content *post.PostContent, photoRowMax int) string {
	if content == nil {
		return ""
	}
	lines := make([]string, 0, len(content.Blocks))
	add := func(line string) {
		// Consecutive text blocks are one stretch of text in a template's form.
		if line == "[본문]" && len(lines) > 0 && lines[len(lines)-1] == line {
			return
		}
		lines = append(lines, line)
	}
	for _, block := range content.Blocks {
		switch block.Type {
		case post.BlockHeading:
			if text := strings.TrimSpace(block.Content); text != "" {
				marker := "## "
				if block.Level >= 3 {
					marker = "### "
				}
				add(marker + text)
			}
		case post.BlockText:
			if strings.TrimSpace(block.Content) != "" {
				add("[본문]")
			}
		case post.BlockQuote:
			if strings.TrimSpace(block.Content) != "" {
				add("[인용]")
			}
		case post.BlockList:
			if len(block.Items) > 0 {
				add(fmt.Sprintf("[목록 %d개]", len(block.Items)))
			}
		case post.BlockImage:
			add("[사진]")
		case post.BlockGallery:
			add(fmt.Sprintf("[사진 %d장 묶음]", min(len(block.Files), photoRowMax)))
		}
	}
	return strings.Join(lines, "\n")
}

var _ authoring.References = authoringReferences{}
