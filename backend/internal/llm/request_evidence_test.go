package llm_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/postpilot/backend/internal/llm"
)

func evidenceComposition() *llm.RequestComposition {
	return &llm.RequestComposition{Stage: "post.write", Mode: "direct", PromptVersion: "post-write-v1", SchemaVersion: "post-v1", Composer: "generation.writeCandidate", Parser: "generation.ParseWriteAnswer", Consumer: "post.SetGeneratedContent", Activation: "explicit owner generation", SourceFiles: []string{"internal/generation/write.go"}, Fragments: []llm.RequestFragment{{ID: "contract", Role: llm.InspectionRoleSystem, Authorship: llm.FragmentAuthorshipCode, MaterialRole: "instruction", Text: "Write from supplied facts.", SourceFiles: []string{"internal/generation/prompts.go"}, Activation: "write"}, {ID: "memo", Role: llm.InspectionRoleUser, Authorship: llm.FragmentAuthorshipAccount, MaterialRole: "factual_material", Text: "카페😀\nCafé", SourceRefs: []string{"memo-input-7"}}, {ID: "media", Role: llm.InspectionRoleUser, Authorship: llm.FragmentAuthorshipAccount, MaterialRole: "visual_evidence", SourceRefs: []string{"photo-second", "photo-first"}}}, SelectedRuleIDs: []string{"stock.facts", "owner.rule-a"}, Omissions: []llm.RequestOmission{{ID: "memory", Reason: "owner option disabled", Activation: "memory opt-in", SourceFiles: []string{"internal/generation/service.go"}}}, Output: llm.OutputContractInspection{Name: "post", Version: "post-v1"}}
}

type evidenceProvider struct {
	fakeProvider
	result  llm.Response
	failure error
	before  func(context.Context, llm.Request)
}
type evidenceFailure struct{ cause error }

func (e *evidenceFailure) Error() string { return "provider rejected output" }
func (e *evidenceFailure) Unwrap() error { return e.cause }

func (p *evidenceProvider) Complete(ctx context.Context, req llm.Request) (llm.Response, error) {
	p.calls++
	p.last = req
	p.ctx = ctx
	if p.before != nil {
		p.before(ctx, req)
	}
	return p.result, p.failure
}
func evidenceRegistry(t *testing.T, p *evidenceProvider, source fakeSource) *llm.Registry {
	t.Helper()
	r, err := llm.Parse([]byte(goodYAML), env(map[string]string{"TEST_KEY": "PRIVATE_API_KEY"}), map[string]llm.AdapterFactory{"fake": func(llm.AdapterConfig) (llm.Provider, error) { return p, nil }}, source, opts)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestPreparedAndCapturedInspectionShareEffectiveRegistryResolutionWithoutExtraCalls(t *testing.T) {
	for _, override := range []llm.ReasoningEffort{llm.ReasoningHigh, llm.ReasoningNone, llm.ReasoningUnset} {
		t.Run(string(override), func(t *testing.T) {
			source := twoModels()
			source.models[0].Reasoning = map[string]llm.ReasoningEffort{"write": override}
			p := &evidenceProvider{result: llm.Response{Text: "done", Usage: llm.Usage{PromptTokens: 13, CompletionTokens: 7, ReasoningTokens: 3, CostMicrousd: 90000, CostReported: true}}}
			registry := evidenceRegistry(t, p, source)
			ref := llm.ModelRef{ProviderID: "openrouter", ModelID: "vision-json"}
			req := llm.Request{Composition: evidenceComposition(), Stage: "write", MaxTokens: -1, Reasoning: llm.ReasoningLow, System: "Actual system", Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart("actual material"), llm.ImagePart([]byte("PRIVATE_IMAGE_BODY"), "image/jpeg")}}}, JSONSchema: []byte(`{"type":"object"}`)}
			prepared, err := registry.Prepare(context.Background(), ref, req)
			if err != nil || p.calls != 0 || prepared.Status != llm.InspectionPrepared || prepared.IssuedAt != nil || prepared.Conditions == nil || *prepared.Conditions.MaxCompletionTokens != 1024 || *prepared.Conditions.ReasoningEffort != override || !*prepared.Conditions.DefaultBudget {
				t.Fatalf("prepared=%+v conditions=%+v calls=%d err=%v", prepared, prepared.Conditions, p.calls, err)
			}
			response, err := registry.Complete(context.Background(), ref, req)
			if err != nil || response.Inspection == nil || p.calls != 1 {
				t.Fatalf("captured=%+v err=%v", response, err)
			}
			captured := response.Inspection
			if captured.Status != llm.InspectionCaptured || captured.IssuedAt == nil || !reflect.DeepEqual(captured.Conditions, prepared.Conditions) || captured.Output.Schema != string(req.JSONSchema) || *captured.Measures.ProviderPromptTokens != 13 || *captured.Measures.ProviderCompletionTokens != 7 || *captured.Measures.ProviderReasoningTokens != 3 {
				t.Fatalf("effective witness drift=%+v", captured)
			}
			expectedDisable := override == llm.ReasoningNone
			if *captured.Conditions.DisableReasoning != expectedDisable || p.last.DisableReasoning != expectedDisable || *captured.Conditions.ReasoningOmitted != (override == llm.ReasoningUnset || expectedDisable) {
				t.Fatal("disable/omission decision not captured")
			}
			raw, _ := json.Marshal(captured)
			for _, secret := range []string{"PRIVATE_API_KEY", "PRIVATE_IMAGE_BODY", "example.test", "90000", "CostMicrousd", "Messages", "ExecutionPolicy"} {
				if strings.Contains(string(raw), secret) {
					t.Fatalf("transport/supplier secret leaked=%s", secret)
				}
			}
		})
	}
}

func TestRegistryCapturePreservesProviderFailureIdentityAndBillableUsageWithoutClaimingSuccess(t *testing.T) {
	cause := &evidenceFailure{cause: llm.ErrBadOutput}
	p := &evidenceProvider{failure: cause, result: llm.Response{Usage: llm.Usage{PromptTokens: 17, CompletionTokens: 2, CostMicrousd: 30, CostReported: true}}}
	registry := evidenceRegistry(t, p, twoModels())
	response, err := registry.Complete(context.Background(), llm.ModelRef{ProviderID: "openrouter", ModelID: "vision-json"}, llm.Request{Stage: "write", Composition: evidenceComposition()})
	var bad *evidenceFailure
	if !errors.Is(err, llm.ErrBadOutput) || !errors.As(err, &bad) || bad != cause {
		t.Fatalf("failure chain replaced=%v", err)
	}
	witness, found := llm.RequestInspectionFromError(err)
	if !found || witness.Status != llm.InspectionCaptured || witness.IssuedAt == nil || response.Inspection == nil || *witness.Measures.ProviderPromptTokens != 17 {
		t.Fatalf("failed invoked witness missing=%+v found=%v", witness, found)
	}
	witness.Fragments[0].Text = "caller mutation"
	again, _ := llm.RequestInspectionFromError(err)
	if again.Fragments[0].Text == witness.Fragments[0].Text {
		t.Fatal("error witness shares mutable caller storage")
	}
	if strings.Contains(err.Error(), "Write from supplied facts") || strings.Contains(err.Error(), "memo-input-7") {
		t.Fatal("private inspection was serialized into error text")
	}
}

func TestRegistryPreflightUnsupportedCancelledAndUnadmittedWorkHasNoIssuedEvidence(t *testing.T) {
	for _, scenario := range []string{"cancelled", "missing admission", "bad grade", "unsupported image", "unsupported schema", "missing model"} {
		t.Run(scenario, func(t *testing.T) {
			p := &evidenceProvider{}
			registry := evidenceRegistry(t, p, twoModels())
			ref := llm.ModelRef{ProviderID: "openrouter", ModelID: "vision-json"}
			req := llm.Request{Composition: evidenceComposition(), Stage: "write"}
			ctx := context.Background()
			switch scenario {
			case "cancelled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "missing admission":
				registry.WithModelGrades()
			case "bad grade":
				registry.WithModelGrades()
				ctx = llm.WithAdmittedCalls(ctx, []llm.AdmittedCall{{Ref: ref, Stage: "write", Grade: "not-a-grade"}})
			case "unsupported image":
				ref.ModelID = "text-only"
				req.Messages = []llm.Message{{Parts: []llm.Part{llm.ImagePart([]byte{1}, "image/jpeg")}}}
			case "unsupported schema":
				ref.ModelID = "text-only"
				req.JSONSchema = []byte(`{}`)
			case "missing model":
				ref.ModelID = "absent"
			}
			if _, err := registry.Prepare(ctx, ref, req); err == nil {
				t.Fatal("failed preflight became prepared")
			}
			response, err := registry.Complete(ctx, ref, req)
			if err == nil || response.Inspection != nil || p.calls != 0 {
				t.Fatalf("failed preflight dispatched=%+v calls=%d err=%v", response, p.calls, err)
			}
			if _, found := llm.RequestInspectionFromError(err); found {
				t.Fatal("preflight error claimed issued witness")
			}
		})
	}
}

func TestFreeAdmissionAndFrozenExecutionInspectionPreserveActualDecision(t *testing.T) {
	source := twoModels()
	source.models[0].InputUSDPerMillion = "0.1"
	source.models[0].OutputUSDPerMillion = "0.7"
	source.models[0].Reasoning = map[string]llm.ReasoningEffort{"write": llm.ReasoningLow}
	p := &evidenceProvider{}
	registry := evidenceRegistry(t, p, source)
	ref := llm.ModelRef{ProviderID: "openrouter", ModelID: "vision-json"}
	frozen, err := registry.FreezeCall(ref, "write", 80, llm.ReasoningHigh)
	if err != nil {
		t.Fatal(err)
	}
	frozen.Pricing = llm.CallPricing{Version: llm.CallPricingVersion, Fingerprint: strings.Repeat("a", 64), Delivery: llm.ExecutionTextOnly, Endpoint: "PRIVATE_ENDPOINT", RequiredParameters: "max_tokens,reasoning", PromptUSDPerMillion: "0.1", CompletionUSDPerMillion: "0.7", RequestUSD: "0", ImageUSD: "0", AudioUSDPerToken: "0", AggregateUsageSufficient: true}
	source.models[0].Reasoning["write"] = llm.ReasoningMax
	request := llm.Request{Composition: evidenceComposition(), Stage: "write", MaxTokens: 80, Reasoning: llm.ReasoningLow, JSONSchema: []byte(`{}`), Execution: &llm.ExecutionPolicy{Call: frozen, Delivery: llm.ExecutionTextOnly, NoFallback: true, RequireParameters: true}}
	prepared, err := registry.Prepare(context.Background(), ref, request)
	if err != nil || !*prepared.Conditions.FrozenExecution || *prepared.Conditions.DefaultBudget || *prepared.Conditions.MaxCompletionTokens != 80 || *prepared.Conditions.ReasoningEffort != llm.ReasoningLow {
		t.Fatalf("frozen policy overwritten=%+v err=%v", prepared.Conditions, err)
	}
	response, err := registry.Complete(context.Background(), ref, request)
	if err != nil || response.Inspection == nil || !reflect.DeepEqual(response.Inspection.Conditions, prepared.Conditions) {
		t.Fatalf("frozen witness drift=%v", err)
	}
	raw, _ := json.Marshal(response.Inspection)
	if strings.Contains(string(raw), "PRIVATE_ENDPOINT") || strings.Contains(string(raw), "0.7") {
		t.Fatal("frozen supplier routing/pricing leaked")
	}
	registry.WithModelGrades()
	request.Execution = nil
	ctx := llm.WithAdmittedCalls(context.Background(), []llm.AdmittedCall{{Ref: ref, Stage: "write", Grade: "free"}})
	free, err := registry.Prepare(ctx, ref, request)
	if err != nil || !*free.Conditions.FreeCall {
		t.Fatalf("admitted free decision missing=%+v err=%v", free.Conditions, err)
	}
	called, err := registry.Complete(ctx, ref, request)
	if err != nil || called.Inspection == nil || !*called.Inspection.Conditions.FreeCall || !p.last.FreeCall {
		t.Fatal("capture guessed current catalog grade instead of admitted grade")
	}
}

func TestSafeCompositionMeasuresExactUnicodeAndNeverOpensOrProjectsRuntimeMedia(t *testing.T) {
	composition := evidenceComposition()
	estimate := int64(123)
	composition.ReferenceTokenEstimate = &estimate
	opened := false
	runtime := llm.InlineVideo{MIME: "video/mp4", Size: 4, DurationMS: 1000, Sampling: llm.VideoSamplingFixed, Open: func(context.Context) (io.ReadCloser, error) {
		opened = true
		return io.NopCloser(strings.NewReader("body")), nil
	}}
	req := llm.Request{Composition: composition, System: "PRIVATE_NETWORK_SYSTEM", Messages: []llm.Message{{Parts: []llm.Part{llm.TextPart("PRIVATE_TRANSPORT_ONLY"), llm.ImagePart([]byte("PRIVATE_IMAGE"), "image/jpeg"), llm.VideoPart("https://signed.private/object?X-Amz-Signature=PRIVATE_SIGNATURE", "video/mp4"), llm.InlineVideoPart(runtime)}}}}
	prepared, err := llm.PreparedRequestInspection(req)
	if err != nil {
		t.Fatal(err)
	}
	text := req.System + req.Messages[0].Parts[0].Text
	if *prepared.Measures.Characters != int64(utf8.RuneCountInString(text)) || *prepared.Measures.UTF8Bytes != int64(len(text)) || *prepared.Measures.ReferenceTokenEstimate != 123 || prepared.Measures.ProviderPromptTokens != nil {
		t.Fatalf("distinct measures confused=%+v", prepared.Measures)
	}
	raw, _ := json.Marshal(prepared)
	for _, secret := range []string{"PRIVATE_NETWORK_SYSTEM", "PRIVATE_TRANSPORT_ONLY", "PRIVATE_IMAGE", "PRIVATE_SIGNATURE", "signed.private", "Open", "VideoURL"} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("runtime field leaked=%s", secret)
		}
	}
	if opened || !reflect.DeepEqual(prepared.Fragments[2].SourceRefs, []string{"photo-second", "photo-first"}) {
		t.Fatal("media opened or safe identity order changed")
	}
	composition.Fragments[0].Text = "later mutation"
	composition.Fragments[2].SourceRefs[0] = "changed"
	composition.Omissions[0].SourceFiles[0] = "changed"
	if prepared.Fragments[0].Text == "later mutation" || prepared.Fragments[2].SourceRefs[0] == "changed" || prepared.Omissions[0].SourceFiles[0] == "changed" {
		t.Fatal("prepared evidence aliases mutable composition")
	}
}

func TestMissingOrInvalidManifestNeverManufacturesCaptureOrChangesCompletion(t *testing.T) {
	for _, invalid := range []bool{false, true} {
		p := &evidenceProvider{result: llm.Response{Text: "existing completion", Inspection: &llm.RequestInspection{Status: llm.InspectionCaptured}}}
		registry := evidenceRegistry(t, p, twoModels())
		request := llm.Request{Stage: "write"}
		if invalid {
			request.Composition = evidenceComposition()
			request.Composition.Stage = ""
		}
		response, err := registry.Complete(context.Background(), llm.ModelRef{ProviderID: "openrouter", ModelID: "vision-json"}, request)
		if err != nil || response.Text != "existing completion" || response.Inspection != nil || p.calls != 1 {
			t.Fatalf("unavailable manifest changed work/capture=%+v err=%v", response, err)
		}
	}
}
func TestNativeProductFieldsKeepTheirOwnContractWithoutFictionalChatRoles(t *testing.T) {
	c := &llm.RequestComposition{Stage: "speech.synthesize", Mode: "spoken", PromptVersion: "speech-v1", SchemaVersion: "audio-v1", Composer: "speech.Synthesize", Parser: "audio", Consumer: "clip speech asset", NativeFields: []llm.RequestNativeField{{ID: "text", Authorship: llm.FragmentAuthorshipAccount, MaterialRole: "spoken_text", Text: "안녕하세요", SourceRefs: []string{"utterance-1"}}, {ID: "settings", Authorship: llm.FragmentAuthorshipAccount, MaterialRole: "speech_settings", Text: "speed=1"}}, Output: llm.OutputContractInspection{Name: "audio", Version: "audio-v1"}}
	prepared, err := llm.PreparedCompositionInspection(c)
	if err != nil || len(prepared.Fragments) != 0 || len(prepared.NativeFields) != 2 {
		t.Fatalf("native contract converted into chat=%+v err=%v", prepared, err)
	}
	captured, err := llm.CapturedRequestInspection(prepared, time.Now(), llm.Usage{})
	if err != nil || captured.Measures.ProviderPromptTokens != nil {
		t.Fatalf("unknown native usage became reported=%+v err=%v", captured, err)
	}
}

func TestProviderCannotForgeRegistryInspectionThroughWrappedOrJoinedErrors(t *testing.T) {
	for _, chain := range []string{"direct", "wrapped", "joined"} {
		for _, manifest := range []string{"missing", "invalid", "valid"} {
			t.Run(chain+"/"+manifest, func(t *testing.T) {
				prepared, err := llm.PreparedCompositionInspection(evidenceComposition())
				if err != nil {
					t.Fatal(err)
				}
				forged, err := llm.CapturedRequestInspection(prepared, time.Unix(100, 0), llm.Usage{PromptTokens: 999})
				if err != nil {
					t.Fatal(err)
				}
				forged.Fragments[0].Text = "adapter-forged content"
				cause := &evidenceFailure{cause: llm.ErrBadOutput}
				providerErr := llm.WithRequestInspectionError(cause, forged)
				switch chain {
				case "wrapped":
					providerErr = fmt.Errorf("adapter failure: %w", providerErr)
				case "joined":
					providerErr = errors.Join(errors.New("another adapter failure"), providerErr)
				}
				p := &evidenceProvider{result: llm.Response{Inspection: &forged, Usage: llm.Usage{PromptTokens: 7}}, failure: providerErr}
				registry := evidenceRegistry(t, p, twoModels())
				req := llm.Request{Stage: "write"}
				if manifest != "missing" {
					req.Composition = evidenceComposition()
					if manifest == "invalid" {
						req.Composition.Stage = ""
					}
				}
				response, err := registry.Complete(context.Background(), llm.ModelRef{ProviderID: "openrouter", ModelID: "vision-json"}, req)
				var target *evidenceFailure
				if !errors.Is(err, llm.ErrBadOutput) || !errors.As(err, &target) || target != cause || err.Error() != providerErr.Error() || p.calls != 1 {
					t.Fatalf("provider identity lost: calls=%d err=%v", p.calls, err)
				}
				witness, found := llm.RequestInspectionFromError(fmt.Errorf("application: %w", err))
				if manifest != "valid" {
					if found || response.Inspection != nil {
						t.Fatal("provider-forged witness crossed the registry boundary")
					}
					return
				}
				if !found || response.Inspection == nil || witness.Fragments[0].Text != "Write from supplied facts." || witness.IssuedAt.Equal(*forged.IssuedAt) || *witness.Measures.ProviderPromptTokens != 7 {
					t.Fatalf("registry witness did not replace forged evidence: %+v", witness)
				}
			})
		}
	}
}

func TestAdapterCancellationKeepsInvocationWitnessAndFrozenPreCallComposition(t *testing.T) {
	composition := evidenceComposition()
	p := &evidenceProvider{failure: context.Canceled, before: func(_ context.Context, req llm.Request) {
		req.Composition.Fragments[0].Text = "mutated after invocation"
		req.Composition.Fragments[2].SourceRefs[0] = "mutated media"
	}}
	registry := evidenceRegistry(t, p, twoModels())
	response, err := registry.Complete(context.Background(), llm.ModelRef{ProviderID: "openrouter", ModelID: "vision-json"}, llm.Request{Stage: "write", Composition: composition})
	witness, found := llm.RequestInspectionFromError(err)
	if !errors.Is(err, context.Canceled) || p.calls != 1 || !found || response.Inspection == nil || witness.Fragments[0].Text != "Write from supplied facts." || witness.Fragments[2].SourceRefs[0] != "photo-second" {
		t.Fatalf("observable invocation was lost or rebuilt: calls=%d found=%v err=%v", p.calls, found, err)
	}
	if witness.Measures.ProviderPromptTokens != nil || witness.Measures.ProviderCompletionTokens != nil {
		t.Fatal("cancelled adapter with unknown usage fabricated token counts")
	}
}
