package main

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/billing"
	billingstore "github.com/postpilot/backend/internal/billing/store"
	"github.com/postpilot/backend/internal/platform/db"
)

func resetFixture(t *testing.T) (*db.DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "entitlements.db")
	handle, err := db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = handle.Close() })
	if err := db.Migrate(context.Background(), handle.Writer); err != nil {
		t.Fatal(err)
	}
	return handle, path
}

func resetExec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
}

func resetCount(t *testing.T, db *sql.DB, query string) int {
	t.Helper()
	var count int
	if err := db.QueryRow(query).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func TestTestEntitlementResetPreservesWorkAndBlocksOrderReplay(t *testing.T) {
	ctx := context.Background()
	handle, path := resetFixture(t)
	at := "2026-09-30T00:00:00Z"
	end := "2026-10-30T00:00:00Z"
	resetExec(t, handle.Writer, "INSERT INTO users(id,password_hash,created_at,email,email_verified_at,plan) VALUES('alice','secret-hash',?,'a@example.invalid',?,'basic'),('root','root-hash',?,NULL,NULL,'master')", at, at, at)
	resetExec(t, handle.Writer, "INSERT INTO sessions(token,user_id,expires_at,created_at) VALUES('session-hash','alice',?,?)", end, at)
	resetExec(t, handle.Writer, "INSERT INTO auth_links(token_hash,user_id,purpose,email,expires_at,created_at) VALUES('reset-link','alice','reset_password','a@example.invalid',?,?)", end, at)
	resetExec(t, handle.Writer, "INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES('voice','alice','Original',1,?,?)", at, at)
	resetExec(t, handle.Writer, "INSERT INTO posts(slug,user_id,voice_id,created_at,updated_at) VALUES('post','alice','voice',?,?)", at, at)
	resetExec(t, handle.Writer, "INSERT INTO videos(id,post_slug,filename,r2_key,content_type,bytes,duration_ms,width,height,created_at) VALUES('video','post','original.mp4','retained-key','video/mp4',100,15000,1080,1920,?)", at)
	resetExec(t, handle.Writer, "INSERT INTO clip_projects(id,user_id,title,ratio,target_duration_ms,result_key,result_content_type,result_bytes,result_duration_ms,result_created_at,created_at,updated_at) VALUES('clip','alice','Result','vertical',15000,'result-key','video/mp4',100,15000,?,?,?)", at, at, at)
	resetExec(t, handle.Writer, "INSERT INTO clip_attempt_results(job_id,user_id,project_id,expected_revision,result_key,result_content_type,result_bytes,result_duration_ms,result_created_at) VALUES('done-job','alice','clip',0,'retained-result','video/mp4',100,15000,?)", at)
	resetExec(t, handle.Writer, "INSERT INTO generation_jobs(id,user_id,kind,status,created_at,updated_at) VALUES('done-job','alice','render_clip','done',?,?)", at, at)
	resetExec(t, handle.Writer, "INSERT INTO usage_events(user_id,kind,job_id,stage,model,prompt_tokens,completion_tokens,cost_microusd,cost_source,created_at) VALUES('alice','clip','done-job','write','priced',10,10,100,'reported',?)", at)
	resetExec(t, handle.Writer, "INSERT INTO usage_admissions(user_id,kind,job_id,created_at,hold_credits,settled_credits,settled_at) VALUES('alice','clip','done-job',?,40,7,?)", at, at)
	resetExec(t, handle.Writer, "INSERT INTO credit_lots(id,user_id,kind,granted,remaining,created_at) VALUES('old-lot','alice','monthly',40,33,?)", at)
	resetExec(t, handle.Writer, "INSERT INTO credit_hold_lots(job_id,lot_id,credits) VALUES('done-job','old-lot',7)")
	resetExec(t, handle.Writer, "INSERT INTO subscriptions(user_id,tier,term,anchor_at,term_start,term_end,next_grant_at,status,created_at,updated_at,coverage_id) VALUES('alice','basic','monthly',?,?,?,?,'active',?,?,'old-coverage')", at, at, end, end, at, at)
	resetExec(t, handle.Writer, "INSERT INTO payment_methods(user_id,provider,billing_key,customer_key,card_label,registered_at) VALUES('alice','fake','billing-secret','customer-secret','test card',?)", at)
	resetExec(t, handle.Writer, "INSERT INTO billing_intents(order_id,user_id,kind,tier,term,billing_key,customer_key,krw,quoted_at,status,created_at,updated_at) VALUES('old-order','alice','subscribe','basic','monthly','billing-secret','customer-secret',4900,?,'applied',?,?)", at, at, at)
	resetExec(t, handle.Writer, "INSERT INTO provider_notifications(provider,event_type,order_id,payload,received_at) VALUES('fake','charged','old-order','sensitive-test-payload',?)", at)
	resetExec(t, handle.Writer, "INSERT INTO billing_events(user_id,kind,order_id,created_at) VALUES('alice','charge','old-order',?)", at)
	resetExec(t, handle.Writer, "INSERT INTO vouchers(id,token,credits,validity_days,issued_by,issued_at,link_expires_at) VALUES('voucher','secret-token',100,30,'root',?,?)", at, end)
	resetExec(t, handle.Writer, "INSERT INTO server_export_windows(user_id,coverage_id,window_start,window_end,allowance,used,reserved) VALUES('alice','old-coverage',?,?,6,1,0)", at, end)
	resetExec(t, handle.Writer, "INSERT INTO server_export_reservations(id,user_id,project_id,plan_revision,job_id,coverage_id,window_start,state,created_at) VALUES('export','alice','clip',1,'done-job','old-coverage',?,'committed',?)", at, at)

	preview, err := resetTestEntitlements(ctx, handle, path, "", false, time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC))
	if err != nil || preview.Affected["subscriptions"] != 1 || preview.Preserved["videos"] != 1 || preview.hasBlockers() {
		t.Fatalf("dry-run=%+v err=%v", preview, err)
	}
	if resetCount(t, handle.Reader, "SELECT count(*) FROM subscriptions") != 1 {
		t.Fatal("dry-run changed subscriptions")
	}
	backupDir := t.TempDir()
	if err := os.Chmod(backupDir, 0o700); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(backupDir, "before-reset.db")
	report, err := resetTestEntitlements(ctx, handle, path, backup, true, time.Date(2026, 9, 30, 1, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if report.BackupSHA256 == "" || report.CompletedAt == "" {
		t.Fatalf("missing completion evidence: %+v", report)
	}
	if stat, err := os.Stat(backup); err != nil || stat.Mode().Perm() != 0o600 {
		t.Fatalf("backup mode=%v err=%v", stat, err)
	}
	for _, table := range testEntitlementTables {
		if got := resetCount(t, handle.Reader, "SELECT count(*) FROM "+table); got != 0 {
			t.Errorf("%s retained %d test rows", table, got)
		}
	}
	for _, table := range []string{"users", "sessions", "auth_links", "posts", "voices", "videos", "clip_projects", "clip_attempt_results", "generation_jobs", "usage_events", "usage_admissions"} {
		if got := resetCount(t, handle.Reader, "SELECT count(*) FROM "+table); got == 0 {
			t.Errorf("%s was lost", table)
		}
	}
	if resetCount(t, handle.Reader, "SELECT count(*) FROM users WHERE id='alice' AND plan='free' AND password_hash='secret-hash' AND email_verified_at IS NOT NULL") != 1 || resetCount(t, handle.Reader, "SELECT count(*) FROM users WHERE id='root' AND plan='master'") != 1 {
		t.Fatal("identity or master access changed")
	}
	if resetCount(t, handle.Reader, "SELECT count(*) FROM test_entitlement_reset_orders WHERE order_id='old-order'") != 1 {
		t.Fatal("order tombstone missing")
	}
	if _, err := handle.Writer.Exec("INSERT INTO billing_intents(order_id,user_id,kind,billing_key,customer_key,krw,quoted_at,created_at,updated_at) VALUES('old-order','alice','pack','secret','secret',3000,?,?,?)", at, at, at); err == nil || !strings.Contains(err.Error(), "retired test order") {
		t.Fatalf("old order replay accepted: %v", err)
	}
	resetExec(t, handle.Writer, "INSERT INTO billing_intents(order_id,user_id,kind,billing_key,customer_key,krw,quoted_at,status,created_at,updated_at) VALUES('new-order','alice','pack','new-key','new-customer',3000,?,'applied',?,?)", at, at, at)
	if _, err := handle.Writer.Exec("UPDATE billing_intents SET order_id='old-order' WHERE order_id='new-order'"); err == nil || !strings.Contains(err.Error(), "retired test order") {
		t.Fatalf("old order update accepted: %v", err)
	}
	service := billing.NewService(billingstore.New(handle.Writer, handle.Reader), nil, nil, nil, nil, nil)
	if err := service.ReconcileOrder(ctx, "old-order"); err != nil || resetCount(t, handle.Reader, "SELECT count(*) FROM credit_lots") != 0 {
		t.Fatalf("late provider notification regranted retired order: %v", err)
	}
	resetExec(t, handle.Writer, "INSERT INTO credit_lots(id,user_id,kind,granted,remaining,created_at) VALUES('new-lot','alice','bonus',10,10,?)", at)
	again, err := resetTestEntitlements(ctx, handle, path, backup, true, time.Now())
	if err != nil || !again.AlreadyApplied || resetCount(t, handle.Reader, "SELECT count(*) FROM test_entitlement_resets") != 1 || resetCount(t, handle.Reader, "SELECT count(*) FROM credit_lots WHERE id='new-lot'") != 1 {
		t.Fatalf("repeat=%+v err=%v", again, err)
	}
	copyDB, err := db.Open(backup)
	if err != nil {
		t.Fatal(err)
	}
	defer copyDB.Close()
	if resetCount(t, copyDB.Reader, "SELECT count(*) FROM subscriptions") != 1 {
		t.Fatal("backup cannot restore prior subscription")
	}
}

func TestTestEntitlementResetRefusesUnresolvedWork(t *testing.T) {
	for _, tc := range []struct{ name, insert, blocker string }{
		{"AI job", "INSERT INTO generation_jobs(id,user_id,kind,status,created_at,updated_at) VALUES('job','alice','generate','queued','now','now')", "ai_jobs"},
		{"render job", "INSERT INTO generation_jobs(id,user_id,kind,status,created_at,updated_at) VALUES('job','alice','render_clip','running','now','now')", "ai_jobs"},
		{"payment", "INSERT INTO billing_intents(order_id,user_id,kind,billing_key,customer_key,krw,quoted_at,created_at,updated_at) VALUES('order','alice','pack','key','customer',3000,'now','now','now')", "payment_intents"},
		{"refund", "INSERT INTO billing_refund_requests(id,user_id,order_id,reason,status,requested_at) VALUES('refund','alice','order','review','requested','now')", "refund_requests"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handle, path := resetFixture(t)
			resetExec(t, handle.Writer, "INSERT INTO users(id,password_hash,created_at,plan) VALUES('alice','hash','now','basic')")
			resetExec(t, handle.Writer, tc.insert)
			backup := filepath.Join(t.TempDir(), "blocked.db")
			report, err := resetTestEntitlements(context.Background(), handle, path, backup, true, time.Now())
			if err == nil || report.Blockers[tc.blocker] == 0 {
				t.Fatalf("reset not refused: %+v %v", report, err)
			}
			if _, statErr := os.Stat(backup); !os.IsNotExist(statErr) {
				t.Fatalf("backup made for blocked reset: %v", statErr)
			}
			if resetCount(t, handle.Reader, "SELECT count(*) FROM users WHERE plan='basic'") != 1 {
				t.Fatal("blocked reset changed account")
			}
		})
	}
}

func TestTestEntitlementResetCLIRequiresExplicitTestScope(t *testing.T) {
	var out bytes.Buffer
	for _, args := range [][]string{{"--apply"}, {"--apply", "--backup=/tmp/copy.db"}, {"--dry-run", "--apply"}} {
		if err := runTestEntitlementReset(context.Background(), "/missing.db", args, &out); err == nil {
			t.Fatalf("accepted unsafe args %v", args)
		}
	}
}

func TestTestEntitlementResetDryRunRequiresMigratedDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	handle, err := db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := handle.Close(); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := runTestEntitlementReset(context.Background(), path, []string{"--dry-run"}, &out); err == nil {
		t.Fatal("dry-run silently migrated an old database")
	}
	handle, err = db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	if resetCount(t, handle.Reader, "SELECT count(*) FROM sqlite_master WHERE name='goose_db_version'") != 0 {
		t.Fatal("dry-run changed the database schema")
	}
}

func TestTestEntitlementResetBackupRejectsSymlinkIntoDatabaseDirectory(t *testing.T) {
	dbDir := t.TempDir()
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(dbDir, alias); err != nil {
		t.Fatal(err)
	}
	if err := validateResetBackup(filepath.Join(dbDir, "active.db"), filepath.Join(alias, "backup.db")); err == nil {
		t.Fatal("backup alias into the active database directory was accepted")
	}
}
