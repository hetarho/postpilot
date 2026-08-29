package experiment

import (
	"context"
	"time"
)

type Store interface {
	Create(ctx context.Context, found Experiment) error
	Delete(ctx context.Context, id string) error
	Get(ctx context.Context, id string) (Experiment, error)
	List(ctx context.Context, userID string, stage Stage) ([]Experiment, error)
	PendingForPost(ctx context.Context, userID, postSlug string) (*Experiment, error)
	SetJob(ctx context.Context, id, userID, jobID string) error
	SetSnapshot(ctx context.Context, id string, snapshot Snapshot, hash string) error
	SetStatus(ctx context.Context, id string, status Status, finishedAt *time.Time) error
	StartCandidate(ctx context.Context, experimentID, candidateID string, now time.Time) error
	CompleteCandidate(ctx context.Context, candidate Candidate) error
	FailUnfinished(ctx context.Context, experimentID, reason string, now time.Time) error
	RecoverInterrupted(ctx context.Context, reason string, now time.Time) (int64, error)
	ListQueued(ctx context.Context) ([]string, error)
	ResetFailedCandidates(ctx context.Context, experimentID string) (int64, error)
	RestoreFailedCandidates(ctx context.Context, experimentID string, candidates []Candidate) error
	Decide(ctx context.Context, id, userID, candidateID string, status Status, outcome Outcome, decidedAt, expiresAt time.Time) (bool, error)
	SetApplyError(ctx context.Context, id, userID, message string) error
	SetApplied(ctx context.Context, id, userID string, now time.Time) error
	LeaderboardData(ctx context.Context, userID string, stage Stage) ([]Experiment, []Candidate, error)
	PurgeExpired(ctx context.Context, before time.Time) (int64, error)
	PurgePost(ctx context.Context, userID, postSlug string) error
}

type Catalog interface {
	Resolve(ref ModelRef) (Model, bool)
	Adopt(ctx context.Context, userID string, stage Stage, ref ModelRef) error
	Active(ctx context.Context, userID string, stage Stage) (ModelRef, bool, error)
	Recommended(stage Stage, ref ModelRef) bool
}

type Jobs interface {
	EnqueueExperiment(ctx context.Context, request JobRequest) (string, error)
	HasRunnableExperiment(ctx context.Context, experimentID string) (bool, error)
}

type Runner interface {
	Snapshot(ctx context.Context, request StartRequest) (Snapshot, error)
	PrepareWrite(ctx context.Context, found Experiment, progress Progress) (Snapshot, error)
	RunCandidate(ctx context.Context, found Experiment, candidate Candidate, progress Progress) (CandidateResult, error)
	ApplyWinner(ctx context.Context, found Experiment, candidate Candidate, confirmStyleguide bool) error
}

type Progress func(stage string, done, total int)
