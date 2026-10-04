package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/auth/provision"
	authstore "github.com/postpilot/backend/internal/auth/store"
	"github.com/postpilot/backend/internal/billing"
	billingstore "github.com/postpilot/backend/internal/billing/store"
	clipstore "github.com/postpilot/backend/internal/clip/store"
	"github.com/postpilot/backend/internal/experiment"
	"github.com/postpilot/backend/internal/mail"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/config"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/usage"
	usagestore "github.com/postpilot/backend/internal/usage/store"
)

func creditBootstrap(ctx context.Context, handle *db.DB, userID string) error {
	authSvc := auth.NewService(authstore.New(handle.Writer, handle.Reader), time.Hour, auth.Deps{Mailer: mail.NewLog()})
	acting, err := authSvc.PlanOf(ctx, userID)
	if err != nil {
		return fmt.Errorf("resolve provisioned plan: %w", err)
	}
	if acting == plan.Free {
		return nil
	}
	if err := assignSupportPlan(ctx, handle, userID, acting); err != nil {
		return fmt.Errorf("open support benefits: %w", err)
	}
	return nil
}

func assignSupportPlan(ctx context.Context, handle *db.DB, userID string, target plan.Plan) error {
	authSvc := auth.NewService(authstore.New(handle.Writer, handle.Reader), time.Hour, auth.Deps{Mailer: mail.NewLog()})
	anchors := usageAnchors{auth: authSvc}
	store := billingstore.New(handle.Writer, handle.Reader)
	store.SetCreditsForTx(func(tx *sql.Tx) billing.Credits {
		return billingCredits{Service: usage.NewService(usagestore.NewTx(tx), emptyModels{}, 0, anchors, shellRates(handle)), exports: clipstore.NewTx(tx)}
	})
	store.SetPlansForTx(func(tx *sql.Tx) billing.Plans {
		return auth.NewService(authstore.NewTx(tx), time.Hour, auth.Deps{Mailer: mail.NewLog()})
	})
	service := billing.NewService(store, nil, nil, nil, nil, nil, nil)
	return service.AssignSupportTier(ctx, userID, target)
}

// shellRates is the rate selector of an operator command's ledger. Those commands grant and
// renew but price nothing, so it never consults the official source: any priced work would be
// refused as a rate outage rather than converted.
func shellRates(handle *db.DB) *usage.RateSelector {
	return usage.NewRateSelector(unavailableRateSource{}, usagestore.New(handle.Writer, handle.Reader))
}

// topUpMonthlyLot raises an account's current monthly grant, for the upgrade half of
// `api setplan`. It is the same ledger call the RPC path gets, injected here because the
// auth context must not learn about credit_lots.
func topUpMonthlyLot(ctx context.Context, handle *db.DB, userID string, credits int) error {
	authSvc := auth.NewService(authstore.New(handle.Writer, handle.Reader), time.Hour, auth.Deps{Mailer: mail.NewLog()})
	ledger := usage.NewService(usagestore.New(handle.Writer, handle.Reader), emptyModels{}, 0, usageAnchors{auth: authSvc}, shellRates(handle))
	return ledger.TopUpMonthlyLot(ctx, userID, credits)
}

// grantCreditsTo opens a bonus lot from the operator's shell.
func grantCreditsTo(ctx context.Context, handle *db.DB, userID string, credits int, expiresAt *time.Time) error {
	authSvc := auth.NewService(authstore.New(handle.Writer, handle.Reader), time.Hour, auth.Deps{Mailer: mail.NewLog()})
	ledger := usage.NewService(usagestore.New(handle.Writer, handle.Reader), emptyModels{}, 0, usageAnchors{auth: authSvc}, shellRates(handle))
	return ledger.Grant(ctx, userID, credits, expiresAt)
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
	if args[0] == "media-manifest" {
		if len(args) != 1 {
			fatal("media-manifest", fmt.Errorf("usage: api media-manifest"))
		}
		if err := mediaManifest(ctx, os.Stdout); err != nil {
			fatal("media-manifest", err)
		}
		return true
	}
	// The environment is read HERE, in the composition root, and handed to the command
	// (ARCH-6): nothing under `internal/` learns where the database is by itself.
	cfg, err := config.Load()
	if err != nil {
		fatal(args[0], err)
	}
	settings := provision.Settings{DBPath: cfg.DBPath, SessionTTL: cfg.SessionTTL}
	switch args[0] {
	case "reset-test-entitlements":
		if err := runTestEntitlementReset(ctx, cfg.DBPath, args[1:], os.Stdout); err != nil {
			fatal("reset-test-entitlements", err)
		}
	case "adduser":
		if err := provision.Run(ctx, settings, args[1:], creditBootstrap); err != nil {
			fatal("adduser", err)
		}
	case "grantcredits":
		if err := provision.GrantCredits(ctx, settings, args[1:], grantCreditsTo); err != nil {
			fatal("grantcredits", err)
		}
	case "setplan":
		if err := provision.SetPlanAssigned(ctx, settings, args[1:], assignSupportPlan); err != nil {
			fatal("setplan", err)
		}
	default:
		return false
	}
	return true
}
