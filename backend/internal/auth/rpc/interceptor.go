package rpc

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/rpcserver"
)

// unauthenticatedMessage is what an unauthenticated call sees. Missing, forged, and
// expired sessions all produce it — the client's next move is the same in every case.
const unauthenticatedMessage = "unauthenticated"

// publicProcedures are the only procedures reachable without a session:
//   - Login, because it is how you get one.
//   - Logout, because logging out must always clear the cookie. Gated, it would answer
//     401 to an already-expired session and leave that HttpOnly cookie sitting in the
//     browser until it aged out on its own — revoking a dead session is a no-op anyway.
//   - Ping, the wire test the frontend runs before any account exists.
//
// Everything else is closed by default: a new service is protected the moment it is
// mounted, with no change here.
var publicProcedures = map[string]bool{
	postpilotv1connect.AuthServiceLoginProcedure:  true,
	postpilotv1connect.AuthServiceLogoutProcedure: true,
	postpilotv1connect.HealthServicePingProcedure: true,
}

// Agent procedures are not authenticated by the human HttpOnly session. The
// publishing context applies its own bearer-token interceptor; bypassing here is what
// prevents either credential from being accepted in the other's trust domain.
var agentProcedures = map[string]bool{
	postpilotv1connect.PublishingAgentServiceEnrollPublishingAgentProcedure: true,
	postpilotv1connect.PublishingAgentServiceSyncAgentProfileProcedure:      true,
	postpilotv1connect.PublishingAgentServiceClaimPublishJobProcedure:       true,
	postpilotv1connect.PublishingAgentServiceRenewPublishLeaseProcedure:     true,
	postpilotv1connect.PublishingAgentServiceReportPublishProgressProcedure: true,
	postpilotv1connect.PublishingAgentServiceCompletePublishProcedure:       true,
	postpilotv1connect.PublishingAgentServiceFailPublishProcedure:           true,
}

// masterProcedures may be called only by the operator tier.
//
// Publishing is here as a whole surface, not per capability: it runs through OUR paired
// agent and OUR infrastructure ([I1]), so an account that cannot be billed for it must
// not be able to pair one, start one, or drive an existing one. Administration is here
// because it is what assigns the tiers.
//
// The set is closed by default in the same sense as publicProcedures: a procedure absent
// from it is reachable by any authenticated plan, so a NEW master-only procedure must be
// added here in the same change that adds it to the proto.
var masterProcedures = map[string]bool{
	postpilotv1connect.PublishingServiceCreateAgentPairingProcedure:       true,
	postpilotv1connect.PublishingServiceListPublishingAgentsProcedure:     true,
	postpilotv1connect.PublishingServiceUpdatePublishingAgentProcedure:    true,
	postpilotv1connect.PublishingServiceRevokePublishingAgentProcedure:    true,
	postpilotv1connect.PublishingServiceStartPublishProcedure:             true,
	postpilotv1connect.PublishingServiceGetPublishJobProcedure:            true,
	postpilotv1connect.PublishingServiceListRetryablePublishJobsProcedure: true,
	postpilotv1connect.PublishingServiceRetryPublishProcedure:             true,
	postpilotv1connect.PublishingServiceCancelPublishProcedure:            true,

	postpilotv1connect.AdminServiceListUsersProcedure:   true,
	postpilotv1connect.AdminServiceSetUserPlanProcedure: true,

	// Curating the model catalog decides what every account may spend money on, so it sits
	// with the tier assignment rather than with the per-account model choice ProviderService
	// serves.
	postpilotv1connect.ModelCatalogServiceListCatalogProcedure:     true,
	postpilotv1connect.ModelCatalogServiceSetModelPurposeProcedure: true,
	postpilotv1connect.ModelCatalogServiceUpdateModelProcedure:     true,
}

// masterOnlyMessage is what a non-master caller sees. It names no account and no tier
// membership beyond the requirement itself.
const masterOnlyMessage = "this procedure requires the master plan"

// Interceptor is the authentication gate for every Connect procedure.
//
// It is the only place a request turns into an acting user: it resolves the session
// cookie once and puts the user id in the context, so downstream handlers never parse
// a cookie, never touch the sessions table, and never take a user id from a payload.
//
// It is a full connect.Interceptor rather than a connect.UnaryInterceptorFunc because
// that helper leaves WrapStreamingHandler as a pass-through — the first streaming
// procedure added later (the generation job queue, plan 05) would ship unauthenticated
// with nothing here to notice.
type Interceptor struct {
	svc *auth.Service
}

// NewInterceptor returns the authentication gate.
func NewInterceptor(svc *auth.Service) *Interceptor {
	return &Interceptor{svc: svc}
}

func (i *Interceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		ctx, err := i.authorize(ctx, req.Spec().Procedure, req.Header())
		if err != nil {
			return nil, err
		}
		return next(ctx, req)
	}
}

func (i *Interceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		ctx, err := i.authorize(ctx, conn.Spec().Procedure, conn.RequestHeader())
		if err != nil {
			return err
		}
		return next(ctx, conn)
	}
}

// WrapStreamingClient is a pass-through: this process is the server, and outbound
// client calls carry no postpilot session.
func (i *Interceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

// authorize returns the context downstream handlers should see, or an Unauthenticated
// error. It fails closed: any path that does not explicitly succeed denies.
func (i *Interceptor) authorize(ctx context.Context, procedure string, header http.Header) (context.Context, error) {
	if publicProcedures[procedure] || agentProcedures[procedure] {
		return ctx, nil
	}

	actor, err := i.svc.Authenticate(ctx, cookieValue(header))
	if err != nil {
		if !errors.Is(err, auth.ErrNoSession) {
			// A store failure is not the caller's fault, but it still must not let the
			// request through — deny, and keep the reason in the log rather than in the
			// response.
			slog.Error("session lookup failed", "procedure", procedure, "err", err)
		}
		return nil, rpcserver.NewAppError(connect.CodeUnauthenticated, unauthenticatedMessage, "AUTH_REQUIRED", nil)
	}

	if masterProcedures[procedure] && actor.Plan != plan.Master {
		return nil, rpcserver.NewAppError(connect.CodePermissionDenied, masterOnlyMessage, plan.ReasonMasterOnly, nil)
	}

	return auth.WithActor(ctx, actor), nil
}

var _ connect.Interceptor = (*Interceptor)(nil)
