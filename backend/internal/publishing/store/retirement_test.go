package store

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/postpilot/backend/internal/platform/db"
)

func TestRetirementSnapshotSupportsAnEmptyInstallation(t *testing.T) {
	handle, err := db.Open(filepath.Join(t.TempDir(), "empty.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { handle.Close() })
	if err := db.Migrate(context.Background(), handle.Writer); err != nil {
		t.Fatal(err)
	}
	snapshot, err := New(handle.Writer, handle.Reader).RetirementSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.CutoffAt == "" || len(snapshot.Agents) != 0 || len(snapshot.Jobs) != 0 || snapshot.Pairings != 0 || snapshot.Reservations != 0 || snapshot.Assets != 0 {
		t.Fatalf("snapshot=%+v", snapshot)
	}
}

func TestRetirementCleanupDeletesAllOwnedRowsInForeignKeyOrder(t *testing.T) {
	handle, err := db.Open(filepath.Join(t.TempDir(), "cleanup.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { handle.Close() })
	ctx := context.Background()
	if err := db.Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	// The fixture is inserted after the cutoff only because this test starts at the
	// latest schema. Production rows predate migration 0072; dropping only the insert
	// guards here recreates that already-cut-off shape without weakening application code.
	for _, trigger := range []string{
		"publishing_retired_pairing_insert", "publishing_retired_agent_insert",
		"publishing_retired_job_id_insert", "publishing_retired_job_insert", "publishing_retired_asset_insert",
	} {
		if _, err := handle.Writer.ExecContext(ctx, `DROP TRIGGER `+trigger); err != nil {
			t.Fatal(err)
		}
	}
	at := "2026-09-22T00:00:00Z"
	for _, statement := range []string{
		`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash','` + at + `')`,
		`INSERT INTO sessions(token,user_id,expires_at,created_at) VALUES('human','alice','2026-10-22T00:00:00Z','` + at + `')`,
		`INSERT INTO publishing_pairings(code_hash,user_id,label,expires_at,consumed_at,created_at) VALUES('pair','alice','Mac','2026-09-23T00:00:00Z','` + at + `','` + at + `')`,
		`INSERT INTO publishing_agents(id,user_id,token_hash,label,platform,revoked_at,created_at,updated_at) VALUES('agent','alice','token','Mac','naver_blog','` + at + `','` + at + `','` + at + `')`,
		`INSERT INTO publish_job_ids(id,user_id,created_at) VALUES('job','alice','` + at + `')`,
		`INSERT INTO publish_jobs(id,user_id,post_slug,post_created_at,agent_id,platform,status,stage,content_revision,settings_json,created_at,published_at,updated_at) VALUES('job','alice','deleted-source','` + at + `','agent','naver_blog','published','published',1,'{}','` + at + `','` + at + `','` + at + `')`,
		`INSERT INTO publish_assets(job_id,user_id,ordinal,filename,source_filename,staged_key,bytes,created_at) VALUES('job','alice',0,'copy.jpg','source.jpg','publishing/alice/job/copy.jpg',10,'` + at + `')`,
	} {
		if _, err := handle.Writer.ExecContext(ctx, statement); err != nil {
			t.Fatalf("seed %q: %v", statement, err)
		}
	}
	store := New(handle.Writer, handle.Reader)
	keys, err := store.RetirementAssetKeys(ctx)
	if err != nil || !reflect.DeepEqual(keys, []string{"publishing/alice/job/copy.jpg"}) {
		t.Fatalf("keys = %v err=%v", keys, err)
	}
	if err := store.DeleteRetirementRows(ctx); err != nil {
		t.Fatal(err)
	}
	snapshot, err := store.RetirementSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Agents) != 0 || len(snapshot.Jobs) != 0 || snapshot.Pairings != 0 || snapshot.Reservations != 0 || snapshot.Assets != 0 {
		t.Fatalf("publishing rows remain: %+v", snapshot)
	}
	var users, sessions int
	if err := handle.Reader.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE id='alice'`).Scan(&users); err != nil {
		t.Fatal(err)
	}
	if err := handle.Reader.QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions WHERE token='human'`).Scan(&sessions); err != nil {
		t.Fatal(err)
	}
	if users != 1 || sessions != 1 {
		t.Fatalf("unrelated rows changed: users=%d sessions=%d", users, sessions)
	}
	if err := store.DeleteRetirementRows(ctx); err != nil {
		t.Fatalf("repeated empty deletion: %v", err)
	}
}
