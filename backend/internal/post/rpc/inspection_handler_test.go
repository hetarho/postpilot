package rpc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/auth"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/post"
	poststore "github.com/postpilot/backend/internal/post/store"
)

type inspectionStoreOriginReader struct{ store *poststore.Store }

func (r inspectionStoreOriginReader) Get(ctx context.Context, userID, slug string) (post.Post, error) {
	value, err := r.store.GetPost(ctx, slug)
	if err != nil {
		return post.Post{}, err
	}
	if value.UserID != userID {
		return post.Post{}, post.ErrForbidden
	}
	return value, nil
}

func TestInspectionRPCReadsEveryRealCapturedCallWithoutMutation(t *testing.T) {
	ctx := context.Background()
	handle, err := db.Open(filepath.Join(t.TempDir(), "inspection.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { handle.Close() })
	if err := db.Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	stamp := time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)
	if _, err := handle.Writer.ExecContext(ctx, "INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash',?)", stamp.Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	store := poststore.New(handle.Writer, handle.Reader)
	if err := store.CreatePost(ctx, post.Post{Slug: "p", UserID: "alice", Memo: "owner material", TargetLanguage: post.LanguageEnglish, TagCount: 3, CreatedAt: stamp, UpdatedAt: stamp}); err != nil {
		t.Fatal(err)
	}
	found, err := store.GetPost(ctx, "p")
	if err != nil {
		t.Fatal(err)
	}
	run := post.RequestCaptureRun{JobID: "admitted-job", UserID: "alice", PostSlug: "p", SourceFingerprint: post.RequestCaptureSourceFingerprint(found), PlanFingerprint: post.StorylineFingerprint(found.Storyline)}
	value := requestInspectionFixture()
	value.Stage = "post-observation"
	for i, id := range []string{"observe:1", "observe:2"} {
		if err := store.WritePostRequestCapture(ctx, run, post.RequestCaptureCall{ID: id, Sequence: i}, value); err != nil {
			t.Fatal(err)
		}
	}
	handler := NewInspectionHandler(store, inspectionStoreOriginReader{store}, store)
	path, rpc := postpilotv1connect.NewWritingInspectionServiceHandler(handler)
	mux := http.NewServeMux()
	mux.Handle(path, rpc)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		mux.ServeHTTP(w, req.WithContext(auth.WithUser(req.Context(), "alice")))
	}))
	t.Cleanup(server.Close)
	client := postpilotv1connect.NewWritingInspectionServiceClient(server.Client(), server.URL)
	before, err := store.GetPost(ctx, "p")
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		response, err := client.GetPostRequestInspection(ctx, connect.NewRequest(&postpilotv1.GetPostRequestInspectionRequest{PostSlug: "p", Stage: "observe", Status: postpilotv1.InspectionStatus_INSPECTION_STATUS_CAPTURED}))
		if err != nil {
			t.Fatal(err)
		}
		if len(response.Msg.Inspections) != 2 || response.Msg.Inspections[0].CallId != "observe:1" || response.Msg.Inspection.CallId != "observe:2" || response.Msg.Inspections[0].Fragments[0].Text != value.Fragments[0].Text {
			t.Fatalf("ordered issued call projections changed: %+v", response.Msg)
		}
	}
	after, err := store.GetPost(ctx, "p")
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("RPC inspection mutated post")
	}
	var jobs int
	if err := handle.Reader.QueryRowContext(ctx, "SELECT count(*) FROM generation_jobs").Scan(&jobs); err != nil || jobs != 0 {
		t.Fatal("RPC inspection created job")
	}
	for _, check := range []struct {
		user, slug, stage string
		code              connect.Code
	}{{"bob", "p", "unknown", connect.CodePermissionDenied}, {"alice", "missing", "observe", connect.CodeNotFound}, {"alice", "p", "unknown", connect.CodeInvalidArgument}} {
		_, err := handler.GetPostRequestInspection(auth.WithUser(ctx, check.user), connect.NewRequest(&postpilotv1.GetPostRequestInspectionRequest{PostSlug: check.slug, Stage: check.stage, Status: postpilotv1.InspectionStatus_INSPECTION_STATUS_CAPTURED}))
		if connect.CodeOf(err) != check.code {
			t.Fatalf("%+v: %v", check, err)
		}
	}
	if err := store.PurgePostRequestCaptures(ctx, "alice", "p"); err != nil {
		t.Fatal(err)
	}
	response, err := client.GetPostRequestInspection(ctx, connect.NewRequest(&postpilotv1.GetPostRequestInspectionRequest{PostSlug: "p", Stage: "observe", Status: postpilotv1.InspectionStatus_INSPECTION_STATUS_CAPTURED}))
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Msg.Inspections) != 0 || response.Msg.Inspection.Status != postpilotv1.InspectionStatus_INSPECTION_STATUS_UNAVAILABLE || len(response.Msg.Inspection.Fragments) != 0 {
		t.Fatal("purged RPC history rebuilt")
	}
	if _, err := handler.GetPostRequestInspection(ctx, connect.NewRequest(&postpilotv1.GetPostRequestInspectionRequest{})); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("anonymous inspection: %v", err)
	}
}
