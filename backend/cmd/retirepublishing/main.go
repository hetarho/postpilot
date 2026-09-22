// Command retirepublishing exports the private bridge report used before publishing
// records and staged copies are removed. It never starts an HTTP listener or contacts Naver.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/postpilot/backend/internal/platform/config"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/publishing"
	publishingstore "github.com/postpilot/backend/internal/publishing/store"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		slog.Error("retire publishing", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] != "report" {
		return fmt.Errorf("usage: retirepublishing report --environment NAME --output FILE")
	}
	flags := flag.NewFlagSet("retirepublishing report", flag.ContinueOnError)
	environment := flags.String("environment", "", "deployment environment identifier")
	output := flags.String("output", "", "new private report path")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("usage: retirepublishing report --environment NAME --output FILE")
	}
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	handle, err := db.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer handle.Close()
	if err := db.Migrate(ctx, handle.Writer); err != nil {
		return err
	}
	identity, err := publishing.DatabaseIdentity(cfg.DBPath)
	if err != nil {
		return err
	}
	report, err := publishing.BuildRetirementReport(ctx, publishingstore.New(handle.Writer, handle.Reader), *environment, identity)
	if err != nil {
		return err
	}
	if err := publishing.WriteRetirementReport(*output, report); err != nil {
		return err
	}
	fmt.Printf("publishing retirement report written: digest=%s\n", report.Digest)
	return nil
}
