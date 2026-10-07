package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/job"
)

func TestBulkHistoryIsOwnedAndChoosesLatestRelevantJobBeforeFailure(t *testing.T) {
	source, handle := subjectHarness(t)
	ctx := context.Background()
	add := func(id, user, slug, kind, status, at string) {
		t.Helper()
		_, err := handle.Writer.Exec("INSERT INTO generation_jobs(id,user_id,post_slug,kind,status,payload,created_at,updated_at) VALUES(?,?,?,?,?,'',?,?)", id, user, slug, kind, status, at, at)
		if err != nil {
			t.Fatal(err)
		}
	}
	at := time.Now().UTC()
	add("failure", "alice", "post-alice", "ordinary", "failed", at.Format(time.RFC3339Nano))
	add("success", "alice", "post-alice", "ordinary", "done", at.Add(time.Second).Format(time.RFC3339Nano))
	add("secondary", "alice", "post-alice", "secondary", "failed", at.Add(2*time.Second).Format(time.RFC3339Nano))
	add("foreign", "bob", "post-bob", "ordinary", "failed", at.Add(3*time.Second).Format(time.RFC3339Nano))
	rows, err := source.LatestForSubjects(ctx, "alice", postSubject, []string{"post-alice", "post-bob"}, []string{"ordinary"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows["post-alice"].ID != "success" {
		t.Fatalf("owned latest rows: %v", rows)
	}
	rows, err = source.LatestForSubjects(ctx, "alice", postSubject, []string{"post-alice"}, nil)
	if err != nil || rows["post-alice"].ID != "secondary" {
		t.Fatalf("all-kind latest: %v %v", rows, err)
	}
	if _, err = source.LatestForSubjects(ctx, "alice", "unrecognized", []string{"post-alice"}, nil); err != job.ErrInvalidTarget {
		t.Fatalf("unknown dimension: %v", err)
	}
	rows, err = source.LatestForSubjects(ctx, "alice", postSubject, nil, nil)
	if err != nil || len(rows) != 0 {
		t.Fatalf("empty history: %v %v", rows, err)
	}
}
