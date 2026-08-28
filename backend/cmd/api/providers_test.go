package main

import (
	"testing"
	"time"

	"github.com/postpilot/backend/internal/llm"
)

// The registry that ships in the image must load with the adapters this binary wires —
// a typo in config/providers.yaml would otherwise be found by the deploy's health gate.
func TestShippedProvidersConfigLoads(t *testing.T) {
	noKeys := func(string) string { return "" }
	reg, err := llm.Load("../../config/providers.yaml", noKeys, adapters, llm.Options{Timeout: time.Minute, MaxTokens: 1})
	if err != nil {
		t.Fatal(err)
	}
	models := reg.Models()
	if len(models) == 0 {
		t.Fatal("the shipped registry declares no models")
	}
	// Without keys every model is disabled with the documented reason (AC2), and the
	// process still comes up.
	for _, m := range models {
		if !m.Disabled || m.DisabledReason != llm.DisabledReasonNoKey {
			t.Errorf("%s without a key: disabled=%v reason=%q", m.Ref, m.Disabled, m.DisabledReason)
		}
	}
	// The observe stage needs at least one vision model to be selectable at all.
	hasVision := false
	for _, m := range models {
		hasVision = hasVision || m.Vision
	}
	if !hasVision {
		t.Error("the shipped registry has no vision model — the observe dropdown would be empty")
	}
}
