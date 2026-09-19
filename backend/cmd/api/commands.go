package main

import (
	"context"
	"fmt"
	"time"

	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/auth/provision"
	authstore "github.com/postpilot/backend/internal/auth/store"
	"github.com/postpilot/backend/internal/experiment"
	"github.com/postpilot/backend/internal/mail"
	"github.com/postpilot/backend/internal/platform/config"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/usage"
	usagestore "github.com/postpilot/backend/internal/usage/store"
	"github.com/postpilot/backend/internal/voice"
	voicestore "github.com/postpilot/backend/internal/voice/store"
)

func creditBootstrap(ctx context.Context, handle *db.DB, userID string) error {
	authSvc := auth.NewService(authstore.New(handle.Writer, handle.Reader), time.Hour, auth.Deps{Mailer: mail.NewLog()})
	acting, err := authSvc.PlanOf(ctx, userID)
	if err != nil {
		return fmt.Errorf("resolve provisioned plan: %w", err)
	}
	ledger := usage.NewService(usagestore.New(handle.Writer, handle.Reader), emptyModels{}, 0, usageAnchors{auth: authSvc})
	if err := ledger.EnsureMonthlyLot(ctx, userID, acting); err != nil {
		return fmt.Errorf("open monthly grant: %w", err)
	}
	return nil
}

// topUpMonthlyLot raises an account's current monthly grant, for the upgrade half of
// `api setplan`. It is the same ledger call the RPC path gets, injected here because the
// auth context must not learn about credit_lots.
func topUpMonthlyLot(ctx context.Context, handle *db.DB, userID string, credits int) error {
	authSvc := auth.NewService(authstore.New(handle.Writer, handle.Reader), time.Hour, auth.Deps{Mailer: mail.NewLog()})
	ledger := usage.NewService(usagestore.New(handle.Writer, handle.Reader), emptyModels{}, 0, usageAnchors{auth: authSvc})
	return ledger.TopUpMonthlyLot(ctx, userID, credits)
}

// grantCreditsTo opens a bonus lot from the operator's shell.
func grantCreditsTo(ctx context.Context, handle *db.DB, userID string, credits int, expiresAt *time.Time) error {
	authSvc := auth.NewService(authstore.New(handle.Writer, handle.Reader), time.Hour, auth.Deps{Mailer: mail.NewLog()})
	ledger := usage.NewService(usagestore.New(handle.Writer, handle.Reader), emptyModels{}, 0, usageAnchors{auth: authSvc})
	return ledger.Grant(ctx, userID, credits, expiresAt)
}

// usageAnchors is the composition seam between the credit ledger, subscriptions and
// account identity. It prefers an active subscription without teaching either context
// about the other's persistence.
func defaultVoiceBootstrap(ctx context.Context, handle *db.DB, userID string) error {
	directory := voice.NewService(voicestore.New(handle.Writer, handle.Reader), nil, nil)
	_, _, err := directory.EnsureDefaultVoice(ctx, userID, voice.LanguageKorean)
	return err
}

var _ experiment.Runner = experimentRunner{}

// meteredRegistry is the model registry every context above the llm port is given. It is
// the registry, unchanged, except that each completed call is written to the account
// ledger — so "every server-side LLM call is metered" holds by construction rather than by
// each caller remembering to record one.

// runCommand dispatches the operator subcommands that share the production image
// (`docker compose run --rm api adduser <id>`), so the binary ships one ENTRYPOINT with
// nothing added to it. It reports whether a subcommand ran.
func runCommand(args []string) bool {
	if len(args) == 0 {
		return false
	}
	ctx := context.Background()
	// The environment is read HERE, in the composition root, and handed to the command
	// (ARCH-6): nothing under `internal/` learns where the database is by itself.
	cfg, err := config.Load()
	if err != nil {
		fatal(args[0], err)
	}
	settings := provision.Settings{DBPath: cfg.DBPath, SessionTTL: cfg.SessionTTL}
	switch args[0] {
	case "adduser":
		if err := provision.Run(ctx, settings, args[1:], defaultVoiceBootstrap, creditBootstrap); err != nil {
			fatal("adduser", err)
		}
	case "grantcredits":
		if err := provision.GrantCredits(ctx, settings, args[1:], grantCreditsTo); err != nil {
			fatal("grantcredits", err)
		}
	case "setplan":
		if err := provision.SetPlan(ctx, settings, args[1:], topUpMonthlyLot); err != nil {
			fatal("setplan", err)
		}
	default:
		return false
	}
	return true
}
