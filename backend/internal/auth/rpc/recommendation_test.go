package rpc_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/auth"
	authrpc "github.com/postpilot/backend/internal/auth/rpc"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
	"github.com/postpilot/backend/internal/plan"
)

// MODEL-69: writing a recommendation set is curation — every account is shown the advice — so
// the three writes sit behind the operator gate, while listing and applying a set stay open to
// any account. The provider handler is unimplemented here: reaching it is how a test knows the
// gate let the call through.
func TestRecommendationSetWritesAreMasterOnly(t *testing.T) {
	svc := auth.NewService(newStore(t), sessionTTL, auth.Deps{Mailer: discardMailer{}, TopUp: func(context.Context, string, int) error { return nil }})
	for id, tier := range map[string]plan.Plan{"alice": plan.Free, "root": plan.Master} {
		if err := svc.CreateUser(context.Background(), id, "s3cret", tier); err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
	}
	interceptor := connect.WithInterceptors(authrpc.NewInterceptor(svc, auth.NewThrottle(), ""))
	mux := http.NewServeMux()
	mux.Handle(postpilotv1connect.NewAuthServiceHandler(authrpc.NewHandler(svc, sessionTTL), interceptor))
	mux.Handle(postpilotv1connect.NewProviderServiceHandler(postpilotv1connect.UnimplementedProviderServiceHandler{}, interceptor))
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	authClient := postpilotv1connect.NewAuthServiceClient(server.Client(), server.URL)
	providers := postpilotv1connect.NewProviderServiceClient(server.Client(), server.URL)

	writes := map[string]func(string) error{
		"SaveRecommendationSet": func(cookie string) error {
			_, err := providers.SaveRecommendationSet(context.Background(), withCookie(&postpilotv1.SaveRecommendationSetRequest{Label: "Set"}, cookie))
			return err
		},
		"DeleteRecommendationSet": func(cookie string) error {
			_, err := providers.DeleteRecommendationSet(context.Background(), withCookie(&postpilotv1.DeleteRecommendationSetRequest{Id: "set"}, cookie))
			return err
		},
		"MoveRecommendationSet": func(cookie string) error {
			_, err := providers.MoveRecommendationSet(context.Background(), withCookie(&postpilotv1.MoveRecommendationSetRequest{Id: "set", Earlier: true}, cookie))
			return err
		},
	}
	free := loginAs(t, authClient, "alice")
	root := loginAs(t, authClient, "root")
	for name, call := range writes {
		t.Run(name, func(t *testing.T) {
			err := call(free)
			if connect.CodeOf(err) != connect.CodePermissionDenied {
				t.Fatalf("as free = %v, want permission_denied", err)
			}
			if detail := authAppErrorDetail(t, err); detail.GetReason() != plan.ReasonMasterOnly {
				t.Errorf("reason = %q, want %q", detail.GetReason(), plan.ReasonMasterOnly)
			}
			if err := call(root); connect.CodeOf(err) != connect.CodeUnimplemented {
				t.Errorf("as master = %v, want the handler to have been reached", err)
			}
		})
	}

	reads := map[string]func(string) error{
		"ListRecommendationSets": func(cookie string) error {
			_, err := providers.ListRecommendationSets(context.Background(), withCookie(&postpilotv1.ListRecommendationSetsRequest{}, cookie))
			return err
		},
		"ApplyRecommendationSet": func(cookie string) error {
			_, err := providers.ApplyRecommendationSet(context.Background(), withCookie(&postpilotv1.ApplyRecommendationSetRequest{Id: "set"}, cookie))
			return err
		},
	}
	for name, call := range reads {
		t.Run(name, func(t *testing.T) {
			if err := call(free); connect.CodeOf(err) != connect.CodeUnimplemented {
				t.Fatalf("as free = %v, want the handler to have been reached", err)
			}
		})
	}
}
