package app

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
)

type analysisRecoveryRows struct {
	AnalysisPreparationStore
	rows []clip.AnalysisPreparation
}

func (s *analysisRecoveryRows) AnalysisPreparationsForRecovery(_ context.Context, after string) ([]clip.AnalysisPreparation, error) {
	var rows []clip.AnalysisPreparation
	for _, r := range s.rows {
		if r.ID > after && len(rows) < 100 {
			rows = append(rows, r)
		}
	}
	return rows, nil
}

type analysisRecoveryTx struct {
	AnalysisPreparationTx
	rows   map[string]clip.AnalysisPreparation
	seen   map[string]int
	closed int
}

func (s *analysisRecoveryTx) GetAnalysisPreparation(_ context.Context, _, id string) (clip.AnalysisPreparation, error) {
	return s.rows[id], nil
}
func (s *analysisRecoveryTx) AuthorizeAnalysisPreparation(_ context.Context, r clip.AnalysisPreparation, _ time.Time) error {
	s.seen[r.ID]++
	return nil
}
func (s *analysisRecoveryTx) AnalysisLeaseStopped(context.Context, clip.AnalysisPreparation, time.Time) (bool, error) {
	return true, nil
}
func (s *analysisRecoveryTx) SetAnalysisPreparationState(context.Context, string, string, string) error {
	s.closed++
	return nil
}
func (*analysisRecoveryTx) RetireAnalysisPreparation(context.Context, string, time.Time) error {
	return nil
}
func (*analysisRecoveryTx) MarkAnalysisPreparationReconciled(context.Context, string, time.Time) error {
	return nil
}

func TestAcceptedBrowserHandoffOutlivesVerificationClockAndRecoveryScanIsFair(t *testing.T) {
	now := time.Now()
	store := &analysisRecoveryRows{}
	ports := &analysisRecoveryTx{rows: map[string]clip.AnalysisPreparation{}, seen: map[string]int{}}
	for i := range 101 {
		r := clip.AnalysisPreparation{ID: fmt.Sprintf("session-%03d", i), UserID: "alice", State: "accepted", ExpiresAt: now.Add(time.Hour), DeadlineAt: now.Add(-time.Minute)}
		if i%2 == 1 {
			r.State = "consumed"
		}
		store.rows = append(store.rows, r)
		ports.rows[r.ID] = r
	}
	writer := sql.OpenDB(&txRecorder{})
	t.Cleanup(func() { writer.Close() })
	a := &AnalysisPreparations{writer: writer, bind: func(*sql.Tx) Ports { return Ports{Analysis: ports} }, store: store, now: func() time.Time { return now }, limits: clip.DefaultAnalysisPreparationLimits(clip.Environment{})}
	for range 2 {
		if e := a.Reconcile(t.Context()); e != nil {
			t.Fatal(e)
		}
	}
	if ports.closed != 0 || len(ports.seen) != 101 {
		t.Fatal("verification clock expired accepted work or first page starved later sessions", ports.closed, len(ports.seen))
	}
	if e := a.Reconcile(t.Context()); e != nil {
		t.Fatal(e)
	}
	if ports.seen["session-000"] != 2 {
		t.Fatal("bounded scan did not wrap to earlier live rows")
	}
}
