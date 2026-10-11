package rpc

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/auth"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/post"
	poststore "github.com/postpilot/backend/internal/post/store"
)

// reorderHarness is rpcServiceWith's real service over SQLite, with the store kept so a test can
// seed photos no RPC confirms without a storage object.
func reorderHarness(t *testing.T) (*Handler, *poststore.Store) {
	t.Helper()
	handle, err := db.Open(filepath.Join(t.TempDir(), "post.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { handle.Close() })
	if err := db.Migrate(context.Background(), handle.Writer); err != nil {
		t.Fatal(err)
	}
	stamp := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
	for _, statement := range []string{
		`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash','` + stamp + `')`,
		`INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES('voice-alice','alice','기본',1,'` + stamp + `','` + stamp + `')`,
	} {
		if _, err := handle.Writer.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	store := poststore.New(handle.Writer, handle.Reader)
	svc := post.NewService(store, noBlobs{}, post.Limits{}, post.Deps{
		Jobs: noJobs{}, Voices: aliceVoice{}, Experiments: noExperiments{}, ContentPurger: noPurge{},
		CandidateLinks: noDetach{}, MemoryLinks: noDetach{}, Fields: knownFields{},
	})
	return NewHandler(svc), store
}

func photoIDs(t *testing.T, h *Handler, ctx context.Context, slug string) ([]string, int64) {
	t.Helper()
	got, err := h.GetPost(ctx, connect.NewRequest(&postpilotv1.GetPostRequest{Slug: slug}))
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, image := range got.Msg.GetPost().GetImages() {
		ids = append(ids, image.GetId())
	}
	return ids, got.Msg.GetPost().GetInputRevision()
}

// POST-116 over the wire: the session's own account reorders its post's photos and GetPost
// answers the new order with the input revision advanced; no session, another account, an
// unknown post and a stale list are each refused with their own reason, and the request has
// nowhere to claim an account.
func TestReorderImagesOverTheWire(t *testing.T) {
	h, store := reorderHarness(t)
	alice := auth.WithUser(context.Background(), "alice")
	saved, err := h.SavePostDraft(alice, draftRequest("", "제주", nil))
	if err != nil {
		t.Fatal(err)
	}
	slug := saved.Msg.GetPost().GetSlug()
	at := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	for _, id := range []string{"a", "b", "c"} {
		if err := store.CreateImage(context.Background(), post.Image{ID: id, PostSlug: slug, Filename: id + ".jpg", Key: post.ObjectKey(slug, id), Width: 1, Height: 1, Bytes: 1, CreatedAt: at}); err != nil {
			t.Fatal(err)
		}
	}
	order, revision := photoIDs(t, h, alice, slug)
	if !reflect.DeepEqual(order, []string{"a", "b", "c"}) {
		t.Fatalf("seeded order = %v", order)
	}
	request := func(ids ...string) *connect.Request[postpilotv1.ReorderImagesRequest] {
		return connect.NewRequest(&postpilotv1.ReorderImagesRequest{PostSlug: slug, ImageIds: ids})
	}

	refusals := []struct {
		name   string
		ctx    context.Context
		req    *connect.Request[postpilotv1.ReorderImagesRequest]
		code   connect.Code
		reason string
	}{
		{"no session", context.Background(), request("c", "a", "b"), connect.CodeUnauthenticated, "AUTH_REQUIRED"},
		{"another account", auth.WithUser(context.Background(), "bob"), request("c", "a", "b"), connect.CodePermissionDenied, "POST_FORBIDDEN"},
		{"unknown post", alice, connect.NewRequest(&postpilotv1.ReorderImagesRequest{PostSlug: "nope", ImageIds: []string{"a"}}), connect.CodeNotFound, "POST_NOT_FOUND"},
		{"a stale list", alice, request("c", "a"), connect.CodeFailedPrecondition, "POST_PHOTO_ORDER_STALE"},
	}
	for _, refusal := range refusals {
		_, err := h.ReorderImages(refusal.ctx, refusal.req)
		if connect.CodeOf(err) != refusal.code || postAppErrorDetail(t, err).GetReason() != refusal.reason {
			t.Errorf("%s = %v, want %v %s", refusal.name, err, refusal.code, refusal.reason)
		}
	}
	if got, gotRevision := photoIDs(t, h, alice, slug); !reflect.DeepEqual(got, order) || gotRevision != revision {
		t.Fatalf("a refused reorder changed the post: %v at %d", got, gotRevision)
	}

	if _, err := h.ReorderImages(alice, request("c", "a", "b")); err != nil {
		t.Fatalf("ReorderImages = %v", err)
	}
	if got, gotRevision := photoIDs(t, h, alice, slug); !reflect.DeepEqual(got, []string{"c", "a", "b"}) || gotRevision != revision+1 {
		t.Fatalf("after the reorder = %v at %d, want [c a b] at %d", got, gotRevision, revision+1)
	}

	fields := (&postpilotv1.ReorderImagesRequest{}).ProtoReflect().Descriptor().Fields()
	for i := 0; i < fields.Len(); i++ {
		switch name := string(fields.Get(i).Name()); name {
		case "user_id", "account_id", "owner_id":
			t.Fatalf("ReorderImagesRequest carries %s", name)
		}
	}
}
