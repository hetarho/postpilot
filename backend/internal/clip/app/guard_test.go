package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/job"
)

type fakeAuthorizer struct{ calls int }

func (a *fakeAuthorizer) AuthorizeDispatch(context.Context, string, string) error {
	a.calls++
	return nil
}

func prepareJob() job.Job {
	j := runningJob(clip.JobKindGenerate)
	j.Stage, j.CancellationPolicyVersion = "prepare", 1
	return j
}
func holdFor(j job.Job) Hold {
	return Hold{UserID: j.UserID, Kind: j.Kind, JobID: j.ID, Reservation: Reservation{CancellationPolicyVersion: 1}}
}

func TestReservableNamesEveryConditionAHoldNeeds(t *testing.T) {
	ok := prepareJob()
	if !Reservable(ok, holdFor(ok)) {
		t.Fatal("the reference job must be reservable")
	}
	at := time.Now()
	mutations := map[string]func(*job.Job, *Hold){
		"foreign user":     func(j *job.Job, _ *Hold) { j.UserID = "bob" },
		"foreign hold":     func(_ *job.Job, h *Hold) { h.UserID = "bob" },
		"render kind":      func(j *job.Job, _ *Hold) { j.Kind = clip.JobKindRender },
		"not running":      func(j *job.Job, _ *Hold) { j.Status = job.StatusQueued },
		"past prepare":     func(j *job.Job, _ *Hold) { j.Stage = "render" },
		"cancel requested": func(j *job.Job, _ *Hold) { j.CancelRequestedAt = &at },
		"other policy":     func(j *job.Job, _ *Hold) { j.CancellationPolicyVersion = 2 },
	}
	for name, mutate := range mutations {
		j, h := prepareJob(), holdFor(prepareJob())
		mutate(&j, &h)
		if Reservable(j, h) {
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
	if err := g.Reserve(context.Background(), holdFor(j)); err != nil || len(admission.held) != 1 {
		t.Fatal(admission.held, err)
	}
	late := holdFor(j)
	late.Reservation.CancellationPolicyVersion = 2
	if err := g.Reserve(context.Background(), late); !errors.Is(err, clip.ErrCreditAllowance) || len(admission.held) != 1 {
		t.Fatal("a refused hold never reaches the ledger", err, admission.held)
	}
	if err := g.Authorize(context.Background(), "alice", "job"); err != nil || authorizer.calls != 1 {
		t.Fatal(err, authorizer.calls)
	}
	noLedger := NewGuard(memoryWriter(t), bindFakes(jobs, newFakeClips(oldProject()), nil), authorizer)
	if err := noLedger.Reserve(context.Background(), holdFor(j)); err == nil {
		t.Fatal("a missing ledger must fail closed")
	}
}
