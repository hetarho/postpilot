package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestAnalysisQualityRefusesLiveBeforePlatformBoot(t *testing.T) {
	if os.Getenv("POSTPILOT_ANALYSIS_COMMAND_TEST") == "1" {
		runCommand([]string{"analysis-quality", "--live", "--input", "/missing-corpus", "--output", "/missing-output"})
		os.Exit(2)
	}
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestAnalysisQualityRefusesLiveBeforePlatformBoot$")
	cmd.Env = append(os.Environ(), "POSTPILOT_ANALYSIS_COMMAND_TEST=1", "DATABASE_PATH=/forbidden/database.sqlite", "PROVIDERS_CONFIG=/missing/provider-config", "R2_ACCOUNT_ID=", "R2_ACCESS_KEY_ID=", "R2_SECRET_ACCESS_KEY=", "R2_BUCKET=")
	data, e := cmd.CombinedOutput()
	if e == nil || !strings.Contains(string(data), "analysis_quality_live_admission_unavailable") || strings.Contains(string(data), "R2_ACCOUNT_ID is required") || strings.Contains(string(data), "platform failed") {
		t.Fatalf("offline operator branch booted platform: %v %s", e, data)
	}
}
