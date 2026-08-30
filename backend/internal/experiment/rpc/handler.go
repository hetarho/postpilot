// Package rpc is the experiment transport edge. It enforces blind responses by never
// mapping model identity/accounting until the aggregate is terminal.
package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/experiment"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
)

type Handler struct{ service *experiment.Service }

func NewHandler(service *experiment.Service) *Handler { return &Handler{service: service} }

func (h *Handler) StartObserveExperiment(ctx context.Context, req *connect.Request[postpilotv1.StartObserveExperimentRequest]) (*connect.Response[postpilotv1.StartExperimentResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	started, err := h.service.Start(ctx, experiment.StartRequest{
		UserID: userID, PostSlug: req.Msg.GetPostSlug(), Stage: experiment.StageObserve,
		ModelA: fromProtoRef(req.Msg.GetModelA()), ModelB: fromProtoRef(req.Msg.GetModelB()),
	})
	if err != nil {
		return nil, toConnectError("start observe experiment", err)
	}
	return connect.NewResponse(&postpilotv1.StartExperimentResponse{ExperimentId: started.ExperimentID, JobId: started.JobID}), nil
}

func (h *Handler) StartAnalyzeExperiment(ctx context.Context, req *connect.Request[postpilotv1.StartAnalyzeExperimentRequest]) (*connect.Response[postpilotv1.StartExperimentResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	started, err := h.service.Start(ctx, experiment.StartRequest{
		UserID: userID, Stage: experiment.StageAnalyze, VoiceID: req.Msg.GetVoiceId(),
		ModelA: fromProtoRef(req.Msg.GetModelA()), ModelB: fromProtoRef(req.Msg.GetModelB()),
	})
	if err != nil {
		return nil, toConnectError("start analyze experiment", err)
	}
	return connect.NewResponse(&postpilotv1.StartExperimentResponse{ExperimentId: started.ExperimentID, JobId: started.JobID}), nil
}

func (h *Handler) StartWriteExperiment(ctx context.Context, req *connect.Request[postpilotv1.StartWriteExperimentRequest]) (*connect.Response[postpilotv1.StartExperimentResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	started, err := h.service.Start(ctx, experiment.StartRequest{
		UserID: userID, PostSlug: req.Msg.GetPostSlug(), Stage: experiment.StageWrite,
		ObserveModel: fromProtoRef(req.Msg.GetObserveModel()), ModelA: fromProtoRef(req.Msg.GetModelA()), ModelB: fromProtoRef(req.Msg.GetModelB()),
		TargetLength: optionalTargetLength(req.Msg.TargetLength),
	})
	if err != nil {
		return nil, toConnectError("start write experiment", err)
	}
	return connect.NewResponse(&postpilotv1.StartExperimentResponse{ExperimentId: started.ExperimentID, JobId: started.JobID}), nil
}

func (h *Handler) GetExperiment(ctx context.Context, req *connect.Request[postpilotv1.GetExperimentRequest]) (*connect.Response[postpilotv1.GetExperimentResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	found, err := h.service.Get(ctx, userID, req.Msg.GetId())
	if err != nil {
		return nil, toConnectError("get experiment", err)
	}
	return connect.NewResponse(&postpilotv1.GetExperimentResponse{Experiment: toProtoExperiment(found)}), nil
}

func (h *Handler) ListExperiments(ctx context.Context, req *connect.Request[postpilotv1.ListExperimentsRequest]) (*connect.Response[postpilotv1.ListExperimentsResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	stage := fromProtoStage(req.Msg.GetStage())
	found, err := h.service.List(ctx, userID, stage)
	if err != nil {
		return nil, toConnectError("list experiments", err)
	}
	out := make([]*postpilotv1.ModelExperiment, 0, len(found))
	for _, item := range found {
		out = append(out, toProtoExperiment(item))
	}
	return connect.NewResponse(&postpilotv1.ListExperimentsResponse{Experiments: out}), nil
}

func (h *Handler) RetryCandidate(ctx context.Context, req *connect.Request[postpilotv1.RetryCandidateRequest]) (*connect.Response[postpilotv1.RetryCandidateResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	started, err := h.service.Retry(ctx, userID, req.Msg.GetExperimentId())
	if err != nil {
		return nil, toConnectError("retry candidate", err)
	}
	found, err := h.service.Get(ctx, userID, started.ExperimentID)
	if err != nil {
		return nil, toConnectError("get retried experiment", err)
	}
	return connect.NewResponse(&postpilotv1.RetryCandidateResponse{JobId: started.JobID, Experiment: toProtoExperiment(found)}), nil
}

func (h *Handler) ChooseWinner(ctx context.Context, req *connect.Request[postpilotv1.ChooseWinnerRequest]) (*connect.Response[postpilotv1.ChooseWinnerResponse], error) {
	return h.choose(ctx, req.Msg.GetExperimentId(), req.Msg.GetCandidateId(), false)
}

func (h *Handler) UseSingleCandidate(ctx context.Context, req *connect.Request[postpilotv1.UseSingleCandidateRequest]) (*connect.Response[postpilotv1.ChooseWinnerResponse], error) {
	return h.choose(ctx, req.Msg.GetExperimentId(), req.Msg.GetCandidateId(), true)
}

func (h *Handler) choose(ctx context.Context, experimentID, candidateID string, single bool) (*connect.Response[postpilotv1.ChooseWinnerResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	found, err := h.service.Choose(ctx, userID, experimentID, candidateID, single)
	if err != nil {
		return nil, toConnectError("choose experiment candidate", err)
	}
	return connect.NewResponse(&postpilotv1.ChooseWinnerResponse{Experiment: toProtoExperiment(found)}), nil
}

func (h *Handler) DismissExperiment(ctx context.Context, req *connect.Request[postpilotv1.DismissExperimentRequest]) (*connect.Response[postpilotv1.DismissExperimentResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	found, err := h.service.Dismiss(ctx, userID, req.Msg.GetExperimentId())
	if err != nil {
		return nil, toConnectError("dismiss experiment", err)
	}
	return connect.NewResponse(&postpilotv1.DismissExperimentResponse{Experiment: toProtoExperiment(found)}), nil
}

func (h *Handler) ApplyWinnerOutput(ctx context.Context, req *connect.Request[postpilotv1.ApplyWinnerOutputRequest]) (*connect.Response[postpilotv1.ApplyWinnerOutputResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	found, err := h.service.ApplyWinner(ctx, userID, req.Msg.GetExperimentId(), req.Msg.GetConfirmStyleguideOverwrite())
	if err != nil {
		return nil, toConnectError("apply experiment winner", err)
	}
	return connect.NewResponse(&postpilotv1.ApplyWinnerOutputResponse{Experiment: toProtoExperiment(found)}), nil
}

func (h *Handler) AdoptWinnerModel(ctx context.Context, req *connect.Request[postpilotv1.AdoptWinnerModelRequest]) (*connect.Response[postpilotv1.AdoptWinnerModelResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	ref, stage, err := h.service.AdoptWinner(ctx, userID, req.Msg.GetExperimentId())
	if err != nil {
		return nil, toConnectError("adopt experiment winner", err)
	}
	return connect.NewResponse(&postpilotv1.AdoptWinnerModelResponse{Selection: &postpilotv1.Selection{
		Stage: toProtoStage(stage), Ref: toProtoRef(ref), Slot: postpilotv1.SelectionSlot_SELECTION_SLOT_ACTIVE,
	}}), nil
}

func (h *Handler) DecideWriteExperiment(ctx context.Context, req *connect.Request[postpilotv1.DecideWriteExperimentRequest]) (*connect.Response[postpilotv1.ChooseWinnerResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	found, err := h.service.DecideWrite(ctx, userID, req.Msg.GetExperimentId(), req.Msg.GetCandidateId(), req.Msg.GetAdoptWinnerModel())
	if err != nil {
		return nil, toConnectError("decide write experiment", err)
	}
	return connect.NewResponse(&postpilotv1.ChooseWinnerResponse{Experiment: toProtoExperiment(found)}), nil
}

func (h *Handler) GetLeaderboard(ctx context.Context, req *connect.Request[postpilotv1.GetLeaderboardRequest]) (*connect.Response[postpilotv1.GetLeaderboardResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	entries, err := h.service.Leaderboard(ctx, userID, fromProtoStage(req.Msg.GetStage()))
	if err != nil {
		return nil, toConnectError("get leaderboard", err)
	}
	out := make([]*postpilotv1.LeaderboardEntry, 0, len(entries))
	for _, entry := range entries {
		out = append(out, &postpilotv1.LeaderboardEntry{
			Rank: int32(entry.Rank), Model: toProtoRef(entry.Model), ModelLabel: entry.ModelLabel,
			Rating: int32(entry.Rating), Matches: int32(entry.Matches), Wins: int32(entry.Wins), Losses: int32(entry.Losses),
			WinRate: entry.WinRate(), SuccessfulCalls: int32(entry.SuccessfulCalls), AverageLatencyMs: entry.AverageLatencyMS(),
			PromptTokens: entry.PromptTokens, CompletionTokens: entry.CompletionTokens, TotalCostMicrousd: entry.TotalCostMicrousd,
			CostQuality: toProtoCost(entry.CostQuality), Provisional: entry.Provisional, Active: entry.Active,
			Recommended: entry.Recommended, Disappeared: entry.Disappeared,
		})
	}
	return connect.NewResponse(&postpilotv1.GetLeaderboardResponse{Entries: out}), nil
}

func actingUser(ctx context.Context) (string, error) {
	userID, ok := auth.UserFromContext(ctx)
	if !ok {
		return "", connect.NewError(connect.CodeUnauthenticated, errors.New("unauthenticated"))
	}
	return userID, nil
}

func toConnectError(op string, err error) error {
	var active *experiment.JobAlreadyInProgressError
	switch {
	case errors.Is(err, experiment.ErrNotFound), errors.Is(err, experiment.ErrCandidateNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, experiment.ErrForbidden):
		return connect.NewError(connect.CodePermissionDenied, errors.New("not yours"))
	case errors.Is(err, experiment.ErrInvalidStage), errors.Is(err, experiment.ErrDuplicateCandidates),
		errors.Is(err, experiment.ErrInvalidTargetLength), errors.Is(err, experiment.ErrVoiceRequired):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, experiment.ErrModelRequired), errors.Is(err, experiment.ErrInvalidState),
		errors.Is(err, experiment.ErrConfirmationRequired), errors.Is(err, experiment.ErrSnapshotUnavailable),
		errors.Is(err, experiment.ErrRetryModelUnavailable), errors.Is(err, experiment.ErrVoiceUnavailable), errors.As(err, &active):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	default:
		slog.Error(op+" failed", "err", err)
		return connect.NewError(connect.CodeInternal, errors.New(op+" failed"))
	}
}

func toProtoExperiment(found experiment.Experiment) *postpilotv1.ModelExperiment {
	candidates := make([]*postpilotv1.ExperimentCandidate, 0, len(found.Candidates))
	for _, candidate := range found.Candidates {
		candidates = append(candidates, toProtoCandidate(found, candidate))
	}
	return &postpilotv1.ModelExperiment{
		Id: found.ID, Stage: toProtoStage(found.Stage), Status: toProtoStatus(found.Status), PostSlug: found.PostSlug,
		VoiceId: found.VoiceID, JobId: found.JobID, Candidates: candidates, WinnerCandidateId: found.WinnerCandidateID,
		Outcome: toProtoOutcome(found.Outcome), ApplyError: found.ApplyError, CreatedAt: formatTime(found.CreatedAt),
		FinishedAt: formatOptional(found.FinishedAt), DecidedAt: formatOptional(found.DecidedAt), Revealed: found.Revealed(),
		AppliedAt:         formatOptional(found.AppliedAt),
		AdoptionRequested: found.AdoptionRequested,
		AdoptionError:     found.AdoptionError, AdoptedAt: formatOptional(found.AdoptedAt),
	}
}

func optionalTargetLength(value *int32) *int {
	if value == nil {
		return nil
	}
	result := int(*value)
	return &result
}

func toProtoCandidate(found experiment.Experiment, candidate experiment.Candidate) *postpilotv1.ExperimentCandidate {
	out := &postpilotv1.ExperimentCandidate{Id: candidate.ID, DisplaySide: toProtoSide(candidate.DisplaySide), Status: toProtoCandidateStatus(candidate.Status)}
	setOutput(out, found.Stage, candidate.Output)
	if found.Revealed() {
		out.Model = toProtoRef(candidate.Model)
		out.ModelLabel = candidate.ModelLabel
		out.Error = candidate.Error
		out.Usage = &postpilotv1.CandidateUsage{PromptTokens: candidate.Usage.PromptTokens, CompletionTokens: candidate.Usage.CompletionTokens,
			CostMicrousd: candidate.Usage.CostMicrousd, CostSource: toProtoCost(candidate.Usage.CostSource), LatencyMs: candidate.Usage.LatencyMS}
	}
	return out
}

type outputPost struct {
	Title   string        `json:"title"`
	Summary string        `json:"summary"`
	Tags    []string      `json:"tags"`
	Blocks  []outputBlock `json:"blocks"`
}
type outputBlock struct {
	Type    string   `json:"type"`
	Content string   `json:"content"`
	Level   int32    `json:"level"`
	File    string   `json:"file"`
	Alt     string   `json:"alt"`
	Caption string   `json:"caption"`
	Items   []string `json:"items"`
}
type outputObservation struct {
	File          string   `json:"file"`
	Scene         string   `json:"scene"`
	Mood          string   `json:"mood"`
	VisibleText   string   `json:"visible_text"`
	Objects       []string `json:"objects"`
	PeoplePresent bool     `json:"people_present"`
}

func setOutput(out *postpilotv1.ExperimentCandidate, stage experiment.Stage, raw []byte) {
	if len(raw) == 0 {
		return
	}
	switch stage {
	case experiment.StageWrite:
		var value outputPost
		if json.Unmarshal(raw, &value) != nil {
			return
		}
		content := &postpilotv1.PostContent{Title: value.Title, Summary: value.Summary, Tags: value.Tags}
		for _, block := range value.Blocks {
			content.Blocks = append(content.Blocks, &postpilotv1.Block{Type: blockType(block.Type), Content: block.Content, Level: block.Level, File: block.File, Alt: block.Alt, Caption: block.Caption, Items: block.Items})
		}
		out.Output = &postpilotv1.ExperimentCandidate_PostContent{PostContent: content}
	case experiment.StageObserve:
		var values []outputObservation
		if json.Unmarshal(raw, &values) != nil {
			return
		}
		set := &postpilotv1.ObservationSet{}
		for _, value := range values {
			set.Observations = append(set.Observations, &postpilotv1.Observation{File: value.File, Scene: value.Scene, Mood: value.Mood, VisibleText: value.VisibleText, Objects: value.Objects, PeoplePresent: value.PeoplePresent})
		}
		out.Output = &postpilotv1.ExperimentCandidate_ObservationSet{ObservationSet: set}
	case experiment.StageAnalyze:
		var value string
		if json.Unmarshal(raw, &value) == nil {
			out.Output = &postpilotv1.ExperimentCandidate_Styleguide{Styleguide: value}
		}
	}
}

func fromProtoRef(ref *postpilotv1.ModelRef) experiment.ModelRef {
	if ref == nil {
		return experiment.ModelRef{}
	}
	return experiment.ModelRef{ProviderID: ref.GetProviderId(), ModelID: ref.GetModelId()}
}
func toProtoRef(ref experiment.ModelRef) *postpilotv1.ModelRef {
	return &postpilotv1.ModelRef{ProviderId: ref.ProviderID, ModelId: ref.ModelID}
}
func fromProtoStage(stage postpilotv1.Stage) experiment.Stage {
	switch stage {
	case postpilotv1.Stage_STAGE_OBSERVE:
		return experiment.StageObserve
	case postpilotv1.Stage_STAGE_ANALYZE:
		return experiment.StageAnalyze
	case postpilotv1.Stage_STAGE_WRITE:
		return experiment.StageWrite
	}
	return ""
}
func toProtoStage(stage experiment.Stage) postpilotv1.Stage {
	switch stage {
	case experiment.StageObserve:
		return postpilotv1.Stage_STAGE_OBSERVE
	case experiment.StageAnalyze:
		return postpilotv1.Stage_STAGE_ANALYZE
	case experiment.StageWrite:
		return postpilotv1.Stage_STAGE_WRITE
	}
	return postpilotv1.Stage_STAGE_UNSPECIFIED
}
func toProtoStatus(status experiment.Status) postpilotv1.ExperimentStatus {
	return map[experiment.Status]postpilotv1.ExperimentStatus{experiment.StatusQueued: postpilotv1.ExperimentStatus_EXPERIMENT_STATUS_QUEUED, experiment.StatusRunning: postpilotv1.ExperimentStatus_EXPERIMENT_STATUS_RUNNING, experiment.StatusReview: postpilotv1.ExperimentStatus_EXPERIMENT_STATUS_REVIEW, experiment.StatusPartial: postpilotv1.ExperimentStatus_EXPERIMENT_STATUS_PARTIAL, experiment.StatusDecided: postpilotv1.ExperimentStatus_EXPERIMENT_STATUS_DECIDED, experiment.StatusDismissed: postpilotv1.ExperimentStatus_EXPERIMENT_STATUS_DISMISSED, experiment.StatusFailed: postpilotv1.ExperimentStatus_EXPERIMENT_STATUS_FAILED}[status]
}
func toProtoSide(side experiment.DisplaySide) postpilotv1.DisplaySide {
	if side == experiment.SideLeft {
		return postpilotv1.DisplaySide_DISPLAY_SIDE_LEFT
	}
	return postpilotv1.DisplaySide_DISPLAY_SIDE_RIGHT
}
func toProtoCandidateStatus(status experiment.CandidateStatus) postpilotv1.CandidateStatus {
	return map[experiment.CandidateStatus]postpilotv1.CandidateStatus{experiment.CandidatePending: postpilotv1.CandidateStatus_CANDIDATE_STATUS_PENDING, experiment.CandidateRunning: postpilotv1.CandidateStatus_CANDIDATE_STATUS_RUNNING, experiment.CandidateSucceeded: postpilotv1.CandidateStatus_CANDIDATE_STATUS_SUCCEEDED, experiment.CandidateFailed: postpilotv1.CandidateStatus_CANDIDATE_STATUS_FAILED}[status]
}
func toProtoOutcome(outcome experiment.Outcome) postpilotv1.ExperimentOutcome {
	return map[experiment.Outcome]postpilotv1.ExperimentOutcome{experiment.OutcomeWinner: postpilotv1.ExperimentOutcome_EXPERIMENT_OUTCOME_WINNER, experiment.OutcomeSkipped: postpilotv1.ExperimentOutcome_EXPERIMENT_OUTCOME_SKIPPED, experiment.OutcomeUnpaired: postpilotv1.ExperimentOutcome_EXPERIMENT_OUTCOME_UNPAIRED}[outcome]
}
func toProtoCost(source experiment.CostSource) postpilotv1.CostSource {
	return map[experiment.CostSource]postpilotv1.CostSource{experiment.CostReported: postpilotv1.CostSource_COST_SOURCE_REPORTED, experiment.CostEstimated: postpilotv1.CostSource_COST_SOURCE_ESTIMATED, experiment.CostUnavailable: postpilotv1.CostSource_COST_SOURCE_UNAVAILABLE, experiment.CostMixed: postpilotv1.CostSource_COST_SOURCE_MIXED}[source]
}
func blockType(value string) postpilotv1.BlockType {
	return map[string]postpilotv1.BlockType{"TEXT": postpilotv1.BlockType_TEXT, "HEADING": postpilotv1.BlockType_HEADING, "IMAGE": postpilotv1.BlockType_IMAGE, "QUOTE": postpilotv1.BlockType_QUOTE, "LIST": postpilotv1.BlockType_LIST}[value]
}
func formatTime(value time.Time) string { return value.UTC().Format(time.RFC3339) }
func formatOptional(value *time.Time) string {
	if value == nil {
		return ""
	}
	return formatTime(*value)
}

var _ postpilotv1connect.ModelExperimentServiceHandler = (*Handler)(nil)
