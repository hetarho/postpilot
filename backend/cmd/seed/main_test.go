package main

import (
	"bytes"
	"context"
	"database/sql"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/auth"
	authstore "github.com/postpilot/backend/internal/auth/store"
	"github.com/postpilot/backend/internal/billing"
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/devseed"
	"github.com/postpilot/backend/internal/mail"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/usage"
	usagestore "github.com/postpilot/backend/internal/usage/store"
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
		{"free", plan.Free, 0}, {"light", plan.Light, 1}, {"base", plan.Basic, 3}, {"pro", plan.Pro, 8},
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

// A seeded paid account holds what a provisioned one of its tier holds: the coverage's daily
// and monthly grants and its server-export window, opened through billing's support
// assignment; free and master accounts get no paid benefits.
func TestSeedOpensEachPaidAccountsCoverageBenefits(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "seed-benefits.db")
	seed(t, path)
	reader := open(t, path)
	for _, fixture := range devseed.Fixtures {
		var daily, monthly, exports, coverages int
		if err := reader.QueryRowContext(ctx, `SELECT
			COALESCE(SUM(CASE WHEN kind = 'daily' THEN granted END), 0),
			COALESCE(SUM(CASE WHEN kind = 'monthly' THEN granted END), 0)
			FROM credit_lots WHERE user_id = ?`, fixture.LoginID).Scan(&daily, &monthly); err != nil {
			t.Fatal(err)
		}
		if err := reader.QueryRowContext(ctx, "SELECT COALESCE(SUM(allowance), 0) FROM server_export_windows WHERE user_id = ?",
			fixture.LoginID).Scan(&exports); err != nil {
			t.Fatal(err)
		}
		if err := reader.QueryRowContext(ctx, "SELECT COUNT(*) FROM support_coverages WHERE user_id = ?",
			fixture.LoginID).Scan(&coverages); err != nil {
			t.Fatal(err)
		}
		if fixture.Plan == plan.Free || fixture.Plan == plan.Master {
			if daily != 0 || monthly != 0 || exports != 0 || coverages != 0 {
				t.Errorf("%s holds paid benefits: daily=%d monthly=%d exports=%d coverages=%d", fixture.LoginID, daily, monthly, exports, coverages)
			}
			continue
		}
		offer, ok := plan.CommercialOffer(fixture.Plan)
		if !ok || daily != offer.DailyCredits || monthly != offer.MonthlyBonus || exports != offer.ServerExports || coverages != 1 {
			t.Errorf("%s benefits daily=%d monthly=%d exports=%d coverages=%d, want the %s offer %+v",
				fixture.LoginID, daily, monthly, exports, coverages, fixture.Plan, offer)
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

// seedShape is what a seed wrote, without the ids and clock readings a rerun renews.
type seedShape struct {
	Statuses  map[string]int // "account/status" → posts
	Nouns     int            // posts carrying nouns
	Fields    map[string]int // account → posts carrying daily_life
	Templates []string       // "account" per template with a title area
}

// checkSeed asserts every fixture invariant the database can show and returns its shape.
func checkSeed(t *testing.T, reader *sql.DB) seedShape {
	t.Helper()
	ctx := context.Background()
	shape := seedShape{Statuses: map[string]int{}, Fields: map[string]int{}}
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
		"light/draft": 1,
		"base/draft":  2, "base/review": 1,
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

	// GUIDE-19: guidelines are never seeded; every one is an owner's, so a seed writes none.
	var guidelineRows int
	if err := reader.QueryRowContext(ctx, `SELECT COUNT(*) FROM guidelines`).Scan(&guidelineRows); err != nil {
		t.Fatal(err)
	}
	if guidelineRows != 0 {
		t.Errorf("the seed wrote %d guideline rows, want none", guidelineRows)
	}
	return shape
}

func TestSeedWritesThePublishedQualityFixtureAndIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "seed.db")
	seed(t, path)
	reader := open(t, path)
	first := checkSeed(t, reader)

	seed(t, path)
	second := checkSeed(t, reader)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("a second seed wrote %+v, the first %+v", second, first)
	}
}

// The report is what an operator reads: a published column beside the other statuses.
func TestPrintReportShowsThePublishedColumn(t *testing.T) {
	var out bytes.Buffer
	printReport(&out, devseed.Report{
		Accounts: []devseed.AccountReport{{
			LoginID: "master", Password: devseed.Password, Plan: plan.Master,
			Drafts: 5, Reviews: 4, Finalized: 3, Published: 11,
		}},
	}, "seed.db")
	lines := strings.Split(out.String(), "\n")
	header, row := strings.Fields(lines[3]), strings.Fields(lines[4])
	if !reflect.DeepEqual(header, []string{"LOGIN", "PLAN", "POSTS", "DRAFT", "REVIEW", "FINAL", "PUBL"}) {
		t.Fatalf("header %v", header)
	}
	if !reflect.DeepEqual(row, []string{"master", "master", "23", "5", "4", "3", "11"}) {
		t.Fatalf("row %v", row)
	}
}

// capturedExports is clip's export windows as the seed's benefit adapter drives them.
type capturedExports struct{ opened, raised []clip.ExportWindow }

func (c *capturedExports) OpenExportWindow(_ context.Context, window clip.ExportWindow, _ string) error {
	c.opened = append(c.opened, window)
	return nil
}
func (c *capturedExports) RaiseExportWindow(_ context.Context, window clip.ExportWindow, _ int, _ string) error {
	c.raised = append(c.raised, window)
	return nil
}
func (c *capturedExports) CurrentExportWindow(context.Context, string, time.Time) (clip.ExportWindow, bool, error) {
	return clip.ExportWindow{}, false, nil
}

// QUOTA-62: the seed takes its export windows from the ledger's rule, so a coverage that ends
// inside its benefit month ends the window there, the upgrade's raise included, as production's
// billing adapter does.
func TestSeedExportWindowsEndWithTheCoverage(t *testing.T) {
	ctx := context.Background()
	handle, err := db.Open(filepath.Join(t.TempDir(), "seed-exports.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	if err := db.Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	authSvc := auth.NewService(authstore.New(handle.Writer, handle.Reader), time.Hour, auth.Deps{Mailer: mail.NewLog()})
	if err := authstore.New(handle.Writer, handle.Reader).CreateUser(ctx, auth.User{ID: "alice", PasswordHash: "hash", Plan: plan.Basic, CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	store := usagestore.New(handle.Writer, handle.Reader)
	exports := &capturedExports{}
	credits := seedBenefitCredits{Service: usage.NewService(store, noModels{}, 0, anchors{auth: authSvc}, usage.NewRateSelector(noRates{}, store)), exports: exports}
	now := time.Now()
	coverage := billing.Coverage{ID: "support:alice", Anchor: now.Add(-time.Hour), End: now.Add(24 * time.Hour), Tier: plan.Max, DailyTier: plan.Basic}
	if err := credits.OpenCoverage(ctx, "alice", coverage, now, "open"); err != nil {
		t.Fatal(err)
	}
	if err := credits.AddUpgradeBonus(ctx, "alice", coverage, now, 10, 3, "upgrade"); err != nil {
		t.Fatal(err)
	}
	if len(exports.opened) != 1 || len(exports.raised) != 1 {
		t.Fatalf("export writes = opened %+v raised %+v", exports.opened, exports.raised)
	}
	for _, window := range []clip.ExportWindow{exports.opened[0], exports.raised[0]} {
		if !window.Start.Equal(coverage.Anchor) || !window.End.Equal(coverage.End) || window.Allowance != 60 || window.CoverageID != coverage.ID {
			t.Fatalf("window = %+v, want [%s, %s) with the Max allowance", window, coverage.Anchor, coverage.End)
		}
	}
}
