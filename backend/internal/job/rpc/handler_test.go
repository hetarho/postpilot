package rpc_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/auth"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/job"
	jobrpc "github.com/postpilot/backend/internal/job/rpc"
	jobstore "github.com/postpilot/backend/internal/job/store"
	"github.com/postpilot/backend/internal/platform/db"
)

func newHandler(t *testing.T) (*jobrpc.Handler, *job.Queue) {
	t.Helper()
	handle, err := db.Open(filepath.Join(t.TempDir(), "rpc.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { handle.Close() })
	ctx := context.Background()
	if err := db.Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, id := range []string{"alice", "bob"} {
		if _, err := handle.Writer.Exec(
			"INSERT INTO users (id, password_hash, created_at) VALUES (?, 'hash', ?)", id, now); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := handle.Writer.Exec(
		"INSERT INTO posts (slug, user_id, created_at, updated_at) VALUES ('post-a', 'alice', ?, ?)", now, now); err != nil {
		t.Fatal(err)
	}
	queue := job.New(jobstore.New(handle.Writer, handle.Reader), time.Second)
	return jobrpc.NewHandler(queue), queue
}

func TestGetGenerationMapsOwnershipAndMissing(t *testing.T) {
	handler, queue := newHandler(t)
	slug := "post-a"
	id, err := queue.Enqueue(context.Background(), job.NewJob{
		Kind: job.KindGenerate, UserID: "alice", PostSlug: &slug,
	})
	if err != nil {
		t.Fatal(err)
	}

	request := func(id, userID string) error {
		_, err := handler.GetGeneration(
			auth.WithUser(context.Background(), userID),
			connect.NewRequest(&postpilotv1.GetGenerationRequest{Id: id}),
		)
		return err
	}
	if code := connect.CodeOf(request(id, "bob")); code != connect.CodePermissionDenied {
		t.Errorf("foreign code = %s", code)
	}
	if code := connect.CodeOf(request("missing", "alice")); code != connect.CodeNotFound {
		t.Errorf("missing code = %s", code)
	}
	if err := request(id, "alice"); err != nil {
		t.Errorf("owner GetGeneration: %v", err)
	}
}
