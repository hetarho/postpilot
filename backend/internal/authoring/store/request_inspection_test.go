package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/authoring"
	"github.com/postpilot/backend/internal/llm"
)

type inspectionProvider struct {
	mu      sync.Mutex
	calls   int
	request llm.Request
	text    string
	failure error
	after   func()
}

func (*inspectionProvider) Name() string { return "p" }
func (p *inspectionProvider) Complete(_ context.Context, request llm.Request) (llm.Response, error) {
	p.mu.Lock()
	p.calls++
	p.request = request
	text, failure, after := p.text, p.failure, p.after
	p.mu.Unlock()
	if after != nil {
		after()
	}
	return llm.Response{Text: text, Usage: llm.Usage{PromptTokens: 17, CompletionTokens: 9, ReasoningTokens: 3, CostMicrousd: 981234, CostReported: true}}, failure
}

type inspectionSource struct{}

func (inspectionSource) Models() []llm.SourceModel {
	return []llm.SourceModel{{ModelID: "writer", Stages: []string{"write"}, Levels: map[string]string{"write": "value"}, StructuredOutput: true, ContextTokens: 131072, Reasoning: map[string]llm.ReasoningEffort{"write": llm.ReasoningHigh}}}
}
func (s inspectionSource) Lookup(id string) (llm.SourceModel, bool) {
	return s.Models()[0], id == "writer"
}

type inspectionRegistry struct{ registry *llm.Registry }

func (m inspectionRegistry) Resolve(ref llm.ModelRef) (llm.ModelInfo, bool) {
	return m.registry.Lookup(ref)
}
func (m inspectionRegistry) Complete(ctx context.Context, ref llm.ModelRef, request llm.Request) (llm.Response, error) {
	return m.registry.Complete(ctx, ref, request)
}
func (m inspectionRegistry) PrepareAuthoringRequest(ctx context.Context, _ string, ref llm.ModelRef, request llm.Request) (llm.RequestInspection, error) {
	return m.registry.Prepare(ctx, ref, request)
}
func (m inspectionRegistry) ModelForInspection(context.Context, string, string) (llm.ModelRef, bool, error) {
	return llm.ModelRef{ProviderID: "p", ModelID: "writer"}, true, nil
}
func inspectedFixture(t *testing.T) (harness, *inspectionProvider) {
	t.Helper()
	h := fixture(t)
	p := &inspectionProvider{}
	r, err := llm.Parse([]byte("providers:\n  - id: p\n    adapter: fake\n    base_url: https://private-provider.test\n    api_key_env: TEST_KEY\n"), func(string) string { return "PRIVATE_KEY" }, map[string]llm.AdapterFactory{"fake": func(llm.AdapterConfig) (llm.Provider, error) { return p, nil }}, inspectionSource{}, llm.Options{Timeout: time.Minute, MaxTokens: 8192})
	if err != nil {
		t.Fatal(err)
	}
	m := inspectionRegistry{r}
	h.svc = authoring.NewInspectedService(authoring.NewService(h.store, m, h.jobs, h.targets, budget{}, estimates{}), authoring.RequestInspectionDependencies{Captures: h.store, Models: m, Selections: m})
	return h, p
}
func inspectionInput(s authoring.Session, mode authoring.Mode, status llm.InspectionStatus) authoring.RequestInspectionInput {
	return authoring.RequestInspectionInput{SessionID: s.ID, Kind: s.Kind, Revision: s.Revision, Mode: mode, Status: status}
}
func startInspectionRun(t *testing.T, h harness, s authoring.Session, mode authoring.Mode, prompt string) (authoring.Session, authoring.Run) {
	t.Helper()
	id, state, err := h.svc.Start(t.Context(), "alice", authoring.Start{SessionID: s.ID, ExpectedRevision: s.Revision, RequestID: fmt.Sprintf("inspection-%s-%d", mode, s.Revision), Mode: mode, Prompt: prompt, WriteModel: llm.ModelRef{ProviderID: "p", ModelID: "writer"}})
	if err != nil {
		t.Fatal(err)
	}
	job, err := h.jobs.Get(t.Context(), "alice", id)
	if err != nil {
		t.Fatal(err)
	}
	return state, authoring.Run{ID: id, UserID: "alice", WriteModel: job.WriteModel, Payload: job.Payload}
}
func inspectionRaw(t *testing.T, h harness, id string) string {
	t.Helper()
	var raw string
	if err := h.db.Reader.QueryRow("SELECT snapshot FROM configuration_authoring_sessions WHERE id=?", id).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	return raw
}
func assertSafeInspection(t *testing.T, out llm.RequestInspection) {
	t.Helper()
	raw, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"PRIVATE_KEY", "private-provider.test", "CostMicrousd", "981234", "APIKey", "ExecutionPolicy"} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("unsafe inspection field %s", secret)
		}
	}
}

func TestAuthoringInspectionAllKindsAndModesUseCurrentMaterialWithoutMutation(t *testing.T) {
	for _, kind := range []authoring.Kind{authoring.PostTemplate, authoring.VideoTemplate, authoring.PostGuideline, authoring.VideoGuideline, authoring.WritingVoice} {
		for _, mode := range []authoring.Mode{authoring.Recommend, authoring.Refine} {
			t.Run(string(kind)+"/"+string(mode), func(t *testing.T) {
				h, p := inspectedFixture(t)
				s := create(t, h, kind)
				body := "current owner direction"
				if kind == authoring.WritingVoice {
					body = strings.Repeat("가", 250)
				}
				var err error
				s, err = h.svc.PatchDraft(t.Context(), authoring.DraftMutation{UserID: "alice", SessionID: s.ID, ExpectedRevision: s.Revision, OperationKey: "current-material", WorkingSource: authoring.Artifact{Name: "current draft", Body: body, Description: "owner description"}})
				if err != nil {
					t.Fatal(err)
				}
				before := inspectionRaw(t, h, s.ID)
				for _, status := range []llm.InspectionStatus{llm.InspectionCurrent, llm.InspectionPrepared} {
					in := inspectionInput(s, mode, status)
					in.Prompt = "owner latest request"
					in.CandidateCount = 4
					out, err := h.svc.InspectRequest(t.Context(), "alice", in)
					if err != nil || out.Status != status || out.Conditions == nil || *out.Conditions.ReasoningEffort != llm.ReasoningHigh || *out.Conditions.MaxCompletionTokens != 8192 {
						t.Fatalf("preview=%+v err=%v", out, err)
					}
					var prompts strings.Builder
					for _, f := range out.Fragments {
						prompts.WriteString(f.Text)
					}
					if !strings.Contains(prompts.String(), body) || !strings.Contains(prompts.String(), "owner latest request") || out.Mode != string(kind)+"/"+string(mode) {
						t.Fatalf("wrong current kind/material: %s", prompts.String())
					}
					if out.IssuedAt != nil || out.Measures.ProviderPromptTokens != nil || out.CallID != "" {
						t.Fatal("preview claimed issuance")
					}
					assertSafeInspection(t, out)
				}
				if p.calls != 0 || h.jobs.enqueues != 0 || h.targets.creates != 0 || inspectionRaw(t, h, s.ID) != before {
					t.Fatal("preview created work or mutated the session")
				}
				in := inspectionInput(s, authoring.Refine, llm.InspectionPrepared)
				out, err := h.svc.InspectRequest(t.Context(), "alice", in)
				if err != nil || out.Status != llm.InspectionUnavailable {
					t.Fatalf("missing instruction %+v %v", out, err)
				}
			})
		}
	}
}

func TestAuthoringInspectionCapturesActualEffectiveCallAndDoesNotReconcileOnRead(t *testing.T) {
	h, p := inspectedFixture(t)
	s := create(t, h, authoring.PostTemplate)
	active, run := startInspectionRun(t, h, s, authoring.Recommend, "actual private prompt")
	p.text = batch(s.Kind)
	prepared, err := h.svc.InspectRequest(t.Context(), "alice", inspectionInput(active, authoring.Recommend, llm.InspectionPrepared))
	if err != nil || prepared.Status != llm.InspectionPrepared {
		t.Fatalf("active preview %+v %v", prepared, err)
	}
	if err = h.svc.Run(t.Context(), run, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	h.jobs.status(run.ID, "done")
	before := inspectionRaw(t, h, s.ID)
	captured, err := h.svc.InspectRequest(t.Context(), "alice", inspectionInput(active, authoring.Recommend, llm.InspectionCaptured))
	if err != nil || captured.Status != llm.InspectionCaptured || captured.CallID != run.ID || *captured.Measures.ProviderPromptTokens != 17 || *captured.Conditions.ReasoningEffort != llm.ReasoningHigh {
		t.Fatalf("capture=%+v %v", captured, err)
	}
	if inspectionRaw(t, h, s.ID) != before || p.calls != 1 {
		t.Fatal("inspection reconciled or executed work")
	}
	if !reflect.DeepEqual(captured.Conditions, prepared.Conditions) || !reflect.DeepEqual(captured.Fragments, prepared.Fragments) || captured.Output.Schema != string(p.request.JSONSchema) {
		t.Fatal("capture diverged from actual composer/effective request")
	}
	assertSafeInspection(t, captured)
	s, err = h.svc.Get(t.Context(), "alice", s.ID)
	if err != nil {
		t.Fatal(err)
	}
	captured, err = h.svc.InspectRequest(t.Context(), "alice", inspectionInput(s, authoring.Recommend, llm.InspectionCaptured))
	if err != nil || captured.Status != llm.InspectionCaptured {
		t.Fatalf("completed capture %+v %v", captured, err)
	}
	selected, err := h.svc.Select(t.Context(), "alice", s.ID, s.Revision, s.Candidates[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	out, err := h.svc.InspectRequest(t.Context(), "alice", inspectionInput(selected, authoring.Recommend, llm.InspectionCaptured))
	if err != nil || out.Status != llm.InspectionUnavailable {
		t.Fatalf("old revision capture relabeled current %+v %v", out, err)
	}
}

func TestAuthoringInspectionFailureAndInvalidOutputKeepActualWitness(t *testing.T) {
	for _, providerFailure := range []bool{false, true} {
		t.Run(fmt.Sprint(providerFailure), func(t *testing.T) {
			h, p := inspectedFixture(t)
			s := create(t, h, authoring.PostGuideline)
			active, run := startInspectionRun(t, h, s, authoring.Recommend, "private request")
			p.text = "invalid output"
			if providerFailure {
				p.failure = errors.New("PRIVATE_PROVIDER_ERROR")
			}
			if err := h.svc.Run(t.Context(), run, func(string, int, int) {}); err == nil {
				t.Fatal("invalid call unexpectedly succeeded")
			}
			out, err := h.svc.InspectRequest(t.Context(), "alice", inspectionInput(active, authoring.Recommend, llm.InspectionCaptured))
			if err != nil || out.Status != llm.InspectionCaptured || *out.Measures.ProviderCompletionTokens != 9 {
				t.Fatalf("failure witness %+v %v", out, err)
			}
			assertSafeInspection(t, out)
			raw, _ := json.Marshal(out)
			if strings.Contains(string(raw), "PRIVATE_PROVIDER_ERROR") {
				t.Fatal("provider error leaked into capture")
			}
			h.jobs.status(run.ID, "failed")
			settled, err := h.svc.Get(t.Context(), "alice", s.ID)
			if err != nil {
				t.Fatal(err)
			}
			out, err = h.svc.InspectRequest(t.Context(), "alice", inspectionInput(settled, authoring.Recommend, llm.InspectionCaptured))
			if err != nil || out.Status != llm.InspectionCaptured {
				t.Fatalf("failed revision witness %+v %v", out, err)
			}
		})
	}
}

func TestAuthoringInspectionOwnerKindRevisionAndCorruptCaptureFences(t *testing.T) {
	h, p := inspectedFixture(t)
	s := create(t, h, authoring.PostGuideline)
	in := inspectionInput(s, authoring.Recommend, llm.InspectionCurrent)
	for _, id := range []string{s.ID, "unknown"} {
		in.SessionID = id
		if _, err := h.svc.InspectRequest(t.Context(), "bob", in); !errors.Is(err, authoring.ErrNotFound) {
			t.Fatalf("foreign/unknown %v", err)
		}
	}
	in = inspectionInput(s, authoring.Recommend, llm.InspectionCurrent)
	in.Kind = authoring.VideoGuideline
	if _, err := h.svc.InspectRequest(t.Context(), "alice", in); !errors.Is(err, authoring.ErrNotFound) {
		t.Fatalf("wrong kind %v", err)
	}
	in.Kind = s.Kind
	in.Revision++
	if _, err := h.svc.InspectRequest(t.Context(), "alice", in); !errors.Is(err, authoring.ErrStale) {
		t.Fatalf("wrong revision %v", err)
	}
	active, run := startInspectionRun(t, h, s, authoring.Recommend, "request")
	p.text = batch(s.Kind)
	if err := h.svc.Run(t.Context(), run, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	if _, err := h.db.Writer.Exec("UPDATE configuration_authoring_operations SET request_capture=? WHERE session_id=?", `{"Version":1,"Status":"captured","SDKBody":"secret"}`, s.ID); err != nil {
		t.Fatal(err)
	}
	out, err := h.svc.InspectRequest(t.Context(), "alice", inspectionInput(active, authoring.Recommend, llm.InspectionCaptured))
	if err != nil || out.Status != llm.InspectionUnavailable || len(out.Fragments) != 0 {
		t.Fatalf("corrupt capture %+v %v", out, err)
	}
}

func TestAuthoringInspectionConcurrentPurgeAndLateCapture(t *testing.T) {
	h, p := inspectedFixture(t)
	s := create(t, h, authoring.PostTemplate)
	active, run := startInspectionRun(t, h, s, authoring.Recommend, "request")
	p.text = batch(s.Kind)
	issued, release := make(chan struct{}), make(chan struct{})
	p.after = func() { close(issued); <-release }
	done := make(chan error, 1)
	go func() { done <- h.svc.Run(t.Context(), run, func(string, int, int) {}) }()
	<-issued
	if err := h.store.PurgeAuthoringRequestCaptures(t.Context(), "alice", s.ID); err != nil {
		t.Fatal(err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	out, err := h.svc.InspectRequest(t.Context(), "alice", inspectionInput(active, authoring.Recommend, llm.InspectionCaptured))
	if err != nil || out.Status != llm.InspectionUnavailable {
		t.Fatalf("purge restored capture %+v %v", out, err)
	}
	var purged int
	var raw *string
	if err = h.db.Reader.QueryRow("SELECT capture_purged,request_capture FROM configuration_authoring_operations WHERE session_id=?", s.ID).Scan(&purged, &raw); err != nil || purged != 1 || raw != nil {
		t.Fatalf("purge receipt=%d payload=%v error=%v", purged, raw, err)
	}
	// The same already-issued durable job cannot populate the purged operation
	// even when a worker replay reaches the provider again.
	p.after = nil
	if err = h.svc.Run(t.Context(), run, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	out, err = h.svc.InspectRequest(t.Context(), "alice", inspectionInput(active, authoring.Recommend, llm.InspectionCaptured))
	if err != nil || out.Status != llm.InspectionUnavailable {
		t.Fatal("replayed callback bypassed durable purge fence")
	}
}

func TestAuthoringInspectionConcurrentAccountDeletionFencesCapture(t *testing.T) {
	h, p := inspectedFixture(t)
	s := create(t, h, authoring.PostGuideline)
	active, run := startInspectionRun(t, h, s, authoring.Recommend, "request")
	p.text = batch(s.Kind)
	issued, release := make(chan struct{}), make(chan struct{})
	p.after = func() { close(issued); <-release }
	done := make(chan error, 1)
	go func() { done <- h.svc.Run(t.Context(), run, func(string, int, int) {}) }()
	<-issued
	if _, err := h.db.Writer.Exec("DELETE FROM users WHERE id='alice'"); err != nil {
		t.Fatal(err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if _, err := h.svc.InspectRequest(t.Context(), "alice", inspectionInput(active, authoring.Recommend, llm.InspectionCaptured)); !errors.Is(err, authoring.ErrNotFound) {
		t.Fatalf("deleted account evidence returned %v", err)
	}
	var count int
	if err := h.db.Reader.QueryRow("SELECT COUNT(*) FROM configuration_authoring_operations").Scan(&count); err != nil || count != 0 {
		t.Fatalf("private operations survived account deletion %d %v", count, err)
	}
	if err := h.svc.Run(t.Context(), run, func(string, int, int) {}); !errors.Is(err, authoring.ErrNotFound) {
		t.Fatalf("deleted session replay %v", err)
	}
}

func TestAuthoringInspectionRefinementCaptureRemainsSeparateFromPublication(t *testing.T) {
	h, p := inspectedFixture(t)
	s, err := h.svc.Create(t.Context(), "alice", authoring.PostGuideline, "owned", "edit-existing")
	if err != nil {
		t.Fatal(err)
	}
	_, run := startInspectionRun(t, h, s, authoring.Refine, "make the existing direction concise")
	p.text = `{"artifact":{"name":"updated guideline","body":"new concise direction"},"reply":"작성 방향을 간결하게 바꿨어요."}`
	if err = h.svc.Run(t.Context(), run, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	h.jobs.status(run.ID, "done")
	s, err = h.svc.Get(t.Context(), "alice", s.ID)
	if err != nil {
		t.Fatal(err)
	}
	out, err := h.svc.InspectRequest(t.Context(), "alice", inspectionInput(s, authoring.Refine, llm.InspectionCaptured))
	if err != nil || out.Status != llm.InspectionCaptured || s.WorkingSource.Body != "new concise direction" {
		t.Fatalf("refine capture %+v %v", out, err)
	}
	var text strings.Builder
	for _, f := range out.Fragments {
		text.WriteString(f.Text)
	}
	if !strings.Contains(text.String(), "old body") || strings.Contains(text.String(), "new concise direction") {
		t.Fatal("captured input was reconstructed from resulting draft")
	}
	if h.targets.creates != 0 {
		t.Fatal("capture published setting")
	}
	saved, err := h.svc.Save(t.Context(), "alice", s.ID, s.Revision, false)
	if err != nil || saved.Saved == nil || h.targets.creates != 1 {
		t.Fatalf("explicit publication %+v %v", saved, err)
	}
	out, err = h.svc.InspectRequest(t.Context(), "alice", inspectionInput(saved, authoring.Refine, llm.InspectionCaptured))
	if err != nil || out.Status != llm.InspectionUnavailable || out.Conditions != nil {
		t.Fatal("saved provenance was relabeled issued capture")
	}
}

func TestAuthoringInspectionConcurrentManualEditCannotBindLateCaptureToNewDraft(t *testing.T) {
	h, p := inspectedFixture(t)
	s, err := h.svc.Create(t.Context(), "alice", authoring.PostGuideline, "owned", "edit-existing")
	if err != nil {
		t.Fatal(err)
	}
	active, run := startInspectionRun(t, h, s, authoring.Refine, "refine direction")
	p.text = `{"artifact":{"name":"generated guideline","body":"old generated direction"},"reply":"작성 방향을 바꿨어요."}`
	issued, release := make(chan struct{}), make(chan struct{})
	p.after = func() { close(issued); <-release }
	done := make(chan error, 1)
	go func() { done <- h.svc.Run(t.Context(), run, func(string, int, int) {}) }()
	<-issued
	manual, err := h.svc.PatchDraft(t.Context(), authoring.DraftMutation{UserID: "alice", SessionID: s.ID, ExpectedRevision: active.Revision, OperationKey: "new-manual-work", WorkingSource: authoring.Artifact{Name: "manual guideline", Body: "new owner direction"}})
	if err != nil {
		t.Fatal(err)
	}
	close(release)
	if err = <-done; err != nil {
		t.Fatal(err)
	}
	out, err := h.svc.InspectRequest(t.Context(), "alice", inspectionInput(manual, authoring.Refine, llm.InspectionCaptured))
	if err != nil || out.Status != llm.InspectionUnavailable {
		t.Fatal("late capture adopted newer manual revision")
	}
	h.jobs.status(run.ID, "done")
	settled, err := h.svc.Get(t.Context(), "alice", s.ID)
	if err != nil || settled.WorkingSource.Body != "new owner direction" {
		t.Fatalf("late canonical reply overwrote owner source %+v %v", settled, err)
	}
	out, err = h.svc.InspectRequest(t.Context(), "alice", inspectionInput(settled, authoring.Refine, llm.InspectionCaptured))
	if err != nil || out.Status != llm.InspectionUnavailable {
		t.Fatal("superseded completion bound old witness to current work")
	}
}

func TestAuthoringInspectionCanceledIssuedCallKeepsWitnessAndPurgeIsIrreversible(t *testing.T) {
	h, p := inspectedFixture(t)
	s := create(t, h, authoring.PostGuideline)
	active, run := startInspectionRun(t, h, s, authoring.Recommend, "request")
	p.text = batch(s.Kind)
	ctx, cancel := context.WithCancel(t.Context())
	p.after = cancel
	if err := h.svc.Run(ctx, run, func(string, int, int) {}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled call %v", err)
	}
	out, err := h.svc.InspectRequest(t.Context(), "alice", inspectionInput(active, authoring.Recommend, llm.InspectionCaptured))
	if err != nil || out.Status != llm.InspectionCaptured {
		t.Fatalf("issued cancellation lost safe witness %+v %v", out, err)
	}
	h.jobs.status(run.ID, "cancelled")
	settled, err := h.svc.Get(t.Context(), "alice", s.ID)
	if err != nil {
		t.Fatal(err)
	}
	out, err = h.svc.InspectRequest(t.Context(), "alice", inspectionInput(settled, authoring.Recommend, llm.InspectionCaptured))
	if err != nil || out.Status != llm.InspectionCaptured {
		t.Fatal("cancellation did not bind terminal revision")
	}
	if err = h.store.PurgeAuthoringRequestCaptures(t.Context(), "alice", s.ID); err != nil {
		t.Fatal(err)
	}
	if err = h.store.PurgeAuthoringRequestCaptures(t.Context(), "alice", s.ID); err != nil {
		t.Fatal(err)
	}
	out, err = h.svc.InspectRequest(t.Context(), "alice", inspectionInput(settled, authoring.Recommend, llm.InspectionCaptured))
	if err != nil || out.Status != llm.InspectionUnavailable || len(out.Fragments) != 0 {
		t.Fatal("purged private payload reconstructed")
	}
	if p.calls != 1 {
		t.Fatal("cancel/purge/inspection created provider work")
	}
}
