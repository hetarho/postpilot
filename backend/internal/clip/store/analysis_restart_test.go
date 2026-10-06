package store_test

import (
	"errors"
	"fmt"
	"github.com/postpilot/backend/internal/clip/app"
	"github.com/postpilot/backend/internal/job"
	"testing"
	"time"
)

func acceptPreparationBeforePark(t *testing.T, f *analysisFixture) {
	t.Helper()
	f.upload(t)
	if _, err := f.a.Complete(t.Context(), "alice", f.p.ID); err != nil {
		t.Fatal(err)
	}
	work, err := f.a.Claim(t.Context(), analysisProfile())
	if err != nil || work == nil {
		t.Fatalf("claim=%v err=%v", work, err)
	}
	if err := f.a.CompleteVerification(t.Context(), work.Credentials, safeAnalysisReceipt(t, work)); err != nil {
		t.Fatal(err)
	}
}
func TestBrowserPreparationRestartBeforeFirstPark(t *testing.T) {
	for _, state := range []string{"preparing", "verifying", "accepted"} {
		t.Run(state, func(t *testing.T) {
			f := newAnalysisFixture(t)
			f.begin(t)
			id := f.start(t)
			picked, err := f.h.jobs.PickNextQueued(t.Context(), f.now)
			if err != nil || picked.ID != id {
				t.Fatal(picked, err)
			}
			// Do not Run the handler/forJob: no durable wait exists yet.
			if state == "verifying" {
				f.upload(t)
				if _, err := f.a.Complete(t.Context(), "alice", f.p.ID); err != nil {
					t.Fatal(err)
				}
			}
			if state == "accepted" {
				acceptPreparationBeforePark(t, f)
			}
			if err := f.a.ReconcileStartup(t.Context()); err != nil {
				t.Fatal(err)
			}
			swept, err := f.h.queue.SweepRunning(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if len(f.h.admitter.calls) != 0 || f.h.planner.observe != 0 {
				t.Fatal("recovery dispatched or reserved paid work")
			}
			if swept != 0 {
				t.Fatalf("live %s browser preparation swept before Park: %d", state, swept)
			}
			wait, err := f.h.jobs.Continuation(t.Context(), id)
			expected := job.ContinuationWaiting
			if state == "accepted" {
				expected = job.ContinuationReady
			}
			if err != nil || wait.State != expected || wait.Policy != job.FailOnInterrupt || wait.WaitKey != app.AnalysisPreparationWaitKey(f.p.ID) {
				t.Fatal(wait, err)
			}
			if err := f.a.ReconcileStartup(t.Context()); err != nil {
				t.Fatal(err)
			}
			if state == "accepted" {
				resumed, err := f.h.jobs.PickNextQueued(t.Context(), f.now)
				if err != nil || resumed.ID != id {
					t.Fatal("accepted work did not use ordinary queue claim", resumed, err)
				}
				wait, err = f.h.jobs.Continuation(t.Context(), id)
				if err != nil || wait.State != job.ContinuationClaimed {
					t.Fatal(wait, err)
				}
			}
		})
	}
}
func TestBrowserPreparationRestartNeverReplaysConsumedClaimed(t *testing.T) {
	f := newAnalysisFixture(t)
	f.begin(t)
	id := f.start(t)
	if _, err := f.h.jobs.PickNextQueued(t.Context(), f.now); err != nil {
		t.Fatal(err)
	}
	acceptPreparationBeforePark(t, f)
	key := app.AnalysisPreparationWaitKey(f.p.ID)
	if err := f.h.jobs.Park(t.Context(), id, key, job.FailOnInterrupt, f.now); err != nil {
		t.Fatal(err)
	}
	if _, err := f.h.jobs.Wake(t.Context(), id, key, f.now); err != nil {
		t.Fatal(err)
	}
	if _, err := f.h.jobs.PickNextQueued(t.Context(), f.now); err != nil {
		t.Fatal(err)
	}
	if err := f.h.store.ConsumeAnalysisPreparation(t.Context(), f.p.ID, id, f.now); err != nil {
		t.Fatal(err)
	}
	if err := f.a.ReconcileStartup(t.Context()); err != nil {
		t.Fatal(err)
	}
	wait, err := f.h.jobs.Continuation(t.Context(), id)
	if err != nil || wait.State != job.ContinuationClaimed || wait.Policy != job.FailOnInterrupt {
		t.Fatal("paid continuation changed", wait, err)
	}
	swept, err := f.h.queue.SweepRunning(t.Context())
	if err != nil || swept != 1 {
		t.Fatal("uncertain paid continuation was replayable", swept, err)
	}
}

func TestBrowserPreparationStartupDrainsPagesBeforeSweep(t *testing.T) {
	f := newAnalysisFixture(t)
	f.begin(t)
	id := f.start(t)
	if _, err := f.h.jobs.PickNextQueued(t.Context(), f.now); err != nil {
		t.Fatal(err)
	}
	// Expired abandoned sessions legitimately occupy the first bounded cleanup page.
	for index := 0; index < 100; index++ {
		name := fmt.Sprintf("00000000000000000000000000000000-%03d", index)
		_, err := f.h.db.Writer.Exec(`INSERT INTO clip_analysis_preparations(id,user_id,project_id,batch_id,quote_id,profile_version,expected_revision,metadata_json,manifest_digest,created_at,expires_at,queue_deadline_at,deadline_at)
    SELECT ?,user_id,project_id,batch_id,?,profile_version,expected_revision,json_set(metadata_json,'$.ID',?,'$.QuoteID',?),manifest_digest,created_at,?,queue_deadline_at,deadline_at FROM clip_analysis_preparations WHERE id=?`, name, name, name, name, f.now.Add(-time.Hour).Format(time.RFC3339Nano), f.p.ID)
		if err != nil {
			t.Fatal(err)
		}
		_, err = f.h.db.Writer.Exec(`INSERT INTO clip_analysis_copies(preparation_id,slot,source_id,source_fingerprint,ordinal,offset_ms,duration_ms,width,height,has_audio)
    SELECT ?,slot,source_id,source_fingerprint,ordinal,offset_ms,duration_ms,width,height,has_audio FROM clip_analysis_copies WHERE preparation_id=?`, name, f.p.ID)
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := f.a.ReconcileStartup(t.Context()); err != nil {
		t.Fatal(err)
	}
	swept, err := f.h.queue.SweepRunning(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if swept != 0 {
		t.Fatalf("live preparation %s beyond first recovery page swept: %d", id, swept)
	}
}

func TestBrowserPreparationRestartNeverReplaysConsumedMissingWait(t *testing.T) {
	f := newAnalysisFixture(t)
	f.begin(t)
	id := f.start(t)
	if _, err := f.h.jobs.PickNextQueued(t.Context(), f.now); err != nil {
		t.Fatal(err)
	}
	acceptPreparationBeforePark(t, f)
	if err := f.h.store.ConsumeAnalysisPreparation(t.Context(), f.p.ID, id, f.now); err != nil {
		t.Fatal(err)
	}
	if err := f.a.ReconcileStartup(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := f.h.jobs.Continuation(t.Context(), id); !errors.Is(err, job.ErrInvalidWait) {
		t.Fatal("consumed work received a new wait", err)
	}
	swept, err := f.h.queue.SweepRunning(t.Context())
	if err != nil || swept != 1 {
		t.Fatal("uncertain consumed work was replayable", swept, err)
	}
}

func TestBrowserPreparationRestartNeverResetsAcceptedClaim(t *testing.T) {
	f := newAnalysisFixture(t)
	f.begin(t)
	id := f.start(t)
	if _, err := f.h.jobs.PickNextQueued(t.Context(), f.now); err != nil {
		t.Fatal(err)
	}
	acceptPreparationBeforePark(t, f)
	key := app.AnalysisPreparationWaitKey(f.p.ID)
	if err := f.h.jobs.Park(t.Context(), id, key, job.FailOnInterrupt, f.now); err != nil {
		t.Fatal(err)
	}
	if _, err := f.h.jobs.Wake(t.Context(), id, key, f.now); err != nil {
		t.Fatal(err)
	}
	if _, err := f.h.jobs.PickNextQueued(t.Context(), f.now); err != nil {
		t.Fatal(err)
	}
	// A claim is already an uncertain paid boundary, even before consumption.
	if err := f.a.ReconcileStartup(t.Context()); err != nil {
		t.Fatal(err)
	}
	wait, err := f.h.jobs.Continuation(t.Context(), id)
	if err != nil || wait.State != job.ContinuationClaimed || wait.Policy != job.FailOnInterrupt {
		t.Fatal(wait, err)
	}
	swept, err := f.h.queue.SweepRunning(t.Context())
	if err != nil || swept != 1 {
		t.Fatal("uncertain accepted claim was replayable", swept, err)
	}
}

func TestBrowserPreparationRestartRequiresCurrentParentAndInputs(t *testing.T) {
	for _, change := range []string{"quote", "batch", "session", "revision", "source", "expired"} {
		t.Run(change, func(t *testing.T) {
			f := newAnalysisFixture(t)
			f.begin(t)
			id := f.start(t)
			if _, e := f.h.jobs.PickNextQueued(t.Context(), f.now); e != nil {
				t.Fatal(e)
			}
			var e error
			switch change {
			case "quote":
				_, e = f.h.db.Writer.Exec(`UPDATE generation_jobs SET payload=json_set(payload,'$.Approval.QuoteID','foreign') WHERE id=?`, id)
			case "batch":
				_, e = f.h.db.Writer.Exec(`UPDATE generation_jobs SET payload=json_set(payload,'$.Batch.ID','foreign') WHERE id=?`, id)
			case "session":
				_, e = f.h.db.Writer.Exec(`UPDATE generation_jobs SET payload=json_set(payload,'$.AnalysisPreparationID','foreign') WHERE id=?`, id)
			case "revision":
				_, e = f.h.db.Writer.Exec(`UPDATE clip_projects SET edit_plan_revision=edit_plan_revision+1 WHERE id=?`, f.p.ProjectID)
			case "source":
				_, e = f.h.db.Writer.Exec(`UPDATE clip_source_leases SET fingerprint='foreign' WHERE id=?`, f.p.Sources[0].ID)
			case "expired":
				f.now = f.p.ExpiresAt.Add(time.Second)
			}
			if e != nil {
				t.Fatal(e)
			}
			// Invalid persisted parent identity refuses recovery; changed live inputs
			// expire/retire the preparation. Neither may create a replayable wait.
			_ = f.a.ReconcileStartup(t.Context())
			if _, e = f.h.jobs.Continuation(t.Context(), id); !errors.Is(e, job.ErrInvalidWait) {
				t.Fatal("invalid preparation restored", e)
			}
			if len(f.h.admitter.calls) != 0 || f.h.planner.observe != 0 {
				t.Fatal("recovery dispatched or reserved paid work")
			}
		})
	}
}
