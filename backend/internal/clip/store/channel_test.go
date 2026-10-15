package store_test

import (
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
		channel := clip.ChannelInstagram
		moved, err := service.UpdateProject(t.Context(), "alice", ratio, clip.ProjectPatch{Channel: &channel})
		if err != nil {
			t.Fatal(err)
		}
		want := before
		want.Channel = channel
		want.UpdatedAt = moved.UpdatedAt
		if !reflect.DeepEqual(moved, want) {
			t.Fatalf("channel move changed the project: got%+v want%+v", moved, want)
		}
	}
	rows, err := st.ListProjectSummaries(t.Context(), "alice")
	if err != nil || len(rows) != 2 {
		t.Fatal(rows, err)
	}
	for _, row := range rows {
		if row.Channel != clip.ChannelInstagram || row.Ratio != row.ID {
			t.Fatal(row)
		}
	}
	if _, err = database.Writer.ExecContext(t.Context(), `UPDATE clip_projects SET channel='unknown'`); err == nil {
		t.Fatal("channel constraint missing")
	}
}
