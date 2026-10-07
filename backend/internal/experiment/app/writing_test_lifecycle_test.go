package app

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/experiment"
	"github.com/postpilot/backend/internal/job"
)

type lifecycleFixture struct {
	events          []string
	purged          bool
	source, pending []experiment.TestExecutionFence
	cancelError     error
}

func (f *lifecycleFixture) PurgeWritingTestPost(context.Context, string, string) error {
	f.events = append(f.events, "fence-private-purge")
	f.purged = true
	return nil
}
func (f *lifecycleFixture) PurgeExpiredWritingTests(context.Context, time.Time) (int64, error) {
	f.events = append(f.events, "current-expiry")
	return 1, nil
}
func (f *lifecycleFixture) WritingTestExecutionsForPost(context.Context, string, string) ([]experiment.TestExecutionFence, error) {
	f.events = append(f.events, "capture-source-epochs")
	return f.source, nil
}
func (f *lifecycleFixture) ListCancelledUnsettledTestExecutions(context.Context) ([]experiment.TestExecutionFence, error) {
	f.events = append(f.events, "closed-metadata")
	return f.pending, nil
}
func (f *lifecycleFixture) RecoverInterruptedTests(context.Context) error {
	f.events = append(f.events, "recover-epochs")
	return nil
}
func (f *lifecycleFixture) CancelWritingTestJob(_ context.Context, user, id string) error {
	f.events = append(f.events, "cancel:"+user+":"+id)
	return f.cancelError
}
func (f *lifecycleFixture) RecoverSettlements(context.Context) error {
	f.events = append(f.events, "boot-settlements")
	return nil
}
func (f *lifecycleFixture) ReconcileTerminalSettlements(context.Context) error {
	f.events = append(f.events, "terminal-only-settlements")
	return nil
}
func (f *lifecycleFixture) ReconcileCancelledSettlements(context.Context, []experiment.TestExecutionFence) error {
	f.events = append(f.events, "closed-epoch-settlements")
	return nil
}
func (f *lifecycleFixture) PurgeExpired(context.Context, time.Time) (int64, error) {
	f.events = append(f.events, "legacy-expiry")
	return 2, nil
}
func (f *lifecycleFixture) PurgePost(context.Context, string, string) error {
	f.events = append(f.events, "legacy-source-purge")
	return nil
}
func TestWritingTestLifecycleFencesSourceBeforeCancellationAndRetainedPurge(t *testing.T) {
	f := &lifecycleFixture{source: []experiment.TestExecutionFence{{UserID: "alice", JobID: "active"}}, pending: []experiment.TestExecutionFence{{UserID: "alice", JobID: "old"}, {UserID: "bob", JobID: "foreign"}}}
	lifecycle := NewWritingTestLifecycle(f, f, f)
	if err := lifecycle.PurgePost(t.Context(), "alice", "source"); err != nil {
		t.Fatal(err)
	}
	want := []string{"capture-source-epochs", "fence-private-purge", "cancel:alice:active", "closed-metadata", "cancel:alice:old", "legacy-source-purge"}
	if !reflect.DeepEqual(f.events, want) {
		t.Fatal(f.events)
	}
	f.events = nil
	f.source = nil
	f.cancelError = job.ErrNotFound
	if err := lifecycle.PurgePost(t.Context(), "alice", "source"); err != nil {
		t.Fatal("closed never-created job not recoverable", err)
	}
	if !f.purged || f.events[len(f.events)-1] != "legacy-source-purge" {
		t.Fatal("source retry lost durable cancellation")
	}
}
func TestWritingTestLifecycleSweepsBothPayloadStoresAndNeverUsesBootRecoveryDuringAdmission(t *testing.T) {
	f := &lifecycleFixture{}
	if count, err := NewWritingTestLifecycle(f, f, f).PurgeExpired(t.Context(), time.Now()); err != nil || count != 3 {
		t.Fatal(count, err)
	}
	want := []string{"current-expiry", "legacy-expiry", "closed-metadata", "closed-epoch-settlements", "terminal-only-settlements"}
	if !reflect.DeepEqual(f.events, want) {
		t.Fatal("live sweep closed a pending admission", f.events)
	}
	f.events = nil
	if err := NewWritingTestLifecycle(f, f, f).Recover(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f.events, []string{"recover-epochs", "closed-metadata", "boot-settlements"}) {
		t.Fatal("boot recovery order", f.events)
	}
}
func TestWritingTestLifecycleCancellationFailureLeavesPrivateFenceAndRetriesMetadata(t *testing.T) {
	f := &lifecycleFixture{source: []experiment.TestExecutionFence{{UserID: "alice", JobID: "active"}}, cancelError: errors.New("cancel response lost")}
	lifecycle := NewWritingTestLifecycle(f, f, f)
	if err := lifecycle.PurgePost(t.Context(), "alice", "source"); !errors.Is(err, f.cancelError) || !f.purged {
		t.Fatal("failure reopened private payload", err)
	}
	f.source = nil
	f.pending = []experiment.TestExecutionFence{{UserID: "alice", JobID: "active"}}
	f.cancelError = nil
	if err := lifecycle.PurgePost(t.Context(), "alice", "source"); err != nil {
		t.Fatal("purged source lost cancellation identity", err)
	}
}
