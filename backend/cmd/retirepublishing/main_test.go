package main

import (
	"testing"

	"github.com/postpilot/backend/internal/publishing"
)

func TestCleanupCommandDefaultsToInspectionAndRequiresExplicitMutationModes(t *testing.T) {
	base := []string{
		"--environment", "production",
		"--report", "/private/report.json",
		"--report-digest", "report-digest",
		"--shutdown-inventory", "/private/shutdown.json",
		"--shutdown-digest", "sha256:shutdown",
		"--receipt", "/private/cleanup.json",
	}
	for name, extra := range map[string][]string{
		"inspection": nil,
		"apply":      {"--apply"},
		"verify":     {"--verify"},
	} {
		t.Run(name, func(t *testing.T) {
			parsed, err := parseCleanupFlags(append(append([]string{}, base...), extra...))
			if err != nil {
				t.Fatal(err)
			}
			want := publishing.RetirementInspect
			if name == "apply" {
				want = publishing.RetirementApply
			} else if name == "verify" {
				want = publishing.RetirementVerify
			}
			if parsed.options.Mode != want || parsed.options.Environment != "production" {
				t.Fatalf("options = %+v, want mode %s", parsed.options, want)
			}
		})
	}
	if _, err := parseCleanupFlags(append(append([]string{}, base...), "--apply", "--verify")); err == nil {
		t.Fatal("apply and verify were accepted together")
	}
	if _, err := parseCleanupFlags(base[:len(base)-2]); err == nil {
		t.Fatal("missing receipt was accepted")
	}
}

func TestTotalRetirementRowsIncludesEveryOwnedTable(t *testing.T) {
	counts := publishing.RetirementCounts{Pairings: 1, Agents: 2, Reservations: 3, Jobs: 4, Assets: 5}
	if got := totalRetirementRows(counts); got != 15 {
		t.Fatalf("total = %d", got)
	}
}
