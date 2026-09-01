// Package provision is the operator's account tool. postpilot has no signup RPC
// (PRD F-1), so this is the only path that creates a user — deliberately reachable
// from a shell on the box and from nowhere on the network.
package provision

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/auth/store"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/config"
	"github.com/postpilot/backend/internal/platform/db"
)

// Bootstrap runs once the user row exists. The composition root supplies it so this
// package stays inside the auth context: the account's default voice is established here
// (cmd/api, cmd/adduser), and a failure is the operator's signal that the account is not
// usable yet.
type Bootstrap func(ctx context.Context, handle *db.DB, userID string) error

// Run executes `adduser <login_id> [--plan=<free|basic|max|master>]`, returning an error
// the caller turns into a non-zero exit.
//
// It opens the database and runs migrations itself rather than assuming the api has
// already booted: on a fresh volume the very first thing an operator does is create an
// account, and requiring a running server first would be a bootstrap deadlock.
//
// The bootstraps also run when the id already exists, so a rerun repairs an account whose
// bootstrap failed the first time — without touching the password — and still exits
// non-zero with the duplicate message.
func Run(ctx context.Context, args []string, bootstraps ...Bootstrap) error {
	loginID, tier, err := parseAddUserArgs(args)
	if err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	handle, err := db.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer handle.Close()

	if err := db.Migrate(ctx, handle.Writer); err != nil {
		return err
	}

	password, err := readPassword()
	if err != nil {
		return err
	}

	svc := auth.NewService(store.New(handle.Writer, handle.Reader), cfg.SessionTTL)
	if err := svc.CreateUser(ctx, loginID, password, tier); err != nil {
		if errors.Is(err, auth.ErrDuplicateUser) {
			if bootErr := runBootstraps(ctx, handle, loginID, bootstraps); bootErr != nil {
				return fmt.Errorf("account %q already exists and its bootstrap failed: %w", loginID, bootErr)
			}
			return fmt.Errorf("account %q already exists; pick another id, or delete the row first", loginID)
		}
		return err
	}
	if err := runBootstraps(ctx, handle, loginID, bootstraps); err != nil {
		return fmt.Errorf("account %q was created but is not usable yet: %w (rerun adduser to repair)", loginID, err)
	}

	fmt.Fprintf(os.Stdout, "created account %q on the %s plan in %s\n", loginID, tier, cfg.DBPath)
	return nil
}

// parseAddUserArgs accepts the id and an optional tier in either order, so an operator
// does not have to remember which comes first.
//
// The default is `free`, not `master`: a provisioning command that hands out unlimited
// spend by omission is the failure mode this ladder exists to prevent. The operator's own
// account is promoted explicitly, with --plan=master or `setplan`.
func parseAddUserArgs(args []string) (string, plan.Plan, error) {
	const usage = "usage: adduser <login_id> [--plan=<free|basic|max|master>]"
	loginID := ""
	tier := plan.Free
	for _, arg := range args {
		value, isPlan := strings.CutPrefix(arg, "--plan=")
		switch {
		case isPlan:
			parsed, err := plan.Parse(value)
			if err != nil {
				return "", "", fmt.Errorf("%s: %w", usage, err)
			}
			tier = parsed
		case loginID != "" || strings.TrimSpace(arg) == "":
			return "", "", errors.New(usage)
		default:
			loginID = strings.TrimSpace(arg)
		}
	}
	if loginID == "" {
		return "", "", errors.New(usage)
	}
	return loginID, tier, nil
}

// SetPlan executes `setplan <login_id> <plan>`. It is the operator's path to the same
// change the master-only RPC makes, for a deployment whose last master needs promoting
// from a shell — and it enforces the same last-master guard, so neither path can lock
// administration out.
func SetPlan(ctx context.Context, args []string) error {
	if len(args) != 2 || strings.TrimSpace(args[0]) == "" {
		return errors.New("usage: setplan <login_id> <free|basic|max|master>")
	}
	loginID := strings.TrimSpace(args[0])
	target, err := plan.Parse(args[1])
	if err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	handle, err := db.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer handle.Close()
	if err := db.Migrate(ctx, handle.Writer); err != nil {
		return err
	}

	svc := auth.NewService(store.New(handle.Writer, handle.Reader), cfg.SessionTTL)
	if err := svc.SetUserPlan(ctx, loginID, target); err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			return fmt.Errorf("account %q does not exist", loginID)
		}
		return err
	}

	fmt.Fprintf(os.Stdout, "account %q is now on the %s plan\n", loginID, target)
	return nil
}

func runBootstraps(ctx context.Context, handle *db.DB, userID string, bootstraps []Bootstrap) error {
	for _, bootstrap := range bootstraps {
		if err := bootstrap(ctx, handle, userID); err != nil {
			return err
		}
	}
	return nil
}

// readPassword prompts twice and requires a match.
//
// On a TTY it reads with echo off. Piped input (`docker compose run -T`, a CI check)
// has no TTY, so it falls back to a plain read — the confirmation prompt still applies,
// which means a piped password must be sent twice.
func readPassword() (string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return readPasswordLines(os.Stdin)
	}

	fmt.Fprint(os.Stderr, "password: ")
	first, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}

	fmt.Fprint(os.Stderr, "confirm password: ")
	second, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}

	return validatePair(string(first), string(second))
}

// readPasswordLines takes two whole lines. Splitting on whitespace (fmt.Fscanln) would
// silently truncate or reject a passphrase containing a space — and a passphrase is
// exactly what an operator should be typing here.
func readPasswordLines(r io.Reader) (string, error) {
	scanner := bufio.NewScanner(r)

	if !scanner.Scan() {
		return "", fmt.Errorf("read password: %w", inputErr(scanner))
	}
	first := scanner.Text()

	if !scanner.Scan() {
		return "", fmt.Errorf("read password confirmation: %w", inputErr(scanner))
	}
	second := scanner.Text()

	return validatePair(first, second)
}

// inputErr distinguishes a read failure from input that simply ended early; both stop
// provisioning, but only one is worth an operator looking at the pipe.
func inputErr(scanner *bufio.Scanner) error {
	if err := scanner.Err(); err != nil {
		return err
	}
	return errors.New("unexpected end of input (send the password twice, one per line)")
}

func validatePair(first, second string) (string, error) {
	if first == "" {
		return "", errors.New("password must not be empty")
	}
	if first != second {
		return "", errors.New("passwords do not match")
	}
	return first, nil
}
