package app

import (
	"context"
	"errors"

	"github.com/postpilot/backend/internal/clip"
)

func (s *Service) WithLinkedPosts(posts clip.LinkedPosts) *Service {
	s.linkedPosts = posts
	return s
}

func (s *Service) linkedPostBySlug(ctx context.Context, user, slug string) (*clip.LinkedPost, error) {
	if s.linkedPosts == nil {
		return nil, errors.New("linked post reader is unavailable")
	}
	p, err := s.linkedPosts.Get(ctx, user, slug)
	if err != nil {
		return nil, err
	}
	if p.Slug != slug || p.CreatedAt.IsZero() {
		return nil, clip.ErrNotFound
	}
	return &p, nil
}

func (s *Service) linkedPostForIdentity(ctx context.Context, user string, identity *clip.LinkedPostIdentity) (*clip.LinkedPost, error) {
	if identity == nil {
		return nil, nil
	}
	p, err := s.linkedPostBySlug(ctx, user, identity.Slug)
	if errors.Is(err, clip.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !p.CreatedAt.Equal(identity.CreatedAt) {
		return nil, nil
	}
	return p, nil
}

func (s *Service) resolveLinkedPost(ctx context.Context, user string, p clip.Project) (clip.Project, error) {
	var err error
	p.LinkedPost, err = s.linkedPostForIdentity(ctx, user, p.LinkedPostRef)
	return p, err
}

func (s *Service) resolveLinkedPosts(ctx context.Context, user string, projects []clip.Project, err error) ([]clip.Project, error) {
	if err != nil {
		return nil, err
	}
	for i := range projects {
		if projects[i], err = s.resolveLinkedPost(ctx, user, projects[i]); err != nil {
			return nil, err
		}
	}
	return projects, nil
}

func (s *Service) chooseLinkedPost(ctx context.Context, user, slug string) (*clip.LinkedPostIdentity, *clip.LinkedPost, error) {
	if slug == "" {
		return &clip.LinkedPostIdentity{}, nil, nil
	}
	p, err := s.linkedPostBySlug(ctx, user, slug)
	if err != nil {
		return nil, nil, err
	}
	if !p.ContentReady {
		return nil, nil, clip.ErrInvalid
	}
	return &clip.LinkedPostIdentity{Slug: p.Slug, CreatedAt: p.CreatedAt}, p, nil
}

func (s *Service) prepareLinkedPostPatch(ctx context.Context, user string, old clip.Project, p *clip.ProjectPatch) error {
	if p.LinkedPostSlug != nil {
		identity, _, err := s.chooseLinkedPost(ctx, user, *p.LinkedPostSlug)
		p.LinkedPostIdentity = identity
		return err
	}
	if old.LinkedPostRef != nil {
		post, err := s.linkedPostForIdentity(ctx, user, old.LinkedPostRef)
		if err != nil {
			return err
		}
		if post == nil {
			p.StaleLinkedPostIdentity = old.LinkedPostRef
		}
	}
	return nil
}
