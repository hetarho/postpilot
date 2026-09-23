// Package rpc is the quality context's authenticated Connect edge.
package rpc

import (
	"context"
	"errors"
	"log/slog"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/auth"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
	"github.com/postpilot/backend/internal/platform/rpcserver"
	"github.com/postpilot/backend/internal/quality"
)

type Handler struct{ service *quality.Service }

func NewHandler(service *quality.Service) *Handler { return &Handler{service: service} }

var _ postpilotv1connect.QualityServiceHandler = (*Handler)(nil)

// GetPostMeasurement answers M2, M3 and M4 in that order, never M1 (QUAL-36). The minimum and
// the published count mean something on M2 only: it is the one per-post metric that reads the
// published window, and its count is the number of others it was judged against.
func (h *Handler) GetPostMeasurement(ctx context.Context, req *connect.Request[postpilotv1.GetPostMeasurementRequest]) (*connect.Response[postpilotv1.GetPostMeasurementResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	reading, err := h.service.PostMeasurement(ctx, userID, req.Msg.GetSlug())
	if err != nil {
		return nil, toConnectError("get post measurement", err)
	}
	m := reading.Measurement
	crossPost := &postpilotv1.QualityReading{
		Metric: toProtoMetric(quality.MetricCrossPostPhrases), Verdict: toProtoVerdict(m.CrossPost.Verdict),
		Minimum: int32(quality.Minimum(quality.MetricCrossPostPhrases)), PublishedCount: int32(m.CrossPost.Others),
		Values: crossPostValues(m.CrossPost.Share),
	}
	repetition := &postpilotv1.QualityReading{
		Metric: toProtoMetric(quality.MetricInPostRepetition), Verdict: toProtoVerdict(m.Repetition.Verdict),
		Values: repetitionValues(m.Repetition.Repetition.Share, m.Repetition.Repetition.TitleRelevance),
	}
	composition := &postpilotv1.QualityReading{
		Metric: toProtoMetric(quality.MetricComposition), Verdict: toProtoVerdict(m.Composition.Verdict),
		Values: compositionValues(postComposition(m.Composition.Composition)),
	}
	return connect.NewResponse(&postpilotv1.GetPostMeasurementResponse{
		ContentRevision: reading.Revision,
		Readings:        []*postpilotv1.QualityReading{crossPost, repetition, composition},
	}), nil
}

// GetAccountQuality answers all four metrics in enum order (POST-81). Only an over-band metric
// carries its rule text, rendered in the target language of the post the brief belongs to.
func (h *Handler) GetAccountQuality(ctx context.Context, req *connect.Request[postpilotv1.GetAccountQualityRequest]) (*connect.Response[postpilotv1.GetAccountQualityResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	reading, err := h.service.AccountQuality(ctx, userID, req.Msg.GetSlug())
	if err != nil {
		return nil, toConnectError("get account quality", err)
	}
	a := reading.Account
	readings := make([]*postpilotv1.QualityReading, 0, len(quality.Metrics()))
	for _, m := range quality.Metrics() {
		row := &postpilotv1.QualityReading{
			Metric: toProtoMetric(m), Verdict: toProtoVerdict(a.Verdict(m)),
			Minimum: int32(quality.Minimum(m)), PublishedCount: int32(a.PublishedCount),
			RuleText: reading.RuleTexts[m],
		}
		switch m {
		case quality.MetricTitleSaturation:
			row.Values = &postpilotv1.QualityReading_TitleSaturation{TitleSaturation: &postpilotv1.QualityTitleSaturation{
				Share: a.TitleSaturation.Value, ShareWarnAbove: quality.TitleSaturationBand,
			}}
		case quality.MetricCrossPostPhrases:
			row.Values = crossPostValues(a.CrossPost.Median)
		case quality.MetricInPostRepetition:
			row.Values = repetitionValues(a.Repetition.Share, a.Repetition.TitleRelevance)
		case quality.MetricComposition:
			c := a.Composition
			row.Values = compositionValues(c.CharCount, c.PhotoCount, c.DistinctBlockTypes, c.AvgSentenceLength)
		}
		readings = append(readings, row)
	}
	return connect.NewResponse(&postpilotv1.GetAccountQualityResponse{Readings: readings}), nil
}

func crossPostValues(share *float64) *postpilotv1.QualityReading_CrossPostPhrases {
	return &postpilotv1.QualityReading_CrossPostPhrases{CrossPostPhrases: &postpilotv1.QualityCrossPostPhrases{
		Share: share, ShareWarnAbove: quality.CrossPostPhrasesBand,
	}}
}

// repetitionValues carries both of M3's bands: it is over band when either half crosses.
func repetitionValues(share, relevance *float64) *postpilotv1.QualityReading_InPostRepetition {
	return &postpilotv1.QualityReading_InPostRepetition{InPostRepetition: &postpilotv1.QualityInPostRepetition{
		RepetitionShare: share, TitleRelevance: relevance,
		RepetitionShareWarnAbove: quality.RepetitionShareBand, TitleRelevanceWarnBelow: quality.TitleRelevanceFloor,
	}}
}

func compositionValues(chars, photos, types, sentence *float64) *postpilotv1.QualityReading_Composition {
	return &postpilotv1.QualityReading_Composition{Composition: &postpilotv1.QualityComposition{
		CharCount: chars, PhotoCount: photos, DistinctBlockTypes: types, AverageSentenceLength: sentence,
		DistinctBlockTypesWarnAtOrBelow: quality.DistinctBlockTypesBand,
	}}
}

// postComposition spreads one post's counts over the account's four optional values; a post
// with no content has none of them.
func postComposition(c *quality.Composition) (chars, photos, types, sentence *float64) {
	if c == nil {
		return nil, nil, nil, nil
	}
	return count(c.CharCount), count(c.PhotoCount), count(c.DistinctBlockTypes), c.AvgSentenceLength
}

func count(n int) *float64 {
	value := float64(n)
	return &value
}

// toProtoMetric has one case per metric and no default: the trailing UNSPECIFIED is reached by
// no domain value, which the walk test proves (ARCH-3).
func toProtoMetric(m quality.Metric) postpilotv1.QualityMetric {
	switch m {
	case quality.MetricTitleSaturation:
		return postpilotv1.QualityMetric_QUALITY_METRIC_TITLE_SATURATION
	case quality.MetricCrossPostPhrases:
		return postpilotv1.QualityMetric_QUALITY_METRIC_CROSS_POST_PHRASES
	case quality.MetricInPostRepetition:
		return postpilotv1.QualityMetric_QUALITY_METRIC_IN_POST_REPETITION
	case quality.MetricComposition:
		return postpilotv1.QualityMetric_QUALITY_METRIC_COMPOSITION
	}
	return postpilotv1.QualityMetric_QUALITY_METRIC_UNSPECIFIED
}

func toProtoVerdict(v quality.Verdict) postpilotv1.QualityVerdict {
	switch v {
	case quality.VerdictOverBand:
		return postpilotv1.QualityVerdict_QUALITY_VERDICT_OVER_BAND
	case quality.VerdictWithinBand:
		return postpilotv1.QualityVerdict_QUALITY_VERDICT_WITHIN_BAND
	case quality.VerdictBelowMinimum:
		return postpilotv1.QualityVerdict_QUALITY_VERDICT_BELOW_MINIMUM
	case quality.VerdictAbsent:
		return postpilotv1.QualityVerdict_QUALITY_VERDICT_ABSENT
	}
	return postpilotv1.QualityVerdict_QUALITY_VERDICT_UNSPECIFIED
}

func actingUser(ctx context.Context) (string, error) {
	userID, ok := auth.UserFromContext(ctx)
	if !ok {
		return "", rpcserver.NewAppError(connect.CodeUnauthenticated, "authentication required", postpilotv1.FailureReason_AUTH_REQUIRED, nil)
	}
	return userID, nil
}

// toConnectError maps the context's one sentinel. A foreign post is NotFound like an unknown
// one: the two must not be distinguishable.
func toConnectError(op string, err error) error {
	if errors.Is(err, quality.ErrPostNotFound) {
		return rpcserver.NewAppError(connect.CodeNotFound, "post not found", postpilotv1.FailureReason_POST_NOT_FOUND, nil)
	}
	slog.Error(op+" failed", "err", err)
	return rpcserver.NewAppError(connect.CodeInternal, op+" failed", postpilotv1.FailureReason_UNKNOWN_FAILURE, nil)
}
