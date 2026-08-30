package publishing

import (
	"context"
	"time"
)

type Clock interface{ Now() time.Time }

type Tokens interface {
	NewID() (string, error)
	NewSecret(bytes int) (string, error)
}

type PostSnapshots interface {
	PostIdentity(ctx context.Context, userID, postSlug string) (time.Time, error)
	PublishingSnapshot(ctx context.Context, userID, postSlug string) (PostSnapshot, error)
}

// ObjectStaging is implemented by the outer storage adapter. Copy is server-side:
// no JPEG bytes cross this port or enter the API process.
type ObjectStaging interface {
	Copy(ctx context.Context, sourceKey, targetKey string) (int64, error)
	SignGet(ctx context.Context, key string, ttl time.Duration) (string, error)
	Delete(ctx context.Context, key string) error
	ListStaged(ctx context.Context, prefix string) ([]StagedObject, error)
}

type StagedObject struct {
	Key          string
	LastModified time.Time
}

type Store interface {
	CreatePairing(ctx context.Context, codeHash, userID, label string, expiresAt, createdAt time.Time, maxPending int) error
	Enroll(ctx context.Context, codeHash, tokenHash, agentID, browserLabel string, now time.Time) (Agent, error)
	AgentByTokenHash(ctx context.Context, tokenHash string) (Agent, error)
	TouchAgent(ctx context.Context, userID, agentID string, now time.Time) error
	OwnedAgent(ctx context.Context, userID, agentID string) (Agent, error)
	ListAgents(ctx context.Context, userID string) ([]Agent, error)
	UpdateAgent(ctx context.Context, userID, agentID, label, categoryID string, visibility Visibility, now time.Time) (Agent, error)
	SyncAgent(ctx context.Context, userID, agentID string, update ProfileUpdate, now time.Time) (Agent, error)
	RevokeAgent(ctx context.Context, userID, agentID string, now time.Time) error
	ReserveJobID(ctx context.Context, userID, jobID string, now time.Time) error
	ReleaseJobID(ctx context.Context, userID, jobID string) error

	// CreateJob runs guard after reserving the serialized writer and revalidating the
	// selected agent, then inserts the job and assets in that same transaction. This
	// gives Start one linearization point without letting this context read post tables.
	CreateJob(ctx context.Context, job Job, assets []Asset, guard func(context.Context) error) error
	RetryAttentionJob(ctx context.Context, userID, jobID string, now time.Time) (Job, error)
	ListRetryableJobs(ctx context.Context, userID string) ([]Job, error)
	OwnedJob(ctx context.Context, userID, jobID string) (Job, error)
	LatestJobForPost(ctx context.Context, userID, postSlug string, postCreatedAt time.Time) (Job, error)
	LatestJobForDeletedPost(ctx context.Context, userID, postSlug string) (Job, error)
	ClaimJob(ctx context.Context, agent Agent, leaseHash string, expiresAt, now time.Time) (Job, error)
	RenewLease(ctx context.Context, agent Agent, jobID, leaseHash string, expiresAt, now time.Time) error
	UpdateProgress(ctx context.Context, agent Agent, jobID, leaseHash string, currentStage Stage, currentSeq int64, nextStage Stage, nextSeq int64, now time.Time) (Job, error)
	Complete(ctx context.Context, agent Agent, jobID, leaseHash string, seq int64, url string, now time.Time) (Job, error)
	Fail(ctx context.Context, agent Agent, jobID, leaseHash string, seq int64, status Status, code, message string, now time.Time) (Job, error)
	Cancel(ctx context.Context, userID, jobID string, now time.Time) (Job, error)
	RequeueExpired(ctx context.Context, now time.Time) (requeued, unknown int64, err error)
	Assets(ctx context.Context, jobID string) ([]Asset, error)
	DeleteAssets(ctx context.Context, jobID string) error
	LiveStagedKeys(ctx context.Context) (map[string]struct{}, error)
	TerminalJobsWithAssets(ctx context.Context) ([]string, error)
}
