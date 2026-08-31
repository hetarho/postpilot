package publishing

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"time"
)

type Config struct {
	PairingTTL             time.Duration
	MaxPendingPairings     int
	LeaseTTL               time.Duration
	AssetURLTTL            time.Duration
	AgentHeartbeatInterval time.Duration
}

type Service struct {
	store   Store
	posts   PostSnapshots
	staging ObjectStaging
	clock   Clock
	tokens  Tokens
	config  Config

	// Suppresses repeat last_seen_at writes: every agent procedure authenticates, and
	// the Mac polls several times per freshness window, so an unthrottled touch spends
	// the single SQLite writer on a display hint. Guarded because the agent interceptor
	// authenticates concurrently. RevokeAgent drops its entry, so the map stays bounded
	// by the agents currently enrolled and needs no sweeper.
	touchMu   sync.Mutex
	lastTouch map[string]time.Time
}

func NewService(store Store, posts PostSnapshots, staging ObjectStaging, cfg Config) *Service {
	return &Service{store: store, posts: posts, staging: staging, clock: realClock{}, tokens: cryptoTokens{}, config: cfg, lastTouch: map[string]time.Time{}}
}

func (s *Service) CreatePairing(ctx context.Context, userID, label string) (Pairing, error) {
	label = strings.TrimSpace(label)
	if userID == "" || label == "" || len([]rune(label)) > 80 {
		return Pairing{}, ErrInvalid
	}
	now := s.clock.Now()
	raw, err := s.tokens.NewSecret(8)
	if err != nil {
		return Pairing{}, fmt.Errorf("generate pairing code: %w", err)
	}
	// Groups make a pasted/read-aloud device code less error-prone; the server stores
	// only its hash and normalizes separators on enrollment. Hex avoids treating a
	// random base64 '-' as if it were the visual separator.
	digest := sha256.Sum256([]byte(raw))
	compactCode := strings.ToUpper(hex.EncodeToString(digest[:])[:12])
	code := compactCode[:4] + "-" + compactCode[4:8] + "-" + compactCode[8:]
	expires := now.Add(s.config.PairingTTL)
	if err := s.store.CreatePairing(ctx, hashCode(code), userID, label, expires, now, s.config.MaxPendingPairings); err != nil {
		if errors.Is(err, ErrPairingLimit) {
			return Pairing{}, err
		}
		return Pairing{}, fmt.Errorf("create pairing: %w", err)
	}
	return Pairing{DeviceCode: code, ExpiresAt: expires}, nil
}

func (s *Service) Enroll(ctx context.Context, deviceCode, browserLabel string) (Enrollment, error) {
	deviceCode = normalizeCode(deviceCode)
	browserLabel = strings.TrimSpace(browserLabel)
	if len(deviceCode) != 12 || browserLabel == "" {
		return Enrollment{}, ErrPairingInvalid
	}
	token, err := s.tokens.NewSecret(32)
	if err != nil {
		return Enrollment{}, err
	}
	agentID, err := s.tokens.NewID()
	if err != nil {
		return Enrollment{}, fmt.Errorf("generate agent id: %w", err)
	}
	_, err = s.store.Enroll(ctx, hashCode(deviceCode), hashToken(token), agentID, browserLabel, s.clock.Now())
	if err != nil {
		if errors.Is(err, ErrPairingInvalid) {
			return Enrollment{}, err
		}
		return Enrollment{}, fmt.Errorf("enroll agent: %w", err)
	}
	return Enrollment{AgentID: agentID, Token: token, LeaseTTL: s.config.LeaseTTL}, nil
}

func (s *Service) AuthenticateAgent(ctx context.Context, rawToken string) (Agent, error) {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return Agent{}, ErrAgentRevoked
	}
	agent, err := s.store.AgentByTokenHash(ctx, hashToken(rawToken))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return Agent{}, ErrAgentRevoked
		}
		return Agent{}, err
	}
	if agent.RevokedAt != nil {
		return Agent{}, ErrAgentRevoked
	}
	now := s.clock.Now()
	if err := s.touchAgent(ctx, agent, now); err != nil {
		return Agent{}, err
	}
	// Reported as seen even when the write was skipped: the agent did just call, this
	// struct is per-request context, and ListAgents reads the column independently.
	agent.LastSeenAt = &now
	agent.UpdatedAt = now
	return agent, nil
}

// touchAgent refreshes last_seen_at at most once per heartbeat interval. A storage
// failure is not the caller's problem — the token was already proven valid, so it
// degrades the freshness display rather than refusing a legitimate agent — but the store
// reports a revocation that landed since the token lookup as ErrAgentRevoked, and that
// one is a real refusal.
func (s *Service) touchAgent(ctx context.Context, agent Agent, now time.Time) error {
	s.touchMu.Lock()
	previous, seen := s.lastTouch[agent.ID]
	if seen && now.Sub(previous) < s.config.AgentHeartbeatInterval {
		s.touchMu.Unlock()
		return nil
	}
	// Claim the interval before releasing the lock. Calls that arrive together at the
	// boundary would otherwise all read the old stamp and all reach the writer, which is
	// the cost this throttle exists to remove. The lock is not held across the write, so
	// authentication never serializes behind the database.
	s.lastTouch[agent.ID] = now
	s.touchMu.Unlock()

	err := s.store.TouchAgent(ctx, agent.UserID, agent.ID, now)
	if err == nil {
		return nil
	}
	// Give the claim back so the next call retries instead of waiting out an interval
	// that was never actually refreshed. A newer claim has already superseded this one.
	s.touchMu.Lock()
	if current, ok := s.lastTouch[agent.ID]; ok && current.Equal(now) {
		if seen {
			s.lastTouch[agent.ID] = previous
		} else {
			delete(s.lastTouch, agent.ID)
		}
	}
	s.touchMu.Unlock()

	if errors.Is(err, ErrAgentRevoked) {
		return err
	}
	slog.Warn("publishing agent last_seen_at refresh failed", "agent_id", agent.ID, "err", err)
	return nil
}

func (s *Service) ListAgents(ctx context.Context, userID string) ([]Agent, error) {
	return s.store.ListAgents(ctx, userID)
}

func (s *Service) UpdateAgent(ctx context.Context, userID, agentID, label, categoryID string, visibility Visibility) (Agent, error) {
	if strings.TrimSpace(label) == "" || !validVisibility(visibility) {
		return Agent{}, ErrInvalid
	}
	agent, err := s.store.OwnedAgent(ctx, userID, agentID)
	if err != nil {
		return Agent{}, err
	}
	if agent.RevokedAt != nil {
		return Agent{}, ErrAgentRevoked
	}
	if !agent.HasCategory(categoryID) {
		return Agent{}, ErrCategoryNotFound
	}
	return s.store.UpdateAgent(ctx, userID, agentID, strings.TrimSpace(label), categoryID, visibility, s.clock.Now())
}

func (s *Service) SyncAgent(ctx context.Context, agent Agent, update ProfileUpdate) (Agent, error) {
	if strings.TrimSpace(update.PlatformAccountID) == "" || strings.TrimSpace(update.BrowserLabel) == "" ||
		!validVisibility(update.DefaultVisibility) || !containsCategory(update.Categories, update.DefaultCategoryID) {
		return Agent{}, ErrInvalid
	}
	return s.store.SyncAgent(ctx, agent.UserID, agent.ID, update, s.clock.Now())
}

func (s *Service) RevokeAgent(ctx context.Context, userID, agentID string) error {
	if err := s.store.RevokeAgent(ctx, userID, agentID, s.clock.Now()); err != nil {
		return err
	}
	// Drops the suppression entry so the map stays bounded by the agents currently
	// enrolled, rather than by every agent this process has ever seen.
	s.touchMu.Lock()
	delete(s.lastTouch, agentID)
	s.touchMu.Unlock()
	return nil
}

func (s *Service) Start(ctx context.Context, request StartRequest) (Job, error) {
	if !validVisibility(request.Visibility) || request.ExpectedContentRevision <= 0 {
		return Job{}, ErrInvalid
	}
	agent, err := s.store.OwnedAgent(ctx, request.UserID, request.AgentID)
	if err != nil {
		return Job{}, err
	}
	if agent.RevokedAt != nil {
		return Job{}, ErrAgentRevoked
	}
	if !agent.Ready() {
		return Job{}, ErrAgentNotReady
	}
	categoryName, categoryFound := agent.CategoryName(request.CategoryID)
	if !categoryFound {
		return Job{}, ErrCategoryNotFound
	}
	postCreatedAt, err := s.posts.PostIdentity(ctx, request.UserID, request.PostSlug)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			previous, latestErr := s.store.LatestJobForDeletedPost(ctx, request.UserID, request.PostSlug)
			if latestErr == nil && previous.Status == StatusNeedsAttention {
				return s.retryAttention(ctx, request, previous)
			}
			if latestErr != nil && !errors.Is(latestErr, ErrNotFound) {
				return Job{}, latestErr
			}
		}
		return Job{}, err
	}
	if previous, latestErr := s.store.LatestJobForPost(ctx, request.UserID, request.PostSlug, postCreatedAt); latestErr == nil {
		switch previous.Status {
		case StatusNeedsAttention:
			return s.retryAttention(ctx, request, previous)
		case StatusQueued, StatusRunning, StatusPublished, StatusOutcomeUnknown:
			return Job{}, ErrAlreadyPublishing
		}
	} else if !errors.Is(latestErr, ErrNotFound) {
		return Job{}, latestErr
	}
	snapshot, err := s.posts.PublishingSnapshot(ctx, request.UserID, request.PostSlug)
	if err != nil {
		return Job{}, err
	}
	if !snapshot.TargetLanguage.Valid() || !snapshot.ContentLanguage.Valid() || !snapshot.VoiceSourceLanguage.Valid() {
		return Job{}, ErrLanguageRequired
	}
	if snapshot.ContentRevision != request.ExpectedContentRevision || snapshot.FinalizedRevision != snapshot.ContentRevision {
		return Job{}, ErrStaleRevision
	}
	ordered, err := orderedImages(snapshot.Content, snapshot.Images)
	if err != nil {
		return Job{}, err
	}
	now := s.clock.Now()
	jobID, err := s.tokens.NewID()
	if err != nil {
		return Job{}, fmt.Errorf("generate publish job id: %w", err)
	}
	if err := s.store.ReserveJobID(ctx, request.UserID, jobID, now); err != nil {
		return Job{}, fmt.Errorf("reserve publish job id: %w", err)
	}
	assets := make([]Asset, 0, len(ordered))
	for ordinal, image := range ordered {
		localFilename := fmt.Sprintf("%04d.jpg", ordinal)
		target := fmt.Sprintf("publishing/%s/%s", jobID, localFilename)
		bytes, copyErr := s.staging.Copy(ctx, image.Key, target)
		if copyErr != nil || bytes <= 0 || (image.Bytes > 0 && bytes != image.Bytes) {
			// CopyObject can succeed even when its verification call fails. Delete the
			// current target as well as prior copies; Delete is intentionally idempotent.
			clean := s.staging.Delete(ctx, target) == nil
			for _, staged := range assets {
				clean = s.staging.Delete(ctx, staged.StagedKey) == nil && clean
			}
			if clean {
				_ = s.store.ReleaseJobID(ctx, request.UserID, jobID)
			}
			if copyErr != nil {
				return Job{}, fmt.Errorf("stage %s: %w", image.Filename, copyErr)
			}
			return Job{}, fmt.Errorf("stage %s: size mismatch", image.Filename)
		}
		assets = append(assets, Asset{JobID: jobID, UserID: request.UserID, Ordinal: ordinal, Filename: localFilename, SourceFilename: image.Filename, StagedKey: target, Bytes: bytes, CreatedAt: now})
	}
	manifest := Manifest{JobID: jobID, PostSlug: snapshot.PostSlug, ContentRevision: snapshot.ContentRevision,
		TargetLanguage: snapshot.TargetLanguage, ContentLanguage: snapshot.ContentLanguage, VoiceSourceLanguage: snapshot.VoiceSourceLanguage,
		Content: cloneContent(snapshot.Content), Tags: append([]string(nil), snapshot.Content.Tags...), CategoryID: request.CategoryID,
		CategoryName: categoryName, Visibility: request.Visibility, ExpectedPlatformAccountID: agent.PlatformAccountID, Assets: assets}
	job := Job{ID: jobID, UserID: request.UserID, PostSlug: snapshot.PostSlug, PostCreatedAt: snapshot.CreatedAt, AgentID: agent.ID,
		Platform: PlatformNaverBlog, Status: StatusQueued, Stage: StageQueued, ContentRevision: snapshot.ContentRevision,
		TargetLanguage: snapshot.TargetLanguage, ContentLanguage: snapshot.ContentLanguage, VoiceSourceLanguage: snapshot.VoiceSourceLanguage,
		Manifest: &manifest, CategoryID: request.CategoryID, Visibility: request.Visibility, CreatedAt: now, UpdatedAt: now}
	// Staging can take long enough for another tab to edit the post or for the owner
	// to revoke/reconfigure the Mac. CreateJob holds the one writer transaction while
	// this callback re-reads both aggregates through their published behaviors. Post
	// writes and agent changes cannot pass that writer reservation until insertion
	// commits, so the accepted request has one atomic current-state boundary.
	guard := func(guardCtx context.Context) error {
		currentAgent, guardErr := s.store.OwnedAgent(guardCtx, request.UserID, request.AgentID)
		if guardErr != nil {
			return guardErr
		}
		if currentAgent.RevokedAt != nil {
			return ErrAgentRevoked
		}
		if !currentAgent.Ready() || currentAgent.PlatformAccountID != agent.PlatformAccountID {
			return ErrAgentNotReady
		}
		currentCategoryName, categoryFound := currentAgent.CategoryName(request.CategoryID)
		if !categoryFound || currentCategoryName != categoryName {
			return ErrCategoryNotFound
		}
		currentSnapshot, guardErr := s.posts.PublishingSnapshot(guardCtx, request.UserID, request.PostSlug)
		if guardErr != nil {
			return guardErr
		}
		if currentSnapshot.ContentRevision != request.ExpectedContentRevision ||
			currentSnapshot.FinalizedRevision != currentSnapshot.ContentRevision ||
			!reflect.DeepEqual(currentSnapshot, snapshot) {
			return ErrStaleRevision
		}
		return nil
	}
	if err := s.store.CreateJob(ctx, job, assets, guard); err != nil {
		clean := true
		for _, staged := range assets {
			clean = s.staging.Delete(ctx, staged.StagedKey) == nil && clean
		}
		if clean {
			_ = s.store.ReleaseJobID(ctx, request.UserID, jobID)
		}
		if errors.Is(err, ErrAlreadyPublishing) {
			return Job{}, err
		}
		return Job{}, fmt.Errorf("create publish job: %w", err)
	}
	return job, nil
}

func (s *Service) retryAttention(ctx context.Context, request StartRequest, previous Job) (Job, error) {
	if previous.PostSlug != "" && previous.PostSlug != request.PostSlug {
		return Job{}, ErrAlreadyPublishing
	}
	if previous.AgentID != request.AgentID || previous.ContentRevision != request.ExpectedContentRevision ||
		previous.CategoryID != request.CategoryID || previous.Visibility != request.Visibility || previous.Manifest == nil {
		return Job{}, ErrAlreadyPublishing
	}
	return s.store.RetryAttentionJob(ctx, request.UserID, previous.ID, s.clock.Now())
}

func cloneContent(content Content) Content {
	cloned := content
	cloned.Tags = append([]string(nil), content.Tags...)
	cloned.Blocks = make([]Block, len(content.Blocks))
	for index, block := range content.Blocks {
		cloned.Blocks[index] = block
		cloned.Blocks[index].Items = append([]string(nil), block.Items...)
	}
	return cloned
}

func (s *Service) GetJob(ctx context.Context, userID, postSlug, jobID string) (Job, error) {
	if jobID != "" {
		return s.store.OwnedJob(ctx, userID, jobID)
	}
	postCreatedAt, err := s.posts.PostIdentity(ctx, userID, postSlug)
	if err != nil {
		return Job{}, err
	}
	return s.store.LatestJobForPost(ctx, userID, postSlug, postCreatedAt)
}

func (s *Service) ListRetryable(ctx context.Context, userID string) ([]Job, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, ErrForbidden
	}
	return s.store.ListRetryableJobs(ctx, userID)
}

// Retry resumes the same immutable pre-commit manifest by account-scoped job
// identity. It intentionally does not require the source post to still exist.
func (s *Service) Retry(ctx context.Context, userID, jobID string) (Job, error) {
	job, err := s.store.OwnedJob(ctx, userID, strings.TrimSpace(jobID))
	if err != nil {
		return Job{}, err
	}
	if job.Status != StatusNeedsAttention || job.CommittedAt != nil || job.Manifest == nil {
		return Job{}, ErrTransition
	}
	agent, err := s.store.OwnedAgent(ctx, userID, job.AgentID)
	if err != nil {
		return Job{}, err
	}
	categoryName, categoryFound := agent.CategoryName(job.CategoryID)
	if agent.RevokedAt != nil {
		return Job{}, ErrAgentRevoked
	}
	if !agent.Ready() || agent.PlatformAccountID != job.Manifest.ExpectedPlatformAccountID {
		return Job{}, ErrAgentNotReady
	}
	if !categoryFound || categoryName != job.Manifest.CategoryName {
		return Job{}, ErrCategoryNotFound
	}
	return s.store.RetryAttentionJob(ctx, userID, job.ID, s.clock.Now())
}

func (s *Service) Cancel(ctx context.Context, userID, jobID string) (Job, error) {
	current, err := s.store.OwnedJob(ctx, userID, jobID)
	if err != nil {
		return Job{}, err
	}
	if !CanCancel(current) {
		return Job{}, ErrCommitFence
	}
	job, err := s.store.Cancel(ctx, userID, jobID, s.clock.Now())
	if err != nil {
		return Job{}, err
	}
	// The durable terminal state is the RPC outcome. Object deletion is idempotent
	// housekeeping and CleanupTerminals retries it; surfacing a cleanup failure here
	// would make a successful cancel look ambiguous to the caller.
	_ = s.cleanupAssets(ctx, job.ID)
	return job, nil
}

func (s *Service) Claim(ctx context.Context, agent Agent) (Claim, error) {
	if !agent.Ready() {
		return Claim{}, ErrAgentNotReady
	}
	raw, err := s.tokens.NewSecret(32)
	if err != nil {
		return Claim{}, err
	}
	now := s.clock.Now()
	expires := now.Add(s.config.LeaseTTL)
	job, err := s.store.ClaimJob(ctx, agent, hashToken(raw), expires, now)
	if err != nil {
		return Claim{}, err
	}
	if job.Manifest == nil {
		return Claim{}, fmt.Errorf("%w: claimed job has no manifest", ErrInvalid)
	}
	manifest := *job.Manifest
	urls := make([]string, len(manifest.Assets))
	for i, asset := range manifest.Assets {
		urls[i], err = s.staging.SignGet(ctx, asset.StagedKey, s.config.AssetURLTTL)
		if err != nil {
			return Claim{}, fmt.Errorf("sign staged asset: %w", err)
		}
	}
	return Claim{Job: job, LeaseToken: raw, LeaseExpires: expires, LeaseTTL: s.config.LeaseTTL, Manifest: manifest, URLs: urls}, nil
}

func (s *Service) Renew(ctx context.Context, agent Agent, jobID, leaseToken string) (time.Time, error) {
	now := s.clock.Now()
	expires := now.Add(s.config.LeaseTTL)
	if err := s.store.RenewLease(ctx, agent, jobID, hashToken(leaseToken), expires, now); err != nil {
		return time.Time{}, err
	}
	return expires, nil
}

func (s *Service) Progress(ctx context.Context, agent Agent, jobID, leaseToken string, stage Stage, seq int64) (Job, error) {
	current, err := s.store.OwnedJob(ctx, agent.UserID, jobID)
	if err != nil {
		return Job{}, err
	}
	if current.AgentID != agent.ID {
		return Job{}, ErrForbidden
	}
	if err := ValidateProgress(current.Stage, stage, current.ProgressSeq, seq); err != nil {
		return Job{}, err
	}
	return s.store.UpdateProgress(ctx, agent, jobID, hashToken(leaseToken), current.Stage, current.ProgressSeq, stage, seq, s.clock.Now())
}

func (s *Service) Complete(ctx context.Context, agent Agent, jobID, leaseToken string, seq int64, publishedURL string) (Job, error) {
	current, err := s.store.OwnedJob(ctx, agent.UserID, jobID)
	if err != nil {
		return Job{}, err
	}
	if current.AgentID != agent.ID || current.Manifest == nil {
		return Job{}, ErrForbidden
	}
	if current.Stage != StageVerifying || seq <= current.ProgressSeq {
		return Job{}, ErrTransition
	}
	if !validNaverURL(publishedURL, current.Manifest.ExpectedPlatformAccountID) {
		return Job{}, ErrPublishedURLInvalid
	}
	job, err := s.store.Complete(ctx, agent, jobID, hashToken(leaseToken), seq, publishedURL, s.clock.Now())
	if err != nil {
		return Job{}, err
	}
	// Publication is already durable and must never be reported as failed merely
	// because staged-object cleanup needs the terminal sweeper to retry it.
	_ = s.cleanupAssets(ctx, job.ID)
	return job, nil
}

func (s *Service) Fail(ctx context.Context, agent Agent, jobID, leaseToken string, seq int64, kind FailureKind, detail string) (Job, error) {
	current, err := s.store.OwnedJob(ctx, agent.UserID, jobID)
	if err != nil {
		return Job{}, err
	}
	if current.AgentID != agent.ID || seq <= current.ProgressSeq {
		return Job{}, ErrLeaseInvalid
	}
	status := FailureStatus(current.Stage, kind)
	precommitFailure := failureForAgentReport(status, kind, detail)
	commitFailure := Failure{Reason: "PUBLISH_OUTCOME_UNKNOWN", TechnicalDetail: precommitFailure.TechnicalDetail}
	job, err := s.store.Fail(ctx, agent, jobID, hashToken(leaseToken), seq, status, precommitFailure, commitFailure, s.clock.Now())
	if err != nil {
		return Job{}, err
	}
	if job.Status != StatusNeedsAttention {
		// As with Complete and Cancel, terminal persistence is authoritative; the
		// cleanup sweeper owns retrying any transient object-store failure.
		_ = s.cleanupAssets(ctx, job.ID)
	}
	return job, nil
}

func failureForAgentReport(status Status, kind FailureKind, detail string) Failure {
	reason := "PUBLISH_AGENT_UNAVAILABLE"
	switch status {
	case StatusNeedsAttention:
		reason = "PUBLISH_NEEDS_ATTENTION"
	case StatusOutcomeUnknown:
		reason = "PUBLISH_OUTCOME_UNKNOWN"
	}
	// The Mac adapter sends bounded redaction metadata rather than browser/model prose.
	// Treat it as inert diagnostic text and never interpolate it into a translation.
	return Failure{Reason: reason, TechnicalDetail: strings.TrimSpace(detail)}
}

func (s *Service) RecoverExpired(ctx context.Context) (int64, int64, error) {
	return s.store.RequeueExpired(ctx, s.clock.Now())
}

func (s *Service) CleanupTerminals(ctx context.Context) error {
	jobs, err := s.store.TerminalJobsWithAssets(ctx)
	if err != nil {
		return err
	}
	var cleanupErr error
	for _, jobID := range jobs {
		if err := s.cleanupAssets(ctx, jobID); err != nil {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("cleanup terminal job %s: %w", jobID, err))
		}
	}
	return cleanupErr
}

func (s *Service) SweepOrphans(ctx context.Context, minAge time.Duration) (int, error) {
	live, err := s.store.LiveStagedKeys(ctx)
	if err != nil {
		return 0, err
	}
	objects, err := s.staging.ListStaged(ctx, "publishing/")
	if err != nil {
		return 0, err
	}
	cutoff := s.clock.Now().Add(-minAge)
	removed := 0
	for _, object := range objects {
		if _, ok := live[object.Key]; ok || !object.LastModified.Before(cutoff) {
			continue
		}
		if err := s.staging.Delete(ctx, object.Key); err != nil {
			return removed, err
		}
		removed++
	}
	return removed, nil
}

func (s *Service) cleanupAssets(ctx context.Context, jobID string) error {
	assets, err := s.store.Assets(ctx, jobID)
	if err != nil {
		return err
	}
	var cleanupErr error
	for _, asset := range assets {
		if err := s.staging.Delete(ctx, asset.StagedKey); err != nil {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("delete staged asset: %w", err))
		}
	}
	if cleanupErr != nil {
		return cleanupErr
	}
	return s.store.DeleteAssets(ctx, jobID)
}

func orderedImages(content Content, images []SnapshotImage) ([]SnapshotImage, error) {
	byName := make(map[string]SnapshotImage, len(images))
	for _, image := range images {
		byName[image.Filename] = image
	}
	ordered := make([]SnapshotImage, 0)
	for _, block := range content.Blocks {
		if block.Type != BlockImage {
			continue
		}
		image, ok := byName[block.File]
		if !ok || image.Key == "" || image.Bytes <= 0 {
			return nil, fmt.Errorf("%w: image %q is missing", ErrInvalid, block.File)
		}
		ordered = append(ordered, image)
	}
	return ordered, nil
}

func validVisibility(value Visibility) bool {
	return value == VisibilityPublic || value == VisibilityPrivate
}

func containsCategory(categories []Category, id string) bool {
	for _, category := range categories {
		if category.ID == id && strings.TrimSpace(category.Name) != "" {
			return true
		}
	}
	return false
}

func normalizeCode(value string) string {
	return strings.ToUpper(strings.NewReplacer("-", "", " ", "").Replace(strings.TrimSpace(value)))
}

func hashCode(value string) string {
	sum := sha256.Sum256([]byte(normalizeCode(value)))
	return hex.EncodeToString(sum[:])
}

func hashToken(value string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return hex.EncodeToString(sum[:])
}

func validNaverURL(raw, accountID string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || strings.ToLower(u.Hostname()) != "blog.naver.com" {
		return false
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) != 2 || parts[0] != accountID || parts[1] == "" {
		return false
	}
	for _, digit := range parts[1] {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now().UTC() }

type cryptoTokens struct{}

func (cryptoTokens) NewID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
func (cryptoTokens) NewSecret(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
