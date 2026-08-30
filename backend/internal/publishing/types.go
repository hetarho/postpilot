// Package publishing owns paired publishing agents and the durable external-side-effect
// job. It deliberately does not reuse generation_jobs: a remote browser click needs a
// lease, an irreversible commit fence, and an honest ambiguous outcome.
package publishing

import (
	"errors"
	"time"
)

const PlatformNaverBlog = "naver_blog"

type Visibility string

const (
	VisibilityPublic  Visibility = "public"
	VisibilityPrivate Visibility = "private"
)

type Status string

const (
	StatusQueued         Status = "queued"
	StatusRunning        Status = "running"
	StatusPublished      Status = "published"
	StatusFailed         Status = "failed"
	StatusNeedsAttention Status = "needs_attention"
	StatusOutcomeUnknown Status = "outcome_unknown"
	StatusCanceled       Status = "canceled"
)

type Stage string

const (
	StageQueued          Stage = "queued"
	StageClaimed         Stage = "claimed"
	StagePreparing       Stage = "preparing"
	StageOpeningEditor   Stage = "opening_editor"
	StageFillingContent  Stage = "filling_content"
	StageUploadingPhotos Stage = "uploading_photos"
	StageCommitting      Stage = "committing"
	StageVerifying       Stage = "verifying"
	StagePublished       Stage = "published"
)

type FailureKind string

const (
	FailureSafe            FailureKind = "safe"
	FailureLoginExpired    FailureKind = "login_expired"
	FailureCaptcha         FailureKind = "captcha"
	FailureTwoFactor       FailureKind = "two_factor"
	FailureAccountMismatch FailureKind = "account_mismatch"
	FailureBrowserLost     FailureKind = "browser_lost"
	FailureEditorChanged   FailureKind = "editor_changed"
	FailureAssetMissing    FailureKind = "asset_missing"
)

var (
	ErrNotFound            = errors.New("publishing record not found")
	ErrForbidden           = errors.New("publishing record belongs to another account")
	ErrInvalid             = errors.New("invalid publishing request")
	ErrPairingLimit        = errors.New("too many pending pairings")
	ErrPairingInvalid      = errors.New("pairing code is invalid or expired")
	ErrAgentRevoked        = errors.New("publishing agent is revoked")
	ErrAgentNotReady       = errors.New("publishing agent is not ready")
	ErrCategoryNotFound    = errors.New("publishing category is not available")
	ErrPostNotFinalized    = errors.New("post is not finalized")
	ErrStaleRevision       = errors.New("post content revision is stale")
	ErrAlreadyPublishing   = errors.New("post already has a live or successful publication")
	ErrLeaseInvalid        = errors.New("publish lease is invalid or stale")
	ErrTransition          = errors.New("publish transition is not allowed")
	ErrCommitFence         = errors.New("publish job crossed the commit fence")
	ErrPublishedURLInvalid = errors.New("published URL does not belong to the paired blog")
)

type Category struct {
	ID   string
	Name string
}

type Agent struct {
	ID                   string
	UserID               string
	Label                string
	Platform             string
	PlatformAccountID    string
	PlatformAccountLabel string
	BrowserLabel         string
	Categories           []Category
	DefaultCategoryID    string
	DefaultVisibility    Visibility
	CompatibilityReady   bool
	HermesVersion        string
	LastSeenAt           *time.Time
	RevokedAt            *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

func (a Agent) Ready() bool {
	return a.RevokedAt == nil && a.CompatibilityReady && a.PlatformAccountID != "" &&
		a.BrowserLabel != "" && a.HasCategory(a.DefaultCategoryID)
}

func (a Agent) HasCategory(id string) bool {
	_, ok := a.CategoryName(id)
	return ok
}

func (a Agent) CategoryName(id string) (string, bool) {
	if id == "" {
		return "", false
	}
	for _, category := range a.Categories {
		if category.ID == id {
			return category.Name, category.Name != ""
		}
	}
	return "", false
}

type BlockType string

const (
	BlockText    BlockType = "TEXT"
	BlockHeading BlockType = "HEADING"
	BlockImage   BlockType = "IMAGE"
	BlockQuote   BlockType = "QUOTE"
	BlockList    BlockType = "LIST"
)

type Block struct {
	Type    BlockType
	Content string
	Level   int32
	File    string
	Alt     string
	Caption string
	Items   []string
}

type Content struct {
	Title   string
	Summary string
	Tags    []string
	Blocks  []Block
}

type SnapshotImage struct {
	Filename string
	Key      string
	Bytes    int64
}

type PostSnapshot struct {
	PostSlug          string
	UserID            string
	CreatedAt         time.Time
	Content           Content
	ContentRevision   int64
	FinalizedRevision int64
	Images            []SnapshotImage
}

type Asset struct {
	JobID          string
	UserID         string
	Ordinal        int
	Filename       string
	SourceFilename string
	StagedKey      string
	Bytes          int64
	CreatedAt      time.Time
}

type Manifest struct {
	JobID                     string
	PostSlug                  string
	ContentRevision           int64
	Content                   Content
	Tags                      []string
	CategoryID                string
	CategoryName              string
	Visibility                Visibility
	ExpectedPlatformAccountID string
	Assets                    []Asset
}

type Job struct {
	ID              string
	UserID          string
	PostSlug        string
	PostCreatedAt   time.Time
	AgentID         string
	Platform        string
	Status          Status
	Stage           Stage
	ProgressSeq     int64
	Attempt         int64
	ContentRevision int64
	Manifest        *Manifest
	CategoryID      string
	Visibility      Visibility
	LeaseTokenHash  string
	LeaseExpiresAt  *time.Time
	ErrorCode       string
	ErrorMessage    string
	PlatformPostURL string
	CreatedAt       time.Time
	ClaimedAt       *time.Time
	CommittedAt     *time.Time
	PublishedAt     *time.Time
	UpdatedAt       time.Time
}

func (j Job) Terminal() bool {
	switch j.Status {
	case StatusPublished, StatusFailed, StatusOutcomeUnknown, StatusCanceled:
		return true
	default:
		return false
	}
}

type Claim struct {
	Job          Job
	LeaseToken   string
	LeaseExpires time.Time
	LeaseTTL     time.Duration
	Manifest     Manifest
	URLs         []string
}

type Pairing struct {
	DeviceCode string
	ExpiresAt  time.Time
}

type Enrollment struct {
	AgentID  string
	Token    string
	LeaseTTL time.Duration
}

type StartRequest struct {
	UserID                  string
	PostSlug                string
	ExpectedContentRevision int64
	AgentID                 string
	CategoryID              string
	Visibility              Visibility
}

type ProfileUpdate struct {
	PlatformAccountID    string
	PlatformAccountLabel string
	BrowserLabel         string
	Categories           []Category
	DefaultCategoryID    string
	DefaultVisibility    Visibility
	CompatibilityReady   bool
	HermesVersion        string
}
