package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"time"

	"github.com/postpilot/backend/internal/platform/db"
)

const testEntitlementResetID = "pricing-v2-test-reset"

func runTestEntitlementReset(ctx context.Context, dbPath string, args []string, out io.Writer) error {
	flags := flag.NewFlagSet("reset-test-entitlements", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	dryRun := flags.Bool("dry-run", false, "report scope without deleting entitlements")
	apply := flags.Bool("apply", false, "perform the one-time reset")
	testOnly := flags.Bool("test-only", false, "acknowledge that the entitlements are test data")
	backup := flags.String("backup", "", "absolute destination in a private directory")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *dryRun == *apply {
		return errors.New("choose exactly one of --dry-run or --apply")
	}
	if *apply && (!*testOnly || *backup == "") {
		return errors.New("--apply requires --test-only and --backup")
	}
	if _, err := os.Stat(dbPath); err != nil {
		return fmt.Errorf("existing database required: %w", err)
	}
	handle, err := db.Open(dbPath)
	if err != nil {
		return err
	}
	defer handle.Close()
	var installed int
	if err := handle.Reader.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name='test_entitlement_resets'").Scan(&installed); err != nil {
		return err
	}
	if installed != 1 {
		return errors.New("database must be migrated through the test-entitlement reset schema before this command")
	}
	report, err := resetTestEntitlements(ctx, handle, dbPath, *backup, *apply, time.Now())
	if encodeErr := json.NewEncoder(out).Encode(report); encodeErr != nil {
		return encodeErr
	}
	return err
}

// These are the test payment and entitlement records the one-time operation
// clears. Authored work, identities, sessions, FX publications and metered
// provider usage are deliberately outside this explicit scope.
var testEntitlementTables = []string{
	"subscriptions", "payment_methods", "billing_events", "billing_intents",
	"billing_quotes", "credit_purchases", "provider_notifications",
	"billing_refund_requests", "billing_refund_decisions", "billing_refund_provider_outcomes",
	"credit_refund_funding_guards", "server_export_refund_guards",
	"credit_lots", "credit_hold_lots", "usage_admission_eligible_lots",
	"entitlement_tier_transitions", "support_coverages", "vouchers",
	"server_export_windows", "server_export_reservations", "server_export_adjustments",
	"clip_generation_quotes",
}

// Every guard is checked again under the writer transaction immediately before
// deletion. A stale dry-run cannot authorize a reset while a job or provider
// outcome appeared in the meantime.
var testEntitlementBlockers = map[string]string{
	"ai_jobs":             "SELECT count(*) FROM generation_jobs WHERE status IN ('queued','running')",
	"model_experiments":   "SELECT count(*) FROM model_experiments WHERE status IN ('queued','running')",
	"ai_admissions":       "SELECT count(*) FROM usage_admissions WHERE settled_at IS NULL",
	"render_stages":       "SELECT count(*) FROM clip_media_stages WHERE state IN ('queued','running')",
	"render_attempts":     "SELECT count(*) FROM clip_media_attempts WHERE finished_at IS NULL",
	"server_reservations": "SELECT count(*) FROM server_export_reservations WHERE state='reserved'",
	"payment_intents":     "SELECT count(*) FROM billing_intents WHERE status IN ('pending','review')",
	"refund_requests":     "SELECT count(*) FROM billing_refund_requests WHERE status IN ('requested','processing','failed')",
}

type testResetReport struct {
	ID             string         `json:"id"`
	AlreadyApplied bool           `json:"already_applied"`
	CompletedAt    string         `json:"completed_at,omitempty"`
	BackupPath     string         `json:"backup_path,omitempty"`
	BackupSHA256   string         `json:"backup_sha256,omitempty"`
	Preserved      map[string]int `json:"preserved"`
	Affected       map[string]int `json:"affected"`
	Blockers       map[string]int `json:"blockers"`
}

type resetQuerier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func countResetRows(ctx context.Context, q resetQuerier, query string) (int, error) {
	var count int
	if err := q.QueryRowContext(ctx, query).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func inspectTestEntitlements(ctx context.Context, q resetQuerier) (testResetReport, error) {
	report := testResetReport{ID: testEntitlementResetID,
		Preserved: map[string]int{}, Affected: map[string]int{}, Blockers: map[string]int{}}
	var completed string
	err := q.QueryRowContext(ctx, "SELECT completed_at FROM test_entitlement_resets WHERE id=?", testEntitlementResetID).Scan(&completed)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return report, err
	}
	if err == nil {
		report.AlreadyApplied, report.CompletedAt = true, completed
	}
	for name, query := range map[string]string{
		"accounts":            "SELECT count(*) FROM users",
		"masters":             "SELECT count(*) FROM users WHERE plan='master'",
		"verified_identities": "SELECT count(*) FROM users WHERE email_verified_at IS NOT NULL",
		"posts":               "SELECT count(*) FROM posts",
		"voices":              "SELECT count(*) FROM voices",
		"videos":              "SELECT count(*) FROM videos",
		"clip_results":        "SELECT count(*) FROM clip_attempt_results",
		"usage_events":        "SELECT count(*) FROM usage_events",
	} {
		value, err := countResetRows(ctx, q, query)
		if err != nil {
			return report, fmt.Errorf("count %s: %w", name, err)
		}
		report.Preserved[name] = value
	}
	for _, name := range testEntitlementTables {
		value, err := countResetRows(ctx, q, "SELECT count(*) FROM "+name)
		if err != nil {
			return report, fmt.Errorf("count %s: %w", name, err)
		}
		report.Affected[name] = value
	}
	paid, err := countResetRows(ctx, q, "SELECT count(*) FROM users WHERE plan NOT IN ('free','master')")
	if err != nil {
		return report, err
	}
	report.Affected["paid_account_plans"] = paid
	for name, query := range testEntitlementBlockers {
		value, err := countResetRows(ctx, q, query)
		if err != nil {
			return report, fmt.Errorf("guard %s: %w", name, err)
		}
		report.Blockers[name] = value
	}
	return report, nil
}

func (r testResetReport) hasBlockers() bool {
	for _, count := range r.Blockers {
		if count > 0 {
			return true
		}
	}
	return false
}

// resetTestEntitlements never runs at boot. A dry-run reports exact table scope;
// applying requires a distinct private backup destination and an explicit CLI flag.
func resetTestEntitlements(ctx context.Context, handle *db.DB, dbPath, backupPath string, apply bool, now time.Time) (testResetReport, error) {
	preview, err := inspectTestEntitlements(ctx, handle.Reader)
	if err != nil || !apply || preview.AlreadyApplied {
		return preview, err
	}
	if preview.hasBlockers() {
		return preview, errors.New("unresolved AI, render, payment or refund work prevents reset")
	}
	if err := validateResetBackup(dbPath, backupPath); err != nil {
		return preview, err
	}
	if _, err := handle.Writer.ExecContext(ctx, "VACUUM INTO ?", backupPath); err != nil {
		return preview, fmt.Errorf("backup database: %w", err)
	}
	if err := os.Chmod(backupPath, 0o600); err != nil {
		return preview, err
	}
	file, err := os.Open(backupPath)
	if err != nil {
		return preview, err
	}
	hash := sha256.New()
	_, copyErr := io.Copy(hash, file)
	closeErr := file.Close()
	if copyErr != nil {
		return preview, copyErr
	}
	if closeErr != nil {
		return preview, closeErr
	}
	preview.BackupPath = backupPath
	preview.BackupSHA256 = hex.EncodeToString(hash.Sum(nil))

	tx, err := handle.Writer.BeginTx(ctx, nil)
	if err != nil {
		return preview, err
	}
	defer tx.Rollback()
	report, err := inspectTestEntitlements(ctx, tx)
	if err != nil {
		return preview, err
	}
	if report.AlreadyApplied {
		return report, nil
	}
	if report.hasBlockers() {
		return report, errors.New("work appeared after backup; reset refused")
	}
	if !reflect.DeepEqual(report.Preserved, preview.Preserved) || !reflect.DeepEqual(report.Affected, preview.Affected) {
		return report, errors.New("database changed after backup; reset refused")
	}
	report.BackupPath, report.BackupSHA256 = preview.BackupPath, preview.BackupSHA256
	report.CompletedAt = now.UTC().Format(time.RFC3339Nano)
	reportJSON, err := json.Marshal(report)
	if err != nil {
		return report, err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO test_entitlement_resets(id,completed_at,backup_sha256,report_json) VALUES(?,?,?,?)", testEntitlementResetID, report.CompletedAt, report.BackupSHA256, string(reportJSON)); err != nil {
		return report, err
	}
	for _, source := range []string{"billing_intents", "credit_purchases", "provider_notifications", "billing_events"} {
		if _, err := tx.ExecContext(ctx, "INSERT OR IGNORE INTO test_entitlement_reset_orders(order_id,reset_id) SELECT order_id,? FROM "+source+" WHERE order_id IS NOT NULL AND order_id<>''", testEntitlementResetID); err != nil {
			return report, err
		}
	}
	// Child tables precede parents. Retained completed jobs/results retain their
	// content and confirmed provider usage, but cannot reopen old entitlements.
	for _, table := range []string{
		"billing_refund_provider_outcomes", "billing_refund_decisions", "credit_refund_funding_guards", "server_export_refund_guards", "billing_refund_requests",
		"provider_notifications", "billing_events", "credit_purchases", "billing_intents", "billing_quotes", "payment_methods", "subscriptions",
		"server_export_reservations", "server_export_adjustments", "server_export_windows", "entitlement_tier_transitions", "support_coverages",
		"vouchers", "clip_generation_quotes", "credit_hold_lots", "usage_admission_eligible_lots", "credit_lots",
	} {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+table); err != nil {
			return report, fmt.Errorf("clear %s: %w", table, err)
		}
	}
	if _, err := tx.ExecContext(ctx, "UPDATE users SET plan='free' WHERE plan<>'master'"); err != nil {
		return report, err
	}
	if err := tx.Commit(); err != nil {
		return report, err
	}
	return report, nil
}

func validateResetBackup(dbPath, backupPath string) error {
	if !filepath.IsAbs(backupPath) {
		return errors.New("backup path must be absolute")
	}
	dbDir, err := filepath.EvalSymlinks(filepath.Dir(dbPath))
	if err != nil {
		return err
	}
	dbDir, err = filepath.Abs(dbDir)
	if err != nil {
		return err
	}
	backupDir, err := filepath.EvalSymlinks(filepath.Dir(backupPath))
	if err != nil {
		return err
	}
	backupDir, err = filepath.Abs(backupDir)
	if err != nil {
		return err
	}
	if backupDir == dbDir {
		return errors.New("backup must be outside the database directory")
	}
	if _, err := os.Lstat(backupPath); err == nil {
		return errors.New("backup already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	parent, err := os.Stat(filepath.Dir(backupPath))
	if err != nil {
		return err
	}
	if !parent.IsDir() || parent.Mode().Perm()&0o077 != 0 {
		return errors.New("backup directory must be private (0700)")
	}
	return nil
}
