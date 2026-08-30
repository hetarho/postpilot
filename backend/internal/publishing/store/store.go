// Package store is the SQLite anti-corruption boundary for publishing. SQL rows and
// JSON persistence shapes stop here; the publishing domain stays transport/storage free.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/postpilot/backend/internal/publishing"
	"github.com/postpilot/backend/internal/publishing/store/sqlc"
)

const writeLayout = "2006-01-02T15:04:05.000000000Z07:00"

type Store struct {
	writer *sql.DB
	read   *sqlc.Queries
	write  *sqlc.Queries
}

func New(writer, reader *sql.DB) *Store {
	return &Store{writer: writer, read: sqlc.New(reader), write: sqlc.New(writer)}
}

func (s *Store) CreatePairing(ctx context.Context, codeHash, userID, label string, expiresAt, createdAt time.Time, maxPending int) error {
	inserted, err := s.write.CreatePairingBelowLimit(ctx, sqlc.CreatePairingBelowLimitParams{
		CodeHash: codeHash, UserID: userID, Label: label, PairingExpiresAt: formatTime(expiresAt),
		CreatedAt: formatTime(createdAt), Now: formatTime(createdAt), MaxPending: int64(maxPending),
	})
	if err != nil {
		return err
	}
	if inserted != 1 {
		return publishing.ErrPairingLimit
	}
	return nil
}

func (s *Store) Enroll(ctx context.Context, codeHash, tokenHash, agentID, browserLabel string, now time.Time) (publishing.Agent, error) {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return publishing.Agent{}, err
	}
	defer tx.Rollback()
	q := sqlc.New(tx)
	stamp := formatTime(now)
	pairing, err := q.ConsumePairing(ctx, sqlc.ConsumePairingParams{ConsumedAt: nullableString(stamp), CodeHash: codeHash, ExpiresAt: stamp})
	if errors.Is(err, sql.ErrNoRows) {
		return publishing.Agent{}, publishing.ErrPairingInvalid
	}
	if err != nil {
		return publishing.Agent{}, fmt.Errorf("consume pairing: %w", err)
	}
	if err := q.CreateAgent(ctx, sqlc.CreateAgentParams{ID: agentID, UserID: pairing.UserID, TokenHash: tokenHash, Label: pairing.Label, BrowserLabel: browserLabel, CreatedAt: stamp, UpdatedAt: stamp}); err != nil {
		return publishing.Agent{}, fmt.Errorf("create agent: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return publishing.Agent{}, err
	}
	return publishing.Agent{ID: agentID, UserID: pairing.UserID, Label: pairing.Label, Platform: publishing.PlatformNaverBlog, BrowserLabel: browserLabel, DefaultVisibility: publishing.VisibilityPublic, CreatedAt: now, UpdatedAt: now}, nil
}

func (s *Store) AgentByTokenHash(ctx context.Context, tokenHash string) (publishing.Agent, error) {
	row, err := s.read.GetActiveAgentByTokenHash(ctx, tokenHash)
	if errors.Is(err, sql.ErrNoRows) {
		return publishing.Agent{}, publishing.ErrNotFound
	}
	if err != nil {
		return publishing.Agent{}, err
	}
	return toAgent(row)
}

func (s *Store) TouchAgent(ctx context.Context, userID, agentID string, now time.Time) error {
	stamp := formatTime(now)
	n, err := s.write.TouchAgent(ctx, sqlc.TouchAgentParams{LastSeenAt: nullableString(stamp), UpdatedAt: stamp, ID: agentID, UserID: userID})
	if err != nil {
		return err
	}
	if n != 1 {
		return publishing.ErrAgentRevoked
	}
	return nil
}

func (s *Store) OwnedAgent(ctx context.Context, userID, agentID string) (publishing.Agent, error) {
	row, err := s.read.GetOwnedAgent(ctx, sqlc.GetOwnedAgentParams{ID: agentID, UserID: userID})
	if errors.Is(err, sql.ErrNoRows) {
		return publishing.Agent{}, publishing.ErrNotFound
	}
	if err != nil {
		return publishing.Agent{}, err
	}
	return toAgent(row)
}

func (s *Store) ListAgents(ctx context.Context, userID string) ([]publishing.Agent, error) {
	rows, err := s.read.ListAgentsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]publishing.Agent, 0, len(rows))
	for _, row := range rows {
		agent, err := toAgent(row)
		if err != nil {
			return nil, err
		}
		out = append(out, agent)
	}
	return out, nil
}

func (s *Store) UpdateAgent(ctx context.Context, userID, agentID, label, categoryID string, visibility publishing.Visibility, now time.Time) (publishing.Agent, error) {
	n, err := s.write.UpdateOwnedAgentDefaults(ctx, sqlc.UpdateOwnedAgentDefaultsParams{Label: label, DefaultCategoryID: categoryID, DefaultVisibility: string(visibility), UpdatedAt: formatTime(now), ID: agentID, UserID: userID})
	if err != nil {
		return publishing.Agent{}, err
	}
	if n != 1 {
		return publishing.Agent{}, publishing.ErrNotFound
	}
	return s.OwnedAgent(ctx, userID, agentID)
}

func (s *Store) SyncAgent(ctx context.Context, userID, agentID string, update publishing.ProfileUpdate, now time.Time) (publishing.Agent, error) {
	categories, err := json.Marshal(update.Categories)
	if err != nil {
		return publishing.Agent{}, err
	}
	ready := int64(0)
	if update.CompatibilityReady {
		ready = 1
	}
	stamp := formatTime(now)
	n, err := s.write.SyncAgentProfile(ctx, sqlc.SyncAgentProfileParams{
		PlatformAccountID: update.PlatformAccountID, PlatformAccountLabel: update.PlatformAccountLabel,
		BrowserLabel: update.BrowserLabel, CategoriesJson: string(categories), DefaultCategoryID: update.DefaultCategoryID,
		DefaultVisibility: string(update.DefaultVisibility), CompatibilityReady: ready, HermesVersion: update.HermesVersion,
		LastSeenAt: nullableString(stamp), UpdatedAt: stamp, ID: agentID, UserID: userID,
	})
	if err != nil {
		return publishing.Agent{}, err
	}
	if n != 1 {
		return publishing.Agent{}, publishing.ErrAgentRevoked
	}
	return s.OwnedAgent(ctx, userID, agentID)
}

func (s *Store) RevokeAgent(ctx context.Context, userID, agentID string, now time.Time) error {
	stamp := formatTime(now)
	n, err := s.write.RevokeOwnedAgent(ctx, sqlc.RevokeOwnedAgentParams{RevokedAt: nullableString(stamp), UpdatedAt: stamp, ID: agentID, UserID: userID})
	if err != nil {
		return err
	}
	if n != 1 {
		return publishing.ErrNotFound
	}
	return nil
}

func (s *Store) ReserveJobID(ctx context.Context, userID, jobID string, now time.Time) error {
	n, err := s.write.ReservePublishJobID(ctx, sqlc.ReservePublishJobIDParams{
		ID: jobID, UserID: userID, CreatedAt: formatTime(now),
	})
	if err != nil {
		return err
	}
	if n != 1 {
		return publishing.ErrAlreadyPublishing
	}
	return nil
}

func (s *Store) ReleaseJobID(ctx context.Context, userID, jobID string) error {
	_, err := s.write.ReleaseUnusedPublishJobID(ctx, sqlc.ReleaseUnusedPublishJobIDParams{
		ID: jobID, UserID: userID,
	})
	return err
}

func (s *Store) CreateJob(ctx context.Context, job publishing.Job, assets []publishing.Asset, guard func(context.Context) error) error {
	manifest, err := marshalManifest(job.Manifest)
	if err != nil {
		return err
	}
	settings, err := json.Marshal(settingsDTO{CategoryID: job.CategoryID, Visibility: string(job.Visibility)})
	if err != nil {
		return err
	}
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	q := sqlc.New(tx)
	locked, err := q.LockReadyAgentForPublish(ctx, sqlc.LockReadyAgentForPublishParams{ID: job.AgentID, UserID: job.UserID})
	if err != nil {
		return err
	}
	if locked != 1 {
		return publishing.ErrAgentNotReady
	}
	if guard == nil {
		return errors.New("publish start guard is required")
	}
	if err := guard(ctx); err != nil {
		return err
	}
	err = q.CreatePublishJob(ctx, sqlc.CreatePublishJobParams{ID: job.ID, UserID: job.UserID, PostSlug: job.PostSlug, PostCreatedAt: formatTime(job.PostCreatedAt), AgentID: job.AgentID, ContentRevision: job.ContentRevision, ManifestJson: nullableString(manifest), SettingsJson: string(settings), CreatedAt: formatTime(job.CreatedAt), UpdatedAt: formatTime(job.UpdatedAt)})
	if err != nil {
		if isUnique(err) {
			return publishing.ErrAlreadyPublishing
		}
		return err
	}
	for _, asset := range assets {
		if err := q.CreatePublishAsset(ctx, sqlc.CreatePublishAssetParams{JobID: asset.JobID, UserID: asset.UserID, Ordinal: int64(asset.Ordinal), Filename: asset.Filename, SourceFilename: asset.SourceFilename, StagedKey: asset.StagedKey, Bytes: asset.Bytes, CreatedAt: formatTime(asset.CreatedAt)}); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) RetryAttentionJob(ctx context.Context, userID, jobID string, now time.Time) (publishing.Job, error) {
	n, err := s.write.RetryAttentionPublishJob(ctx, sqlc.RetryAttentionPublishJobParams{
		UpdatedAt: formatTime(now), ID: jobID, UserID: userID,
	})
	if err != nil {
		return publishing.Job{}, err
	}
	if n != 1 {
		return publishing.Job{}, publishing.ErrTransition
	}
	return s.OwnedJob(ctx, userID, jobID)
}

func (s *Store) ListRetryableJobs(ctx context.Context, userID string) ([]publishing.Job, error) {
	rows, err := s.read.ListRetryablePublishJobs(ctx, userID)
	if err != nil {
		return nil, err
	}
	jobs := make([]publishing.Job, 0, len(rows))
	for _, row := range rows {
		job, convertErr := toJob(row)
		if convertErr != nil {
			return nil, convertErr
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

func (s *Store) OwnedJob(ctx context.Context, userID, jobID string) (publishing.Job, error) {
	row, err := s.read.GetOwnedPublishJob(ctx, sqlc.GetOwnedPublishJobParams{ID: jobID, UserID: userID})
	if errors.Is(err, sql.ErrNoRows) {
		return publishing.Job{}, publishing.ErrNotFound
	}
	if err != nil {
		return publishing.Job{}, err
	}
	return toJob(row)
}

func (s *Store) LatestJobForPost(ctx context.Context, userID, postSlug string, postCreatedAt time.Time) (publishing.Job, error) {
	row, err := s.read.GetLatestPublishJobForPost(ctx, sqlc.GetLatestPublishJobForPostParams{PostSlug: postSlug, UserID: userID, PostCreatedAt: formatTime(postCreatedAt)})
	if errors.Is(err, sql.ErrNoRows) {
		return publishing.Job{}, publishing.ErrNotFound
	}
	if err != nil {
		return publishing.Job{}, err
	}
	return toJob(row)
}

func (s *Store) LatestJobForDeletedPost(ctx context.Context, userID, postSlug string) (publishing.Job, error) {
	row, err := s.read.GetLatestPublishJobForDeletedPost(ctx, sqlc.GetLatestPublishJobForDeletedPostParams{PostSlug: postSlug, UserID: userID})
	if errors.Is(err, sql.ErrNoRows) {
		return publishing.Job{}, publishing.ErrNotFound
	}
	if err != nil {
		return publishing.Job{}, err
	}
	return toJob(row)
}

func (s *Store) ClaimJob(ctx context.Context, agent publishing.Agent, leaseHash string, expiresAt, now time.Time) (publishing.Job, error) {
	stamp := formatTime(now)
	row, err := s.write.ClaimQueuedPublishJob(ctx, sqlc.ClaimQueuedPublishJobParams{LeaseTokenHash: nullableString(leaseHash), LeaseExpiresAt: nullableString(formatTime(expiresAt)), ClaimedAt: nullableString(stamp), UpdatedAt: stamp, ID: agent.ID, UserID: agent.UserID})
	if errors.Is(err, sql.ErrNoRows) {
		return publishing.Job{}, publishing.ErrNotFound
	}
	if err != nil {
		return publishing.Job{}, err
	}
	return toJob(row)
}

func (s *Store) RenewLease(ctx context.Context, agent publishing.Agent, jobID, leaseHash string, expiresAt, now time.Time) error {
	n, err := s.write.RenewPublishLease(ctx, sqlc.RenewPublishLeaseParams{LeaseExpiresAt: nullableString(formatTime(expiresAt)), UpdatedAt: formatTime(now), ID: jobID, UserID: agent.UserID, AgentID: agent.ID, LeaseTokenHash: nullableString(leaseHash), LeaseExpiresAt_2: nullableString(formatTime(now))})
	if err != nil {
		return err
	}
	if n != 1 {
		return publishing.ErrLeaseInvalid
	}
	return nil
}

func (s *Store) UpdateProgress(ctx context.Context, agent publishing.Agent, jobID, leaseHash string, currentStage publishing.Stage, currentSeq int64, nextStage publishing.Stage, nextSeq int64, now time.Time) (publishing.Job, error) {
	stamp := formatTime(now)
	n, err := s.write.UpdatePublishProgress(ctx, sqlc.UpdatePublishProgressParams{
		NextStage: string(nextStage), NextSeq: nextSeq, CommittedAt: nullableString(stamp), UpdatedAt: stamp,
		ID: jobID, UserID: agent.UserID, AgentID: agent.ID, LeaseTokenHash: nullableString(leaseHash), Now: nullableString(stamp),
		CurrentStage: string(currentStage), CurrentSeq: currentSeq,
	})
	if err != nil {
		return publishing.Job{}, err
	}
	if n != 1 {
		return publishing.Job{}, publishing.ErrLeaseInvalid
	}
	return s.OwnedJob(ctx, agent.UserID, jobID)
}

func (s *Store) Complete(ctx context.Context, agent publishing.Agent, jobID, leaseHash string, seq int64, publishedURL string, now time.Time) (publishing.Job, error) {
	stamp := formatTime(now)
	n, err := s.write.CompletePublishJob(ctx, sqlc.CompletePublishJobParams{ProgressSeq: seq, PlatformPostUrl: nullableString(publishedURL), PublishedAt: nullableString(stamp), UpdatedAt: stamp, ID: jobID, UserID: agent.UserID, AgentID: agent.ID, LeaseTokenHash: nullableString(leaseHash), LeaseExpiresAt: nullableString(stamp), ProgressSeq_2: seq})
	if err != nil {
		return publishing.Job{}, err
	}
	if n != 1 {
		return publishing.Job{}, publishing.ErrLeaseInvalid
	}
	return s.OwnedJob(ctx, agent.UserID, jobID)
}

func (s *Store) Fail(ctx context.Context, agent publishing.Agent, jobID, leaseHash string, seq int64, status publishing.Status, code, message string, now time.Time) (publishing.Job, error) {
	stamp := formatTime(now)
	n, err := s.write.FailPublishJob(ctx, sqlc.FailPublishJobParams{
		CommitStatus: string(publishing.StatusOutcomeUnknown), PrecommitStatus: string(status), NextSeq: seq,
		CommitErrorCode: nullableString("commit_outcome_unknown"), PrecommitErrorCode: nullableString(code),
		CommitErrorMessage: nullableString("최종 발행 결과를 확인할 수 없어요."), PrecommitErrorMessage: nullableString(message), UpdatedAt: stamp, ID: jobID, UserID: agent.UserID,
		AgentID: agent.ID, LeaseTokenHash: nullableString(leaseHash), Now: nullableString(stamp),
	})
	if err != nil {
		return publishing.Job{}, err
	}
	if n != 1 {
		return publishing.Job{}, publishing.ErrLeaseInvalid
	}
	return s.OwnedJob(ctx, agent.UserID, jobID)
}

func (s *Store) Cancel(ctx context.Context, userID, jobID string, now time.Time) (publishing.Job, error) {
	n, err := s.write.CancelPublishJob(ctx, sqlc.CancelPublishJobParams{UpdatedAt: formatTime(now), ID: jobID, UserID: userID})
	if err != nil {
		return publishing.Job{}, err
	}
	if n != 1 {
		return publishing.Job{}, publishing.ErrCommitFence
	}
	return s.OwnedJob(ctx, userID, jobID)
}

func (s *Store) RequeueExpired(ctx context.Context, now time.Time) (int64, int64, error) {
	stamp := formatTime(now)
	requeued, err := s.write.RequeueExpiredPreCommitJobs(ctx, sqlc.RequeueExpiredPreCommitJobsParams{UpdatedAt: stamp, LeaseExpiresAt: nullableString(stamp)})
	if err != nil {
		return 0, 0, err
	}
	unknown, err := s.write.MarkExpiredCommittedJobsUnknown(ctx, sqlc.MarkExpiredCommittedJobsUnknownParams{ErrorCode: nullableString("commit_outcome_unknown"), ErrorMessage: nullableString("최종 발행 결과를 확인할 수 없어요."), UpdatedAt: stamp, LeaseExpiresAt: nullableString(stamp)})
	return requeued, unknown, err
}

func (s *Store) Assets(ctx context.Context, jobID string) ([]publishing.Asset, error) {
	rows, err := s.read.ListPublishAssets(ctx, jobID)
	if err != nil {
		return nil, err
	}
	out := make([]publishing.Asset, 0, len(rows))
	for _, row := range rows {
		created, err := parseTime(row.CreatedAt)
		if err != nil {
			return nil, err
		}
		out = append(out, publishing.Asset{JobID: row.JobID, UserID: row.UserID, Ordinal: int(row.Ordinal), Filename: row.Filename, SourceFilename: row.SourceFilename, StagedKey: row.StagedKey, Bytes: row.Bytes, CreatedAt: created})
	}
	return out, nil
}

func (s *Store) DeleteAssets(ctx context.Context, jobID string) error {
	return s.write.DeletePublishAssets(ctx, jobID)
}

func (s *Store) LiveStagedKeys(ctx context.Context) (map[string]struct{}, error) {
	keys, err := s.read.ListLiveStagedKeys(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		out[key] = struct{}{}
	}
	return out, nil
}

func (s *Store) TerminalJobsWithAssets(ctx context.Context) ([]string, error) {
	return s.read.ListTerminalJobsWithAssets(ctx)
}

func toAgent(row sqlc.PublishingAgent) (publishing.Agent, error) {
	var categories []publishing.Category
	if err := json.Unmarshal([]byte(row.CategoriesJson), &categories); err != nil {
		return publishing.Agent{}, fmt.Errorf("decode agent categories: %w", err)
	}
	created, err := parseTime(row.CreatedAt)
	if err != nil {
		return publishing.Agent{}, err
	}
	updated, err := parseTime(row.UpdatedAt)
	if err != nil {
		return publishing.Agent{}, err
	}
	lastSeen, err := parseOptionalTime(row.LastSeenAt)
	if err != nil {
		return publishing.Agent{}, err
	}
	revoked, err := parseOptionalTime(row.RevokedAt)
	if err != nil {
		return publishing.Agent{}, err
	}
	return publishing.Agent{ID: row.ID, UserID: row.UserID, Label: row.Label, Platform: row.Platform,
		PlatformAccountID: row.PlatformAccountID, PlatformAccountLabel: row.PlatformAccountLabel,
		BrowserLabel: row.BrowserLabel, Categories: categories, DefaultCategoryID: row.DefaultCategoryID,
		DefaultVisibility: publishing.Visibility(row.DefaultVisibility), CompatibilityReady: row.CompatibilityReady == 1,
		HermesVersion: row.HermesVersion, LastSeenAt: lastSeen, RevokedAt: revoked, CreatedAt: created, UpdatedAt: updated}, nil
}

func toJob(row sqlc.PublishJob) (publishing.Job, error) {
	created, err := parseTime(row.CreatedAt)
	if err != nil {
		return publishing.Job{}, err
	}
	updated, err := parseTime(row.UpdatedAt)
	if err != nil {
		return publishing.Job{}, err
	}
	claimed, err := parseOptionalTime(row.ClaimedAt)
	if err != nil {
		return publishing.Job{}, err
	}
	committed, err := parseOptionalTime(row.CommittedAt)
	if err != nil {
		return publishing.Job{}, err
	}
	publishedAt, err := parseOptionalTime(row.PublishedAt)
	if err != nil {
		return publishing.Job{}, err
	}
	leaseExpires, err := parseOptionalTime(row.LeaseExpiresAt)
	if err != nil {
		return publishing.Job{}, err
	}
	var settings settingsDTO
	if err := json.Unmarshal([]byte(row.SettingsJson), &settings); err != nil {
		return publishing.Job{}, fmt.Errorf("decode job settings: %w", err)
	}
	var manifest *publishing.Manifest
	if row.ManifestJson.Valid {
		manifest, err = unmarshalManifest(row.ManifestJson.String)
		if err != nil {
			return publishing.Job{}, err
		}
	}
	postCreatedAt, err := parseTime(row.PostCreatedAt)
	if err != nil {
		return publishing.Job{}, err
	}
	return publishing.Job{ID: row.ID, UserID: row.UserID, PostSlug: row.PostSlug, PostCreatedAt: postCreatedAt, AgentID: row.AgentID, Platform: row.Platform,
		Status: publishing.Status(row.Status), Stage: publishing.Stage(row.Stage), ProgressSeq: row.ProgressSeq, Attempt: row.Attempt,
		ContentRevision: row.ContentRevision, Manifest: manifest, CategoryID: settings.CategoryID, Visibility: publishing.Visibility(settings.Visibility),
		LeaseTokenHash: row.LeaseTokenHash.String, LeaseExpiresAt: leaseExpires, ErrorCode: row.ErrorCode.String,
		ErrorMessage: row.ErrorMessage.String, PlatformPostURL: row.PlatformPostUrl.String, CreatedAt: created,
		ClaimedAt: claimed, CommittedAt: committed, PublishedAt: publishedAt, UpdatedAt: updated}, nil
}

type settingsDTO struct {
	CategoryID string `json:"category_id"`
	Visibility string `json:"visibility"`
}

type manifestDTO struct {
	JobID                     string             `json:"job_id"`
	PostSlug                  string             `json:"post_slug"`
	ContentRevision           int64              `json:"content_revision"`
	Content                   contentDTO         `json:"content"`
	Tags                      []string           `json:"tags"`
	CategoryID                string             `json:"category_id"`
	CategoryName              string             `json:"category_name"`
	Visibility                string             `json:"visibility"`
	ExpectedPlatformAccountID string             `json:"expected_platform_account_id"`
	Assets                    []publishing.Asset `json:"assets"`
}

type contentDTO struct {
	Title   string     `json:"title"`
	Summary string     `json:"summary"`
	Tags    []string   `json:"tags"`
	Blocks  []blockDTO `json:"blocks"`
}

type blockDTO struct {
	Type    string   `json:"type"`
	Content string   `json:"content"`
	Level   int32    `json:"level"`
	File    string   `json:"file"`
	Alt     string   `json:"alt"`
	Caption string   `json:"caption"`
	Items   []string `json:"items"`
}

func marshalManifest(manifest *publishing.Manifest) (string, error) {
	if manifest == nil {
		return "", publishing.ErrInvalid
	}
	dto := manifestDTO{JobID: manifest.JobID, PostSlug: manifest.PostSlug, ContentRevision: manifest.ContentRevision,
		Content: toContentDTO(manifest.Content), Tags: manifest.Tags, CategoryID: manifest.CategoryID,
		CategoryName: manifest.CategoryName, Visibility: string(manifest.Visibility), ExpectedPlatformAccountID: manifest.ExpectedPlatformAccountID, Assets: manifest.Assets}
	data, err := json.Marshal(dto)
	return string(data), err
}

func unmarshalManifest(value string) (*publishing.Manifest, error) {
	var dto manifestDTO
	if err := json.Unmarshal([]byte(value), &dto); err != nil {
		return nil, fmt.Errorf("decode job manifest: %w", err)
	}
	return &publishing.Manifest{JobID: dto.JobID, PostSlug: dto.PostSlug, ContentRevision: dto.ContentRevision,
		Content: fromContentDTO(dto.Content), Tags: dto.Tags, CategoryID: dto.CategoryID,
		CategoryName: dto.CategoryName, Visibility: publishing.Visibility(dto.Visibility), ExpectedPlatformAccountID: dto.ExpectedPlatformAccountID, Assets: dto.Assets}, nil
}

func toContentDTO(content publishing.Content) contentDTO {
	blocks := make([]blockDTO, len(content.Blocks))
	for i, block := range content.Blocks {
		blocks[i] = blockDTO{Type: string(block.Type), Content: block.Content, Level: block.Level, File: block.File, Alt: block.Alt, Caption: block.Caption, Items: block.Items}
	}
	return contentDTO{Title: content.Title, Summary: content.Summary, Tags: content.Tags, Blocks: blocks}
}

func fromContentDTO(content contentDTO) publishing.Content {
	blocks := make([]publishing.Block, len(content.Blocks))
	for i, block := range content.Blocks {
		blocks[i] = publishing.Block{Type: publishing.BlockType(block.Type), Content: block.Content, Level: block.Level, File: block.File, Alt: block.Alt, Caption: block.Caption, Items: block.Items}
	}
	return publishing.Content{Title: content.Title, Summary: content.Summary, Tags: content.Tags, Blocks: blocks}
}

func formatTime(value time.Time) string         { return value.UTC().Format(writeLayout) }
func parseTime(value string) (time.Time, error) { return time.Parse(writeLayout, value) }
func parseOptionalTime(value sql.NullString) (*time.Time, error) {
	if !value.Valid || value.String == "" {
		return nil, nil
	}
	parsed, err := parseTime(value.String)
	return &parsed, err
}
func nullableString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}
func isUnique(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "unique constraint failed")
}

var _ publishing.Store = (*Store)(nil)
