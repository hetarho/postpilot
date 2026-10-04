// Command seed replaces a local installation's account data with the fixed test fixture
// in internal/devseed. It is what `pnpm dev --seed` runs.
//
// It is a separate binary from `api` on purpose, and that is a safety property rather
// than a layout preference: the production image builds `./cmd/api` alone, so a command
// that deletes every account simply does not exist on the box. Operator subcommands that
// SHOULD exist there (`adduser`, `setplan`, `grantcredits`) stay dispatched from cmd/api;
// this one must not join them.
//
// Run it through `pnpm dev --seed`, which stops the api first. Running it by hand while
// the api is up means two processes contending for SQLite's file lock, and the seed will
// fail on SQLITE_BUSY after the five-second timeout rather than wait its turn —
// `docker compose --profile dev stop backend` first if you need to.
//
// This file is a composition root like cmd/api's: it reads the environment, opens the
// database, runs migrations and adapts each context's store to the port devseed declares.
// The fixture itself — how many accounts, which plans, how many posts — lives in devseed
// and none of it is decided here.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/postpilot/backend/internal/auth"
	authstore "github.com/postpilot/backend/internal/auth/store"
	"github.com/postpilot/backend/internal/billing"
	billingstore "github.com/postpilot/backend/internal/billing/store"
	"github.com/postpilot/backend/internal/clip"
	clipstore "github.com/postpilot/backend/internal/clip/store"
	"github.com/postpilot/backend/internal/devseed"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/mail"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/config"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/post"
	poststore "github.com/postpilot/backend/internal/post/store"
	"github.com/postpilot/backend/internal/quality"
	"github.com/postpilot/backend/internal/template"
	templatestore "github.com/postpilot/backend/internal/template/store"
	"github.com/postpilot/backend/internal/usage"
	usagestore "github.com/postpilot/backend/internal/usage/store"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn})))
	if err := run(context.Background()); err != nil {
		slog.Error("seed failed", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	// The environment is read here, in the composition root, and handed to the command
	// (ARCH-6): nothing under internal/ learns where the database is by itself.
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	handle, err := db.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer handle.Close()

	// The seed runs against a schema it can trust, and on a fresh volume it is the first
	// thing to touch the file at all — so it migrates rather than assuming the api has
	// already booted once.
	if err := db.Migrate(ctx, handle.Writer); err != nil {
		return err
	}

	authSvc := auth.NewService(authstore.New(handle.Writer, handle.Reader), cfg.SessionTTL, auth.Deps{Mailer: mail.NewLog()})
	boundary := anchors{auth: authSvc}
	rates := usage.NewRateSelector(noRates{}, usagestore.New(handle.Writer, handle.Reader))
	benefitStore := billingstore.New(handle.Writer, handle.Reader)
	benefitStore.SetCreditsForTx(func(tx *sql.Tx) billing.Credits {
		return seedBenefitCredits{Service: usage.NewService(usagestore.NewTx(tx), noModels{}, 0, boundary, rates), exports: clipstore.NewTx(tx)}
	})
	benefitStore.SetPlansForTx(func(tx *sql.Tx) billing.Plans {
		return auth.NewService(authstore.NewTx(tx), cfg.SessionTTL, auth.Deps{Mailer: mail.NewLog()})
	})
	support := billing.NewService(benefitStore, nil, nil, nil, nil, nil)
	report, err := devseed.Run(ctx, devseed.Deps{
		Accounts: accounts{svc: authSvc, store: authstore.New(handle.Writer, handle.Reader)},
		Credits:  credits{support: support},
		Posts:    posts{store: poststore.New(handle.Writer, handle.Reader)},
		Media:    media{store: clipstore.New(handle.Writer, handle.Reader)},
		Templates: templates{svc: template.NewService(
			templatestore.New(handle.Writer, handle.Reader),
			// cmd/api's templateLimits, so the fixture is held to the same ceilings a real save is.
			// Every value is named, not a literal: a zero bound added later fails NewService's
			// check at seed start, which main_test.go catches.
			template.NewLimits(template.Ceilings(cfg.Template), template.NumberBounds{
				TargetLengthMin: post.TargetLengthMin, TargetLengthMax: post.TargetLengthMax, TagCountMin: post.TagCountRange.Min, TagCountMax: post.TagCountRange.Max,
			}),
		)},
		Now: time.Now,
	})
	if err != nil {
		return err
	}
	printReport(os.Stdout, report, cfg.DBPath)
	return nil
}

// accounts adapts the auth context. Creation goes through the service because hashing the
// password is its rule, while the wipe is a store operation with no use-case above it.
type accounts struct {
	svc   *auth.Service
	store *authstore.Store
}

func (a accounts) DeleteAll(ctx context.Context) (int64, error) {
	return a.store.DeleteAllUsers(ctx)
}

func (a accounts) Create(ctx context.Context, loginID, password string, tier plan.Plan) error {
	return a.svc.CreateUser(ctx, loginID, password, tier)
}

// credits adapts billing's support assignment, the call `adduser` and `setplan` make: it opens
// the tier's coverage (daily and monthly grants, server-export window) so a seeded account's
// balance is the one its plan entitles it to rather than a number invented here.
type credits struct{ support *billing.Service }

func (c credits) OpenBenefits(ctx context.Context, userID string, tier plan.Plan) error {
	if tier == plan.Free || tier == plan.Master {
		return nil
	}
	return c.support.AssignSupportTier(ctx, userID, tier)
}

// anchors is the ledger's monthly boundary. The seed creates no subscriptions, so the
// account's own creation date is the only anchor there could be.
type anchors struct{ auth *auth.Service }

func (a anchors) AnchorFor(ctx context.Context, userID string) (time.Time, error) {
	return a.auth.CreatedAt(ctx, userID)
}
func (a anchors) CoverageFor(context.Context, string, time.Time) (usage.Coverage, bool, error) {
	return usage.Coverage{}, false, nil
}

type seedBenefitCredits struct {
	*usage.Service
	exports clip.ExportWindows
}

func (c seedBenefitCredits) OpenCoverage(ctx context.Context, userID string, coverage billing.Coverage, at time.Time, correlation string) error {
	if err := c.Service.OpenCoverage(ctx, userID, seedCoverage(coverage), at, correlation); err != nil {
		return err
	}
	window, _ := usage.ExportWindowAt(userID, seedCoverage(coverage), at)
	return c.exports.OpenExportWindow(ctx, seedExportWindow(window), correlation)
}
func (c seedBenefitCredits) AddUpgradeBonus(ctx context.Context, userID string, coverage billing.Coverage, at time.Time, credits, exportDelta int, correlation string) error {
	if err := c.Service.AddUpgradeBonus(ctx, userID, seedCoverage(coverage), at, credits, correlation); err != nil {
		return err
	}
	window, _ := usage.ExportWindowAt(userID, seedCoverage(coverage), at)
	return c.exports.RaiseExportWindow(ctx, seedExportWindow(window), exportDelta, correlation)
}

// seedCoverage and seedExportWindow translate between the contexts the way the API's
// billing adapter does; the window itself is the ledger's rule (usage.ExportWindowAt).
func seedCoverage(coverage billing.Coverage) usage.Coverage {
	return usage.Coverage{ID: coverage.ID, Anchor: coverage.Anchor, End: coverage.End, Tier: coverage.Tier, DailyTier: coverage.DailyTier}
}
func seedExportWindow(window usage.ExportWindow) clip.ExportWindow {
	return clip.ExportWindow{UserID: window.UserID, CoverageID: window.CoverageID, Start: window.Start, End: window.End, Allowance: window.Allowance}
}

// noModels stands in for the model registry. The ledger only consults it to price a call,
// and a seed makes none.
type noModels struct{}

func (noModels) Lookup(llm.ModelRef) (llm.ModelInfo, bool) { return llm.ModelInfo{}, false }

// noRates stands in for the official exchange-rate source for the same reason: a seed prices
// nothing, so any priced work would be refused as a rate outage rather than converted.
type noRates struct{}

func (noRates) KRWPerUSD(context.Context, time.Time) (int64, bool, error) {
	return 0, false, usage.ErrRateUnavailable
}

// posts adapts the drafting context, mapping one fixture Article to the sequence of writes
// that actually produces a post in that state. This is the anti-corruption mapping at the
// edge: devseed says what the installation should look like, and knowing that a finalized
// post is a create plus a generated baseline plus a finalization belongs here.
//
// It writes through the store rather than post.Service because the service's collaborators
// are a job finder, an experiment fence, a content purger and two detachers — every one of
// which guards a transition a seed does not make. Wiring five fences to write a row whose
// content this file authored would be ceremony, not safety.
type posts struct{ store *poststore.Store }

func (p posts) Write(ctx context.Context, article devseed.Article) error {
	language, err := post.ParseLanguage(article.Language)
	if err != nil {
		return err
	}
	slug, err := p.mintSlug(ctx, article)
	if err != nil {
		return err
	}
	created := post.Post{
		Slug:           slug,
		UserID:         article.UserID,
		Title:          article.Title,
		Memo:           article.Memo,
		TargetLanguage: language,
		CreatedAt:      article.CreatedAt,
		UpdatedAt:      article.CreatedAt,
	}
	if err := p.store.CreatePost(ctx, created); err != nil {
		return err
	}
	// The 분야 and the template come before any content, since every write after the publish
	// below is refused on a published post (POST-86).
	if article.Field != "" {
		if !quality.Known(article.Field) {
			return fmt.Errorf("unknown blog field %q", article.Field)
		}
		assigned, err := p.store.AssignField(ctx, slug, article.UserID, &article.Field, article.CreatedAt)
		if err != nil {
			return err
		}
		if !assigned {
			return fmt.Errorf("blog field for %q changed nothing", slug)
		}
	}
	if article.TemplateID != "" {
		assigned, err := p.store.AssignTemplate(ctx, slug, article.UserID, &article.TemplateID, post.TemplateNumbers{}, article.CreatedAt)
		if err != nil {
			return err
		}
		if !assigned {
			return fmt.Errorf("template for %q changed nothing", slug)
		}
	}
	if article.Content == nil {
		// A draft is finished here: NULL content is what "never generated" means.
		return nil
	}

	content := mapContent(article.Title, *article.Content)
	// The fixture's own guard. ValidateContent is the rule the editor and the generation
	// pipeline both apply, and applying it here means a malformed fixture fails the seed
	// instead of producing a post no screen can render. A seeded post has no attachments,
	// so no block may name a file.
	if err := post.ValidateContent(content, nil, nil); err != nil {
		return err
	}
	// The nouns a write returns beside its content, stored apart from it the way a real write
	// stores them (GEN-55).
	annotations := post.WriteAnnotations{Nouns: article.Content.Nouns}
	written, err := p.store.UpdateGeneratedContent(ctx, slug, article.UserID, content, language, annotations, article.CreatedAt)
	if err != nil {
		return err
	}
	if !written {
		return fmt.Errorf("generated content for %q changed nothing", slug)
	}
	if article.Status != devseed.StatusFinalized && article.Status != devseed.StatusPublished {
		return nil
	}

	// The revision is read back rather than assumed: finalization is fenced on the exact
	// revision the baseline write landed at, and hardcoding 1 here would make this adapter
	// break the day anything else writes to a post before it is finalized.
	stored, err := p.store.GetPost(ctx, slug)
	if err != nil {
		return err
	}
	finalized, err := p.store.Finalize(ctx, slug, article.UserID, content.Title, stored.ContentRevision, article.CreatedAt)
	if err != nil {
		return err
	}
	if !finalized {
		return fmt.Errorf("finalize %q was refused at revision %d", slug, stored.ContentRevision)
	}
	if article.Status != devseed.StatusPublished {
		return nil
	}

	// The post context's own address rule is the fixture's guard, and what it returns is what
	// a real paste would store.
	address, err := post.ParseNaverBlogURL(article.PublishedURL)
	if err != nil {
		return err
	}
	published, err := p.store.PublishPost(ctx, slug, article.UserID, address, article.PublishedAt)
	if err != nil {
		return err
	}
	if !published {
		return fmt.Errorf("publish %q was refused", slug)
	}
	return nil
}

// mintSlug asks the post context for the slug its own rule produces, so a seeded post's
// URL is indistinguishable from a real one's — including the serial suffix two posts with
// the same title on the same day get.
func (p posts) mintSlug(ctx context.Context, article devseed.Article) (string, error) {
	var lookupErr error
	slug := post.MintSlug(article.CreatedAt.Format("20060102"), article.Title, func(candidate string) bool {
		taken, err := p.store.SlugExists(ctx, candidate)
		if err != nil {
			lookupErr = err
			// Reporting "taken" stops the search at a lookup failure rather than letting
			// it loop, and the error below is what the caller sees.
			return true
		}
		return taken
	})
	if lookupErr != nil {
		return "", lookupErr
	}
	return slug, nil
}

func mapContent(fallbackTitle string, content devseed.Content) post.PostContent {
	blocks := make([]post.Block, 0, len(content.Blocks))
	for _, block := range content.Blocks {
		blocks = append(blocks, post.Block{
			Type:    post.BlockType(block.Type),
			Content: block.Content,
			Level:   block.Level,
			Items:   block.Items,
		})
	}
	title := content.Title
	if title == "" {
		title = fallbackTitle
	}
	return post.PostContent{Title: title, Summary: content.Summary, Tags: content.Tags, Blocks: blocks}
}

// templates adapts the template context through its service, so the fixture's title area and
// body pass the same parse and limits a save from the builder does (TMPL-50).
type templates struct{ svc *template.Service }

func (t templates) Create(ctx context.Context, fixture devseed.Template) (string, error) {
	created, err := t.svc.Create(ctx, fixture.UserID, template.Authored{Name: fixture.Name, Description: fixture.Description, Body: fixture.Body, TitleArea: fixture.TitleArea})
	if err != nil {
		return "", err
	}
	return created.ID, nil
}

// media adapts the clip context's source staging, the one account-owned table the cascade
// from `users` does not reach.
type media struct{ store *clipstore.Store }

func (m media) DeleteAllSourceBatches(ctx context.Context) (int64, error) {
	return m.store.DeleteAllSourceBatches(ctx)
}

// printReport is the command's output. It goes to stdout rather than the logger because it is the
// answer to "what can I log in as now", and it is the only reason someone reads this
// command's output at all.
func printReport(out io.Writer, report devseed.Report, dbPath string) {
	fmt.Fprintf(out, "seeded %s\n", dbPath)
	fmt.Fprintf(out, "  removed %d account(s) and %d source batch(es)\n\n", report.DeletedAccounts, report.DeletedBatches)
	fmt.Fprintf(out, "  %-14s %-8s %-6s %-7s %-7s %-7s %s\n", "LOGIN", "PLAN", "POSTS", "DRAFT", "REVIEW", "FINAL", "PUBL")
	for _, account := range report.Accounts {
		fmt.Fprintf(out, "  %-14s %-8s %-6d %-7d %-7d %-7d %d\n",
			account.LoginID, account.Plan, account.Posts(), account.Drafts, account.Reviews, account.Finalized, account.Published)
	}
	if len(report.Accounts) > 0 {
		fmt.Fprintf(out, "\n  password for every account: %s\n", report.Accounts[0].Password)
	}
}
