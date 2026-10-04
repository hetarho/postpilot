package store_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// namedQuery returns one `-- name:` statement of queries/*.sql as SQLite can plan it: every
// sqlc.arg/narg becomes a plain parameter, and the result counts them.
func namedQuery(t *testing.T, name string) (string, int) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("queries", "*.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("queries: %v (%d files)", err, len(files))
	}
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var body strings.Builder
		inside := false
		for _, line := range strings.Split(string(raw), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "-- name:") {
				if inside {
					break
				}
				inside = len(strings.Fields(trimmed)) > 2 && strings.Fields(trimmed)[2] == name
				continue
			}
			if inside && !strings.HasPrefix(trimmed, "--") {
				body.WriteString(line + "\n")
			}
		}
		if inside {
			query := sqlcArg.ReplaceAllString(body.String(), "?")
			return query, strings.Count(query, "?")
		}
	}
	t.Fatalf("no statement named %s", name)
	return "", 0
}

var sqlcArg = regexp.MustCompile(`sqlc\.n?arg\(\w+\)`)

// F4: a clip project's latest job is read on every clip poll and once per project in the
// list. It must be found through an index, newest first, not by scanning every job ever run.
// Only SQLite's SCAN word is pinned: an index name or the plan's nesting is not stable.
func TestProjectLatestReadsSearchAnIndex(t *testing.T) {
	_, handle := subjectHarness(t)
	for _, name := range []string{"LatestForProject", "LatestForProjectExcept"} {
		query, params := namedQuery(t, name)
		args := make([]any, params)
		for i := range args {
			args[i] = "x"
		}
		rows, err := handle.Writer.Query("EXPLAIN QUERY PLAN "+query, args...)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var details []string
		for rows.Next() {
			var id, parent, notUsed int
			var detail string
			if err := rows.Scan(&id, &parent, &notUsed, &detail); err != nil {
				t.Fatal(err)
			}
			details = append(details, detail)
		}
		rows.Close()
		searched := false
		for _, detail := range details {
			if strings.HasPrefix(detail, "SCAN generation_jobs") || strings.Contains(detail, "TEMP B-TREE") {
				t.Errorf("%s scans or sorts generation_jobs: %q", name, details)
			}
			searched = searched || strings.HasPrefix(detail, "SEARCH generation_jobs") && strings.Contains(detail, " INDEX ")
		}
		if !searched {
			t.Errorf("%s never searches generation_jobs through an index: %q", name, details)
		}
	}
}
