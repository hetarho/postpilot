package store_test

import (
	"errors"
	"github.com/postpilot/backend/internal/clip"
	"testing"
)

func TestEarlierProjectsKeepTheirRatio(t *testing.T) {
	service, st, database := setup(t)
	const stamp = "2026-10-01T00:00:00Z"
	for _, ratio := range []string{"horizontal", "square"} {
		_, err := database.Writer.ExecContext(t.Context(), `INSERT INTO clip_projects(id,user_id,title,ratio,target_duration_ms,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`, ratio, "alice", "legacy", ratio, 15000, stamp, stamp)
		if err != nil {
			t.Fatal(err)
		}
		before, err := st.GetProject(t.Context(), "alice", ratio)
		if err != nil {
			t.Fatal(err)
		}
		if before.Ratio != ratio {
			t.Fatal(before)
		}
	}

	// A completed clip refuses a patch mixing its advisory ticks with canonical content.
	for _, ratio := range []string{"horizontal", "square"} {
		_, err := database.Writer.ExecContext(t.Context(), `UPDATE clip_projects SET result_id='result-'||id,result_key='result-'||id,result_content_type='video/mp4',result_bytes=1,result_duration_ms=15000,result_created_at=?,edit_plan_revision=1,rendered_plan_revision=1,finalized_at=?,finalized_plan_revision=1,finalized_result_key='result-'||id,source_access_revoked_at=? WHERE id=?`, stamp, stamp, stamp, ratio)
		if err != nil {
			t.Fatal(err)
		}
		before, err := st.GetProject(t.Context(), "alice", ratio)
		if err != nil {
			t.Fatal(err)
		}
		ticks, title := []string{"line"}, "forbidden"
		for _, update := range []func() error{
			func() error {
				_, err := service.UpdateProject(t.Context(), "alice", ratio, clip.ProjectPatch{ChecklistTicks: &ticks, Title: &title})
				return err
			},
			func() error {
				_, err := st.UpdateProject(t.Context(), "alice", ratio, clip.ProjectPatch{ChecklistTicks: &ticks, Title: &title}, before.UpdatedAt)
				return err
			},
		} {
			if err := update(); !errors.Is(err, clip.ErrFinalized) {
				t.Fatal("mixed finalized edit accepted", err)
			}
		}
	}
	rows, err := st.ListProjectSummaries(t.Context(), "alice")
	if err != nil || len(rows) != 2 {
		t.Fatal(rows, err)
	}
	for _, row := range rows {
		if row.Ratio != row.ID {
			t.Fatal(row)
		}
	}
}
