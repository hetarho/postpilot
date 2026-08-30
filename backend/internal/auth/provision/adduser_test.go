package provision

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/platform/db"
)

// runWithStdin drives Run the way a piped invocation does: no TTY, so the password is
// read twice from stdin.
func runWithStdin(t *testing.T, dbPath, stdin string, first string, rest ...any) error {
	args := []string{}
	var bootstraps []Bootstrap
	if first != "" {
		args = append(args, first)
	}
	for _, item := range rest {
		switch value := item.(type) {
		case string:
			args = append(args, value)
		case Bootstrap:
			bootstraps = append(bootstraps, value)
		case func(context.Context, *db.DB, string) error:
			bootstraps = append(bootstraps, value)
		}
	}
	t.Helper()
	t.Setenv("DB_PATH", dbPath)

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	if _, err := w.WriteString(stdin); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	w.Close()

	original := os.Stdin
	os.Stdin = r
	defer func() {
		os.Stdin = original
		r.Close()
	}()

	return Run(context.Background(), args, bootstraps...)
}

// TestRunCreatesAccountOnAFreshVolume is job 01 A10's precondition: adduser must work
// against a database that does not exist yet, because provisioning the first account
// is the first thing an operator does after a deploy.
func TestRunCreatesAccountOnAFreshVolume(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "fresh", "postpilot.db")

	if err := runWithStdin(t, dbPath, "hunter2\nhunter2\n", "alice"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	handle, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer handle.Close()

	var storedHash string
	if err := handle.Reader.QueryRow("SELECT password_hash FROM users WHERE id = 'alice'").Scan(&storedHash); err != nil {
		t.Fatalf("account was not created: %v", err)
	}
	if !strings.HasPrefix(storedHash, "$argon2id$") {
		t.Errorf("password_hash = %q, want an argon2id PHC string", storedHash)
	}
	if strings.Contains(storedHash, "hunter2") {
		t.Error("the plaintext password reached the database")
	}
}

// TestRunDuplicateID is plan 01 AC5: a duplicate id is refused with a clear message.
func TestRunDuplicateID(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "postpilot.db")

	if err := runWithStdin(t, dbPath, "hunter2\nhunter2\n", "alice"); err != nil {
		t.Fatalf("first Run: %v", err)
	}

	err := runWithStdin(t, dbPath, "other\nother\n", "alice")
	if err == nil {
		t.Fatal("a duplicate id was accepted")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("message = %q, want it to say the account already exists", err)
	}
}

func TestRunRejectsBadInput(t *testing.T) {
	cases := map[string]struct {
		args  []string
		stdin string
		want  string
	}{
		"no id":              {args: nil, stdin: "", want: "usage"},
		"blank id":           {args: []string{"  "}, stdin: "", want: "usage"},
		"too many args":      {args: []string{"a", "b"}, stdin: "", want: "usage"},
		"mismatched confirm": {args: []string{"alice"}, stdin: "one\ntwo\n", want: "do not match"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			first := ""
			rest := []any{}
			for i, arg := range tc.args {
				if i == 0 {
					first = arg
					continue
				}
				rest = append(rest, arg)
			}
			err := runWithStdin(t, filepath.Join(t.TempDir(), "postpilot.db"), tc.stdin, first, rest...)
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("message = %q, want it to contain %q", err, tc.want)
			}
		})
	}
}

// TestRunAcceptsAPassphraseWithSpaces guards the piped path DEPLOY.md §4 documents:
// whitespace tokenizing would silently truncate the passphrase an operator typed.
func TestRunAcceptsAPassphraseWithSpaces(t *testing.T) {
	const passphrase = "correct horse battery staple"
	dbPath := filepath.Join(t.TempDir(), "postpilot.db")

	if err := runWithStdin(t, dbPath, passphrase+"\n"+passphrase+"\n", "alice"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	handle, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer handle.Close()

	var storedHash string
	if err := handle.Reader.QueryRow("SELECT password_hash FROM users WHERE id = 'alice'").Scan(&storedHash); err != nil {
		t.Fatalf("account was not created: %v", err)
	}
	// The whole passphrase must be what was hashed, not the first word.
	if ok, err := auth.VerifyPassword(passphrase, storedHash); err != nil || !ok {
		t.Errorf("the stored hash does not verify the full passphrase (ok=%v err=%v)", ok, err)
	}
	if ok, _ := auth.VerifyPassword("correct", storedHash); ok {
		t.Error("only the first word was hashed — the passphrase was truncated at a space")
	}
}

// A bootstrap runs after the user row exists and again on a rerun, so an account whose
// first bootstrap failed is repaired by running adduser again; a failing bootstrap makes Run
// exit non-zero because the account is not usable yet.
func TestRunBootstrapsTheAccountAndRepairsOnRerun(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "postpilot.db")
	var calls []string
	failing := true
	bootstrap := func(_ context.Context, _ *db.DB, userID string) error {
		calls = append(calls, userID)
		if failing {
			return errors.New("voice table unavailable")
		}
		return nil
	}
	err := runWithStdin(t, dbPath, "hunter2\nhunter2\n", "alice", bootstrap)
	if err == nil || !strings.Contains(err.Error(), "not usable yet") {
		t.Fatalf("failed bootstrap error = %v", err)
	}
	if len(calls) != 1 || calls[0] != "alice" {
		t.Fatalf("bootstrap calls = %v", calls)
	}
	// The user row exists, so the rerun is a duplicate — and it still runs the bootstrap.
	failing = false
	err = runWithStdin(t, dbPath, "hunter2\nhunter2\n", "alice", bootstrap)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("rerun error = %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("rerun did not repair: calls = %v", calls)
	}
}
