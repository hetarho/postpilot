package authoring

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/postpilot/backend/internal/llm"
)

type Service struct {
	store      Store
	models     Models
	jobs       Jobs
	targets    Targets
	budget     Budget
	estimates  Estimator
	now        func() time.Time
	ownerLocks sync.Map
}

func NewService(store Store, models Models, jobs Jobs, targets Targets, budget Budget, estimates Estimator) *Service {
	if store == nil || models == nil || jobs == nil || targets == nil || budget == nil || estimates == nil {
		panic("authoring: all behavior ports are required")
	}
	return &Service{store: store, models: models, jobs: jobs, targets: targets, budget: budget, estimates: estimates, now: time.Now}
}
func (s *Service) lock(owner string) func() {
	v, _ := s.ownerLocks.LoadOrStore(owner, &sync.Mutex{})
	m := v.(*sync.Mutex)
	m.Lock()
	return m.Unlock
}
func newID() string {
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b[:])
}
func requestKey(k string) bool { return strings.TrimSpace(k) != "" && utf8.RuneCountInString(k) <= 128 }
func (s *Service) Create(ctx context.Context, owner string, kind Kind, targetID, requestID string) (Session, error) {
	return s.CreateWithReference(ctx, owner, kind, targetID, requestID, "")
}

const MaxReferencePostChars = 12000

func (s *Service) CreateWithReference(ctx context.Context, owner string, kind Kind, targetID, requestID, reference string) (Session, error) {
	if !utf8.ValidString(reference) || utf8.RuneCountInString(reference) > MaxReferencePostChars || (reference != "" && (kind != PostTemplate || targetID != "")) {
		return Session{}, ErrInvalid
	}
	if owner == "" {
		return Session{}, ErrNotFound
	}
	if !kind.Valid() {
		return Session{}, ErrInvalidKind
	}
	if !requestKey(requestID) {
		return Session{}, ErrInvalid
	}
	defer s.lock(owner)()
	prior, e := s.store.Created(ctx, owner, requestID)
	if e != nil {
		return Session{}, e
	}
	if prior != nil {
		if prior.Kind != kind || prior.TargetID != targetID || prior.ReferencePost != reference {
			return Session{}, ErrStale
		}
		return s.get(ctx, owner, prior.ID)
	}
	seed, e := s.targets.Seed(ctx, owner, kind, targetID)
	if e != nil {
		return Session{}, e
	}
	now := s.now().UTC()
	state := Session{ID: newID(), UserID: owner, Kind: kind, Phase: "choosing", TargetID: targetID, TargetVersion: seed.TargetVersion, Selected: seed.Artifact, SavedBaseline: seed.Artifact, WorkingSource: seed.Artifact, SavedAvailable: targetID != "", DraftState: DraftValid, RequestedCandidateCount: CandidateCount, ForkVoice: seed.ForkVoice, SourceContext: seed.SourceContext, ReferencePost: reference, Candidates: []Artifact{}, Turns: []Turn{}, CreatedAt: now, UpdatedAt: now}
	if seed.Artifact == nil {
		source := Artifact{ID: state.ID + "-manual"}
		if seed.WorkingSource != nil {
			source = *seed.WorkingSource
			source.ID = state.ID + "-manual"
		}
		if kind == PostTemplate {
			empty := ""
			source.TargetLength, source.TagCount = &empty, &empty
		}
		if kind == PostGuideline || kind == VideoGuideline {
			global := "global"
			source.Scope = &global
		}
		state.WorkingSource = &source
		state.DraftState = DraftIncomplete
	}
	if seed.Artifact != nil {
		state.Phase = "editing"
		if strings.TrimSpace(seed.Artifact.Body) == "" {
			state.DraftState = DraftIncomplete
		} else if s.targets.Validate(kind, *seed.Artifact) != nil {
			state.DraftState = DraftInvalid
		}
	}
	return s.store.Create(ctx, state, requestID)
}
func (s *Service) Get(ctx context.Context, owner, id string) (Session, error) {
	defer s.lock(owner)()
	return s.get(ctx, owner, id)
}
func (s *Service) Latest(ctx context.Context, owner string, kind Kind, targetID string) (*Session, error) {
	if !kind.Valid() {
		return nil, ErrInvalidKind
	}
	defer s.lock(owner)()
	state, e := s.store.Latest(ctx, owner, kind, targetID)
	if e != nil || state == nil {
		return state, e
	}
	found, e := s.get(ctx, owner, state.ID)
	return &found, e
}
func (s *Service) get(ctx context.Context, owner, id string) (Session, error) {
	state, e := s.store.Get(ctx, owner, id)
	if e != nil || state.ActiveRequestID == "" {
		return state, e
	}
	op, e := s.store.ActiveOperation(ctx, owner, id)
	if errors.Is(e, ErrNotFound) {
		return state, nil
	}
	if e != nil {
		return state, e
	}
	if op.JobID == "" {
		latest, e := s.jobs.Latest(ctx, owner)
		if e != nil {
			return state, e
		}
		if latest != nil && receiptMatches(latest.Payload, op) {
			state, e = s.store.Bind(ctx, owner, op.ID, latest.ID)
			if e != nil {
				return state, e
			}
			op.JobID = latest.ID
		}
		// An in-flight start is never interpreted as a crash. A later process only releases
		// abandoned preparation after a bounded grace; it never repeats provider work.
		if op.JobID == "" {
			if s.now().Sub(op.CreatedAt) < 30*time.Second {
				return state, nil
			}
			return s.store.Reject(ctx, owner, op.ID, "UNKNOWN_FAILURE")
		}
	}
	found, e := s.jobs.Get(ctx, owner, op.JobID)
	if e != nil {
		return state, e
	}
	if !receiptMatches(found.Payload, op) {
		return state, ErrStale
	}
	switch found.Status {
	case "done":
		result, e := s.readResult(state.Kind, op, found)
		if e != nil {
			return s.store.Reconcile(ctx, owner, op.ID, "failed", "AUTHORING_OUTPUT_INVALID", nil)
		}
		return s.store.Reconcile(ctx, owner, op.ID, "done", "", &result)
	case "failed", "cancelled":
		reason := found.FailureReason
		if reason == "" && found.Status == "failed" {
			reason = "UNKNOWN_FAILURE"
		}
		return s.store.Reconcile(ctx, owner, op.ID, found.Status, reason, nil)
	default:
		return state, nil
	}
}
func (s *Service) eligible(ref llm.ModelRef) (llm.ModelInfo, error) {
	info, ok := s.models.Resolve(ref)
	grade := info.Levels[llm.StageNameWrite]
	if ref.ProviderID == "" || ref.ModelID == "" || !ok || info.Disabled || !info.ServesStage(llm.StageNameWrite) || (grade != "free" && grade != "value" && grade != "balanced" && grade != "premium" && grade != "top") {
		return info, ErrModel
	}
	return info, nil
}

// RecommendationOutputChars is the bounded generation allowance the config adapter sizes.
func RecommendationOutputChars(kind Kind) int {
	switch kind {
	case WritingVoice:
		return 8 * 1000
	case PostGuideline, VideoGuideline:
		return 8 * 600
	default:
		return 8 * 2200
	}
}
func (s *Service) Estimate(ctx context.Context, kind Kind, mode Mode, ref llm.ModelRef) (Estimate, error) {
	return s.EstimateFor(ctx, "", kind, mode, ref, "")
}
func (s *Service) EstimateFor(ctx context.Context, owner string, kind Kind, mode Mode, ref llm.ModelRef, sessionID string) (Estimate, error) {
	return s.EstimateCount(ctx, owner, kind, mode, ref, sessionID, 0)
}
func (s *Service) EstimateCount(ctx context.Context, owner string, kind Kind, mode Mode, ref llm.ModelRef, sessionID string, count int) (Estimate, error) {
	count, e := NormalizeCandidateCount(count)
	if e != nil {
		return Estimate{}, e
	}
	if !kind.Valid() {
		return Estimate{}, ErrInvalidKind
	}
	if !mode.Valid() {
		return Estimate{}, ErrInvalid
	}
	info, e := s.eligible(ref)
	if e != nil {
		return Estimate{}, e
	}
	in := operationInput{Kind: kind, Mode: mode, CandidateCount: count, Guide: s.targets.Guide(kind)}
	chars := 0
	if sessionID != "" {
		state, e := s.Get(ctx, owner, sessionID)
		if e != nil {
			return Estimate{}, e
		}
		if state.Kind != kind {
			return Estimate{}, ErrInvalidKind
		}
		in.Purpose = state.Purpose
		in.SourceContext = state.SourceContext
		in.ReferencePost = state.ReferencePost
		if current := currentSource(state); current != nil {
			a := artifactToWire(*current)
			in.Selected = &a
			chars = utf8.RuneCountInString(a.Name + a.Description + a.Body + a.TitleArea)
		}
	} else if mode == Refine {
		chars = MaxDocumentChars
	}
	cap := s.completionCap(in, chars, info)
	if cap <= 0 {
		return Estimate{}, ErrModel
	}
	if info.Levels[llm.StageNameWrite] == "free" {
		if !llm.ZeroUnitPrice(info.InputUSDPerMillion) || !llm.ZeroUnitPrice(info.OutputUSDPerMillion) {
			return Estimate{}, ErrModel
		}
		return Estimate{Free: true, Available: true}, nil
	}
	credits, available := s.estimates.CallCredits(ctx, info, MaxModelPromptChars, int64(cap))
	return Estimate{Credits: credits, Available: available}, nil
}
func (s *Service) Start(ctx context.Context, owner string, in Start) (string, Session, error) {
	defer s.lock(owner)()
	if !in.Mode.Valid() || !requestKey(in.RequestID) || utf8.RuneCountInString(in.Prompt) > MaxPromptChars || (in.Mode == Refine && strings.TrimSpace(in.Prompt) == "") {
		return "", Session{}, ErrInvalid
	}
	count, e := NormalizeCandidateCount(in.RequestedCandidateCount)
	if e != nil {
		return "", Session{}, e
	}
	in.RequestedCandidateCount = count
	hash := sha256.Sum256([]byte(fmt.Sprintf("%d\x00%s\x00%s\x00%s\x00%d", in.ExpectedRevision, in.Mode, in.Prompt, in.WriteModel.String(), count)))
	fingerprint := hex.EncodeToString(hash[:])
	prior, e := s.store.Operation(ctx, owner, in.SessionID, in.RequestID)
	if e == nil {
		if prior.Fingerprint != fingerprint {
			return "", Session{}, ErrStale
		}
		state, e := s.get(ctx, owner, in.SessionID)
		if e != nil {
			return "", state, e
		}
		if prior.JobID == "" {
			prior, _ = s.store.Operation(ctx, owner, in.SessionID, in.RequestID)
		}
		if prior.JobID == "" {
			return "", state, ErrBusy
		}
		return prior.JobID, state, nil
	}
	if !errors.Is(e, ErrNotFound) {
		return "", Session{}, e
	}
	state, e := s.get(ctx, owner, in.SessionID)
	if e != nil {
		return "", state, e
	}
	if state.Revision != in.ExpectedRevision {
		return "", state, ErrStale
	}
	if state.ActiveRequestID != "" || state.Phase == "saving" || state.Phase == "saved" {
		return "", state, ErrBusy
	}
	if in.Mode == Refine {
		if currentSource(state) == nil {
			return "", state, ErrNoSelection
		}
		done := 0
		for _, t := range state.Turns {
			if t.Status == "done" {
				done++
			}
		}
		if done >= MaxTurns {
			return "", state, ErrHistoryFull
		}
	}
	if e = s.targets.CanStart(ctx, owner, state.Kind, state.TargetID); e != nil {
		return "", state, e
	}
	info, e := s.eligible(in.WriteModel)
	if e != nil {
		return "", state, e
	}
	op := Operation{ID: newID(), UserID: owner, SessionID: state.ID, RequestID: in.RequestID, Fingerprint: fingerprint, BaseRevision: in.ExpectedRevision, Mode: in.Mode, CreatedAt: s.now().UTC()}
	input, e := s.freezeInput(state, op, in, info)
	if e != nil {
		return "", state, e
	}
	payload, e := encodeInput(input)
	if e != nil {
		return "", state, e
	}
	op.Payload = payload
	state, op, created, e := s.store.Reserve(ctx, owner, state.ID, in.ExpectedRevision, op)
	if e != nil {
		return "", state, e
	}
	if !created {
		if op.JobID == "" {
			state, e = s.get(ctx, owner, state.ID)
			if e != nil {
				return "", state, e
			}
			op, e = s.store.Operation(ctx, owner, state.ID, in.RequestID)
			if e != nil {
				return "", state, e
			}
			if op.JobID == "" {
				return "", state, ErrBusy
			}
		}
		return op.JobID, state, nil
	}
	id, e := s.jobs.Enqueue(ctx, JobRequest{UserID: owner, WriteModel: in.WriteModel.String(), Payload: payload, CompletionTokens: input.CompletionTokens, PromptTokens: input.PromptTokens})
	if e != nil {
		_, _ = s.store.Reject(context.WithoutCancel(ctx), owner, op.ID, failureReason(e))
		return "", state, e
	}
	state, e = s.store.Bind(context.WithoutCancel(ctx), owner, op.ID, id)
	return id, state, e
}
func failureReason(err error) string {
	f := llm.NormalizeFailure(err)
	if f.Reason == "" {
		return "UNKNOWN_FAILURE"
	}
	return f.Reason
}
func (s *Service) Select(ctx context.Context, owner, id string, revision uint32, candidateID string) (Session, error) {
	defer s.lock(owner)()
	if _, e := s.get(ctx, owner, id); e != nil {
		return Session{}, e
	}
	return s.store.Select(ctx, owner, id, revision, candidateID)
}
func (s *Service) Cancel(ctx context.Context, owner, id, jobID string) (Session, error) {
	defer s.lock(owner)()
	state, e := s.get(ctx, owner, id)
	if e != nil {
		return state, e
	}
	found, e := s.jobs.Get(ctx, owner, jobID)
	if e != nil {
		return state, e
	}
	sessionID, _, _, e := envelopeIdentity(found.Payload)
	if e != nil || sessionID != id {
		return state, ErrNotFound
	}
	if found.Status == "done" || found.Status == "failed" || found.Status == "cancelled" {
		return state, nil
	}
	if state.ActiveJobID != jobID {
		return state, ErrStale
	}
	if e = s.jobs.Cancel(ctx, owner, jobID); e != nil {
		return state, e
	}
	return s.get(ctx, owner, id)
}
func (s *Service) Save(ctx context.Context, owner, id string, revision uint32, makeDefault bool) (Session, error) {
	defer s.lock(owner)()
	state, e := s.get(ctx, owner, id)
	if e != nil {
		return state, e
	}
	if state.Phase == "saved" {
		return state, nil
	}
	if state.Selected == nil {
		return state, ErrNoSelection
	}
	if state.Phase != "saving" && state.DraftState != DraftValid {
		return state, ErrDraftInvalid
	}
	if state.Phase != "saving" {
		if e = s.targets.Validate(state.Kind, *state.Selected); e != nil {
			return state, fmt.Errorf("%w: %w", ErrOutput, e)
		}
	}
	state, p, e := s.store.PrepareSave(ctx, owner, id, revision, makeDefault)
	if e != nil {
		return state, e
	}
	ref, e := s.targets.Publish(ctx, p)
	if e != nil {
		s.publicationFailure(ctx, owner, id, e)
		if errors.Is(e, ErrNotFound) {
			return state, ErrTargetConflict
		}
		return state, e
	}
	if ref.Kind != state.Kind || ref.ID == "" {
		return state, ErrPublication
	}
	return s.finalizePublication(ctx, owner, id, p, ref)
}
