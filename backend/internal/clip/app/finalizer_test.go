package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
)

func TestFinalizerReturnsAnAlreadyFinalizedProjectWithoutWriting(t *testing.T) {
	p := oldProject()
	p.Finalized = &clip.Finalization{PlanRevision: 1, ResultID: "r1"}
	clips := newFakeClips(p)
	jobs := &fakeJobs{}
	f := NewFinalizer(memoryWriter(t), bindFakes(jobs, clips, nil), clips, clip.RenderConfig{}, nil)
	got, err := f.Finalize(context.Background(), clip.FinalizationRequest{UserID: "alice", ProjectID: "clip", ExpectedRevision: 1, ExpectedResultID: "r1"})
	if err != nil || got.Finalized == nil || len(clips.finalized) != 0 {
		t.Fatal(got, clips.finalized, err)
	}
}

func TestFinalizerRefusesAForeignProject(t *testing.T) {
	clips := newFakeClips(oldProject())
	f := NewFinalizer(memoryWriter(t), bindFakes(&fakeJobs{}, clips, nil), clips, clip.RenderConfig{}, nil)
	if _, err := f.Finalize(context.Background(), clip.FinalizationRequest{UserID: "bob", ProjectID: "clip", ExpectedRevision: 1, ExpectedResultID: "r1"}); !errors.Is(err, clip.ErrNotFound) || len(clips.finalized) != 0 {
		t.Fatal(err)
	}
}

func TestFinalizerResolvesALostCommitFromDurableIdentity(t *testing.T) {
	durable := oldProject()
	durable.Finalized = &clip.Finalization{PlanRevision: 1, ResultID: "r1"}
	outside := newFakeClips(durable)
	inside := newFakeClips(oldProject())
	inside.projectErr = errors.New("connection reset") // the transaction's reply is lost
	f := NewFinalizer(memoryWriter(t), bindFakes(&fakeJobs{}, inside, nil), outside, clip.RenderConfig{}, func() time.Time { return time.Unix(1, 0) })
	got, err := f.Finalize(context.Background(), clip.FinalizationRequest{UserID: "alice", ProjectID: "clip", ExpectedRevision: 1, ExpectedResultID: "r1"})
	if err != nil || got.Finalized == nil {
		t.Fatal("a finalization the rows already show must not be reported as failed", got, err)
	}
	if _, err := f.Finalize(context.Background(), clip.FinalizationRequest{UserID: "alice", ProjectID: "clip", ExpectedRevision: 2, ExpectedResultID: "r2"}); err == nil {
		t.Fatal("a different identity cannot borrow the durable finalization")
	}
}
