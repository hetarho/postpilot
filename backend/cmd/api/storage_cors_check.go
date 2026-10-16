package main

import (
	"context"
	"fmt"
	"io"

	"github.com/postpilot/backend/internal/platform/config"
	"github.com/postpilot/backend/internal/storage"
)

func runStorageCORSCheck(ctx context.Context, cfg *config.Config, args []string, out io.Writer) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: api check-storage-cors <origin>")
	}
	if err := cfg.RequireObjectStorage(); err != nil {
		return fmt.Errorf("check storage CORS (deploy/r2-cors.json): %w", err)
	}
	bucket, err := storage.New(ctx, storage.Config{
		Endpoint: cfg.R2Endpoint, PublicEndpoint: cfg.R2PublicEndpoint,
		AccessKeyID: cfg.R2AccessKeyID, SecretAccessKey: cfg.R2SecretAccessKey,
		Bucket: cfg.R2Bucket, MaxReadBytes: cfg.MaxImageBytes,
	})
	if err != nil {
		return fmt.Errorf("check storage CORS (deploy/r2-cors.json): %w", err)
	}
	if err := bucket.CheckCORS(ctx, args[0]); err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, "storage CORS: ranged GET exposes Content-Range for", args[0])
	return err
}
