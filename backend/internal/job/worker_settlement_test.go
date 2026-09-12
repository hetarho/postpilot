package job

import (
	"context"
	"errors"
	"testing"
	"time"
)

type terminalStore struct {
	Store
	persisted    Job
	finishErr    error
	readErr      error
	commit       bool
	runningReads int
}

func (s *terminalStore) Finish(ctx context.Context, id, status string, failure *Failure, at time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.commit {
		s.persisted.ID, s.persisted.Status, s.persisted.Failure = id, status, failure
		s.persisted.FinishedAt = &at
	}
	return s.finishErr
}

func TestWorkerResourceReleaseUsesOnlyDurableTerminalTime(t *testing.T) {
	at := time.Now().UTC()
	for _, committed := range []bool{false, true} {
		t.Run(map[bool]string{false: "uncommitted", true: "committed"}[committed], func(t *testing.T) {
			store := &terminalStore{persisted: Job{Status: StatusRunning}, commit: committed, finishErr: errors.New("ambiguous terminal write")}
			q := New(store, time.Second)
			q.now = func() time.Time { return at }
			released := 0
			q.Register(KindRenderClip, func(context.Context, Job, Progress) error { return nil })
			q.OnTerminal(KindRenderClip, func(ctx context.Context, j Job, terminal time.Time) error {
				if ctx.Err() != nil || store.persisted.Status != StatusDone || store.persisted.FinishedAt == nil || !terminal.Equal(*store.persisted.FinishedAt) {
					t.Fatal("release preceded durable terminal outcome")
				}
				released++
				return errors.New("cleanup unavailable")
			})
			q.run(context.Background(), Job{ID: "render", Kind: KindRenderClip})
			if committed && (released != 1 || store.persisted.Status != StatusDone) || !committed && released != 0 {
				t.Fatal("incorrect terminal resource release", released)
			}
		})
	}
}

func (s *terminalStore) GetByID(context.Context, string) (Job, error) {
	if s.runningReads > 0 {
		s.runningReads--
		return Job{Status: StatusRunning}, nil
	}
	return s.persisted, s.readErr
}

type terminalAdmitter struct {
	Admitter
	statuses []string
	open     []string
	releases int
}

func (a *terminalAdmitter) Settle(ctx context.Context, _ string, status string) {
	if ctx.Err() != nil {
		panic("settling on canceled worker context")
	}
	a.statuses = append(a.statuses, status)
}

func (a *terminalAdmitter) OpenHolds(context.Context) ([]string, error) { return a.open, nil }
func (a *terminalAdmitter) Release(context.Context, string)             { a.releases++ }

func TestWorkerSettlesOnlyPersistedTerminalOutcome(t *testing.T) {
	errWrite := errors.New("terminal write failed")
	errRun := errors.New("provider failed")
	for _, tc := range []struct {
		name, persisted, want      string
		finishErr, readErr, runErr error
		commit, cancelWorker       bool
	}{
		{name: "success", commit: true, want: StatusDone},
		{name: "failure", commit: true, runErr: errRun, want: StatusFailed},
		{name: "failed write leaves running", persisted: StatusRunning, finishErr: errWrite},
		{name: "failed write and read", finishErr: errWrite, readErr: errors.New("read failed")},
		{name: "write committed but returned error", commit: true, finishErr: errWrite, want: StatusDone},
		{name: "persisted success overrides tentative failure", persisted: StatusDone, finishErr: errWrite, runErr: errRun, want: StatusDone},
		{name: "persisted failure overrides tentative success", persisted: StatusFailed, finishErr: errWrite, want: StatusFailed},
		{name: "success racing cancellation", commit: true, cancelWorker: true, want: StatusDone},
		{name: "failure racing cancellation", commit: true, cancelWorker: true, runErr: errRun, want: StatusFailed},
		{name: "interrupted work stays running", cancelWorker: true, runErr: context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &terminalStore{persisted: Job{Status: tc.persisted}, commit: tc.commit, finishErr: tc.finishErr, readErr: tc.readErr, runningReads: 2}
			admitter := &terminalAdmitter{}
			queue := New(store, time.Second)
			queue.Admit(admitter)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			queue.Register(KindGenerateClip, func(context.Context, Job, Progress) error {
				calls++
				if tc.cancelWorker {
					cancel()
				}
				return tc.runErr
			})
			queue.run(ctx, Job{ID: "clip", Kind: KindGenerateClip})
			if calls != 1 {
				t.Fatalf("handler called %d times", calls)
			}
			if tc.want == "" {
				if len(admitter.statuses) != 0 {
					t.Fatalf("unconfirmed outcome settled: %v", admitter.statuses)
				}
			} else if len(admitter.statuses) != 1 || admitter.statuses[0] != tc.want {
				t.Fatalf("settlement outcomes = %v, want %s", admitter.statuses, tc.want)
			}
		})
	}
}

func TestRecoveryUsesSamePersistedOutcomeWithoutHandlerReplay(t *testing.T) {
	for _, status := range []string{StatusQueued, StatusRunning, StatusDone, StatusFailed, StatusCancelled} {
		t.Run(status, func(t *testing.T) {
			store := &terminalStore{persisted: Job{ID: "clip", Kind: KindGenerateClip, Status: status}}
			admitter := &terminalAdmitter{open: []string{"clip"}}
			queue := New(store, time.Second)
			queue.Admit(admitter)
			queue.Register(KindGenerateClip, func(context.Context, Job, Progress) error { t.Fatal("recovery replayed provider work"); return nil })
			for range 2 {
				n, err := queue.SweepOpenHolds(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				if Terminal(status) {
					if n != 1 || admitter.statuses[len(admitter.statuses)-1] != status {
						t.Fatal(n, admitter.statuses)
					}
				} else if n != 0 || len(admitter.statuses) != 0 {
					t.Fatal(n, admitter.statuses)
				}
			}
		})
	}
	store := &terminalStore{readErr: ErrNotFound}
	admitter := &terminalAdmitter{open: []string{"missing"}}
	queue := New(store, time.Second)
	queue.Admit(admitter)
	if n, err := queue.SweepOpenHolds(context.Background()); err != nil || n != 1 || admitter.releases != 1 || len(admitter.statuses) != 0 {
		t.Fatal(n, err, admitter)
	}
}
