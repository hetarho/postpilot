// Command retirepublishing exports the private bridge report used before publishing
// records and staged copies are removed. It never starts an HTTP listener or contacts Naver.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/postpilot/backend/internal/platform/config"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/publishing"
	publishingstore "github.com/postpilot/backend/internal/publishing/store"
	"github.com/postpilot/backend/internal/storage"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		slog.Error("retire publishing", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return usageError()
	}
	switch args[0] {
	case "report":
		return runReport(ctx, args[1:])
	case "cleanup":
		return runCleanup(ctx, args[1:])
	default:
		return usageError()
	}
}

func runReport(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("retirepublishing report", flag.ContinueOnError)
	environment := flags.String("environment", "", "deployment environment identifier")
	output := flags.String("output", "", "new private report path")
	if err := flags.Parse(args); err != nil {
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

type cleanupFlags struct {
	options publishing.RetirementCleanupOptions
}

func parseCleanupFlags(args []string) (cleanupFlags, error) {
	flags := flag.NewFlagSet("retirepublishing cleanup", flag.ContinueOnError)
	environment := flags.String("environment", "", "deployment environment identifier")
	report := flags.String("report", "", "original private retirement report")
	reportDigest := flags.String("report-digest", "", "digest printed when the original report was created")
	shutdownInventory := flags.String("shutdown-inventory", "", "private reconciled companion-shutdown inventory")
	shutdownDigest := flags.String("shutdown-digest", "", "sha256 digest of the shutdown inventory file")
	receipt := flags.String("receipt", "", "private cleanup recovery/verification receipt")
	apply := flags.Bool("apply", false, "delete verified publishing objects and rows")
	verify := flags.Bool("verify", false, "verify an existing complete cleanup receipt")
	if err := flags.Parse(args); err != nil {
		return cleanupFlags{}, err
	}
	if flags.NArg() != 0 || *apply && *verify {
		return cleanupFlags{}, usageError()
	}
	for name, value := range map[string]string{
		"environment": *environment, "report": *report, "report-digest": *reportDigest,
		"shutdown-inventory": *shutdownInventory, "shutdown-digest": *shutdownDigest, "receipt": *receipt,
	} {
		if strings.TrimSpace(value) == "" {
			return cleanupFlags{}, fmt.Errorf("cleanup --%s is required", name)
		}
	}
	mode := publishing.RetirementInspect
	if *apply {
		mode = publishing.RetirementApply
	} else if *verify {
		mode = publishing.RetirementVerify
	}
	return cleanupFlags{options: publishing.RetirementCleanupOptions{
		Mode: mode, Environment: strings.TrimSpace(*environment), ReportPath: *report,
		ExpectedReportDigest: strings.TrimSpace(*reportDigest), ShutdownInventoryPath: *shutdownInventory,
		ExpectedShutdownDigest: strings.TrimSpace(*shutdownDigest), ReceiptPath: *receipt,
	}}, nil
}

func runCleanup(ctx context.Context, args []string) error {
	parsed, err := parseCleanupFlags(args)
	if err != nil {
		return err
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
	parsed.options.DatabaseIdentity = identity
	if err := cfg.RequireObjectStorage(); err != nil {
		return fmt.Errorf("object storage config invalid: %w", err)
	}
	bucket, err := storage.New(ctx, storage.Config{
		Endpoint: cfg.R2Endpoint, PublicEndpoint: cfg.R2PublicEndpoint, AccessKeyID: cfg.R2AccessKeyID,
		SecretAccessKey: cfg.R2SecretAccessKey, Bucket: cfg.R2Bucket, MaxReadBytes: cfg.MaxImageBytes,
	})
	if err != nil {
		return fmt.Errorf("object storage setup: %w", err)
	}
	receipt, err := (publishing.RetirementCleaner{
		Store: publishingstore.New(handle.Writer, handle.Reader), Objects: bucket,
	}).Run(ctx, parsed.options)
	if err != nil {
		return err
	}
	fmt.Printf("publishing retirement cleanup: status=%s rows=%d objects=%d receipt_digest=%s\n", receipt.Status, totalRetirementRows(receipt.RemainingCounts), receipt.RemainingObjects, receipt.Digest)
	return nil
}

func totalRetirementRows(counts publishing.RetirementCounts) int64 {
	return counts.Pairings + counts.Agents + counts.Reservations + counts.Jobs + counts.Assets
}

func usageError() error {
	return fmt.Errorf("usage: retirepublishing report --environment NAME --output FILE | retirepublishing cleanup --environment NAME --report FILE --report-digest DIGEST --shutdown-inventory FILE --shutdown-digest SHA256 --receipt FILE [--apply|--verify]")
}
