package rpc

import (
	"connectrpc.com/connect"
	"context"
	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/clip"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/platform/config"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
	"strings"
	"testing"
	"time"
)

type rpcSourceStore struct {
	clip.SourceStore
	batch clip.SourceBatch
}

func (s *rpcSourceStore) ReplaceSourceBatch(_ context.Context, b clip.SourceBatch) ([]clip.SourceBatch, error) {
	if b.ProjectID != "owned" {
		return nil, clip.ErrNotFound
	}
	s.batch = b
	return nil, nil
}
func (s *rpcSourceStore) GetSourceBatch(_ context.Context, user, id string) (clip.SourceBatch, error) {
	if user != s.batch.UserID || id != s.batch.ID {
		return clip.SourceBatch{}, clip.ErrNotFound
	}
	return s.batch, nil
}
func (s *rpcSourceStore) ConfirmSourceLease(_ context.Context, user, batch, id string, actual int64, _ time.Time) (clip.SourceBatch, error) {
	if user != s.batch.UserID || batch != s.batch.ID || id != s.batch.Sources[0].ID {
		return clip.SourceBatch{}, clip.ErrNotFound
	}
	s.batch.State = "ready"
	s.batch.Sources[0].State = "ready"
	s.batch.Sources[0].ActualBytes = actual
	return s.batch, nil
}
func (s *rpcSourceStore) MarkSourceCleanup(_ context.Context, user, id string, _ bool) (clip.SourceBatch, error) {
	if user != s.batch.UserID || id != s.batch.ID {
		return clip.SourceBatch{}, clip.ErrNotFound
	}
	s.batch.State = "cleanup_pending"
	return s.batch, nil
}
func (s *rpcSourceStore) RemoveSourceBatch(context.Context, string, string, time.Time) error {
	s.batch = clip.SourceBatch{}
	return nil
}

type rpcSourceObjects struct{ clip.ObjectStore }

func (rpcSourceObjects) PresignSource(context.Context, string, string, time.Duration) (clip.SignedSourcePut, error) {
	return clip.SignedSourcePut{URL: "https://storage.example/signed", Headers: map[string]string{"Content-Type": "video/mp4", "If-None-Match": "*"}}, nil
}
func (rpcSourceObjects) HeadSource(context.Context, string) (clip.SourceObjectInfo, error) {
	return clip.SourceObjectInfo{Bytes: 123, ContentType: "video/mp4"}, nil
}
func (rpcSourceObjects) Delete(context.Context, string) error { return nil }
func (rpcSourceObjects) PresignSourcePlayback(context.Context, string, string, time.Duration) (string, error) {
	return "https://storage.example/private?temporary=secret", nil
}
func (s *rpcSourceStore) ProjectSourceBatches(_ context.Context, user, project string) ([]clip.SourceBatch, error) {
	if user != s.batch.UserID || project != s.batch.ProjectID {
		return nil, clip.ErrNotFound
	}
	return []clip.SourceBatch{s.batch}, nil
}
func TestSourceRPCUsesActorAndNeverSerializesObjectIdentity(t *testing.T) {
	store := &rpcSourceStore{}
	h := NewHandler(nil).WithSources(clip.NewSourceService(store, rpcSourceObjects{}, config.ClipSourceLimits(6*time.Hour, 10*time.Minute)))
	ctx := auth.WithUser(context.Background(), "alice")
	request := &v1.CreateClipSourceBatchRequest{ProjectId: "owned", Sources: []*v1.ClipSourceMetadata{{Filename: "original.mp4", ContentType: "video/mp4", Bytes: 123, DurationMs: 1000, Width: 1920, Height: 1080, Fingerprint: strings.Repeat("a", 64)}}}
	out, err := h.CreateClipSourceBatch(ctx, connect.NewRequest(request))
	if err != nil {
		t.Fatal(err)
	}
	if store.batch.UserID != "alice" || len(out.Msg.Uploads) != 1 || out.Msg.Uploads[0].Headers["Content-Type"] != "video/mp4" || out.Msg.Batch.Sources[0].Metadata.Bytes != 123 {
		t.Fatal(out.Msg)
	}
	for _, user := range []string{"bob", "alice"} {
		id := out.Msg.Batch.Id
		if user == "alice" {
			id = "unknown"
		}
		_, err := h.ConfirmClipSource(auth.WithUser(context.Background(), user), connect.NewRequest(&v1.ConfirmClipSourceRequest{BatchId: id, SourceId: out.Msg.Batch.Sources[0].Id}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatal(err)
		}
	}
	confirmed, err := h.ConfirmClipSource(ctx, connect.NewRequest(&v1.ConfirmClipSourceRequest{BatchId: out.Msg.Batch.Id, SourceId: out.Msg.Batch.Sources[0].Id}))
	if err != nil || confirmed.Msg.Batch.State != "ready" || confirmed.Msg.Batch.Sources[0].ActualBytes != 123 {
		t.Fatal(confirmed, err)
	}
	if _, err := h.DiscardClipSourceBatch(ctx, connect.NewRequest(&v1.DiscardClipSourceBatchRequest{BatchId: out.Msg.Batch.Id})); err != nil {
		t.Fatal(err)
	}
	if store.batch.State != "cleanup_pending" {
		t.Fatal("discard did not retain the PUT tombstone")
	}
	for _, message := range []protoreflect.MessageDescriptor{(&v1.ClipSourceMetadata{}).ProtoReflect().Descriptor(), (&v1.ClipSource{}).ProtoReflect().Descriptor(), (&v1.ClipSourceBatch{}).ProtoReflect().Descriptor(), (&v1.CreateClipSourceBatchRequest{}).ProtoReflect().Descriptor()} {
		fields := message.Fields()
		for i := 0; i < fields.Len(); i++ {
			f := fields.Get(i)
			if f.Kind() == protoreflect.BytesKind || strings.Contains(string(f.Name()), "key") || f.Name() == "user_id" {
				t.Fatalf("private data in contract: %s.%s", message.Name(), f.Name())
			}
		}
	}
}

func TestRetainedSourcesRPCSeparatesMetadataFromOwnedPlaybackCapabilities(t *testing.T) {
	expires := time.Now().Add(time.Hour)
	source := clip.SourceLease{ID: "source", Key: "clip-inputs/alice/private-original", State: "ready", ExpiresAt: expires, ActualBytes: 123, SourceMetadata: clip.SourceMetadata{Filename: "source.mp4", ContentType: "video/mp4", Bytes: 123, Fingerprint: strings.Repeat("a", 64)}}
	store := &rpcSourceStore{batch: clip.SourceBatch{ID: "batch", UserID: "alice", ProjectID: "owned", State: "ready", Current: true, ExpiresAt: expires, Sources: []clip.SourceLease{source}}}
	h := NewHandler(nil).WithSources(clip.NewSourceService(store, rpcSourceObjects{}, config.ClipSourceLimits(6*time.Hour, 10*time.Minute)))
	ctx := auth.WithUser(context.Background(), "alice")
	response, err := h.GetClipSources(ctx, connect.NewRequest(&v1.GetClipSourcesRequest{ProjectId: "owned"}))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := protojson.Marshal(response.Msg)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "clip-inputs") || strings.Contains(string(raw), "temporary") || strings.Contains(string(raw), "https:") {
		t.Fatal("playback capability or object identity leaked into metadata")
	}
	if response.Header().Get("Cache-Control") != "private, no-store" || response.Msg.Batches[0].Sources[0].Availability != "available" {
		t.Fatal("source availability response", response)
	}
	request := connect.NewRequest(&v1.GetClipSourcePlaybackRequest{ProjectId: "owned", SourceId: "source", ExpectedFingerprint: source.Fingerprint})
	link, err := h.GetClipSourcePlayback(ctx, request)
	if err != nil || link.Msg.Url == "" || link.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal(link, err)
	}
	if _, err = h.GetClipSourcePlayback(auth.WithUser(context.Background(), "bob"), request); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatal("foreign playback", err)
	}
	request.Msg.ExpectedFingerprint = "stale"
	if _, err = h.GetClipSourcePlayback(ctx, request); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatal("stale playback", err)
	}
	request.Msg.ExpectedFingerprint = source.Fingerprint
	store.batch.Sources[0].ExpiresAt = time.Now().Add(-time.Second)
	if _, err = h.GetClipSourcePlayback(ctx, request); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatal("expired playback", err)
	}
}
