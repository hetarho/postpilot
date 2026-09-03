package rpc_test

import (
	"context"
	"errors"
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

// newPlanServer mounts the auth and admin surfaces behind the real interceptor over a real
// SQLite store, seeded with one account per tier. It returns clients for both services so a
// test can prove what an ordinary account is refused and what the operator is not.
func newPlanServer(t *testing.T) (postpilotv1connect.AuthServiceClient, postpilotv1connect.AdminServiceClient, postpilotv1connect.PublishingServiceClient, postpilotv1connect.ModelCatalogServiceClient) {
	t.Helper()

	svc := auth.NewService(newStore(t), sessionTTL)
	for id, tier := range map[string]plan.Plan{"alice": plan.Free, "root": plan.Master} {
		if err := svc.CreateUser(context.Background(), id, "s3cret", tier); err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
	}

	interceptor := connect.WithInterceptors(authrpc.NewInterceptor(svc))
	mux := http.NewServeMux()
	mux.Handle(postpilotv1connect.NewAuthServiceHandler(authrpc.NewHandler(svc, sessionTTL), interceptor))
	mux.Handle(postpilotv1connect.NewAdminServiceHandler(authrpc.NewAdminHandler(svc), interceptor))
	// Unimplemented on purpose: the gate is the interceptor, so a refusal must arrive without
	// the handler ever running. Reaching the handler would answer `unimplemented` instead.
	mux.Handle(postpilotv1connect.NewPublishingServiceHandler(
		postpilotv1connect.UnimplementedPublishingServiceHandler{}, interceptor))
	mux.Handle(postpilotv1connect.NewModelCatalogServiceHandler(
		postpilotv1connect.UnimplementedModelCatalogServiceHandler{}, interceptor))

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return postpilotv1connect.NewAuthServiceClient(server.Client(), server.URL),
		postpilotv1connect.NewAdminServiceClient(server.Client(), server.URL),
		postpilotv1connect.NewPublishingServiceClient(server.Client(), server.URL),
		postpilotv1connect.NewModelCatalogServiceClient(server.Client(), server.URL)
}

func loginAs(t *testing.T, client postpilotv1connect.AuthServiceClient, id string) string {
	t.Helper()
	res, err := client.Login(context.Background(), connect.NewRequest(&postpilotv1.LoginRequest{
		LoginId: id, Password: "s3cret",
	}))
	if err != nil {
		t.Fatalf("login %s: %v", id, err)
	}
	cookie := res.Header().Get("Set-Cookie")
	if cookie == "" {
		t.Fatalf("login %s returned no cookie", id)
	}
	return cookie
}

// A12: the session probe carries the tier, so the app gates without a second round-trip.
func TestGetMeCarriesTheTier(t *testing.T) {
	authClient, _, _, _ := newPlanServer(t)

	for id, want := range map[string]postpilotv1.Plan{
		"alice": postpilotv1.Plan_PLAN_FREE,
		"root":  postpilotv1.Plan_PLAN_MASTER,
	} {
		cookie := loginAs(t, authClient, id)
		res, err := authClient.GetMe(context.Background(), withCookie(&postpilotv1.GetMeRequest{}, cookie))
		if err != nil {
			t.Fatalf("GetMe as %s: %v", id, err)
		}
		if got := res.Msg.GetPlan(); got != want {
			t.Errorf("%s plan = %v, want %v", id, got, want)
		}
	}
}

// A8: every human publishing procedure is master-only, and the refusal is the typed one the
// frontend renders from.
func TestPublishingIsMasterOnly(t *testing.T) {
	authClient, _, publishing, _ := newPlanServer(t)
	free := loginAs(t, authClient, "alice")

	_, err := publishing.CreateAgentPairing(context.Background(),
		withCookie(&postpilotv1.CreateAgentPairingRequest{}, free))
	if connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("pairing as free = %v, want permission_denied", err)
	}
	if detail := authAppErrorDetail(t, err); detail.GetReason() != plan.ReasonMasterOnly {
		t.Errorf("reason = %q, want %q", detail.GetReason(), plan.ReasonMasterOnly)
	}

	// The operator reaches the handler — which is unimplemented here, and that is the point:
	// the gate let it through.
	master := loginAs(t, authClient, "root")
	_, err = publishing.CreateAgentPairing(context.Background(),
		withCookie(&postpilotv1.CreateAgentPairingRequest{}, master))
	if connect.CodeOf(err) != connect.CodeUnimplemented {
		t.Errorf("pairing as master = %v, want the handler to have been reached", err)
	}
}

// A1 (plan 18): curating the model catalog decides what every account may spend money on,
// so the whole service sits behind the same gate as the tier assignment — enforced here,
// whatever the client rendered.
func TestModelCatalogIsMasterOnly(t *testing.T) {
	authClient, _, _, catalog := newPlanServer(t)

	free := loginAs(t, authClient, "alice")
	for name, call := range map[string]func(string) error{
		"ListCatalog": func(cookie string) error {
			_, err := catalog.ListCatalog(context.Background(), withCookie(&postpilotv1.ListCatalogRequest{}, cookie))
			return err
		},
		"SetModelPurpose": func(cookie string) error {
			_, err := catalog.SetModelPurpose(context.Background(), withCookie(&postpilotv1.SetModelPurposeRequest{
				ModelId: "openai/gpt-x", Purpose: "writing", Registered: true,
			}, cookie))
			return err
		},
		"UpdateModel": func(cookie string) error {
			_, err := catalog.UpdateModel(context.Background(), withCookie(&postpilotv1.UpdateModelRequest{
				ModelId: "openai/gpt-x",
			}, cookie))
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			err := call(free)
			if connect.CodeOf(err) != connect.CodePermissionDenied {
				t.Fatalf("as free = %v, want permission_denied", err)
			}
			if detail := authAppErrorDetail(t, err); detail.GetReason() != plan.ReasonMasterOnly {
				t.Errorf("reason = %q, want %q", detail.GetReason(), plan.ReasonMasterOnly)
			}
			// The operator reaches the handler — unimplemented here, which is how we know the
			// gate let it through rather than the handler agreeing with it.
			if err := call(loginAs(t, authClient, "root")); connect.CodeOf(err) != connect.CodeUnimplemented {
				t.Errorf("as master = %v, want the handler to have been reached", err)
			}
		})
	}
}

// A10: the admin surface answers the operator and refuses everyone else.
func TestAdminIsMasterOnly(t *testing.T) {
	authClient, admin, _, _ := newPlanServer(t)

	free := loginAs(t, authClient, "alice")
	_, err := admin.ListUsers(context.Background(), withCookie(&postpilotv1.ListUsersRequest{}, free))
	if connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("ListUsers as free = %v, want permission_denied", err)
	}

	master := loginAs(t, authClient, "root")
	res, err := admin.ListUsers(context.Background(), withCookie(&postpilotv1.ListUsersRequest{}, master))
	if err != nil {
		t.Fatalf("ListUsers as master: %v", err)
	}
	if got, want := len(res.Msg.GetUsers()), 2; got != want {
		t.Fatalf("users = %d, want %d", got, want)
	}

	changed, err := admin.SetUserPlan(context.Background(),
		withCookie(&postpilotv1.SetUserPlanRequest{UserId: "alice", Plan: postpilotv1.Plan_PLAN_MAX}, master))
	if err != nil {
		t.Fatalf("SetUserPlan: %v", err)
	}
	if got := changed.Msg.GetUser().GetPlan(); got != postpilotv1.Plan_PLAN_MAX {
		t.Errorf("echoed plan = %v", got)
	}

	// The tier is resolved per request, so the promotion is in force on the very next call
	// rather than at the next login.
	promoted := admin
	if _, err := promoted.ListUsers(context.Background(), withCookie(&postpilotv1.ListUsersRequest{}, free)); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("max is still not master: %v, want permission_denied", err)
	}
}

// A10: the ladder must not be able to lock administration out of the deployment.
func TestTheLastMasterCannotBeDemoted(t *testing.T) {
	authClient, admin, _, _ := newPlanServer(t)
	master := loginAs(t, authClient, "root")

	_, err := admin.SetUserPlan(context.Background(),
		withCookie(&postpilotv1.SetUserPlanRequest{UserId: "root", Plan: postpilotv1.Plan_PLAN_MAX}, master))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("demoting the last master = %v, want failed_precondition", err)
	}
	if detail := authAppErrorDetail(t, err); detail.GetReason() != "LAST_MASTER" {
		t.Errorf("reason = %q", detail.GetReason())
	}

	// With a second master the same demotion is allowed — the guard is about the last one,
	// not about master accounts in general.
	if _, err := admin.SetUserPlan(context.Background(),
		withCookie(&postpilotv1.SetUserPlanRequest{UserId: "alice", Plan: postpilotv1.Plan_PLAN_MASTER}, master)); err != nil {
		t.Fatalf("promote alice: %v", err)
	}
	if _, err := admin.SetUserPlan(context.Background(),
		withCookie(&postpilotv1.SetUserPlanRequest{UserId: "root", Plan: postpilotv1.Plan_PLAN_MAX}, master)); err != nil {
		t.Fatalf("demoting one of two masters: %v", err)
	}
}

// The same guard has to hold on the CLI path, which does not go through the interceptor.
func TestServiceRefusesTheLastMasterDemotion(t *testing.T) {
	svc := auth.NewService(newStore(t), sessionTTL)
	ctx := context.Background()
	if err := svc.CreateUser(ctx, "root", "s3cret", plan.Master); err != nil {
		t.Fatal(err)
	}
	if err := svc.CreateUser(ctx, "alice", "s3cret", plan.Free); err != nil {
		t.Fatal(err)
	}

	if err := svc.SetUserPlan(ctx, "root", plan.Free); !errors.Is(err, auth.ErrLastMaster) {
		t.Fatalf("error = %v, want ErrLastMaster", err)
	}
	// Setting a master to master is not a demotion, so it must not be refused.
	if err := svc.SetUserPlan(ctx, "root", plan.Master); err != nil {
		t.Errorf("no-op set = %v, want nil", err)
	}
	if err := svc.SetUserPlan(ctx, "ghost", plan.Free); !errors.Is(err, auth.ErrUserNotFound) {
		t.Errorf("unknown account = %v, want ErrUserNotFound", err)
	}
}
