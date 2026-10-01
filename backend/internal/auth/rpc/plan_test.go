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
func newPlanServer(t *testing.T) (postpilotv1connect.AuthServiceClient, postpilotv1connect.AdminServiceClient, postpilotv1connect.ModelCatalogServiceClient) {
	t.Helper()

	// Production wires the ledger's top-up here (QUOTA-35). These cases are about the
	// master-only gate, so the credit side is a no-op; that it actually RUNS on this path is
	// pinned by TestSetUserPlanRunsTheUpgradeTopUp below.
	svc := auth.NewService(newStore(t), sessionTTL, auth.Deps{Mailer: discardMailer{}, TopUp: func(context.Context, string, int) error { return nil }})
	for id, tier := range map[string]plan.Plan{"alice": plan.Free, "root": plan.Master} {
		if err := svc.CreateUser(context.Background(), id, "s3cret", tier); err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
	}

	interceptor := connect.WithInterceptors(authrpc.NewInterceptor(svc, auth.NewThrottle(), ""))
	mux := http.NewServeMux()
	mux.Handle(postpilotv1connect.NewAuthServiceHandler(authrpc.NewHandler(svc, sessionTTL), interceptor))
	mux.Handle(postpilotv1connect.NewAdminServiceHandler(
		authrpc.NewAdminHandler(svc, &fakeComboAssigner{}, fakeRates{}), interceptor))
	mux.Handle(postpilotv1connect.NewModelCatalogServiceHandler(
		postpilotv1connect.UnimplementedModelCatalogServiceHandler{}, interceptor))

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return postpilotv1connect.NewAuthServiceClient(server.Client(), server.URL),
		postpilotv1connect.NewAdminServiceClient(server.Client(), server.URL),
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
	authClient, _, _ := newPlanServer(t)

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

// MODEL-13: curating the model catalog decides what every account may spend money on,
// so the whole service sits behind the same gate as the tier assignment — enforced here,
// whatever the client rendered.
func TestModelCatalogIsMasterOnly(t *testing.T) {
	authClient, _, catalog := newPlanServer(t)

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

func TestRefundReviewProceduresAreMasterOnly(t *testing.T) {
	svc := auth.NewService(newStore(t), sessionTTL, auth.Deps{Mailer: discardMailer{}})
	for id, tier := range map[string]plan.Plan{"alice": plan.Free, "root": plan.Master} {
		if err := svc.CreateUser(context.Background(), id, "s3cret", tier); err != nil {
			t.Fatal(err)
		}
	}
	interceptor := connect.WithInterceptors(authrpc.NewInterceptor(svc, auth.NewThrottle(), ""))
	mux := http.NewServeMux()
	mux.Handle(postpilotv1connect.NewAuthServiceHandler(authrpc.NewHandler(svc, sessionTTL), interceptor))
	mux.Handle(postpilotv1connect.NewBillingServiceHandler(postpilotv1connect.UnimplementedBillingServiceHandler{}, interceptor))
	server := httptest.NewServer(mux)
	defer server.Close()
	authClient := postpilotv1connect.NewAuthServiceClient(server.Client(), server.URL)
	billingClient := postpilotv1connect.NewBillingServiceClient(server.Client(), server.URL)
	free, master := loginAs(t, authClient, "alice"), loginAs(t, authClient, "root")
	for name, call := range map[string]func(string) error{
		"list": func(cookie string) error {
			_, err := billingClient.ListRefundReviews(context.Background(), withCookie(&postpilotv1.ListRefundReviewsRequest{}, cookie))
			return err
		},
		"review": func(cookie string) error {
			_, err := billingClient.ReviewRefund(context.Background(), withCookie(&postpilotv1.ReviewRefundRequest{}, cookie))
			return err
		},
		"reconcile": func(cookie string) error {
			_, err := billingClient.ReconcileRefund(context.Background(), withCookie(&postpilotv1.ReconcileRefundRequest{}, cookie))
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			if code := connect.CodeOf(call(free)); code != connect.CodePermissionDenied {
				t.Fatalf("free code=%s", code)
			}
			if code := connect.CodeOf(call(master)); code != connect.CodeUnimplemented {
				t.Fatalf("master code=%s", code)
			}
		})
	}
}

// A10: the admin surface answers the operator and refuses everyone else.
func TestAdminIsMasterOnly(t *testing.T) {
	authClient, admin, _ := newPlanServer(t)

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

// QUOTA-63: an operator cannot change their own tier on the RPC path, for any target — the
// last-master guard (QUOTA-4) is no longer what stands between a lone operator and a click.
func TestAnOperatorCannotChangeTheirOwnPlan(t *testing.T) {
	authClient, admin, _ := newPlanServer(t)
	master := loginAs(t, authClient, "root")

	for _, target := range []postpilotv1.Plan{postpilotv1.Plan_PLAN_MAX, postpilotv1.Plan_PLAN_FREE, postpilotv1.Plan_PLAN_MASTER} {
		_, err := admin.SetUserPlan(context.Background(),
			withCookie(&postpilotv1.SetUserPlanRequest{UserId: "root", Plan: target}, master))
		if connect.CodeOf(err) != connect.CodeFailedPrecondition {
			t.Fatalf("self-assigning %v = %v, want failed_precondition", target, err)
		}
		if detail := authAppErrorDetail(t, err); detail.GetReason() != "MASTER_SELF_PLAN" {
			t.Errorf("self-assigning %v: reason = %q", target, detail.GetReason())
		}
	}
	res, err := admin.ListUsers(context.Background(), withCookie(&postpilotv1.ListUsersRequest{}, master))
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	for _, user := range res.Msg.GetUsers() {
		if user.GetId() == "root" && user.GetPlan() != postpilotv1.Plan_PLAN_MASTER {
			t.Errorf("root plan = %v after refused self-assignments, want master", user.GetPlan())
		}
	}

	// Another master may still move this account: that is the one ordinary way out.
	if _, err := admin.SetUserPlan(context.Background(),
		withCookie(&postpilotv1.SetUserPlanRequest{UserId: "alice", Plan: postpilotv1.Plan_PLAN_MASTER}, master)); err != nil {
		t.Fatalf("promote alice: %v", err)
	}
	second := loginAs(t, authClient, "alice")
	if _, err := admin.SetUserPlan(context.Background(),
		withCookie(&postpilotv1.SetUserPlanRequest{UserId: "root", Plan: postpilotv1.Plan_PLAN_MAX}, second)); err != nil {
		t.Fatalf("another master demoting root: %v", err)
	}
	if _, err := admin.ListUsers(context.Background(), withCookie(&postpilotv1.ListUsersRequest{}, master)); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("demoted root = %v, want permission_denied", err)
	}
}

// The same guard has to hold on the CLI path, which does not go through the interceptor.
func TestServiceRefusesTheLastMasterDemotion(t *testing.T) {
	svc := auth.NewService(newStore(t), sessionTTL, auth.Deps{Mailer: discardMailer{}})
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

// QUOTA-35 on the RPC path: promoting an account owes the cycle already running the
// difference between the two grants, and the handler must run that credit side rather than
// leaving it to whoever remembers.
func TestSetUserPlanRunsTheUpgradeTopUp(t *testing.T) {
	type topUpCall struct {
		userID  string
		credits int
	}
	var topUps []topUpCall
	svc := auth.NewService(newStore(t), sessionTTL, auth.Deps{Mailer: discardMailer{}, TopUp: func(_ context.Context, userID string, credits int) error {
		topUps = append(topUps, topUpCall{userID, credits})
		return nil
	}})
	if err := svc.CreateUser(context.Background(), "alice", "s3cret", plan.Free); err != nil {
		t.Fatalf("seed alice: %v", err)
	}

	admin := authrpc.NewAdminHandler(svc, &fakeComboAssigner{}, nil)
	ctx := auth.WithActor(context.Background(), auth.Actor{UserID: "root", Plan: plan.Master})

	if _, err := admin.SetUserPlan(ctx, connect.NewRequest(&postpilotv1.SetUserPlanRequest{
		UserId: "alice", Plan: postpilotv1.Plan_PLAN_PRO,
	})); err != nil {
		t.Fatalf("SetUserPlan: %v", err)
	}
	owed := plan.MonthlyCredits(plan.Pro) - plan.MonthlyCredits(plan.Free)
	if len(topUps) != 1 || topUps[0].userID != "alice" || topUps[0].credits != owed {
		t.Fatalf("top-ups = %v, want one of %d credits for alice", topUps, owed)
	}

	// A downgrade owes nothing: the cycle already granted stands untouched (QUOTA-35).
	if _, err := admin.SetUserPlan(ctx, connect.NewRequest(&postpilotv1.SetUserPlanRequest{
		UserId: "alice", Plan: postpilotv1.Plan_PLAN_BASIC,
	})); err != nil {
		t.Fatalf("downgrade: %v", err)
	}
	if len(topUps) != 1 {
		t.Errorf("top-ups after a downgrade = %v, want the upgrade's one", topUps)
	}
}

// fakeComboAssigner records the assignment and can answer with either refusal the edge maps.
type fakeRates struct {
	rate plan.RateSnapshot
	err  error
}

func (f fakeRates) CurrentRate(context.Context) (plan.RateSnapshot, error) { return f.rate, f.err }

// QUOTA-65: the operator reads the rate customers are priced at on /admin only, so the read is
// master-only, and a missing or ineligible rate is a state to show rather than a failed read.
func TestGetExchangeRateIsMasterOnlyAndShowsEveryState(t *testing.T) {
	authClient, admin, _ := newPlanServer(t)
	free := loginAs(t, authClient, "alice")
	_, err := admin.GetExchangeRate(context.Background(), withCookie(&postpilotv1.GetExchangeRateRequest{}, free))
	var ce *connect.Error
	if connect.CodeOf(err) != connect.CodePermissionDenied || !errors.As(err, &ce) || len(ce.Details()) != 1 {
		t.Fatalf("as free = %v, want permission_denied", err)
	}
	if detail, derr := ce.Details()[0].Value(); derr != nil || detail.(*postpilotv1.AppErrorDetail).GetReason() != "MASTER_ONLY" {
		t.Fatalf("refusal detail = %v (%v), want MASTER_ONLY", detail, derr)
	}

	ctx := auth.WithActor(context.Background(), auth.Actor{UserID: "root", Plan: plan.Master})
	confirmed := plan.RateSnapshot{Source: "korea-eximbank", PublicationDate: "2026-09-30", ReferenceE4: 13_584_000, AppliedE4: 13_600_000}
	temporary := confirmed
	temporary.Temporary = true
	for name, tc := range map[string]struct {
		rates       authrpc.RateReader
		unavailable bool
		temporary   bool
	}{
		"confirmed":   {rates: fakeRates{rate: confirmed}},
		"temporary":   {rates: fakeRates{rate: temporary}, temporary: true},
		"unavailable": {rates: fakeRates{err: errors.New("no eligible rate")}, unavailable: true},
		"invalid":     {rates: fakeRates{}, unavailable: true},
		"unwired":     {rates: nil, unavailable: true},
	} {
		handler := authrpc.NewAdminHandler(nil, nil, tc.rates)
		res, err := handler.GetExchangeRate(ctx, connect.NewRequest(&postpilotv1.GetExchangeRateRequest{}))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		got := res.Msg
		if got.GetUnavailable() != tc.unavailable || (got.GetRate() == nil) != tc.unavailable {
			t.Fatalf("%s: response = %+v", name, got)
		}
		if !tc.unavailable && (got.GetRate().GetAppliedE4() != 13_600_000 || got.GetRate().GetReferenceE4() != 13_584_000 ||
			got.GetRate().GetSource() != "korea-eximbank" || got.GetRate().GetTemporary() != tc.temporary) {
			t.Fatalf("%s: rate = %+v", name, got.GetRate())
		}
	}
}

type fakeComboAssigner struct {
	calls []string
	err   error
}

func (f *fakeComboAssigner) AssignCombo(_ context.Context, combo, observe, write string) error {
	f.calls = append(f.calls, combo+":"+observe+"/"+write)
	return f.err
}

// The estimator assignment is privileged like every other admin action, and its two refusals
// reach the client as reasons it can render copy from (QUOTA-39).
func TestSetEstimatorComboIsMasterOnlyAndMapsItsRefusals(t *testing.T) {
	authClient, admin, _ := newPlanServer(t)

	free := loginAs(t, authClient, "alice")
	_, err := admin.SetEstimatorCombo(context.Background(), withCookie(&postpilotv1.SetEstimatorComboRequest{
		Combo: "top", ObserveModelId: "vendor/eyes", WriteModelId: "vendor/pen",
	}, free))
	if connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("as free = %v, want permission_denied", err)
	}

	svc := auth.NewService(newStore(t), sessionTTL, auth.Deps{Mailer: discardMailer{}, TopUp: func(context.Context, string, int) error { return nil }})
	assigner := &fakeComboAssigner{}
	handler := authrpc.NewAdminHandler(svc, assigner, nil)
	ctx := auth.WithActor(context.Background(), auth.Actor{UserID: "root", Plan: plan.Master})

	if _, err := handler.SetEstimatorCombo(ctx, connect.NewRequest(&postpilotv1.SetEstimatorComboRequest{
		Combo: "top", ObserveModelId: "vendor/eyes", WriteModelId: "vendor/pen",
	})); err != nil {
		t.Fatalf("SetEstimatorCombo: %v", err)
	}
	if len(assigner.calls) != 1 || assigner.calls[0] != "top:vendor/eyes/vendor/pen" {
		t.Errorf("assignments = %v", assigner.calls)
	}

	for _, tc := range []struct {
		name   string
		err    error
		reason string
		code   connect.Code
	}{
		{"unknown combo", authrpc.ErrComboUnknown, "COMBO_UNKNOWN", connect.CodeInvalidArgument},
		{"unregistered model", authrpc.ErrComboModelUnusable, "MODEL_NOT_REGISTERED", connect.CodeFailedPrecondition},
	} {
		assigner.err = tc.err
		_, err := handler.SetEstimatorCombo(ctx, connect.NewRequest(&postpilotv1.SetEstimatorComboRequest{
			Combo: "top", ObserveModelId: "vendor/eyes", WriteModelId: "vendor/pen",
		}))
		if connect.CodeOf(err) != tc.code {
			t.Errorf("%s code = %v, want %v", tc.name, connect.CodeOf(err), tc.code)
		}
		if detail := authAppErrorDetail(t, err); detail.GetReason() != tc.reason {
			t.Errorf("%s reason = %q, want %s", tc.name, detail.GetReason(), tc.reason)
		}
	}
	assigner.err = nil

	// An incomplete request never reaches the assigner.
	before := len(assigner.calls)
	if _, err := handler.SetEstimatorCombo(ctx, connect.NewRequest(&postpilotv1.SetEstimatorComboRequest{
		Combo: "top",
	})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("incomplete request = %v, want invalid_argument", err)
	}
	if len(assigner.calls) != before {
		t.Error("an incomplete request reached the assigner")
	}
}

// GIFT-2, GIFT-8: the gift page reads a voucher before the visitor has an account, redeeming
// needs a session, and issuing, listing and revoking are the operator's. An unimplemented
// handler behind the real interceptor tells the three apart: reaching it means the gate let
// the call through.
func TestVoucherServiceGates(t *testing.T) {
	svc := auth.NewService(newStore(t), sessionTTL, auth.Deps{Mailer: discardMailer{}})
	for id, tier := range map[string]plan.Plan{"alice": plan.Free, "root": plan.Master} {
		if err := svc.CreateUser(context.Background(), id, "s3cret", tier); err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
	}
	interceptor := connect.WithInterceptors(authrpc.NewInterceptor(svc, auth.NewThrottle(), ""))
	mux := http.NewServeMux()
	mux.Handle(postpilotv1connect.NewAuthServiceHandler(authrpc.NewHandler(svc, sessionTTL), interceptor))
	mux.Handle(postpilotv1connect.NewVoucherServiceHandler(postpilotv1connect.UnimplementedVoucherServiceHandler{}, interceptor))
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	authClient := postpilotv1connect.NewAuthServiceClient(server.Client(), server.URL)
	vouchers := postpilotv1connect.NewVoucherServiceClient(server.Client(), server.URL)

	if _, err := vouchers.GetVoucher(context.Background(), connect.NewRequest(&postpilotv1.GetVoucherRequest{Token: "t"})); connect.CodeOf(err) != connect.CodeUnimplemented {
		t.Fatalf("anonymous GetVoucher = %v, want the handler reached", err)
	}
	redeem := func(cookie string) error {
		request := connect.NewRequest(&postpilotv1.RedeemVoucherRequest{Token: "t"})
		if cookie != "" {
			request.Header().Set("Cookie", cookie)
		}
		_, err := vouchers.RedeemVoucher(context.Background(), request)
		return err
	}
	if err := redeem(""); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("anonymous RedeemVoucher = %v, want unauthenticated", err)
	}
	free := loginAs(t, authClient, "alice")
	if err := redeem(free); connect.CodeOf(err) != connect.CodeUnimplemented {
		t.Fatalf("free RedeemVoucher = %v, want the handler reached", err)
	}

	master := loginAs(t, authClient, "root")
	for name, call := range map[string]func(string) error{
		"IssueVoucher": func(cookie string) error {
			_, err := vouchers.IssueVoucher(context.Background(), withCookie(&postpilotv1.IssueVoucherRequest{}, cookie))
			return err
		},
		"ListVouchers": func(cookie string) error {
			_, err := vouchers.ListVouchers(context.Background(), withCookie(&postpilotv1.ListVouchersRequest{}, cookie))
			return err
		},
		"RevokeVoucher": func(cookie string) error {
			_, err := vouchers.RevokeVoucher(context.Background(), withCookie(&postpilotv1.RevokeVoucherRequest{}, cookie))
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
			if err := call(master); connect.CodeOf(err) != connect.CodeUnimplemented {
				t.Errorf("as master = %v, want the handler reached", err)
			}
		})
	}
}
