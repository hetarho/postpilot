package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/usage"
	usagestore "github.com/postpilot/backend/internal/usage/store"
	"github.com/postpilot/backend/internal/voice/spoken"
	app "github.com/postpilot/backend/internal/voice/spoken/app"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type qualificationFixtureCatalog struct {
	maximum   string
	published int
}

func (s *qualificationFixtureCatalog) Session(_ context.Context, owner, id string) (app.QualificationSession, error) {
	return app.QualificationSession{ID: "qualification", OwnerID: "alice", ProfileID: "curated", Revision: 1, MaximumUSD: s.maximum, ExpiresAt: time.Now().Add(time.Hour)}, nil
}
func (s *qualificationFixtureCatalog) Publish(context.Context, app.QualificationEvidence) error {
	s.published++
	return nil
}
func TestSpokenQualificationProductionJobsBoundCostReuseAndEvidence(t *testing.T) {
	f := spokenGenerationFresh(t)
	f.profiles.qualificationMaximum = "1"
	bytes, e := os.ReadFile("../../internal/llm/testdata/speech-tone.mp3")
	if e != nil {
		t.Fatal(e)
	}
	audio, e := llm.InspectSpeechAudio(t.Context(), bytes)
	if e != nil {
		t.Fatal(e)
	}
	f.provider.fixtureAudio = &audio
	input := f.input()
	input.QualificationSessionID = "qualification"
	d, e := f.library.CreateDraft(t.Context(), "alice", plan.Master, "qualified-voice", input)
	if e != nil {
		t.Fatal(e)
	}
	design := f.start(app.GenerationInput{Kind: spoken.JobKindDesign, DraftID: d.ID, Revision: d.Revision}, "qualified-design")
	if e = f.run(design); e != nil {
		t.Fatal(e)
	}
	d, e = f.library.GetDraft(t.Context(), "alice", d.ID)
	if e != nil {
		t.Fatal(e)
	}
	d = f.audition(d, "qualified-listen")
	confirm := f.start(app.GenerationInput{Kind: spoken.JobKindConfirm, DraftID: d.ID, Revision: d.Revision, CandidateID: d.SelectedCandidateID}, "qualified-confirm")
	if e = f.run(confirm); e != nil {
		t.Fatal(e)
	}
	sessions := &qualificationFixtureCatalog{maximum: "1"}
	h := app.NewQualificationHarness(f.generation, sessions, spokenQualificationAccounting{usagestore.New(f.handle.Writer, f.handle.Reader)})
	in := app.QualificationInput{OwnerID: "alice", SessionID: "qualification", DesignOperationID: design.ID, ConfirmOperationID: confirm.ID, FirstText: "김민지 님, 사과는 12,500원이고 밀가루는 250g입니다. OpenAI를 읽어 주세요.", SecondText: "박준호 님, 우유는 1.5L예요. 오후 2시에 만나요! USB-C도 챙기세요.", ChangedFirstText: "김민지 님, 사과는 13,500원이고 밀가루는 300g입니다. OpenAI를 읽어 주세요."}
	sessions.maximum = "0.01"
	if _, e = h.Plan(t.Context(), in); !errors.Is(e, app.ErrQualificationBudget) {
		t.Fatal("missing cumulative bound", e)
	}
	if f.provider.speech != 0 {
		t.Fatal("preflight synthesized")
	}
	sessions.maximum = "1"
	planned, e := h.Plan(t.Context(), in)
	if e != nil {
		t.Fatal(e)
	}
	if planned.NewCalls != 3 {
		t.Fatal(planned)
	}
	if _, e = h.Run(t.Context(), planned, false, planned.Digest); !errors.Is(e, app.ErrQualificationEvidence) {
		t.Fatal(e)
	}
	tampered := planned
	tampered.MaximumUSD = "0"
	if _, e = h.Run(t.Context(), tampered, true, tampered.Digest); !errors.Is(e, spoken.ErrConflict) || f.provider.speech != 0 {
		t.Fatal("changed approved ceiling was accepted", e)
	}
	for _, kind := range []string{spoken.JobKindDesign, spoken.JobKindConfirm, spoken.JobKindProbe} {
		f.queue.Register(kind, metered(f.generation.Run))
		f.queue.OnTerminal(kind, func(ctx context.Context, j job.Job, _ time.Time) error { return f.generation.OnTerminal(ctx, j) })
	}
	workerCtx, stop := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() { defer close(done); f.queue.Run(workerCtx) }()
	defer func() { stop(); <-done }()
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	report, e := h.Run(ctx, planned, true, planned.Digest)
	if e != nil {
		t.Fatal(e)
	}
	if f.provider.speech != 3 || report.Samples[1].AssetID != report.Probes[1].AssetIDs[1] || report.Audition.DurationMS <= 0 || len(report.Samples[0].SHA256) != 64 {
		t.Fatal("reuse/duration evidence missing", report)
	}
	replay, e := h.Run(ctx, planned, true, planned.Digest)
	if e != nil || f.provider.speech != 3 || replay.Probes[0].ID != report.Probes[0].ID {
		t.Fatal("explicit retry generated", e)
	}
	if e = h.Publish(ctx, report, "fixture-report"); !errors.Is(e, app.ErrQualificationEvidence) || sessions.published != 0 {
		t.Fatal("fixture flag qualified", e)
	}
	report.Review = app.QualificationReview{Reviewer: "fixture listener; no live qualification", ReviewedAt: time.Now(), AuditionAccepted: true, KoreanAccepted: true, ContinuityAccepted: true, Names: "fixture", PricesNumbers: "fixture", Units: "fixture", Punctuation: "fixture", MixedLanguage: "fixture", OmissionsRepeats: "fixture", Clipping: "fixture", Continuity: "fixture", ConfirmationCapacity: "fixture"}
	stale := report
	stale.Review.ReviewedAt = time.Now().Add(-time.Hour)
	stale.Probes[0].UpdatedAt = stale.Review.ReviewedAt
	stale.Probes[1].UpdatedAt = stale.Review.ReviewedAt
	if e = h.Publish(ctx, stale, "stale-fixture-report"); !errors.Is(e, app.ErrQualificationEvidence) || sessions.published != 0 {
		t.Fatal("file timestamps bypassed the saved human-review ordering", e)
	}
	if e = h.Publish(ctx, report, "fixture-report"); e != nil || sessions.published != 1 {
		t.Fatal("private verified evidence was not delivered to fake publisher", e)
	}
	// Publication in this test only reaches a fake; no real profile becomes ready.
	report.Probes[1].ID = report.Probes[0].ID
	if _, e = h.Audit(ctx, report); !errors.Is(e, app.ErrQualificationEvidence) {
		t.Fatal("unchanged script was accepted as a new request", e)
	}
}
func TestSpokenQualificationCLIRefusesUnapprovedOrPublicInputBeforePlatform(t *testing.T) {
	for _, mode := range []string{"run", "publish"} {
		if e := runSpokenQualification(t.Context(), []string{"--mode", mode, "--input", "missing"}, io.Discard); !errors.Is(e, app.ErrQualificationEvidence) {
			t.Fatal(e)
		}
	}
	path := filepath.Join(t.TempDir(), "private.json")
	if e := os.WriteFile(path, []byte(`{"version":1,"input":{"OwnerID":"alice"}}`), 0644); e != nil {
		t.Fatal(e)
	}
	if _, _, e := readSpokenQualificationFile(path); e == nil {
		t.Fatal("public input accepted")
	}
	if e := os.Chmod(path, 0600); e != nil {
		t.Fatal(e)
	}
	file, _, e := readSpokenQualificationFile(path)
	if e != nil || file.Input.OwnerID != "alice" {
		t.Fatal(file, e)
	}
	output := filepath.Join(t.TempDir(), "report.json")
	if e = writeSpokenQualificationFile(output, file); e != nil {
		t.Fatal(e)
	}
	stat, e := os.Stat(output)
	if e != nil || stat.Mode().Perm() != 0600 {
		t.Fatal("private output permissions", e)
	}
	if e = writeSpokenQualificationFile(output, file); e == nil {
		t.Fatal("existing report overwritten")
	}
}

func TestSpokenQualificationSessionCeilingSerializesConcurrentStarts(t *testing.T) {
	f := spokenGenerationFresh(t)
	f.profiles.qualificationMaximum = "0.02"
	input := f.input()
	input.QualificationSessionID = "qualification"
	drafts := make([]spoken.Draft, 2)
	quotes := make([]app.Quote, 2)
	inputs := make([]app.GenerationInput, 2)
	for i := range drafts {
		d, e := f.library.CreateDraft(t.Context(), "alice", plan.Master, fmt.Sprintf("bounded-%d", i), input)
		if e != nil {
			t.Fatal(e)
		}
		drafts[i] = d
		inputs[i] = app.GenerationInput{Kind: spoken.JobKindDesign, DraftID: d.ID, Revision: d.Revision}
		quotes[i], e = f.generation.Quote(t.Context(), "alice", plan.Master, inputs[i])
		if e != nil {
			t.Fatal(e)
		}
	}
	start := make(chan struct{})
	result := make(chan error, 2)
	for i := range inputs {
		go func() {
			<-start
			q := quotes[i]
			_, e := f.generation.Start(t.Context(), "alice", plan.Master, inputs[i], fmt.Sprintf("bounded-start-%d", i), usage.UnitApproval{QuoteID: q.ID, ApprovedMaxCredits: &q.MaximumCredits, CancellationPolicyVersion: 1})
			result <- e
		}()
	}
	close(start)
	allowed, refused := 0, 0
	for range 2 {
		e := <-result
		if e == nil {
			allowed++
		} else if errors.Is(e, spoken.ErrQualificationBudget) {
			refused++
		} else {
			t.Fatal(e)
		}
	}
	if allowed != 1 || refused != 1 || f.provider.design != 0 || f.count("SELECT count(*) FROM spoken_voice_operations") != 1 {
		t.Fatal("cumulative reservation was not once bounded", allowed, refused)
	}
}

func TestSpokenQualificationZeroCeilingAcceptsOnlyDocumentedFreeCalls(t *testing.T) {
	f := spokenGenerationFresh(t)
	f.profiles.qualificationMaximum = "0"
	f.profiles.zeroPriced = true
	input := f.input()
	input.QualificationSessionID = "qualification"
	d, e := f.library.CreateDraft(t.Context(), "alice", plan.Master, "free-qualified", input)
	if e != nil {
		t.Fatal(e)
	}
	in := app.GenerationInput{Kind: spoken.JobKindDesign, DraftID: d.ID, Revision: d.Revision}
	q, e := f.generation.Quote(t.Context(), "alice", plan.Master, in)
	if e != nil || q.MaximumCredits != 0 {
		t.Fatal("documented zero-price quote", q, e)
	}
	credits := q.MaximumCredits
	o, e := f.generation.Start(t.Context(), "alice", plan.Master, in, "free-qualified-start", usage.UnitApproval{QuoteID: q.ID, ApprovedMaxCredits: &credits, CancellationPolicyVersion: 1})
	if e != nil || o.QualificationReservedUSD != "0" {
		t.Fatal("zero-price qualification refused", o, e)
	}
	f.profiles.zeroPriced = false
	if _, e = f.generation.Start(t.Context(), "alice", plan.Master, in, "price-drift", usage.UnitApproval{QuoteID: q.ID, ApprovedMaxCredits: &credits, CancellationPolicyVersion: 1}); !errors.Is(e, spoken.ErrQualificationBudget) {
		t.Fatal("positive price crossed the zero ceiling", e)
	}
}
