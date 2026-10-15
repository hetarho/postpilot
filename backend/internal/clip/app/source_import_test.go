package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
)

type importScopeStore struct {
	clip.SourceStore
	batch     clip.SourceBatch
	refuse    bool
	discarded int
	checks    int
}

func (s *importScopeStore) GetSourceBatch(_ context.Context, user, id string) (clip.SourceBatch, error) {
	if user != s.batch.UserID || id != s.batch.ID {
		return clip.SourceBatch{}, clip.ErrNotFound
	}
	return s.batch, nil
}
func (s *importScopeStore) ImportSourceBatch(_ context.Context, user, project, batch string, _ time.Time) (clip.SourceBatch, error) {
	s.checks++
	if s.refuse {
		return clip.SourceBatch{}, clip.ErrBusy
	}
	return s.batch, nil
}
func (s *importScopeStore) CompleteSourceImport(context.Context, string, string, string, string, time.Time) (*clip.SourceBatch, error) {
	return &s.batch, nil
}
func (s *importScopeStore) MarkSourceCleanup(context.Context, string, string, bool) (clip.SourceBatch, error) {
	s.discarded++
	return s.batch, nil
}

type importObjectProbe struct {
	clip.ObjectStore
	heads int
}

func (s *importObjectProbe) HeadSource(context.Context, string) (clip.SourceObjectInfo, error) {
	s.heads++
	return clip.SourceObjectInfo{Bytes: 1, ContentType: "video/mp4"}, nil
}
func TestImportConfirmationQualifiesScopeBeforeObjectWorkAndNeverDiscardsSiblings(t *testing.T) {
	for _, refuse := range []bool{false, true} {
		store := &importScopeStore{refuse: refuse, batch: clip.SourceBatch{ID: "batch", UserID: "alice", ProjectID: "project", State: "uploading", UploadExpiresAt: time.Now().Add(time.Hour), Sources: []clip.SourceLease{{ID: "pending", Key: "private", State: "pending", SourceMetadata: clip.SourceMetadata{Bytes: 100, ContentType: "video/mp4"}}}}}
		objects := &importObjectProbe{}
		service := NewSourceService(store, objects, clip.DefaultSourceLimits(clip.Environment{SourceBatchTTL: time.Hour, PutTTL: time.Minute}))
		_, e := service.ConfirmImport(t.Context(), "alice", "batch", "pending")
		if refuse {
			if !errors.Is(e, clip.ErrBusy) || objects.heads != 0 {
				t.Fatal("scope refusal reached storage", e, objects.heads)
			}
		} else if !errors.Is(e, clip.ErrInvalid) || objects.heads != 1 {
			t.Fatal(e, objects.heads)
		}
		if store.discarded != 0 || store.checks != 1 {
			t.Fatal("import invalidated its siblings", store.discarded, store.checks)
		}
	}
}
func TestImportConfirmationRejectsMissingCapabilityBeforeReadingState(t *testing.T) {
	service := NewSourceService(struct{ clip.SourceStore }{}, &importObjectProbe{}, clip.DefaultSourceLimits(clip.Environment{SourceBatchTTL: time.Hour, PutTTL: time.Minute}))
	if _, e := service.ConfirmImport(t.Context(), "alice", "batch", "pending"); !errors.Is(e, clip.ErrSourceState) {
		t.Fatal(e)
	}
	if _, e := service.CompleteImport(t.Context(), "alice", "project", "batch"); !errors.Is(e, clip.ErrSourceState) {
		t.Fatal(e)
	}
}
