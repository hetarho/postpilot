package main

import (
	"context"
	"io"
	"time"

	"github.com/postpilot/backend/internal/clip/diagnostic/analysisquality"
)

func runAnalysisQuality(ctx context.Context, args []string, out io.Writer) error {
	return analysisquality.Execute(ctx, args, out, time.Now().UTC())
}
