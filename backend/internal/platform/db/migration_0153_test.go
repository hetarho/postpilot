package db

import (
	"testing"
)

func TestMigration0153AddsDurableExecutionWithoutChangingPaidHistory(t *testing.T) {
	h := openTemp(t)
	ctx := t.Context()
	if err := migrate(ctx, h.Writer, migrationsBefore(t, "0153_")); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`INSERT INTO users(id,password_hash,created_at) VALUES('owner','hash','created')`,
		`INSERT INTO model_experiments(id,user_id,stage,status,created_at,origin,source,review_mode,input_hash,prompt_version,target_language) VALUES('old-paid','owner','write','completed','created','lab','post','candidate_ranking','retained-hash','retained-v1','ko')`,
		`INSERT INTO writing_tests(id,user_id,operation_key,fingerprint,kind,factor,model_stage,count,status,context,common_hash,prompt_version,confirmed_credits,created_at,updated_at) VALUES('kept-test','owner','old-request','fingerprint','knockout','model','write',4,'failed','{"private":"retained"}','hash','v1',19,'created','updated')`,
	} {
		if _, err := h.Writer.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if err := Migrate(ctx, h.Writer); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, h.Writer); err != nil {
		t.Fatal("second boot", err)
	}
	var private, status string
	var count, credits int
	if err := h.Reader.QueryRow(`SELECT context,status,count,confirmed_credits FROM writing_tests WHERE id='kept-test'`).Scan(&private, &status, &count, &credits); err != nil {
		t.Fatal(err)
	}
	if private != `{"private":"retained"}` || status != "failed" || count != 4 || credits != 19 {
		t.Fatal("paid test changed")
	}
	var legacy int
	if err := h.Reader.QueryRow(`SELECT count(*) FROM model_experiments WHERE id='old-paid' AND status='completed' AND review_mode='candidate_ranking'`).Scan(&legacy); err != nil || legacy != 1 {
		t.Fatal("legacy paid history changed", err)
	}
	for _, table := range []string{"writing_test_quotes", "writing_test_attempts", "writing_test_attempt_checkpoints", "writing_test_operations"} {
		var n int
		if err := h.Reader.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&n); err != nil || n != 1 {
			t.Fatalf("missing table %s: %v", table, err)
		}
	}
	rows, err := h.Reader.Query("PRAGMA foreign_key_check")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if rows.Next() {
		t.Fatal("foreign key violation")
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
}
