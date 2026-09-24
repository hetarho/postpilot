package store_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	clipapp "github.com/postpilot/backend/internal/clip/app"
	cliprpc "github.com/postpilot/backend/internal/clip/rpc"
	"github.com/postpilot/backend/internal/clip/workerclient"
)

// These tests isolate the transport/lease receipt contract. Artifact admission
// and verification are covered separately by the complete artifact saga tests.
type protocolArtifacts struct{ queue *clipapp.MediaStages }

func (p protocolArtifacts) Complete(ctx context.Context, a clip.MediaLeaseCredentials, result string) error {
	_, err := p.queue.Complete(ctx, a, result)
	return err
}
func (protocolArtifacts) Read(context.Context, clip.MediaLeaseCredentials, string) (clip.MediaArtifactAccess, error) {
	return clip.MediaArtifactAccess{}, clip.ErrMediaUnsupported
}
func (protocolArtifacts) Reserve(context.Context, clip.MediaLeaseCredentials, []clip.MediaOutput) ([]clip.MediaArtifactAccess, error) {
	return nil, clip.ErrMediaUnsupported
}

func TestMediaWorkerHTTPContractAndFencedReceipts(t *testing.T) {
	q, st, in, now, _ := mediaFixture(t, clip.DefaultMediaStageLimits(clip.Environment{}))
	api := httptest.NewServer(cliprpc.NewMediaWorkerServer("", map[string]string{"prod-one": "one", "prod-two": "two"}, clipapp.NewMediaWorker(st, protocolArtifacts{q}, func() time.Time { return *now })).Handler)
	defer api.Close()
	c := workerclient.New(api.URL, "prod-one", "one")
	other := workerclient.New(api.URL, "prod-two", "two")
	p := mediaProfile()
	p.WorkerID = "forged"
	ctx := t.Context()
	if _, err := workerclient.New(api.URL, "staging-one", "one").Status(ctx); !errors.Is(err, clip.ErrMediaUnauthenticated) {
		t.Fatal("other environment", err)
	}
	if work, err := c.Claim(ctx, p); err != nil || work != nil {
		t.Fatal("empty queue", err)
	}
	for _, mutate := range []func(*clip.MediaWorkerProfile){
		func(p *clip.MediaWorkerProfile) { p.ContractVersion++ }, func(p *clip.MediaWorkerProfile) { p.RendererVersion = "future" }, func(p *clip.MediaWorkerProfile) { p.AssetVersion = "future" }, func(p *clip.MediaWorkerProfile) { p.Profile = "nvenc" },
	} {
		bad := p
		mutate(&bad)
		if _, err := c.Claim(ctx, bad); !errors.Is(err, clip.ErrMediaIncompatible) {
			t.Fatal("incompatible runtime silently polled", err)
		}
	}
	bad := p
	bad.Operation = "arbitrary-command"
	if _, err := c.Claim(ctx, bad); !errors.Is(err, clip.ErrInvalid) {
		t.Fatal("unknown operation", err)
	}
	if _, err := q.Create(ctx, in); err != nil {
		t.Fatal(err)
	}
	status, err := c.Status(ctx)
	if err != nil || status.Waiting != 1 || status.OwnActive != 0 {
		t.Fatal(status, err)
	}
	before := time.Now()
	work, err := c.Claim(ctx, p)
	if err != nil || work == nil {
		t.Fatal("claim", err)
	}
	if work.Credentials.WorkerID != "prod-one" || work.Payload != in.Payload || work.LeaseExpiresAt.After(time.Now().Add(in.Limits.LeaseTTL)) || work.LeaseRemaining > in.Limits.LeaseTTL || !work.LeaseExpiresAt.After(before) {
		t.Fatal("bad mapping or unsafe clock", work)
	}
	status, err = c.Status(ctx)
	if err != nil || status.Active != 1 || status.Waiting != 0 || status.OwnActive != 1 {
		t.Fatal(status, err)
	}
	status, err = other.Status(ctx)
	if err != nil || status.Active != 1 || status.OwnActive != 0 {
		t.Fatal("other worker scope", status, err)
	}
	if _, err := other.Renew(ctx, work.Credentials, 10); !errors.Is(err, clip.ErrMediaLeaseLost) {
		t.Fatal("stolen lease", err)
	}
	if _, err := c.Renew(ctx, work.Credentials, 1001); !errors.Is(err, clip.ErrInvalid) {
		t.Fatal("unbounded progress", err)
	}
	if _, err := c.Renew(ctx, work.Credentials, 800); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Renew(ctx, work.Credentials, 10); err != nil {
		t.Fatal(err)
	}
	if err := c.Fail(ctx, work.Credentials, clip.MediaFailure("raw stderr https://signed?secret")); !errors.Is(err, clip.ErrInvalid) {
		t.Fatal("free prose failure", err)
	}
	if err := c.Complete(ctx, work.Credentials, `{"output":"reserved-slot"}`); err != nil {
		t.Fatal(err)
	}
	if err := c.Complete(ctx, work.Credentials, `{"output":"reserved-slot"}`); err != nil {
		t.Fatal("duplicate receipt", err)
	}
	if err := c.Complete(ctx, work.Credentials, `{"output":"different"}`); !errors.Is(err, clip.ErrMediaConflict) {
		t.Fatal("conflicting receipt", err)
	}
	if err := other.Complete(ctx, work.Credentials, `{"output":"reserved-slot"}`); !errors.Is(err, clip.ErrMediaLeaseLost) {
		t.Fatal("other worker receipt", err)
	}
	if _, err := c.Renew(ctx, work.Credentials, 900); !errors.Is(err, clip.ErrMediaLeaseLost) {
		t.Fatal("terminal lease", err)
	}
	// The private listener cannot call public RPCs, even with a valid worker.
	r, _ := http.NewRequest(http.MethodPost, api.URL+"/postpilot.v1.PostService/ListPosts", strings.NewReader("{}"))
	r.Header.Set("X-Media-Worker-ID", "prod-one")
	r.Header.Set("Authorization", "Bearer one")
	r.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatal("user RPC on internal listener", resp.StatusCode)
	}
}

func TestMediaWorkerCancellationAndFailureAreDistinctFromLeaseLoss(t *testing.T) {
	q, st, in, now, d := mediaFixture(t, clip.DefaultMediaStageLimits(clip.Environment{}))
	ctx := t.Context()
	if _, err := q.Create(ctx, in); err != nil {
		t.Fatal(err)
	}
	api := httptest.NewServer(cliprpc.NewMediaWorkerServer("", map[string]string{"worker-1": "token", "worker-2": "other"}, clipapp.NewMediaWorker(st, protocolArtifacts{q}, func() time.Time { return *now })).Handler)
	defer api.Close()
	c := workerclient.New(api.URL, "worker-1", "token")
	w, err := c.Claim(ctx, mediaProfile())
	if err != nil || w == nil {
		t.Fatal(err)
	}
	*now = now.Add(2 * time.Minute)
	if err := c.Fail(ctx, w.Credentials, clip.MediaFailureInvalidInput); !errors.Is(err, clip.ErrMediaLeaseLost) {
		t.Fatal("expired failure accepted", err)
	}
	next, err := c.Claim(ctx, mediaProfile())
	if err != nil || next == nil {
		t.Fatal(err)
	}
	if err := c.Complete(ctx, w.Credentials, `{}`); !errors.Is(err, clip.ErrMediaLeaseLost) {
		t.Fatal("stale receipt accepted", err)
	}
	if _, err := d.Writer.Exec(`UPDATE clip_media_stages SET state='cancelled' WHERE id=?`, in.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Renew(ctx, next.Credentials, 5); !errors.Is(err, clip.ErrMediaCancelled) {
		t.Fatal("cancel signal missing", err)
	}
	other := workerclient.New(api.URL, "worker-2", "other")
	if _, err := other.Renew(ctx, next.Credentials, 5); !errors.Is(err, clip.ErrMediaLeaseLost) {
		t.Fatal("cancel leaked to wrong worker", err)
	}
	if err := c.Complete(ctx, next.Credentials, `{}`); !errors.Is(err, clip.ErrMediaCancelled) {
		t.Fatal("cancelled completion", err)
	}
	if _, err := d.Writer.Exec(`UPDATE clip_media_stages SET state='running' WHERE id=?`, in.ID); err != nil {
		t.Fatal(err)
	}
	if err := c.Fail(ctx, next.Credentials, clip.MediaFailureInvalidOutput); err != nil {
		t.Fatal(err)
	}
	if err := c.Fail(ctx, next.Credentials, clip.MediaFailureInvalidOutput); err != nil {
		t.Fatal("failure replay", err)
	}
	if err := c.Fail(ctx, next.Credentials, clip.MediaFailureInternal); !errors.Is(err, clip.ErrMediaConflict) {
		t.Fatal("failure rewrite", err)
	}
	stage, err := st.GetMediaStage(ctx, in.ID)
	if err != nil || stage.Failure != clip.MediaFailureInvalidOutput || stage.State != clip.MediaFailed {
		t.Fatal(stage, err)
	}
}

func TestMediaWorkerRefusesPersistedFutureContractsAfterRollback(t *testing.T) {
	q, st, in, now, _ := mediaFixture(t, clip.DefaultMediaStageLimits(clip.Environment{}))
	in.ContractVersion++
	if _, err := q.Create(t.Context(), in); err != nil {
		t.Fatal(err)
	}
	service := clipapp.NewMediaWorker(st, protocolArtifacts{q}, func() time.Time { return *now })
	if _, err := service.Claim(t.Context(), mediaProfile()); !errors.Is(err, clip.ErrMediaIncompatible) {
		t.Fatal("incompatible queued work silently polled", err)
	}
}
