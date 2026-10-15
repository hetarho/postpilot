package store_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	clipapp "github.com/postpilot/backend/internal/clip/app"
)

func TestSourceImportPreservesVerifiedOriginalsAndReplaysWithoutRenewal(t *testing.T) {
	projects, store, database := setup(t)
	_, p := create(t, projects)
	now := time.Now().UTC()
	objects := fakeSources()
	service := clipapp.NewSourceService(store, objects, clip.DefaultSourceLimits(clip.Environment{SourceBatchTTL: time.Hour, PutTTL: 10 * time.Minute}), func() time.Time { return now })
	upload, e := service.Create(t.Context(), "alice", p.ID, manifest(3))
	if e != nil {
		t.Fatal(e)
	}
	objects.upload(upload.Batch)
	for _, i := range []int{0, 2} {
		if _, e = service.ConfirmImport(t.Context(), "alice", upload.Batch.ID, upload.Batch.Sources[i].ID); e != nil {
			t.Fatal(e)
		}
	}
	if _, e = database.Writer.Exec(`UPDATE clip_source_leases SET retain_original_audio=1,position=5 WHERE canonical_id=?`, upload.Batch.Sources[0].ID); e != nil {
		t.Fatal(e)
	}
	before, e := store.GetSourceBatch(t.Context(), "alice", upload.Batch.ID)
	if e != nil {
		t.Fatal(e)
	}
	expected := []clip.SourceLease{}
	for _, v := range before.Sources {
		if v.State == "ready" {
			expected = append(expected, v)
		}
	}
	result, e := service.CompleteImport(t.Context(), "alice", p.ID, upload.Batch.ID)
	if e != nil || result == nil || result.ID == upload.Batch.ID || !result.Current || result.State != "ready" || !reflect.DeepEqual(result.Sources, expected) {
		t.Fatal(result, expected, e)
	}
	if result.ID != clip.CompletedSourceImportID("alice", p.ID, upload.Batch.ID) {
		t.Fatal("unstable completion receipt")
	}
	old, e := store.GetSourceBatch(t.Context(), "alice", upload.Batch.ID)
	if e != nil || old.State != "cleanup_pending" || len(old.Sources) != 1 || old.Sources[0].ID != upload.Batch.Sources[1].ID || !old.Sources[0].CleanupPending {
		t.Fatal(old, e)
	}
	for _, v := range expected {
		if _, ok := objects.info[v.Key]; !ok {
			t.Fatal("verified object deleted", v.ID)
		}
		if _, e = service.Playback(t.Context(), "alice", p.ID, v.ID, v.Fingerprint); e != nil {
			t.Fatal("moved original cannot play", e)
		}
	}
	now = now.Add(time.Minute)
	replay, e := service.CompleteImport(t.Context(), "alice", p.ID, upload.Batch.ID)
	if e != nil || !reflect.DeepEqual(replay, result) {
		t.Fatal("replay changed identity or retention", replay, e)
	}
	now = now.Add(11 * time.Minute)
	if e = service.Sweep(t.Context()); e != nil {
		t.Fatal(e)
	}
	for _, v := range expected {
		if _, ok := objects.info[v.Key]; !ok {
			t.Fatal("old batch sweep deleted moved original", v.ID)
		}
	}
	replay, e = service.CompleteImport(t.Context(), "alice", p.ID, upload.Batch.ID)
	if e != nil || !reflect.DeepEqual(replay, result) {
		t.Fatal("old cleanup lost receipt", replay, e)
	}
	if _, e = service.CompleteImport(t.Context(), "bob", p.ID, upload.Batch.ID); !errors.Is(e, clip.ErrNotFound) {
		t.Fatal("foreign completion", e)
	}
}

func TestSourceImportCompletionGuardsAndRollback(t *testing.T) {
	for _, scenario := range []string{"active", "past job", "material", "plan", "revoked", "finalized", "stale", "rollback"} {
		t.Run(scenario, func(t *testing.T) {
			projects, store, database := setup(t)
			_, p := create(t, projects)
			now := time.Now().UTC()
			at := now.Format(time.RFC3339Nano)
			objects := fakeSources()
			service := clipapp.NewSourceService(store, objects, clip.DefaultSourceLimits(clip.Environment{SourceBatchTTL: time.Hour, PutTTL: 10 * time.Minute}), func() time.Time { return now })
			upload, e := service.Create(t.Context(), "alice", p.ID, manifest(2))
			if e != nil {
				t.Fatal(e)
			}
			objects.upload(upload.Batch)
			if _, e = service.ConfirmImport(t.Context(), "alice", upload.Batch.ID, upload.Batch.Sources[0].ID); e != nil {
				t.Fatal(e)
			}
			switch scenario {
			case "active", "past job":
				status := "queued"
				if scenario == "past job" {
					status = "done"
				}
				_, e = database.Writer.Exec(`INSERT INTO generation_jobs(id,user_id,clip_project_id,kind,status,created_at,updated_at) VALUES('import-job','alice',?,'generate_clip',?,?,?)`, p.ID, status, at, at)
			case "material":
				_, e = database.Writer.Exec(`UPDATE clip_projects SET analysis_json='[]' WHERE id=?`, p.ID)
			case "plan":
				_, e = database.Writer.Exec(`UPDATE clip_projects SET edit_plan_revision=1 WHERE id=?`, p.ID)
			case "revoked":
				_, e = database.Writer.Exec(`UPDATE clip_projects SET source_access_revoked_at=? WHERE id=?`, at, p.ID)
			case "finalized":
				_, e = database.Writer.Exec(`UPDATE clip_projects SET result_id='result',result_key='result',result_content_type='video/mp4',result_bytes=1,result_duration_ms=15000,result_created_at=?,edit_plan_revision=1,rendered_plan_revision=1,finalized_at=?,finalized_plan_revision=1,finalized_result_key='result',source_access_revoked_at=? WHERE id=?`, at, at, at, p.ID)
			case "stale":
				_, e = service.Create(t.Context(), "alice", p.ID, manifest(1))
			case "rollback":
				_, e = database.Writer.Exec(`CREATE TRIGGER reject_import_move BEFORE UPDATE OF batch_id ON clip_source_leases BEGIN SELECT RAISE(ABORT,'injected move failure'); END`)
			}
			if e != nil {
				t.Fatal(e)
			}
			if _, e = service.CompleteImport(t.Context(), "alice", p.ID, upload.Batch.ID); e == nil {
				t.Fatal("unsafe completion admitted", scenario)
			}
			old, e := store.GetSourceBatch(t.Context(), "alice", upload.Batch.ID)
			if e != nil || old.State != "uploading" || len(old.Sources) != 2 || old.Sources[0].Key != upload.Batch.Sources[0].Key {
				t.Fatal("refusal changed original batch", old, e)
			}
			if len(objects.deleted) > 0 {
				t.Fatal("refusal deleted objects", objects.deleted)
			}
		})
	}
}

func TestSourceImportAllReadyAndEmptyAreExplicit(t *testing.T) {
	for _, ready := range []bool{false, true} {
		t.Run(map[bool]string{false: "empty", true: "ready"}[ready], func(t *testing.T) {
			projects, store, _ := setup(t)
			_, p := create(t, projects)
			objects := fakeSources()
			service := clipapp.NewSourceService(store, objects, clip.DefaultSourceLimits(clip.Environment{SourceBatchTTL: time.Hour, PutTTL: 10 * time.Minute}))
			upload, e := service.Create(t.Context(), "alice", p.ID, manifest(1))
			if e != nil {
				t.Fatal(e)
			}
			if ready {
				objects.upload(upload.Batch)
				if _, e = service.ConfirmImport(t.Context(), "alice", upload.Batch.ID, upload.Batch.Sources[0].ID); e != nil {
					t.Fatal(e)
				}
			}
			out, e := service.CompleteImport(t.Context(), "alice", p.ID, upload.Batch.ID)
			if e != nil {
				t.Fatal(e)
			}
			if ready && (out == nil || out.ID != upload.Batch.ID || out.State != "ready") {
				t.Fatal(out)
			}
			if !ready && out != nil {
				t.Fatal(out)
			}
			again, e := service.CompleteImport(t.Context(), "alice", p.ID, upload.Batch.ID)
			if e != nil || !reflect.DeepEqual(again, out) {
				t.Fatal(again, e)
			}
			if ready && len(objects.deleted) != 0 {
				t.Fatal("all-ready import deleted an object")
			}
		})
	}
}

func TestSourceImportInvalidObjectDoesNotDiscardVerifiedSiblings(t *testing.T) {
	for _, preserve := range []bool{false, true} {
		t.Run(map[bool]string{false: "ordinary", true: "import"}[preserve], func(t *testing.T) {
			projects, store, _ := setup(t)
			_, p := create(t, projects)
			objects := fakeSources()
			service := clipapp.NewSourceService(store, objects, clip.DefaultSourceLimits(clip.Environment{SourceBatchTTL: time.Hour, PutTTL: 10 * time.Minute}))
			upload, e := service.Create(t.Context(), "alice", p.ID, manifest(2))
			if e != nil {
				t.Fatal(e)
			}
			objects.upload(upload.Batch)
			if _, e = service.Confirm(t.Context(), "alice", upload.Batch.ID, upload.Batch.Sources[0].ID); e != nil {
				t.Fatal(e)
			}
			objects.info[upload.Batch.Sources[1].Key] = clip.SourceObjectInfo{Bytes: 1, ContentType: "video/mp4"}
			if preserve {
				_, e = service.ConfirmImport(t.Context(), "alice", upload.Batch.ID, upload.Batch.Sources[1].ID)
			} else {
				_, e = service.Confirm(t.Context(), "alice", upload.Batch.ID, upload.Batch.Sources[1].ID)
			}
			if !errors.Is(e, clip.ErrInvalid) {
				t.Fatal(e)
			}
			b, e := store.GetSourceBatch(t.Context(), "alice", upload.Batch.ID)
			if e != nil {
				t.Fatal(e)
			}
			if preserve {
				if b.State != "uploading" || b.Sources[0].CleanupPending || !strings.HasPrefix(b.Sources[0].Key, clip.SourcePrefix) {
					t.Fatal("confirmed sibling discarded", b)
				}
				done, e := service.CompleteImport(t.Context(), "alice", p.ID, b.ID)
				if e != nil || done == nil || len(done.Sources) != 1 || done.Sources[0].ID != upload.Batch.Sources[0].ID {
					t.Fatal(done, e)
				}
			} else if b.State != "cleanup_pending" {
				t.Fatal("ordinary invalid confirmation changed its contract", b)
			}
		})
	}
}

func TestSourceImportCompletedBatchRunsTheExistingQuotedGeneration(t *testing.T) {
	h := generationSetup(t)
	declared := manifest(3)
	declared[0].DurationMS = 60000
	declared[1].DurationMS = 15000
	upload, e := h.sources.Create(t.Context(), "alice", h.project.ID, declared)
	if e != nil {
		t.Fatal(e)
	}
	h.objects.upload(upload.Batch)
	for _, source := range upload.Batch.Sources[:2] {
		if _, e = h.sources.ConfirmImport(t.Context(), "alice", upload.Batch.ID, source.ID); e != nil {
			t.Fatal(e)
		}
	}
	completed, e := h.sources.CompleteImport(t.Context(), "alice", h.project.ID, upload.Batch.ID)
	if e != nil || completed == nil || len(completed.Sources) != 2 {
		t.Fatal(completed, e)
	}
	h.batch = *completed
	h.start(t)
	if e = h.run(t); e != nil {
		t.Fatal("moved originals failed the ordinary quote, binding or processing path", e)
	}
	p, e := h.store.GetProject(t.Context(), "alice", h.project.ID)
	if e != nil || p.EditPlanRevision != 1 || p.Analysis == "" {
		t.Fatal(p, e)
	}
}

func TestSourceImportAllReadyReceiptDoesNotResetWorkStartedAfterCompletion(t *testing.T) {
	projects, store, database := setup(t)
	_, p := create(t, projects)
	objects := fakeSources()
	service := clipapp.NewSourceService(store, objects, clip.DefaultSourceLimits(clip.Environment{SourceBatchTTL: time.Hour, PutTTL: 10 * time.Minute}))
	upload, e := service.Create(t.Context(), "alice", p.ID, manifest(1))
	if e != nil {
		t.Fatal(e)
	}
	objects.upload(upload.Batch)
	if _, e = service.ConfirmImport(t.Context(), "alice", upload.Batch.ID, upload.Batch.Sources[0].ID); e != nil {
		t.Fatal(e)
	}
	original, e := service.CompleteImport(t.Context(), "alice", p.ID, upload.Batch.ID)
	if e != nil {
		t.Fatal(e)
	}
	at := time.Now().UTC().Format(time.RFC3339Nano)
	if _, e = database.Writer.Exec(`INSERT INTO generation_jobs(id,user_id,clip_project_id,kind,status,created_at,updated_at) VALUES('completed-import-job','alice',?,'generate_clip','done',?,?)`, p.ID, at, at); e != nil {
		t.Fatal(e)
	}
	replay, e := service.CompleteImport(t.Context(), "alice", p.ID, upload.Batch.ID)
	if e != nil || !reflect.DeepEqual(replay, original) {
		t.Fatal("receipt revalidated or changed completed work", replay, e)
	}
}
