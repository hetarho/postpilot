package rpc

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/experiment"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
	"github.com/postpilot/backend/internal/platform/rpcserver"
)

// Publication is deliberately separate: champion decisions do not have a target mutation port.
type writingTestActions interface {
	Estimate(context.Context, experiment.TestStart) (experiment.TestQuote, error)
	EstimateFailed(context.Context, experiment.TestRetryQuoteRequest) (experiment.TestQuote, error)
	Start(context.Context, experiment.TestStart) (experiment.WritingTest, error)
	Get(context.Context, string, string) (experiment.WritingTest, error)
	List(context.Context, string, int, string) ([]experiment.WritingTest, string, error)
	Retry(context.Context, experiment.TestRetry) (experiment.WritingTest, error)
	Decide(context.Context, experiment.MatchDecision) (experiment.WritingTest, error)
	Cancel(context.Context, experiment.TestMutation) (experiment.WritingTest, error)
}
type writingTestPublicationActions interface {
	SaveWinner(context.Context, experiment.WinnerPublication) (experiment.TestPublication, experiment.WritingTest, error)
	ApplyOutput(context.Context, experiment.OutputApplication) (experiment.TestPublication, experiment.WritingTest, error)
}
type WritingTestHandler struct {
	tests       writingTestActions
	publication writingTestPublicationActions
}

func NewWritingTestHandler(tests writingTestActions, publication writingTestPublicationActions) *WritingTestHandler {
	if tests == nil || publication == nil {
		panic("experiment: writing test actions and publication are required")
	}
	return &WritingTestHandler{tests: tests, publication: publication}
}
func (h *WritingTestHandler) EstimateWritingTest(ctx context.Context, req *connect.Request[v1.EstimateWritingTestRequest]) (*connect.Response[v1.EstimateWritingTestResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	start, err := writingStartFromProto(user, req.Msg.GetPlan())
	if err != nil {
		return nil, writingError(err)
	}
	quote, err := h.tests.Estimate(ctx, start)
	if err != nil {
		return nil, writingError(err)
	}
	return writingQuoteResponse(quote)
}
func (h *WritingTestHandler) EstimateFailedTestCandidates(ctx context.Context, req *connect.Request[v1.EstimateFailedTestCandidatesRequest]) (*connect.Response[v1.EstimateWritingTestResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	quote, err := h.tests.EstimateFailed(ctx, experiment.TestRetryQuoteRequest{UserID: user, TestID: req.Msg.GetTestId(), ExpectedRevision: req.Msg.GetExpectedRevision(), CandidateIDs: req.Msg.GetCandidateIds()})
	if err != nil {
		return nil, writingError(err)
	}
	return writingQuoteResponse(quote)
}
func writingQuoteResponse(quote experiment.TestQuote) (*connect.Response[v1.EstimateWritingTestResponse], error) {
	if quote.Key == "" || quote.Credits < 0 || quote.ExpiresAt.IsZero() {
		return nil, writingError(experiment.ErrTestQuoteRequired)
	}
	credits := int64(quote.Credits)
	return connect.NewResponse(&v1.EstimateWritingTestResponse{Credits: &credits, Free: quote.Free, QuoteKey: quote.Key, ExpiresAt: formatTime(quote.ExpiresAt)}), nil
}
func (h *WritingTestHandler) StartWritingTest(ctx context.Context, req *connect.Request[v1.StartWritingTestRequest]) (*connect.Response[v1.WritingTestResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	start, err := writingStartFromProto(user, req.Msg.GetPlan())
	if err != nil {
		return nil, writingError(err)
	}
	start.RequestKey, start.QuoteKey = req.Msg.GetRequestKey(), req.Msg.GetQuoteKey()
	found, err := h.tests.Start(ctx, start)
	return writingResponse(found, err)
}
func (h *WritingTestHandler) GetWritingTest(ctx context.Context, req *connect.Request[v1.GetWritingTestRequest]) (*connect.Response[v1.WritingTestResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	found, err := h.tests.Get(ctx, user, req.Msg.GetTestId())
	return writingResponse(found, err)
}
func (h *WritingTestHandler) ListWritingTests(ctx context.Context, req *connect.Request[v1.ListWritingTestsRequest]) (*connect.Response[v1.ListWritingTestsResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	found, next, err := h.tests.List(ctx, user, int(req.Msg.GetPageSize()), req.Msg.GetPageToken())
	if err != nil {
		return nil, writingError(err)
	}
	result := &v1.ListWritingTestsResponse{NextPageToken: next}
	for _, test := range found {
		wire, err := writingTestToProto(test)
		if err != nil {
			return nil, writingError(err)
		}
		result.Tests = append(result.Tests, wire)
	}
	return connect.NewResponse(result), nil
}
func (h *WritingTestHandler) RetryFailedTestCandidates(ctx context.Context, req *connect.Request[v1.RetryFailedTestCandidatesRequest]) (*connect.Response[v1.WritingTestResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	found, err := h.tests.Retry(ctx, experiment.TestRetry{TestMutation: experiment.TestMutation{UserID: user, TestID: req.Msg.GetTestId(), ExpectedRevision: req.Msg.GetExpectedRevision(), RequestKey: req.Msg.GetRequestKey()}, CandidateIDs: req.Msg.GetCandidateIds(), QuoteKey: req.Msg.GetQuoteKey()})
	return writingResponse(found, err)
}
func (h *WritingTestHandler) DecideTestMatch(ctx context.Context, req *connect.Request[v1.DecideTestMatchRequest]) (*connect.Response[v1.WritingTestResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	found, err := h.tests.Decide(ctx, experiment.MatchDecision{TestMutation: experiment.TestMutation{UserID: user, TestID: req.Msg.GetTestId(), ExpectedRevision: req.Msg.GetExpectedRevision(), RequestKey: req.Msg.GetRequestKey()}, MatchID: req.Msg.GetMatchId(), WinnerCandidateID: req.Msg.GetWinnerCandidateId()})
	return writingResponse(found, err)
}
func (h *WritingTestHandler) CancelWritingTest(ctx context.Context, req *connect.Request[v1.CancelWritingTestRequest]) (*connect.Response[v1.WritingTestResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	found, err := h.tests.Cancel(ctx, experiment.TestMutation{UserID: user, TestID: req.Msg.GetTestId(), ExpectedRevision: req.Msg.GetExpectedRevision(), RequestKey: req.Msg.GetRequestKey()})
	return writingResponse(found, err)
}
func (h *WritingTestHandler) SaveWritingTestWinner(ctx context.Context, req *connect.Request[v1.SaveWritingTestWinnerRequest]) (*connect.Response[v1.WritingTestPublicationResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	action, err := writingActionFromProto(req.Msg.GetAction())
	if err != nil || action == experiment.TestApplyOutput {
		return nil, writingError(experiment.ErrTestOperation)
	}
	publication, test, err := h.publication.SaveWinner(ctx, experiment.WinnerPublication{TestMutation: experiment.TestMutation{UserID: user, TestID: req.Msg.GetTestId(), ExpectedRevision: req.Msg.GetExpectedRevision(), RequestKey: req.Msg.GetRequestKey()}, WinnerID: req.Msg.GetWinnerCandidateId(), Action: string(action), MakeDefault: req.Msg.GetMakeDefault(), Name: req.Msg.GetName(), Scope: req.Msg.GetScope(), ScopeIDs: req.Msg.GetScopeIds()})
	return writingPublicationResponse(publication, test, err)
}
func (h *WritingTestHandler) ApplyWritingTestOutput(ctx context.Context, req *connect.Request[v1.ApplyWritingTestOutputRequest]) (*connect.Response[v1.WritingTestPublicationResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	publication, test, err := h.publication.ApplyOutput(ctx, experiment.OutputApplication{TestMutation: experiment.TestMutation{UserID: user, TestID: req.Msg.GetTestId(), ExpectedRevision: req.Msg.GetExpectedRevision(), RequestKey: req.Msg.GetRequestKey()}, WinnerID: req.Msg.GetWinnerCandidateId(), InputRevision: req.Msg.GetExpectedInputRevision(), ContentRevision: req.Msg.GetExpectedContentRevision()})
	return writingPublicationResponse(publication, test, err)
}
func writingResponse(found experiment.WritingTest, err error) (*connect.Response[v1.WritingTestResponse], error) {
	if err != nil {
		return nil, writingError(err)
	}
	wire, err := writingTestToProto(found)
	if err != nil {
		return nil, writingError(err)
	}
	return connect.NewResponse(&v1.WritingTestResponse{Test: wire}), nil
}
func writingPublicationResponse(publication experiment.TestPublication, found experiment.WritingTest, err error) (*connect.Response[v1.WritingTestPublicationResponse], error) {
	if err != nil {
		return nil, writingError(err)
	}
	wire, err := writingTestToProto(found)
	if err != nil {
		return nil, writingError(err)
	}
	receipt, err := writingPublicationToProto(publication)
	if err != nil {
		return nil, writingError(err)
	}
	return connect.NewResponse(&v1.WritingTestPublicationResponse{Publication: receipt, Test: wire}), nil
}
func writingError(err error) error {
	var refusal *experiment.TestRefusal
	if errors.As(err, &refusal) {
		code := connect.CodeFailedPrecondition
		switch {
		case errors.Is(err, experiment.ErrTestNotFound):
			code = connect.CodeNotFound
		case errors.Is(err, experiment.ErrTestRevisionConflict), errors.Is(err, experiment.ErrTestDecisionConflict), errors.Is(err, experiment.ErrTestPublicationConflict):
			code = connect.CodeAborted
		case errors.Is(err, experiment.ErrTestCount), errors.Is(err, experiment.ErrTestFactor), errors.Is(err, experiment.ErrTestEntrant), errors.Is(err, experiment.ErrTestDuplicate), errors.Is(err, experiment.ErrTestOperation), errors.Is(err, experiment.ErrTestMatchInvalid), errors.Is(err, experiment.ErrTestMaterialInvalid):
			code = connect.CodeInvalidArgument
		}
		return rpcserver.AppErrorFrom(code, refusal)
	}
	if mapped, ok := writingTargetError(err); ok {
		return mapped
	}
	// Reuse the established allowlisted entitlement/credit/required-input edge.
	return toConnectError("writing test operation", err)
}

var _ postpilotv1connect.WritingTestServiceHandler = (*WritingTestHandler)(nil)
