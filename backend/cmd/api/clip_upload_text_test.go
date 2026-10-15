package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	clipapp "github.com/postpilot/backend/internal/clip/app"
	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/platform/config"
	"github.com/postpilot/backend/internal/template"
	"github.com/postpilot/backend/internal/usage"
)

type uploadFixtureProvider struct {
	inspectionProvider
	gate    <-chan struct{}
	entered chan<- struct{}
}

func (p *uploadFixtureProvider) FreezePricing(_ context.Context, policy llm.CallPolicy, delivery llm.ExecutionDelivery) (llm.CallPolicy, error) {
	if delivery != llm.ExecutionTextOnly {
		return llm.CallPolicy{}, llm.ErrUnsupported
	}
	policy.Pricing = llm.CallPricing{Version: llm.CallPricingVersion, Fingerprint: strings.Repeat("a", 64), Delivery: delivery, Endpoint: "fixture-text", RequiredParameters: strings.Join(policy.RequiredParameters(), ","), PromptUSDPerMillion: "0", CompletionUSDPerMillion: "0", RequestUSD: "0", ImageUSD: "0", AudioUSDPerToken: "0", AggregateUsageSufficient: true}
	return policy, nil
}

func (p *uploadFixtureProvider) Complete(_ context.Context, request llm.Request) (llm.Response, error) {
	p.mu.Lock()
	p.requests = append(p.requests, request)
	p.mu.Unlock()
	if p.entered != nil {
		p.entered <- struct{}{}
	}
	if p.gate != nil {
		<-p.gate
	}
	return llm.Response{Text: `{"body":"내가 남긴 방문 기록이에요.","tags":["방문"]}`, FinishReason: "stop", Usage: llm.Usage{PromptTokens: 20, CompletionTokens: 15, CostReported: true}}, nil
}
func uploadBoot(t *testing.T) (*contexts, *platform, *uploadFixtureProvider) {
	t.Helper()
	t.Setenv("MAIL_DRIVER", "log")
	cfg, e := config.Load()
	if e != nil {
		t.Fatal(e)
	}
	p := wiringPlatform(t, cfg)
	provider := &uploadFixtureProvider{}
	p.registry, e = llm.Parse([]byte("providers:\n  - id: fixture\n    adapter: fixture\n    base_url: https://fixture.invalid\n    api_key_env: FIXTURE_KEY\n"), func(string) string { return "fixture" }, map[string]llm.AdapterFactory{"fixture": func(llm.AdapterConfig) (llm.Provider, error) { return provider, nil }}, inspectionModels{}, llm.Options{Timeout: time.Minute, MaxTokens: 32768})
	if e != nil {
		t.Fatal(e)
	}
	p.registry.WithModelGrades()
	for _, user := range []string{"alice", "bob"} {
		if _, e = p.db.Writer.Exec(`INSERT INTO users(id,password_hash,created_at) VALUES(?,?,?)`, user, "fixture", time.Now().UTC().Format(time.RFC3339Nano)); e != nil {
			t.Fatal(e)
		}
	}
	app, e := buildContexts(t.Context(), p)
	if e != nil {
		t.Fatal(e)
	}
	registerJobs(app)
	return app, p, provider
}

func TestDefaultVoiceConsumersRunAfterTheRealCompositionBoot(t *testing.T) {
	app, _, provider := uploadBoot(t)
	start := template.HeadingSuggestionStart{WriteModel: "fixture/writer", TargetKind: "clip", TargetPath: []string{"맛집", "식사"}, Sections: []template.HeadingSection{{Role: "훅", Description: "첫 장면"}}}
	id, e := app.templateHeadings.Start(t.Context(), "alice", start)
	if e != nil || id == "" {
		t.Fatal("default voice was not ready before the heading consumer", id, e)
	}
	if j, e := app.jobs.Get(t.Context(), id, "alice"); e != nil || j.Status != job.StatusQueued {
		t.Fatal(j, e)
	}
	provider.mu.Lock()
	defer provider.mu.Unlock()
	if len(provider.requests) != 0 {
		t.Fatal("starting an unrun suggestion issued a provider call")
	}
}

func TestClipUploadTextRegistrationAdmissionAndTerminalSettlement(t *testing.T) {
	app, p, provider := uploadBoot(t)
	project, e := app.clip.CreateProject(t.Context(), "alice", clip.ProjectInput{Title: "내 방문", Language: "ko", Ratio: "vertical", Instruction: "내가 겪은 방문"})
	if e != nil {
		t.Fatal(e)
	}
	plan, e := clip.EncodeEditPlan(clip.EditPlan{Ratio: "vertical", DurationMS: 15000, Cuts: []clip.Cut{{ID: "cut", SourceID: "expired-original", Fingerprint: strings.Repeat("a", 64), EndMS: 15000, Copies: []clip.Copy{{Text: "내가 쓴 자막"}}}}})
	if e != nil {
		t.Fatal(e)
	}
	stamp := time.Now().UTC().Format(time.RFC3339Nano)
	_, e = p.db.Writer.Exec(`UPDATE clip_projects SET edit_plan_json=?,edit_plan_revision=1,rendered_plan_revision=1,generated_plan_revision=1,result_id='original-result',result_key='original-result',result_content_type='video/mp4',result_bytes=1,result_duration_ms=15000,result_created_at=?,finalized_at=?,finalized_plan_revision=1,finalized_result_key='original-result',source_access_revoked_at=? WHERE id=?`, plan, stamp, stamp, stamp, project.ID)
	if e != nil {
		t.Fatal(e)
	}
	quote, e := app.clipGeneration.QuoteUploadText(t.Context(), "alice", project.ID, clip.ChannelNaver, "fixture/writer", false)
	if e != nil {
		t.Fatal(e)
	}
	if quote.Pricing.PlanCalls() != 1 || quote.Pricing.Observe.Ref != (llm.ModelRef{}) || quote.Pricing.ObservationCalls != 0 || !quote.Pricing.UploadText {
		t.Fatal(quote.Pricing)
	}
	approval := clip.QuoteApproval{QuoteID: quote.ID, MaxCredits: &quote.Pricing.MaxCredits, CancellationPolicyVersion: clip.CancellationPolicyVersion}
	id, e := app.clipGeneration.StartUploadText(t.Context(), "alice", project.ID, clip.ChannelNaver, "fixture/writer", false, approval)
	if e != nil {
		t.Fatal(e)
	}
	ctx, stop := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() { defer close(done); app.jobs.Run(ctx) }()
	defer func() { stop(); <-done }()
	deadline := time.Now().Add(15 * time.Second)
	for {
		j, e := app.jobs.Get(t.Context(), id, "alice")
		if e != nil {
			t.Fatal(e)
		}
		if job.Terminal(j.Status) {
			if j.Status != job.StatusDone {
				t.Fatalf("upload text job failed: %+v", j)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("upload text did not become terminal")
		}
		time.Sleep(10 * time.Millisecond)
	}
	for {
		a, e := app.clipGeneration.Accounting(t.Context(), "alice", project.ID)
		if e != nil {
			t.Fatal(e)
		}
		if a != nil && a.Settled {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("upload text settlement missing: %+v", a)
		}
		time.Sleep(10 * time.Millisecond)
	}
	rows, e := app.clipGeneration.UploadTexts(t.Context(), "alice", project.ID)
	if e != nil || len(rows) != 1 || rows[0].Revision != 1 || rows[0].Edited {
		t.Fatal(rows, e)
	}
	read, e := app.clipStore.GetProject(t.Context(), "alice", project.ID)
	if e != nil || read.EditPlanRevision != 1 || read.EditPlan != plan || read.Result == nil || read.Result.ID != "original-result" || read.Finalized == nil {
		t.Fatal("writing text changed the finished clip", read, e)
	}
	duplicate, e := app.clipGeneration.StartUploadText(t.Context(), "alice", project.ID, clip.ChannelNaver, "fixture/writer", false, approval)
	if e != nil || duplicate != id {
		t.Fatal("approval was consumed twice", duplicate, e)
	}
	if _, e = app.clipGeneration.UploadTexts(t.Context(), "bob", project.ID); !errors.Is(e, clip.ErrNotFound) {
		t.Fatal("foreign text disclosed", e)
	}
	provider.mu.Lock()
	defer provider.mu.Unlock()
	if len(provider.requests) != 1 {
		t.Fatal("upload text was not exactly one call", len(provider.requests))
	}
	req := provider.requests[0]
	if req.Stage != llm.StageNameWrite || req.Composition.Mode != "upload-text" || strings.Contains(req.System, clip.UploadTextDisclosure("ko")) {
		t.Fatal("wrong writing contract", req.Stage, req.Composition.Mode)
	}
}

func TestCancelledUploadTextNeverPublishesItsCandidate(t *testing.T) {
	app, p, provider := uploadBoot(t)
	project, e := app.clip.CreateProject(t.Context(), "alice", clip.ProjectInput{Title: "취소할 문구", Language: "ko", Ratio: "vertical"})
	if e != nil {
		t.Fatal(e)
	}
	plan, e := clip.EncodeEditPlan(clip.EditPlan{Ratio: "vertical", DurationMS: 15000, Cuts: []clip.Cut{{ID: "cut", SourceID: "expired", Fingerprint: strings.Repeat("a", 64), EndMS: 15000}}})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = p.db.Writer.Exec(`UPDATE clip_projects SET edit_plan_json=?,edit_plan_revision=1 WHERE id=?`, plan, project.ID); e != nil {
		t.Fatal(e)
	}
	quote, e := app.clipGeneration.QuoteUploadText(t.Context(), "alice", project.ID, clip.ChannelNaver, "fixture/writer", false)
	if e != nil {
		t.Fatal(e)
	}
	id, e := app.clipGeneration.StartUploadText(t.Context(), "alice", project.ID, clip.ChannelNaver, "fixture/writer", false, clip.QuoteApproval{QuoteID: quote.ID, MaxCredits: &quote.Pricing.MaxCredits, CancellationPolicyVersion: 1})
	if e != nil {
		t.Fatal(e)
	}
	gate := make(chan struct{})
	entered := make(chan struct{}, 1)
	provider.gate = gate
	provider.entered = entered
	ctx, stop := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() { defer close(done); app.jobs.Run(ctx) }()
	defer func() { stop(); <-done }()
	released := false
	defer func() {
		if !released {
			close(gate)
		}
	}()
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("writer did not enter")
	}
	if _, e = app.jobs.Cancel(t.Context(), "alice", job.Subject{Dimension: clip.JobSubject, ID: project.ID}, id); e != nil {
		t.Fatal(e)
	}
	close(gate)
	released = true
	deadline := time.Now().Add(10 * time.Second)
	for {
		j, e := app.jobs.Get(t.Context(), id, "alice")
		if e != nil {
			t.Fatal(e)
		}
		if job.Terminal(j.Status) {
			if j.Status != job.StatusCancelled {
				t.Fatal(j)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("cancellation did not settle")
		}
		time.Sleep(10 * time.Millisecond)
	}
	rows, e := app.clipGeneration.UploadTexts(t.Context(), "alice", project.ID)
	if e != nil || len(rows) != 0 {
		t.Fatal("cancelled candidate was published", rows, e)
	}
	read, e := app.clipStore.GetProject(t.Context(), "alice", project.ID)
	if e != nil || read.EditPlan != plan || read.EditPlanRevision != 1 {
		t.Fatal("cancellation changed the video", read, e)
	}
}

func TestUploadTextPaidAllowanceHoldsOneCallAndSettlesItsRemainder(t *testing.T) {
	h := newCancellationHarness(t, nil)
	reserve := cancellationReservation()
	reserve.Calls = reserve.Calls[1:]
	pricing := clip.GenerationPricing{UploadText: true, Plan: reserve.Calls[0].Policy, Rate: reserve.Rate}
	var e error
	reserve.ApprovedMaxCredits, e = clipapp.QuoteCredits(pricing, 0, 0)
	if e != nil || reserve.ApprovedMaxCredits <= 1 {
		t.Fatal(reserve, e)
	}
	id, e := h.queue.Enqueue(t.Context(), clipJob(job.NewJob{UserID: "alice", Kind: clip.JobKindUploadText, WriteModel: "p/w", CancellationPolicyVersion: 1}, "clip"))
	if e != nil {
		t.Fatal(e)
	}
	terminal := h.run(t, clip.JobKindUploadText, func(ctx context.Context, j job.Job, _ job.Progress) error {
		admitted, e := clipapp.NewJobs(h.queue, h.guard).Reserve(ctx, "alice", j.ID, []job.PlannedCall{{Ref: "p/w", Stage: "write", CompletionTokens: 32768, Count: 1}}, reserve)
		if e != nil {
			return e
		}
		if _, e = clipapp.ConsumePolicy(admitted, "alice", j.ID, "p/w", 32768, "write"); e != nil {
			return e
		}
		if _, e = clipapp.ConsumePolicy(admitted, "alice", j.ID, "p/w", 32768, "write"); !errors.Is(e, clip.ErrCreditAllowance) {
			return errors.New("second upload text call admitted")
		}
		work := usage.WithWork(ctx, usage.Work{UserID: "alice", Kind: clip.JobKindUploadText, JobID: j.ID})
		return h.ledger.RecordCall(work, llm.ModelRef{ProviderID: "p", ModelID: "w"}, "write", llm.Usage{CostReported: true, CostMicrousd: 100}, nil)
	})
	if e = h.queue.Activate(t.Context(), "alice", id); e != nil {
		t.Fatal(e)
	}
	awaitCancellationSignal(t, terminal)
	j, e := h.jobs.GetByID(t.Context(), id)
	if e != nil || j.Status != job.StatusDone {
		t.Fatal(j, e)
	}
	accounting, e := h.ledger.ReservationAccounting(t.Context(), "alice", id)
	if e != nil || accounting == nil || !accounting.Settled || accounting.NominalReservation == nil || *accounting.NominalReservation != reserve.ApprovedMaxCredits || accounting.FinalCharge == nil || *accounting.FinalCharge != 1 || accounting.Refund == nil || *accounting.Refund != reserve.ApprovedMaxCredits-1 {
		t.Fatal(accounting, e)
	}
}
