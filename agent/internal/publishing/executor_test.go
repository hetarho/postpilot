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
	"github.com/postpilot/agent/internal/hermes"
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

type publisherFunc func(context.Context, string, string, string, string) (hermes.Result, error)

func (f publisherFunc) Run(ctx context.Context, handle, dir, callback, token string) (hermes.Result, error) {
	return f(ctx, handle, dir, callback, token)
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

func TestCallbackPersistsCommitFenceBeforeAcknowledging(t *testing.T) {
	api := &fakeAPI{}
	callback, err := newCallback(api, "job", "lease", "secret", 1)
	if err != nil {
		t.Fatal(err)
	}
	defer callback.Close()
	stages := []string{"preparing", "opening_editor", "filling_content", "uploading_photos", "committing", "verifying"}
	for _, stage := range stages {
		body := bytes.NewBufferString(`{"stage":"` + stage + `"}`)
		request, _ := http.NewRequest(http.MethodPost, callback.URL()+"/progress", body)
		request.Header.Set("Authorization", "Bearer secret")
		request.Header.Set("Content-Type", "application/json")
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusNoContent {
			t.Fatalf("%s = %d", stage, response.StatusCode)
		}
		if got := api.calls[len(api.calls)-1].stage; got != callbackStages[stage] {
			t.Fatalf("acknowledged before persistence: got %v", got)
		}
	}
	if api.calls[4].stage != postpilotv1.PublishStage_PUBLISH_STAGE_COMMITTING {
		t.Fatal("commit fence was not the fifth durable transition")
	}
}

func TestCallbackRejectsUnauthorizedAndSkippedProgress(t *testing.T) {
	api := &fakeAPI{}
	callback, err := newCallback(api, "job", "lease", "secret", 1)
	if err != nil {
		t.Fatal(err)
	}
	defer callback.Close()
	for _, test := range []struct {
		token, stage string
		status       int
	}{{"wrong", "preparing", 401}, {"secret", "committing", 409}} {
		request, _ := http.NewRequest(http.MethodPost, callback.URL()+"/progress", bytes.NewBufferString(`{"stage":"`+test.stage+`"}`))
		request.Header.Set("Authorization", "Bearer "+test.token)
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, response.Body)
		response.Body.Close()
		if response.StatusCode != test.status {
			t.Fatalf("%s status = %d", test.stage, response.StatusCode)
		}
	}
	if len(api.calls) != 0 {
		t.Fatal("rejected transitions reached API")
	}
}

func postLocalCallback(t *testing.T, callbackURL, token, path, body string) int {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, callbackURL+path, bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, response.Body)
	response.Body.Close()
	return response.StatusCode
}

func reportAllStages(t *testing.T, callbackURL, token string) {
	t.Helper()
	for _, stage := range []string{"preparing", "opening_editor", "filling_content", "uploading_photos", "committing", "verifying"} {
		if status := postLocalCallback(t, callbackURL, token, "/progress", `{"stage":"`+stage+`"}`); status != http.StatusNoContent {
			t.Fatalf("progress %s status = %d", stage, status)
		}
	}
}

func TestCallbackRecordsOnlyAuthenticatedTerminalResultAfterReadback(t *testing.T) {
	callback, err := newCallback(&fakeAPI{}, "job", "lease", "secret", 1)
	if err != nil {
		t.Fatal(err)
	}
	defer callback.Close()
	if status := postLocalCallback(t, callback.URL(), "wrong", "/finish", `{"status":"failed"}`); status != http.StatusUnauthorized {
		t.Fatalf("unauthorized finish status = %d", status)
	}
	if status := postLocalCallback(t, callback.URL(), "secret", "/finish", `{"status":"published","published_url":"https://blog.naver.com/alice/1"}`); status != http.StatusConflict {
		t.Fatalf("pre-readback finish status = %d", status)
	}
	reportAllStages(t, callback.URL(), "secret")
	if status := postLocalCallback(t, callback.URL(), "secret", "/finish", `{"status":"published","published_url":"https://blog.naver.com/alice/1"}`); status != http.StatusNoContent {
		t.Fatalf("verified finish status = %d", status)
	}
	result, ok := callback.Result()
	if !ok || result.Status != "published" || result.PublishedURL != "https://blog.naver.com/alice/1" {
		t.Fatalf("terminal result = %+v, %v", result, ok)
	}
}

func TestExecutorIgnoresPublisherReturnUntilAuthenticatedFinish(t *testing.T) {
	api := &fakeAPI{}
	claim := &postpilotv1.ClaimPublishJobResponse{
		Job: &postpilotv1.PublishJob{Id: "job", ProgressSequence: 1}, Manifest: &postpilotv1.PublishManifest{},
		LeaseToken: "lease", LeaseTtlSeconds: 45,
	}
	executor := Executor{
		API: api, JobsRoot: t.TempDir(), ConnectionID: "connection", HeartbeatEvery: time.Second, Timeout: time.Second,
		Publisher: publisherFunc(func(_ context.Context, _, _, callbackURL, token string) (hermes.Result, error) {
			reportAllStages(t, callbackURL, token)
			return hermes.Result{Status: "published", PublishedURL: "https://blog.naver.com/alice/invented"}, nil
		}),
	}
	if err := executor.Execute(context.Background(), claim); err != nil {
		t.Fatal(err)
	}
	if len(api.completed) != 0 || len(api.failDetails) != 1 {
		t.Fatalf("stdout result was accepted: completed=%v failures=%v", api.completed, api.failDetails)
	}
	if api.failKinds[0] != postpilotv1.PublishFailureKind_PUBLISH_FAILURE_BROWSER_LOST {
		t.Fatalf("post-fence exit kind = %v", api.failKinds[0])
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
		Publisher: publisherFunc(func(_ context.Context, _, _, callbackURL, token string) (hermes.Result, error) {
			for _, stage := range []string{"preparing", "opening_editor"} {
				if status := postLocalCallback(t, callbackURL, token, "/progress", `{"stage":"`+stage+`"}`); status != http.StatusNoContent {
					t.Fatalf("progress %s status = %d", stage, status)
				}
			}
			return hermes.Result{}, errors.New("publisher crashed")
		}),
	}
	if err := executor.Execute(context.Background(), claim); err != nil {
		t.Fatal(err)
	}
	if len(api.failKinds) != 1 || api.failKinds[0] != postpilotv1.PublishFailureKind_PUBLISH_FAILURE_SAFE {
		t.Fatalf("pre-fence exit kinds = %v", api.failKinds)
	}
}

func TestExecutorUsesAuthenticatedFinishInsteadOfPublisherReturn(t *testing.T) {
	api := &fakeAPI{}
	claim := &postpilotv1.ClaimPublishJobResponse{
		Job: &postpilotv1.PublishJob{Id: "job", ProgressSequence: 1}, Manifest: &postpilotv1.PublishManifest{},
		LeaseToken: "lease", LeaseTtlSeconds: 45,
	}
	executor := Executor{
		API: api, JobsRoot: t.TempDir(), ConnectionID: "connection", HeartbeatEvery: time.Second, Timeout: time.Second,
		Publisher: publisherFunc(func(_ context.Context, _, _, callbackURL, token string) (hermes.Result, error) {
			reportAllStages(t, callbackURL, token)
			if status := postLocalCallback(t, callbackURL, token, "/finish", `{"status":"published","published_url":"https://blog.naver.com/alice/real"}`); status != http.StatusNoContent {
				t.Fatalf("finish status = %d", status)
			}
			return hermes.Result{Status: "failed", FailureKind: "safe"}, nil
		}),
	}
	if err := executor.Execute(context.Background(), claim); err != nil {
		t.Fatal(err)
	}
	if len(api.completed) != 1 || api.completed[0] != "https://blog.naver.com/alice/real" || len(api.failDetails) != 0 {
		t.Fatalf("authenticated result was not authoritative: completed=%v failures=%v", api.completed, api.failDetails)
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
		t.Fatalf("signed URL reached Hermes manifest: %s", data)
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
		Publisher: publisherFunc(func(context.Context, string, string, string, string) (hermes.Result, error) {
			return hermes.Result{Status: "failed", FailureKind: "safe"}, nil
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
		Publisher: publisherFunc(func(context.Context, string, string, string, string) (hermes.Result, error) {
			called = true
			return hermes.Result{}, nil
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
		Publisher: publisherFunc(func(ctx context.Context, _, _, _, _ string) (hermes.Result, error) {
			<-ctx.Done()
			return hermes.Result{}, errors.New("secret post text /Users/alice/private.jpg")
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
