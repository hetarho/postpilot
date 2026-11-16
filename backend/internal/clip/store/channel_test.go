package store_test

import (
	"errors"
	"github.com/postpilot/backend/internal/clip"
	"reflect"
	"testing"
)

func TestEarlierProjectsAreNaverClipAndKeepTheirRatio(t *testing.T) {
	service, st, database := setup(t)
	const stamp = "2026-10-01T00:00:00Z"
	for _, ratio := range []string{"horizontal", "square"} {
		// Earlier inserts know no channel. The migrated schema supplies the default.
		_, err := database.Writer.ExecContext(t.Context(), `INSERT INTO clip_projects(id,user_id,title,ratio,target_duration_ms,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`, ratio, "alice", "legacy", ratio, 15000, stamp, stamp)
		if err != nil {
			t.Fatal(err)
		}
		before, err := st.GetProject(t.Context(), "alice", ratio)
		if err != nil {
			t.Fatal(err)
		}
		if before.Channel != clip.ChannelNaver || before.Ratio != ratio {
			t.Fatal(before)
		}
		channel := "instagram_clip"
		moved, err := service.UpdateProject(t.Context(), "alice", ratio, clip.ProjectPatch{Channel: &channel})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(moved, before) {
			t.Fatalf("a channel patch changed the project: got%+v want%+v", moved, before)
		}
	}

	// A channel patch on a completed clip is ignored and reopens none of its confirmed content.
	for _, ratio := range []string{"horizontal", "square"} {
		_, err := database.Writer.ExecContext(t.Context(), `UPDATE clip_projects SET result_id='result-'||id,result_key='result-'||id,result_content_type='video/mp4',result_bytes=1,result_duration_ms=15000,result_created_at=?,edit_plan_revision=1,rendered_plan_revision=1,finalized_at=?,finalized_plan_revision=1,finalized_result_key='result-'||id,source_access_revoked_at=? WHERE id=?`, stamp, stamp, stamp, ratio)
		if err != nil {
			t.Fatal(err)
		}
		before, err := st.GetProject(t.Context(), "alice", ratio)
		if err != nil {
			t.Fatal(err)
		}
		channel, title := clip.ChannelNaver, "forbidden"
		for _, update := range []func() error{
			func() error {
				_, err := service.UpdateProject(t.Context(), "alice", ratio, clip.ProjectPatch{Channel: &channel, Title: &title})
				return err
			},
			func() error {
				_, err := st.UpdateProject(t.Context(), "alice", ratio, clip.ProjectPatch{Channel: &channel, Title: &title}, before.UpdatedAt)
				return err
			},
		} {
			if err := update(); !errors.Is(err, clip.ErrFinalized) {
				t.Fatal("mixed finalized edit accepted", err)
			}
		}
		moved, err := service.UpdateProject(t.Context(), "alice", ratio, clip.ProjectPatch{Channel: &channel})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(moved, before) {
			t.Fatalf("finalized channel patch changed canonical state: %+v %+v", moved, before)
		}
	}
	rows, err := st.ListProjectSummaries(t.Context(), "alice")
	if err != nil || len(rows) != 2 {
		t.Fatal(rows, err)
	}
	for _, row := range rows {
		if row.Channel != clip.ChannelNaver || row.Ratio != row.ID {
			t.Fatal(row)
		}
	}
	if _, err = database.Writer.ExecContext(t.Context(), `UPDATE clip_projects SET channel='unknown'`); err == nil {
		t.Fatal("channel constraint missing")
	}
}
