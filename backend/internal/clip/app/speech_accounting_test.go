package app

import (
	"context"
	"github.com/postpilot/backend/internal/clip"
	"testing"
)

type accountingSpeechStore struct {
	SpeechStorage
	run SpeechRun
}

func (s accountingSpeechStore) GetSpeechRun(_ context.Context, owner, id string) (SpeechRun, error) {
	if owner != s.run.OwnerID || id != s.run.ID {
		return SpeechRun{}, clip.ErrNotFound
	}
	return s.run, nil
}

type speechAccountingReader struct {
	value clip.Accounting
	calls int
}

func (r *speechAccountingReader) ForJob(_ context.Context, owner, job string) (*clip.Accounting, error) {
	r.calls++
	copy := r.value
	return &copy, nil
}
func TestSpeechAccountingUsesOwnerBoundDurableManifestAndActualUnitLedger(t *testing.T) {
	approved, reserved, charge, refund := 17, 12, 5, 7
	ledger := &speechAccountingReader{value: clip.Accounting{ApprovedMax: &approved, Reserved: &reserved, FinalCharge: &charge, Refund: &refund, Settled: true}}
	run := SpeechRun{ID: "operation", OwnerID: "alice", ProjectID: "project", JobID: "job"}
	s := &GenerationService{speech: &SpeechService{SpeechDeps: SpeechDeps{Store: accountingSpeechStore{run: run}}}, accounting: ledger}
	job := &clip.ClipJob{ID: "job", Kind: clip.JobKindSpeech, Status: "failed", Payload: []byte("operation")}
	got, e := s.accountingForJob(t.Context(), "alice", "project", job)
	if e != nil || got.Status != "settled" || *got.ApprovedMax != 17 || *got.FinalCharge != 5 || *got.Refund != 7 {
		t.Fatal(got, e)
	}
	if _, e = s.accountingForJob(t.Context(), "bob", "project", job); e == nil || ledger.calls != 1 {
		t.Fatal("foreign manifest reached ledger", e)
	}
	got, e = s.accountingForJob(t.Context(), "alice", "other-project", job)
	if e != nil || got.Status != "unavailable" || ledger.calls != 1 {
		t.Fatal("foreign project accounting", got, e)
	}
	job.ID = "another-job"
	got, e = s.accountingForJob(t.Context(), "alice", "project", job)
	if e != nil || got.Status != "unavailable" || ledger.calls != 1 {
		t.Fatal("mismatched job accounting", got, e)
	}
}
