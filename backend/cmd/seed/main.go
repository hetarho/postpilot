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
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/postpilot/backend/internal/auth"
	authstore "github.com/postpilot/backend/internal/auth/store"
	clipstore "github.com/postpilot/backend/internal/clip/store"
	"github.com/postpilot/backend/internal/devseed"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/mail"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/config"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/post"
	poststore "github.com/postpilot/backend/internal/post/store"
	"github.com/postpilot/backend/internal/usage"
	usagestore "github.com/postpilot/backend/internal/usage/store"
	"github.com/postpilot/backend/internal/voice"
	voicestore "github.com/postpilot/backend/internal/voice/store"
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
	report, err := devseed.Run(ctx, devseed.Deps{
		Accounts: accounts{svc: authSvc, store: authstore.New(handle.Writer, handle.Reader)},
		Voices:   voices{svc: voice.NewService(voicestore.New(handle.Writer, handle.Reader), nil, nil)},
		Credits: credits{ledger: usage.NewService(
			usagestore.New(handle.Writer, handle.Reader),
			noModels{},
			0,
			anchors{auth: authSvc},
		)},
		Posts: posts{store: poststore.New(handle.Writer, handle.Reader)},
		Media: media{store: clipstore.New(handle.Writer, handle.Reader)},
		Now:   time.Now,
	})
	if err != nil {
		return err
	}
	printReport(report, cfg.DBPath)
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

// voices adapts the voice context. The service is built without a model port or a job queue
// because establishing an account's default voice needs neither — it is a row, not a
// learning run, and a seeded voice holds no profile.
type voices struct{ svc *voice.Service }

func (v voices) EnsureDefault(ctx context.Context, userID string) (string, error) {
	created, _, err := v.svc.EnsureDefaultVoice(ctx, userID, voice.LanguageKorean)
	if err != nil {
		return "", err
	}
	return created.ID, nil
}

// credits adapts the ledger, making the same call `adduser` makes so a seeded account's
// balance is the one its plan entitles it to rather than a number invented here.
type credits struct{ ledger *usage.Service }

func (c credits) OpenMonthlyLot(ctx context.Context, userID string, tier plan.Plan) error {
	return c.ledger.EnsureMonthlyLot(ctx, userID, tier)
}

// anchors is the ledger's monthly boundary. The seed creates no subscriptions, so the
// account's own creation date is the only anchor there could be.
type anchors struct{ auth *auth.Service }

func (a anchors) AnchorFor(ctx context.Context, userID string) (time.Time, error) {
	return a.auth.CreatedAt(ctx, userID)
}

// noModels stands in for the model registry. The ledger only consults it to price a call,
// and a seed makes none.
type noModels struct{}

func (noModels) Lookup(llm.ModelRef) (llm.ModelInfo, bool) { return llm.ModelInfo{}, false }

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
		VoiceID:        article.VoiceID,
		Title:          article.Title,
		Memo:           article.Memo,
		TargetLanguage: language,
		CreatedAt:      article.CreatedAt,
		UpdatedAt:      article.CreatedAt,
	}
	if err := p.store.CreatePost(ctx, created); err != nil {
		return err
	}
	if article.Content == nil {
		// A draft is finished at the create: NULL content is what "never generated" means.
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
	written, err := p.store.UpdateGeneratedContent(ctx, slug, article.UserID, content, language, post.WriteAnnotations{}, article.CreatedAt)
	if err != nil {
		return err
	}
	if !written {
		return fmt.Errorf("generated content for %q changed nothing", slug)
	}
	if article.Status != devseed.StatusFinalized {
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

// media adapts the clip context's source staging, the one account-owned table the cascade
// from `users` does not reach.
type media struct{ store *clipstore.Store }

func (m media) DeleteAllSourceBatches(ctx context.Context) (int64, error) {
	return m.store.DeleteAllSourceBatches(ctx)
}

// printReport is the command's output. It goes to stdout rather than the logger because it is the
// answer to "what can I log in as now", and it is the only reason someone reads this
// command's output at all.
func printReport(report devseed.Report, dbPath string) {
	fmt.Fprintf(os.Stdout, "seeded %s\n", dbPath)
	fmt.Fprintf(os.Stdout, "  removed %d account(s) and %d source batch(es)\n\n", report.DeletedAccounts, report.DeletedBatches)
	fmt.Fprintf(os.Stdout, "  %-14s %-8s %-6s %-7s %-7s %s\n", "LOGIN", "PLAN", "POSTS", "DRAFT", "REVIEW", "FINAL")
	for _, account := range report.Accounts {
		fmt.Fprintf(os.Stdout, "  %-14s %-8s %-6d %-7d %-7d %d\n",
			account.LoginID, account.Plan, account.Posts(), account.Drafts, account.Reviews, account.Finalized)
	}
	if len(report.Accounts) > 0 {
		fmt.Fprintf(os.Stdout, "\n  password for every account: %s\n", report.Accounts[0].Password)
	}
}
