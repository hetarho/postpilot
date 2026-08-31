package store_test

import (
	"context"
	"errors"
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

// The frozen voice round-trips, and the publishable count follows the lifecycle: queued,
// review and decided-but-unapplied hold the voice; applied and dismissed release it.
func TestStorePersistsVoiceAndCountsPublishableWork(t *testing.T) {
	store, _ := testStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	found := sample("exp-voice", "alice", "", now)
	found.Stage = experiment.StageAnalyze
	found.TargetLanguage = nil
	found.VoiceID = "voice-alice"
	if err := store.Create(ctx, found); err != nil {
		t.Fatal(err)
	}
	reloaded, err := store.Get(ctx, found.ID)
	if err != nil || reloaded.VoiceID != "voice-alice" {
		t.Fatalf("reloaded voice = %q err=%v", reloaded.VoiceID, err)
	}
	count := func(user, voiceID string) int {
		n, err := store.CountPublishableForVoice(ctx, user, voiceID)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	if count("alice", "voice-alice") != 1 || count("bob", "voice-alice") != 0 || count("alice", "voice-bob") != 0 {
		t.Fatalf("queued counts = %d/%d/%d", count("alice", "voice-alice"), count("bob", "voice-alice"), count("alice", "voice-bob"))
	}
	finished := now.Add(time.Second)
	for _, candidate := range found.Candidates {
		candidate.Status = experiment.CandidateSucceeded
		candidate.Output = []byte(`"style"`)
		candidate.FinishedAt = &finished
		if err := store.CompleteCandidate(ctx, candidate); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.SetStatus(ctx, found.ID, experiment.StatusReview, &finished); err != nil {
		t.Fatal(err)
	}
	if count("alice", "voice-alice") != 1 {
		t.Fatal("review does not hold the voice")
	}
	decided := now.Add(2 * time.Second)
	if changed, err := store.Decide(ctx, found.ID, "alice", found.Candidates[0].ID, experiment.StatusDecided, experiment.OutcomeWinner, false, decided, decided.Add(time.Hour)); err != nil || !changed {
		t.Fatalf("decide = %v, %v", changed, err)
	}
	if count("alice", "voice-alice") != 1 {
		t.Fatal("decided-but-unapplied does not hold the voice")
	}
	if err := store.SetApplied(ctx, found.ID, "alice", decided.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if count("alice", "voice-alice") != 0 {
		t.Fatal("applied experiment still holds the voice")
	}
}

func sample(id, user, slug string, at time.Time) experiment.Experiment {
	targetLanguage := experiment.LanguageKorean
	return experiment.Experiment{
		ID: id, UserID: user, PostSlug: slug, Stage: experiment.StageWrite, Status: experiment.StatusQueued,
		TargetLanguage: &targetLanguage,
		InputSnapshot:  []byte(`{"private":true}`), InputHash: "hash", PromptVersion: "v1", CreatedAt: at,
		Candidates: []experiment.Candidate{
			{ID: id + "-left", ExperimentID: id, Model: experiment.ModelRef{ProviderID: "p", ModelID: "a"}, ModelLabel: "A snapshot", DisplaySide: experiment.SideLeft, Status: experiment.CandidatePending},
			{ID: id + "-right", ExperimentID: id, Model: experiment.ModelRef{ProviderID: "p", ModelID: "b"}, ModelLabel: "B snapshot", DisplaySide: experiment.SideRight, Status: experiment.CandidatePending},
		},
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
	observe.Stage = experiment.StageObserve
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
	found := sample("exp-deleted-voice", "alice", "", time.Now().UTC())
	found.Stage = experiment.StageAnalyze
	found.TargetLanguage = nil
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
	changed, err := store.Decide(ctx, found.ID, "alice", left.ID, experiment.StatusDecided, experiment.OutcomeUnpaired, false, decided, decided)
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
	if changed, err := store.Decide(ctx, found.ID, "alice", found.Candidates[0].ID, experiment.StatusDecided, experiment.OutcomeWinner, true, decided, decided.Add(30*24*time.Hour)); err != nil || !changed {
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
