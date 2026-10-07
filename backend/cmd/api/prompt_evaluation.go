package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/postpilot/backend/internal/generation"
	"github.com/postpilot/backend/internal/llm"
)

var errPromptEvaluationLiveAdmission = errors.New("prompt evaluation live execution is unverified: this offline command has no admitted live executor; explicit owner, curated ModelRef, bounded usage budget, credentials and free capacity (when applicable) must be checked through normal product admission/metering/private capture paths; no provider call was made; see docs/research/prompt-evaluation-runbook.md")

type promptInventoryIdentity struct {
	Key                   string          `json:"key"`
	FixtureID             string          `json:"fixture_id,omitempty"`
	OriginProtocolVersion *int            `json:"origin_protocol_version,omitempty"`
	PromptVersion         string          `json:"prompt_version"`
	SchemaVersion         string          `json:"schema_version"`
	CompositionSHA256     string          `json:"composition_sha256"`
	SchemaSHA256          string          `json:"schema_sha256"`
	TextSHA256            string          `json:"text_sha256"`
	Operation             string          `json:"operation"`
	Links                 promptCodeLinks `json:"links"`
}

// These are synthetic reference settings, never effective registry admission.
// The baseline and variant preserve them byte-for-byte, independently of output
// target language. Native speech units never enter the token fields below.
type promptEvaluationConditions struct {
	Scope               string `json:"scope"`
	Model               string `json:"model"`
	MaxCompletionTokens int    `json:"max_completion_tokens"`
	Reasoning           string `json:"reasoning"`
	Stage               string `json:"stage"`
	StructuredOutput    bool   `json:"structured_output"`
}

type promptReferenceMeasures struct {
	Characters             int64  `json:"unicode_scalars"`
	UTF8Bytes              int64  `json:"utf8_bytes"`
	ReferenceTokenEstimate int64  `json:"reference_token_estimate"`
	Method                 string `json:"method"`
	Scope                  string `json:"scope"`
}

type promptEvaluationCase struct {
	ID                    string                                   `json:"id"`
	Scenario              string                                   `json:"scenario"`
	InstructionLanguage   string                                   `json:"instruction_language"`
	OutputLanguage        generation.Language                      `json:"output_language"`
	OriginProtocolVersion int                                      `json:"origin_protocol_version"`
	ModelRef              string                                   `json:"model_ref"`
	FixtureVersion        string                                   `json:"fixture_version"`
	CompilerVersion       string                                   `json:"compiler_version"`
	FixtureSHA256         string                                   `json:"fixture_sha256"`
	RequestSHA256         string                                   `json:"request_sha256"`
	CompositionSHA256     string                                   `json:"composition_sha256"`
	SchemaSHA256          string                                   `json:"schema_sha256"`
	Prepared              llm.RequestInspection                    `json:"prepared"`
	ApplicationText       promptApplicationText                    `json:"application_text"`
	ReferenceConditions   promptEvaluationConditions               `json:"reference_conditions"`
	ReferenceMeasures     promptReferenceMeasures                  `json:"reference_measures"`
	Expectations          []generation.PromptEvaluationExpectation `json:"expectations"`
	SyntheticResponse     string                                   `json:"synthetic_response"`
	ResponseProvenance    string                                   `json:"response_provenance"`
}

// Unknown outcomes are nil, not zero or a synthetic passing score. No generated
// candidate, automated source judge, winner or superiority claim exists here.
type promptEvaluationOutcomes struct {
	Status                   string   `json:"status"`
	ProviderPromptTokens     *int64   `json:"provider_prompt_tokens"`
	ProviderCompletionTokens *int64   `json:"provider_completion_tokens"`
	ProviderReasoningTokens  *int64   `json:"provider_reasoning_tokens"`
	ProviderNativeUnits      *string  `json:"provider_native_units"`
	InputCostMicrousd        *int64   `json:"input_cost_microusd"`
	OutputCostMicrousd       *int64   `json:"output_cost_microusd"`
	TotalCostMicrousd        *int64   `json:"total_cost_microusd"`
	UsableResponseRate       *float64 `json:"usable_response_rate"`
	TruncationRate           *float64 `json:"truncation_rate"`
	OriginCoverage           *float64 `json:"origin_coverage"`
	SemanticSupportReview    *string  `json:"semantic_support_review"`
	HumanExpressionReview    *string  `json:"human_expression_review"`
	HumanVoiceReview         *string  `json:"human_voice_review"`
	OwnerPublicationTrial    *string  `json:"owner_publication_trial"`
	Note                     string   `json:"note"`
}

type promptEvaluationReport struct {
	Version    int                                `json:"version"`
	Kind       string                             `json:"kind"`
	Material   string                             `json:"material"`
	Provenance promptProvenance                   `json:"provenance"`
	Inventory  []promptInventoryIdentity          `json:"inventory"`
	Cases      []promptEvaluationCase             `json:"cases"`
	Checks     []generation.PromptEvaluationCheck `json:"deterministic_contract_checks"`
	Outcomes   promptEvaluationOutcomes           `json:"runtime_and_human_outcomes"`
	Execution  promptOfflineExecution             `json:"execution"`
	Assessment string                             `json:"assessment_boundary"`
}

func promptRequestApplicationText(request llm.Request) promptApplicationText {
	out := promptApplicationText{System: request.System, Scope: "exact synthetic application text; media bytes/URLs omitted; prepared fragment references identify media; schema and SDK framing excluded"}
	for _, message := range request.Messages {
		text := promptTextMessage{Role: llm.InspectionRole(message.Role)}
		for _, part := range message.Parts {
			if !part.IsImage() && !part.IsVideo() {
				text.Text += part.Text
			}
		}
		out.Messages = append(out.Messages, text)
	}
	return out
}

func runPromptEvaluation(args []string, output io.Writer) error {
	// Refuse live and execution-shaped inputs before reading source, creating a
	// fixture or emitting any passing evidence. Paid calls need a normal owner
	// executor; flags cannot create that admission seam.
	for _, arg := range args {
		if arg == "--live" || arg == "--execute" || arg == "--run" {
			return errPromptEvaluationLiveAdmission
		}
	}
	if len(args) != 0 {
		return fmt.Errorf("usage: api prompt-evaluation (offline); --live refuses pending normal bounded owner admission")
	}
	cases, err := generation.PromptEvaluationCases()
	if err != nil {
		return err
	}
	report, err := buildPromptEvaluation(cases)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

func buildPromptEvaluation(cases []generation.PromptEvaluationCase) (promptEvaluationReport, error) {
	out := promptEvaluationReport{Version: 1, Kind: "controlled-writing-evaluation", Material: "synthetic", Execution: promptOfflineExecution{Mode: "offline"}, Outcomes: promptEvaluationOutcomes{Status: "unverified", Note: "No provider response or human review has occurred. Origin coverage measures structurally referenced ranges only and cannot establish semantic support. Reference-token estimates do not measure usage or cost; native speech usage requires its separately reported billing units."}, Assessment: "Deterministic compiler/fixture checks assess structure and preservation only. Fixture expected-source labels are supplied synthetic review material, never a truth classifier, actual model output or human semantic/prose result."}
	if len(cases) == 0 {
		return out, fmt.Errorf("prompt evaluation has no deterministic fixture selection")
	}
	caseIDs := map[string]bool{}
	for _, item := range cases {
		if item.ID == "" || caseIDs[item.ID] {
			return out, fmt.Errorf("prompt evaluation fixture identity is empty or duplicated: %s", item.ID)
		}
		caseIDs[item.ID] = true
	}
	inventory, err := buildPromptInventory(productRequestCompositions())
	if err != nil {
		return out, err
	}
	if inventory.Provenance.SourceStatus != "available" {
		return out, fmt.Errorf("exact-version prompt evaluation requires the matching source checkout: %s", inventory.Provenance.UnavailableReason)
	}
	root, err := promptBackendRoot()
	if err != nil {
		return out, err
	}
	compositions := append([]llm.RequestComposition(nil), inventory.Compositions...)
	for _, fixture := range cases {
		if fixture.Request.Composition == nil {
			return out, fmt.Errorf("evaluation fixture %s has no actual composer descriptor", fixture.ID)
		}
		compositions = append(compositions, *fixture.Request.Composition)
	}
	paths := append(promptSourcePaths(compositions), "cmd/api/prompt_evaluation.go")
	// Sources emitted by the fixture API include its definition and compiler seam;
	// add the command itself so changes to report semantics alter the source ID.
	out.Provenance, _, err = loadPromptSources(root, paths)
	if err != nil {
		return out, err
	}
	for _, entry := range inventory.Entries {
		out.Inventory = append(out.Inventory, promptInventoryIdentity{Key: entry.Key, FixtureID: entry.FixtureID, OriginProtocolVersion: entry.OriginProtocolVersion, PromptVersion: entry.Prepared.PromptVersion, SchemaVersion: entry.Prepared.SchemaVersion, CompositionSHA256: entry.CompositionSHA256, SchemaSHA256: entry.SchemaSHA256, TextSHA256: entry.TextSHA256, Operation: entry.Operation, Links: entry.Links})
	}
	out.Checks = generation.CheckPromptEvaluationCases(cases)
	for _, check := range out.Checks {
		if !check.Passed {
			return out, fmt.Errorf("deterministic prompt contract failed: %s/%s: %s", check.CaseID, check.ID, check.Detail)
		}
	}
	for _, fixture := range cases {
		prepared, err := llm.PreparedRequestInspection(fixture.Request)
		if err != nil {
			return out, fmt.Errorf("prepare evaluation fixture %s: %w", fixture.ID, err)
		}
		application := promptRequestApplicationText(fixture.Request)
		conditions := promptEvaluationConditions{Scope: "synthetic frozen reference; not deployment configuration, eligibility or admission", Model: fixture.ModelRef, MaxCompletionTokens: fixture.Request.MaxTokens, Reasoning: string(fixture.Request.Reasoning), Stage: fixture.Request.Stage, StructuredOutput: fixture.Request.JSONSchema != nil}
		fixtureHash, err := promptJSONHash(struct {
			ID, Scenario, InstructionLanguage, ModelRef, FixtureVersion, CompilerVersion, SyntheticResponse, ResponseProvenance string
			OriginProtocolVersion                                                                                               int
			OutputLanguage                                                                                                      generation.Language
			Expectations                                                                                                        []generation.PromptEvaluationExpectation
		}{fixture.ID, fixture.Scenario, string(fixture.InstructionLanguage), fixture.ModelRef, fixture.FixtureVersion, fixture.CompilerVersion, fixture.SyntheticResponse, fixture.ResponseProvenance, fixture.OriginProtocolVersion, fixture.OutputLanguage, fixture.Expectations})
		if err != nil {
			return out, err
		}
		requestHash, err := promptJSONHash(struct {
			Text       promptApplicationText
			Schema     string
			Conditions promptEvaluationConditions
		}{application, string(fixture.Request.JSONSchema), conditions})
		if err != nil {
			return out, err
		}
		compositionHash, err := promptJSONHash(fixture.Request.Composition)
		if err != nil {
			return out, err
		}
		if prepared.Measures.Characters == nil || prepared.Measures.UTF8Bytes == nil {
			return out, fmt.Errorf("evaluation fixture %s omitted exact application text measures", fixture.ID)
		}
		reference := promptReferenceMeasures{Characters: *prepared.Measures.Characters, UTF8Bytes: *prepared.Measures.UTF8Bytes, ReferenceTokenEstimate: (*prepared.Measures.UTF8Bytes + 3) / 4, Method: "ceil(utf8_text_bytes / 4), uncalibrated reference-only estimate", Scope: "application System and nonmedia message text only; excludes schema, media and provider framing; no tokenizer, provider usage or cost claim"}
		out.Cases = append(out.Cases, promptEvaluationCase{ID: fixture.ID, Scenario: fixture.Scenario, InstructionLanguage: string(fixture.InstructionLanguage), OutputLanguage: fixture.OutputLanguage, OriginProtocolVersion: fixture.OriginProtocolVersion, ModelRef: fixture.ModelRef, FixtureVersion: fixture.FixtureVersion, CompilerVersion: fixture.CompilerVersion, FixtureSHA256: fixtureHash, RequestSHA256: requestHash, CompositionSHA256: compositionHash, SchemaSHA256: promptHash(fixture.Request.JSONSchema), Prepared: prepared, ApplicationText: application, ReferenceConditions: conditions, ReferenceMeasures: reference, Expectations: fixture.Expectations, SyntheticResponse: fixture.SyntheticResponse, ResponseProvenance: fixture.ResponseProvenance})
	}
	return out, nil
}
