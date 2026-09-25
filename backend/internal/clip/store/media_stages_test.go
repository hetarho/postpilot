package store_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	clipapp "github.com/postpilot/backend/internal/clip/app"
	"github.com/postpilot/backend/internal/clip/store"
	"github.com/postpilot/backend/internal/platform/db"
)

func mediaFixture(t *testing.T, limits clip.MediaStageLimits) (*clipapp.MediaStages, *store.Store, clip.MediaStageInput, *time.Time, *db.DB) {
	t.Helper()
	s, st, d := setup(t)
	_, p := create(t, s)
	now := time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)
	_, err := d.Writer.Exec(`INSERT INTO generation_jobs(id,user_id,clip_project_id,kind,status,created_at,updated_at) VALUES('media-job','alice',?,'render_clip','running',?,?)`, p.ID, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	if err != nil {
		t.Fatal(err)
	}
	queue, err := clipapp.NewMediaStages(st, limits, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	in := clip.MediaStageInput{ID: "stage", ParentJobID: "media-job", UserID: "alice", ProjectID: p.ID, ExpectedRevision: p.EditPlanRevision, Operation: clip.MediaRender, ContractVersion: 1, RendererVersion: clip.MediaRendererVersion, AssetVersion: clip.MediaAssetVersion, Payload: `{"plan":1}`, Limits: limits}
	in.InputDigest = clip.MediaPayloadDigest(in.Payload)
	return queue, st, in, &now, d
}

func mediaProfile() clip.MediaWorkerProfile {
	return clip.MediaWorkerProfile{WorkerID: "worker-1", Operation: clip.MediaRender, ContractVersion: 1, RendererVersion: clip.MediaRendererVersion, AssetVersion: clip.MediaAssetVersion, Profile: "cpu", RuntimeManifest: `{"ffmpeg":"test-v1"}`}
}

func TestMediaStageCreationAndCompatibility(t *testing.T) {
	q, st, in, now, _ := mediaFixture(t, clip.DefaultMediaStageLimits(clip.Environment{}))
	ctx := context.Background()
	first, err := q.Create(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	*now = now.Add(time.Second)
	again, err := q.Create(ctx, in)
	if err != nil || first != again {
		t.Fatalf("duplicate changed frozen work: %+v %v", again, err)
	}
	for _, mutate := range []func(*clip.MediaStageInput){
		func(x *clip.MediaStageInput) {
			x.Payload = `{"plan":2}`
			x.InputDigest = clip.MediaPayloadDigest(x.Payload)
		},
		func(x *clip.MediaStageInput) { x.ParentJobID = "other" },
		func(x *clip.MediaStageInput) { x.UserID = "bob" },
		func(x *clip.MediaStageInput) { x.ProjectID = "other" },
		func(x *clip.MediaStageInput) { x.ExpectedRevision++ },
		func(x *clip.MediaStageInput) { x.Operation = clip.MediaPrepare },
		func(x *clip.MediaStageInput) { x.RendererVersion = "cpu-v2" },
	} {
		other := in
		mutate(&other)
		if _, err := q.Create(ctx, other); err == nil {
			t.Fatalf("mutable stage accepted: %+v", other)
		}
	}
	for _, mutate := range []func(*clip.MediaWorkerProfile){
		func(x *clip.MediaWorkerProfile) { x.ContractVersion++ },
		func(x *clip.MediaWorkerProfile) { x.AssetVersion = "other" },
		func(x *clip.MediaWorkerProfile) { x.RendererVersion = "other" },
		func(x *clip.MediaWorkerProfile) { x.Operation = clip.MediaPrepare },
	} {
		p := mediaProfile()
		mutate(&p)
		if got, err := q.Claim(ctx, p); err != nil || got != nil {
			t.Fatalf("incompatible claim: %+v %v", got, err)
		}
	}
	got, err := st.GetMediaStage(ctx, in.ID)
	if err != nil || got.AttemptCount != 0 || got.MediaStageInput != first.MediaStageInput {
		t.Fatalf("refusal mutated stage: %+v %v", got, err)
	}
}

func TestMediaLeaseConcurrentClaimsReclaimAndImmutableReceipt(t *testing.T) {
	q, st, in, now, d := mediaFixture(t, clip.DefaultMediaStageLimits(clip.Environment{}))
	ctx := context.Background()
	if _, err := q.Create(ctx, in); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	leases := make(chan *clip.MediaLease, 12)
	for i := range 12 {
		wg.Go(func() {
			p := mediaProfile()
			p.WorkerID = fmt.Sprintf("worker-%d", i)
			lease, err := q.Claim(ctx, p)
			if err != nil {
				t.Error(err)
			}
			if lease != nil {
				leases <- lease
			}
		})
	}
	wg.Wait()
	close(leases)
	if len(leases) != 1 {
		t.Fatalf("claim winners: %d", len(leases))
	}
	first := <-leases
	var hash, profile, manifest string
	var count int
	if err := d.Reader.QueryRow(`SELECT token_hash, selected_profile, runtime_manifest FROM clip_media_attempts WHERE id=?`, first.Credentials.AttemptID).Scan(&hash, &profile, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(hash) != 64 || hash == first.Credentials.Token || profile != "cpu" || manifest != first.Profile.RuntimeManifest {
		t.Fatal("unsafe or incomplete persisted attempt")
	}
	if err := d.Reader.QueryRow(`SELECT count(*) FROM clip_media_attempts`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("orphan competing attempts: %d %v", count, err)
	}
	if first.Credentials.Token == "" || first.ExpiresAt.Sub(*now) != time.Minute {
		t.Fatal("missing finite opaque lease")
	}
	artifact := clip.MediaArtifactReservation{Slot: "result", ObjectKey: "clip-media/first/result", ContentType: "video/mp4", MaxBytes: 100}
	if err := q.ReserveOutput(ctx, first.Credentials, artifact); err != nil {
		t.Fatal(err)
	}
	if err := q.ReserveOutput(ctx, first.Credentials, artifact); err != nil {
		t.Fatal("identical reservation", err)
	}
	*now = now.Add(15 * time.Second)
	end, err := q.Heartbeat(ctx, first.Credentials, 250)
	if err != nil || !end.Equal(now.Add(time.Minute)) {
		t.Fatalf("renew: %v %v", end, err)
	}
	*now = end
	if _, err := q.Heartbeat(ctx, first.Credentials, 300); !errors.Is(err, clip.ErrMediaLeaseLost) {
		t.Fatalf("revived expired lease: %v", err)
	}
	second, err := q.Claim(ctx, mediaProfile())
	if err != nil || second == nil || second.Stage.AttemptCount != 2 || second.Credentials.Token == first.Credentials.Token || second.Credentials.AttemptID == first.Credentials.AttemptID {
		t.Fatalf("reclaim: %+v %v", second, err)
	}
	for _, a := range []clip.MediaLeaseCredentials{first.Credentials, {StageID: second.Credentials.StageID, AttemptID: second.Credentials.AttemptID, WorkerID: second.Credentials.WorkerID, Token: first.Credentials.Token}} {
		if _, err := q.Heartbeat(ctx, a, 400); !errors.Is(err, clip.ErrMediaLeaseLost) {
			t.Fatalf("stale progress: %v", err)
		}
		if err := q.ReserveOutput(ctx, a, artifact); !errors.Is(err, clip.ErrMediaLeaseLost) {
			t.Fatalf("stale upload: %v", err)
		}
		if _, err := q.Complete(ctx, a, `{"result":"old"}`); !errors.Is(err, clip.ErrMediaLeaseLost) {
			t.Fatalf("stale result: %v", err)
		}
	}
	result := `{"result":"accepted"}`
	accepted, err := q.Complete(ctx, second.Credentials, result)
	if err != nil || accepted.State != clip.MediaSucceeded {
		t.Fatalf("complete: %+v %v", accepted, err)
	}
	*now = now.Add(3 * time.Hour)
	duplicate, err := q.Complete(ctx, second.Credentials, result)
	if err != nil || duplicate != accepted {
		t.Fatalf("lost reply not recoverable: %+v %v", duplicate, err)
	}
	if _, err := q.Complete(ctx, second.Credentials, `{"result":"replacement"}`); !errors.Is(err, clip.ErrMediaConflict) {
		t.Fatalf("conflicting duplicate: %v", err)
	}
	if lease, err := q.Claim(ctx, mediaProfile()); err != nil || lease != nil {
		t.Fatalf("completed stage reclaimed: %+v %v", lease, err)
	}
	got, err := st.GetMediaStage(ctx, in.ID)
	if err != nil || got.AcceptedResult != result {
		t.Fatalf("receipt changed: %+v %v", got, err)
	}
}

func TestMediaStageBudgetsAndCompletionExpiryRace(t *testing.T) {
	limits := clip.MediaStageLimits{LeaseTTL: time.Second, WaitTimeout: 2 * time.Second, StageTimeout: 3 * time.Second, MaxAttempts: 2}
	q, st, in, now, _ := mediaFixture(t, limits)
	ctx := context.Background()
	if _, err := q.Create(ctx, in); err != nil {
		t.Fatal(err)
	}
	first, err := q.Claim(ctx, mediaProfile())
	if err != nil || first == nil {
		t.Fatal(err)
	}
	*now = first.ExpiresAt
	var wg sync.WaitGroup
	wg.Go(func() {
		if _, err := q.Complete(ctx, first.Credentials, `{}`); !errors.Is(err, clip.ErrMediaLeaseLost) {
			t.Errorf("expiry accepted result: %v", err)
		}
	})
	var second *clip.MediaLease
	wg.Go(func() {
		var err error
		second, err = q.Claim(ctx, mediaProfile())
		if err != nil {
			t.Error(err)
		}
	})
	wg.Wait()
	if second == nil || second.Stage.AttemptCount != 2 {
		t.Fatal("replacement was lost")
	}
	*now = second.ExpiresAt
	if third, err := q.Claim(ctx, mediaProfile()); err != nil || third != nil {
		t.Fatalf("attempt budget exceeded: %+v %v", third, err)
	}
	got, err := st.GetMediaStage(ctx, in.ID)
	if err != nil || got.AcceptedResult != "" {
		t.Fatal("expired completion published", err)
	}
}

func TestMediaStageWaitDeadlineAndClampedRenewal(t *testing.T) {
	for _, wait := range []bool{true, false} {
		t.Run(fmt.Sprint(wait), func(t *testing.T) {
			limits := clip.MediaStageLimits{LeaseTTL: time.Second, WaitTimeout: 2 * time.Second, StageTimeout: 3 * time.Second, MaxAttempts: 5}
			q, _, in, now, _ := mediaFixture(t, limits)
			ctx := context.Background()
			if _, err := q.Create(ctx, in); err != nil {
				t.Fatal(err)
			}
			if wait {
				*now = now.Add(limits.WaitTimeout)
				if got, err := q.Claim(ctx, mediaProfile()); err != nil || got != nil {
					t.Fatal("expired initial wait claimed", err)
				}
				return
			}
			deadline := now.Add(limits.StageTimeout)
			lease, err := q.Claim(ctx, mediaProfile())
			if err != nil || lease == nil {
				t.Fatal(err)
			}
			for range 3 {
				*now = now.Add(750 * time.Millisecond)
				end, err := q.Heartbeat(ctx, lease.Credentials, 100)
				if err != nil || end.After(deadline) {
					t.Fatalf("renew beyond stage lifetime: %v %v", end, err)
				}
			}
			*now = deadline
			if _, err := q.Heartbeat(ctx, lease.Credentials, 100); !errors.Is(err, clip.ErrMediaLeaseLost) {
				t.Fatal(err)
			}
			if got, err := q.Claim(ctx, mediaProfile()); err != nil || got != nil {
				t.Fatal("lifetime exceeded", err)
			}
		})
	}
}
