package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
)

type linkedPostReader struct {
	texts map[string]*clip.LinkedPostText
	posts map[string]clip.LinkedPost
	calls int
}

func (r *linkedPostReader) Get(_ context.Context, user, slug string) (clip.LinkedPost, error) {
	r.calls++
	p, ok := r.posts[user+":"+slug]
	if !ok {
		return clip.LinkedPost{}, clip.ErrNotFound
	}
	return p, nil
}

func (r *linkedPostReader) Text(ctx context.Context, user, slug string) (clip.LinkedPostDocument, error) {
	p, err := r.Get(ctx, user, slug)
	if err != nil {
		return clip.LinkedPostDocument{}, err
	}
	return clip.LinkedPostDocument{CreatedAt: p.CreatedAt, Text: r.texts[user+":"+slug]}, nil
}

type linkedProjectStore struct {
	clip.Store
	clip.SourceStore
	project clip.Project
	patch   clip.ProjectPatch
	writes  int
	inserts int
}

func (s *linkedProjectStore) GetProject(_ context.Context, user, id string) (clip.Project, error) {
	if user != s.project.UserID || id != s.project.ID {
		return clip.Project{}, clip.ErrNotFound
	}
	return s.project, nil
}
func (s *linkedProjectStore) ListProjectRequests(context.Context, string, string) ([]clip.ProjectRequest, error) {
	return nil, nil
}
func (s *linkedProjectStore) ListProjects(context.Context, string) ([]clip.Project, error) {
	return []clip.Project{s.project}, nil
}
func (s *linkedProjectStore) ListProjectSummaries(ctx context.Context, user string) ([]clip.Project, error) {
	return s.ListProjects(ctx, user)
}
func (s *linkedProjectStore) InsertProject(_ context.Context, p clip.Project) error {
	s.inserts++
	p.LinkedPost = nil
	s.project = p
	return nil
}
func (s *linkedProjectStore) UpdateProject(_ context.Context, _, _ string, p clip.ProjectPatch, _ time.Time) (clip.Project, error) {
	s.writes++
	s.patch = p
	if p.LinkedPostIdentity != nil {
		s.project.LinkedPostRef = p.LinkedPostIdentity
		if p.LinkedPostIdentity.Slug == "" {
			s.project.LinkedPostRef = nil
		}
	}
	if p.StaleLinkedPostIdentity != nil && s.project.LinkedPostRef != nil && *p.StaleLinkedPostIdentity == *s.project.LinkedPostRef {
		s.project.LinkedPostRef = nil
	}
	if p.Title != nil {
		s.project.Title = *p.Title
	}
	if p.Channel != nil {
		s.project.Channel = *p.Channel
	}
	return s.project, nil
}

func linkedPostSetup(t *testing.T) (*Service, *linkedProjectStore, *linkedPostReader) {
	t.Helper()
	store := &linkedProjectStore{project: clip.Project{ID: "clip", UserID: "alice", Title: "clip title", Language: "ko", Ratio: "vertical", Instruction: "owner memo", EditPlan: "retained plan", Result: &clip.Result{Key: "retained result"}}}
	reader := &linkedPostReader{posts: map[string]clip.LinkedPost{
		"alice:story": {Slug: "story", Title: "owned post", CreatedAt: time.Date(2026, 10, 15, 1, 0, 0, 0, time.UTC), UpdatedAt: time.Now(), ContentReady: true},
		"alice:empty": {Slug: "empty", CreatedAt: time.Now()},
		"bob:foreign": {Slug: "foreign", CreatedAt: time.Now(), ContentReady: true},
	}}
	s := NewService(store, clip.DefaultLimits(), NewSourceService(store, struct{ clip.ObjectStore }{}, clip.DefaultSourceLimits(clip.Environment{SourceBatchTTL: time.Hour, PutTTL: time.Minute})), struct{ clip.ProjectFinalizer }{}).WithLinkedPosts(reader)
	return s, store, reader
}

func TestLinkPostRequiresOwnedCanonicalPost(t *testing.T) {
	s, store, _ := linkedPostSetup(t)
	for slug, want := range map[string]error{"missing": clip.ErrNotFound, "foreign": clip.ErrNotFound, "empty": clip.ErrInvalid} {
		if _, err := s.UpdateProject(t.Context(), "alice", "clip", clip.ProjectPatch{LinkedPostSlug: &slug}); !errors.Is(err, want) {
			t.Fatalf("link %s: %v", slug, err)
		}
	}
	if store.writes != 0 {
		t.Fatal("invalid links changed the project")
	}
	slug := "story"
	linked, err := s.UpdateProject(t.Context(), "alice", "clip", clip.ProjectPatch{LinkedPostSlug: &slug})
	if err != nil || linked.LinkedPost == nil || linked.LinkedPost.Title != "owned post" {
		t.Fatal(linked.LinkedPost, err)
	}
	if linked.Title != "clip title" || linked.Instruction != "owner memo" || linked.EditPlan != "retained plan" || linked.Result.Key != "retained result" || store.patch.Title != nil || store.patch.Composition != nil || store.patch.Campaign != nil {
		t.Fatal("linking changed unrelated settings or content")
	}
}

type linkedBusyJobs struct{ clip.GenerationJobs }

func (linkedBusyJobs) Active(context.Context, string, string) (*clip.ClipJob, error) {
	return &clip.ClipJob{ID: "running"}, nil
}

func TestLinkPostRefusedWhileRunningOrFinalized(t *testing.T) {
	for _, state := range []string{"running", "finalized"} {
		t.Run(state, func(t *testing.T) {
			s, store, reader := linkedPostSetup(t)
			want := clip.ErrBusy
			if state == "running" {
				s.generation = &GenerationService{jobs: linkedBusyJobs{}}
			} else {
				store.project.Finalized = &clip.Finalization{}
				want = clip.ErrFinalized
			}
			slug := "story"
			if _, err := s.UpdateProject(t.Context(), "alice", "clip", clip.ProjectPatch{LinkedPostSlug: &slug}); !errors.Is(err, want) {
				t.Fatal(err)
			}
			if store.writes != 0 || reader.calls != 0 {
				t.Fatal("read-only project reached link lookup or write")
			}
		})
	}
}

func TestLinkedPostAbsentAfterDeleteOrSlugReuse(t *testing.T) {
	s, store, reader := linkedPostSetup(t)
	slug := "story"
	if _, err := s.UpdateProject(t.Context(), "alice", "clip", clip.ProjectPatch{LinkedPostSlug: &slug}); err != nil {
		t.Fatal(err)
	}
	old := reader.posts["alice:story"]
	delete(reader.posts, "alice:story")
	for _, reused := range []bool{false, true} {
		if reused {
			old.CreatedAt = old.CreatedAt.Add(time.Second)
			reader.posts["alice:story"] = old
		}
		p, err := s.GetProject(t.Context(), "alice", "clip")
		if err != nil || p.LinkedPost != nil {
			t.Fatalf("stale link resolved: %v %v", p.LinkedPost, err)
		}
		for _, list := range []func(context.Context, string) ([]clip.Project, error){s.ListProjects, s.ListProjectSummaries} {
			rows, err := list(t.Context(), "alice")
			if err != nil || rows[0].LinkedPost != nil {
				t.Fatal("directory retained stale link", err)
			}
		}
	}
	if store.writes != 1 || store.project.LinkedPostRef == nil {
		t.Fatal("a read changed storage")
	}
	title := "new title"
	if _, err := s.UpdateProject(t.Context(), "alice", "clip", clip.ProjectPatch{Title: &title}); err != nil || store.project.LinkedPostRef != nil {
		t.Fatal("next write did not clear stale identity", err)
	}
}

func TestUnlinkClearsLink(t *testing.T) {
	s, store, reader := linkedPostSetup(t)
	slug := "story"
	if _, err := s.UpdateProject(t.Context(), "alice", "clip", clip.ProjectPatch{LinkedPostSlug: &slug}); err != nil {
		t.Fatal(err)
	}
	before := reader.calls
	slug = ""
	p, err := s.UpdateProject(t.Context(), "alice", "clip", clip.ProjectPatch{LinkedPostSlug: &slug})
	if err != nil || p.LinkedPost != nil || store.project.LinkedPostRef != nil || reader.calls != before {
		t.Fatal("unlink did not clear without consulting the post", err)
	}
}

func TestClipCreationSeedsLinkedPostAtomically(t *testing.T) {
	s, store, _ := linkedPostSetup(t)
	slug := "missing"
	input := clip.ProjectInput{Title: "from post", Ratio: "vertical", Language: "ko", Instruction: "seed memo", LinkedPostSlug: &slug}
	if _, err := s.CreateProject(t.Context(), "alice", input); !errors.Is(err, clip.ErrNotFound) || store.inserts != 0 {
		t.Fatal("invalid source left a partial project", err)
	}
	slug = "story"
	p, err := s.CreateProject(t.Context(), "alice", input)
	if err != nil || store.inserts != 1 || store.writes != 0 || p.LinkedPost == nil || store.project.LinkedPostRef == nil || p.Instruction != "seed memo" {
		t.Fatal("seed and link did not share creation", err)
	}
}

func TestStaleLinkClearsOnFinalizedChannelMove(t *testing.T) {
	s, store, reader := linkedPostSetup(t)
	slug := "story"
	if _, err := s.UpdateProject(t.Context(), "alice", "clip", clip.ProjectPatch{LinkedPostSlug: &slug}); err != nil {
		t.Fatal(err)
	}
	delete(reader.posts, "alice:story")
	store.project.Finalized = &clip.Finalization{}
	channel := clip.ChannelInstagram
	if _, err := s.UpdateProject(t.Context(), "alice", "clip", clip.ProjectPatch{Channel: &channel}); err != nil || store.project.LinkedPostRef != nil {
		t.Fatal("stale maintenance blocked an allowed directory move", err)
	}
}
