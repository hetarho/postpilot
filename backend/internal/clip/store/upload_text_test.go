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
