package store_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/experiment"
	experimentstore "github.com/postpilot/backend/internal/experiment/store"
	"github.com/postpilot/backend/internal/platform/db"
)

func testStore(t *testing.T) (*experimentstore.Store, *db.DB) {
	t.Helper()
	handle, err := db.Open(filepath.Join(t.TempDir(), "experiment.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = handle.Close() })
	if err := db.Migrate(context.Background(), handle.Writer); err != nil {
		t.Fatal(err)
	}
	now := "2026-08-29T00:00:00Z"
	for _, user := range []string{"alice", "bob"} {
		if _, err := handle.Writer.Exec(`INSERT INTO users(id,password_hash,created_at) VALUES(?,?,?)`, user, "hash", now); err != nil {
			t.Fatal(err)
		}
	}
	for _, row := range []struct{ slug, user string }{{"post-a", "alice"}, {"post-b", "bob"}} {
		if _, err := handle.Writer.Exec(`INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES(?,?,'기본 말투',1,?,?)`, "voice-"+row.user, row.user, now, now); err != nil {
			t.Fatal(err)
		}
		if _, err := handle.Writer.Exec(`INSERT INTO posts(slug,user_id,voice_id,created_at,updated_at) VALUES(?,?,?,?,?)`, row.slug, row.user, "voice-"+row.user, now, now); err != nil {
			t.Fatal(err)
		}
	}
	return experimentstore.New(handle.Writer, handle.Reader), handle
}

// The frozen voice round-trips.
func TestStorePersistsTheFrozenVoice(t *testing.T) {
	store, _ := testStore(t)
	ctx := context.Background()
	found := sample("exp-voice", "alice", "post-a", time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC))
	found.VoiceID = "voice-alice"
	if err := store.Create(ctx, found); err != nil {
		t.Fatal(err)
	}
	reloaded, err := store.Get(ctx, found.ID)
	if err != nil || reloaded.VoiceID != "voice-alice" {
		t.Fatalf("reloaded voice = %q err=%v", reloaded.VoiceID, err)
	}
}

func formatAt(value time.Time) string {
	return value.UTC().Format("2006-01-02T15:04:05.000000000Z07:00")
}

func sample(id, user, slug string, at time.Time) experiment.Experiment {
	targetLanguage := experiment.LanguageKorean
	return experiment.Experiment{
		ID: id, UserID: user, PostSlug: slug, Stage: experiment.StageWrite, Origin: experiment.OriginEditor, Status: experiment.StatusQueued,
		TargetLanguage: &targetLanguage,
		InputSnapshot:  []byte(`{"private":true}`), InputHash: "hash", PromptVersion: "v1", CreatedAt: at,
		Candidates: []experiment.Candidate{
			{ID: id + "-left", ExperimentID: id, Model: experiment.ModelRef{ProviderID: "p", ModelID: "a"}, ModelLabel: "A snapshot", DisplaySide: experiment.SideLeft, Status: experiment.CandidatePending},
			{ID: id + "-right", ExperimentID: id, Model: experiment.ModelRef{ProviderID: "p", ModelID: "b"}, ModelLabel: "B snapshot", DisplaySide: experiment.SideRight, Status: experiment.CandidatePending},
		},
	}
}

func TestStorePersistsFiveRankedCandidatesInDisplayOrder(t *testing.T) {
	store, _ := testStore(t)
	ctx := context.Background()
	found := sample("exp-five", "alice", "post-a", time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC))
	found.Origin = experiment.OriginLab
	found.ReviewMode = experiment.ReviewCandidateRanking
	found.Candidates = []experiment.Candidate{}
	for i, side := range []experiment.DisplaySide{experiment.SideE, experiment.SideC, experiment.SideLeft, experiment.SideD, experiment.SideRight} {
		found.Candidates = append(found.Candidates, experiment.Candidate{
			ID: fmt.Sprintf("exp-five-%d", i), ExperimentID: found.ID,
			Model:      experiment.ModelRef{ProviderID: "p", ModelID: fmt.Sprintf("%d", i)},
			ModelLabel: "Model", DisplaySide: side, Status: experiment.CandidatePending,
		})
	}
	if err := store.Create(ctx, found); err != nil {
		t.Fatal(err)
	}
	reloaded, err := store.Get(ctx, found.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.ReviewMode != experiment.ReviewCandidateRanking || len(reloaded.Candidates) != 5 {
		t.Fatalf("reloaded = %+v", reloaded)
	}
	for i, side := range []experiment.DisplaySide{experiment.SideLeft, experiment.SideRight, experiment.SideC, experiment.SideD, experiment.SideE} {
		if reloaded.Candidates[i].DisplaySide != side {
			t.Fatalf("position %d = %s, want %s", i, reloaded.Candidates[i].DisplaySide, side)
		}
	}
	legacy := sample("exp-legacy", "alice", "post-b", found.CreatedAt.Add(time.Second))
	if err := store.Create(ctx, legacy); err != nil {
		t.Fatal(err)
	}
	old, err := store.Get(ctx, legacy.ID)
	if err != nil || old.ReviewMode != experiment.ReviewPairwise {
		t.Fatalf("legacy = %+v, %v", old, err)
	}
}

func TestRankedCompletionPersistsTiesActionsAndPrivatePurge(t *testing.T) {
	store, handle := testStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	found := sample("ranked-completion", "alice", "post-a", now)
	found.ReviewMode = experiment.ReviewCandidateRanking
	found.Origin = experiment.OriginEditor
	if err := store.Create(ctx, found); err != nil {
		t.Fatal(err)
	}
	for _, candidate := range found.Candidates {
		candidate.Status = experiment.CandidateSucceeded
		candidate.Output = []byte(`{"private":"output"}`)
		if err := store.CompleteCandidate(ctx, candidate); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.SetStatus(ctx, found.ID, experiment.StatusReview, nil); err != nil {
		t.Fatal(err)
	}
	ranks := []experiment.CandidateRank{
		{CandidateID: found.Candidates[0].ID, Rank: 1, Badges: []experiment.Badge{experiment.BadgeFast}},
		{CandidateID: found.Candidates[1].ID, Rank: 1, Badges: []experiment.Badge{experiment.BadgeOther}, OtherNote: "private note"},
	}
	if ok, err := store.CompleteRanking(ctx, found.ID, "bob", ranks, now, now.Add(time.Hour)); err != nil || ok {
		t.Fatalf("cross-account completion=%v,%v", ok, err)
	}
	if ok, err := store.CompleteRanking(ctx, found.ID, "alice", ranks, now, now.Add(time.Hour)); err != nil || !ok {
		t.Fatalf("completion=%v,%v", ok, err)
	}
	reloaded, err := store.Get(ctx, found.ID)
	if err != nil || reloaded.Status != experiment.StatusCompleted || reloaded.CompletedAt == nil ||
		reloaded.Candidates[0].Rank != 1 || reloaded.Candidates[1].Rank != 1 || reloaded.Candidates[1].OtherNote != "private note" {
		t.Fatalf("reloaded=%+v, %v", reloaded, err)
	}
	if blocked, err := store.BlockingWriteForPost(ctx, "alice", "post-a"); err != nil || blocked != found.ID {
		t.Fatalf("block=%s,%v", blocked, err)
	}
	if err := store.MarkCandidateApply(ctx, found.ID, "alice", found.Candidates[0].ID, false); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkCandidateApply(ctx, found.ID, "alice", found.Candidates[1].ID, false); !errors.Is(err, experiment.ErrInvalidState) {
		t.Fatalf("second candidate apply: %v", err)
	}
	if err := store.SetApplied(ctx, found.ID, "alice", now); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkCandidateAdopt(ctx, found.ID, "alice", found.Candidates[1].ID); err != nil {
		t.Fatal(err)
	}
	if err := store.SetAdopted(ctx, found.ID, "alice", now); err != nil {
		t.Fatal(err)
	}
	if blocked, err := store.BlockingWriteForPost(ctx, "alice", "post-a"); err != nil || blocked != "" {
		t.Fatalf("block after apply=%s,%v", blocked, err)
	}
	if _, err := store.PurgeExpired(ctx, now.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	purged, err := store.Get(ctx, found.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(purged.InputSnapshot) != 0 || len(purged.Candidates[0].Output) != 0 || purged.Candidates[1].OtherNote != "" ||
		purged.Candidates[0].Rank != 1 || len(purged.Candidates[0].Badges) != 1 ||
		purged.AppliedCandidateID != found.Candidates[0].ID || purged.AdoptedCandidateID != found.Candidates[1].ID ||
		purged.AppliedAt == nil || purged.AdoptedAt == nil {
		t.Fatalf("purged=%+v", purged)
	}
	var fkErrors int
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM pragma_foreign_key_check`).Scan(&fkErrors); err != nil || fkErrors != 0 {
		t.Fatalf("foreign keys=%d,%v", fkErrors, err)
	}
}

func TestRankedPostPurgeKeepsEvaluationMetadata(t *testing.T) {
	store, _ := testStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	found := sample("ranked-post-purge", "alice", "post-a", now)
	found.ReviewMode = experiment.ReviewCandidateRanking
	found.Origin = experiment.OriginLab
	if err := store.Create(ctx, found); err != nil {
		t.Fatal(err)
	}
	for _, candidate := range found.Candidates {
		candidate.Status = experiment.CandidateSucceeded
		candidate.Output = []byte(`{"private":true}`)
		if err := store.CompleteCandidate(ctx, candidate); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.SetStatus(ctx, found.ID, experiment.StatusReview, nil); err != nil {
		t.Fatal(err)
	}
	ranks := []experiment.CandidateRank{{CandidateID: found.Candidates[0].ID, Rank: 1, Badges: []experiment.Badge{experiment.BadgeOther}, OtherNote: "private"},
		{CandidateID: found.Candidates[1].ID, Rank: 2}}
	if ok, err := store.CompleteRanking(ctx, found.ID, "alice", ranks, now, now.Add(30*24*time.Hour)); err != nil || !ok {
		t.Fatalf("complete=%v,%v", ok, err)
	}
	if err := store.PurgePost(ctx, "alice", "post-a"); err != nil {
		t.Fatal(err)
	}
	purged, err := store.Get(ctx, found.ID)
	if err != nil || len(purged.InputSnapshot) != 0 || len(purged.Candidates[0].Output) != 0 ||
		purged.Candidates[0].OtherNote != "" || purged.Candidates[0].Rank != 1 ||
		len(purged.Candidates[0].Badges) != 1 || purged.Candidates[0].Model.ModelID != "a" {
		t.Fatalf("post purge=%+v,%v", purged, err)
	}
}

func TestRankedLeaderboardDataRespectsScopeClockAndTally(t *testing.T) {
	store, _ := testStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	makeEvent := func(id, user string, when time.Time, rankValues []int) {
		found := sample(id, user, "", when)
		found.Stage = experiment.StageObserve
		found.TargetLanguage = nil
		found.Origin = experiment.OriginLab
		found.ReviewMode = experiment.ReviewCandidateRanking
		if len(rankValues) == 3 {
			found.Candidates = append(found.Candidates, experiment.Candidate{ID: id + "-third", ExperimentID: id,
				Model: experiment.ModelRef{ProviderID: "p", ModelID: "c"}, ModelLabel: "C",
				DisplaySide: experiment.SideC, Status: experiment.CandidatePending})
		}
		if err := store.Create(ctx, found); err != nil {
			t.Fatal(err)
		}
		ranks := []experiment.CandidateRank{}
		for i, candidate := range found.Candidates {
			candidate.Status = experiment.CandidateSucceeded
			candidate.Output = []byte(`[{"private":true}]`)
			candidate.Usage = experiment.Usage{PromptTokens: 11, LatencyMS: 25, CostSource: experiment.CostReported, CostMicrousd: 5}
			if err := store.CompleteCandidate(ctx, candidate); err != nil {
				t.Fatal(err)
			}
			rank := experiment.CandidateRank{CandidateID: candidate.ID, Rank: rankValues[i]}
			if i == 0 {
				rank.Badges = []experiment.Badge{experiment.BadgeFast}
			}
			ranks = append(ranks, rank)
		}
		if err := store.SetStatus(ctx, id, experiment.StatusReview, nil); err != nil {
			t.Fatal(err)
		}
		if ok, err := store.CompleteRanking(ctx, id, user, ranks, when, when.Add(30*24*time.Hour)); err != nil || !ok {
			t.Fatalf("complete %s=%v,%v", id, ok, err)
		}
	}
	makeEvent("ranked-alice", "alice", now, []int{1, 1, 2})
	makeEvent("ranked-bob", "bob", now.Add(time.Minute), []int{2, 1})
	mine, candidates, tallies, err := store.LeaderboardData(ctx, "alice", experiment.StageObserve, now.Add(-time.Hour), experiment.ScopeMe)
	if err != nil || len(mine) != 1 || len(candidates) != 3 || len(tallies) != 1 ||
		mine[0].Status != experiment.StatusCompleted || candidates[2].Rank != 2 || tallies[0].Badge != experiment.BadgeFast {
		t.Fatalf("mine events=%+v calls=%+v tallies=%+v err=%v", mine, candidates, tallies, err)
	}
	all, candidates, _, err := store.LeaderboardData(ctx, "alice", experiment.StageObserve, now.Add(-time.Hour), experiment.ScopeAll)
	if err != nil || len(all) != 2 || len(candidates) != 5 || all[0].ID != "ranked-alice" || all[1].ID != "ranked-bob" {
		t.Fatalf("all events=%+v calls=%d err=%v", all, len(candidates), err)
	}
	fromBob, _, _, err := store.LeaderboardData(ctx, "alice", experiment.StageObserve, now.Add(30*time.Second), experiment.ScopeAll)
	if err != nil || len(fromBob) != 1 || fromBob[0].ID != "ranked-bob" {
		t.Fatalf("window events=%+v err=%v", fromBob, err)
	}
}

func TestStorePersistsAndValidatesFrozenTargetLanguage(t *testing.T) {
	store, handle := testStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)

	english := experiment.LanguageEnglish
	found := sample("exp-en", "alice", "post-a", now)
	found.TargetLanguage = &english
	if err := store.Create(ctx, found); err != nil {
		t.Fatal(err)
	}
	reloaded, err := store.Get(ctx, found.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.TargetLanguage == nil || *reloaded.TargetLanguage != experiment.LanguageEnglish {
		t.Fatalf("target language = %v, want en", reloaded.TargetLanguage)
	}

	missing := sample("exp-missing", "alice", "post-b", now.Add(time.Second))
	missing.TargetLanguage = nil
	if err := store.Create(ctx, missing); !errors.Is(err, experiment.ErrLanguageRequired) {
		t.Fatalf("missing write language = %v, want ErrLanguageRequired", err)
	}

	invalid := experiment.Language("fr")
	bad := sample("exp-bad", "alice", "post-b", now.Add(2*time.Second))
	bad.TargetLanguage = &invalid
	if err := store.Create(ctx, bad); !errors.Is(err, experiment.ErrLanguageRequired) {
		t.Fatalf("invalid write language = %v, want ErrLanguageRequired", err)
	}

	observe := sample("exp-observe", "alice", "", now.Add(3*time.Second))
	observe.Stage, observe.Origin = experiment.StageObserve, experiment.OriginLab
	observe.TargetLanguage = nil
	if err := store.Create(ctx, observe); err != nil {
		t.Fatal(err)
	}
	observeReloaded, err := store.Get(ctx, observe.ID)
	if err != nil || observeReloaded.TargetLanguage != nil {
		t.Fatalf("observe target = %v, err=%v", observeReloaded.TargetLanguage, err)
	}

	if _, err := handle.Writer.ExecContext(ctx, `UPDATE model_experiments SET target_language = NULL WHERE id = ?`, found.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(ctx, found.ID); !errors.Is(err, experiment.ErrLanguageRequired) {
		t.Fatalf("invalid persisted write target = %v, want ErrLanguageRequired", err)
	}
}

func TestStoreOwnershipStableSidesAndUnresolvedWriteGuard(t *testing.T) {
	store, _ := testStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	found := sample("exp-1", "alice", "post-a", now)
	if err := store.Create(ctx, found); err != nil {
		t.Fatal(err)
	}
	reloaded, err := store.Get(ctx, found.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Candidates[0].DisplaySide != experiment.SideLeft || reloaded.Candidates[0].ModelLabel != "A snapshot" {
		t.Fatalf("reloaded = %+v", reloaded.Candidates)
	}
	if pending, err := store.PendingForPost(ctx, "alice", "post-a"); err != nil || pending == nil || pending.ID != found.ID {
		t.Fatalf("pending = %+v, %v", pending, err)
	}
	if pending, err := store.PendingForPost(ctx, "bob", "post-a"); err != nil || pending != nil {
		t.Fatalf("foreign pending = %+v, %v", pending, err)
	}
	if err := store.Create(ctx, sample("exp-2", "alice", "post-a", now.Add(time.Second))); !errors.Is(err, experiment.ErrInvalidState) {
		t.Fatalf("duplicate unresolved = %v", err)
	}
}

// GEN-23, GEN-38: only an editor write comparison still queued, running, partial or in review,
// or decided with a requested application or adoption pending, holds the post's generation and
// revision. A lab comparison and a failed one never do, though both stay the post's unresolved
// comparison (MODEL-34), and a resolved one releases it.
func TestBlockingWriteForPostIsTheEditorComparisonStillInFlight(t *testing.T) {
	for name, tc := range map[string]struct {
		origin, status string
		apply, applied bool
		adopt, adopted bool
		blocks         bool
	}{
		"queued":                {origin: "editor", status: "queued", blocks: true},
		"running":               {origin: "editor", status: "running", blocks: true},
		"partial":               {origin: "editor", status: "partial", blocks: true},
		"review":                {origin: "editor", status: "review", blocks: true},
		"failed":                {origin: "editor", status: "failed"},
		"decided, apply due":    {origin: "editor", status: "decided", apply: true, blocks: true},
		"decided, adoption due": {origin: "editor", status: "decided", apply: true, applied: true, adopt: true, blocks: true},
		"decided and applied":   {origin: "editor", status: "decided", apply: true, applied: true},
		"dismissed":             {origin: "editor", status: "dismissed"},
		"lab queued":            {origin: "lab", status: "queued"},
		"lab review":            {origin: "lab", status: "review"},
	} {
		t.Run(name, func(t *testing.T) {
			store, handle := testStore(t)
			ctx := context.Background()
			now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
			if err := store.Create(ctx, sample("exp-1", "alice", "post-a", now)); err != nil {
				t.Fatal(err)
			}
			at := func(set bool) any {
				if set {
					return now.Format(time.RFC3339)
				}
				return nil
			}
			flag := func(set bool) int {
				if set {
					return 1
				}
				return 0
			}
			if _, err := handle.Writer.ExecContext(ctx,
				`UPDATE model_experiments SET origin=?, status=?, apply_requested=?, applied_at=?, adoption_requested=?, adopted_at=? WHERE id='exp-1'`,
				tc.origin, tc.status, flag(tc.apply), at(tc.applied), flag(tc.adopt), at(tc.adopted)); err != nil {
				t.Fatal(err)
			}
			id, err := store.BlockingWriteForPost(ctx, "alice", "post-a")
			if err != nil {
				t.Fatal(err)
			}
			if (id == "exp-1") != tc.blocks || (id != "" && id != "exp-1") {
				t.Fatalf("blocking = %q, want blocks=%v", id, tc.blocks)
			}
			if foreign, err := store.BlockingWriteForPost(ctx, "bob", "post-a"); err != nil || foreign != "" {
				t.Fatalf("foreign blocking = %q, %v", foreign, err)
			}
		})
	}
}

// The service checks the voice before creating the experiment, but deletion may commit
// between that read and this write. Preserve the trigger's lifecycle error at the context
// boundary so callers receive FailedPrecondition rather than Internal.
func TestStoreMapsInactiveVoiceTrigger(t *testing.T) {
	store, handle := testStore(t)
	ctx := context.Background()
	if _, err := handle.Writer.ExecContext(ctx,
		"INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES('voice-alice-2','alice','다른 말투',0,?,?)",
		"2026-08-30T00:00:00Z", "2026-08-30T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.ExecContext(ctx,
		"UPDATE voices SET deleted_at = ? WHERE id = 'voice-alice-2'",
		"2026-08-30T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	found := sample("exp-deleted-voice", "alice", "post-a", time.Now().UTC())
	found.VoiceID = "voice-alice-2"
	if err := store.Create(ctx, found); !errors.Is(err, experiment.ErrVoiceUnavailable) {
		t.Fatalf("inactive voice create = %v, want ErrVoiceUnavailable", err)
	}
}

func TestStorePreservesSiblingOutputAndPurgesPrivateContent(t *testing.T) {
	store, _ := testStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	found := sample("exp-1", "alice", "post-a", now)
	if err := store.Create(ctx, found); err != nil {
		t.Fatal(err)
	}
	finished := now.Add(time.Second)
	left := found.Candidates[0]
	left.Status = experiment.CandidateSucceeded
	left.Output = []byte(`{"title":"paid result"}`)
	left.Usage = experiment.Usage{PromptTokens: 12, CostMicrousd: 7, CostSource: experiment.CostReported, LatencyMS: 90}
	left.FinishedAt = &finished
	right := found.Candidates[1]
	right.Status = experiment.CandidateFailed
	right.Failure = &experiment.Failure{Reason: "MODEL_RATE_LIMITED", Params: map[string]string{"retry": "later"}, TechnicalDetail: "provider quota"}
	right.Usage = experiment.Usage{CostSource: experiment.CostUnavailable, LatencyMS: 110}
	right.FinishedAt = &finished
	if err := store.CompleteCandidate(ctx, left); err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteCandidate(ctx, right); err != nil {
		t.Fatal(err)
	}
	if err := store.SetStatus(ctx, found.ID, experiment.StatusPartial, &finished); err != nil {
		t.Fatal(err)
	}
	reloaded, _ := store.Get(ctx, found.ID)
	failed := reloaded.Candidates[1]
	if string(reloaded.Candidates[0].Output) != string(left.Output) || failed.Status != experiment.CandidateFailed ||
		failed.Failure == nil || failed.Failure.Reason != "MODEL_RATE_LIMITED" || failed.Failure.Params["retry"] != "later" ||
		failed.Failure.TechnicalDetail != "provider quota" {
		t.Fatalf("partial lost sibling output: %+v", reloaded.Candidates)
	}
	failed.Failure.Params["retry"] = "mutated"
	reloadedAgain, err := store.Get(ctx, found.ID)
	if err != nil || reloadedAgain.Candidates[1].Failure.Params["retry"] != "later" {
		t.Fatalf("failure params alias store state: %+v, err=%v", reloadedAgain.Candidates[1], err)
	}
	decided := now.Add(2 * time.Second)
	changed, err := store.Decide(ctx, found.ID, "alice", left.ID, experiment.StatusDecided, experiment.OutcomeUnpaired, true, false, nil, decided, decided)
	if err != nil || !changed {
		t.Fatalf("decide = %v, %v", changed, err)
	}
	if n, err := store.PurgeExpired(ctx, decided.Add(time.Second)); err != nil || n != 1 {
		t.Fatalf("purge = %d, %v", n, err)
	}
	reloaded, _ = store.Get(ctx, found.ID)
	if len(reloaded.InputSnapshot) != 0 || len(reloaded.Candidates[0].Output) != 0 || reloaded.Candidates[0].ModelLabel != "A snapshot" || reloaded.Candidates[0].Usage.PromptTokens != 12 {
		t.Fatalf("purge removed durable metadata or retained payload: %+v", reloaded)
	}
}

func TestCandidateFailureRetryRestoreAndSuccessClearAtomically(t *testing.T) {
	store, handle := testStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	found := sample("exp-failure-clear", "alice", "post-a", now)
	if err := store.Create(ctx, found); err != nil {
		t.Fatal(err)
	}
	failed := found.Candidates[0]
	failed.Status = experiment.CandidateFailed
	failed.Failure = &experiment.Failure{
		Reason: "MODEL_RATE_LIMITED", Params: map[string]string{"safe": "value"}, TechnicalDetail: "provider detail",
	}
	finished := now.Add(time.Second)
	failed.StartedAt, failed.FinishedAt = &now, &finished
	if err := store.CompleteCandidate(ctx, failed); err != nil {
		t.Fatal(err)
	}
	if count, err := store.ResetFailedCandidates(ctx, found.ID); err != nil || count != 1 {
		t.Fatalf("reset = %d, %v", count, err)
	}
	reset, err := store.Get(ctx, found.ID)
	if err != nil || reset.Candidates[0].Status != experiment.CandidatePending || reset.Candidates[0].Failure != nil {
		t.Fatalf("reset candidate = %+v, err=%v", reset.Candidates[0], err)
	}
	withoutFailure := failed
	withoutFailure.Failure = nil
	if err := store.RestoreFailedCandidates(ctx, found.ID, []experiment.Candidate{withoutFailure}); err == nil || !strings.Contains(err.Error(), "failure is required") {
		t.Fatalf("restore without failure error = %v", err)
	}
	if err := store.RestoreFailedCandidates(ctx, found.ID, []experiment.Candidate{failed}); err != nil {
		t.Fatal(err)
	}
	restored, err := store.Get(ctx, found.ID)
	if err != nil || restored.Candidates[0].Failure == nil || restored.Candidates[0].Failure.Reason != "MODEL_RATE_LIMITED" ||
		restored.Candidates[0].Failure.Params["safe"] != "value" || restored.Candidates[0].Failure.TechnicalDetail != "provider detail" {
		t.Fatalf("restored candidate = %+v, err=%v", restored.Candidates[0], err)
	}
	if err := store.StartCandidate(ctx, found.ID, failed.ID, now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	succeeded := restored.Candidates[0]
	succeeded.Status = experiment.CandidateSucceeded
	succeeded.Failure = nil
	succeeded.Output = []byte(`{"ok":true}`)
	succeeded.FinishedAt = ptrTime(now.Add(3 * time.Second))
	if err := store.CompleteCandidate(ctx, succeeded); err != nil {
		t.Fatal(err)
	}
	var raw, reason, params, detail any
	if err := handle.Reader.QueryRow(
		"SELECT error, error_reason, error_params, technical_detail FROM model_experiment_candidates WHERE id=?", failed.ID,
	).Scan(&raw, &reason, &params, &detail); err != nil {
		t.Fatal(err)
	}
	if raw != nil || reason != nil || params != nil || detail != nil {
		t.Fatalf("candidate failure columns not cleared: %#v %#v %#v %#v", raw, reason, params, detail)
	}
}

func TestStoreMapsLegacyCandidateFailureAndRejectsMalformedParams(t *testing.T) {
	t.Run("legacy", func(t *testing.T) {
		store, handle := testStore(t)
		ctx := context.Background()
		found := sample("exp-legacy", "alice", "post-a", time.Now().UTC())
		if err := store.Create(ctx, found); err != nil {
			t.Fatal(err)
		}
		if _, err := handle.Writer.Exec(
			"UPDATE model_experiment_candidates SET status='failed', error='legacy candidate detail', error_reason=NULL, error_params=NULL, technical_detail=NULL WHERE id=?",
			found.Candidates[0].ID,
		); err != nil {
			t.Fatal(err)
		}
		reloaded, err := store.Get(ctx, found.ID)
		failure := reloaded.Candidates[0].Failure
		if err != nil || failure == nil || failure.Reason != experiment.FailureReasonUnknown ||
			failure.TechnicalDetail != "legacy candidate detail" || failure.Params != nil {
			t.Fatalf("legacy failure = %#v, err=%v", failure, err)
		}
	})

	t.Run("malformed params", func(t *testing.T) {
		store, handle := testStore(t)
		ctx := context.Background()
		found := sample("exp-malformed", "alice", "post-a", time.Now().UTC())
		if err := store.Create(ctx, found); err != nil {
			t.Fatal(err)
		}
		if _, err := handle.Writer.Exec("PRAGMA ignore_check_constraints = ON"); err != nil {
			t.Fatal(err)
		}
		if _, err := handle.Writer.Exec(
			"UPDATE model_experiment_candidates SET error_reason='MODEL_UNAVAILABLE', error_params='[]' WHERE id=?",
			found.Candidates[0].ID,
		); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Get(ctx, found.ID); err == nil || !strings.Contains(err.Error(), "JSON object") {
			t.Fatalf("malformed params error = %v", err)
		}
	})

	malformed := map[string]struct {
		set  string
		want string
	}{
		"invalid JSON params": {
			set:  "error_reason='MODEL_UNAVAILABLE', error_params='{not-json}', technical_detail=NULL",
			want: "decode failure params",
		},
		"non-string param value": {
			set:  "error_reason='MODEL_UNAVAILABLE', error_params='{\"retry\":1}', technical_detail=NULL",
			want: "decode failure params",
		},
		"reason without params": {
			set:  "error_reason='MODEL_UNAVAILABLE', error_params=NULL, technical_detail=NULL",
			want: "JSON object",
		},
		"params without reason": {
			set:  "error_reason=NULL, error_params='{}', technical_detail=NULL",
			want: "without reason",
		},
		"detail without reason": {
			set:  "error_reason=NULL, error_params=NULL, technical_detail='provider detail'",
			want: "without reason",
		},
		"invalid lowercase reason": {
			set:  "error_reason='model_unavailable', error_params='{}', technical_detail=NULL",
			want: "invalid reason",
		},
		"invalid repeated underscore": {
			set:  "error_reason='MODEL__UNAVAILABLE', error_params='{}', technical_detail=NULL",
			want: "invalid reason",
		},
		"invalid trailing underscore": {
			set:  "error_reason='MODEL_UNAVAILABLE_', error_params='{}', technical_detail=NULL",
			want: "invalid reason",
		},
		"invalid leading digit": {
			set:  "error_reason='1MODEL_UNAVAILABLE', error_params='{}', technical_detail=NULL",
			want: "invalid reason",
		},
	}
	for name, test := range malformed {
		t.Run(name, func(t *testing.T) {
			store, handle := testStore(t)
			ctx := context.Background()
			found := sample("exp-malformed", "alice", "post-a", time.Now().UTC())
			if err := store.Create(ctx, found); err != nil {
				t.Fatal(err)
			}
			if _, err := handle.Writer.Exec("PRAGMA ignore_check_constraints = ON"); err != nil {
				t.Fatal(err)
			}
			if _, err := handle.Writer.Exec(
				"UPDATE model_experiment_candidates SET "+test.set+" WHERE id=?", found.Candidates[0].ID,
			); err != nil {
				t.Fatal(err)
			}
			if _, err := store.Get(ctx, found.ID); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("malformed failure error = %v, want %q", err, test.want)
			}
		})
	}

	t.Run("invalid reason rejected before write", func(t *testing.T) {
		store, _ := testStore(t)
		ctx := context.Background()
		found := sample("exp-invalid-write", "alice", "post-a", time.Now().UTC())
		if err := store.Create(ctx, found); err != nil {
			t.Fatal(err)
		}
		candidate := found.Candidates[0]
		candidate.Status = experiment.CandidateFailed
		candidate.Failure = &experiment.Failure{Reason: "not_stable"}
		if err := store.CompleteCandidate(ctx, candidate); err == nil || !strings.Contains(err.Error(), "invalid reason") {
			t.Fatalf("invalid reason write error = %v", err)
		}
	})
}

func ptrTime(value time.Time) *time.Time { return &value }

func TestStorePostHookAndAccountCascade(t *testing.T) {
	store, handle := testStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	postExperiment := sample("exp-post", "alice", "post-a", now)
	if err := store.Create(ctx, postExperiment); err != nil {
		t.Fatal(err)
	}
	candidate := postExperiment.Candidates[0]
	candidate.Status = experiment.CandidateSucceeded
	candidate.Output = []byte(`{"secret":true}`)
	if err := store.CompleteCandidate(ctx, candidate); err != nil {
		t.Fatal(err)
	}
	if err := store.PurgePost(ctx, "alice", "post-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.Exec(`DELETE FROM posts WHERE slug='post-a'`); err != nil {
		t.Fatal(err)
	}
	reloaded, err := store.Get(ctx, postExperiment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.PostSlug != "" || len(reloaded.InputSnapshot) != 0 || len(reloaded.Candidates[0].Output) != 0 {
		t.Fatalf("post deletion retained content: %+v", reloaded)
	}

	accountExperiment := sample("exp-account", "bob", "post-b", now)
	if err := store.Create(ctx, accountExperiment); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.Exec(`DELETE FROM users WHERE id='bob'`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(ctx, accountExperiment.ID); !errors.Is(err, experiment.ErrNotFound) {
		t.Fatalf("account cascade get = %v", err)
	}
}

func TestStoreRecoversInterruptedCandidatesAtomically(t *testing.T) {
	store, _ := testStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	found := sample("exp-running", "alice", "post-a", now)
	if err := store.Create(ctx, found); err != nil {
		t.Fatal(err)
	}
	if err := store.SetStatus(ctx, found.ID, experiment.StatusRunning, nil); err != nil {
		t.Fatal(err)
	}
	if err := store.StartCandidate(ctx, found.ID, found.Candidates[0].ID, now); err != nil {
		t.Fatal(err)
	}
	left := found.Candidates[0]
	left.Status = experiment.CandidateSucceeded
	left.Output = []byte(`{"ok":true}`)
	left.FinishedAt = &now
	if err := store.CompleteCandidate(ctx, left); err != nil {
		t.Fatal(err)
	}

	finished := now.Add(time.Minute)
	interrupted := experiment.Failure{Reason: experiment.FailureReasonInterrupted}
	count, err := store.RecoverInterrupted(ctx, interrupted, finished)
	if err != nil || count != 1 {
		t.Fatalf("recover = %d, %v", count, err)
	}
	reloaded, _ := store.Get(ctx, found.ID)
	if reloaded.Status != experiment.StatusPartial || reloaded.FinishedAt == nil {
		t.Fatalf("reloaded = %+v", reloaded)
	}
	if reloaded.Candidates[0].Status != experiment.CandidateSucceeded || reloaded.Candidates[1].Status != experiment.CandidateFailed ||
		reloaded.Candidates[1].Failure == nil || reloaded.Candidates[1].Failure.Reason != experiment.FailureReasonInterrupted {
		t.Fatalf("candidates = %+v", reloaded.Candidates)
	}
	if count, err := store.RecoverInterrupted(ctx, interrupted, finished); err != nil || count != 0 {
		t.Fatalf("second recovery = %d, %v", count, err)
	}
}

func TestPendingWriteKeepsUnappliedVerdictRecoverable(t *testing.T) {
	store, handle := testStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	found := sample("exp-apply", "alice", "post-a", now)
	if err := store.Create(ctx, found); err != nil {
		t.Fatal(err)
	}
	finished := now.Add(time.Second)
	for _, candidate := range found.Candidates {
		candidate.Status = experiment.CandidateSucceeded
		candidate.Output = []byte(`{"title":"ok"}`)
		candidate.FinishedAt = &finished
		if err := store.CompleteCandidate(ctx, candidate); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.SetStatus(ctx, found.ID, experiment.StatusReview, &finished); err != nil {
		t.Fatal(err)
	}
	decided := now.Add(2 * time.Second)
	if changed, err := store.Decide(ctx, found.ID, "alice", found.Candidates[0].ID, experiment.StatusDecided, experiment.OutcomeWinner, true, true, nil, decided, decided.Add(30*24*time.Hour)); err != nil || !changed {
		t.Fatalf("decide = %v, %v", changed, err)
	}
	if pending, err := store.PendingForPost(ctx, "alice", "post-a"); err != nil || pending == nil || pending.ID != found.ID {
		t.Fatalf("unapplied pending = %+v, %v", pending, err)
	}
	applyFailure := experiment.Failure{Reason: "MODEL_UNAVAILABLE", TechnicalDetail: "provider detail"}
	if err := store.SetApplyFailure(ctx, found.ID, "alice", applyFailure); err != nil {
		t.Fatal(err)
	}
	if reloaded, err := store.Get(ctx, found.ID); err != nil || reloaded.ApplyFailure == nil ||
		reloaded.ApplyFailure.Reason != "MODEL_UNAVAILABLE" || reloaded.ApplyFailure.TechnicalDetail != "provider detail" {
		t.Fatalf("apply failure reload = %+v, %v", reloaded, err)
	}
	if err := store.SetApplied(ctx, found.ID, "alice", decided.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	var applyRaw, applyReason, applyParams, applyDetail any
	if err := handle.Reader.QueryRow(
		"SELECT apply_error, apply_error_reason, apply_error_params, apply_technical_detail FROM model_experiments WHERE id=?", found.ID,
	).Scan(&applyRaw, &applyReason, &applyParams, &applyDetail); err != nil {
		t.Fatal(err)
	}
	if applyRaw != nil || applyReason != nil || applyParams != nil || applyDetail != nil {
		t.Fatalf("apply failure columns not cleared: %#v %#v %#v %#v", applyRaw, applyReason, applyParams, applyDetail)
	}
	if pending, err := store.PendingForPost(ctx, "alice", "post-a"); err != nil || pending == nil || pending.ID != found.ID {
		t.Fatalf("requested adoption should remain pending after apply = %+v, %v", pending, err)
	}
	adoptionFailure := experiment.Failure{Reason: experiment.FailureReasonUnknown, Params: map[string]string{"safe": "value"}}
	if err := store.SetAdoptionFailure(ctx, found.ID, "alice", adoptionFailure); err != nil {
		t.Fatal(err)
	}
	reloaded, err := store.Get(ctx, found.ID)
	if err != nil || reloaded.AdoptionFailure == nil || reloaded.AdoptionFailure.Reason != experiment.FailureReasonUnknown ||
		reloaded.AdoptionFailure.Params["safe"] != "value" || reloaded.AdoptedAt != nil {
		t.Fatalf("adoption error reload = %+v, %v", reloaded, err)
	}
	if pending, err := store.PendingForPost(ctx, "alice", "post-a"); err != nil || pending == nil || pending.ID != found.ID {
		t.Fatalf("adoption retry pending = %+v, %v", pending, err)
	}
	if err := store.Create(ctx, sample("exp-blocked", "alice", "post-a", decided.Add(2*time.Second))); !errors.Is(err, experiment.ErrInvalidState) {
		t.Fatalf("new comparison during adoption retry = %v", err)
	}
	if err := store.SetAdopted(ctx, found.ID, "alice", decided.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	var adoptionRaw, adoptionReason, adoptionParams, adoptionDetail any
	if err := handle.Reader.QueryRow(
		"SELECT adoption_error, adoption_error_reason, adoption_error_params, adoption_technical_detail FROM model_experiments WHERE id=?", found.ID,
	).Scan(&adoptionRaw, &adoptionReason, &adoptionParams, &adoptionDetail); err != nil {
		t.Fatal(err)
	}
	if adoptionRaw != nil || adoptionReason != nil || adoptionParams != nil || adoptionDetail != nil {
		t.Fatalf("adoption failure columns not cleared: %#v %#v %#v %#v", adoptionRaw, adoptionReason, adoptionParams, adoptionDetail)
	}
	reloaded, err = store.Get(ctx, found.ID)
	if err != nil || reloaded.AdoptionFailure != nil || reloaded.AdoptedAt == nil {
		t.Fatalf("adopted reload = %+v, %v", reloaded, err)
	}
	if pending, err := store.PendingForPost(ctx, "alice", "post-a"); err != nil || pending != nil {
		t.Fatalf("adopted pending = %+v, %v", pending, err)
	}
	if err := store.Create(ctx, sample("exp-next", "alice", "post-a", decided.Add(3*time.Second))); err != nil {
		t.Fatalf("new comparison after adoption = %v", err)
	}
}

// The unresolved-per-post guard and its query read the same definition: a decided comparison
// holds its post only while an application or adoption it asked for is incomplete. A lab pick
// asks for neither, so it releases the post the moment it is decided and the next comparison
// may start.
func TestLabPickReleasesThePostAndAnAskedApplicationHoldsIt(t *testing.T) {
	t.Run("a pick that applies nothing releases the post", func(t *testing.T) {
		store, _ := testStore(t)
		ctx := context.Background()
		now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
		found := sample("exp-pick", "alice", "post-a", now)
		found.Origin = experiment.OriginLab
		if err := store.Create(ctx, found); err != nil {
			t.Fatal(err)
		}
		if err := store.SetStatus(ctx, found.ID, experiment.StatusReview, &now); err != nil {
			t.Fatal(err)
		}
		if reloaded, err := store.Get(ctx, found.ID); err != nil || reloaded.Origin != experiment.OriginLab || reloaded.ApplyRequested {
			t.Fatalf("lab round trip = %+v, %v", reloaded, err)
		}
		decided := now.Add(time.Second)
		if changed, err := store.Decide(ctx, found.ID, "alice", found.Candidates[0].ID, experiment.StatusDecided, experiment.OutcomeWinner, false, false, nil, decided, decided.Add(time.Hour)); err != nil || !changed {
			t.Fatalf("decide = %v, %v", changed, err)
		}
		if pending, err := store.PendingForPost(ctx, "alice", "post-a"); err != nil || pending != nil {
			t.Fatalf("pick still holds the post = %+v, %v", pending, err)
		}
		if err := store.Create(ctx, sample("exp-after-pick", "alice", "post-a", decided.Add(time.Second))); err != nil {
			t.Fatalf("new comparison after a pick = %v", err)
		}
	})

	t.Run("an asked application holds the post until it completes", func(t *testing.T) {
		store, _ := testStore(t)
		ctx := context.Background()
		now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
		found := sample("exp-ask", "alice", "post-a", now)
		found.Origin = experiment.OriginLab
		if err := store.Create(ctx, found); err != nil {
			t.Fatal(err)
		}
		if err := store.SetStatus(ctx, found.ID, experiment.StatusReview, &now); err != nil {
			t.Fatal(err)
		}
		decided := now.Add(time.Second)
		if changed, err := store.Decide(ctx, found.ID, "alice", found.Candidates[0].ID, experiment.StatusDecided, experiment.OutcomeWinner, false, false, nil, decided, decided.Add(time.Hour)); err != nil || !changed {
			t.Fatalf("decide = %v, %v", changed, err)
		}
		if err := store.SetApplyRequested(ctx, found.ID, "alice"); err != nil {
			t.Fatal(err)
		}
		reloaded, err := store.Get(ctx, found.ID)
		if err != nil || !reloaded.ApplyRequested {
			t.Fatalf("asked application = %+v, %v", reloaded, err)
		}
		if pending, err := store.PendingForPost(ctx, "alice", "post-a"); err != nil || pending == nil || pending.ID != found.ID {
			t.Fatalf("asked application does not hold the post = %+v, %v", pending, err)
		}
		if err := store.Create(ctx, sample("exp-blocked-by-ask", "alice", "post-a", decided.Add(time.Second))); !errors.Is(err, experiment.ErrInvalidState) {
			t.Fatalf("new comparison during an owed application = %v", err)
		}
		if err := store.SetApplied(ctx, found.ID, "alice", decided.Add(2*time.Second)); err != nil {
			t.Fatal(err)
		}
		if pending, err := store.PendingForPost(ctx, "alice", "post-a"); err != nil || pending != nil {
			t.Fatalf("completed application still holds the post = %+v, %v", pending, err)
		}
		// Asking again after the application completed changes nothing: the debt is over.
		if err := store.SetApplyRequested(ctx, found.ID, "alice"); err != nil {
			t.Fatal(err)
		}
		if pending, err := store.PendingForPost(ctx, "alice", "post-a"); err != nil || pending != nil {
			t.Fatalf("repeat ask revived the debt = %+v, %v", pending, err)
		}
	})
}

// The board's two scopes read the same rows through different predicates: `me` is the
// caller's own verdicts, `all` is every account's. Both are bounded by the window, and both
// leave a comparison still awaiting its verdict out of the call accounting.
// A dismissal reaches the board query beside the winner verdicts, in decision order, since
// a dismissal of two delivered candidates is a counted outcome (MODEL-38).
// A lab pick requests nothing at its verdict; its adoption follow-up marks the debt first,
// and only then can adopted_at land (MODEL-36).
func TestALabPickRecordsItsAdoptionOnceRequested(t *testing.T) {
	store, _ := testStore(t)
	ctx := context.Background()
	now := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	found := sample("exp-lab", "alice", "post-a", now)
	found.Origin = experiment.OriginLab
	if err := store.Create(ctx, found); err != nil {
		t.Fatal(err)
	}
	for _, candidate := range found.Candidates {
		candidate.Status = experiment.CandidateSucceeded
		candidate.Output = []byte(`{"title":"ok"}`)
		candidate.FinishedAt = &now
		if err := store.CompleteCandidate(ctx, candidate); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.SetStatus(ctx, found.ID, experiment.StatusReview, &now); err != nil {
		t.Fatal(err)
	}
	if changed, err := store.Decide(ctx, found.ID, "alice", found.Candidates[0].ID, experiment.StatusDecided, experiment.OutcomeWinner, false, false, nil, now, now.Add(time.Hour)); err != nil || !changed {
		t.Fatalf("decide = %v, %v", changed, err)
	}
	if err := store.SetAdopted(ctx, found.ID, "alice", now); err == nil {
		if reloaded, _ := store.Get(ctx, found.ID); reloaded.AdoptedAt != nil {
			t.Fatal("an adoption nobody requested was recorded")
		}
	}
	for range 2 {
		if err := store.SetAdoptionRequested(ctx, found.ID, "alice"); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.SetAdopted(ctx, found.ID, "alice", now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	reloaded, err := store.Get(ctx, found.ID)
	if err != nil || !reloaded.AdoptionRequested || reloaded.AdoptedAt == nil {
		t.Fatalf("adoption = %+v, %v", reloaded, err)
	}
	if err := store.SetAdoptionRequested(ctx, found.ID, "alice"); err != nil {
		t.Fatalf("a repeat after adoption = %v", err)
	}
}

func TestLeaderboardDataCarriesDismissalsInDecisionOrder(t *testing.T) {
	store, _ := testStore(t)
	ctx := context.Background()
	now := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	found := sample("exp-dismissed", "alice", "post-a", now.Add(-time.Hour))
	found.Origin = experiment.OriginLab
	if err := store.Create(ctx, found); err != nil {
		t.Fatal(err)
	}
	at := now.Add(-time.Hour)
	for _, candidate := range found.Candidates {
		candidate.Status = experiment.CandidateSucceeded
		candidate.Output = []byte(`{"title":"ok"}`)
		candidate.FinishedAt = &at
		if err := store.CompleteCandidate(ctx, candidate); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.SetStatus(ctx, found.ID, experiment.StatusReview, &at); err != nil {
		t.Fatal(err)
	}
	if changed, err := store.Decide(ctx, found.ID, "alice", "", experiment.StatusDismissed, experiment.OutcomeSkipped, false, false, nil, at, at.Add(time.Hour)); err != nil || !changed {
		t.Fatalf("dismiss = %v, %v", changed, err)
	}
	for _, scope := range []experiment.Scope{experiment.ScopeMe, experiment.ScopeAll} {
		decided, calls, _, err := store.LeaderboardData(ctx, "alice", experiment.StageWrite, now.Add(-24*time.Hour), scope)
		if err != nil {
			t.Fatal(err)
		}
		if len(decided) != 1 || decided[0].ID != found.ID || decided[0].Status != experiment.StatusDismissed || len(calls) != 2 {
			t.Fatalf("%s: dismissal missing from the board data: %+v (%d calls)", scope, decided, len(calls))
		}
	}
}

func TestLeaderboardDataFollowsItsScopeAndWindow(t *testing.T) {
	store, handle := testStore(t)
	ctx := context.Background()
	now := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	if _, err := handle.Writer.ExecContext(ctx,
		`INSERT INTO posts(slug,user_id,voice_id,created_at,updated_at) VALUES('post-c','alice','voice-alice',?,?)`,
		"2026-08-29T00:00:00Z", "2026-08-29T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	decide := func(id, user, slug string, at time.Time) {
		t.Helper()
		found := sample(id, user, slug, at)
		found.Origin = experiment.OriginLab
		if err := store.Create(ctx, found); err != nil {
			t.Fatal(err)
		}
		for _, candidate := range found.Candidates {
			candidate.Status = experiment.CandidateSucceeded
			candidate.Output = []byte(`{"title":"ok"}`)
			candidate.FinishedAt = &at
			if err := store.CompleteCandidate(ctx, candidate); err != nil {
				t.Fatal(err)
			}
		}
		if err := store.SetStatus(ctx, id, experiment.StatusReview, &at); err != nil {
			t.Fatal(err)
		}
		if changed, err := store.Decide(ctx, id, user, found.Candidates[0].ID, experiment.StatusDecided,
			experiment.OutcomeWinner, false, false, nil, at, at.Add(time.Hour)); err != nil || !changed {
			t.Fatalf("decide %s = %v, %v", id, changed, err)
		}
	}
	decide("exp-mine-fresh", "alice", "post-a", now.Add(-2*time.Hour))
	decide("exp-mine-stale", "alice", "post-c", now.Add(-20*24*time.Hour))
	decide("exp-theirs", "bob", "post-b", now.Add(-3*time.Hour))
	// Still awaiting its verdict: it belongs to no board.
	undecided := sample("exp-open", "alice", "", now.Add(-time.Hour))
	undecided.Stage, undecided.Origin, undecided.TargetLanguage = experiment.StageObserve, experiment.OriginLab, nil
	if err := store.Create(ctx, undecided); err != nil {
		t.Fatal(err)
	}

	week := now.Add(-7 * 24 * time.Hour)
	mine, mineCalls, _, err := store.LeaderboardData(ctx, "alice", experiment.StageWrite, week, experiment.ScopeMe)
	if err != nil {
		t.Fatal(err)
	}
	if len(mine) != 1 || mine[0].ID != "exp-mine-fresh" || len(mineCalls) != 2 {
		t.Fatalf("my week = %d verdicts %d calls (%+v)", len(mine), len(mineCalls), mine)
	}
	all, allCalls, _, err := store.LeaderboardData(ctx, "alice", experiment.StageWrite, week, experiment.ScopeAll)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 || len(allCalls) != 4 {
		t.Fatalf("the shared week = %d verdicts %d calls (%+v)", len(all), len(allCalls), all)
	}
	// Verdicts arrive in decision order, which is the order the replay depends on.
	if all[0].ID != "exp-theirs" || all[1].ID != "exp-mine-fresh" {
		t.Fatalf("shared verdicts out of decision order: %s then %s", all[0].ID, all[1].ID)
	}
	month, _, _, err := store.LeaderboardData(ctx, "alice", experiment.StageWrite, now.Add(-30*24*time.Hour), experiment.ScopeMe)
	if err != nil {
		t.Fatal(err)
	}
	if len(month) != 2 {
		t.Fatalf("my month = %d verdicts, want the stale one back (%+v)", len(month), month)
	}
	observe, observeCalls, _, err := store.LeaderboardData(ctx, "alice", experiment.StageObserve, week, experiment.ScopeAll)
	if err != nil {
		t.Fatal(err)
	}
	if len(observe) != 0 || len(observeCalls) != 0 {
		t.Fatalf("an undecided comparison reached a board: %d verdicts %d calls", len(observe), len(observeCalls))
	}
}

// Target language does not partition the board (LANG-18): two comparisons frozen to
// different targets replay onto the same one.
func TestLeaderboardDataDoesNotPartitionByTargetLanguage(t *testing.T) {
	store, _ := testStore(t)
	ctx := context.Background()
	now := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	english := experiment.LanguageEnglish
	for i, target := range []*experiment.Language{nil, &english} {
		id := []string{"exp-ko", "exp-en"}[i]
		slug := []string{"post-a", "post-b"}[i]
		user := []string{"alice", "bob"}[i]
		found := sample(id, user, slug, now.Add(-time.Duration(i+1)*time.Hour))
		found.Origin = experiment.OriginLab
		if target != nil {
			found.TargetLanguage = target
		}
		if err := store.Create(ctx, found); err != nil {
			t.Fatal(err)
		}
		at := now.Add(-time.Duration(i+1) * time.Hour)
		if err := store.SetStatus(ctx, id, experiment.StatusReview, &at); err != nil {
			t.Fatal(err)
		}
		if changed, err := store.Decide(ctx, id, user, found.Candidates[0].ID, experiment.StatusDecided,
			experiment.OutcomeWinner, false, false, nil, at, at.Add(time.Hour)); err != nil || !changed {
			t.Fatalf("decide %s = %v, %v", id, changed, err)
		}
	}
	all, _, _, err := store.LeaderboardData(ctx, "alice", experiment.StageWrite, now.Add(-7*24*time.Hour), experiment.ScopeAll)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("target language partitioned the board: %+v", all)
	}
}

// Badges ride the verdict's own transaction, survive a reload, and their free note leaves
// with the rest of the private payload while the ids stay for a board to tally (MODEL-42).
func TestVerdictBadgesArePersistedAndTheirNotesPurged(t *testing.T) {
	store, handle := testStore(t)
	ctx := context.Background()
	now := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	found := sample("exp-badges", "alice", "post-a", now)
	found.Origin = experiment.OriginLab
	if err := store.Create(ctx, found); err != nil {
		t.Fatal(err)
	}
	if err := store.SetStatus(ctx, found.ID, experiment.StatusReview, &now); err != nil {
		t.Fatal(err)
	}
	decided := now.Add(time.Second)
	badges := []experiment.CandidateBadges{
		{CandidateID: found.Candidates[0].ID, Badges: []experiment.Badge{experiment.BadgeFast, experiment.BadgeAccurate}},
		{CandidateID: found.Candidates[1].ID, Badges: []experiment.Badge{experiment.BadgeOther}, OtherNote: "형식이 흔들려요"},
	}
	if changed, err := store.Decide(ctx, found.ID, "alice", found.Candidates[0].ID, experiment.StatusDecided,
		experiment.OutcomeWinner, false, false, badges, decided, decided); err != nil || !changed {
		t.Fatalf("decide = %v, %v", changed, err)
	}
	reloaded, err := store.Get(ctx, found.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, candidate := range reloaded.Candidates {
		switch candidate.ID {
		case found.Candidates[0].ID:
			if len(candidate.Badges) != 2 {
				t.Fatalf("winner badges = %v", candidate.Badges)
			}
		case found.Candidates[1].ID:
			if candidate.OtherNote != "형식이 흔들려요" {
				t.Fatalf("loser note = %q", candidate.OtherNote)
			}
		}
	}
	if _, err := store.PurgeExpired(ctx, decided.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	purged, err := store.Get(ctx, found.ID)
	if err != nil {
		t.Fatal(err)
	}
	kept := 0
	for _, candidate := range purged.Candidates {
		kept += len(candidate.Badges)
		if candidate.OtherNote != "" {
			t.Fatalf("a private note outlived the retention window: %q", candidate.OtherNote)
		}
	}
	if kept != 3 {
		t.Fatalf("badge ids kept = %d, want all three", kept)
	}
	// The row itself is gone with the experiment, never orphaned.
	if _, err := handle.Writer.ExecContext(ctx, `DELETE FROM model_experiments WHERE id = ?`, found.ID); err != nil {
		t.Fatal(err)
	}
	var orphans int
	if err := handle.Reader.QueryRowContext(ctx,
		`SELECT count(*) FROM model_experiment_badges WHERE experiment_id = ?`, found.ID).Scan(&orphans); err != nil {
		t.Fatal(err)
	}
	if orphans != 0 {
		t.Fatalf("badges outlived their experiment: %d", orphans)
	}
}

// A verdict written again replaces its explanation rather than accumulating one.
func TestVerdictBadgesAreReplacedRatherThanAccumulated(t *testing.T) {
	store, _ := testStore(t)
	ctx := context.Background()
	now := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	found := sample("exp-replace", "alice", "post-a", now)
	found.Origin = experiment.OriginLab
	if err := store.Create(ctx, found); err != nil {
		t.Fatal(err)
	}
	decide := func(badge experiment.Badge) {
		t.Helper()
		if err := store.SetStatus(ctx, found.ID, experiment.StatusReview, &now); err != nil {
			t.Fatal(err)
		}
		if changed, err := store.Decide(ctx, found.ID, "alice", found.Candidates[0].ID, experiment.StatusDecided,
			experiment.OutcomeWinner, false, false,
			[]experiment.CandidateBadges{{CandidateID: found.Candidates[0].ID, Badges: []experiment.Badge{badge}}},
			now.Add(time.Second), now.Add(time.Hour)); err != nil || !changed {
			t.Fatalf("decide = %v, %v", changed, err)
		}
	}
	decide(experiment.BadgeFast)
	decide(experiment.BadgeConcise)
	reloaded, err := store.Get(ctx, found.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, candidate := range reloaded.Candidates {
		if candidate.ID != found.Candidates[0].ID {
			continue
		}
		if len(candidate.Badges) != 1 || candidate.Badges[0] != experiment.BadgeConcise {
			t.Fatalf("badges accumulated across verdicts: %v", candidate.Badges)
		}
	}
}

// The board's tallies are grouped by the model a candidate ran, not by the candidate, and
// they follow the same scope and window the ratings do. The note is never selected.
func TestBadgeTalliesFollowTheBoardsScopeAndWindow(t *testing.T) {
	store, handle := testStore(t)
	ctx := context.Background()
	now := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	if _, err := handle.Writer.ExecContext(ctx,
		`INSERT INTO posts(slug,user_id,voice_id,created_at,updated_at) VALUES('post-c','alice','voice-alice',?,?)`,
		"2026-08-29T00:00:00Z", "2026-08-29T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	decide := func(id, user, slug string, at time.Time, badges []experiment.CandidateBadges) {
		t.Helper()
		found := sample(id, user, slug, at)
		found.Origin = experiment.OriginLab
		if err := store.Create(ctx, found); err != nil {
			t.Fatal(err)
		}
		if err := store.SetStatus(ctx, id, experiment.StatusReview, &at); err != nil {
			t.Fatal(err)
		}
		for i := range badges {
			badges[i].CandidateID = found.Candidates[i].ID
		}
		if changed, err := store.Decide(ctx, id, user, found.Candidates[0].ID, experiment.StatusDecided,
			experiment.OutcomeWinner, false, false, badges, at, at.Add(time.Hour)); err != nil || !changed {
			t.Fatalf("decide %s = %v, %v", id, changed, err)
		}
	}
	// Two of my own comparisons in the week, one of them older than a day, plus one that is
	// someone else's and one of mine that aged out of the month.
	decide("exp-1", "alice", "post-a", now.Add(-2*time.Hour), []experiment.CandidateBadges{
		{Badges: []experiment.Badge{experiment.BadgeFast}},
		{Badges: []experiment.Badge{experiment.BadgeSlow, experiment.BadgeOther}, OtherNote: "사적인 메모"},
	})
	decide("exp-2", "alice", "post-c", now.Add(-3*24*time.Hour), []experiment.CandidateBadges{
		{Badges: []experiment.Badge{experiment.BadgeFast}},
	})
	decide("exp-3", "bob", "post-b", now.Add(-4*time.Hour), []experiment.CandidateBadges{
		{Badges: []experiment.Badge{experiment.BadgeFast}},
	})

	countOf := func(tallies []experiment.BadgeTally, model string, badge experiment.Badge) int {
		for _, tally := range tallies {
			if tally.Model.ModelID == model && tally.Badge == badge {
				return tally.Count
			}
		}
		return 0
	}

	_, _, day, err := store.LeaderboardData(ctx, "alice", experiment.StageWrite, now.Add(-24*time.Hour), experiment.ScopeMe)
	if err != nil {
		t.Fatal(err)
	}
	if countOf(day, "a", experiment.BadgeFast) != 1 || countOf(day, "b", experiment.BadgeSlow) != 1 {
		t.Fatalf("day tallies = %+v", day)
	}
	_, _, week, err := store.LeaderboardData(ctx, "alice", experiment.StageWrite, now.Add(-7*24*time.Hour), experiment.ScopeMe)
	if err != nil {
		t.Fatal(err)
	}
	// The same model earned the same badge in two of my comparisons: one row, count two.
	if countOf(week, "a", experiment.BadgeFast) != 2 {
		t.Fatalf("week tallies did not group by model: %+v", week)
	}
	_, _, all, err := store.LeaderboardData(ctx, "alice", experiment.StageWrite, now.Add(-7*24*time.Hour), experiment.ScopeAll)
	if err != nil {
		t.Fatal(err)
	}
	if countOf(all, "a", experiment.BadgeFast) != 3 {
		t.Fatalf("shared tallies = %+v", all)
	}
	// `other` is counted; its note is not part of a tally at all.
	if countOf(week, "b", experiment.BadgeOther) != 1 {
		t.Fatalf("other was not counted: %+v", week)
	}
	for _, tally := range all {
		if tally.Badge == "" || tally.Count <= 0 {
			t.Fatalf("a tally carried nothing usable: %+v", tally)
		}
	}
}

// MODEL-67, MODEL-42, VOICE-13: a voice-sourced comparison names no post, round-trips its source,
// prompt and answer ids, is kept apart from the post history, loses its snapshot (the answer's
// text) and both pieces to the purge, and never holds its voice's deletion.
func TestStoreKeepsAVoiceSourcedComparison(t *testing.T) {
	store, handle := testStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	voiced := sample("exp-voice", "alice", "", now)
	voiced.Origin, voiced.Source, voiced.VoiceID = experiment.OriginLab, experiment.SourceVoice, "voice-alice"
	voiced.VoicePromptKey, voiced.VoiceMaterialID = "opening_greeting", "answer-1"
	voiced.InputSnapshot = []byte(`{"answer":"내 답"}`)
	if err := store.Create(ctx, voiced); err != nil {
		t.Fatal(err)
	}
	// A second one of the same voice is not an unresolved post comparison.
	second := sample("exp-voice-2", "alice", "", now.Add(time.Second))
	second.Origin, second.Source, second.VoiceID, second.VoicePromptKey = experiment.OriginLab, experiment.SourceVoice, "voice-alice", "closing_greeting"
	if err := store.Create(ctx, second); err != nil {
		t.Fatalf("a second voice comparison: %v", err)
	}
	if err := store.Create(ctx, sample("exp-post", "alice", "post-a", now)); err != nil {
		t.Fatal(err)
	}
	reloaded, err := store.Get(ctx, voiced.ID)
	if err != nil || reloaded.Source != experiment.SourceVoice || reloaded.PostSlug != "" || reloaded.VoicePromptKey != "opening_greeting" || reloaded.VoiceMaterialID != "answer-1" {
		t.Fatalf("reloaded = %+v err=%v", reloaded, err)
	}
	if post, _ := store.Get(ctx, "exp-post"); post.Source != experiment.SourcePost {
		t.Fatalf("a post comparison reads source %q", post.Source)
	}
	for source, want := range map[experiment.Source]int{experiment.SourceVoice: 2, experiment.SourcePost: 1, "": 3} {
		if found, err := store.List(ctx, "alice", experiment.StageWrite, source); err != nil || len(found) != want {
			t.Fatalf("list %q = %d err=%v", source, len(found), err)
		}
	}

	finished := now.Add(time.Second)
	for _, candidate := range reloaded.Candidates {
		candidate.Status, candidate.Output, candidate.FinishedAt = experiment.CandidateSucceeded, []byte("piece"), &finished
		if err := store.CompleteCandidate(ctx, candidate); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.SetStatus(ctx, voiced.ID, experiment.StatusReview, &finished); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.Exec(`UPDATE voices SET deleted_at=?, is_default=0 WHERE id='voice-alice'`, formatAt(finished)); err != nil {
		t.Fatalf("a voice comparison held the voice's deletion: %v", err)
	}
	decided := now.Add(2 * time.Second)
	if changed, err := store.Decide(ctx, voiced.ID, "alice", reloaded.Candidates[0].ID, experiment.StatusDecided, experiment.OutcomeWinner, false, false, nil, decided, decided); err != nil || !changed {
		t.Fatalf("decide = %v %v", changed, err)
	}
	if _, err := store.PurgeExpired(ctx, decided.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	purged, _ := store.Get(ctx, voiced.ID)
	if len(purged.InputSnapshot) != 0 || len(purged.Candidates[0].Output) != 0 || len(purged.Candidates[1].Output) != 0 || purged.VoicePromptKey != "opening_greeting" {
		t.Fatalf("the purge kept private content or lost the prompt: %+v", purged)
	}
}

// countingReader is the read pool with a count of the statements sent through it.
type countingReader struct {
	*sql.DB
	queries int
}

func (r *countingReader) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	r.queries++
	return r.DB.QueryContext(ctx, query, args...)
}

func (r *countingReader) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	r.queries++
	return r.DB.QueryRowContext(ctx, query, args...)
}

// The history and the posts page read summaries: three reads whatever the number of comparisons,
// with neither the frozen input nor any candidate's output, while each comparison keeps its
// candidates in display order and its verdict badges.
func TestListReadsSummariesInThreeQueries(t *testing.T) {
	writerStore, handle := testStore(t)
	reader := &countingReader{DB: handle.Reader}
	store := experimentstore.NewWithReader(handle.Writer, reader)
	ctx := context.Background()
	if found, err := store.List(ctx, "alice", "", ""); err != nil || len(found) != 0 || reader.queries != 1 {
		t.Fatalf("an empty history = %d rows, %d reads, err=%v", len(found), reader.queries, err)
	}
	now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	finished := now.Add(time.Minute)
	for i := range 4 {
		found := sample(fmt.Sprintf("exp-%d", i), "alice", "post-a", now.Add(time.Duration(i)*time.Second))
		// Observe comparisons, so four of them may be unresolved on one post at once.
		found.Origin, found.Stage, found.TargetLanguage = experiment.OriginLab, experiment.StageObserve, nil
		if err := writerStore.Create(ctx, found); err != nil {
			t.Fatal(err)
		}
		for _, candidate := range found.Candidates {
			candidate.Status, candidate.Output, candidate.FinishedAt = experiment.CandidateSucceeded, []byte(`{"title":"private"}`), &finished
			if err := writerStore.CompleteCandidate(ctx, candidate); err != nil {
				t.Fatal(err)
			}
		}
		if err := writerStore.SetStatus(ctx, found.ID, experiment.StatusReview, &finished); err != nil {
			t.Fatal(err)
		}
	}
	badges := []experiment.CandidateBadges{{CandidateID: "exp-1-left", Badges: []experiment.Badge{experiment.BadgeOther}, OtherNote: "note"}}
	if changed, err := writerStore.Decide(ctx, "exp-1", "alice", "exp-1-left", experiment.StatusDecided, experiment.OutcomeWinner, false, false, badges, finished, finished.Add(time.Hour)); err != nil || !changed {
		t.Fatalf("decide = %v %v", changed, err)
	}
	reader.queries = 0
	found, err := store.List(ctx, "alice", "", "")
	if err != nil || len(found) != 4 || reader.queries != 3 {
		t.Fatalf("list = %d rows, %d reads, err=%v", len(found), reader.queries, err)
	}
	for i, item := range found {
		if want := fmt.Sprintf("exp-%d", 3-i); item.ID != want {
			t.Fatalf("row %d = %s, want %s newest first", i, item.ID, want)
		}
		if len(item.InputSnapshot) != 0 || len(item.Candidates) != 2 {
			t.Fatalf("%s carried its snapshot or lost a candidate: %+v", item.ID, item)
		}
		if item.Candidates[0].DisplaySide != experiment.SideLeft || item.Candidates[1].DisplaySide != experiment.SideRight {
			t.Fatalf("%s candidates out of display order: %+v", item.ID, item.Candidates)
		}
		for _, candidate := range item.Candidates {
			if len(candidate.Output) != 0 || candidate.Status != experiment.CandidateSucceeded || candidate.ExperimentID != item.ID {
				t.Fatalf("%s candidate = %+v", item.ID, candidate)
			}
		}
	}
	decided := found[2]
	if decided.Status != experiment.StatusDecided || len(decided.Candidates[0].Badges) != 1 || decided.Candidates[0].OtherNote != "note" || len(decided.Candidates[1].Badges) != 0 {
		t.Fatalf("the decided comparison = %+v", decided)
	}
	// The review still reads the whole comparison.
	full, err := store.Get(ctx, "exp-1")
	if err != nil || len(full.InputSnapshot) == 0 || len(full.Candidates[0].Output) == 0 {
		t.Fatalf("get = %+v err=%v", full, err)
	}
}
