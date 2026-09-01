package publishing

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	postpilotv1 "github.com/postpilot/agent/internal/gen/postpilot/v1"
	"github.com/postpilot/agent/internal/workdir"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type API interface {
	Renew(context.Context, string, string) error
	Progress(context.Context, string, string, int64, postpilotv1.PublishStage) error
	Complete(context.Context, string, string, int64, string) error
	Fail(context.Context, string, string, int64, postpilotv1.PublishFailureKind, string) error
}

// Result is the executor-neutral terminal report produced by the local Naver
// publisher. It deliberately contains only the bounded data needed to complete
// or fail the durable server job.
type Result struct {
	Status       string
	PublishedURL string
	FailureKind  string
	Detail       string
}

// Reporter is the narrow in-process protocol between the reviewed Naver driver
// and the durable job executor. Advance is synchronous: returning from the
// committing transition is the only authorization to activate the final control.
type Reporter interface {
	Advance(context.Context, Stage) error
}

type Stage string

const (
	StagePreparing       Stage = "preparing"
	StageOpeningEditor   Stage = "opening_editor"
	StageFillingContent  Stage = "filling_content"
	StageUploadingPhotos Stage = "uploading_photos"
	StageCommitting      Stage = "committing"
	StageVerifying       Stage = "verifying"
)

type Publisher interface {
	Run(context.Context, string, Reporter) (Result, error)
}

type Executor struct {
	API            API
	Publisher      Publisher
	JobsRoot       string
	ConnectionID   string
	HeartbeatEvery time.Duration
	Timeout        time.Duration
	Logger         *slog.Logger
}

func (e Executor) Execute(ctx context.Context, claim *postpilotv1.ClaimPublishJobResponse) error {
	if claim.GetJob() == nil || claim.GetManifest() == nil || claim.GetLeaseToken() == "" {
		return errors.New("claim is incomplete")
	}
	heartbeatEvery := e.heartbeatEvery()
	leaseTTL := time.Duration(claim.GetLeaseTtlSeconds()) * time.Second
	if leaseTTL <= 0 || heartbeatEvery*2 >= leaseTTL {
		return errors.New("claim lease TTL is too short for the configured heartbeat")
	}
	jobID := claim.GetJob().GetId()
	dir, err := workdir.Create(e.JobsRoot, e.ConnectionID, jobID)
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	runTimeout := e.Timeout
	if runTimeout <= 0 {
		runTimeout = 15 * time.Minute
	}
	runCtx, cancel := context.WithTimeout(ctx, runTimeout)
	heartbeatDone := make(chan error, 1)
	go e.heartbeat(runCtx, claim, heartbeatDone, cancel)
	heartbeatStopped := false
	stopHeartbeat := func() error {
		if heartbeatStopped {
			return nil
		}
		cancel()
		heartbeatStopped = true
		return <-heartbeatDone
	}
	defer func() { _ = stopHeartbeat() }()
	if err := downloadAssets(runCtx, dir, claim.GetManifest().GetAssets()); err != nil {
		if renewalErr := stopHeartbeat(); renewalErr != nil {
			return fmt.Errorf("publishing lease renewal failed; leaving the job for server recovery: %w", renewalErr)
		}
		return e.fail(ctx, claim, claim.GetJob().GetProgressSequence()+1, postpilotv1.PublishFailureKind_PUBLISH_FAILURE_ASSET_MISSING, err.Error())
	}
	manifest, err := localManifestBytes(claim.GetManifest())
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), manifest, 0o600); err != nil {
		return err
	}
	reporter := newProgressReporter(e.API, jobID, claim.GetLeaseToken(), claim.GetJob().GetProgressSequence())
	result, runErr := e.Publisher.Run(runCtx, dir, reporter)
	reporter.Close()
	if renewalErr := stopHeartbeat(); renewalErr != nil {
		// Do not race a still-valid lease with FailPublish. The server-owned expiry
		// recovery will safely requeue a pre-commit job or mark a fenced job unknown.
		return fmt.Errorf("publishing lease renewal failed; leaving the job for server recovery: %w", renewalErr)
	}
	sequence, stage := reporter.State()
	if runErr != nil {
		kind := postpilotv1.PublishFailureKind_PUBLISH_FAILURE_SAFE
		if stage >= postpilotv1.PublishStage_PUBLISH_STAGE_COMMITTING {
			kind = postpilotv1.PublishFailureKind_PUBLISH_FAILURE_BROWSER_LOST
		}
		return e.fail(ctx, claim, sequence+1, kind, runErr.Error())
	}
	if result.Status == "published" {
		if stage != postpilotv1.PublishStage_PUBLISH_STAGE_VERIFYING || strings.TrimSpace(result.PublishedURL) == "" {
			return e.fail(ctx, claim, sequence+1, postpilotv1.PublishFailureKind_PUBLISH_FAILURE_BROWSER_LOST, "publisher returned an incomplete verified result")
		}
		return e.API.Complete(ctx, jobID, claim.GetLeaseToken(), sequence+1, result.PublishedURL)
	}
	if result.Status == "failed" {
		return e.fail(ctx, claim, sequence+1, failureKind(result.FailureKind), result.Detail)
	}
	return e.fail(ctx, claim, sequence+1, postpilotv1.PublishFailureKind_PUBLISH_FAILURE_SAFE, "publisher returned an invalid terminal result")
}

// localManifestBytes preserves the immutable manifest but removes the temporary
// signed storage capabilities after the agent has downloaded and verified every
// JPEG. The local publisher receives filenames and local resolver handles only.
func localManifestBytes(manifest *postpilotv1.PublishManifest) ([]byte, error) {
	local, ok := proto.Clone(manifest).(*postpilotv1.PublishManifest)
	if !ok || local == nil {
		return nil, errors.New("manifest is unavailable")
	}
	for _, asset := range local.GetAssets() {
		asset.DownloadUrl = ""
	}
	return protojson.MarshalOptions{UseProtoNames: true}.Marshal(local)
}

func (e Executor) fail(ctx context.Context, claim *postpilotv1.ClaimPublishJobResponse, sequence int64, kind postpilotv1.PublishFailureKind, detail string) error {
	logger := e.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.Error(
		"publisher reported a local failure",
		"job_id", claim.GetJob().GetId(),
		"failure_kind", kind.String(),
		"detail", redactedLocalDetail(detail),
	)
	// The RPC carries only bounded redaction metadata, never model/browser text, local
	// paths, post content, browser endpoints, or driver diagnostics. The server maps
	// the constrained kind to a stable reason and retains this inert value as optional
	// technical detail.
	return e.API.Fail(ctx, claim.GetJob().GetId(), claim.GetLeaseToken(), sequence, kind, redactedLocalDetail(detail))
}

func (e Executor) heartbeatEvery() time.Duration {
	every := e.HeartbeatEvery
	if every <= 0 {
		return 10 * time.Second
	}
	return every
}

func (e Executor) heartbeat(ctx context.Context, claim *postpilotv1.ClaimPublishJobResponse, done chan<- error, cancel context.CancelFunc) {
	ticker := time.NewTicker(e.heartbeatEvery())
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			done <- nil
			return
		case <-ticker.C:
			if err := e.API.Renew(ctx, claim.GetJob().GetId(), claim.GetLeaseToken()); err != nil {
				if ctx.Err() != nil {
					done <- nil
					return
				}
				logger := e.Logger
				if logger == nil {
					logger = slog.Default()
				}
				logger.Error("publishing lease renewal failed", "job_id", claim.GetJob().GetId())
				cancel()
				done <- err
				return
			}
		}
	}
}

func redactedLocalDetail(detail string) string {
	detail = strings.TrimSpace(detail)
	if detail == "" {
		return ""
	}
	// Raw publisher/browser diagnostics can contain post text, URLs and local paths.
	// Record that a diagnostic existed, and its bounded size for troubleshooting,
	// without reproducing any of those values in logs.
	return fmt.Sprintf("redacted (%d bytes)", min(len(detail), 10_000))
}

func downloadAssets(ctx context.Context, dir string, assets []*postpilotv1.StagedPublishAsset) error {
	client := &http.Client{Timeout: 2 * time.Minute}
	client.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if len(via) == 0 || request.URL.Scheme != via[0].URL.Scheme || !strings.EqualFold(request.URL.Host, via[0].URL.Host) {
			return errors.New("signed asset redirect changed origin")
		}
		return nil
	}
	seen := make(map[string]struct{}, len(assets))
	for ordinal, asset := range assets {
		if asset.GetBytes() <= 0 {
			return errors.New("asset has invalid byte count")
		}
		if asset.GetOrdinal() != int32(ordinal) {
			return errors.New("asset ordinals are not contiguous")
		}
		extension := strings.ToLower(filepath.Ext(asset.GetFilename()))
		if extension != ".jpg" && extension != ".jpeg" {
			return errors.New("asset is not a JPEG")
		}
		if _, exists := seen[asset.GetFilename()]; exists {
			return errors.New("duplicate asset filename")
		}
		seen[asset.GetFilename()] = struct{}{}
		target, err := workdir.SafeJoin(dir, asset.GetFilename())
		if err != nil {
			return err
		}
		u, err := url.Parse(asset.GetDownloadUrl())
		if err != nil || u.Host == "" || (u.Scheme != "https" && !(u.Scheme == "http" && isLoopback(u.Hostname()))) {
			return errors.New("asset URL is not an allowed signed origin")
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return err
		}
		response, err := client.Do(request)
		if err != nil {
			return err
		}
		if response.StatusCode != http.StatusOK {
			response.Body.Close()
			return fmt.Errorf("asset download returned %d", response.StatusCode)
		}
		file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			response.Body.Close()
			return err
		}
		written, copyErr := io.CopyN(file, response.Body, asset.GetBytes()+1)
		closeErr := file.Close()
		response.Body.Close()
		if copyErr != nil && !errors.Is(copyErr, io.EOF) {
			return copyErr
		}
		if closeErr != nil || written != asset.GetBytes() {
			return errors.New("asset byte count does not match manifest")
		}
	}
	return nil
}

type progressReporter struct {
	api        API
	jobID      string
	leaseToken string
	mu         sync.Mutex
	sequence   int64
	stage      postpilotv1.PublishStage
	closed     bool
}

func newProgressReporter(api API, jobID, leaseToken string, sequence int64) *progressReporter {
	return &progressReporter{
		api: api, jobID: jobID, leaseToken: leaseToken, sequence: sequence,
		stage: postpilotv1.PublishStage_PUBLISH_STAGE_CLAIMED,
	}
}

func (s *progressReporter) State() (int64, postpilotv1.PublishStage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sequence, s.stage
}

func (s *progressReporter) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
}

func (s *progressReporter) Advance(ctx context.Context, stage Stage) error {
	next, ok := publisherStages[stage]
	if !ok {
		return errors.New("publisher reported an invalid stage")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("publisher reported progress after returning a terminal result")
	}
	if next != s.stage+1 {
		return errors.New("publisher reported non-monotonic progress")
	}
	sequence := s.sequence + 1
	// In particular, committing is persisted by this synchronous call before the
	// driver returns from Advance and is allowed to activate Naver's final control.
	if err := s.api.Progress(ctx, s.jobID, s.leaseToken, sequence, next); err != nil {
		return fmt.Errorf("server rejected publisher progress: %w", err)
	}
	s.sequence, s.stage = sequence, next
	return nil
}

var publisherStages = map[Stage]postpilotv1.PublishStage{
	StagePreparing:       postpilotv1.PublishStage_PUBLISH_STAGE_PREPARING,
	StageOpeningEditor:   postpilotv1.PublishStage_PUBLISH_STAGE_OPENING_EDITOR,
	StageFillingContent:  postpilotv1.PublishStage_PUBLISH_STAGE_FILLING_CONTENT,
	StageUploadingPhotos: postpilotv1.PublishStage_PUBLISH_STAGE_UPLOADING_PHOTOS,
	StageCommitting:      postpilotv1.PublishStage_PUBLISH_STAGE_COMMITTING,
	StageVerifying:       postpilotv1.PublishStage_PUBLISH_STAGE_VERIFYING,
}

func failureKind(value string) postpilotv1.PublishFailureKind {
	switch value {
	case "login_expired":
		return postpilotv1.PublishFailureKind_PUBLISH_FAILURE_LOGIN_EXPIRED
	case "captcha":
		return postpilotv1.PublishFailureKind_PUBLISH_FAILURE_CAPTCHA
	case "two_factor":
		return postpilotv1.PublishFailureKind_PUBLISH_FAILURE_TWO_FACTOR
	case "account_mismatch":
		return postpilotv1.PublishFailureKind_PUBLISH_FAILURE_ACCOUNT_MISMATCH
	case "editor_changed":
		return postpilotv1.PublishFailureKind_PUBLISH_FAILURE_EDITOR_CHANGED
	case "asset_missing":
		return postpilotv1.PublishFailureKind_PUBLISH_FAILURE_ASSET_MISSING
	default:
		return postpilotv1.PublishFailureKind_PUBLISH_FAILURE_SAFE
	}
}

func isLoopback(host string) bool { return host == "localhost" || host == "127.0.0.1" || host == "::1" }
