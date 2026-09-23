package rpc

import (
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/post"
)

// qualityRuleFromProto maps one tick onto post's closed list. UNSPECIFIED and any number the
// enum does not name are a refusal, never a value: there is no default (ARCH-3).
func qualityRuleFromProto(metric postpilotv1.QualityMetric) (string, bool) {
	switch metric {
	case postpilotv1.QualityMetric_QUALITY_METRIC_TITLE_SATURATION:
		return post.QualityRuleTitleSaturation, true
	case postpilotv1.QualityMetric_QUALITY_METRIC_CROSS_POST_PHRASES:
		return post.QualityRuleCrossPostPhrases, true
	case postpilotv1.QualityMetric_QUALITY_METRIC_IN_POST_REPETITION:
		return post.QualityRuleInPostRepetition, true
	case postpilotv1.QualityMetric_QUALITY_METRIC_COMPOSITION:
		return post.QualityRuleComposition, true
	}
	return "", false
}

func qualityRuleToProto(id string) (postpilotv1.QualityMetric, bool) {
	switch id {
	case post.QualityRuleTitleSaturation:
		return postpilotv1.QualityMetric_QUALITY_METRIC_TITLE_SATURATION, true
	case post.QualityRuleCrossPostPhrases:
		return postpilotv1.QualityMetric_QUALITY_METRIC_CROSS_POST_PHRASES, true
	case post.QualityRuleInPostRepetition:
		return postpilotv1.QualityMetric_QUALITY_METRIC_IN_POST_REPETITION, true
	case post.QualityRuleComposition:
		return postpilotv1.QualityMetric_QUALITY_METRIC_COMPOSITION, true
	}
	return postpilotv1.QualityMetric_QUALITY_METRIC_UNSPECIFIED, false
}
