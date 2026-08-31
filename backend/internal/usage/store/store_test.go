package store_test

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/usage"
	usagestore "github.com/postpilot/backend/internal/usage/store"
)

type noModels struct{}

func (noModels) Lookup(llm.ModelRef) (llm.ModelInfo, bool) { return llm.ModelInfo{}, false }

func newService(t *testing.T) *usage.Service {
	t.Helper()
	handle, err := db.Open(filepath.Join(t.TempDir(), "usage.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { handle.Close() })
	ctx := context.Background()
	if err := db.Migrate(ctx, handle.Writer); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := handle.Writer.ExecContext(ctx,
		"INSERT INTO users (id, password_hash, plan, created_at) VALUES ('alice','hash','free',?)",
		time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return usage.NewService(usagestore.New(handle.Writer, handle.Reader), noModels{})
}

// A2 under concurrency: the count and the row that changes it are one decision. Without the
// admission transaction every one of these requests reads the same empty window and passes,
// which is how a 10-start day becomes a 20-start day.
func TestConcurrentAdmissionsCannotExceedTheDailyCount(t *testing.T) {
	svc := newService(t)
	const attempts = 20

	var wg sync.WaitGroup
	results := make([]error, attempts)
	start := make(chan struct{})
	for i := range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results[i] = svc.Admit(context.Background(), usage.Start{
				UserID: "alice", Plan: plan.Free, Kind: "generate", JobID: string(rune('a' + i)),
			})
		}()
	}
	close(start)
	wg.Wait()

	var admitted, refused int
	for _, err := range results {
		var quota *plan.QuotaError
		switch {
		case err == nil:
			admitted++
		case errors.As(err, &quota):
			refused++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if want := plan.LimitsFor(plan.Free).DailyJobStarts; admitted != want {
		t.Fatalf("admitted = %d, want exactly %d", admitted, want)
	}
	if refused != attempts-admitted {
		t.Fatalf("refused = %d, want %d", refused, attempts-admitted)
	}

	summary, err := svc.Summary(context.Background(), "alice")
	if err != nil {
		t.Fatal(err)
	}
	if summary.JobsStartedToday != admitted {
		t.Fatalf("ledger says %d starts, want %d", summary.JobsStartedToday, admitted)
	}
}

// The ledger and the admission table are read back through the same store the gate writes
// with, so a released admission really is gone from the window it was counted in.
func TestReleaseRemovesTheAdmissionFromTheWindow(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	if err := svc.Admit(ctx, usage.Start{UserID: "alice", Plan: plan.Free, Kind: "generate", JobID: "job"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Release(ctx, "job"); err != nil {
		t.Fatal(err)
	}
	summary, err := svc.Summary(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if summary.JobsStartedToday != 0 {
		t.Fatalf("starts = %d, want the released admission gone", summary.JobsStartedToday)
	}
}
