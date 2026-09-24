package workerclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/clip"
	pb "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
)

type delayedRenew struct {
	postpilotv1connect.UnimplementedClipMediaWorkerServiceHandler
	started chan time.Time
}

func (h *delayedRenew) RenewMediaStage(ctx context.Context, _ *connect.Request[pb.RenewMediaStageRequest]) (*connect.Response[pb.RenewMediaStageResponse], error) {
	h.started <- time.Now()
	select {
	case <-time.After(40 * time.Millisecond):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return connect.NewResponse(&pb.RenewMediaStageResponse{LeaseRemainingMs: 1000}), nil
}
func TestRenewUsesRequestStartInsteadOfAPIEpochOrResponseArrival(t *testing.T) {
	h := &delayedRenew{started: make(chan time.Time, 1)}
	mux := http.NewServeMux()
	mux.Handle(postpilotv1connect.NewClipMediaWorkerServiceHandler(h))
	s := httptest.NewServer(mux)
	defer s.Close()
	c := New(s.URL, "worker", "token")
	expires, err := c.Renew(t.Context(), clip.MediaLeaseCredentials{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if expires.After((<-h.started).Add(time.Second)) || !expires.After(time.Now()) {
		t.Fatal("unsafe expiry", expires)
	}
	for range 50 {
		if d := PollDelay(); d < clip.MediaPollInterval || d > clip.MediaPollInterval+clip.MediaPollJitter {
			t.Fatal("unbounded jitter", d)
		}
	}
}
func TestWorkerTransportDoesNotForwardSecretsThroughRedirects(t *testing.T) {
	reached := false
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached = true }))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer redirect.Close()
	_, err := New(redirect.URL, "worker", "secret").Status(t.Context())
	if reached || !errors.Is(err, clip.ErrMediaUnavailable) {
		t.Fatal("redirect leaked authority", err)
	}
}
