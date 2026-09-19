package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/job"
)

type fakeAuthorizer struct{ calls int }

func (a *fakeAuthorizer) AuthorizeClipDispatch(context.Context, string, string) error {
	a.calls++
	return nil
}

func prepareJob() job.Job {
	j := runningJob(job.KindGenerateClip)
	j.Stage, j.CancellationPolicyVersion = "prepare", 1
	return j
}
func startFor(j job.Job) job.Start {
	return job.Start{UserID: j.UserID, Kind: j.Kind, JobID: j.ID, Clip: &job.ClipReservation{CancellationPolicyVersion: 1}}
}

func TestReservableNamesEveryConditionAHoldNeeds(t *testing.T) {
	ok := prepareJob()
	if !Reservable(ok, startFor(ok)) {
		t.Fatal("the reference job must be reservable")
	}
	at := time.Now()
	mutations := map[string]func(*job.Job, *job.Start){
		"no clip reservation": func(_ *job.Job, s *job.Start) { s.Clip = nil },
		"foreign user":        func(j *job.Job, _ *job.Start) { j.UserID = "bob" },
		"render kind":         func(j *job.Job, _ *job.Start) { j.Kind = job.KindRenderClip },
		"not running":         func(j *job.Job, _ *job.Start) { j.Status = job.StatusQueued },
		"past prepare":        func(j *job.Job, _ *job.Start) { j.Stage = "render" },
		"cancel requested":    func(j *job.Job, _ *job.Start) { j.CancelRequestedAt = &at },
		"other policy":        func(j *job.Job, _ *job.Start) { j.CancellationPolicyVersion = 2 },
	}
	for name, mutate := range mutations {
		j, s := prepareJob(), startFor(prepareJob())
		mutate(&j, &s)
		if Reservable(j, s) {
			t.Fatal(name, "must refuse a hold")
		}
	}
}

func TestGuardHoldsInsideTheTransactionOnlyForAReservableJob(t *testing.T) {
	j := prepareJob()
	jobs := &fakeJobs{jobs: map[string]job.Job{"job": j}}
	admission := &fakeAdmission{}
	authorizer := &fakeAuthorizer{}
	g := NewGuard(memoryWriter(t), bindFakes(jobs, newFakeClips(oldProject()), admission), authorizer)
	if err := g.Reserve(context.Background(), startFor(j)); err != nil || len(admission.held) != 1 {
		t.Fatal(admission.held, err)
	}
	late := startFor(j)
	late.Clip.CancellationPolicyVersion = 2
	if err := g.Reserve(context.Background(), late); !errors.Is(err, job.ErrCreditAllowance) || len(admission.held) != 1 {
		t.Fatal("a refused hold never reaches the ledger", err, admission.held)
	}
	if err := g.Authorize(context.Background(), "alice", "job"); err != nil || authorizer.calls != 1 {
		t.Fatal(err, authorizer.calls)
	}
	noLedger := NewGuard(memoryWriter(t), bindFakes(jobs, newFakeClips(oldProject()), nil), authorizer)
	if err := noLedger.Reserve(context.Background(), startFor(j)); err == nil {
		t.Fatal("a missing ledger must fail closed")
	}
}
