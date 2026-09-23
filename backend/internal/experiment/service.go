package experiment

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"
)

type Service struct {
	runs       RunLedger
	purge      RunRetention
	candidates CandidateLedger
	outcome    OutcomeLedger
	catalog    Catalog
	jobs       Jobs
	runner     Runner
	posts      PostDirectory
	voices     VoiceDirectory
	retention  time.Duration
	applyMu    sync.Mutex
	adoptMu    sync.Mutex
	now        func() time.Time
	newID      func() string
}

func NewService(store Storage, catalog Catalog, jobs Jobs, runner Runner, posts PostDirectory, retention time.Duration) *Service {
	if retention <= 0 {
		panic("experiment: retention must be positive")
	}
	if posts == nil {
		panic("experiment: post directory is required")
	}
	return &Service{runs: store, candidates: store, outcome: store, purge: store, catalog: catalog, jobs: jobs, runner: runner, posts: posts, retention: retention, now: time.Now, newID: newID}
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
	if request.Stage == StageWrite {
		if snapshot.TargetLanguage == nil || !snapshot.TargetLanguage.Valid() {
			return StartResult{}, ErrLanguageRequired
		}
	} else if snapshot.TargetLanguage != nil {
		return StartResult{}, ErrLanguageRequired
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
		ID: s.newID(), UserID: request.UserID, PostSlug: request.PostSlug, VoiceID: frozen.VoiceID,
		TemplateName: frozen.TemplateName, TargetLanguage: cloneLanguage(frozen.TargetLanguage), Stage: request.Stage,
		Origin: startOrigin(request), Status: StatusQueued, InputSnapshot: frozen.Content, InputHash: hash,
		PromptVersion: frozen.PromptVersion, CreatedAt: s.now(),
	}
	found.Candidates = []Candidate{
		{ID: s.newID(), ExperimentID: found.ID, Model: request.ModelA, ModelLabel: modelA.Label, DisplaySide: sides[0], Status: CandidatePending},
		{ID: s.newID(), ExperimentID: found.ID, Model: request.ModelB, ModelLabel: modelB.Label, DisplaySide: sides[1], Status: CandidatePending},
	}
	if err := s.runs.Create(ctx, found); err != nil {
		if errors.Is(err, ErrInvalidState) && request.Stage == StageWrite {
			if pending, findErr := s.runs.PendingForPost(ctx, request.UserID, request.PostSlug); findErr == nil && pending != nil {
				return StartResult{ExperimentID: pending.ID, JobID: pending.JobID}, &JobAlreadyInProgressError{ActiveID: pending.JobID}
			}
		}
		return StartResult{}, err
	}
	jobID, err := s.jobs.EnqueueExperiment(ctx, JobRequest{
		UserID: request.UserID, PostSlug: request.PostSlug, VoiceID: found.VoiceID, ExperimentID: found.ID, Stage: request.Stage,
		TargetLanguage: cloneLanguage(found.TargetLanguage),
		Models:         startModels(request),
	})
	if err != nil {
		_ = s.runs.Delete(ctx, found.ID)
		return StartResult{}, err
	}
	if err := s.runs.SetJob(ctx, found.ID, request.UserID, jobID); err != nil {
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
	return s.runs.List(ctx, userID, stage)
}

func (s *Service) PendingForPost(ctx context.Context, userID, postSlug string) (*Experiment, error) {
	return s.runs.PendingForPost(ctx, userID, postSlug)
}

func (s *Service) PurgePost(ctx context.Context, userID, postSlug string) error {
	return s.purge.PurgePost(ctx, userID, postSlug)
}

// HasPublishableForVoice is the guard the voice context asks before a soft delete: an
// experiment frozen to the voice that is unfinished, awaiting a verdict, or decided but not
// yet applied could still write into it.
func (s *Service) HasPublishableForVoice(ctx context.Context, userID, voiceID string) (bool, error) {
	n, err := s.runs.CountPublishableForVoice(ctx, userID, voiceID)
	return n > 0, err
}

// RecoverInterrupted turns experiments left running by a process exit into retryable
// terminal states. It runs before workers start, so it cannot race a live candidate.
func (s *Service) RecoverInterrupted(ctx context.Context) (int64, error) {
	now := s.now()
	count, err := s.candidates.RecoverInterrupted(ctx, interruptedFailure, now)
	if err != nil {
		return 0, err
	}
	queued, err := s.runs.ListQueued(ctx)
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
		if err := s.candidates.FailUnfinished(ctx, id, interruptedFailure, now); err != nil {
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
	// An editor retry re-prepares into the post, so it asks first; a lab retry writes nothing
	// to the post and stays open whatever its status.
	if found.AppliesOnVerdict() {
		if err := s.allowPostWrite(ctx, found); err != nil {
			return StartResult{}, err
		}
	}
	count, err := s.candidates.ResetFailedCandidates(ctx, found.ID)
	if err != nil {
		return StartResult{}, err
	}
	if count == 0 {
		return StartResult{}, ErrInvalidState
	}
	if err := s.runs.SetStatus(ctx, found.ID, StatusQueued, nil); err != nil {
		_ = s.candidates.RestoreFailedCandidates(ctx, found.ID, found.Candidates)
		return StartResult{}, err
	}
	// A retry re-runs the candidates the experiment froze, so it passes them through the gate
	// again: a downgrade after the first run must refuse a model that is now above the tier.
	jobID, err := s.jobs.EnqueueExperiment(ctx, JobRequest{
		UserID: userID, PostSlug: found.PostSlug, VoiceID: found.VoiceID, ExperimentID: found.ID, Stage: found.Stage,
		TargetLanguage: cloneLanguage(found.TargetLanguage), Models: candidateModels(found),
	})
	if err != nil {
		_ = s.candidates.RestoreFailedCandidates(ctx, found.ID, found.Candidates)
		_ = s.runs.SetStatus(ctx, found.ID, found.Status, found.FinishedAt)
		return StartResult{}, err
	}
	if err := s.runs.SetJob(ctx, found.ID, userID, jobID); err != nil {
		return StartResult{}, err
	}
	return StartResult{ExperimentID: found.ID, JobID: jobID}, nil
}

// startOrigin freezes where a comparison was started. Observe and analyze can only be
// started in the lab; a write comparison honours its caller, and an absent value means the
// editor so that a client predating the field keeps the behaviour it was written against.
func startOrigin(request StartRequest) Origin {
	if request.Stage != StageWrite {
		return OriginLab
	}
	if request.Origin == OriginLab {
		return OriginLab
	}
	return OriginEditor
}

// Choose records a verdict. For a paired winner it is the lab's whole decision (MODEL-36):
// the verdict is written and nothing is applied, so an editor write comparison — whose
// verdict commits — has no pick-only form and is decided through DecideWrite instead.
//
// single is the other path entirely: the survivor of a comparison whose sibling failed. It
// ranks nothing (MODEL-34) and stays available to both origins, because a post written as an
// editor comparison still has no content until that survivor is applied (MODEL-37).
func (s *Service) Choose(ctx context.Context, userID, id, candidateID string, single bool, badges []CandidateBadges) (Experiment, error) {
	found, err := s.owned(ctx, userID, id)
	if err != nil {
		return Experiment{}, err
	}
	if !single && found.AppliesOnVerdict() {
		return Experiment{}, ErrInvalidState
	}
	return s.choose(ctx, userID, id, candidateID, single, false, badges)
}

func (s *Service) choose(ctx context.Context, userID, id, candidateID string, single, adoptionRequested bool, badges []CandidateBadges) (Experiment, error) {
	found, err := s.owned(ctx, userID, id)
	if err != nil {
		return Experiment{}, err
	}
	if found.Status == StatusDecided && found.WinnerCandidateID == candidateID {
		// An applied verdict repeated answers what is stored, whatever the post is now.
		if found.AppliesOnVerdict() && found.AppliedAt == nil {
			if err := s.allowPostWrite(ctx, found); err != nil {
				return Experiment{}, err
			}
			return s.apply(ctx, found, false)
		}
		return found, nil
	}
	// A verdict that commits is refused before it is recorded: an editor verdict on a post that
	// cannot take its result records nothing.
	if found.AppliesOnVerdict() {
		if err := s.allowPostWrite(ctx, found); err != nil {
			return Experiment{}, err
		}
	}
	candidate, err := ValidateVerdict(found, candidateID, single)
	if err != nil {
		return Experiment{}, err
	}
	// Refused before the verdict is written: a verdict recorded with its explanation dropped
	// would be a verdict the owner did not give.
	normalized, err := NormalizeBadges(found, badges)
	if err != nil {
		return Experiment{}, err
	}
	outcome := OutcomeWinner
	if single {
		outcome = OutcomeUnpaired
	}
	now := s.now()
	changed, err := s.outcome.Decide(ctx, found.ID, userID, candidate.ID, StatusDecided, outcome, found.AppliesOnVerdict(), adoptionRequested, normalized, now, now.Add(s.retention))
	if err != nil {
		return Experiment{}, err
	}
	if !changed {
		current, loadErr := s.owned(ctx, userID, id)
		if loadErr == nil && current.Status == StatusDecided && current.WinnerCandidateID == candidateID {
			if current.AppliesOnVerdict() && current.AppliedAt == nil {
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
	if found.AppliesOnVerdict() {
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
	// Dismissal carries no badges: nothing was chosen, so there is nothing to explain
	// (MODEL-37).
	changed, err := s.outcome.Decide(ctx, found.ID, userID, "", StatusDismissed, OutcomeSkipped, false, false, nil, now, now.Add(s.retention))
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
	if err := s.allowPostWrite(ctx, found); err != nil {
		return Experiment{}, err
	}
	// Mark the debt BEFORE the runner is called: from here on a failure has to leave the
	// comparison unresolved for its post, with the retry the owner can see. A lab pick
	// carried no such debt until this moment, so without the mark a failed application
	// would silently resolve (MODEL-36).
	if !found.ApplyRequested {
		if err := s.outcome.SetApplyRequested(ctx, found.ID, userID); err != nil {
			return Experiment{}, err
		}
		if found, err = s.owned(ctx, userID, id); err != nil {
			return Experiment{}, err
		}
	}
	return s.apply(ctx, found, confirmStyleguide)
}

// allowPostWrite decides whether a comparison's result may land in its post as the post is
// now (MODEL-37). An analyze winner publishes into a voice and never asks. A draft or a post in
// revision takes either origin's result, and a published post takes neither, since it is locked
// (POST-74). Any other status takes the editor's, whose result reopens a finalized post as
// saving content does (POST-13), and refuses the lab's: its gate is an allowlist.
func (s *Service) allowPostWrite(ctx context.Context, found Experiment) error {
	if found.Stage == StageAnalyze {
		return nil
	}
	if found.PostSlug == "" {
		return ErrInvalidState
	}
	status, err := s.posts.Status(ctx, found.UserID, found.PostSlug)
	if err != nil {
		return err
	}
	switch status {
	case PostStatusDraft, PostStatusReview:
		return nil
	case PostStatusPublished:
		return ErrPostPublished
	}
	if found.Origin != OriginLab {
		return nil
	}
	return ErrPostFinalized
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
func (s *Service) DecideWrite(ctx context.Context, userID, id, candidateID string, adopt bool, badges []CandidateBadges) (Experiment, error) {
	found, err := s.owned(ctx, userID, id)
	if err != nil {
		return Experiment{}, err
	}
	if found.Stage != StageWrite {
		return Experiment{}, ErrInvalidStage
	}
	// The committing write verdict belongs to the editor alone: a lab comparison picks a
	// winner and applies nothing (MODEL-36).
	if found.Origin != OriginEditor {
		return Experiment{}, ErrInvalidState
	}
	found, err = s.choose(ctx, userID, id, candidateID, false, adopt, badges)
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
		slog.Error("experiment winner adoption model unavailable", "experiment_id", found.ID, "err", err)
		if storeErr := s.outcome.SetAdoptionFailure(ctx, found.ID, userID, normalizeFailure(err)); storeErr != nil {
			return Experiment{}, storeErr
		}
		return s.owned(ctx, userID, id)
	}
	active, selected, err := s.catalog.Active(ctx, userID, StageWrite)
	if err != nil {
		slog.Error("experiment active winner lookup failed", "experiment_id", found.ID, "err", err)
		if storeErr := s.outcome.SetAdoptionFailure(ctx, found.ID, userID, normalizeFailure(err)); storeErr != nil {
			return Experiment{}, storeErr
		}
		return s.owned(ctx, userID, id)
	}
	if selected && active == winner.Model {
		if err := s.outcome.SetAdopted(ctx, found.ID, userID, s.now()); err != nil {
			return Experiment{}, err
		}
		return s.owned(ctx, userID, id)
	}
	if err := s.catalog.Adopt(ctx, userID, StageWrite, winner.Model); err != nil {
		slog.Error("experiment winner adoption failed", "experiment_id", found.ID, "err", err)
		if storeErr := s.outcome.SetAdoptionFailure(ctx, found.ID, userID, normalizeFailure(err)); storeErr != nil {
			return Experiment{}, storeErr
		}
		return s.owned(ctx, userID, id)
	}
	if err := s.outcome.SetAdopted(ctx, found.ID, userID, s.now()); err != nil {
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

// Leaderboard replays the winner verdicts of one (scope, stage, window) from 1500. The
// window is rolling and measured here, at the moment of the request (MODEL-38).
func (s *Service) Leaderboard(ctx context.Context, userID string, stage Stage, window Window, scope Scope) ([]LeaderboardEntry, error) {
	if _, err := ParseStage(string(stage)); err != nil {
		return nil, err
	}
	if _, err := ParseWindow(string(window)); err != nil {
		return nil, err
	}
	if _, err := ParseScope(string(scope)); err != nil {
		return nil, err
	}
	decided, calls, tallies, err := s.outcome.LeaderboardData(ctx, userID, stage, s.now().Add(-window.Length()), scope)
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
	entries := BuildLeaderboard(matches, calls, labels, tallies)
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
		_ = s.outcome.SetApplyFailure(ctx, found.ID, found.UserID, normalizeFailure(err))
		return s.owned(ctx, found.UserID, found.ID)
	}
	if err := s.outcome.SetApplied(ctx, found.ID, found.UserID, s.now()); err != nil {
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
	found, err := s.runs.Get(ctx, id)
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
	// Membership in the stage's template (change 20) replaced the observe-needs-vision check:
	// the registration gate already required vision, and a deregistered model must be as
	// unusable for an experiment as it is in the picker.
	if !ok || !model.Enabled || !slices.Contains(model.Stages, string(stage)) {
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

// startModels are every ref a comparison will run. A write comparison also runs the caller's
// explicit observe model over the post's photos, and that call spends tokens like any other —
// gating only the two candidates would leave one paid stage ungated.
func startModels(request StartRequest) []string {
	models := []string{request.ModelA.String(), request.ModelB.String()}
	if request.Stage == StageWrite && request.ObserveModel.ProviderID != "" {
		models = append(models, request.ObserveModel.String())
	}
	return models
}

// candidateModels are the refs a retry will re-run, in candidate order.
func candidateModels(found Experiment) []string {
	models := make([]string, 0, len(found.Candidates))
	for _, candidate := range found.Candidates {
		models = append(models, candidate.Model.String())
	}
	return models
}
