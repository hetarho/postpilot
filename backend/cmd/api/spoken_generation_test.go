package main

import (
	"bytes"
	"connectrpc.com/connect"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/postpilot/backend/internal/auth"
	authstore "github.com/postpilot/backend/internal/auth/store"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/job"
	jobstore "github.com/postpilot/backend/internal/job/store"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/mail"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/usage"
	usagestore "github.com/postpilot/backend/internal/usage/store"
	"github.com/postpilot/backend/internal/voice/spoken"
	spokenapp "github.com/postpilot/backend/internal/voice/spoken/app"
	spokenrpc "github.com/postpilot/backend/internal/voice/spoken/rpc"
	spokenstore "github.com/postpilot/backend/internal/voice/spoken/store"
	"google.golang.org/protobuf/encoding/protojson"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type spokenTestProfiles struct {
	qualificationMaximum string
	profile              spoken.Profile
	unavailable          bool
	paidConfirmation     bool
	zeroPriced           bool
}

func (p *spokenTestProfiles) Session(_ context.Context, owner, id string) (spokenapp.QualificationSession, error) {
	if owner != "alice" || id != "qualification" {
		return spokenapp.QualificationSession{}, spokenapp.ErrQualificationOnly
	}
	maximum := p.qualificationMaximum
	if maximum == "" {
		maximum = "100"
	}
	return spokenapp.QualificationSession{ID: id, OwnerID: owner, ProfileID: p.profile.ID, Revision: p.profile.Revision, MaximumUSD: maximum, ExpiresAt: time.Now().Add(time.Hour)}, nil
}
func (p *spokenTestProfiles) ResolveSpokenProfile(_ context.Context, owner string, tier plan.Plan, id string, rev int64, session string) (spoken.Profile, error) {
	if p.unavailable || id != p.profile.ID || rev != p.profile.Revision {
		return spoken.Profile{}, spoken.ErrConflict
	}
	if session != "" && (owner != "alice" || tier != plan.Master || session != "qualification") {
		return spoken.Profile{}, spokenapp.ErrQualificationOnly
	}
	return p.profile, nil
}
func (p *spokenTestProfiles) Budget(ctx context.Context, owner string, tier plan.Plan, id string, rev int64, session, scope string, in llm.SpeechInput, n int) (usage.UnitBudget, error) {
	if _, err := p.ResolveSpokenProfile(ctx, owner, tier, id, rev, session); err != nil {
		return usage.UnitBudget{}, err
	}
	price := "0.0001"
	unit := llm.SpeechUnitCharacterCost
	maxUnits := "1000"
	factor := "1"
	if in.Operation == spoken.JobKindConfirm {
		price = "0"
		unit = llm.SpeechUnitRequests
		maxUnits = "1"
		factor = ""
		if p.paidConfirmation {
			price = "0.001"
		}
	}
	if p.zeroPriced {
		price = "0"
	}
	return usage.UnitBudget{PolicyID: id, Revision: rev, AuthorizationID: session, ScopeDigest: scope, Ref: in.Ref, Operation: in.Operation, InputDigest: in.Digest, Count: n, InputCharacters: in.InputCharacters, AuxiliaryCharacters: in.AuxiliaryCharacters, ParametersDigest: in.ParametersDigest, Source: "https://example.com/prices", BoundsSource: "https://example.com/limits", CheckedAt: time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC), Complete: true, Tariffs: []usage.UnitTariff{{Unit: unit, USDPerUnit: price, Multiplier: "1", MaximumUnits: maxUnits, UnitsPerInputCharacter: factor}}}, nil
}
func (p *spokenTestProfiles) ValidateUnitBudget(ctx context.Context, owner string, tier plan.Plan, b usage.UnitBudget) error {
	current, err := p.Budget(ctx, owner, tier, b.PolicyID, b.Revision, b.AuthorizationID, b.ScopeDigest, llm.SpeechInput{Ref: b.Ref, Operation: b.Operation, Digest: b.InputDigest, InputCharacters: b.InputCharacters, AuxiliaryCharacters: b.AuxiliaryCharacters, ParametersDigest: b.ParametersDigest}, b.Count)
	if err != nil {
		return err
	}
	if current.Fingerprint() != b.Fingerprint() {
		return usage.ErrUnitPricing
	}
	return nil
}

type spokenTestRates struct{}

func (spokenTestRates) KRWPerUSD(context.Context, time.Time) (int64, bool, error) {
	return 14800000, true, nil
}

type spokenTestAnchors struct{}

func (spokenTestAnchors) AnchorFor(context.Context, string) (time.Time, error) {
	return time.Now(), nil
}
func (spokenTestAnchors) CoverageFor(context.Context, string, time.Time) (usage.Coverage, bool, error) {
	return usage.Coverage{}, false, nil
}

type spokenTestObjects struct {
	data     map[string][]byte
	afterPut func()
	fail     bool
}

func (o *spokenTestObjects) PutSpokenAudio(_ context.Context, key string, b []byte) error {
	if o.fail {
		return errors.New("storage down")
	}
	if _, ok := o.data[key]; ok {
		return errors.New("immutable")
	}
	o.data[key] = bytes.Clone(b)
	if o.afterPut != nil {
		fn := o.afterPut
		o.afterPut = nil
		fn()
	}
	return nil
}
func (o *spokenTestObjects) ReadSpokenAudio(_ context.Context, key string, _ int64) ([]byte, error) {
	b, ok := o.data[key]
	if !ok {
		return nil, errors.New("missing")
	}
	return bytes.Clone(b), nil
}
func (o *spokenTestObjects) DeleteSpokenAudio(_ context.Context, key string) error {
	delete(o.data, key)
	return nil
}
func spokenTestAudio(text string) llm.EncodedAudio {
	b := []byte("validated-adapter-mp3:" + text)
	h := sha256.Sum256(b)
	return llm.EncodedAudio{Bytes: b, SHA256: hex.EncodeToString(h[:]), Format: llm.SpeechOutputFormat, Samples: 44100, SampleRate: 44100, Channels: 2}
}

type spokenTestProvider struct {
	fixtureAudio            *llm.EncodedAudio
	design, confirm, speech int
	lastCandidate           llm.CandidateHandle
	lastVoice               llm.VoiceHandle
	confirmErr              error
	loseHandle              bool
	hook                    func()
}

func (p *spokenTestProvider) audio(text string) llm.EncodedAudio {
	if p.fixtureAudio != nil {
		return *p.fixtureAudio
	}
	return spokenTestAudio(text)
}
func (p *spokenTestProvider) DesignVoice(_ context.Context, r llm.VoiceDesignRequest) (llm.VoiceDesignResponse, error) {
	p.design++
	if p.hook != nil {
		fn := p.hook
		p.hook = nil
		fn()
	}
	out := llm.VoiceDesignResponse{Evidence: llm.SpeechEvidence{RequestID: fmt.Sprintf("design-%d", p.design), Units: []llm.SpeechUnitEvidence{{Unit: llm.SpeechUnitCharacterCost, Quantity: fmt.Sprint(len([]rune(r.PreviewText)))}}}}
	for i := 0; i < 3; i++ {
		out.Candidates = append(out.Candidates, llm.VoiceCandidate{Handle: llm.CandidateHandle(fmt.Sprintf("private-candidate-%d-%d", p.design, i)), Audio: p.audio(fmt.Sprintf("%d-%d", p.design, i))})
	}
	return out, nil
}
func (p *spokenTestProvider) ConfirmVoice(_ context.Context, r llm.VoiceConfirmationRequest) (llm.VoiceConfirmationResponse, error) {
	p.confirm++
	p.lastCandidate = r.Candidate
	if p.hook != nil {
		fn := p.hook
		p.hook = nil
		fn()
	}
	handle := llm.VoiceHandle(fmt.Sprintf("private-confirmed-%d", p.confirm))
	if p.loseHandle {
		handle = ""
	}
	return llm.VoiceConfirmationResponse{Voice: handle, Evidence: llm.SpeechEvidence{RequestID: fmt.Sprintf("confirm-%d", p.confirm), Units: []llm.SpeechUnitEvidence{{Unit: llm.SpeechUnitRequests, Quantity: "1"}}}}, p.confirmErr
}
func (p *spokenTestProvider) SynthesizeSpeech(_ context.Context, r llm.SpeechRequest) (llm.SpeechResponse, error) {
	p.speech++
	p.lastVoice = r.Voice
	return llm.SpeechResponse{Audio: p.audio(r.Text), Evidence: llm.SpeechEvidence{RequestID: fmt.Sprintf("speech-%d", p.speech), Units: []llm.SpeechUnitEvidence{{Unit: llm.SpeechUnitCharacterCost, Quantity: fmt.Sprint(len([]rune(r.Text)))}}}}, nil
}

type spokenTestPublisher struct {
	base spokenPublishLibraries
	fail bool
}
type spokenFailPublication struct{ spoken.OperationStorage }

func (s spokenFailPublication) Mutate(context.Context, spoken.RequestIdentity, func(spoken.Storage) (spoken.MutationResult, error)) (spoken.MutationResult, error) {
	return spoken.MutationResult{}, errors.New("publication temporarily down")
}
func (p *spokenTestPublisher) LibraryForOperation(id string) *spoken.Service {
	if p.fail {
		return spoken.NewService(spokenFailPublication{p.base.base.Operations}, p.base.base.Profiles, p.base.base.Objects)
	}
	return p.base.LibraryForOperation(id)
}

type spokenGenerationFixture struct {
	t          *testing.T
	handle     *db.DB
	path       string
	store      *spokenstore.Store
	jobs       *jobstore.Store
	queue      *job.Queue
	library    *spoken.Service
	generation *spokenapp.GenerationService
	profiles   *spokenTestProfiles
	provider   *spokenTestProvider
	objects    *spokenTestObjects
	publisher  *spokenTestPublisher
	ledger     *usage.Service
	admission  jobAdmission
}

func spokenGenerationFresh(t *testing.T) *spokenGenerationFixture {
	t.Helper()
	path := filepath.Join(t.TempDir(), "spoken-jobs.db")
	h, err := db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { h.Close() })
	if err := db.Migrate(t.Context(), h.Writer); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, id := range []string{"alice", "bob"} {
		if _, err := h.Writer.Exec("INSERT INTO users(id,password_hash,plan,created_at) VALUES (?,?,'master',?)", id, "hash", now); err != nil {
			t.Fatal(err)
		}
	}
	p := &spokenTestProfiles{profile: spoken.Profile{ID: "curated", Revision: 1, ConnectionScope: strings.Repeat("a", 64), Design: llm.ModelRef{ProviderID: "speech", ModelID: "design"}, Synthesis: llm.ModelRef{ProviderID: "speech", ModelID: "tts"}, DescriptionMax: 1000, PreviewMax: 1000, SpeechMax: 1000, OutputFormat: llm.SpeechOutputFormat, Settings: llm.SpeechSettings{Speed: 1, Stability: .5, SimilarityBoost: .75}}}
	store := spokenstore.New(h.Writer, h.Reader)
	js := jobstore.New(h.Writer, h.Reader, jobKinds())
	q := job.New(js, time.Millisecond, jobReporting{})
	q.AllowCancellation(jobCancellation{})
	us := usagestore.New(h.Writer, h.Reader)
	ledger := usage.NewService(us, nil, 0, spokenTestAnchors{}).WithRateSelector(usage.NewRateSelector(spokenTestRates{}, us)).WithUnitAccounting(p)
	plans := auth.NewService(authstore.New(h.Writer, h.Reader), time.Hour, auth.Deps{Mailer: mail.NewLog()})
	admission := jobAdmission{ledger: ledger, plans: plans, jobs: js}
	q.Admit(admission)
	objects := &spokenTestObjects{data: map[string][]byte{}}
	library := spoken.NewService(store, p, objects)
	provider := &spokenTestProvider{}
	deps := spokenapp.GenerationDeps{Qualifications: p, Library: library, Operations: store, Profiles: p, Prices: p, Ledger: ledger, Models: usage.SpeechMeter{Provider: provider, Ledger: ledger}, Jobs: spokenapp.NewJobs(q), Transactions: spokenTransactions{h.Writer}, Objects: objects}
	pub := &spokenTestPublisher{base: spokenPublishLibraries{deps}}
	deps.Publisher = pub
	f := &spokenGenerationFixture{t: t, path: path, handle: h, store: store, jobs: js, queue: q, library: library, generation: spokenapp.NewGenerationService(deps), profiles: p, provider: provider, objects: objects, publisher: pub, ledger: ledger, admission: admission}
	return f
}
func (f *spokenGenerationFixture) input() spoken.DraftInput {
	return spoken.DraftInput{Name: "Sound", Description: strings.Repeat("A calm Korean speaking voice. ", 3), PreviewText: strings.Repeat("Korean audition numbers and units. ", 5), ProfileID: "curated", ProfileRevision: 1}
}
func (f *spokenGenerationFixture) draft(key string) spoken.Draft {
	f.t.Helper()
	d, err := f.library.CreateDraft(f.t.Context(), "alice", plan.Master, key, f.input())
	if err != nil {
		f.t.Fatal(err)
	}
	return d
}
func (f *spokenGenerationFixture) start(in spokenapp.GenerationInput, key string) spoken.Operation {
	f.t.Helper()
	q, err := f.generation.Quote(f.t.Context(), "alice", plan.Master, in)
	if err != nil {
		f.t.Fatal(err)
	}
	a := usage.UnitApproval{}
	if q.ApprovalRequired {
		a = usage.UnitApproval{QuoteID: q.ID, ApprovedMaxCredits: &q.MaximumCredits, CancellationPolicyVersion: 1}
	}
	o, err := f.generation.Start(f.t.Context(), "alice", plan.Master, in, key, a)
	if err != nil {
		f.t.Fatal(err)
	}
	return o
}
func (f *spokenGenerationFixture) run(o spoken.Operation) error {
	f.t.Helper()
	j, err := f.jobs.PickNextQueued(f.t.Context(), time.Now())
	if err != nil || j.ID != o.JobID {
		f.t.Fatalf("pick %s = %+v %v", o.JobID, j, err)
	}
	err = metered(f.generation.Run)(f.t.Context(), j, func(string, int, int) {})
	current, e := f.jobs.GetByID(f.t.Context(), j.ID)
	if e != nil {
		f.t.Fatal(e)
	}
	if !job.Terminal(current.Status) {
		status := job.StatusDone
		var failure *job.Failure
		if current.CancelRequestedAt != nil {
			status = job.StatusCancelled
		} else if err != nil {
			status = job.StatusFailed
			normalized := jobReporting{}.Failure(err)
			failure = &normalized
		}
		if e := f.jobs.Finish(f.t.Context(), j.ID, status, failure, time.Now()); e != nil {
			f.t.Fatal(e)
		}
		current, e = f.jobs.GetByID(f.t.Context(), j.ID)
		if e != nil {
			f.t.Fatal(e)
		}
	}
	f.admission.Settle(f.t.Context(), j.ID, current.Status)
	f.admission.Settle(f.t.Context(), j.ID, current.Status)
	if e := f.generation.OnTerminal(f.t.Context(), current); e != nil {
		f.t.Fatal(e)
	}
	return err
}
func (f *spokenGenerationFixture) designed(key string) spoken.Draft {
	f.t.Helper()
	d := f.draft(key)
	o := f.start(spokenapp.GenerationInput{Kind: spoken.JobKindDesign, DraftID: d.ID, Revision: d.Revision}, key+"-design")
	if err := f.run(o); err != nil {
		f.t.Fatal(err)
	}
	d, err := f.library.GetDraft(f.t.Context(), "alice", d.ID)
	if err != nil || len(d.Candidates) != 3 {
		f.t.Fatal(d, err)
	}
	return d
}
func (f *spokenGenerationFixture) audition(d spoken.Draft, key string) spoken.Draft {
	f.t.Helper()
	c := d.Candidates[1]
	ticket, err := f.library.SampleAccess(f.t.Context(), "alice", c.AssetID)
	if err != nil {
		f.t.Fatal(err)
	}
	if _, err := f.library.ReadPlayback(f.t.Context(), "alice", ticket.ID); err != nil {
		f.t.Fatal(err)
	}
	d, err = f.library.AcknowledgeCandidate(f.t.Context(), "alice", d.ID, d.Revision, key+"-ack", c.ID, ticket.ID)
	if err != nil {
		f.t.Fatal(err)
	}
	d, err = f.library.SelectCandidate(f.t.Context(), "alice", d.ID, d.Revision, key+"-select", c.ID)
	if err != nil {
		f.t.Fatal(err)
	}
	return d
}
func (f *spokenGenerationFixture) confirmed(key string) spoken.Voice {
	f.t.Helper()
	d := f.audition(f.designed(key), key)
	o := f.start(spokenapp.GenerationInput{Kind: spoken.JobKindConfirm, DraftID: d.ID, Revision: d.Revision, CandidateID: d.SelectedCandidateID}, key+"-confirm")
	if err := f.run(o); err != nil {
		f.t.Fatal(err)
	}
	op, err := f.generation.Get(f.t.Context(), "alice", o.ID)
	if err != nil {
		f.t.Fatal(err)
	}
	v, err := f.library.GetVoice(f.t.Context(), "alice", op.ResultID)
	if err != nil {
		f.t.Fatal(err)
	}
	return v
}
func (f *spokenGenerationFixture) count(query string) int {
	f.t.Helper()
	var n int
	if err := f.handle.Reader.QueryRow(query).Scan(&n); err != nil {
		f.t.Fatal(err)
	}
	return n
}

func TestSpokenGenerationApprovalAuditionIdempotencyAndReuse(t *testing.T) {
	f := spokenGenerationFresh(t)
	d := f.draft("voice")
	in := spokenapp.GenerationInput{Kind: spoken.JobKindDesign, DraftID: d.ID, Revision: d.Revision}
	if _, err := f.generation.Start(t.Context(), "alice", plan.Master, in, "no-approval", usage.UnitApproval{}); err == nil {
		t.Fatal("unapproved start accepted")
	}
	if f.provider.design != 0 {
		t.Fatal("unapproved provider call")
	}
	q, err := f.generation.Quote(t.Context(), "alice", plan.Master, in)
	if err != nil {
		t.Fatal(err)
	}
	approval := usage.UnitApproval{QuoteID: q.ID, ApprovedMaxCredits: &q.MaximumCredits, CancellationPolicyVersion: 1}
	o, err := f.generation.Start(t.Context(), "alice", plan.Master, in, "generate", approval)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.run(o); err != nil {
		t.Fatal(err)
	}
	again, err := f.generation.Start(t.Context(), "alice", plan.Master, in, "generate", approval)
	if err != nil || again.ID != o.ID {
		t.Fatal("duplicate start after revision moved", again, err)
	}
	j, _ := f.jobs.GetByID(t.Context(), o.JobID)
	if err := metered(f.generation.Run)(t.Context(), j, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	if f.provider.design != 1 || f.count("SELECT count(*) FROM usage_unit_events") != 1 {
		t.Fatal("duplicate generation/accounting")
	}
	d, err = f.library.GetDraft(t.Context(), "alice", d.ID)
	if err != nil || len(d.Candidates) != 3 {
		t.Fatal(d, err)
	}
	noListen := spokenapp.GenerationInput{Kind: spoken.JobKindConfirm, DraftID: d.ID, Revision: d.Revision, CandidateID: d.Candidates[1].ID}
	if _, err := f.generation.Quote(t.Context(), "alice", plan.Master, noListen); !errors.Is(err, spoken.ErrAuditionRequired) {
		t.Fatal("unheard confirmation", err)
	}
	for _, c := range d.Candidates {
		p, err := f.library.SampleAccess(t.Context(), "alice", c.AssetID)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.library.ReadPlayback(t.Context(), "alice", p.ID); err != nil {
			t.Fatal(err)
		}
	}
	metadata := f.input()
	metadata.Name = "Renamed draft"
	d, err = f.library.UpdateDraft(t.Context(), "alice", plan.Master, d.ID, d.Revision, "metadata", metadata)
	if err != nil || len(d.Candidates) != 3 {
		t.Fatal(d, err)
	}
	if f.provider.design != 1 || f.provider.confirm != 0 || f.provider.speech != 0 {
		t.Fatal("read/save/play called provider")
	}
	d = f.audition(d, "voice")
	candidate := d.Candidates[1]
	confirm := spokenapp.GenerationInput{Kind: spoken.JobKindConfirm, DraftID: d.ID, Revision: d.Revision, CandidateID: candidate.ID}
	q, err = f.generation.Quote(t.Context(), "alice", plan.Master, confirm)
	if err != nil || q.MaximumCredits != 0 || !q.ApprovalRequired {
		t.Fatal("free confirm must be explicit and durable", q, err)
	}
	approval = usage.UnitApproval{QuoteID: q.ID, ApprovedMaxCredits: &q.MaximumCredits, CancellationPolicyVersion: 1}
	o, err = f.generation.Start(t.Context(), "alice", plan.Master, confirm, "confirm", approval)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.run(o); err != nil {
		t.Fatal(err)
	}
	again, err = f.generation.Start(t.Context(), "alice", plan.Master, confirm, "confirm", approval)
	if err != nil || again.ID != o.ID || f.provider.confirm != 1 || f.provider.lastCandidate != candidate.Handle {
		t.Fatal("confirmation identity or once-only", again, err)
	}
	op, _ := f.generation.Get(t.Context(), "alice", o.ID)
	v, err := f.library.GetVoice(t.Context(), "alice", op.ResultID)
	if err != nil || v.SampleAssetID != candidate.AssetID || v.Profile != d.Profile {
		t.Fatal("wrong saved sound", v, err)
	}
	probe := spokenapp.GenerationInput{Kind: spoken.JobKindProbe, VoiceID: v.ID, Revision: v.Revision, QualificationSessionID: "qualification", Texts: [2]string{"First different Korean names, numbers and units.", "Second different Korean names, numbers and units."}}
	if _, err := f.generation.Quote(t.Context(), "alice", plan.Pro, probe); !errors.Is(err, spokenapp.ErrQualificationOnly) {
		t.Fatal("public speech playground", err)
	}
	o = f.start(probe, "probe")
	if err := f.run(o); err != nil {
		t.Fatal(err)
	}
	first, _ := f.generation.Get(t.Context(), "alice", o.ID)
	if first.AssetIDs[0] == "" || first.AssetIDs[1] == "" || f.provider.speech != 2 || f.provider.lastVoice != v.Handle {
		t.Fatal("probe identity/audio", first)
	}
	probe.Texts[0] = "Changed first segment only, preserving the second audio."
	o = f.start(probe, "changed-probe")
	if err := f.run(o); err != nil {
		t.Fatal(err)
	}
	second, _ := f.generation.Get(t.Context(), "alice", o.ID)
	if f.provider.speech != 3 || second.AssetIDs[0] == first.AssetIDs[0] || second.AssetIDs[1] != first.AssetIDs[1] {
		t.Fatal("unchanged input resynthesized", second)
	}
	p, err := f.store.GetProbeAudio(t.Context(), "alice", v.ID, first.SpeechInputs[1])
	if err != nil || p.OperationID != first.ID || p.JobID != first.JobID || p.Evidence.RequestID != "speech-2" {
		t.Fatal("reused evidence lost its origin", p, err)
	}
	o = f.start(probe, "cached-probe")
	if err := f.run(o); err != nil || f.provider.speech != 3 {
		t.Fatal("cached probe", err)
	}
	if _, err := f.generation.Get(t.Context(), "bob", o.ID); !errors.Is(err, spoken.ErrNotFound) {
		t.Fatal("foreign operation", err)
	}
	rpc := spokenrpc.NewGenerationHandler(f.generation)
	ctx := auth.WithActor(t.Context(), auth.Actor{UserID: "alice", Plan: plan.Master})
	res, err := rpc.GetSpokenOperation(ctx, connect.NewRequest(&v1.SpokenIDRequest{Id: o.ID}))
	if err != nil {
		t.Fatal(err)
	}
	wire, _ := protojson.Marshal(res.Msg)
	for _, private := range []string{"private-confirmed", "scope_digest", "character_cost", "supplier", "object_key", strings.Repeat("a", 64)} {
		if bytes.Contains(wire, []byte(private)) {
			t.Fatal("private operation leaked", private)
		}
	}
	if f.count("SELECT count(*) FROM usage_unit_events") != 5 || f.count("SELECT count(*) FROM spoken_probe_audio") != 3 {
		t.Fatal("unexpected billable call/evidence count")
	}
}

func TestSpokenGenerationDriftCancellationReplacementAndUncertainRecovery(t *testing.T) {
	f := spokenGenerationFresh(t)
	ctx := t.Context()
	t.Run("quoted input and profile drift", func(t *testing.T) {
		d := f.draft("drift")
		in := spokenapp.GenerationInput{Kind: spoken.JobKindDesign, DraftID: d.ID, Revision: d.Revision}
		q, err := f.generation.Quote(ctx, "alice", plan.Master, in)
		if err != nil {
			t.Fatal(err)
		}
		changed := f.input()
		changed.PreviewText += " changed"
		d, err = f.library.UpdateDraft(ctx, "alice", plan.Master, d.ID, d.Revision, "drift-edit", changed)
		if err != nil {
			t.Fatal(err)
		}
		in.Revision = d.Revision
		if _, err := f.generation.Start(ctx, "alice", plan.Master, in, "drift-start", usage.UnitApproval{QuoteID: q.ID, ApprovedMaxCredits: &q.MaximumCredits, CancellationPolicyVersion: 1}); err == nil {
			t.Fatal("stale quote accepted")
		}
		if f.provider.design != 0 {
			t.Fatal("stale paid call")
		}
		o := f.start(in, "profile-drift")
		f.profiles.profile.ConnectionScope = strings.Repeat("b", 64)
		if err := f.run(o); err == nil {
			t.Fatal("changed connection accepted")
		}
		f.profiles.profile.ConnectionScope = strings.Repeat("a", 64)
		if f.provider.design != 0 {
			t.Fatal("changed binding paid call")
		}
	})
	t.Run("cancel wins and late result cannot replace", func(t *testing.T) {
		d := f.designed("cancel")
		ids := []string{d.Candidates[0].ID, d.Candidates[1].ID, d.Candidates[2].ID}
		o := f.start(spokenapp.GenerationInput{Kind: spoken.JobKindDesign, DraftID: d.ID, Revision: d.Revision}, "cancel-second")
		f.provider.hook = func() {
			if _, err := f.generation.Cancel(ctx, "alice", o.ID); err != nil {
				t.Fatal(err)
			}
		}
		if err := f.run(o); err == nil {
			t.Fatal("cancelled work published")
		}
		current, err := f.library.GetDraft(ctx, "alice", d.ID)
		if err != nil || current.Candidates[0].ID != ids[0] {
			t.Fatal("prior candidates changed", current, err)
		}
		op, _ := f.generation.Get(ctx, "alice", o.ID)
		if op.State != spoken.OperationCancelled {
			t.Fatal(op)
		}
		before := f.provider.design
		if err := f.generation.Recover(ctx); err != nil {
			t.Fatal(err)
		}
		if f.provider.design != before {
			t.Fatal("cancel auto retry")
		}
	})
	t.Run("replacement race leaves only orphan objects", func(t *testing.T) {
		d := f.designed("race")
		o := f.start(spokenapp.GenerationInput{Kind: spoken.JobKindDesign, DraftID: d.ID, Revision: d.Revision}, "race-second")
		f.objects.afterPut = func() {
			input := f.input()
			input.Description += " changed sound"
			if _, err := f.library.UpdateDraft(ctx, "alice", plan.Master, d.ID, d.Revision, "race-edit", input); err != nil {
				t.Fatal(err)
			}
		}
		if err := f.run(o); err == nil {
			t.Fatal("obsolete candidates published")
		}
		current, _ := f.library.GetDraft(ctx, "alice", d.ID)
		if len(current.Candidates) != 0 {
			t.Fatal("late candidates won")
		}
		if err := f.library.Cleanup(ctx, time.Now().Add(24*time.Hour)); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("known confirmation survives storage failure without replay", func(t *testing.T) {
		d := f.audition(f.designed("known"), "known")
		o := f.start(spokenapp.GenerationInput{Kind: spoken.JobKindConfirm, DraftID: d.ID, Revision: d.Revision, CandidateID: d.SelectedCandidateID}, "known-confirm")
		f.publisher.fail = true
		if err := f.run(o); err == nil {
			t.Fatal("injected publication did not fail")
		}
		op, _ := f.generation.Get(ctx, "alice", o.ID)
		if op.State != spoken.OperationReceived || op.ReceivedHandle == "" {
			t.Fatal("known identity lost", op)
		}
		before := f.provider.confirm
		f.publisher.fail = false
		if err := f.generation.Recover(ctx); err != nil {
			t.Fatal(err)
		}
		op, _ = f.generation.Get(ctx, "alice", o.ID)
		if op.State != spoken.OperationPublished || op.ResultID == "" || f.provider.confirm != before {
			t.Fatal("confirmation replayed", op)
		}
		if _, err := f.generation.RetryPublication(ctx, "alice", o.ID); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("lost supplier identity stays unresolved", func(t *testing.T) {
		d := f.audition(f.designed("lost"), "lost")
		in := spokenapp.GenerationInput{Kind: spoken.JobKindConfirm, DraftID: d.ID, Revision: d.Revision, CandidateID: d.SelectedCandidateID}
		o := f.start(in, "lost-confirm")
		f.provider.loseHandle = true
		f.provider.confirmErr = errors.New("connection lost after supplier execution")
		if err := f.run(o); err == nil {
			t.Fatal("unknown confirmation succeeded")
		}
		f.provider.loseHandle = false
		f.provider.confirmErr = nil
		op, _ := f.generation.Get(ctx, "alice", o.ID)
		if op.State != spoken.OperationUnresolved {
			t.Fatal(op)
		}
		before := f.provider.confirm
		if err := f.generation.Recover(ctx); err != nil {
			t.Fatal(err)
		}
		q, err := f.generation.Quote(ctx, "alice", plan.Master, in)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.generation.Start(ctx, "alice", plan.Master, in, "lost-again", usage.UnitApproval{QuoteID: q.ID, ApprovedMaxCredits: &q.MaximumCredits, CancellationPolicyVersion: 1}); !errors.Is(err, spoken.ErrOperationUnresolved) {
			t.Fatal("second supplier create allowed", err)
		}
		if f.provider.confirm != before {
			t.Fatal("unknown paid replay")
		}
	})
	t.Run("restart a claimed operation", func(t *testing.T) {
		d := f.audition(f.designed("restart"), "restart")
		o := f.start(spokenapp.GenerationInput{Kind: spoken.JobKindConfirm, DraftID: d.ID, Revision: d.Revision, CandidateID: d.SelectedCandidateID}, "restart-confirm")
		j, err := f.jobs.PickNextQueued(ctx, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		coordinator := spokenapp.Coordinator{Transactions: spokenTransactions{f.handle.Writer}}
		if err := coordinator.Claim(ctx, "alice", o.ID, j.ID); err != nil {
			t.Fatal(err)
		}
		before := f.provider.confirm
		if err := f.generation.Recover(ctx); err != nil {
			t.Fatal(err)
		}
		op, _ := f.generation.Get(ctx, "alice", o.ID)
		if op.State != spoken.OperationUnresolved || f.provider.confirm != before {
			t.Fatal("interrupted confirm replayed", op)
		}
		f.admission.Settle(ctx, j.ID, job.StatusFailed)
	})

	t.Run("queued cancellation makes zero calls", func(t *testing.T) {
		d := f.draft("queued-cancel")
		o := f.start(spokenapp.GenerationInput{Kind: spoken.JobKindDesign, DraftID: d.ID, Revision: d.Revision}, "queued-cancel-start")
		before := f.provider.design
		op, err := f.generation.Cancel(ctx, "alice", o.ID)
		if err != nil || op.State != spoken.OperationCancelled {
			t.Fatal(op, err)
		}
		if f.provider.design != before {
			t.Fatal("queued cancellation called provider")
		}
		j, err := f.jobs.GetByID(ctx, o.JobID)
		if err != nil || j.Status != job.StatusCancelled {
			t.Fatal(j, err)
		}
	})
	t.Run("late confirmed handle stays private after cancellation", func(t *testing.T) {
		d := f.audition(f.designed("late-confirm"), "late-confirm")
		in := spokenapp.GenerationInput{Kind: spoken.JobKindConfirm, DraftID: d.ID, Revision: d.Revision, CandidateID: d.SelectedCandidateID}
		o := f.start(in, "late-confirm-start")
		f.provider.hook = func() {
			if _, err := f.generation.Cancel(ctx, "alice", o.ID); err != nil {
				t.Fatal(err)
			}
		}
		if err := f.run(o); err == nil {
			t.Fatal("cancelled confirmation published")
		}
		op, _ := f.generation.Get(ctx, "alice", o.ID)
		if op.State != spoken.OperationCancelled || op.ReceivedHandle == "" {
			t.Fatal("late audit identity lost", op)
		}
		q, err := f.generation.Quote(ctx, "alice", plan.Master, in)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.generation.Start(ctx, "alice", plan.Master, in, "late-confirm-repeat", usage.UnitApproval{QuoteID: q.ID, ApprovedMaxCredits: &q.MaximumCredits, CancellationPolicyVersion: 1}); !errors.Is(err, spoken.ErrOperationUnresolved) {
			t.Fatal("cancelled supplier creation repeated", err)
		}
	})
	t.Run("failed upload keeps earlier audition", func(t *testing.T) {
		d := f.designed("upload-failure")
		asset := d.Candidates[0].AssetID
		o := f.start(spokenapp.GenerationInput{Kind: spoken.JobKindDesign, DraftID: d.ID, Revision: d.Revision}, "upload-failure-new")
		f.objects.fail = true
		if err := f.run(o); err == nil {
			t.Fatal("upload failure published")
		}
		f.objects.fail = false
		current, err := f.library.GetDraft(ctx, "alice", d.ID)
		if err != nil || current.Candidates[0].AssetID != asset {
			t.Fatal("previous audition lost", current, err)
		}
		before := f.provider.design
		if err := f.generation.Recover(ctx); err != nil || f.provider.design != before {
			t.Fatal("failed upload automatically retried", err)
		}
	})
	if f.count("SELECT count(*) FROM spoken_voices WHERE owner_id='alice'") != 1 {
		t.Fatal("failure replaced/created a voice")
	}
}

func (f *spokenGenerationFixture) reopen() {
	f.t.Helper()
	path := f.path
	if err := f.handle.Close(); err != nil {
		f.t.Fatal(err)
	}
	h, err := db.Open(path)
	if err != nil {
		f.t.Fatal(err)
	}
	f.t.Cleanup(func() { h.Close() })
	f.handle = h
	f.store = spokenstore.New(h.Writer, h.Reader)
	f.jobs = jobstore.New(h.Writer, h.Reader, jobKinds())
	f.queue = job.New(f.jobs, time.Millisecond, jobReporting{})
	f.queue.AllowCancellation(jobCancellation{})
	f.ledger = f.ledger.WithStore(usagestore.New(h.Writer, h.Reader))
	plans := auth.NewService(authstore.New(h.Writer, h.Reader), time.Hour, auth.Deps{Mailer: mail.NewLog()})
	f.admission = jobAdmission{ledger: f.ledger, plans: plans, jobs: f.jobs}
	f.queue.Admit(f.admission)
	f.library = spoken.NewService(f.store, f.profiles, f.objects)
	deps := spokenapp.GenerationDeps{Qualifications: f.profiles, Library: f.library, Operations: f.store, Profiles: f.profiles, Prices: f.profiles, Ledger: f.ledger, Models: usage.SpeechMeter{Provider: f.provider, Ledger: f.ledger}, Jobs: spokenapp.NewJobs(f.queue), Transactions: spokenTransactions{h.Writer}, Objects: f.objects}
	f.publisher.base = spokenPublishLibraries{deps}
	deps.Publisher = f.publisher
	f.generation = spokenapp.NewGenerationService(deps)
}
func TestSpokenGenerationPersistedConfirmationAndPrivateCleanup(t *testing.T) {
	f := spokenGenerationFresh(t)
	ctx := t.Context()
	d := f.audition(f.designed("restart-known"), "restart-known")
	in := spokenapp.GenerationInput{Kind: spoken.JobKindConfirm, DraftID: d.ID, Revision: d.Revision, CandidateID: d.SelectedCandidateID}
	f.profiles.paidConfirmation = true
	q, err := f.generation.Quote(ctx, "alice", plan.Master, in)
	if err != nil || q.MaximumCredits <= 0 {
		t.Fatal("applicable confirmation fee omitted", q, err)
	}
	o := f.start(in, "received-before-restart")
	j, err := f.jobs.PickNextQueued(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	coordinator := spokenapp.Coordinator{Transactions: spokenTransactions{f.handle.Writer}}
	if err := coordinator.Claim(ctx, "alice", o.ID, j.ID); err != nil {
		t.Fatal(err)
	}
	response, err := f.generation.Models.ConfirmVoice(usage.WithWork(ctx, usage.Work{UserID: "alice", Kind: o.Kind, JobID: j.ID, UnitScopeDigest: o.ScopeDigest}), llm.VoiceConfirmationRequest{DesignModel: o.Profile.Design, Candidate: o.CandidateHandle, Name: o.Name, Description: o.Description})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.store.RecordConfirmationResult(ctx, "alice", o.ID, response.Voice, response.Evidence); err != nil {
		t.Fatal(err)
	}
	f.reopen()
	if err := f.generation.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := f.queue.SweepRunning(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := f.queue.SweepOpenHolds(ctx); err != nil {
		t.Fatal(err)
	}
	op, err := f.generation.Get(ctx, "alice", o.ID)
	if err != nil || op.State != spoken.OperationPublished || f.provider.confirm != 1 {
		t.Fatal("persisted confirmation failed or repeated", op, err)
	}
	saved, err := f.library.GetVoice(ctx, "alice", op.ResultID)
	if err != nil || saved.Handle != response.Voice {
		t.Fatal(saved, err)
	}
	if f.count("SELECT count(*) FROM usage_unit_events") != 2 {
		t.Fatal("restart duplicated reported usage")
	}
	// A second confirmation whose publication loses to a draft edit must retain
	// its exact audition sample for reconciliation, even after the candidates clear.
	d = f.audition(f.designed("retained"), "retained")
	o = f.start(spokenapp.GenerationInput{Kind: spoken.JobKindConfirm, DraftID: d.ID, Revision: d.Revision, CandidateID: d.SelectedCandidateID}, "retained-confirm")
	sample := o.SampleAssetID
	f.provider.hook = func() {
		input := f.input()
		input.PreviewText += " Different input."
		if _, err := f.library.UpdateDraft(ctx, "alice", plan.Master, d.ID, d.Revision, "supersede-confirmation", input); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.run(o); err == nil {
		t.Fatal("superseded confirmation published")
	}
	op, _ = f.generation.Get(ctx, "alice", o.ID)
	if op.State != spoken.OperationUnresolved || op.ReceivedHandle == "" {
		t.Fatal("known stale identity discarded", op)
	}
	if _, err := f.store.GetAsset(ctx, "alice", sample); err != nil {
		t.Fatal("reconciliation sample discarded", err)
	}
	if err := f.library.Cleanup(ctx, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := f.store.DeleteOwner(ctx, "alice"); err != nil {
		t.Fatal(err)
	}
	if f.count("SELECT count(*) FROM spoken_voice_operations WHERE owner_id='alice'") != 0 || f.count("SELECT count(*) FROM spoken_audio_assets WHERE owner_id='alice'") != 0 {
		t.Fatal("private operations remain after account deletion")
	}
	if err := f.library.Cleanup(ctx, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if len(f.objects.data) != 0 {
		t.Fatal("private objects remain after cleanup", len(f.objects.data))
	}
}

func TestSpokenGenerationCancellationAndPublicationRace(t *testing.T) {
	f := spokenGenerationFresh(t)
	ctx := t.Context()
	for i := 0; i < 4; i++ {
		key := fmt.Sprintf("publish-cancel-%d", i)
		d := f.audition(f.designed(key), key)
		o := f.start(spokenapp.GenerationInput{Kind: spoken.JobKindConfirm, DraftID: d.ID, Revision: d.Revision, CandidateID: d.SelectedCandidateID}, key+"-start")
		j, err := f.jobs.PickNextQueued(ctx, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		c := spokenapp.Coordinator{Transactions: spokenTransactions{f.handle.Writer}}
		if err := c.Claim(ctx, "alice", o.ID, j.ID); err != nil {
			t.Fatal(err)
		}
		r, err := f.generation.Models.ConfirmVoice(usage.WithWork(ctx, usage.Work{UserID: "alice", Kind: o.Kind, JobID: j.ID, UnitScopeDigest: o.ScopeDigest}), llm.VoiceConfirmationRequest{DesignModel: o.Profile.Design, Candidate: o.CandidateHandle, Name: o.Name, Description: o.Description})
		if err != nil {
			t.Fatal(err)
		}
		if err := f.store.RecordConfirmationResult(ctx, "alice", o.ID, r.Voice, r.Evidence); err != nil {
			t.Fatal(err)
		}
		start := make(chan struct{})
		publication, cancellation := make(chan error, 1), make(chan error, 1)
		go func() { <-start; _, err := f.generation.RetryPublication(ctx, "alice", o.ID); publication <- err }()
		go func() { <-start; _, err := f.generation.Cancel(ctx, "alice", o.ID); cancellation <- err }()
		close(start)
		publishErr, cancelErr := <-publication, <-cancellation
		if cancelErr != nil {
			t.Fatal(cancelErr)
		}
		op, err := f.generation.Get(ctx, "alice", o.ID)
		if err != nil {
			t.Fatal(err)
		}
		current, err := f.jobs.GetByID(ctx, j.ID)
		if err != nil {
			t.Fatal(err)
		}
		switch op.State {
		case spoken.OperationPublished:
			if publishErr != nil || current.Status != job.StatusDone || op.ResultID == "" {
				t.Fatal("publication did not win atomically", op, current, publishErr)
			}
			v, err := f.library.GetVoice(ctx, "alice", op.ResultID)
			if err != nil || v.Handle != r.Voice {
				t.Fatal("wrong sound published", v, err)
			}
		case spoken.OperationCancelled:
			if publishErr == nil || current.CancelRequestedAt == nil || op.ResultID != "" {
				t.Fatal("cancelled result published", op, current, publishErr)
			}
			if err := f.jobs.Finish(ctx, j.ID, job.StatusCancelled, nil, time.Now()); err != nil {
				t.Fatal(err)
			}
		default:
			t.Fatal("race left an intermediate result", op)
		}
		terminal, _ := f.jobs.GetByID(ctx, j.ID)
		f.admission.Settle(ctx, j.ID, terminal.Status)
		f.admission.Settle(ctx, j.ID, terminal.Status)
	}
	if f.provider.confirm != 4 || f.count("SELECT count(*) FROM usage_unit_events") != 8 {
		t.Fatal("race repeated supplier creation or settlement")
	}
}
