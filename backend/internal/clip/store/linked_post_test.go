package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
)

type linkedPostsFixture map[string]clip.LinkedPost

func (p linkedPostsFixture) Get(_ context.Context, user, slug string) (clip.LinkedPost, error) {
	post, ok := p[user+":"+slug]
	if !ok {
		return clip.LinkedPost{}, clip.ErrNotFound
	}
	return post, nil
}

func TestLinkedPostIdentityRoundTripsAndClearsOnNextWrite(t *testing.T) {
	service, store, _ := setup(t)
	at := time.Date(2026, 10, 15, 1, 2, 3, 456, time.UTC)
	posts := linkedPostsFixture{"alice:story": {Slug: "story", Title: "original", CreatedAt: at, UpdatedAt: at, ContentReady: true}}
	service.WithLinkedPosts(posts)
	slug := "story"
	p, err := service.CreateProject(t.Context(), "alice", clip.ProjectInput{Title: "clip", Ratio: "vertical", Language: "ko", LinkedPostSlug: &slug})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := store.GetProject(t.Context(), "alice", p.ID)
	if err != nil || raw.LinkedPostRef == nil || !raw.LinkedPostRef.CreatedAt.Equal(at) {
		t.Fatal("identity did not round trip", raw.LinkedPostRef, err)
	}
	rows, err := service.ListProjectSummaries(t.Context(), "alice")
	if err != nil || rows[0].LinkedPost == nil || rows[0].LinkedPost.Title != "original" {
		t.Fatal("directory omitted the linked post", err)
	}
	unchanged := "settings edit"
	kept, err := service.UpdateProject(t.Context(), "alice", p.ID, clip.ProjectPatch{Title: &unchanged})
	if err != nil || kept.LinkedPost == nil || kept.LinkedPost.Slug != slug {
		t.Fatal("an absent link field changed the link", err)
	}
	// A new post reuses the slug; neither detail nor directory may attach to it.
	posts["alice:story"] = clip.LinkedPost{Slug: slug, Title: "replacement", CreatedAt: at.Add(time.Second), UpdatedAt: at.Add(time.Second), ContentReady: true}
	read, err := service.GetProject(t.Context(), "alice", p.ID)
	if err != nil || read.LinkedPost != nil {
		t.Fatal("slug reuse attached a different post", err)
	}
	title := "edited"
	if _, err = service.UpdateProject(t.Context(), "alice", p.ID, clip.ProjectPatch{Title: &title}); err != nil {
		t.Fatal(err)
	}
	raw, err = store.GetProject(t.Context(), "alice", p.ID)
	if err != nil || raw.LinkedPostRef != nil || raw.Title != title {
		t.Fatal("stale columns survived the next write", err)
	}
	slug = ""
	if _, err = service.UpdateProject(t.Context(), "alice", p.ID, clip.ProjectPatch{LinkedPostSlug: &slug}); err != nil {
		t.Fatal(err)
	}
}

func TestOldStaleLinkCleanupCannotEraseAReplacement(t *testing.T) {
	service, store, _ := setup(t)
	at := time.Now().UTC()
	posts := linkedPostsFixture{
		"alice:first":  {Slug: "first", CreatedAt: at, ContentReady: true},
		"alice:second": {Slug: "second", CreatedAt: at.Add(time.Second), ContentReady: true},
	}
	service.WithLinkedPosts(posts)
	slug := "first"
	p, err := service.CreateProject(t.Context(), "alice", clip.ProjectInput{Title: "clip", Ratio: "vertical", Language: "ko", LinkedPostSlug: &slug})
	if err != nil {
		t.Fatal(err)
	}
	old := p.LinkedPostRef
	slug = "second"
	if _, err = service.UpdateProject(t.Context(), "alice", p.ID, clip.ProjectPatch{LinkedPostSlug: &slug}); err != nil {
		t.Fatal(err)
	}
	title := "concurrent edit"
	got, err := store.UpdateProject(t.Context(), "alice", p.ID, clip.ProjectPatch{Title: &title, StaleLinkedPostIdentity: old}, time.Now())
	if err != nil || got.LinkedPostRef == nil || got.LinkedPostRef.Slug != "second" {
		t.Fatal("an older stale read cleared the new link", err)
	}
}

func TestLinkWriteRefusesAJobStartedBeforeItsTransaction(t *testing.T) {
	h := generationSetup(t)
	h.start(t)
	_, err := h.store.UpdateProject(t.Context(), "alice", h.project.ID, clip.ProjectPatch{
		LinkedPostIdentity: &clip.LinkedPostIdentity{Slug: "story", CreatedAt: time.Now()},
	}, time.Now())
	if !errors.Is(err, clip.ErrBusy) {
		t.Fatalf("link write crossed an active job: %v", err)
	}
}
