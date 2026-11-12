package store_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/store"
)

// CLIP-130, CLIP-229, CLIP-231, CLIP-232: an untitled clip saves, and the confirmation,
// the posted URL and the checklist ticks round-trip through their own columns.
func TestConfirmationPublicationAndTicksRoundTrip(t *testing.T) {
	service, st, database := setup(t)
	ctx := t.Context()
	p, err := service.CreateProject(ctx, "alice", clip.ProjectInput{Title: "   ", Language: "ko", Ratio: "vertical"})
	if err != nil || p.Title != "" {
		t.Fatal("an empty title was refused or kept its spaces", p.Title, err)
	}
	if _, err := service.CreateProject(ctx, "alice", clip.ProjectInput{Title: strings.Repeat("가", clip.TitleChars+1), Language: "ko", Ratio: "vertical"}); !errors.Is(err, clip.ErrInvalid) {
		t.Fatal("an over-long title was accepted", err)
	}
	empty := " "
	if updated, err := service.UpdateProject(ctx, "alice", p.ID, clip.ProjectPatch{Title: &empty}); err != nil || updated.Title != "" {
		t.Fatal("a title could not be cleared", err)
	}
	read, err := st.GetProject(ctx, "alice", p.ID)
	if err != nil || read.Confirmed != nil || read.Published != nil || !reflect.DeepEqual(read.ChecklistTicks, []string{}) {
		t.Fatal("a new project read confirmed, published or ticked", read.Confirmed, read.Published, read.ChecklistTicks, err)
	}

	ticks := []string{" 영상 30초 이상 ", "영상 30초 이상", "", "지도 첨부"}
	ticked, err := service.UpdateProject(ctx, "alice", p.ID, clip.ProjectPatch{ChecklistTicks: &ticks})
	if err != nil || !reflect.DeepEqual(ticked.ChecklistTicks, []string{"영상 30초 이상", "지도 첨부"}) {
		t.Fatal("ticks were not normalized and saved", ticked.ChecklistTicks, err)
	}

	// Confirmation is written inside the coordinator's transaction.
	if _, err := database.Writer.ExecContext(ctx, `UPDATE clip_projects SET edit_plan_revision=2 WHERE id=?`, p.ID); err != nil {
		t.Fatal(err)
	}
	tx, err := database.Writer.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.NewTx(tx).RecordConfirmation(ctx, clip.ConfirmationRequest{UserID: "alice", ProjectID: p.ID, ExpectedPlanRevision: 1}, time.Unix(10, 0)); !errors.Is(err, clip.ErrPlanConflict) {
		t.Fatal("a stale revision was confirmed", err)
	}
	confirmed, err := store.NewTx(tx).RecordConfirmation(ctx, clip.ConfirmationRequest{UserID: "alice", ProjectID: p.ID, ExpectedPlanRevision: 2}, time.Unix(10, 0))
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if confirmed.Confirmed == nil || confirmed.Confirmed.PlanRevision != 2 || !confirmed.Confirmed.At.Equal(time.Unix(10, 0)) || !clip.Confirmed(confirmed) {
		t.Fatal("the confirmation did not round-trip", confirmed.Confirmed)
	}

	// A published URL needs a finalized project, by the query and by the CHECK.
	tx, _ = database.Writer.BeginTx(ctx, nil)
	if _, err := store.NewTx(tx).RecordPublishedURL(ctx, "alice", p.ID, "https://naver.me/a", time.Unix(11, 0)); !errors.Is(err, clip.ErrNotFound) {
		t.Fatal("an unfinalized project saved a URL", err)
	}
	_ = tx.Rollback()
	if _, err := database.Writer.ExecContext(ctx, `UPDATE clip_projects SET published_url='https://naver.me/a', published_at='x' WHERE id=?`, p.ID); err == nil {
		t.Fatal("the schema accepted a URL on an unfinalized project")
	}
	const at = "2026-11-12T00:00:00Z"
	if _, err := database.Writer.ExecContext(ctx, `UPDATE clip_projects SET rendered_plan_revision=2, result_key='clip-results/r.mp4', result_id='r', result_content_type='video/mp4', result_bytes=1, result_duration_ms=15000, result_created_at=? WHERE id=?`, at, p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Writer.ExecContext(ctx, `UPDATE clip_projects SET finalized_at=?, finalized_plan_revision=2, finalized_result_key=result_key, source_access_revoked_at=? WHERE id=?`, at, at, p.ID); err != nil {
		t.Fatal(err)
	}
	tx, _ = database.Writer.BeginTx(ctx, nil)
	published, err := store.NewTx(tx).RecordPublishedURL(ctx, "alice", p.ID, "https://naver.me/a", time.Unix(11, 0))
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if published.Published == nil || published.Published.URL != "https://naver.me/a" || !published.Published.At.Equal(time.Unix(11, 0)) || published.Confirmed == nil {
		t.Fatal("the URL did not round-trip", published.Published)
	}

	// Ticks stay writable after finalization; the confirmation is frozen with it.
	again := []string{"지도 첨부"}
	if after, err := service.UpdateProject(ctx, "alice", p.ID, clip.ProjectPatch{ChecklistTicks: &again}); err != nil || !reflect.DeepEqual(after.ChecklistTicks, again) {
		t.Fatal("a finalized project refused its ticks", err)
	}
	title := "새 제목"
	if _, err := service.UpdateProject(ctx, "alice", p.ID, clip.ProjectPatch{Title: &title, ChecklistTicks: &again}); !errors.Is(err, clip.ErrFinalized) {
		t.Fatal("ticks opened a finalized project's title", err)
	}
	if _, err := database.Writer.ExecContext(ctx, `UPDATE clip_projects SET confirmed_plan_revision=5 WHERE id=?`, p.ID); err == nil {
		t.Fatal("a finalized project's confirmation was rewritten")
	}

	// A directory row carries the confirmation and the URL without the plan.
	rows, err := st.ListProjectSummaries(ctx, "alice")
	if err != nil || len(rows) != 1 || rows[0].Confirmed == nil || rows[0].Published == nil {
		t.Fatal("the directory row lost the confirmation or the URL", err)
	}
}
