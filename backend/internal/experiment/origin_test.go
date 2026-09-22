package experiment

import (
	"context"
	"errors"
	"testing"
)

// ready starts a comparison, runs both candidates and returns it awaiting a verdict.
func ready(t *testing.T, svc *Service, store *memoryStore, request StartRequest) Experiment {
	t.Helper()
	started, err := svc.Start(context.Background(), request)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := svc.Handle(context.Background(), started.ExperimentID, func(string, int, int) {}); err != nil {
		t.Fatalf("handle: %v", err)
	}
	found, err := store.Get(context.Background(), started.ExperimentID)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	return found
}

func writeRequest(origin Origin) StartRequest {
	return StartRequest{
		UserID: "alice", PostSlug: "post", Stage: StageWrite, Origin: origin,
		ModelA: ModelRef{"p", "a"}, ModelB: ModelRef{"p", "b"},
	}
}

// A comparison's origin is frozen at start and is the caller's only for a write comparison:
// observe and analyze can be started nowhere but the lab, and an unstated origin is the
// editor, which is what every client predating the field was.
func TestStartFreezesTheComparisonOrigin(t *testing.T) {
	cases := []struct {
		name    string
		request StartRequest
		want    Origin
	}{
		{"write defaults to the editor", writeRequest(""), OriginEditor},
		{"write honours the editor", writeRequest(OriginEditor), OriginEditor},
		{"write honours the lab", writeRequest(OriginLab), OriginLab},
		{"observe is always the lab", StartRequest{UserID: "alice", PostSlug: "post", Stage: StageObserve, Origin: OriginEditor, ModelA: ModelRef{"p", "a"}, ModelB: ModelRef{"p", "b"}}, OriginLab},
		{"analyze is always the lab", StartRequest{UserID: "alice", VoiceID: "voice", Stage: StageAnalyze, Origin: OriginEditor, ModelA: ModelRef{"p", "a"}, ModelB: ModelRef{"p", "b"}}, OriginLab},
	}
	for _, sample := range cases {
		t.Run(sample.name, func(t *testing.T) {
			svc, store, _, _, _ := newTestService()
			svc.SetVoiceDirectory(okVoices{})
			started, err := svc.Start(context.Background(), sample.request)
			if err != nil {
				t.Fatalf("start: %v", err)
			}
			found, err := store.Get(context.Background(), started.ExperimentID)
			if err != nil {
				t.Fatal(err)
			}
			if found.Origin != sample.want {
				t.Fatalf("origin = %q, want %q", found.Origin, sample.want)
			}
		})
	}
}

type okVoices struct{}

func (okVoices) ActiveVoice(context.Context, string, string) error { return nil }

// A lab verdict is a ranking pick: it records the winner, applies nothing and releases the
// post it ran on, so the editor may start the next comparison at once.
func TestLabVerdictPicksWithoutApplyingAndReleasesThePost(t *testing.T) {
	svc, store, _, _, runner := newTestService()
	pair := ready(t, svc, store, writeRequest(OriginLab))
	decided, err := svc.Choose(context.Background(), "alice", pair.ID, pair.Candidates[0].ID, false, nil)
	if err != nil {
		t.Fatalf("choose: %v", err)
	}
	if decided.Status != StatusDecided || decided.Outcome != OutcomeWinner || decided.WinnerCandidateID != pair.Candidates[0].ID {
		t.Fatalf("verdict = %+v", decided)
	}
	if runner.applyCalls != 0 || decided.AppliedAt != nil || decided.ApplyRequested {
		t.Fatalf("lab pick applied: calls=%d applied=%v requested=%v", runner.applyCalls, decided.AppliedAt, decided.ApplyRequested)
	}
	pending, err := store.PendingForPost(context.Background(), "alice", "post")
	if err != nil || pending != nil {
		t.Fatalf("post still held: %+v, %v", pending, err)
	}
}

// The two verdict forms are exclusive. Each refuses the other's call before writing anything,
// so a client that asks with the wrong one cannot silently rank without applying, or apply
// without being asked to.
func TestVerdictFormsRefuseEachOther(t *testing.T) {
	t.Run("the editor has no pick-only verdict", func(t *testing.T) {
		svc, store, _, _, runner := newTestService()
		pair := ready(t, svc, store, writeRequest(OriginEditor))
		if _, err := svc.Choose(context.Background(), "alice", pair.ID, pair.Candidates[0].ID, false, nil); !errors.Is(err, ErrInvalidState) {
			t.Fatalf("choose = %v, want ErrInvalidState", err)
		}
		after, _ := store.Get(context.Background(), pair.ID)
		if after.Status != StatusReview || after.WinnerCandidateID != "" || runner.applyCalls != 0 {
			t.Fatalf("refusal wrote something: %+v applies=%d", after, runner.applyCalls)
		}
	})

	t.Run("the lab has no committing verdict", func(t *testing.T) {
		svc, store, catalog, _, runner := newTestService()
		pair := ready(t, svc, store, writeRequest(OriginLab))
		if _, err := svc.DecideWrite(context.Background(), "alice", pair.ID, pair.Candidates[0].ID, true, nil); !errors.Is(err, ErrInvalidState) {
			t.Fatalf("decide write = %v, want ErrInvalidState", err)
		}
		after, _ := store.Get(context.Background(), pair.ID)
		if after.Status != StatusReview || runner.applyCalls != 0 || len(catalog.adopted) != 0 {
			t.Fatalf("refusal wrote something: %+v applies=%d adopted=%v", after, runner.applyCalls, catalog.adopted)
		}
	})
}

// The survivor of a comparison whose sibling failed is not the paired verdict, so it stays
// available to both origins: it ranks nothing either way, and it applies exactly where a
// verdict applies — the editor's post has no content until it does (MODEL-34, MODEL-37).
func TestUnpairedSurvivorServesBothOrigins(t *testing.T) {
	cases := []struct {
		name        string
		origin      Origin
		wantApplies int
	}{
		{"the editor applies its survivor", OriginEditor, 1},
		{"the lab records it and applies nothing", OriginLab, 0},
	}
	for _, sample := range cases {
		t.Run(sample.name, func(t *testing.T) {
			svc, store, _, _, runner := newTestService()
			runner.fail["b"] = errors.New("provider failed")
			pair := ready(t, svc, store, writeRequest(sample.origin))
			if pair.Status != StatusPartial {
				t.Fatalf("status = %q, want partial", pair.Status)
			}
			var survivor string
			for _, candidate := range pair.Candidates {
				if candidate.Status == CandidateSucceeded {
					survivor = candidate.ID
				}
			}
			used, err := svc.Choose(context.Background(), "alice", pair.ID, survivor, true, nil)
			if err != nil || used.Outcome != OutcomeUnpaired || used.Status != StatusDecided {
				t.Fatalf("use single = %+v, %v", used, err)
			}
			if runner.applyCalls != sample.wantApplies {
				t.Fatalf("applies = %d, want %d", runner.applyCalls, sample.wantApplies)
			}
		})
	}
}

// A lab comparison's content application is offered against the post as it is now: a draft or
// a post in revision takes it, a finalized one does not, and a post that is gone is not a
// destination at all. Analyze publishes into a voice and never consults the post.
func TestLabApplicationFollowsThePostStatus(t *testing.T) {
	cases := []struct {
		name   string
		status string
		want   error
	}{
		{"draft", "draft", nil},
		{"in revision", "review", nil},
		{"finalized", "finalized", ErrPostFinalized},
	}
	for _, sample := range cases {
		t.Run(sample.name, func(t *testing.T) {
			svc, store, _, _, runner := newTestService()
			svc.posts.(*fakePosts).statuses["post"] = sample.status
			pair := ready(t, svc, store, writeRequest(OriginLab))
			if _, err := svc.Choose(context.Background(), "alice", pair.ID, pair.Candidates[0].ID, false, nil); err != nil {
				t.Fatal(err)
			}
			applied, err := svc.ApplyWinner(context.Background(), "alice", pair.ID, false)
			if !errors.Is(err, sample.want) {
				t.Fatalf("apply = %v, want %v", err, sample.want)
			}
			if sample.want != nil {
				if runner.applyCalls != 0 {
					t.Fatalf("refused application still called the runner: %d", runner.applyCalls)
				}
				after, _ := store.Get(context.Background(), pair.ID)
				if after.ApplyRequested {
					t.Fatal("refused application recorded a debt")
				}
				return
			}
			if applied.AppliedAt == nil || !applied.ApplyRequested || runner.applyCalls != 1 {
				t.Fatalf("applied = %+v applies=%d", applied, runner.applyCalls)
			}
		})
	}

	t.Run("a post that is gone is not a destination", func(t *testing.T) {
		svc, store, _, _, runner := newTestService()
		pair := ready(t, svc, store, writeRequest(OriginLab))
		if _, err := svc.Choose(context.Background(), "alice", pair.ID, pair.Candidates[0].ID, false, nil); err != nil {
			t.Fatal(err)
		}
		svc.posts.(*fakePosts).err = ErrInvalidState
		if _, err := svc.ApplyWinner(context.Background(), "alice", pair.ID, false); !errors.Is(err, ErrInvalidState) {
			t.Fatalf("apply = %v, want ErrInvalidState", err)
		}
		if runner.applyCalls != 0 {
			t.Fatalf("runner called for a missing post: %d", runner.applyCalls)
		}
	})

	t.Run("an editor verdict never asks about the post", func(t *testing.T) {
		svc, store, _, _, runner := newTestService()
		posts := svc.posts.(*fakePosts)
		posts.statuses["post"] = "finalized"
		pair := ready(t, svc, store, writeRequest(OriginEditor))
		decided, err := svc.DecideWrite(context.Background(), "alice", pair.ID, pair.Candidates[0].ID, false, nil)
		if err != nil || decided.AppliedAt == nil || runner.applyCalls != 1 || posts.calls != 0 {
			t.Fatalf("editor verdict = %+v err=%v applies=%d post reads=%d", decided, err, runner.applyCalls, posts.calls)
		}
	})

	t.Run("analyze publishes into its voice without a post", func(t *testing.T) {
		svc, store, _, _, runner := newTestService()
		svc.SetVoiceDirectory(okVoices{})
		posts := svc.posts.(*fakePosts)
		pair := ready(t, svc, store, StartRequest{UserID: "alice", VoiceID: "voice", Stage: StageAnalyze, ModelA: ModelRef{"p", "a"}, ModelB: ModelRef{"p", "b"}})
		if _, err := svc.Choose(context.Background(), "alice", pair.ID, pair.Candidates[0].ID, false, nil); err != nil {
			t.Fatal(err)
		}
		applied, err := svc.ApplyWinner(context.Background(), "alice", pair.ID, true)
		if err != nil || applied.AppliedAt == nil || runner.applyCalls != 1 || posts.calls != 0 {
			t.Fatalf("analyze apply = %+v err=%v applies=%d post reads=%d", applied, err, runner.applyCalls, posts.calls)
		}
	})
}

// Once a lab pick's owner asks for the content application, that application is owed: a
// failure leaves the comparison holding its post with a retry, exactly as the editor's
// failed application does, and the retry completes it.
func TestFailedLabApplicationKeepsThePostHeldUntilItSucceeds(t *testing.T) {
	svc, store, _, _, runner := newTestService()
	pair := ready(t, svc, store, writeRequest(OriginLab))
	if _, err := svc.Choose(context.Background(), "alice", pair.ID, pair.Candidates[0].ID, false, nil); err != nil {
		t.Fatal(err)
	}
	runner.applyErr = errors.New("post unavailable")
	failed, err := svc.ApplyWinner(context.Background(), "alice", pair.ID, false)
	if err != nil {
		t.Fatalf("apply = %v, want the failure recorded on the experiment", err)
	}
	if failed.AppliedAt != nil || !failed.ApplyRequested || failed.ApplyFailure == nil {
		t.Fatalf("failed application = %+v", failed)
	}
	pending, err := store.PendingForPost(context.Background(), "alice", "post")
	if err != nil || pending == nil || pending.ID != pair.ID {
		t.Fatalf("failed application released the post: %+v, %v", pending, err)
	}
	runner.applyErr = nil
	recovered, err := svc.ApplyWinner(context.Background(), "alice", pair.ID, false)
	if err != nil || recovered.AppliedAt == nil || recovered.ApplyFailure != nil || runner.applyCalls != 2 {
		t.Fatalf("retry = %+v err=%v applies=%d", recovered, err, runner.applyCalls)
	}
	if pending, err := store.PendingForPost(context.Background(), "alice", "post"); err != nil || pending != nil {
		t.Fatalf("applied comparison still holds the post: %+v, %v", pending, err)
	}
}

// Adopting the winning model is a follow-up in its own right, so it serves a lab pick that
// applied nothing.
func TestAdoptWinnerServesALabPick(t *testing.T) {
	svc, store, catalog, _, runner := newTestService()
	pair := ready(t, svc, store, writeRequest(OriginLab))
	decided, err := svc.Choose(context.Background(), "alice", pair.ID, pair.Candidates[0].ID, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	winner := decided.Winner()
	ref, stage, err := svc.AdoptWinner(context.Background(), "alice", pair.ID)
	if err != nil || stage != StageWrite || ref != winner.Model || len(catalog.adopted) != 1 || runner.applyCalls != 0 {
		t.Fatalf("adopt = %v/%v err=%v adopted=%v applies=%d", ref, stage, err, catalog.adopted, runner.applyCalls)
	}
}

// The origin has to reach whatever freezes the input, because the freezing side is what
// decides whether the preparing observation is written onto the post (MODEL-66).
func TestTheFrozenInputIsToldWhereTheComparisonStarted(t *testing.T) {
	cases := []struct {
		name    string
		request StartRequest
		want    Origin
	}{
		{"a lab write comparison", writeRequest(OriginLab), OriginLab},
		{"an editor write comparison", writeRequest(OriginEditor), OriginEditor},
		{"an unstated one, which is the editor", writeRequest(""), OriginEditor},
	}
	for _, sample := range cases {
		t.Run(sample.name, func(t *testing.T) {
			svc, _, _, _, runner := newTestService()
			if _, err := svc.Start(context.Background(), sample.request); err != nil {
				t.Fatal(err)
			}
			if len(runner.snapshotRequests) != 1 {
				t.Fatalf("snapshot requests = %d", len(runner.snapshotRequests))
			}
			if got := startOrigin(runner.snapshotRequests[0]); got != sample.want {
				t.Fatalf("the freezing side was told %q, want %q", got, sample.want)
			}
		})
	}
}
