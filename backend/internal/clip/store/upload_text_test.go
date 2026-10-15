package store_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
)

func TestUploadTextRowsKeepOwnershipAndOptimisticRevisions(t *testing.T) {
	projects, store, database := setup(t)
	_, p := create(t, projects)
	raw, e := clip.EncodeEditPlan(clip.EditPlan{Ratio: "vertical", DurationMS: 15000, Cuts: []clip.Cut{{ID: "cut", SourceID: "expired", Fingerprint: strings.Repeat("a", 64), EndMS: 15000}}})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = database.Writer.Exec(`UPDATE clip_projects SET edit_plan_json=?,edit_plan_revision=1 WHERE id=?`, raw, p.ID); e != nil {
		t.Fatal(e)
	}
	in := clip.UploadTextSave{UserID: "alice", ProjectID: p.ID, Channel: clip.ChannelNaver, Body: "내가 쓴 문구", Tags: []string{"#방문"}, Edited: true, Now: time.Now()}
	first, e := store.SaveUploadText(t.Context(), in)
	if e != nil || first.Revision != 1 || !first.Edited || first.Tags[0] != "방문" {
		t.Fatal(first, e)
	}
	in.Body = "오래된 편집"
	if _, e = store.SaveUploadText(t.Context(), in); !errors.Is(e, clip.ErrPlanConflict) {
		t.Fatal("stale edit accepted", e)
	}
	in.ExpectedRevision = 1
	in.Body = "나중에 고친 문구"
	second, e := store.SaveUploadText(t.Context(), in)
	if e != nil || second.Revision != 2 {
		t.Fatal(second, e)
	}
	if _, e = store.ListUploadTexts(t.Context(), "bob", p.ID); !errors.Is(e, clip.ErrNotFound) {
		t.Fatal("foreign row disclosed", e)
	}
	in.UserID = "bob"
	in.ExpectedRevision = 2
	if _, e = store.SaveUploadText(t.Context(), in); !errors.Is(e, clip.ErrNotFound) {
		t.Fatal("foreign row changed", e)
	}
	// The database also refuses a mismatched project-owner pair.
	if _, e = database.Writer.Exec(`INSERT INTO clip_upload_texts(project_id,user_id,channel,body,updated_at) VALUES(?,'bob','instagram_clip','foreign',?)`, p.ID, time.Now().UTC().Format(time.RFC3339Nano)); e == nil {
		t.Fatal("ownership FK missing")
	}
	after, e := store.GetProject(t.Context(), "alice", p.ID)
	if e != nil || after.EditPlan != raw || after.EditPlanRevision != 1 {
		t.Fatal("metadata changed video", after, e)
	}
	if _, e = database.Writer.Exec(`DELETE FROM clip_projects WHERE id=?`, p.ID); e != nil {
		t.Fatal(e)
	}
	var count int
	if e = database.Reader.QueryRow(`SELECT COUNT(*) FROM clip_upload_texts WHERE project_id=?`, p.ID).Scan(&count); e != nil || count != 0 {
		t.Fatal("orphan upload text retained", count, e)
	}
}

func TestUploadTextLeavesCheckpointAndFinishedVideoGuardsIntact(t *testing.T) {
	projects, store, database := setup(t)
	_, p := create(t, projects)
	stamp := time.Now().UTC().Format(time.RFC3339Nano)
	raw, e := clip.EncodeEditPlan(clip.EditPlan{Ratio: "vertical", DurationMS: 15000, Cuts: []clip.Cut{{ID: "cut", SourceID: "expired", Fingerprint: strings.Repeat("a", 64), EndMS: 15000}}})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = database.Writer.Exec(`INSERT INTO generation_jobs(id,user_id,clip_project_id,kind,status,created_at,updated_at) VALUES('old','alice',?,'generate_clip','done',?,?)`, p.ID, stamp, stamp); e != nil {
		t.Fatal(e)
	}
	if _, e = database.Writer.Exec(`INSERT INTO clip_attempt_checkpoints(project_id,user_id,job_id,checkpoint_json) VALUES(?,'alice','old','{"kept":"yes"}')`, p.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = database.Writer.Exec(`UPDATE clip_projects SET edit_plan_json=?,result_id='result',result_key='result',edit_plan_revision=1,rendered_plan_revision=1,finalized_at=?,finalized_plan_revision=1,finalized_result_key='result',source_access_revoked_at=? WHERE id=?`, raw, stamp, stamp, p.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = database.Writer.Exec(`INSERT INTO generation_jobs(id,user_id,clip_project_id,kind,status,dispatch_ready,cancellation_policy_version,created_at,updated_at) VALUES('upload','alice',?,'clip_upload_text','queued',0,1,?,?)`, p.ID, stamp, stamp); e != nil {
		t.Fatal("upload text was refused on finished video", e)
	}
	if _, e = store.SaveUploadText(t.Context(), clip.UploadTextSave{UserID: "alice", ProjectID: p.ID, Channel: clip.ChannelNaver, Body: "edited", Edited: true, Now: time.Now()}); !errors.Is(e, clip.ErrBusy) {
		t.Fatal("manual edit raced active job", e)
	}
	if _, e = database.Writer.Exec(`UPDATE generation_jobs SET dispatch_ready=1,status='running' WHERE id='upload'`); e != nil {
		t.Fatal("upload activation refused", e)
	}
	var checkpoint string
	if e = database.Reader.QueryRow(`SELECT checkpoint_json FROM clip_attempt_checkpoints WHERE project_id=?`, p.ID).Scan(&checkpoint); e != nil || checkpoint != `{"kept":"yes"}` {
		t.Fatal("metadata job erased generation evidence", checkpoint, e)
	}
	if _, e = database.Writer.Exec(`UPDATE generation_jobs SET status='done' WHERE id='upload'`); e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"generate_clip", "render_clip", "revise_clip", "speech_clip", "storyline_clip"} {
		if _, e = database.Writer.Exec(`INSERT INTO generation_jobs(id,user_id,clip_project_id,kind,status,created_at,updated_at) VALUES(?,'alice',?,?,'queued',?,?)`, kind, p.ID, kind, stamp, stamp); e == nil {
			t.Fatal("finished video guard admitted", kind)
		}
	}
}
