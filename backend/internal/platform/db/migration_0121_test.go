package db

import (
	"context"
	"io/fs"
	"testing"

	"github.com/pressly/goose/v3"
)

func TestMigration0121FreshAndPreRedesignDatabasePreserveContent(t *testing.T) {
	ctx := context.Background()
	for _, version := range []int64{0, 113} {
		t.Run(map[bool]string{true: "fresh", false: "pre-redesign"}[version == 0], func(t *testing.T) {
			handle := openTemp(t)
			if version != 0 {
				sub, err := fs.Sub(migrationsFS, "migrations")
				if err != nil {
					t.Fatal(err)
				}
				provider, err := goose.NewProvider(goose.DialectSQLite3, handle.Writer, sub, goose.WithLogger(goose.NopLogger()))
				if err != nil {
					t.Fatal(err)
				}
				if _, err := provider.UpTo(ctx, version); err != nil {
					t.Fatal(err)
				}
				at := "2026-09-30T00:00:00Z"
				end := "2026-10-30T00:00:00Z"
				mustExec(t, handle.Writer, "INSERT INTO users(id,password_hash,created_at,plan) VALUES('alice','hash',?,'basic')", at)
				mustExec(t, handle.Writer, "INSERT INTO users(id,password_hash,created_at,plan) VALUES('operator','operator-hash',?,'master')", at)
				mustExec(t, handle.Writer, "INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES('voice','alice','Kept',1,?,?)", at, at)
				mustExec(t, handle.Writer, "INSERT INTO posts(slug,user_id,voice_id,created_at,updated_at) VALUES('post','alice','voice',?,?)", at, at)
				mustExec(t, handle.Writer, "INSERT INTO videos(id,post_slug,filename,r2_key,content_type,bytes,duration_ms,width,height,created_at) VALUES('video','post','file.mp4','kept-key','video/mp4',100,15000,100,100,?)", at)
				mustExec(t, handle.Writer, "INSERT INTO subscriptions(user_id,tier,term,anchor_at,term_start,term_end,next_grant_at,status,created_at,updated_at) VALUES('alice','basic','monthly',?,?,?,?,'active',?,?)", at, at, end, end, at, at)
				mustExec(t, handle.Writer, "INSERT INTO credit_lots(id,user_id,kind,granted,remaining,created_at) VALUES('old-lot','alice','monthly',330,300,?)", at)
			}
			if err := Migrate(ctx, handle.Writer); err != nil {
				t.Fatal(err)
			}
			for _, table := range []string{"test_entitlement_resets", "test_entitlement_reset_orders"} {
				var count int
				if err := handle.Reader.QueryRow("SELECT count(*) FROM " + table).Scan(&count); err != nil || count != 0 {
					t.Fatalf("%s = %d, %v", table, count, err)
				}
			}
			if version != 0 {
				for _, table := range []string{"voices", "posts", "videos", "subscriptions", "credit_lots"} {
					var count int
					if err := handle.Reader.QueryRow("SELECT count(*) FROM " + table).Scan(&count); err != nil || count != 1 {
						t.Fatalf("%s = %d, %v", table, count, err)
					}
				}
				var masters int
				if err := handle.Reader.QueryRow("SELECT count(*) FROM users WHERE id='operator' AND plan='master' AND password_hash='operator-hash'").Scan(&masters); err != nil || masters != 1 {
					t.Fatalf("operator preserved = %d, %v", masters, err)
				}
			}
		})
	}
}

func TestMigration0121CannotRemoveAppliedReplayGuards(t *testing.T) {
	ctx := context.Background()
	handle := openTemp(t)
	if err := Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	mustExec(t, handle.Writer, `INSERT INTO test_entitlement_resets(id,completed_at,backup_sha256,report_json)
		VALUES('pricing-v2-test-reset','2026-09-30T00:00:00Z','checksum','{}')`)
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, handle.Writer, sub, goose.WithLogger(goose.NopLogger()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.DownTo(ctx, 120); err == nil {
		t.Fatal("rollback removed an applied reset's replay guards")
	}
	var count int
	if err := handle.Reader.QueryRow("SELECT count(*) FROM test_entitlement_resets").Scan(&count); err != nil || count != 1 {
		t.Fatalf("completion marker after failed rollback = %d, %v", count, err)
	}
}
