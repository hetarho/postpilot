package db

import (
	"fmt"
	"io/fs"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

// fkGuardLintAfter is the last migration the foreign-key guard rule exempts: 0001–0129 are
// applied and never edited (ARCH-43), so their whole-database guards and 0125's bare PRAGMA stay
// as they are, and every migration numbered above it is held to the rule.
const fkGuardLintAfter = 129

// fkGuardRule is what a refused migration is told.
const fkGuardRule = "a migration checks the foreign keys of the tables it rebuilds and their FK children only, " +
	"each as `EXISTS (SELECT 1 FROM pragma_foreign_key_check('<table>'))` inside its integrity guard, as 0129 does: " +
	"a whole-database check costs a scan of every table at that boot and lets a violation the migration never touched abort the deploy, " +
	"and a `PRAGMA foreign_key_check` statement checks nothing, because a migration never reads its rows"

var (
	sqlLineComment = regexp.MustCompile(`--[^\n]*`)
	// fkCheckFunction is the table-valued form, which a guard can read; scopedFKCheck is the
	// table argument that has to follow it.
	fkCheckFunction = regexp.MustCompile(`(?i)\bpragma_foreign_key_check\b`)
	scopedFKCheck   = regexp.MustCompile(`^\s*\(\s*'[^']+'\s*[,)]`)
	// fkCheckPragma is the statement form, with or without a schema or a table.
	fkCheckPragma = regexp.MustCompile(`(?i)^pragma\s+(?:\w+\s*\.\s*)?foreign_key_check\b`)
)

// fkGuardProblems lints one migration's SQL: every foreign-key check that is not scoped to a
// named table, by line. It reads the SQL loosely — `--` comments stripped, statements split on
// `;` — because it is a lint, not a SQL parser.
func fkGuardProblems(sql string) []string {
	// The comments go and their newlines stay, so an offset still names its line in the file.
	sql = sqlLineComment.ReplaceAllString(sql, "")
	lineAt := func(offset int) int { return strings.Count(sql[:offset], "\n") + 1 }
	var problems []string
	start := 0
	for _, statement := range strings.SplitAfter(sql, ";") {
		trimmed := strings.TrimLeftFunc(statement, unicode.IsSpace)
		if fkCheckPragma.MatchString(trimmed) {
			begin := start + len(statement) - len(trimmed)
			problems = append(problems, fmt.Sprintf("line %d: a PRAGMA foreign_key_check statement, whose rows nothing reads", lineAt(begin)))
		} else {
			for _, at := range fkCheckFunction.FindAllStringIndex(statement, -1) {
				if !scopedFKCheck.MatchString(statement[at[1]:]) {
					problems = append(problems, fmt.Sprintf("line %d: a pragma_foreign_key_check with no table, which checks the whole database", lineAt(start+at[0])))
				}
			}
		}
		start += len(statement)
	}
	return problems
}

// F22: a migration after 0129 checks foreign keys only of the tables it rebuilds, so boot time
// and deploy safety stop depending on unrelated data, and never leaves a check whose rows are
// discarded.
func TestNewMigrationsScopeTheirForeignKeyGuards(t *testing.T) {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		number, err := strconv.Atoi(strings.SplitN(entry.Name(), "_", 2)[0])
		if err != nil {
			t.Fatalf("%s: a migration file is named NNNN_<slug>.sql (ARCH-10)", entry.Name())
		}
		if number <= fkGuardLintAfter {
			continue
		}
		data, err := fs.ReadFile(migrationsFS, "migrations/"+entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		for _, problem := range fkGuardProblems(string(data)) {
			t.Errorf("%s: %s\n  the rule: %s", entry.Name(), problem, fkGuardRule)
		}
	}
}

// The lint itself: it refuses a whole-database guard and a bare PRAGMA, in any spelling, passes
// a scoped guard and a mention in a comment, and finds what it exists for in the exempt files.
func TestTheForeignKeyGuardLint(t *testing.T) {
	for name, test := range map[string]struct {
		sql     string
		refused int
	}{
		"a whole-database guard": {
			"INSERT INTO migration_guard(problem)\nSELECT 'violation' WHERE EXISTS (SELECT 1 FROM pragma_foreign_key_check);", 1,
		},
		"a whole-database guard over lines": {
			"WHERE EXISTS (\n    SELECT 1 FROM pragma_foreign_key_check\n    UNION ALL SELECT 1 FROM posts WHERE voice_id IS NULL\n);", 1,
		},
		"an empty argument list": {"SELECT 1 FROM Pragma_Foreign_Key_Check();", 1},
		"a bare PRAGMA":          {"COMMIT;\nPRAGMA foreign_key_check;\nPRAGMA foreign_keys=ON;", 1},
		"a bare PRAGMA naming a table and a schema": {
			"pragma main.foreign_key_check(generation_jobs);\nPRAGMA foreign_key_check('posts')", 2,
		},
		"a scoped guard": {
			"WHERE EXISTS (SELECT 1 FROM pragma_foreign_key_check('generation_jobs'))\n   OR EXISTS (SELECT 1 FROM pragma_foreign_key_check( 'posts' , 'main' ));", 0,
		},
		"one scoped check beside a whole-database one": {
			"WHERE EXISTS (SELECT 1 FROM pragma_foreign_key_check('generation_jobs')) OR EXISTS (SELECT 1 FROM pragma_foreign_key_check);", 1,
		},
		"checks mentioned only in comments": {
			"-- `foreign_key_check` proves the graph; PRAGMA foreign_key_check;\nPRAGMA foreign_keys=ON; -- pragma_foreign_key_check", 0,
		},
		"other pragmas": {"PRAGMA foreign_keys=OFF;\nPRAGMA foreign_key_list('posts');", 0},
	} {
		if got := fkGuardProblems(test.sql); len(got) != test.refused {
			t.Errorf("%s: refused %d %q, want %d", name, len(got), got, test.refused)
		}
	}
	for file, refused := range map[string][]string{
		// The scoped guard the rule points at: the rebuilt table and its four FK children, twice.
		"0129_job_kind_checks_out_of_schema.sql": nil,
		// Exempt by number alone: the bare PRAGMA that verified nothing, and a whole-database guard.
		"0125_ranked_experiment_completion.sql": {"line 106: a PRAGMA foreign_key_check statement, whose rows nothing reads"},
		"0115_entitlement_windows.sql":          {"line 120: a pragma_foreign_key_check with no table, which checks the whole database"},
	} {
		data, err := fs.ReadFile(migrationsFS, "migrations/"+file)
		if err != nil {
			t.Fatal(err)
		}
		if got := fkGuardProblems(string(data)); !reflect.DeepEqual(got, refused) {
			t.Errorf("%s: refused %q, want %q", file, got, refused)
		}
	}
}
