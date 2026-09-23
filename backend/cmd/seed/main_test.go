package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/auth"
	authstore "github.com/postpilot/backend/internal/auth/store"
	"github.com/postpilot/backend/internal/devseed"
	"github.com/postpilot/backend/internal/mail"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/db"
)

func TestSeedCreatesShortEmailFreeAccountsThatCanLogIn(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "seed.db")
	t.Setenv("DB_PATH", path)
	t.Setenv("MAIL_DRIVER", "log")
	t.Setenv("GOOGLE_CLIENT_ID", "")
	t.Setenv("GOOGLE_CLIENT_SECRET", "")
	if err := run(ctx); err != nil {
		t.Fatal(err)
	}
	handle, err := db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	store := authstore.New(handle.Writer, handle.Reader)
	svc := auth.NewService(store, 720*time.Hour, auth.Deps{Mailer: mail.NewLog()})
	expected := []struct {
		id    string
		tier  plan.Plan
		posts int
	}{
		{"free", plan.Free, 0}, {"base", plan.Basic, 3}, {"pro", plan.Pro, 8},
		{"max", plan.Max, 14}, {"master", plan.Master, 23},
	}
	var accounts int
	if err := handle.Reader.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&accounts); err != nil {
		t.Fatal(err)
	}
	if accounts != len(expected) {
		t.Fatalf("accounts = %d", accounts)
	}
	for _, want := range expected {
		user, token, err := svc.Login(ctx, want.id, devseed.Password)
		if err != nil {
			t.Fatalf("login %s: %v", want.id, err)
		}
		if user.ID != want.id || user.Plan != want.tier || user.Email != "" || user.EmailVerifiedAt != nil || token == "" {
			t.Fatalf("unexpected account identity for %s", want.id)
		}
		var posts int
		if err := handle.Reader.QueryRowContext(ctx, "SELECT COUNT(*) FROM posts WHERE user_id = ?", want.id).Scan(&posts); err != nil {
			t.Fatal(err)
		}
		if posts != want.posts {
			t.Errorf("%s posts = %d, want %d", want.id, posts, want.posts)
		}
	}
}

// seed runs the command against path, the way `pnpm dev --seed` does.
func seed(t *testing.T, path string) {
	t.Helper()
	t.Setenv("DB_PATH", path)
	t.Setenv("MAIL_DRIVER", "log")
	t.Setenv("GOOGLE_CLIENT_ID", "")
	t.Setenv("GOOGLE_CLIENT_SECRET", "")
	if err := run(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func open(t *testing.T, path string) *sql.DB {
	t.Helper()
	handle, err := db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { handle.Close() })
	return handle.Reader
}

// phraseRow is a field_phrase_lists row, every column, so "left standing" can mean
// byte-identical.
type phraseRow struct {
	Field, Phrases, NextRefreshAt string
	CorpusSize                    int64
	RefreshedAt                   sql.NullString
}

func phraseRows(t *testing.T, reader *sql.DB) []phraseRow {
	t.Helper()
	rows, err := reader.Query("SELECT field, phrases, corpus_size, refreshed_at, next_refresh_at FROM field_phrase_lists ORDER BY field")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []phraseRow
	for rows.Next() {
		var row phraseRow
		if err := rows.Scan(&row.Field, &row.Phrases, &row.CorpusSize, &row.RefreshedAt, &row.NextRefreshAt); err != nil {
			t.Fatal(err)
		}
		out = append(out, row)
	}
	return out
}

// seedShape is what a seed wrote, without the ids and clock readings a rerun renews.
type seedShape struct {
	Statuses   map[string]int // "account/status" → posts
	Nouns      int            // posts carrying nouns
	Fields     map[string]int // account → posts carrying daily_life
	Candidates map[string]int // account → posts carrying candidates
	Templates  []string       // "account" per template with a title area
	Phrases    []string       // the phrase lists' fields
}

// checkSeed asserts every fixture invariant the database can show and returns its shape.
func checkSeed(t *testing.T, reader *sql.DB) seedShape {
	t.Helper()
	ctx := context.Background()
	shape := seedShape{Statuses: map[string]int{}, Fields: map[string]int{}, Candidates: map[string]int{}}
	scan := func(query string, each func(*sql.Rows) error) {
		t.Helper()
		rows, err := reader.QueryContext(ctx, query)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		for rows.Next() {
			if err := each(rows); err != nil {
				t.Fatal(err)
			}
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
	}

	scan("SELECT user_id, status, COUNT(*) FROM posts GROUP BY user_id, status", func(rows *sql.Rows) error {
		var user, status string
		var n int
		if err := rows.Scan(&user, &status, &n); err != nil {
			return err
		}
		shape.Statuses[user+"/"+status] = n
		return nil
	})
	want := map[string]int{
		"base/draft": 2, "base/review": 1,
		"pro/draft": 3, "pro/review": 2, "pro/finalized": 1, "pro/published": 2,
		"max/draft": 4, "max/review": 3, "max/finalized": 7,
		"master/draft": 5, "master/review": 4, "master/finalized": 3, "master/published": 11,
	}
	if !reflect.DeepEqual(shape.Statuses, want) {
		t.Fatalf("status spread %v, want %v", shape.Statuses, want)
	}

	// POST-73: every published row has an address of the stored shape and a distinct moment.
	moments := map[string]bool{}
	scan("SELECT user_id, published_url, published_at FROM posts WHERE status = 'published'", func(rows *sql.Rows) error {
		var user string
		var address, at sql.NullString
		if err := rows.Scan(&user, &address, &at); err != nil {
			return err
		}
		if !strings.HasPrefix(address.String, "https://blog.naver.com/") || !at.Valid || moments[user+at.String] {
			t.Errorf("published row of %s carries %q at %v", user, address.String, at)
		}
		moments[user+at.String] = true
		return nil
	})

	// GEN-55: nouns exactly on the posts a write produced.
	var misplacedNouns int
	if err := reader.QueryRowContext(ctx, `SELECT COUNT(*) FROM posts
		WHERE (content_nouns IS NOT NULL) <> (status <> 'draft')`).Scan(&misplacedNouns); err != nil {
		t.Fatal(err)
	}
	if err := reader.QueryRowContext(ctx, "SELECT COUNT(*) FROM posts WHERE content_nouns IS NOT NULL").Scan(&shape.Nouns); err != nil {
		t.Fatal(err)
	}
	if misplacedNouns != 0 {
		t.Errorf("%d posts carry nouns against their status", misplacedNouns)
	}

	scan("SELECT user_id, COALESCE(field, '') FROM posts", func(rows *sql.Rows) error {
		var user, field string
		if err := rows.Scan(&user, &field); err != nil {
			return err
		}
		wantField := ""
		if user == "pro" || user == "master" {
			wantField = "daily_life"
		}
		if field != wantField {
			t.Errorf("a post of %s carries 분야 %q, want %q", user, field, wantField)
		}
		if field != "" {
			shape.Fields[user]++
		}
		return nil
	})

	// GEN-53: three candidates exactly on pro's and master's review posts.
	scan("SELECT user_id, status, replacement_candidates FROM posts WHERE replacement_candidates IS NOT NULL", func(rows *sql.Rows) error {
		var user, status, encoded string
		if err := rows.Scan(&user, &status, &encoded); err != nil {
			return err
		}
		var candidates []struct {
			Surface string `json:"surface"`
		}
		if err := json.Unmarshal([]byte(encoded), &candidates); err != nil {
			return err
		}
		if status != "review" || (user != "pro" && user != "master") || len(candidates) != 3 {
			t.Errorf("a %s post of %s carries %d candidates", status, user, len(candidates))
		}
		var surfaces []string
		for _, candidate := range candidates {
			surfaces = append(surfaces, candidate.Surface)
		}
		if strings.Join(surfaces, ",") != "title,tag,body" {
			t.Errorf("a post of %s stores candidates on %v, want title, tag and body", user, surfaces)
		}
		shape.Candidates[user]++
		return nil
	})
	if shape.Candidates["pro"] != 2 || shape.Candidates["master"] != 4 {
		t.Errorf("candidates on %v, want every review post of pro and master", shape.Candidates)
	}

	// TMPL-50: master's one template, on master's first draft and nowhere else.
	var templateID string
	scan("SELECT id, user_id, title_area FROM templates ORDER BY user_id", func(rows *sql.Rows) error {
		var id, user, titleArea string
		if err := rows.Scan(&id, &user, &titleArea); err != nil {
			return err
		}
		if titleArea == "" {
			t.Errorf("template of %s has no title area", user)
		}
		templateID = id
		shape.Templates = append(shape.Templates, user)
		return nil
	})
	if !reflect.DeepEqual(shape.Templates, []string{"master"}) {
		t.Fatalf("templates %v, want master's one", shape.Templates)
	}
	var firstDraft sql.NullString
	if err := reader.QueryRowContext(ctx, `SELECT template_id FROM posts
		WHERE user_id = 'master' AND status = 'draft' ORDER BY created_at DESC LIMIT 1`).Scan(&firstDraft); err != nil {
		t.Fatal(err)
	}
	var assigned int
	if err := reader.QueryRowContext(ctx, "SELECT COUNT(*) FROM posts WHERE template_id IS NOT NULL").Scan(&assigned); err != nil {
		t.Fatal(err)
	}
	if firstDraft.String != templateID || assigned != 1 {
		t.Errorf("master's first draft names template %v and %d posts name one, want %q on it alone",
			firstDraft, assigned, templateID)
	}

	// QUAL-42: exactly the fixture's list.
	lists := phraseRows(t, reader)
	if len(lists) != 1 || lists[0].Field != devseed.PhraseList.Field {
		t.Fatalf("phrase lists %+v, want the fixture's one", lists)
	}
	var phrases []string
	if err := json.Unmarshal([]byte(lists[0].Phrases), &phrases); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(phrases, devseed.PhraseList.Phrases) || lists[0].CorpusSize != int64(devseed.PhraseList.CorpusSize) {
		t.Errorf("phrase list holds %v of %d, want the fixture's", phrases, lists[0].CorpusSize)
	}
	// Due at once: a box with Naver keys replaces the fixture on its next pass (QUAL-42).
	if !lists[0].RefreshedAt.Valid || lists[0].NextRefreshAt != lists[0].RefreshedAt.String {
		t.Errorf("phrase list refreshed %v and due %q, want due the moment it was written",
			lists[0].RefreshedAt, lists[0].NextRefreshAt)
	}
	shape.Phrases = []string{lists[0].Field}

	// GUIDE-34: the preset is the product's own and every guideline is an owner's; a seed
	// writes neither.
	var guidelineRows int
	if err := reader.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM guidelines)
		+ (SELECT COUNT(*) FROM guideline_presets) + (SELECT COUNT(*) FROM guideline_preset_fields)`).Scan(&guidelineRows); err != nil {
		t.Fatal(err)
	}
	if guidelineRows != 0 {
		t.Errorf("the seed wrote %d guideline or preset rows, want none", guidelineRows)
	}
	return shape
}

func TestSeedWritesThePublishedQualityFixtureAndIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "seed.db")
	seed(t, path)
	reader := open(t, path)
	first := checkSeed(t, reader)
	list := phraseRows(t, reader)

	seed(t, path)
	second := checkSeed(t, reader)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("a second seed wrote %+v, the first %+v", second, first)
	}
	// The second run found the list and left it, clock readings included.
	if after := phraseRows(t, reader); !reflect.DeepEqual(after, list) {
		t.Fatalf("the phrase list was rewritten: %+v, then %+v", list, after)
	}
}

// ARCH-44: a list already there — here edited the way a real batch would replace it — stands.
func TestSeedLeavesAnExistingPhraseListStanding(t *testing.T) {
	path := filepath.Join(t.TempDir(), "seed.db")
	seed(t, path)
	handle, err := db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.Exec(`UPDATE field_phrase_lists SET phrases = '["다른 문구"]'`); err != nil {
		t.Fatal(err)
	}
	collected := phraseRows(t, handle.Reader)
	handle.Close()

	seed(t, path)
	if after := phraseRows(t, open(t, path)); !reflect.DeepEqual(after, collected) {
		t.Fatalf("the seed replaced a standing list: %+v, want %+v", after, collected)
	}
}

// The report is what an operator reads: a published column, and whether the list went in or
// stood.
func TestPrintReportShowsThePublishedColumnAndThePhraseList(t *testing.T) {
	for _, written := range []bool{true, false} {
		var out bytes.Buffer
		printReport(&out, devseed.Report{
			Accounts: []devseed.AccountReport{{
				LoginID: "master", Password: devseed.Password, Plan: plan.Master,
				Drafts: 5, Reviews: 4, Finalized: 3, Published: 11,
			}},
			PhraseListWritten: written,
		}, "seed.db")
		lines := strings.Split(out.String(), "\n")
		header, row := strings.Fields(lines[3]), strings.Fields(lines[4])
		if !reflect.DeepEqual(header, []string{"LOGIN", "PLAN", "POSTS", "DRAFT", "REVIEW", "FINAL", "PUBL"}) {
			t.Fatalf("header %v", header)
		}
		if !reflect.DeepEqual(row, []string{"master", "master", "23", "5", "4", "3", "11"}) {
			t.Fatalf("row %v", row)
		}
		want := "phrase list daily_life: left standing"
		if written {
			want = "phrase list daily_life: written"
		}
		if !strings.Contains(out.String(), want) {
			t.Fatalf("report %q does not say %q", out.String(), want)
		}
	}
}
