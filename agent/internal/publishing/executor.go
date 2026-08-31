package publishing

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	postpilotv1 "github.com/postpilot/agent/internal/gen/postpilot/v1"
	"github.com/postpilot/agent/internal/hermes"
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

type Publisher interface {
	Run(context.Context, string, string, string, string) (hermes.Result, error)
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
	handle, token, err := randomPair()
	if err != nil {
		return err
	}
	callback, err := newCallback(e.API, jobID, claim.GetLeaseToken(), token, claim.GetJob().GetProgressSequence())
	if err != nil {
		return err
	}
	defer callback.Close()
	_, runErr := e.Publisher.Run(runCtx, handle, dir, callback.URL(), token)
	if renewalErr := stopHeartbeat(); renewalErr != nil {
		// Do not race a still-valid lease with FailPublish. The server-owned expiry
		// recovery will safely requeue a pre-commit job or mark a fenced job unknown.
		return fmt.Errorf("publishing lease renewal failed; leaving the job for server recovery: %w", renewalErr)
	}
	sequence, stage := callback.State()
	result, reported := callback.Result()
	if !reported {
		kind := postpilotv1.PublishFailureKind_PUBLISH_FAILURE_SAFE
		if stage >= postpilotv1.PublishStage_PUBLISH_STAGE_COMMITTING {
			kind = postpilotv1.PublishFailureKind_PUBLISH_FAILURE_BROWSER_LOST
		}
		detail := "publisher exited without an authenticated terminal report"
		if runErr != nil {
			detail = runErr.Error()
		}
		return e.fail(ctx, claim, sequence+1, kind, detail)
	}
	if result.Status == "published" {
		if stage != postpilotv1.PublishStage_PUBLISH_STAGE_VERIFYING {
			return e.fail(ctx, claim, sequence+1, postpilotv1.PublishFailureKind_PUBLISH_FAILURE_BROWSER_LOST, "publisher did not reach verifying")
		}
		return e.API.Complete(ctx, jobID, claim.GetLeaseToken(), sequence+1, result.PublishedURL)
	}
	return e.fail(ctx, claim, sequence+1, failureKind(result.FailureKind), result.Detail)
}

// localManifestBytes preserves the immutable manifest but removes the temporary
// signed storage capabilities after the agent has downloaded and verified every
// JPEG. Hermes receives filenames and local resolver handles only.
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
	// paths, post content, callback addresses, or provider diagnostics. The server maps
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
	// Raw Hermes/browser diagnostics can contain post text, URLs and local paths.
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

type callbackServer struct {
	api        API
	jobID      string
	leaseToken string
	token      string
	listener   net.Listener
	server     *http.Server
	mu         sync.Mutex
	sequence   int64
	stage      postpilotv1.PublishStage
	result     *hermes.Result
}

func newCallback(api API, jobID, leaseToken, token string, sequence int64) (*callbackServer, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	callback := &callbackServer{api: api, jobID: jobID, leaseToken: leaseToken, token: token, listener: listener, sequence: sequence, stage: postpilotv1.PublishStage_PUBLISH_STAGE_CLAIMED}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /progress", callback.progress)
	mux.HandleFunc("POST /finish", callback.finish)
	callback.server = &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go callback.server.Serve(listener)
	return callback, nil
}

func (s *callbackServer) URL() string { return "http://" + s.listener.Addr().String() }

func (s *callbackServer) Close() { _ = s.server.Close() }

func (s *callbackServer) State() (int64, postpilotv1.PublishStage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sequence, s.stage
}

func (s *callbackServer) Result() (hermes.Result, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.result == nil {
		return hermes.Result{}, false
	}
	return *s.result, true
}

func (s *callbackServer) progress(writer http.ResponseWriter, request *http.Request) {
	if request.Header.Get("Authorization") != "Bearer "+s.token {
		http.Error(writer, "unauthorized", http.StatusUnauthorized)
		return
	}
	var body struct {
		Stage string `json:"stage"`
	}
	decoder := json.NewDecoder(io.LimitReader(request.Body, 1024))
	if decoder.Decode(&body) != nil {
		http.Error(writer, "invalid JSON", http.StatusBadRequest)
		return
	}
	next, ok := callbackStages[body.Stage]
	if !ok {
		http.Error(writer, "invalid stage", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if next != s.stage+1 {
		http.Error(writer, "non-monotonic stage", http.StatusConflict)
		return
	}
	sequence := s.sequence + 1
	// In particular, committing is persisted by this synchronous call before the
	// plugin receives 204 and is allowed to activate Naver's final control.
	if err := s.api.Progress(request.Context(), s.jobID, s.leaseToken, sequence, next); err != nil {
		http.Error(writer, "server rejected progress", http.StatusConflict)
		return
	}
	s.sequence, s.stage = sequence, next
	writer.WriteHeader(http.StatusNoContent)
}

func (s *callbackServer) finish(writer http.ResponseWriter, request *http.Request) {
	if request.Header.Get("Authorization") != "Bearer "+s.token {
		http.Error(writer, "unauthorized", http.StatusUnauthorized)
		return
	}
	var result hermes.Result
	decoder := json.NewDecoder(io.LimitReader(request.Body, 2048))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&result) != nil {
		http.Error(writer, "invalid JSON", http.StatusBadRequest)
		return
	}
	if result.Status != "published" && result.Status != "failed" {
		http.Error(writer, "invalid terminal status", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.result != nil {
		http.Error(writer, "terminal result already recorded", http.StatusConflict)
		return
	}
	if result.Status == "published" {
		if s.stage != postpilotv1.PublishStage_PUBLISH_STAGE_VERIFYING || result.PublishedURL == "" {
			http.Error(writer, "published result requires verified readback", http.StatusConflict)
			return
		}
		result.FailureKind, result.Detail = "", ""
	} else {
		result.PublishedURL = ""
		result.Detail = truncateRunes(result.Detail, 500)
	}
	s.result = &result
	writer.WriteHeader(http.StatusNoContent)
}

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

var callbackStages = map[string]postpilotv1.PublishStage{
	"preparing":        postpilotv1.PublishStage_PUBLISH_STAGE_PREPARING,
	"opening_editor":   postpilotv1.PublishStage_PUBLISH_STAGE_OPENING_EDITOR,
	"filling_content":  postpilotv1.PublishStage_PUBLISH_STAGE_FILLING_CONTENT,
	"uploading_photos": postpilotv1.PublishStage_PUBLISH_STAGE_UPLOADING_PHOTOS,
	"committing":       postpilotv1.PublishStage_PUBLISH_STAGE_COMMITTING,
	"verifying":        postpilotv1.PublishStage_PUBLISH_STAGE_VERIFYING,
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

func randomPair() (string, string, error) {
	values := make([]string, 2)
	for i := range values {
		bytes := make([]byte, 24)
		if _, err := rand.Read(bytes); err != nil {
			return "", "", err
		}
		values[i] = hex.EncodeToString(bytes)
	}
	return values[0], values[1], nil
}

func isLoopback(host string) bool { return host == "localhost" || host == "127.0.0.1" || host == "::1" }
