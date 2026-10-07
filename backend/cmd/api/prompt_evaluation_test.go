package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/generation"
	"github.com/postpilot/backend/internal/llm"
)

func TestPromptEvaluationOfflineReportPreservesUnitsAndUnknownOutcomes(t *testing.T) {
	var output, repeated bytes.Buffer
	if err := runPromptEvaluation(nil, &output); err != nil {
		t.Fatal(err)
	}
	if err := runPromptEvaluation(nil, &repeated); err != nil || !bytes.Equal(output.Bytes(), repeated.Bytes()) {
		t.Fatal("fixed source and fixtures did not reproduce identical reports", err)
	}
	var report promptEvaluationReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Kind != "controlled-writing-evaluation" || report.Material != "synthetic" || report.Execution != (promptOfflineExecution{Mode: "offline"}) || report.Provenance.SourceStatus != "available" || len(report.Inventory) != 71 || len(report.Cases) != 48 || len(report.Checks) == 0 {
		t.Fatalf("wrong controlled offline selection: inventory=%d cases=%d checks=%d execution=%+v", len(report.Inventory), len(report.Cases), len(report.Checks), report.Execution)
	}
	unknown := promptEvaluationOutcomes{Status: "unverified", Note: report.Outcomes.Note}
	if !reflect.DeepEqual(report.Outcomes, unknown) || !strings.Contains(report.Assessment, "never a truth classifier") {
		t.Fatalf("offline fixtures invented runtime/human results: %+v", report.Outcomes)
	}
	for _, check := range report.Checks {
		if !check.Passed || check.CaseID == "" || check.ID == "" || check.Detail == "" {
			t.Fatalf("required deterministic contract failed: %+v", check)
		}
	}
	for _, item := range report.Cases {
		if item.Prepared.Status != llm.InspectionPrepared || item.Prepared.IssuedAt != nil || item.Prepared.Conditions != nil || item.Prepared.Measures.ProviderPromptTokens != nil || item.OutputLanguage != generation.LanguageKorean || item.ReferenceConditions.Model != "synthetic/prompt-evaluation" || item.ReferenceConditions.MaxCompletionTokens <= 0 || !strings.Contains(item.ReferenceConditions.Scope, "synthetic frozen reference") {
			t.Fatalf("fixture omitted frozen reference or claimed admission: %+v", item.ReferenceConditions)
		}
		if item.ReferenceMeasures.UTF8Bytes != *item.Prepared.Measures.UTF8Bytes || item.ReferenceMeasures.Characters != *item.Prepared.Measures.Characters || item.ReferenceMeasures.ReferenceTokenEstimate != (item.ReferenceMeasures.UTF8Bytes+3)/4 || !strings.Contains(item.ReferenceMeasures.Method, "uncalibrated reference-only") {
			t.Fatalf("units or reference estimate merged with actual usage: %+v", item.ReferenceMeasures)
		}
		if item.FixtureVersion != generation.PromptEvaluationFixtureVersion || item.CompilerVersion != generation.PromptEvaluationCompilerVersion || item.FixtureSHA256 == "" || item.RequestSHA256 == "" || item.SchemaSHA256 != promptHash([]byte(item.Prepared.Output.Schema)) || item.SyntheticResponse == "" || item.ResponseProvenance != "fixed-synthetic-output-not-model-response" {
			t.Fatalf("fixture identity or synthetic response provenance lost: %s", item.ID)
		}
	}
}

func TestPromptEvaluationCommandRunsBeforePlatformBoot(t *testing.T) {
	if os.Getenv("POSTPILOT_PROMPT_EVALUATION_PROCESS") == "1" {
		if !runCommand([]string{"prompt-evaluation"}) {
			t.Fatal("evaluation command was not discovered")
		}
		return
	}
	command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestPromptEvaluationCommandRunsBeforePlatformBoot$")
	command.Env = append(os.Environ(), "POSTPILOT_PROMPT_EVALUATION_PROCESS=1", "PORT=invalid-before-boot", "DB_PATH=/does-not-exist/private-customer.sqlite", "PROVIDERS_CONFIG=/does-not-exist/private-provider-config.yaml")
	output, err := command.CombinedOutput()
	if err != nil || !bytes.Contains(output, []byte(`"provider_calls": 0`)) || !bytes.Contains(output, []byte(`"customer_reads": 0`)) || !bytes.Contains(output, []byte(`"platform_boot": false`)) || bytes.Contains(output, []byte("migration applied")) {
		t.Fatalf("offline evaluation booted platform or read configured customer/provider state: %s %v", output, err)
	}
}

func TestPromptEvaluationLiveAndExecutionFlagsRefuseBeforeAnyOutput(t *testing.T) {
	for _, args := range [][]string{{"--live"}, {"--execute"}, {"--run"}, {"--live", "--owner", "synthetic-owner", "--budget-microusd", "10"}, {"--owner", "synthetic-owner", "--execute"}} {
		var output bytes.Buffer
		if err := runPromptEvaluation(args, &output); !errors.Is(err, errPromptEvaluationLiveAdmission) || output.Len() != 0 {
			t.Fatalf("execution-shaped arguments returned passing evidence or lost admission refusal: %v %v", args, err)
		}
	}
	var output bytes.Buffer
	if err := runPromptEvaluation([]string{"--private-post", "other-owner"}, &output); err == nil || output.Len() != 0 {
		t.Fatal("offline evaluation accepted customer-read arguments")
	}
}

func TestPromptEvaluationFailsWhenCompilerFixturesDrift(t *testing.T) {
	cases, err := generation.PromptEvaluationCases()
	if err != nil {
		t.Fatal(err)
	}
	cases[1].Request.System += "unexpected untranslated instruction"
	if _, err := buildPromptEvaluation(cases); err == nil || !strings.Contains(err.Error(), "deterministic prompt contract failed") {
		t.Fatal("a failed deterministic contract produced a successful evaluation report", err)
	}
	if _, err := buildPromptEvaluation(nil); err == nil {
		t.Fatal("empty test selection claimed offline verification")
	}
	cases[1] = cases[0]
	if _, err := buildPromptEvaluation(cases); err == nil || !strings.Contains(err.Error(), "duplicated") {
		t.Fatal("duplicate case identities claimed complete verification", err)
	}
}

func TestPromptInventoryPinsCurrentRetainedModesAndSyntheticBranches(t *testing.T) {
	report, err := buildPromptInventory(productRequestCompositions())
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{}
	for _, mode := range []string{"direct", "frozen-storyline", "full-test-direct", "photo-observation", "video-observation", "full-test-photo-observation", "full-test-video-observation", "storyline-create", "storyline-rewrite", "revision", "direct-no-voice", "revision-with-voice"} {
		for _, protocol := range []string{"0", "1"} {
			want[mode+"/protocol-"+protocol] = true
		}
	}
	for _, kind := range []string{"post_template", "video_template", "post_guideline", "video_guideline", "writing_voice"} {
		for _, mode := range []string{"recommend", "refine"} {
			want["setting-authoring/"+kind+"/"+mode] = true
		}
	}
	for _, mode := range []string{"analyze", "recommend", "legacy-check"} {
		want["writing-style/"+mode] = true
	}
	want["memory-extraction/propose"] = true
	for _, mode := range []string{"request", "correction"} {
		want["template-request/"+mode] = true
	}
	for _, mode := range []string{"observe", "flow", "flow-follow-storyline", "flow-measured-speech", "flow-follow-storyline-measured-speech", "flow-revision", "narration", "narration-revision", "storyline", "storyline-revision", "spoken-script", "spoken-script-follow-storyline", "spoken-script-revision"} {
		stage := "video-composition"
		if mode == "observe" {
			stage = "video-observation"
		}
		want[stage+"/"+mode], want[stage+"/"+mode+"/correction"] = true, true
	}
	for _, mode := range []string{"design", "confirm", "speech"} {
		want["spoken-voice/"+mode] = true
	}
	want["video-speech/segment-synthesis"], want["video-speech/initial-segment-synthesis"] = true, true
	if len(want) != 71 || len(report.Entries) != len(want) {
		t.Fatalf("actual admitted mode/scenario coverage changed: want=%d actual=%d", len(want), len(report.Entries))
	}
	fixtures := map[string]generation.PromptInventoryFixture{}
	for _, item := range generation.PromptInventoryFixtures() {
		fixtures[item.ID] = item
	}
	contracts := map[string]bool{}
	for i, entry := range report.Entries {
		contracts[promptCompositionKey(report.Compositions[i])] = true
		key := entry.Prepared.Stage + "/" + entry.Prepared.Mode
		if entry.FixtureID != "" {
			key = entry.FixtureID
			fixture, ok := fixtures[key]
			if !ok || entry.OriginProtocolVersion == nil || *entry.OriginProtocolVersion != fixture.OriginProtocolVersion || !reflect.DeepEqual(entry.ApplicationText, promptRequestApplicationText(fixture.Request)) || entry.Prepared.Output.Schema != string(fixture.Request.JSONSchema) {
				t.Fatalf("generation inventory did not use exact real preparation: %s", key)
			}
		}
		if !want[key] {
			t.Fatal("unexpected/duplicated inventory mode or protocol branch", key)
		}
		delete(want, key)
	}
	if len(want) != 0 {
		t.Fatal("retained/current/native mode coverage omitted", want)
	}
	if len(contracts) != 67 {
		t.Fatalf("mode/protocol contracts should distinguish 67 contracts from 71 fixture scenarios: %d", len(contracts))
	}
}
