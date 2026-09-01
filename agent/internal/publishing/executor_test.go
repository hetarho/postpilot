package publishing

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	postpilotv1 "github.com/postpilot/agent/internal/gen/postpilot/v1"
)

type progressCall struct {
	sequence int64
	stage    postpilotv1.PublishStage
}
type fakeAPI struct {
	calls       []progressCall
	failDetails []string
	failKinds   []postpilotv1.PublishFailureKind
	completed   []string
}

func (f *fakeAPI) Renew(context.Context, string, string) error { return nil }
func (f *fakeAPI) Progress(_ context.Context, _, _ string, sequence int64, stage postpilotv1.PublishStage) error {
	f.calls = append(f.calls, progressCall{sequence, stage})
	return nil
}
func (f *fakeAPI) Complete(_ context.Context, _, _ string, _ int64, url string) error {
	f.completed = append(f.completed, url)
	return nil
}
func (f *fakeAPI) Fail(_ context.Context, _, _ string, _ int64, kind postpilotv1.PublishFailureKind, detail string) error {
	f.failDetails = append(f.failDetails, detail)
	f.failKinds = append(f.failKinds, kind)
	return nil
}

type publisherFunc func(context.Context, string, Reporter) (Result, error)

func (f publisherFunc) Run(ctx context.Context, dir string, reporter Reporter) (Result, error) {
	return f(ctx, dir, reporter)
}

type renewalAPI struct {
	renewed chan struct{}
	once    sync.Once
}

func (f *renewalAPI) Renew(context.Context, string, string) error {
	f.once.Do(func() { close(f.renewed) })
	return nil
}
func (*renewalAPI) Progress(context.Context, string, string, int64, postpilotv1.PublishStage) error {
	return nil
}
func (*renewalAPI) Complete(context.Context, string, string, int64, string) error { return nil }
func (*renewalAPI) Fail(context.Context, string, string, int64, postpilotv1.PublishFailureKind, string) error {
	return nil
}

func TestReporterPersistsCommitFenceBeforeReturning(t *testing.T) {
	api := &fakeAPI{}
	reporter := newProgressReporter(api, "job", "lease", 1)
	stages := []Stage{StagePreparing, StageOpeningEditor, StageFillingContent, StageUploadingPhotos, StageCommitting, StageVerifying}
	for _, stage := range stages {
		if err := reporter.Advance(context.Background(), stage); err != nil {
			t.Fatal(err)
		}
		if got := api.calls[len(api.calls)-1].stage; got != publisherStages[stage] {
			t.Fatalf("acknowledged before persistence: got %v", got)
		}
	}
	if api.calls[4].stage != postpilotv1.PublishStage_PUBLISH_STAGE_COMMITTING {
		t.Fatal("commit fence was not the fifth durable transition")
	}
}

func TestReporterRejectsSkippedAndPostTerminalProgress(t *testing.T) {
	api := &fakeAPI{}
	reporter := newProgressReporter(api, "job", "lease", 1)
	if err := reporter.Advance(context.Background(), StageCommitting); err == nil {
		t.Fatal("skipped progress was accepted")
	}
	if len(api.calls) != 0 {
		t.Fatal("rejected transitions reached API")
	}
	if err := reporter.Advance(context.Background(), StagePreparing); err != nil {
		t.Fatal(err)
	}
	reporter.Close()
	if err := reporter.Advance(context.Background(), StageOpeningEditor); err == nil {
		t.Fatal("progress after terminal return was accepted")
	}
	if len(api.calls) != 1 {
		t.Fatalf("post-terminal progress reached API: %+v", api.calls)
	}
}

func advanceAllStages(t *testing.T, ctx context.Context, reporter Reporter) {
	t.Helper()
	for _, stage := range []Stage{StagePreparing, StageOpeningEditor, StageFillingContent, StageUploadingPhotos, StageCommitting, StageVerifying} {
		if err := reporter.Advance(ctx, stage); err != nil {
			t.Fatalf("progress %s: %v", stage, err)
		}
	}
}

func TestExecutorUsesTypedPublisherResultAfterReadback(t *testing.T) {
	api := &fakeAPI{}
	claim := &postpilotv1.ClaimPublishJobResponse{
		Job: &postpilotv1.PublishJob{Id: "job", ProgressSequence: 1}, Manifest: &postpilotv1.PublishManifest{},
		LeaseToken: "lease", LeaseTtlSeconds: 45,
	}
	executor := Executor{
		API: api, JobsRoot: t.TempDir(), ConnectionID: "connection", HeartbeatEvery: time.Second, Timeout: time.Second,
		Publisher: publisherFunc(func(ctx context.Context, _ string, reporter Reporter) (Result, error) {
			advanceAllStages(t, ctx, reporter)
			return Result{Status: "published", PublishedURL: "https://blog.naver.com/alice/real"}, nil
		}),
	}
	if err := executor.Execute(context.Background(), claim); err != nil {
		t.Fatal(err)
	}
	if len(api.completed) != 1 || api.completed[0] != "https://blog.naver.com/alice/real" || len(api.failDetails) != 0 {
		t.Fatalf("typed result was not authoritative: completed=%v failures=%v", api.completed, api.failDetails)
	}
}

func TestExecutorReportsSafeFailureWhenPublisherDiesBeforeCommitFence(t *testing.T) {
	api := &fakeAPI{}
	claim := &postpilotv1.ClaimPublishJobResponse{
		Job: &postpilotv1.PublishJob{Id: "job", ProgressSequence: 1}, Manifest: &postpilotv1.PublishManifest{},
		LeaseToken: "lease", LeaseTtlSeconds: 45,
	}
	executor := Executor{
		API: api, JobsRoot: t.TempDir(), ConnectionID: "connection", HeartbeatEvery: time.Second, Timeout: time.Second,
		Publisher: publisherFunc(func(ctx context.Context, _ string, reporter Reporter) (Result, error) {
			for _, stage := range []Stage{StagePreparing, StageOpeningEditor} {
				if err := reporter.Advance(ctx, stage); err != nil {
					t.Fatal(err)
				}
			}
			return Result{}, errors.New("publisher crashed")
		}),
	}
	if err := executor.Execute(context.Background(), claim); err != nil {
		t.Fatal(err)
	}
	if len(api.failKinds) != 1 || api.failKinds[0] != postpilotv1.PublishFailureKind_PUBLISH_FAILURE_SAFE {
		t.Fatalf("pre-fence exit kinds = %v", api.failKinds)
	}
	if len(api.failDetails) != 1 || api.failDetails[0] != "redacted (17 bytes)" {
		t.Fatalf("failure detail = %#v, want bounded redaction metadata", api.failDetails)
	}
	if strings.Contains(api.failDetails[0], "publisher crashed") {
		t.Fatalf("raw local detail crossed the RPC boundary: %q", api.failDetails[0])
	}
}

func TestRedactedLocalDetailKeepsEmptyDiagnosticsAbsent(t *testing.T) {
	if got := redactedLocalDetail(" \n\t "); got != "" {
		t.Fatalf("empty diagnostic = %q", got)
	}
}

func TestExecutorRejectsPublishedResultBeforeReadback(t *testing.T) {
	api := &fakeAPI{}
	claim := &postpilotv1.ClaimPublishJobResponse{
		Job: &postpilotv1.PublishJob{Id: "job", ProgressSequence: 1}, Manifest: &postpilotv1.PublishManifest{},
		LeaseToken: "lease", LeaseTtlSeconds: 45,
	}
	executor := Executor{
		API: api, JobsRoot: t.TempDir(), ConnectionID: "connection", HeartbeatEvery: time.Second, Timeout: time.Second,
		Publisher: publisherFunc(func(ctx context.Context, _ string, reporter Reporter) (Result, error) {
			if err := reporter.Advance(ctx, StagePreparing); err != nil {
				t.Fatal(err)
			}
			return Result{Status: "published", PublishedURL: "https://blog.naver.com/alice/unverified"}, nil
		}),
	}
	if err := executor.Execute(context.Background(), claim); err != nil {
		t.Fatal(err)
	}
	if len(api.completed) != 0 || len(api.failKinds) != 1 || api.failKinds[0] != postpilotv1.PublishFailureKind_PUBLISH_FAILURE_BROWSER_LOST {
		t.Fatalf("unverified result completed=%v failure kinds=%v", api.completed, api.failKinds)
	}
}

func TestDownloadAssetsVerifiesBytesAndRedirectOrigin(t *testing.T) {
	payload := []byte("jpeg")
	storage := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { writer.Write(payload) }))
	defer storage.Close()
	dir := t.TempDir()
	err := downloadAssets(context.Background(), dir, []*postpilotv1.StagedPublishAsset{{Filename: "0000.jpg", DownloadUrl: storage.URL + "/signed", Bytes: int64(len(payload))}})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "0000.jpg"))
	if err != nil || !bytes.Equal(data, payload) {
		t.Fatalf("downloaded bytes = %q, %v", data, err)
	}
	other := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { writer.Write(payload) }))
	defer other.Close()
	redirect := httptest.NewServer(http.RedirectHandler(other.URL, http.StatusFound))
	defer redirect.Close()
	err = downloadAssets(context.Background(), t.TempDir(), []*postpilotv1.StagedPublishAsset{{Filename: "0000.jpg", DownloadUrl: redirect.URL, Bytes: int64(len(payload))}})
	if err == nil {
		t.Fatal("cross-origin redirect was accepted")
	}

	err = downloadAssets(context.Background(), t.TempDir(), []*postpilotv1.StagedPublishAsset{{Ordinal: 1, Filename: "0000.jpg", DownloadUrl: storage.URL, Bytes: int64(len(payload))}})
	if err == nil {
		t.Fatal("non-contiguous asset ordinal was accepted")
	}
	err = downloadAssets(context.Background(), t.TempDir(), []*postpilotv1.StagedPublishAsset{{Filename: "0000.png", DownloadUrl: storage.URL, Bytes: int64(len(payload))}})
	if err == nil {
		t.Fatal("non-JPEG staged asset was accepted")
	}
}

func TestLocalManifestRemovesSignedStorageCapabilities(t *testing.T) {
	original := &postpilotv1.PublishManifest{Assets: []*postpilotv1.StagedPublishAsset{{
		Ordinal: 0, Filename: "0000.jpg", DownloadUrl: "https://storage.test/private?signature=secret", Bytes: 4,
	}}}
	data, err := localManifestBytes(original)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("storage.test")) || bytes.Contains(data, []byte("secret")) {
		t.Fatalf("signed URL reached local publisher manifest: %s", data)
	}
	if original.GetAssets()[0].GetDownloadUrl() == "" {
		t.Fatal("local manifest sanitization mutated the server claim")
	}
}

func TestLeaseRenewsWhileAssetsAreStillDownloading(t *testing.T) {
	api := &renewalAPI{renewed: make(chan struct{})}
	storage := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		select {
		case <-api.renewed:
			_, _ = writer.Write([]byte("jpeg"))
		case <-request.Context().Done():
		}
	}))
	defer storage.Close()
	claim := &postpilotv1.ClaimPublishJobResponse{
		Job:             &postpilotv1.PublishJob{Id: "job", ProgressSequence: 1},
		LeaseToken:      "lease",
		LeaseTtlSeconds: 45,
		Manifest: &postpilotv1.PublishManifest{Assets: []*postpilotv1.StagedPublishAsset{{
			Ordinal: 0, Filename: "0000.jpg", DownloadUrl: storage.URL + "/signed", Bytes: 4,
		}}},
	}
	executor := Executor{
		API: api, JobsRoot: t.TempDir(), ConnectionID: "connection", HeartbeatEvery: 5 * time.Millisecond,
		Timeout: time.Second,
		Publisher: publisherFunc(func(context.Context, string, Reporter) (Result, error) {
			return Result{Status: "failed", FailureKind: "safe"}, nil
		}),
	}
	if err := executor.Execute(context.Background(), claim); err != nil {
		t.Fatal(err)
	}
	select {
	case <-api.renewed:
	default:
		t.Fatal("lease did not renew before asset download completed")
	}
}

func TestExecutorRefusesUnsafeAdvertisedLeaseBeforeRunningPublisher(t *testing.T) {
	called := false
	executor := Executor{
		API: &fakeAPI{}, JobsRoot: t.TempDir(), ConnectionID: "connection", HeartbeatEvery: 10 * time.Second,
		Publisher: publisherFunc(func(context.Context, string, Reporter) (Result, error) {
			called = true
			return Result{}, nil
		}),
	}
	claim := &postpilotv1.ClaimPublishJobResponse{
		Job: &postpilotv1.PublishJob{Id: "job"}, Manifest: &postpilotv1.PublishManifest{},
		LeaseToken: "lease", LeaseTtlSeconds: 20,
	}
	if err := executor.Execute(context.Background(), claim); err == nil {
		t.Fatal("unsafe claim lease was accepted")
	}
	if called {
		t.Fatal("publisher ran for an unsafe claim lease")
	}
}

type failedRenewalAPI struct {
	fakeAPI
	renewed chan struct{}
	once    sync.Once
}

func (f *failedRenewalAPI) Renew(context.Context, string, string) error {
	f.once.Do(func() { close(f.renewed) })
	return io.ErrUnexpectedEOF
}

func TestLeaseRenewalFailureCancelsPublisherAndLeavesServerToRecoverLease(t *testing.T) {
	api := &failedRenewalAPI{renewed: make(chan struct{})}
	var logs bytes.Buffer
	executor := Executor{
		API: api, JobsRoot: t.TempDir(), ConnectionID: "connection", HeartbeatEvery: 5 * time.Millisecond,
		Timeout: time.Second, Logger: slog.New(slog.NewTextHandler(&logs, nil)),
		Publisher: publisherFunc(func(ctx context.Context, _ string, _ Reporter) (Result, error) {
			<-ctx.Done()
			return Result{}, errors.New("secret post text /Users/alice/private.jpg")
		}),
	}
	claim := &postpilotv1.ClaimPublishJobResponse{
		Job: &postpilotv1.PublishJob{Id: "job", ProgressSequence: 1}, Manifest: &postpilotv1.PublishManifest{},
		LeaseToken: "lease", LeaseTtlSeconds: 45,
	}
	if err := executor.Execute(context.Background(), claim); err == nil || !strings.Contains(err.Error(), "server recovery") {
		t.Fatalf("renewal failure error = %v", err)
	}
	select {
	case <-api.renewed:
	default:
		t.Fatal("renewal failure path did not run")
	}
	if len(api.failDetails) != 0 {
		t.Fatalf("renewal failure raced server recovery with FailPublish: %#v", api.failDetails)
	}
	if strings.Contains(logs.String(), "secret post text") || strings.Contains(logs.String(), "/Users/alice") {
		t.Fatalf("raw failure detail reached local log: %s", logs.String())
	}
	if !strings.Contains(logs.String(), "lease renewal failed") {
		t.Fatalf("renewal failure was not logged safely: %s", logs.String())
	}
}
