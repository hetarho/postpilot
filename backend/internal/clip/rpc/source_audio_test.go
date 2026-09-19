package rpc

import (
	"context"
	"strings"
	"testing"
	"time"

	clipapp "github.com/postpilot/backend/internal/clip/app"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/clip"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/platform/config"
)

// audioStore records what actually reached the domain, so the test can assert
// that the actor comes from the session and never from the payload.
type audioStore struct {
	clip.SourceStore
	batch  clip.SourceBatch
	user   string
	change clip.SourceAudioChange
	err    error
}

func (s *audioStore) SetSourceOriginalAudio(_ context.Context, user string, c clip.SourceAudioChange) (clip.SourceBatch, clip.Project, error) {
	s.user, s.change = user, c
	if s.err != nil {
		return clip.SourceBatch{}, clip.Project{}, s.err
	}
	out := s.batch
	out.Sources = []clip.SourceLease{s.batch.Sources[0]}
	out.Sources[0].RetainOriginalAudio = c.RetainOriginal
	return out, clip.Project{ID: c.ProjectID, EditPlanRevision: c.ExpectedRevision + 1, RenderedPlanRevision: c.ExpectedRevision}, nil
}

func TestSourceSoundRPCIsOwnerScopedAndReturnsBothProjections(t *testing.T) {
	expires := time.Now().Add(time.Hour)
	fingerprint := strings.Repeat("a", 64)
	source := clip.SourceLease{ID: "source", Key: "clip-inputs/alice/private-original", State: "ready", ExpiresAt: expires, ActualBytes: 123,
		SourceMetadata: clip.SourceMetadata{Filename: "source.mp4", ContentType: "video/mp4", Bytes: 123, Fingerprint: fingerprint}}
	store := &audioStore{batch: clip.SourceBatch{ID: "batch", UserID: "alice", ProjectID: "owned", State: "ready", Current: true, ExpiresAt: expires, Sources: []clip.SourceLease{source}}}
	h := NewHandler(nil).WithSources(clipapp.NewSourceService(store, rpcSourceObjects{}, config.ClipSourceLimits(6*time.Hour, 10*time.Minute)))
	ctx := auth.WithUser(context.Background(), "alice")
	request := &v1.SetClipSourceOriginalSoundRequest{ProjectId: "owned", BatchId: "batch", SourceId: "source", ExpectedFingerprint: fingerprint, RetainOriginalAudio: true, ExpectedRevision: 3}
	out, err := h.SetClipSourceOriginalSound(ctx, connect.NewRequest(request))
	if err != nil {
		t.Fatal(err)
	}
	if store.user != "alice" || store.change.ExpectedRevision != 3 || !store.change.RetainOriginal || store.change.Fingerprint != fingerprint {
		t.Fatalf("%s %+v", store.user, store.change)
	}
	if !out.Msg.Batch.Sources[0].RetainOriginalAudio || out.Msg.Project.EditPlanRevision != 4 || out.Msg.Project.RenderedPlanRevision != 3 {
		t.Fatal(out.Msg)
	}
	// The response never carries the object key or the owner id.
	if strings.Contains(out.Msg.String(), "clip-inputs") || strings.Contains(out.Msg.String(), "alice") {
		t.Fatal("private data in the response", out.Msg.String())
	}
	if out.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal(out.Header())
	}
	// No session, no action — and the request carries no owner field to forge.
	if _, err := h.SetClipSourceOriginalSound(context.Background(), connect.NewRequest(request)); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatal(err)
	}
	fields := (&v1.SetClipSourceOriginalSoundRequest{}).ProtoReflect().Descriptor().Fields()
	for i := range fields.Len() {
		if fields.Get(i).Name() == "user_id" {
			t.Fatal("the owner became a payload field")
		}
	}
	// Every typed refusal keeps its own code through the existing boundary.
	for _, c := range []struct {
		err  error
		code connect.Code
	}{
		{clip.ErrPlanConflict, connect.CodeAborted},
		{clip.ErrNotFound, connect.CodeNotFound},
		{clip.ErrBusy, connect.CodeFailedPrecondition},
		{clip.ErrFinalized, connect.CodeFailedPrecondition},
		{clip.ErrSourceState, connect.CodeFailedPrecondition},
	} {
		store.err = c.err
		if _, err := h.SetClipSourceOriginalSound(ctx, connect.NewRequest(request)); connect.CodeOf(err) != c.code {
			t.Fatalf("%v became %v", c.err, connect.CodeOf(err))
		}
	}
}
