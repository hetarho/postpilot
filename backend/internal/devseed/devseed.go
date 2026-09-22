// Package devseed replaces a local installation's account data with a fixed set of test
// accounts. It exists so `pnpm dev --seed` produces the same five accounts, the same posts
// and the same plan spread on every machine and after every reset, which is what makes a
// screen reviewable: "the list is wrong" means something only when everyone's list holds
// the same rows.
//
// It is NOT an operator tool and never ships. Only `cmd/seed` calls it, the production
// image builds `./cmd/api` alone, so the binary on the box carries no path that can
// delete every account. That is the whole reason this is a separate command rather than
// an `api seed` subcommand beside `api adduser` — a destructive fixture loader reachable
// from the deployed ENTRYPOINT is one mistyped argument away from being an outage.
//
// What it deletes is bounded on purpose: account-owned rows, which the schema cascades
// from `users`, plus the one staging table that deliberately holds no account foreign key.
// Installation-wide curation — the registered models and their purposes — is left alone.
// OpenRouter supplies the candidate list again on its own, but WHICH model serves which
// purpose is the operator's own choice in /admin, and a seed that erased it would leave a
// fresh install unable to generate anything until someone re-registered five models by hand.
package devseed

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/postpilot/backend/internal/plan"
)

// Accounts is the auth context's half of a seed: emptying the installation, then
// establishing each fixture account with its plan and its verified address.
//
// DeleteAll is the only destructive call in any of these ports, and it is deliberately
// all-or-nothing rather than a list of ids: a seed that deleted only the accounts it was
// about to write would leave whatever an earlier seed or a manual `adduser` had created,
// and "the same five accounts" would stop being true on the second run.
type Accounts interface {
	DeleteAll(ctx context.Context) (int64, error)
	Create(ctx context.Context, loginID, password string, tier plan.Plan) error
	// VerifyEmail attaches an already-verified address. The fixture skips the mail round
	// trip because there is no inbox to read in a dev stack, and an account stuck at
	// "verify your email" cannot reach any of the screens the seed exists to fill.
	VerifyEmail(ctx context.Context, loginID, email string, at time.Time) error
}

// Voices is the voice context's half. An account cannot hold a post without a default
// voice ([I4]: every post selects exactly one), so this runs before any article is written.
type Voices interface {
	EnsureDefault(ctx context.Context, userID string) (voiceID string, err error)
}

// Credits is the ledger's half: the monthly grant the account's plan entitles it to. It is
// the same call `adduser` makes, so a seeded account's balance screen shows what a really
// provisioned account of that tier would show.
type Credits interface {
	OpenMonthlyLot(ctx context.Context, userID string, tier plan.Plan) error
}

// Posts is the drafting context's half. It takes a whole Article rather than a draft to
// patch: a seeded post is one decision with a known outcome, not the autosave sequence a
// real one accumulates, and the adapter is what knows that a finalized post is assembled
// from a create, a generated baseline and a finalization.
type Posts interface {
	Write(ctx context.Context, article Article) error
}

// Media clears source staging. `clip_source_batches` carries a user_id but deliberately no
// foreign key to `users` (migration 0036: cleanup identities must outlive their project),
// so it is the one account-owned table `DELETE FROM users` does not reach and the only
// reason this port exists.
type Media interface {
	DeleteAllSourceBatches(ctx context.Context) (int64, error)
}

// Deps are the context halves a seed needs. Every one is required (ARCH-40): a seed that
// silently skipped voices would produce five accounts that cannot open the post editor,
// and the failure would surface as an empty screen rather than as this error.
type Deps struct {
	Accounts Accounts
	Voices   Voices
	Credits  Credits
	Posts    Posts
	Media    Media
	// Now is the clock the fixture dates itself against. Injected so a test can pin the
	// spread of created_at values it asserts on.
	Now func() time.Time
}

// Report is what the command prints. It is returned rather than logged from inside so the
// caller owns the output format, and so a test can assert on what was written without
// parsing a log line.
type Report struct {
	DeletedAccounts int64
	DeletedBatches  int64
	Accounts        []AccountReport
}

// AccountReport is one seeded account as the operator sees it: the id and password to log
// in with, and how many posts sit behind it in each status.
type AccountReport struct {
	LoginID   string
	Password  string
	Email     string
	Plan      plan.Plan
	Drafts    int
	Reviews   int
	Finalized int
}

// Posts is the account's total, for the summary line.
func (r AccountReport) Posts() int { return r.Drafts + r.Reviews + r.Finalized }

// Run empties the installation's account data and writes the fixture in Fixtures.
//
// It is not a transaction. SQLite gives one writer and the writes cross five contexts'
// stores, so wrapping them would mean a transaction handle threaded through every port —
// paid for by a dev tool whose recovery from a half-finished run is to run it again. A
// failure therefore reports which account it stopped on, which is the information needed
// to decide whether to rerun or to look at the data.
func Run(ctx context.Context, deps Deps) (Report, error) {
	if err := deps.valid(); err != nil {
		return Report{}, err
	}
	now := deps.Now()

	deletedAccounts, err := deps.Accounts.DeleteAll(ctx)
	if err != nil {
		return Report{}, fmt.Errorf("delete existing accounts: %w", err)
	}
	// After the accounts, not before: the cascade removes the batches' owners, and doing
	// this first would leave a window where a live batch has no account at all.
	deletedBatches, err := deps.Media.DeleteAllSourceBatches(ctx)
	if err != nil {
		return Report{}, fmt.Errorf("clear source staging: %w", err)
	}

	report := Report{DeletedAccounts: deletedAccounts, DeletedBatches: deletedBatches}
	for _, fixture := range Fixtures {
		written, err := seedAccount(ctx, deps, fixture, now)
		if err != nil {
			return report, fmt.Errorf("seed account %q: %w", fixture.LoginID, err)
		}
		report.Accounts = append(report.Accounts, written)
	}
	return report, nil
}

// seedAccount establishes one account and everything behind it, in the order the product's
// own rules require: the account exists, then it can be verified and funded, then it has a
// voice, and only then can it hold a post.
func seedAccount(ctx context.Context, deps Deps, fixture Account, now time.Time) (AccountReport, error) {
	if err := deps.Accounts.Create(ctx, fixture.LoginID, Password, fixture.Plan); err != nil {
		return AccountReport{}, fmt.Errorf("create: %w", err)
	}
	if err := deps.Accounts.VerifyEmail(ctx, fixture.LoginID, fixture.Email(), now); err != nil {
		return AccountReport{}, fmt.Errorf("verify email: %w", err)
	}
	if err := deps.Credits.OpenMonthlyLot(ctx, fixture.LoginID, fixture.Plan); err != nil {
		return AccountReport{}, fmt.Errorf("open monthly grant: %w", err)
	}
	voiceID, err := deps.Voices.EnsureDefault(ctx, fixture.LoginID)
	if err != nil {
		return AccountReport{}, fmt.Errorf("default voice: %w", err)
	}

	for _, article := range fixture.Articles(voiceID, now) {
		if err := deps.Posts.Write(ctx, article); err != nil {
			return AccountReport{}, fmt.Errorf("write post %q: %w", article.Title, err)
		}
	}
	return AccountReport{
		LoginID:   fixture.LoginID,
		Password:  Password,
		Email:     fixture.Email(),
		Plan:      fixture.Plan,
		Drafts:    fixture.Drafts,
		Reviews:   fixture.Reviews,
		Finalized: fixture.Finalized,
	}, nil
}

func (d Deps) valid() error {
	missing := ""
	switch {
	case d.Accounts == nil:
		missing = "accounts"
	case d.Voices == nil:
		missing = "voices"
	case d.Credits == nil:
		missing = "credits"
	case d.Posts == nil:
		missing = "posts"
	case d.Media == nil:
		missing = "media"
	case d.Now == nil:
		missing = "clock"
	}
	if missing != "" {
		return errors.New("devseed: " + missing + " collaborator is required")
	}
	return nil
}
