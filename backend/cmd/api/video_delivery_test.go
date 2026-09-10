package main

import (
	"errors"
	"testing"

	"github.com/postpilot/backend/internal/experiment"
	"github.com/postpilot/backend/internal/generation"
)

func TestWriteExperimentPreservesSignedVideoRefusal(t *testing.T) {
	err := mapSnapshotError(&generation.VideoUnsupportedError{Model: "provider/video"})
	var refusal *experiment.VideoUnsupportedError
	if !errors.Is(err, experiment.ErrVideoUnsupported) || !errors.As(err, &refusal) || refusal.Model != "provider/video" {
		t.Fatalf("refusal = %v", err)
	}
}
