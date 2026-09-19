package app

import (
	"context"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/platform/config"
)

// clockStore records the template the service inserts; nothing else is read.
type clockStore struct {
	clip.Store
	clip.SourceStore
	inserted clip.VideoTemplate
}

func (s *clockStore) InsertTemplate(_ context.Context, t clip.VideoTemplate) error {
	s.inserted = t
	return nil
}

type clockObjects struct{ clip.ObjectStore }
type clockFinalizer struct{ clip.ProjectFinalizer }

// TestServiceStampsTemplatesFromItsClock pins the injected clock: every timestamp the
// project service stores comes from `now`, so a test can hold time still.
func TestServiceStampsTemplatesFromItsClock(t *testing.T) {
	store := &clockStore{}
	s := NewService(store, config.ClipLimits(), NewSourceService(store, clockObjects{}, config.ClipSourceLimits(6*time.Hour, 10*time.Minute)), clockFinalizer{})
	frozen := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return frozen }
	created, err := s.CreateTemplate(context.Background(), "alice", clip.Recipe{Name: "여행", Preset: "stay"})
	if err != nil {
		t.Fatal(err)
	}
	if !created.CreatedAt.Equal(frozen) || !created.UpdatedAt.Equal(frozen) || !store.inserted.CreatedAt.Equal(frozen) {
		t.Fatalf("template stamped %v/%v, stored %v; want the service clock %v", created.CreatedAt, created.UpdatedAt, store.inserted.CreatedAt, frozen)
	}
}
