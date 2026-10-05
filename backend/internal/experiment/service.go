package experiment

import (
	"cmp"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
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
	reflection VoiceReflection
	retention  time.Duration
	// concurrency is how many candidates of one comparison may call providers at once.
	concurrency int
	applyMu     sync.Mutex
	adoptMu     sync.Mutex
	now         func() time.Time
	newID       func() string
}

func NewService(store Storage, catalog Catalog, jobs Jobs, runner Runner, posts PostDirectory, retention time.Duration, candidateConcurrency int) *Service {
	if retention <= 0 {
		panic("experiment: retention must be positive")
	}
	if candidateConcurrency <= 0 {
		panic("experiment: candidate concurrency must be positive")
	}
	if posts == nil {
		panic("experiment: post directory is required")
	}
	return &Service{runs: store, candidates: store, outcome: store, purge: store, catalog: catalog, jobs: jobs, runner: runner, posts: posts, retention: retention, concurrency: candidateConcurrency, now: time.Now, newID: newID}
}

// SetVoiceDirectory wires the voice context's published check once both services exist.
func (s *Service) SetVoiceDirectory(voices VoiceDirectory) { s.voices = voices }

// SetVoiceReflection wires the voice context's 말투 반영 비교 behaviour once both services exist.
func (s *Service) SetVoiceReflection(reflection VoiceReflection) { s.reflection = reflection }

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
	refs, reviewMode, err := startCandidates(request.ModelA, request.ModelB, request.Candidates, startOrigin(request))
	if err != nil {
		return StartResult{}, err
	}
	models, err := s.resolveCandidates(request.Stage, refs)
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
	observeCalls, err := s.runner.ObserveCalls(request.Stage, frozen.Content)
	if err != nil {
		return StartResult{}, fmt.Errorf("count observe calls: %w", err)
	}
	found := Experiment{
		ID: s.newID(), UserID: request.UserID, PostSlug: request.PostSlug, VoiceID: frozen.VoiceID,
		TemplateName: frozen.TemplateName, TargetLanguage: cloneLanguage(frozen.TargetLanguage), Stage: request.Stage,
		Origin: startOrigin(request), ReviewMode: reviewMode, Status: StatusQueued, InputSnapshot: frozen.Content, InputHash: hash,
		PromptVersion: frozen.PromptVersion, CreatedAt: s.now(),
	}
	job := JobRequest{
		UserID: request.UserID, PostSlug: request.PostSlug, VoiceID: found.VoiceID, ExperimentID: found.ID, Stage: request.Stage,
		TargetLanguage: cloneLanguage(found.TargetLanguage),
		Models:         refsToStrings(refs), ObserveModel: writeObserveModel(request), ObserveCalls: observeCalls,
	}
	return s.createQueued(ctx, found, refs, models, job, func(err error) (StartResult, error) {
		if errors.Is(err, ErrInvalidState) && request.Stage == StageWrite {
			if pending, findErr := s.runs.PendingForPost(ctx, request.UserID, request.PostSlug); findErr == nil && pending != nil {
				return StartResult{ExperimentID: pending.ID, JobID: pending.JobID}, &JobAlreadyInProgressError{ActiveID: pending.JobID}
			}
		}
		return StartResult{}, err
	})
}

// createQueued is every comparison start's tail: the comparison is created with its candidates
// on shuffled display sides, its one job is queued, and the two are linked. A comparison whose
// job cannot be queued is deleted again, so no row waits on a job that never existed. refused
// answers a create the store refuses, for a start that has its own answer to give; nil passes the
// refusal through.
func (s *Service) createQueued(ctx context.Context, found Experiment, refs []ModelRef, models []Model, job JobRequest, refused func(error) (StartResult, error)) (StartResult, error) {
	sides, err := shuffledSides(len(refs))
	if err != nil {
		return StartResult{}, fmt.Errorf("assign candidate sides: %w", err)
	}
	found.Candidates = make([]Candidate, 0, len(refs))
	for i, ref := range refs {
		found.Candidates = append(found.Candidates, Candidate{ID: s.newID(), ExperimentID: found.ID, Model: ref, ModelLabel: models[i].Label, DisplaySide: sides[i], Status: CandidatePending})
	}
	if err := s.runs.Create(ctx, found); err != nil {
		if refused != nil {
			return refused(err)
		}
		return StartResult{}, err
	}
	jobID, err := s.jobs.EnqueueExperiment(ctx, job)
	if err != nil {
		_ = s.runs.Delete(ctx, found.ID)
		return StartResult{}, err
	}
	if err := s.runs.SetJob(ctx, found.ID, found.UserID, jobID); err != nil {
		return StartResult{}, fmt.Errorf("link experiment job: %w", err)
	}
	return StartResult{ExperimentID: found.ID, JobID: jobID}, nil
}

func (s *Service) Get(ctx context.Context, userID, id string) (Experiment, error) {
	return s.owned(ctx, userID, id)
}

// List is the account's comparisons newest first, narrowed to one stage and one source when
// given: the write history reads post-sourced comparisons, 말투 반영 voice-sourced ones (MODEL-67).
func (s *Service) List(ctx context.Context, userID string, stage Stage, source Source) ([]Experiment, error) {
	if stage != "" {
		if _, err := ParseStage(string(stage)); err != nil {
			return nil, err
		}
	}
	if _, err := ParseSource(string(source)); err != nil {
		return nil, err
	}
	return s.runs.List(ctx, userID, stage, source)
}

// StartVoiceReflection is 말투 반영 비교's start (MODEL-31, MODEL-67): a named, owned, active and
// made voice, one of its answered prompts and two different write models — both reading images
// for a photo prompt. The voice context freezes the snapshot; the comparison and its one job,
// which holds both candidates' calls under one admission, exist before any provider call. The
// job names no voice, so deleting the voice is never blocked by it (VOICE-13).
func (s *Service) StartVoiceReflection(ctx context.Context, request ReflectionStartRequest) (StartResult, error) {
	if request.VoiceID == "" {
		return StartResult{}, ErrVoiceRequired
	}
	refs, reviewMode, err := startCandidates(request.ModelA, request.ModelB, request.Candidates, OriginLab)
	if err != nil {
		return StartResult{}, err
	}
	models, err := s.resolveCandidates(StageWrite, refs)
	if err != nil {
		return StartResult{}, err
	}
	if s.reflection == nil {
		return StartResult{}, errors.New("experiment: voice reflection is not configured")
	}
	if err := s.requireActiveVoice(ctx, request.UserID, request.VoiceID); err != nil {
		return StartResult{}, err
	}
	input, err := s.reflection.Snapshot(ctx, request.UserID, request.VoiceID, request.PromptKey)
	if err != nil {
		return StartResult{}, err
	}
	if input.Photo {
		for _, model := range models {
			if !model.Vision {
				return StartResult{}, ErrPhotoUnsupported
			}
		}
	}
	korean := LanguageKorean
	frozen, hash, err := FreezeSnapshot(Snapshot{Content: input.Content, PromptVersion: input.PromptVersion, VoiceID: request.VoiceID, TargetLanguage: &korean})
	if err != nil {
		return StartResult{}, err
	}
	found := Experiment{
		ID: s.newID(), UserID: request.UserID, VoiceID: request.VoiceID, Source: SourceVoice,
		VoicePromptKey: input.PromptKey, VoiceMaterialID: input.MaterialID, TargetLanguage: cloneLanguage(frozen.TargetLanguage),
		Stage: StageWrite, Origin: OriginLab, ReviewMode: reviewMode, Status: StatusQueued, InputSnapshot: frozen.Content, InputHash: hash,
		PromptVersion: frozen.PromptVersion, CreatedAt: s.now(),
	}
	return s.createQueued(ctx, found, refs, models, JobRequest{
		UserID: request.UserID, ExperimentID: found.ID, Stage: StageWrite,
		TargetLanguage: cloneLanguage(found.TargetLanguage), Models: refsToStrings(refs),
	}, nil)
}

// ReflectionPromptText is a voice-sourced comparison's prompt, as its history row names it; it
// is product copy and outlives the purged snapshot.
func (s *Service) ReflectionPromptText(found Experiment) string {
	if found.Source != SourceVoice || s.reflection == nil {
		return ""
	}
	return s.reflection.PromptText(found.VoicePromptKey)
}

// ReflectionDetail is what a 말투 반영 비교's review reads beside its pieces (MODEL-67): the prompt,
// the owner's answer while the snapshot keeps it, and each delivered piece measured against the
// voice's current analysis. Nothing about it reveals a candidate's identity (MODEL-32). Every
// piece is measured in one comparison call, so a poll reads the voice and its analysis once
// however many candidates delivered.
func (s *Service) ReflectionDetail(ctx context.Context, found Experiment) (ReflectionDetail, error) {
	if found.Source != SourceVoice || s.reflection == nil {
		return ReflectionDetail{}, nil
	}
	detail := ReflectionDetail{PromptText: s.reflection.PromptText(found.VoicePromptKey), Comparisons: map[string][]ItemComparison{}}
	if len(found.InputSnapshot) > 0 {
		answer, err := s.reflection.Answer(found.InputSnapshot)
		if err != nil {
			return ReflectionDetail{}, err
		}
		detail.Answer = answer
	}
	var delivered []string
	var pieces []string
	for _, candidate := range found.Candidates {
		if candidate.Status != CandidateSucceeded || len(candidate.Output) == 0 {
			continue
		}
		delivered = append(delivered, candidate.ID)
		pieces = append(pieces, string(candidate.Output))
	}
	if len(pieces) == 0 {
		return detail, nil
	}
	comparisons, err := s.reflection.Compare(ctx, found.UserID, found.VoiceID, pieces)
	if err != nil {
		return ReflectionDetail{}, err
	}
	if len(comparisons) != len(pieces) {
		return ReflectionDetail{}, fmt.Errorf("voice comparison answered %d of %d pieces", len(comparisons), len(pieces))
	}
	for i, candidateID := range delivered {
		detail.Comparisons[candidateID] = comparisons[i]
	}
	return detail, nil
}

func (s *Service) PendingForPost(ctx context.Context, userID, postSlug string) (*Experiment, error) {
	return s.runs.PendingForPost(ctx, userID, postSlug)
}

// BlockingWriteForPost is the id of the editor write comparison that holds the post's
// generation and revision, or empty (GEN-23, GEN-38). It is narrower than PendingForPost: a
// lab comparison is a reading of the post (MODEL-66) and a failed one has nothing to apply,
// so neither holds the post, although both stay the post's unresolved comparison until
// resolved (MODEL-34).
func (s *Service) BlockingWriteForPost(ctx context.Context, userID, postSlug string) (string, error) {
	return s.runs.BlockingWriteForPost(ctx, userID, postSlug)
}

func (s *Service) PurgePost(ctx context.Context, userID, postSlug string) error {
	return s.purge.PurgePost(ctx, userID, postSlug)
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
		if candidate.Status != CandidateFailed {
			continue
		}
		// The snapshot is the retry boundary: a candidate that failed on a photo the snapshot
		// no longer has is a new comparison, never a retry (MODEL-30).
		if candidate.Failure != nil && candidate.Failure.Reason == FailureReasonSnapshotUnavailable {
			return StartResult{}, ErrSnapshotUnavailable
		}
		if _, err := s.resolveForStage(found.Stage, candidate.Model); err != nil {
			return StartResult{}, ErrRetryModelUnavailable
		}
	}
	// An editor retry re-prepares into the post, so it asks first; a lab retry writes nothing
	// to the post and stays open whatever its status.
	if found.AppliesOnVerdict() {
		if err := s.allowPostWrite(ctx, found); err != nil {
			return StartResult{}, err
		}
	}
	// A retried observe candidate makes every call its first run made, so the hold counts them
	// again over the same snapshot.
	observeCalls := 0
	if found.Stage == StageObserve {
		if observeCalls, err = s.runner.ObserveCalls(found.Stage, found.InputSnapshot); err != nil {
			return StartResult{}, fmt.Errorf("count observe calls: %w", err)
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
		TargetLanguage: cloneLanguage(found.TargetLanguage), Models: candidateModels(found), ObserveCalls: observeCalls,
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

// startOrigin freezes where a comparison was started. Observe can only be started in the
// lab; a write comparison honours its caller, and an absent value means the
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
	if found.ReviewMode == ReviewCandidateRanking {
		return Experiment{}, ErrInvalidState
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
			return s.apply(ctx, found)
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
				return s.apply(ctx, current)
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
		return s.apply(ctx, found)
	}
	return found, nil
}

func (s *Service) Dismiss(ctx context.Context, userID, id string) (Experiment, error) {
	found, err := s.owned(ctx, userID, id)
	if err != nil {
		return Experiment{}, err
	}
	if found.ReviewMode == ReviewCandidateRanking {
		return Experiment{}, ErrInvalidState
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

// CompleteReview records the complete ordering in one database transaction. A tied
// position is repeated; the next distinct position is the next integer.
func (s *Service) CompleteReview(ctx context.Context, userID, id string, offered []CandidateRank, skip bool) (Experiment, error) {
	found, err := s.owned(ctx, userID, id)
	if err != nil {
		return Experiment{}, err
	}
	if found.ReviewMode != ReviewCandidateRanking {
		return Experiment{}, ErrInvalidState
	}
	ranks, err := normalizeRanks(found, offered, skip)
	if err != nil {
		return Experiment{}, err
	}
	if found.Status == StatusCompleted {
		if sameRanks(found, ranks) {
			return found, nil
		}
		return Experiment{}, ErrInvalidState
	}
	if found.Status != StatusReview && found.Status != StatusPartial && found.Status != StatusFailed {
		return Experiment{}, ErrInvalidState
	}
	now := s.now()
	changed, err := s.outcome.CompleteRanking(ctx, id, userID, ranks, now, now.Add(s.retention))
	if err != nil {
		return Experiment{}, err
	}
	if !changed {
		current, loadErr := s.owned(ctx, userID, id)
		if loadErr == nil && current.Status == StatusCompleted && sameRanks(current, ranks) {
			return current, nil
		}
		return Experiment{}, ErrInvalidState
	}
	return s.owned(ctx, userID, id)
}

func normalizeRanks(found Experiment, offered []CandidateRank, skip bool) ([]CandidateRank, error) {
	if skip {
		if len(offered) != 0 {
			return nil, ErrRanksInvalid
		}
		return nil, nil
	}
	successful := map[string]bool{}
	for _, candidate := range found.Candidates {
		if candidate.Status == CandidateSucceeded {
			successful[candidate.ID] = true
		}
	}
	if len(successful) < 2 || len(offered) != len(successful) {
		return nil, ErrRanksInvalid
	}
	seen := map[string]bool{}
	positions := map[int]bool{}
	badges := make([]CandidateBadges, 0, len(offered))
	for _, rank := range offered {
		if !successful[rank.CandidateID] || seen[rank.CandidateID] || rank.Rank < 1 || rank.Rank > len(successful) {
			return nil, ErrRanksInvalid
		}
		seen[rank.CandidateID] = true
		positions[rank.Rank] = true
		badges = append(badges, CandidateBadges{CandidateID: rank.CandidateID, Badges: rank.Badges, OtherNote: rank.OtherNote})
	}
	for i := 1; i <= len(positions); i++ {
		if !positions[i] {
			return nil, ErrRanksInvalid
		}
	}
	normalized, err := NormalizeBadges(found, badges)
	if err != nil {
		return nil, err
	}
	byID := map[string]CandidateBadges{}
	for _, badge := range normalized {
		byID[badge.CandidateID] = badge
	}
	out := make([]CandidateRank, 0, len(offered))
	for _, rank := range offered {
		item := byID[rank.CandidateID]
		out = append(out, CandidateRank{CandidateID: rank.CandidateID, Rank: rank.Rank, Badges: item.Badges, OtherNote: item.OtherNote})
	}
	return out, nil
}

func sameRanks(found Experiment, ranks []CandidateRank) bool {
	count := 0
	for _, candidate := range found.Candidates {
		if candidate.Rank > 0 {
			count++
		}
	}
	if count != len(ranks) {
		return false
	}
	byID := map[string]CandidateRank{}
	for _, rank := range ranks {
		byID[rank.CandidateID] = rank
	}
	for _, candidate := range found.Candidates {
		expected, ok := byID[candidate.ID]
		if !ok {
			if candidate.Rank != 0 {
				return false
			}
			continue
		}
		// Private notes leave with the frozen input. After that purge the durable rank
		// and badge ids still identify the same completion; a replay changes nothing.
		if candidate.Rank != expected.Rank || (len(found.InputSnapshot) > 0 && candidate.OtherNote != expected.OtherNote) {
			return false
		}
		actual := append([]Badge(nil), candidate.Badges...)
		want := append([]Badge(nil), expected.Badges...)
		slices.Sort(actual)
		slices.Sort(want)
		if !slices.Equal(actual, want) {
			return false
		}
	}
	return true
}

func (s *Service) ApplyCandidateOutput(ctx context.Context, userID, id, candidateID string, adopt bool) (Experiment, error) {
	found, err := s.owned(ctx, userID, id)
	if err != nil {
		return Experiment{}, err
	}
	if found.ReviewMode != ReviewCandidateRanking || found.Status != StatusCompleted ||
		found.Source == SourceVoice || (found.Origin == OriginEditor && found.Stage != StageWrite) {
		return Experiment{}, ErrInvalidState
	}
	candidate := found.Candidate(candidateID)
	if candidate == nil || candidate.Status != CandidateSucceeded {
		return Experiment{}, ErrCandidateNotFound
	}
	if found.AppliedCandidateID != "" && found.AppliedCandidateID != candidateID {
		return Experiment{}, ErrInvalidState
	}
	if adopt && found.AdoptedCandidateID != "" && found.AdoptedCandidateID != candidateID {
		return Experiment{}, ErrInvalidState
	}
	if found.AppliedAt == nil {
		if err := s.allowPostWrite(ctx, found); err != nil {
			return Experiment{}, err
		}
	}
	if err := s.outcome.MarkCandidateApply(ctx, id, userID, candidateID, adopt); err != nil {
		return Experiment{}, err
	}
	found, err = s.owned(ctx, userID, id)
	if err != nil {
		return Experiment{}, err
	}
	if found.AppliedAt == nil {
		found, err = s.apply(ctx, found)
		if err != nil || found.AppliedAt == nil {
			return found, err
		}
	}
	if adopt && found.AdoptedAt == nil {
		_, _, err = s.AdoptCandidateModel(ctx, userID, id, candidateID)
		if err != nil {
			return s.owned(ctx, userID, id)
		}
		return s.owned(ctx, userID, id)
	}
	return found, nil
}

func (s *Service) AdoptCandidateModel(ctx context.Context, userID, id, candidateID string) (ModelRef, Stage, error) {
	s.adoptMu.Lock()
	defer s.adoptMu.Unlock()
	found, err := s.owned(ctx, userID, id)
	if err != nil {
		return ModelRef{}, "", err
	}
	if found.ReviewMode != ReviewCandidateRanking || found.Status != StatusCompleted {
		return ModelRef{}, "", ErrInvalidState
	}
	candidate := found.Candidate(candidateID)
	if candidate == nil || candidate.Status != CandidateSucceeded {
		return ModelRef{}, "", ErrCandidateNotFound
	}
	if found.AdoptedCandidateID != "" && found.AdoptedCandidateID != candidateID {
		return ModelRef{}, "", ErrInvalidState
	}
	if found.AdoptedAt != nil {
		return candidate.Model, found.Stage, nil
	}
	if found.Origin == OriginEditor && found.AppliedAt == nil {
		return ModelRef{}, "", ErrInvalidState
	}
	if found.Source == SourceVoice {
		if err := s.requireActiveVoice(ctx, userID, found.VoiceID); err != nil {
			return ModelRef{}, "", err
		}
	}
	if _, err := s.resolveForStage(found.Stage, candidate.Model); err != nil {
		return ModelRef{}, "", err
	}
	if err := s.outcome.MarkCandidateAdopt(ctx, id, userID, candidateID); err != nil {
		return ModelRef{}, "", err
	}
	active, selected, err := s.catalog.Active(ctx, userID, found.Stage)
	if err == nil && !(selected && active == candidate.Model) {
		err = s.catalog.Adopt(ctx, userID, found.Stage, candidate.Model)
	}
	if err != nil {
		if storeErr := s.outcome.SetAdoptionFailure(ctx, id, userID, normalizeFailure(err)); storeErr != nil {
			return ModelRef{}, "", storeErr
		}
		return ModelRef{}, "", err
	}
	if err := s.outcome.SetAdopted(ctx, id, userID, s.now()); err != nil {
		return ModelRef{}, "", err
	}
	return candidate.Model, found.Stage, nil
}

func (s *Service) ApplyWinner(ctx context.Context, userID, id string) (Experiment, error) {
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
	return s.apply(ctx, found)
}

// allowPostWrite decides whether a comparison's result may land in its post as the post is
// now (MODEL-37). A draft or a post in revision takes either origin's result, and a published post takes neither, since it is locked
// (POST-74). Any other status takes the editor's, whose result reopens a finalized post as
// saving content does (POST-13), and refuses the lab's: its gate is an allowlist.
func (s *Service) allowPostWrite(ctx context.Context, found Experiment) error {
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

// AdoptWinner is the decided result's active-model follow-up (MODEL-36). The debt is marked
// before the catalog call and adopted_at after it, so a reload knows the adoption happened
// and a retry checks the catalog's active selection before it adopts anything.
func (s *Service) AdoptWinner(ctx context.Context, userID, id string) (ModelRef, Stage, error) {
	s.adoptMu.Lock()
	defer s.adoptMu.Unlock()
	found, err := s.owned(ctx, userID, id)
	if err != nil {
		return ModelRef{}, "", err
	}
	winner := found.Winner()
	if found.Status != StatusDecided || winner == nil {
		return ModelRef{}, "", ErrInvalidState
	}
	if found.AdoptedAt != nil {
		return winner.Model, found.Stage, nil
	}
	if _, err := s.resolveForStage(found.Stage, winner.Model); err != nil {
		return ModelRef{}, "", err
	}
	if !found.AdoptionRequested {
		if err := s.outcome.SetAdoptionRequested(ctx, found.ID, userID); err != nil {
			return ModelRef{}, "", err
		}
	}
	active, selected, err := s.catalog.Active(ctx, userID, found.Stage)
	if err == nil && !(selected && active == winner.Model) {
		err = s.catalog.Adopt(ctx, userID, found.Stage, winner.Model)
	}
	if err != nil {
		if storeErr := s.outcome.SetAdoptionFailure(ctx, found.ID, userID, normalizeFailure(err)); storeErr != nil {
			return ModelRef{}, "", storeErr
		}
		return ModelRef{}, "", err
	}
	if err := s.outcome.SetAdopted(ctx, found.ID, userID, s.now()); err != nil {
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
	if found.ReviewMode == ReviewCandidateRanking {
		return Experiment{}, ErrInvalidState
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

// Leaderboard replays completed rankings and eligible historical pairwise outcomes of one
// (scope, stage, window) from 1500. The window is rolling from request time (MODEL-38).
func (s *Service) Leaderboard(ctx context.Context, userID string, stage Stage, window Window, scope Scope) ([]LeaderboardEntry, error) {
	entries, err := s.leaderboardEntries(ctx, userID, stage, window, scope)
	if err != nil {
		return nil, err
	}
	active, hasActive, err := s.catalog.Active(ctx, userID, stage)
	if err != nil {
		return nil, err
	}
	recommended, err := s.catalog.Recommended(ctx, stage)
	if err != nil {
		return nil, err
	}
	for i := range entries {
		entries[i].Active = hasActive && entries[i].Model == active
		entries[i].Recommended = slices.Contains(recommended, entries[i].Model)
		_, present := s.catalog.Resolve(entries[i].Model)
		entries[i].Disappeared = !present
	}
	return entries, nil
}

// leaderboardEntries replays the same counted events without account-specific display
// decoration, so the operator's all-account cost read needs no synthetic user id.
func (s *Service) leaderboardEntries(ctx context.Context, userID string, stage Stage, window Window, scope Scope) ([]LeaderboardEntry, error) {
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
	// Only a counted outcome's calls are accounted: a failed, unpaired or single-delivery run
	// puts nothing on the board, its calls, latency and cost included.
	var counted []Candidate
	for _, found := range decided {
		pair := byExperiment[found.ID]
		if found.ReviewMode == ReviewCandidateRanking {
			ranked := make([]Candidate, 0, len(pair))
			for _, candidate := range pair {
				if candidate.Status == CandidateSucceeded && candidate.Rank > 0 {
					ranked = append(ranked, candidate)
				}
			}
			if len(ranked) < 2 {
				continue
			}
			slices.SortFunc(ranked, func(a, b Candidate) int { return sideOrder(a.DisplaySide) - sideOrder(b.DisplaySide) })
			participants := make([]RankedParticipant, 0, len(ranked))
			for _, candidate := range ranked {
				participants = append(participants, RankedParticipant{Model: candidate.Model, Rank: candidate.Rank, DisplaySide: candidate.DisplaySide})
			}
			matches = append(matches, Match{Ranked: participants})
			counted = append(counted, ranked...)
			continue
		}
		if found.Status == StatusDismissed {
			if len(pair) != 2 || pair[0].Status != CandidateSucceeded || pair[1].Status != CandidateSucceeded {
				continue
			}
			matches = append(matches, Match{Dismissed: []ModelRef{pair[0].Model, pair[1].Model}})
			counted = append(counted, pair...)
			continue
		}
		var winner, loser *Candidate
		for i := range pair {
			candidate := &pair[i]
			if candidate.ID == found.WinnerCandidateID {
				winner = candidate
			} else {
				loser = candidate
			}
		}
		if winner != nil && loser != nil {
			matches = append(matches, Match{Winner: winner.Model, Loser: loser.Model})
			counted = append(counted, pair...)
		}
	}
	return BuildLeaderboard(matches, counted, labels, tallies), nil
}

// ComparisonCosts uses the same counted all-account events and usage samples as the
// public leaderboard, then exposes only the cost totals through AdminService.
func (s *Service) ComparisonCosts(ctx context.Context, stage Stage, window Window) ([]ComparisonCostRow, error) {
	entries, err := s.leaderboardEntries(ctx, "", stage, window, ScopeAll)
	if err != nil {
		return nil, err
	}
	rows := make([]ComparisonCostRow, 0, len(entries))
	for _, entry := range entries {
		rows = append(rows, ComparisonCostRow{
			Model: entry.Model, ModelLabel: entry.ModelLabel,
			EvaluatedComparisons: entry.EvaluatedComparisons,
			TotalCostMicrousd:    entry.TotalCostMicrousd, CostQuality: entry.CostQuality,
		})
	}
	slices.SortFunc(rows, func(a, b ComparisonCostRow) int {
		if a.TotalCostMicrousd > b.TotalCostMicrousd {
			return -1
		}
		if a.TotalCostMicrousd < b.TotalCostMicrousd {
			return 1
		}
		return cmp.Compare(a.Model.String(), b.Model.String())
	})
	return rows, nil
}

func (s *Service) apply(ctx context.Context, found Experiment) (Experiment, error) {
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
	if found.ReviewMode == ReviewCandidateRanking {
		winner = found.Candidate(found.AppliedCandidateID)
	}
	if winner == nil || len(winner.Output) == 0 {
		return Experiment{}, ErrInvalidState
	}
	if err := s.runner.ApplyWinner(ctx, found, *winner); err != nil {
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
	// Membership in the stage's purpose (MODEL-16) replaced the observe-needs-vision check:
	// the registration gate already required vision, and a deregistered model must be as
	// unusable for an experiment as it is in the picker.
	if !ok || !model.Enabled || !slices.Contains(model.Stages, string(stage)) {
		return Model{}, fmt.Errorf("%w: %s", ErrModelRequired, ref)
	}
	return model, nil
}

func startCandidates(a, b ModelRef, full []ModelRef, origin Origin) ([]ModelRef, ReviewMode, error) {
	mode := ReviewPairwise
	refs := []ModelRef{a, b}
	if len(full) > 0 {
		if a != (ModelRef{}) || b != (ModelRef{}) {
			return nil, "", ErrMixedCandidateForms
		}
		mode = ReviewCandidateRanking
		refs = full
	}
	if len(refs) < 2 || len(refs) > 5 || (origin == OriginEditor && len(refs) != 2) {
		return nil, "", ErrCandidateCount
	}
	seen := make(map[ModelRef]bool, len(refs))
	for _, ref := range refs {
		if seen[ref] {
			return nil, "", ErrDuplicateCandidates
		}
		seen[ref] = true
	}
	return refs, mode, nil
}

func (s *Service) resolveCandidates(stage Stage, refs []ModelRef) ([]Model, error) {
	models := make([]Model, 0, len(refs))
	for _, ref := range refs {
		model, err := s.resolveForStage(stage, ref)
		if err != nil {
			return nil, err
		}
		models = append(models, model)
	}
	return models, nil
}

func shuffledSides(count int) ([]DisplaySide, error) {
	sides := append([]DisplaySide(nil), []DisplaySide{SideLeft, SideRight, SideC, SideD, SideE}[:count]...)
	for i := count - 1; i > 0; i-- {
		position, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return nil, err
		}
		j := int(position.Int64())
		sides[i], sides[j] = sides[j], sides[i]
	}
	return sides, nil
}

func newID() string {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		panic("experiment: cannot create id: " + err.Error())
	}
	return hex.EncodeToString(value)
}

// runCandidates runs the pending candidates, at most s.concurrency at once: a ranked comparison
// has up to five, and five parallel calls to one provider invite the rate limit a retry would
// pay for again.
func (s *Service) runCandidates(ctx context.Context, found Experiment, progress Progress) error {
	var wg sync.WaitGroup
	var progressMu sync.Mutex
	slots := make(chan struct{}, s.concurrency)
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
		slots <- struct{}{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-slots }()
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

// The shared preparation call is priced at the observe stage even if its ref also writes.
func writeObserveModel(request StartRequest) string {
	if request.Stage == StageWrite && request.ObserveModel.ProviderID != "" {
		return request.ObserveModel.String()
	}
	return ""
}

func refsToStrings(refs []ModelRef) []string {
	models := make([]string, 0, len(refs))
	for _, ref := range refs {
		models = append(models, ref.String())
	}
	return models
}

// candidateModels are the refs a retry will re-run, in candidate order.
func candidateModels(found Experiment) []string {
	models := make([]string, 0, len(found.Candidates))
	for _, candidate := range found.Candidates {
		if candidate.Status == CandidateFailed {
			models = append(models, candidate.Model.String())
		}
	}
	return models
}
