package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

const creationMigrationFile = "0150_creation_working_sources.sql"
const writingMigrationFile = "0151_writing_tests.sql"
const browserAnalysisMigrationFile = "0148_browser_analysis_preparations.sql"

var creationAlter = regexp.MustCompile(`(?is)ALTER\s+TABLE\s+(\w+)\s+ADD\s+COLUMN\s+(\w+)\s+([^;]+);`)
var creationObject = regexp.MustCompile(`(?is)CREATE\s+(?:UNIQUE\s+)?(TABLE|INDEX)\s+(\w+)\b[^;]*;`)
var creationTrigger = regexp.MustCompile(`(?is)CREATE\s+(TRIGGER)\s+(\w+)\b.*?\bEND\s*;`)

// Only these unpublished creation schemas moved. Their complete definitions
// authorize a replay; a partially applied or differently constrained schema fails.
func creationUp(fsys fs.FS, name string) (string, error) {
	raw, err := fs.ReadFile(fsys, name)
	if err != nil {
		return "", err
	}
	up, _, ok := strings.Cut(string(raw), "-- +goose Down")
	if !ok || !strings.Contains(up, "-- +goose Up") {
		return "", fmt.Errorf("invalid guarded migration %s", name)
	}
	return up, nil
}

// Tokens preserve literal case and values while ignoring formatting/comments.
func creationSQLTokens(input string) string {
	var tokens []string
	for i := 0; i < len(input); {
		if unicode.IsSpace(rune(input[i])) {
			i++
			continue
		}
		if i+1 < len(input) && input[i:i+2] == "--" {
			for i < len(input) && input[i] != '\n' {
				i++
			}
			continue
		}
		if input[i] == '\'' || input[i] == '"' || input[i] == '`' {
			start, quote := i, input[i]
			i++
			for i < len(input) {
				if input[i] == quote {
					i++
					if i < len(input) && input[i] == quote {
						i++
						continue
					}
					break
				}
				i++
			}
			tokens = append(tokens, input[start:i])
			continue
		}
		start := i
		if input[i] >= 'a' && input[i] <= 'z' || input[i] >= 'A' && input[i] <= 'Z' || input[i] >= '0' && input[i] <= '9' || input[i] == '_' {
			for i < len(input) && (input[i] >= 'a' && input[i] <= 'z' || input[i] >= 'A' && input[i] <= 'Z' || input[i] >= '0' && input[i] <= '9' || input[i] == '_') {
				i++
			}
			tokens = append(tokens, strings.ToLower(input[start:i]))
		} else {
			tokens = append(tokens, input[i:i+1])
			i++
		}
	}
	return strings.Join(tokens, " ")
}

// Returns false only when every owned addition is absent. Existing objects must
// match the entire canonical DDL, including ownership/check/index constraints.
func creationSchemaPresent(ctx context.Context, tx *sql.Tx, up string) (bool, error) {
	any, complete := false, true
	for _, addition := range creationAlter.FindAllStringSubmatch(up, -1) {
		var definition string
		err := tx.QueryRowContext(ctx, "SELECT sql FROM sqlite_master WHERE type='table' AND name=?", addition[1]).Scan(&definition)
		if errors.Is(err, sql.ErrNoRows) {
			complete = false
			continue
		}
		if err != nil {
			return false, err
		}
		rows, err := tx.QueryContext(ctx, "PRAGMA table_info("+addition[1]+")")
		if err != nil {
			return false, err
		}
		found := false
		for rows.Next() {
			var cid, notNull, pk int
			var column, kind string
			var value sql.NullString
			if err := rows.Scan(&cid, &column, &kind, &notNull, &value, &pk); err != nil {
				rows.Close()
				return false, err
			}
			if column == addition[2] {
				found = true
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return false, err
		}
		if err := rows.Close(); err != nil {
			return false, err
		}
		if !found {
			complete = false
			continue
		}
		any = true
		if !strings.Contains(creationSQLTokens(definition), creationSQLTokens(addition[2]+" "+addition[3])) {
			return false, fmt.Errorf("incompatible creation column %s.%s", addition[1], addition[2])
		}
	}
	objects := append(creationObject.FindAllStringSubmatch(up, -1), creationTrigger.FindAllStringSubmatch(up, -1)...)
	if len(objects) == 0 {
		return false, fmt.Errorf("guarded schema has no owned objects")
	}
	for _, object := range objects {
		var kind, definition string
		err := tx.QueryRowContext(ctx, "SELECT type,sql FROM sqlite_master WHERE name=?", object[2]).Scan(&kind, &definition)
		if errors.Is(err, sql.ErrNoRows) {
			complete = false
			continue
		}
		if err != nil {
			return false, err
		}
		any = true
		if kind != strings.ToLower(object[1]) || creationSQLTokens(strings.TrimSuffix(strings.TrimSpace(object[0]), ";")) != creationSQLTokens(definition) {
			return false, fmt.Errorf("incompatible creation object %s", object[2])
		}
	}
	if any && !complete {
		return false, fmt.Errorf("partial creation schema; owned additions must be all present or all absent")
	}
	return any, nil
}

func applyCreationSchema(ctx context.Context, tx *sql.Tx, fsys fs.FS, name string) error {
	up, err := creationUp(fsys, name)
	if err != nil {
		return err
	}
	present, err := creationSchemaPresent(ctx, tx, up)
	if err != nil {
		return err
	}
	if present {
		return nil
	}
	if _, err := tx.ExecContext(ctx, up); err != nil {
		return err
	}
	present, err = creationSchemaPresent(ctx, tx, up)
	if err != nil {
		return err
	}
	if !present {
		return fmt.Errorf("guarded creation schema did not complete")
	}
	return nil
}

// Goose stores version numbers rather than filenames. Recognize only the two
// divergent local versions at the old tip, prove all prerequisite/owned schema,
// then preserve their ledger rows at150/151 and apply the published148/149 in the
// same writer transaction. Ordinary out-of-order migrations remain forbidden.
func reconcileCreationLineage(ctx context.Context, writer *sql.DB, fsys fs.FS) error {
	for _, name := range []string{creationMigrationFile, writingMigrationFile, browserAnalysisMigrationFile, waitExpiryMigrationFile} {
		if _, err := fs.Stat(fsys, name); errors.Is(err, fs.ErrNotExist) {
			return nil
		} else if err != nil {
			return err
		}
	}
	tx, err := writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var exists int
	if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name='goose_db_version'").Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return nil
	}
	rows, err := tx.QueryContext(ctx, "SELECT version_id,is_applied FROM goose_db_version ORDER BY id")
	if err != nil {
		return err
	}
	states := map[int64]bool{}
	maxVersion := int64(0)
	for rows.Next() {
		var version int64
		var applied bool
		if err := rows.Scan(&version, &applied); err != nil {
			rows.Close()
			return err
		}
		states[version] = applied
		if version > maxVersion {
			maxVersion = version
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if maxVersion < 148 || maxVersion > 149 {
		return nil
	}
	creation, err := creationUp(fsys, creationMigrationFile)
	if err != nil {
		return err
	}
	local, err := creationSchemaPresent(ctx, tx, creation)
	if err != nil {
		return err
	}
	if !local {
		return nil
	}
	browser, err := creationUp(fsys, browserAnalysisMigrationFile)
	if err != nil {
		return err
	}
	remote, err := creationSchemaPresent(ctx, tx, browser)
	if err != nil {
		return err
	}
	if remote {
		return nil
	}
	if _, ok := states[148]; !ok {
		return fmt.Errorf("local creation schema lacks148 migration history")
	}
	writing, err := creationUp(fsys, writingMigrationFile)
	if err != nil {
		return err
	}
	written, err := creationSchemaPresent(ctx, tx, writing)
	if err != nil {
		return err
	}
	_, has149 := states[149]
	if has149 && !written || !has149 && written {
		return fmt.Errorf("local149 history and writing schema disagree")
	}
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		prefix, _, ok := strings.Cut(entry.Name(), "_")
		if !ok {
			continue
		}
		version, err := strconv.ParseInt(prefix, 10, 64)
		if err != nil {
			return err
		}
		if version > 0 && version < 148 && !states[version] {
			return fmt.Errorf("missing prerequisite migration%d before local creation reconciliation", version)
		}
	}
	violations, err := tx.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return err
	}
	invalid := violations.Next()
	checkErr := violations.Err()
	violations.Close()
	if checkErr != nil {
		return checkErr
	}
	if invalid {
		return fmt.Errorf("local creation reconciliation requires valid foreign keys")
	}
	if _, err := tx.ExecContext(ctx, browser); err != nil {
		return err
	}
	if err := addWaitExpiry(ctx, tx); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE goose_db_version SET version_id=150 WHERE version_id=148"); err != nil {
		return err
	}
	if has149 {
		if _, err := tx.ExecContext(ctx, "UPDATE goose_db_version SET version_id=151 WHERE version_id=149"); err != nil {
			return err
		}
	}
	for _, version := range []int64{148, 149} {
		if _, err := tx.ExecContext(ctx, "INSERT INTO goose_db_version(version_id,is_applied) VALUES(?,1)", version); err != nil {
			return err
		}
	}
	return tx.Commit()
}
