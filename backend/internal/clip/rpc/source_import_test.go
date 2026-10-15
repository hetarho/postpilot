package rpc

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/clip"
	clipapp "github.com/postpilot/backend/internal/clip/app"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
)

type importRPCStore struct {
	rpcSourceStore
	checked, completed int
}

func (s *importRPCStore) ImportSourceBatch(ctx context.Context, user, project, batch string, _ time.Time) (clip.SourceBatch, error) {
	s.checked++
	b, e := s.GetSourceBatch(ctx, user, batch)
	if e == nil && b.ProjectID != project {
		return b, clip.ErrNotFound
	}
	return b, e
}
func (s *importRPCStore) CompleteSourceImport(ctx context.Context, user, project, batch, destination string, _ time.Time) (*clip.SourceBatch, error) {
	b, e := s.GetSourceBatch(ctx, user, batch)
	if e != nil {
		return nil, e
	}
	if project != b.ProjectID || destination != clip.CompletedSourceImportID(user, project, batch) {
		return nil, clip.ErrInvalid
	}
	s.completed++
	return &b, nil
}
func TestImportRPCUsesActorAndMapsConfirmationOption(t *testing.T) {
	store := &importRPCStore{rpcSourceStore: rpcSourceStore{batch: clip.SourceBatch{ID: "batch", UserID: "alice", ProjectID: "owned", State: "uploading", UploadExpiresAt: time.Now().Add(time.Hour), Sources: []clip.SourceLease{{ID: "source", Key: "private", State: "pending", SourceMetadata: clip.SourceMetadata{Bytes: 123, ContentType: "video/mp4"}}}}}}
	h := NewHandler(nil).WithSources(clipapp.NewSourceService(store, rpcSourceObjects{}, clip.DefaultSourceLimits(clip.Environment{SourceBatchTTL: time.Hour, PutTTL: time.Minute})))
	if _, e := h.CompleteClipSourceImport(context.Background(), connect.NewRequest(&v1.CompleteClipSourceImportRequest{ProjectId: "owned", BatchId: "batch"})); connect.CodeOf(e) != connect.CodeUnauthenticated {
		t.Fatal(e)
	}
	if _, e := h.CompleteClipSourceImport(auth.WithUser(t.Context(), "bob"), connect.NewRequest(&v1.CompleteClipSourceImportRequest{ProjectId: "owned", BatchId: "batch"})); connect.CodeOf(e) != connect.CodeNotFound {
		t.Fatal(e)
	}
	ctx := auth.WithUser(t.Context(), "alice")
	confirmed, e := h.ConfirmClipSource(ctx, connect.NewRequest(&v1.ConfirmClipSourceRequest{BatchId: "batch", SourceId: "source", PreserveConfirmedImport: true}))
	if e != nil || confirmed.Msg.Batch == nil || store.checked != 1 {
		t.Fatal(confirmed, e, store.checked)
	}
	done, e := h.CompleteClipSourceImport(ctx, connect.NewRequest(&v1.CompleteClipSourceImportRequest{ProjectId: "owned", BatchId: "batch"}))
	if e != nil || done.Msg.Empty || done.Msg.Batch == nil || store.completed != 1 || done.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal(done, e)
	}
}
