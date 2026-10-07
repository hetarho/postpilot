package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/usage"
	"github.com/postpilot/backend/internal/voice/spoken"
	"strconv"
	"strings"
	"time"
)

const JournalTimeout = 5 * time.Second

var ErrQualificationOnly = errors.New("owned master qualification session required")

type BudgetPricer interface {
	Budget(context.Context, string, plan.Plan, string, int64, string, string, llm.SpeechInput, int) (usage.UnitBudget, error)
}
type Ledger interface {
	QuoteUnits(context.Context, string, plan.Plan, string, []usage.UnitBudget) (usage.UnitQuote, error)
	ReservationForUnits(context.Context, string, plan.Plan, string, usage.UnitApproval, []usage.UnitBudget) (*usage.Reservation, error)
	AdmissionForJob(context.Context, string) (usage.Admission, bool, error)
}
type Jobs interface {
	Enqueue(context.Context, spoken.Operation, []usage.UnitBudget, bool) (string, error)
	SignalCancellation(context.Context, string, string) error
}
type Publisher interface{ LibraryForOperation(string) *spoken.Service }
type GenerationDeps struct {
	Qualifications QualificationReader
	Library        *spoken.Service
	Operations     spoken.OperationStorage
	Profiles       spoken.ProfileResolver
	Prices         BudgetPricer
	Ledger         Ledger
	Models         llm.SpeechProvider
	Jobs           Jobs
	Transactions   Transactions
	Publisher      Publisher
	Objects        spoken.AudioObjects
}
type GenerationService struct {
	GenerationDeps
	coordinator Coordinator
	newID       func() string
}

func NewGenerationService(d GenerationDeps) *GenerationService {
	if d.Qualifications == nil || d.Library == nil || d.Operations == nil || d.Profiles == nil || d.Prices == nil || d.Ledger == nil || d.Models == nil || d.Jobs == nil || d.Transactions == nil || d.Publisher == nil || d.Objects == nil {
		panic("spoken generation dependencies required")
	}
	return &GenerationService{GenerationDeps: d, coordinator: Coordinator{d.Transactions}, newID: operationID}
}
func operationID() string {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		panic("spoken operation identity unavailable")
	}
	return hex.EncodeToString(id[:])
}

type GenerationInput struct {
	Kind, DraftID, VoiceID, CandidateID, QualificationSessionID string
	Revision                                                    int64
	Texts                                                       [2]string
}
type Quote struct {
	ID               string
	MaximumCredits   int
	ExpiresAt        time.Time
	ApprovalRequired bool
	ExistingVoiceID  string
}
type prepared struct {
	operation spoken.Operation
	inputs    []llm.SpeechInput
	budgets   []usage.UnitBudget
	existing  string
}

func validKey(key string) bool {
	if len(key) < 1 || len(key) > 200 {
		return false
	}
	for _, r := range key {
		if r < 33 || r > 126 {
			return false
		}
	}
	return true
}

func (s *GenerationService) prepare(ctx context.Context, owner string, tier plan.Plan, in GenerationInput) (p prepared, err error) {
	if owner == "" || !tier.Valid() || !spoken.IsJobKind(in.Kind) || in.Revision <= 0 {
		return p, spoken.ErrInvalid
	}
	o := spoken.Operation{OwnerID: owner, Kind: in.Kind, ExpectedRevision: in.Revision, DraftID: in.DraftID, VoiceID: in.VoiceID}
	if in.Kind == spoken.JobKindProbe {
		if tier != plan.Master || in.QualificationSessionID == "" {
			return p, ErrQualificationOnly
		}
		v, e := s.Library.GetVoice(ctx, owner, in.VoiceID)
		if e != nil {
			return p, e
		}
		if v.RemovedAt != nil {
			return p, spoken.ErrNotFound
		}
		if v.Revision != in.Revision {
			return p, spoken.ErrConflict
		}
		o.Profile = v.Profile
		o.VoiceHandle = v.Handle
		o.QualificationSessionID = in.QualificationSessionID
		o.Texts = in.Texts
		o.Evidence = make([]llm.SpeechEvidence, 2)
		if in.Texts[0] == in.Texts[1] {
			return p, spoken.ErrInvalid
		}
		for i, text := range in.Texts {
			request := llm.SpeechRequest{Model: v.Profile.Synthesis, Voice: v.Handle, Text: text, Settings: v.Profile.Settings}
			input, e := request.Input()
			if e != nil || input.InputCharacters > v.Profile.SpeechMax {
				return p, spoken.ErrInvalid
			}
			o.SpeechInputs[i] = input.Digest
			p.inputs = append(p.inputs, input)
			cached, e := s.Operations.GetProbeAudio(ctx, owner, v.ID, input.Digest)
			if e == nil {
				o.AssetIDs[i] = cached.AssetID
				o.Evidence[i] = cached.Evidence
			} else if !errors.Is(e, spoken.ErrNotFound) {
				return p, e
			}
		}
	} else {
		d, e := s.Library.GetDraft(ctx, owner, in.DraftID)
		if e != nil {
			return p, e
		}
		if d.Revision != in.Revision {
			return p, spoken.ErrConflict
		}
		if d.ConfirmedVoiceID != "" {
			if in.Kind == spoken.JobKindConfirm {
				if in.CandidateID != d.SelectedCandidateID {
					return p, spoken.ErrImmutable
				}
				p.existing = d.ConfirmedVoiceID
				return p, nil
			}
			return p, spoken.ErrImmutable
		}
		o.Profile = d.Profile
		o.Name = d.Name
		o.Description = d.Description
		o.PreviewText = d.PreviewText
		o.QualificationSessionID = d.QualificationSessionID
		if in.QualificationSessionID != "" && in.QualificationSessionID != d.QualificationSessionID {
			return p, spoken.ErrInvalid
		}
		if in.Kind == spoken.JobKindDesign {
			input, e := designRequest(o).Input()
			if e != nil {
				return p, e
			}
			p.inputs = []llm.SpeechInput{input}
		} else {
			if in.CandidateID == "" || in.CandidateID != d.SelectedCandidateID {
				return p, spoken.ErrAuditionRequired
			}
			for _, c := range d.Candidates {
				if c.ID == in.CandidateID && c.AuditionedAt != nil {
					o.CandidateID = c.ID
					o.CandidateHandle = c.Handle
					o.SampleAssetID = c.AssetID
				}
			}
			if o.CandidateHandle == "" {
				return p, spoken.ErrAuditionRequired
			}
			input, e := confirmRequest(o).Input()
			if e != nil {
				return p, e
			}
			p.inputs = []llm.SpeechInput{input}
		}
	}
	current, e := s.Profiles.ResolveSpokenProfile(ctx, owner, tier, o.Profile.ID, o.Profile.Revision, o.QualificationSessionID)
	if e != nil {
		return p, e
	}
	if len(current.ConnectionScope) != 64 || current != o.Profile {
		return p, spoken.ErrConflict
	}
	parts := []string{"spoken-operation-v1", owner, o.Kind, o.DraftID, o.VoiceID, strconv.FormatInt(o.ExpectedRevision, 10), o.Profile.ID, strconv.FormatInt(o.Profile.Revision, 10), o.Profile.ConnectionScope, o.CandidateID, o.SampleAssetID}
	for _, input := range p.inputs {
		parts = append(parts, input.Digest)
	}
	o.ScopeDigest = usage.UnitDigest(parts...)
	for i, input := range p.inputs {
		if o.Kind == spoken.JobKindProbe && o.AssetIDs[i] != "" {
			continue
		}
		b, e := s.Prices.Budget(ctx, owner, tier, o.Profile.ID, o.Profile.Revision, o.QualificationSessionID, o.ScopeDigest, input, 1)
		if e != nil {
			return p, e
		}
		p.budgets = append(p.budgets, b)
	}
	p.operation = o
	return p, nil
}
func designRequest(o spoken.Operation) llm.VoiceDesignRequest {
	return llm.VoiceDesignRequest{Composition: spokenComposition("design", o, 0), Model: o.Profile.Design, Description: o.Description, PreviewText: o.PreviewText}
}
func confirmRequest(o spoken.Operation) llm.VoiceConfirmationRequest {
	return llm.VoiceConfirmationRequest{Composition: spokenComposition("confirm", o, 0), DesignModel: o.Profile.Design, Candidate: o.CandidateHandle, Name: o.Name, Description: o.Description}
}
func speechRequest(o spoken.Operation, index int) llm.SpeechRequest {
	return llm.SpeechRequest{Composition: spokenComposition("speech", o, index), Model: o.Profile.Synthesis, Voice: o.VoiceHandle, Text: o.Texts[index], Settings: o.Profile.Settings}
}
func (s *GenerationService) Quote(ctx context.Context, owner string, tier plan.Plan, in GenerationInput) (Quote, error) {
	p, err := s.prepare(ctx, owner, tier, in)
	if err != nil {
		return Quote{}, err
	}
	if p.existing != "" {
		return Quote{ExistingVoiceID: p.existing}, nil
	}
	if len(p.budgets) == 0 {
		return Quote{}, nil
	}
	q, err := s.Ledger.QuoteUnits(ctx, owner, tier, in.Kind, p.budgets)
	if err != nil {
		return Quote{}, err
	}
	return Quote{ID: q.ID, MaximumCredits: q.MaxCredits, ExpiresAt: q.ExpiresAt, ApprovalRequired: true}, nil
}
func (s *GenerationService) Start(ctx context.Context, owner string, tier plan.Plan, in GenerationInput, key string, approval usage.UnitApproval) (spoken.Operation, error) {
	if !validKey(key) {
		return spoken.Operation{}, spoken.ErrInvalid
	}
	if owner == "" || !tier.Valid() || !spoken.IsJobKind(in.Kind) {
		return spoken.Operation{}, spoken.ErrInvalid
	}
	if in.Kind == spoken.JobKindProbe && tier != plan.Master {
		return spoken.Operation{}, ErrQualificationOnly
	}
	maximum := "none"
	if approval.ApprovedMaxCredits != nil {
		maximum = strconv.Itoa(*approval.ApprovedMaxCredits)
	}
	requestDigest := usage.UnitDigest("spoken-start-v1", owner, in.Kind, in.DraftID, in.VoiceID, in.CandidateID, strconv.FormatInt(in.Revision, 10), in.QualificationSessionID, in.Texts[0], in.Texts[1], approval.QuoteID, maximum, strconv.Itoa(approval.CancellationPolicyVersion))
	prior, err := s.Operations.GetOperationRequest(ctx, owner, in.Kind, key)
	if err == nil {
		if prior.RequestDigest != requestDigest {
			return spoken.Operation{}, spoken.ErrConflict
		}
		return prior, nil
	}
	if !errors.Is(err, spoken.ErrNotFound) {
		return spoken.Operation{}, err
	}
	p, err := s.prepare(ctx, owner, tier, in)
	if err != nil {
		return spoken.Operation{}, err
	}
	if p.existing != "" {
		return spoken.Operation{OwnerID: owner, Kind: in.Kind, State: spoken.OperationPublished, ResultID: p.existing}, nil
	}
	o := p.operation
	o.ID = s.newID()
	o.IdempotencyKey = key
	o.RequestDigest = requestDigest
	if o.QualificationSessionID != "" {
		session, e := s.Qualifications.Session(ctx, owner, o.QualificationSessionID)
		if e != nil {
			return spoken.Operation{}, e
		}
		if session.OwnerID != owner || session.ProfileID != o.Profile.ID || session.Revision != o.Profile.Revision || !session.ExpiresAt.After(time.Now()) {
			return spoken.Operation{}, ErrQualificationOnly
		}
		maximum, e := qualificationUSD(session.MaximumUSD)
		if e != nil {
			return spoken.Operation{}, e
		}
		reserved, e := budgetsUSD(p.budgets)
		if e != nil {
			return spoken.Operation{}, e
		}
		o.QualificationLimitUSD, o.QualificationReservedUSD = maximum.RatString(), reserved.RatString()
	}
	// Check an already consumed idempotent start before reading its consumed quote.
	prior, created, err := s.Operations.ReserveOperation(ctx, o)
	if err != nil {
		return spoken.Operation{}, err
	}
	if !created {
		return prior, nil
	}
	o = prior
	var reservation *usage.Reservation
	if len(p.budgets) > 0 {
		reservation, err = s.Ledger.ReservationForUnits(ctx, owner, tier, in.Kind, approval, p.budgets)
	} else if approval.QuoteID != "" {
		err = usage.ErrUnitApproval
	}
	if err != nil {
		s.fail(ctx, o, "APPROVAL_CHANGED", false)
		return spoken.Operation{}, err
	}
	if reservation != nil {
		ctx = usage.WithUnitReservation(ctx, reservation)
	}
	jobID, err := s.Jobs.Enqueue(ctx, o, p.budgets, len(p.budgets) == 0)
	if err != nil {
		s.fail(ctx, o, "JOB_NOT_CREATED", false)
		return spoken.Operation{}, err
	}
	// The worker may have claimed/published before this request reaches the bind.
	bind, cancel := context.WithTimeout(context.WithoutCancel(ctx), JournalTimeout)
	defer cancel()
	if err := s.Operations.BindOperation(bind, owner, o.ID, jobID); err != nil {
		return spoken.Operation{}, err
	}
	return s.Operations.GetOperation(bind, owner, o.ID)
}
func (s *GenerationService) Get(ctx context.Context, owner, id string) (spoken.Operation, error) {
	return s.Operations.GetOperation(ctx, owner, id)
}
func (s *GenerationService) Cancel(ctx context.Context, owner, id string) (spoken.Operation, error) {
	if err := s.coordinator.Cancel(ctx, owner, id); err != nil {
		return spoken.Operation{}, err
	}
	o, err := s.Operations.GetOperation(ctx, owner, id)
	if err != nil {
		return o, err
	}
	if o.State == spoken.OperationCancelled && o.JobID != "" {
		if err := s.Jobs.SignalCancellation(ctx, owner, o.JobID); err != nil {
			return o, err
		}
	}
	return o, nil
}
func (s *GenerationService) fail(ctx context.Context, o spoken.Operation, reason string, uncertain bool) {
	audit, cancel := context.WithTimeout(context.WithoutCancel(ctx), JournalTimeout)
	defer cancel()
	_ = s.Operations.FailOperation(audit, o.OwnerID, o.ID, reason, uncertain)
}
func (s *GenerationService) Run(ctx context.Context, j job.Job, progress job.Progress) error {
	id := strings.TrimSpace(string(j.Payload))
	o, err := s.Operations.GetOperation(ctx, j.UserID, id)
	if err != nil {
		return err
	}
	if o.Kind != j.Kind {
		return spoken.ErrInvalid
	}
	if o.State == spoken.OperationPublished {
		return nil
	}
	admission, admitted, err := s.Ledger.AdmissionForJob(ctx, j.ID)
	if err != nil {
		return err
	}
	tier := admission.AdmittedPlan
	if !admitted && o.Kind == spoken.JobKindProbe && o.AssetIDs[0] != "" && o.AssetIDs[1] != "" {
		tier = plan.Master
	}
	current, err := s.Profiles.ResolveSpokenProfile(ctx, o.OwnerID, tier, o.Profile.ID, o.Profile.Revision, o.QualificationSessionID)
	if err != nil || current != o.Profile {
		s.fail(ctx, o, "BINDING_CHANGED", false)
		if err != nil {
			return err
		}
		return spoken.ErrConflict
	}
	if err := s.coordinator.Claim(ctx, j.UserID, id, j.ID); err != nil {
		if !errors.Is(err, spoken.ErrOperationStopped) {
			s.fail(ctx, o, "INPUT_CHANGED", false)
		}
		return err
	}
	o.JobID = j.ID
	work, ok := usage.WorkFromContext(ctx)
	if !ok || work.JobID != j.ID || work.UserID != o.OwnerID || work.Kind != o.Kind {
		return usage.ErrUnitCall
	}
	work.UnitScopeDigest = o.ScopeDigest
	ctx = usage.WithWork(ctx, work)
	progress(o.Kind, 0, 1)
	switch o.Kind {
	case spoken.JobKindDesign:
		response, e := s.Models.DesignVoice(ctx, designRequest(o))
		s.recordEvidence(ctx, o, 0, response.Evidence)
		if e != nil {
			s.fail(ctx, o, "VOICE_DESIGN_FAILED", false)
			return e
		}
		_, e = s.Publisher.LibraryForOperation(o.ID).SaveCandidates(ctx, o.OwnerID, o.DraftID, o.ExpectedRevision, o.ID, response.Candidates)
		if e != nil {
			s.fail(ctx, o, "VOICE_PUBLICATION_FAILED", false)
			return e
		}
	case spoken.JobKindConfirm:
		response, e := s.Models.ConfirmVoice(ctx, confirmRequest(o))
		if response.Voice != "" {
			record, cancel := context.WithTimeout(context.WithoutCancel(ctx), JournalTimeout)
			saveErr := s.Operations.RecordConfirmationResult(record, o.OwnerID, o.ID, response.Voice, response.Evidence)
			cancel()
			if saveErr != nil {
				return saveErr
			}
			o.ReceivedHandle = response.Voice
		}
		if e != nil && response.Voice == "" {
			s.recordEvidence(ctx, o, 0, response.Evidence)
			s.fail(ctx, o, "CONFIRMATION_UNRESOLVED", true)
			return e
		}
		if o.ReceivedHandle == "" {
			s.fail(ctx, o, "CONFIRMATION_UNRESOLVED", true)
			return spoken.ErrOperationUnresolved
		}
		if e = s.publishConfirmation(ctx, o); e != nil {
			s.fail(ctx, o, "VOICE_PUBLICATION_FAILED", publicationStale(e))
			return e
		}
	case spoken.JobKindProbe:
		if err := s.runProbe(ctx, o); err != nil {
			s.fail(ctx, o, "VOICE_PROBE_FAILED", false)
			return err
		}
	default:
		return spoken.ErrInvalid
	}
	progress("complete", 1, 1)
	return nil
}
func (s *GenerationService) publishConfirmation(ctx context.Context, o spoken.Operation) error {
	_, err := s.Publisher.LibraryForOperation(o.ID).SaveConfirmation(ctx, o.OwnerID, o.DraftID, o.ExpectedRevision, o.ID, o.CandidateID, o.ReceivedHandle)
	return err
}
func (s *GenerationService) Recover(ctx context.Context) error {
	ops, err := s.Operations.ListRecoverableOperations(ctx)
	if err != nil {
		return err
	}
	for _, o := range ops {
		if o.State == spoken.OperationReceived && o.ReceivedHandle != "" {
			if err := s.publishConfirmation(ctx, o); err == nil {
				continue
			} else if !publicationStale(err) {
				return err
			}
		}
		if err := s.coordinator.FailInterrupted(ctx, o); err != nil {
			return err
		}
	}
	return nil
}
func (s *GenerationService) RetryPublication(ctx context.Context, owner, id string) (spoken.Operation, error) {
	o, err := s.Operations.GetOperation(ctx, owner, id)
	if err != nil {
		return o, err
	}
	if o.State == spoken.OperationPublished {
		return o, nil
	}
	if o.State != spoken.OperationReceived || o.ReceivedHandle == "" {
		return o, spoken.ErrOperationUnresolved
	}
	if err := s.publishConfirmation(ctx, o); err != nil {
		return o, err
	}
	return s.Operations.GetOperation(ctx, owner, id)
}

// OnTerminal records cancellations and interrupted handlers even when the worker
// could not return through Run. Received identities remain metadata-recoverable.
func (s *GenerationService) OnTerminal(ctx context.Context, j job.Job) error {
	o, err := s.Operations.GetOperation(ctx, j.UserID, string(j.Payload))
	if errors.Is(err, spoken.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if o.Terminal() {
		return nil
	}
	if j.Status == job.StatusCancelled {
		return s.Operations.CancelOperation(ctx, o.OwnerID, o.ID)
	}
	if j.Status == job.StatusFailed {
		return s.Operations.FailOperation(ctx, o.OwnerID, o.ID, "JOB_FAILED", o.Kind == spoken.JobKindConfirm && o.ReceivedHandle == "")
	}
	return nil
}
