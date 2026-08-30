package experiment

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

const (
	interruptedReason = "서버가 재시작되어 비교가 중단됐어요. 실패한 후보를 다시 시도해 주세요."
	applyFailedReason = "선택한 결과를 적용하지 못했어요. 다시 시도해 주세요."
	adoptFailedReason = "결과는 적용했지만 활성 모델은 변경하지 못했어요. 다시 시도해 주세요."
)

type Service struct {
	store     Store
	catalog   Catalog
	jobs      Jobs
	runner    Runner
	voices    VoiceDirectory
	retention time.Duration
	applyMu   sync.Mutex
	adoptMu   sync.Mutex
	now       func() time.Time
	newID     func() string
}

func NewService(store Store, catalog Catalog, jobs Jobs, runner Runner, retention time.Duration) *Service {
	if retention <= 0 {
		panic("experiment: retention must be positive")
	}
	return &Service{store: store, catalog: catalog, jobs: jobs, runner: runner, retention: retention, now: time.Now, newID: newID}
}

// SetVoiceDirectory wires the voice context's published check once both services exist.
func (s *Service) SetVoiceDirectory(voices VoiceDirectory) { s.voices = voices }

// requireActiveVoice refuses work in a voice that is not the account's or is a tombstone.
func (s *Service) requireActiveVoice(ctx context.Context, userID, voiceID string) error {
	if voiceID == "" {
		return nil
	}
	if s.voices == nil {
		return ErrVoiceUnavailable
	}
	return s.voices.ActiveVoice(ctx, userID, voiceID)
}

func (s *Service) Start(ctx context.Context, request StartRequest) (StartResult, error) {
	if _, err := ParseStage(string(request.Stage)); err != nil {
		return StartResult{}, err
	}
	if request.TargetLength != nil && *request.TargetLength <= 0 {
		return StartResult{}, ErrInvalidTargetLength
	}
	if request.Stage == StageAnalyze {
		if request.VoiceID == "" {
			return StartResult{}, ErrVoiceRequired
		}
		if err := s.requireActiveVoice(ctx, request.UserID, request.VoiceID); err != nil {
			return StartResult{}, err
		}
	} else {
		request.VoiceID = ""
	}
	if request.ModelA == request.ModelB {
		return StartResult{}, ErrDuplicateCandidates
	}
	modelA, err := s.resolveForStage(request.Stage, request.ModelA)
	if err != nil {
		return StartResult{}, err
	}
	modelB, err := s.resolveForStage(request.Stage, request.ModelB)
	if err != nil {
		return StartResult{}, err
	}
	snapshot, err := s.runner.Snapshot(ctx, request)
	if err != nil {
		return StartResult{}, err
	}
	frozen, hash, err := FreezeSnapshot(snapshot)
	if err != nil {
		return StartResult{}, err
	}
	leftA, err := randomBool()
	if err != nil {
		return StartResult{}, fmt.Errorf("assign candidate sides: %w", err)
	}
	sides := []DisplaySide{SideLeft, SideRight}
	if !leftA {
		sides[0], sides[1] = sides[1], sides[0]
	}
	found := Experiment{
		ID: s.newID(), UserID: request.UserID, PostSlug: request.PostSlug, VoiceID: frozen.VoiceID, Stage: request.Stage,
		Status: StatusQueued, InputSnapshot: frozen.Content, InputHash: hash,
		PromptVersion: frozen.PromptVersion, CreatedAt: s.now(),
	}
	found.Candidates = []Candidate{
		{ID: s.newID(), ExperimentID: found.ID, Model: request.ModelA, ModelLabel: modelA.Label, DisplaySide: sides[0], Status: CandidatePending},
		{ID: s.newID(), ExperimentID: found.ID, Model: request.ModelB, ModelLabel: modelB.Label, DisplaySide: sides[1], Status: CandidatePending},
	}
	if err := s.store.Create(ctx, found); err != nil {
		if errors.Is(err, ErrInvalidState) && request.Stage == StageWrite {
			if pending, findErr := s.store.PendingForPost(ctx, request.UserID, request.PostSlug); findErr == nil && pending != nil {
				return StartResult{ExperimentID: pending.ID, JobID: pending.JobID}, &JobAlreadyInProgressError{ActiveID: pending.JobID}
			}
		}
		return StartResult{}, err
	}
	jobID, err := s.jobs.EnqueueExperiment(ctx, JobRequest{
		UserID: request.UserID, PostSlug: request.PostSlug, VoiceID: found.VoiceID, ExperimentID: found.ID, Stage: request.Stage,
	})
	if err != nil {
		_ = s.store.Delete(ctx, found.ID)
		return StartResult{}, err
	}
	if err := s.store.SetJob(ctx, found.ID, request.UserID, jobID); err != nil {
		return StartResult{}, fmt.Errorf("link experiment job: %w", err)
	}
	return StartResult{ExperimentID: found.ID, JobID: jobID}, nil
}

func (s *Service) Get(ctx context.Context, userID, id string) (Experiment, error) {
	return s.owned(ctx, userID, id)
}

func (s *Service) List(ctx context.Context, userID string, stage Stage) ([]Experiment, error) {
	if stage != "" {
		if _, err := ParseStage(string(stage)); err != nil {
			return nil, err
		}
	}
	return s.store.List(ctx, userID, stage)
}

func (s *Service) PendingForPost(ctx context.Context, userID, postSlug string) (*Experiment, error) {
	return s.store.PendingForPost(ctx, userID, postSlug)
}

func (s *Service) PurgePost(ctx context.Context, userID, postSlug string) error {
	return s.store.PurgePost(ctx, userID, postSlug)
}

// HasPublishableForVoice is the guard the voice context asks before a soft delete: an
// experiment frozen to the voice that is unfinished, awaiting a verdict, or decided but not
// yet applied could still write into it.
func (s *Service) HasPublishableForVoice(ctx context.Context, userID, voiceID string) (bool, error) {
	n, err := s.store.CountPublishableForVoice(ctx, userID, voiceID)
	return n > 0, err
}

// RecoverInterrupted turns experiments left running by a process exit into retryable
// terminal states. It runs before workers start, so it cannot race a live candidate.
func (s *Service) RecoverInterrupted(ctx context.Context) (int64, error) {
	now := s.now()
	count, err := s.store.RecoverInterrupted(ctx, interruptedReason, now)
	if err != nil {
		return 0, err
	}
	queued, err := s.store.ListQueued(ctx)
	if err != nil {
		return count, err
	}
	for _, id := range queued {
		runnable, err := s.jobs.HasRunnableExperiment(ctx, id)
		if err != nil {
			return count, err
		}
		if runnable {
			continue
		}
		if err := s.store.FailUnfinished(ctx, id, interruptedReason, now); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func (s *Service) Retry(ctx context.Context, userID, id string) (StartResult, error) {
	found, err := s.owned(ctx, userID, id)
	if err != nil {
		return StartResult{}, err
	}
	if found.Status != StatusPartial && found.Status != StatusFailed {
		return StartResult{}, ErrInvalidState
	}
	if len(found.InputSnapshot) == 0 {
		return StartResult{}, ErrSnapshotUnavailable
	}
	if err := s.requireActiveVoice(ctx, userID, found.VoiceID); err != nil {
		return StartResult{}, err
	}
	for _, candidate := range found.Candidates {
		if candidate.Status == CandidateFailed {
			if _, err := s.resolveForStage(found.Stage, candidate.Model); err != nil {
				return StartResult{}, ErrRetryModelUnavailable
			}
		}
	}
	count, err := s.store.ResetFailedCandidates(ctx, found.ID)
	if err != nil {
		return StartResult{}, err
	}
	if count == 0 {
		return StartResult{}, ErrInvalidState
	}
	if err := s.store.SetStatus(ctx, found.ID, StatusQueued, nil); err != nil {
		_ = s.store.RestoreFailedCandidates(ctx, found.ID, found.Candidates)
		return StartResult{}, err
	}
	jobID, err := s.jobs.EnqueueExperiment(ctx, JobRequest{UserID: userID, PostSlug: found.PostSlug, VoiceID: found.VoiceID, ExperimentID: found.ID, Stage: found.Stage})
	if err != nil {
		_ = s.store.RestoreFailedCandidates(ctx, found.ID, found.Candidates)
		_ = s.store.SetStatus(ctx, found.ID, found.Status, found.FinishedAt)
		return StartResult{}, err
	}
	if err := s.store.SetJob(ctx, found.ID, userID, jobID); err != nil {
		return StartResult{}, err
	}
	return StartResult{ExperimentID: found.ID, JobID: jobID}, nil
}

func (s *Service) Choose(ctx context.Context, userID, id, candidateID string, single bool) (Experiment, error) {
	return s.choose(ctx, userID, id, candidateID, single, false)
}

func (s *Service) choose(ctx context.Context, userID, id, candidateID string, single, adoptionRequested bool) (Experiment, error) {
	found, err := s.owned(ctx, userID, id)
	if err != nil {
		return Experiment{}, err
	}
	if found.Status == StatusDecided && found.WinnerCandidateID == candidateID {
		if found.Stage == StageWrite && found.AppliedAt == nil {
			return s.apply(ctx, found, false)
		}
		return found, nil
	}
	candidate, err := ValidateVerdict(found, candidateID, single)
	if err != nil {
		return Experiment{}, err
	}
	outcome := OutcomeWinner
	if single {
		outcome = OutcomeUnpaired
	}
	now := s.now()
	changed, err := s.store.Decide(ctx, found.ID, userID, candidate.ID, StatusDecided, outcome, adoptionRequested, now, now.Add(s.retention))
	if err != nil {
		return Experiment{}, err
	}
	if !changed {
		current, loadErr := s.owned(ctx, userID, id)
		if loadErr == nil && current.Status == StatusDecided && current.WinnerCandidateID == candidateID {
			if current.Stage == StageWrite && current.AppliedAt == nil {
				return s.apply(ctx, current, false)
			}
			return current, nil
		}
		return Experiment{}, ErrInvalidState
	}
	found, err = s.owned(ctx, userID, id)
	if err != nil {
		return Experiment{}, err
	}
	if found.Stage == StageWrite {
		return s.apply(ctx, found, false)
	}
	return found, nil
}

func (s *Service) Dismiss(ctx context.Context, userID, id string) (Experiment, error) {
	found, err := s.owned(ctx, userID, id)
	if err != nil {
		return Experiment{}, err
	}
	if found.Status == StatusDismissed {
		return found, nil
	}
	if found.Status != StatusReview && found.Status != StatusPartial && found.Status != StatusFailed {
		return Experiment{}, ErrInvalidState
	}
	now := s.now()
	changed, err := s.store.Decide(ctx, found.ID, userID, "", StatusDismissed, OutcomeSkipped, false, now, now.Add(s.retention))
	if err != nil {
		return Experiment{}, err
	}
	if !changed {
		return Experiment{}, ErrInvalidState
	}
	return s.owned(ctx, userID, id)
}

func (s *Service) ApplyWinner(ctx context.Context, userID, id string, confirmStyleguide bool) (Experiment, error) {
	found, err := s.owned(ctx, userID, id)
	if err != nil {
		return Experiment{}, err
	}
	if found.Status != StatusDecided || found.Winner() == nil {
		return Experiment{}, ErrInvalidState
	}
	if found.AppliedAt != nil {
		return found, nil
	}
	if found.Stage == StageAnalyze && !confirmStyleguide {
		return Experiment{}, ErrConfirmationRequired
	}
	return s.apply(ctx, found, confirmStyleguide)
}

func (s *Service) AdoptWinner(ctx context.Context, userID, id string) (ModelRef, Stage, error) {
	found, err := s.owned(ctx, userID, id)
	if err != nil {
		return ModelRef{}, "", err
	}
	winner := found.Winner()
	if found.Status != StatusDecided || winner == nil {
		return ModelRef{}, "", ErrInvalidState
	}
	if _, err := s.resolveForStage(found.Stage, winner.Model); err != nil {
		return ModelRef{}, "", err
	}
	if err := s.catalog.Adopt(ctx, userID, found.Stage, winner.Model); err != nil {
		return ModelRef{}, "", err
	}
	return winner.Model, found.Stage, nil
}

// DecideWrite records one blind write verdict, applies the selected content exactly
// once, and optionally adopts only the winner's write model. Each completed boundary
// is persisted so an adoption retry never reapplies content or reranks the verdict.
func (s *Service) DecideWrite(ctx context.Context, userID, id, candidateID string, adopt bool) (Experiment, error) {
	found, err := s.owned(ctx, userID, id)
	if err != nil {
		return Experiment{}, err
	}
	if found.Stage != StageWrite {
		return Experiment{}, ErrInvalidStage
	}
	found, err = s.choose(ctx, userID, id, candidateID, false, adopt)
	if err != nil || !found.AdoptionRequested || found.AppliedAt == nil {
		return found, err
	}
	if found.AdoptedAt != nil {
		return found, nil
	}
	s.adoptMu.Lock()
	defer s.adoptMu.Unlock()
	found, err = s.owned(ctx, userID, id)
	if err != nil {
		return Experiment{}, err
	}
	if found.AdoptedAt != nil {
		return found, nil
	}
	winner := found.Winner()
	if winner == nil {
		return Experiment{}, ErrInvalidState
	}
	if _, err := s.resolveForStage(StageWrite, winner.Model); err != nil {
		if storeErr := s.store.SetAdoptionError(ctx, found.ID, userID, adoptFailedReason); storeErr != nil {
			return Experiment{}, storeErr
		}
		return s.owned(ctx, userID, id)
	}
	active, selected, err := s.catalog.Active(ctx, userID, StageWrite)
	if err != nil {
		slog.Error("experiment active winner lookup failed", "experiment_id", found.ID, "err", err)
		if storeErr := s.store.SetAdoptionError(ctx, found.ID, userID, adoptFailedReason); storeErr != nil {
			return Experiment{}, storeErr
		}
		return s.owned(ctx, userID, id)
	}
	if selected && active == winner.Model {
		if err := s.store.SetAdopted(ctx, found.ID, userID, s.now()); err != nil {
			return Experiment{}, err
		}
		return s.owned(ctx, userID, id)
	}
	if err := s.catalog.Adopt(ctx, userID, StageWrite, winner.Model); err != nil {
		slog.Error("experiment winner adoption failed", "experiment_id", found.ID, "err", err)
		if storeErr := s.store.SetAdoptionError(ctx, found.ID, userID, adoptFailedReason); storeErr != nil {
			return Experiment{}, storeErr
		}
		return s.owned(ctx, userID, id)
	}
	if err := s.store.SetAdopted(ctx, found.ID, userID, s.now()); err != nil {
		if errors.Is(err, ErrInvalidState) {
			current, loadErr := s.owned(ctx, userID, id)
			if loadErr == nil && current.AdoptedAt != nil {
				return current, nil
			}
		}
		return Experiment{}, err
	}
	return s.owned(ctx, userID, id)
}

func (s *Service) Leaderboard(ctx context.Context, userID string, stage Stage) ([]LeaderboardEntry, error) {
	if _, err := ParseStage(string(stage)); err != nil {
		return nil, err
	}
	decided, calls, err := s.store.LeaderboardData(ctx, userID, stage)
	if err != nil {
		return nil, err
	}
	byExperiment := map[string][]Candidate{}
	labels := map[ModelRef]string{}
	for _, candidate := range calls {
		byExperiment[candidate.ExperimentID] = append(byExperiment[candidate.ExperimentID], candidate)
		labels[candidate.Model] = candidate.ModelLabel
	}
	matches := make([]Match, 0, len(decided))
	for _, found := range decided {
		var winner, loser *Candidate
		for i := range byExperiment[found.ID] {
			candidate := &byExperiment[found.ID][i]
			if candidate.ID == found.WinnerCandidateID {
				winner = candidate
			} else {
				loser = candidate
			}
		}
		if winner != nil && loser != nil {
			matches = append(matches, Match{Winner: winner.Model, Loser: loser.Model})
		}
	}
	entries := BuildLeaderboard(matches, calls, labels)
	active, hasActive, err := s.catalog.Active(ctx, userID, stage)
	if err != nil {
		return nil, err
	}
	for i := range entries {
		entries[i].Active = hasActive && entries[i].Model == active
		entries[i].Recommended = s.catalog.Recommended(stage, entries[i].Model)
		_, present := s.catalog.Resolve(entries[i].Model)
		entries[i].Disappeared = !present
	}
	return entries, nil
}

func (s *Service) apply(ctx context.Context, found Experiment, confirmStyleguide bool) (Experiment, error) {
	// The post write and this aggregate's applied marker cannot share a transaction.
	// Serialize their read-call-mark sequence inside the single API process, then rely
	// on the post boundary's value-idempotent SQL for a crash between the two writes.
	s.applyMu.Lock()
	defer s.applyMu.Unlock()
	current, err := s.owned(ctx, found.UserID, found.ID)
	if err != nil {
		return Experiment{}, err
	}
	found = current
	if found.AppliedAt != nil {
		return found, nil
	}
	winner := found.Winner()
	if winner == nil || len(winner.Output) == 0 {
		return Experiment{}, ErrInvalidState
	}
	if err := s.runner.ApplyWinner(ctx, found, *winner, confirmStyleguide); err != nil {
		slog.Error("experiment winner apply failed", "experiment_id", found.ID, "stage", found.Stage, "err", err)
		_ = s.store.SetApplyError(ctx, found.ID, found.UserID, applyFailedReason)
		return s.owned(ctx, found.UserID, found.ID)
	}
	if err := s.store.SetApplied(ctx, found.ID, found.UserID, s.now()); err != nil {
		if errors.Is(err, ErrInvalidState) {
			current, loadErr := s.owned(ctx, found.UserID, found.ID)
			if loadErr == nil && current.AppliedAt != nil {
				return current, nil
			}
		}
		return Experiment{}, err
	}
	return s.owned(ctx, found.UserID, found.ID)
}

func (s *Service) owned(ctx context.Context, userID, id string) (Experiment, error) {
	found, err := s.store.Get(ctx, id)
	if err != nil {
		return Experiment{}, err
	}
	if found.UserID != userID {
		return Experiment{}, ErrForbidden
	}
	return found, nil
}

func (s *Service) resolveForStage(stage Stage, ref ModelRef) (Model, error) {
	if ref.ProviderID == "" || ref.ModelID == "" {
		return Model{}, ErrModelRequired
	}
	model, ok := s.catalog.Resolve(ref)
	if !ok || !model.Enabled || (stage == StageObserve && !model.Vision) {
		return Model{}, fmt.Errorf("%w: %s", ErrModelRequired, ref)
	}
	return model, nil
}

func randomBool() (bool, error) {
	var value [1]byte
	if _, err := rand.Read(value[:]); err != nil {
		return false, err
	}
	return value[0]&1 == 0, nil
}

func newID() string {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		panic("experiment: cannot create id: " + err.Error())
	}
	return hex.EncodeToString(value)
}

func (s *Service) runCandidates(ctx context.Context, found Experiment, progress Progress) error {
	var wg sync.WaitGroup
	var progressMu sync.Mutex
	errorsByCandidate := make(chan error, len(found.Candidates))
	completed := 0
	pending := 0
	for _, candidate := range found.Candidates {
		if candidate.Status == CandidatePending {
			pending++
		}
	}
	stage := "compare_" + string(found.Stage)
	progress(stage, 0, pending)
	for _, candidate := range found.Candidates {
		if candidate.Status != CandidatePending {
			continue
		}
		candidate := candidate
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.runCandidate(ctx, found, candidate, progress); err != nil {
				errorsByCandidate <- err
			}
			progressMu.Lock()
			completed++
			progress(stage, completed, pending)
			progressMu.Unlock()
		}()
	}
	wg.Wait()
	close(errorsByCandidate)
	for err := range errorsByCandidate {
		return err
	}
	return nil
}
