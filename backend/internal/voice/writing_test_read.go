package voice

import "context"

type FrozenTestProfile struct {
	Voice      Voice
	Analysis   Analysis
	Projection PromptProfile
	Revision   string
}

// FreezeTestProfile projects the same accepted analysis it returns. Pending
// source edits never substitute newer prose, and withdrawn examples are omitted
// by the ordinary projection shared with this owner-side snapshot read.
func (s *Service) FreezeTestProfile(ctx context.Context, user, id, revision string, target Language, topic string) (FrozenTestProfile, error) {
	if !target.Valid() {
		return FrozenTestProfile{}, ErrLanguageRequired
	}
	found, err := s.activeVoice(ctx, user, id)
	if err != nil {
		return FrozenTestProfile{}, err
	}
	analysis, err := s.analyses.CurrentAnalysis(ctx, user, id)
	if err != nil {
		return FrozenTestProfile{}, err
	}
	if analysis == nil {
		return FrozenTestProfile{}, ErrVoiceNotMade
	}
	accepted := AcceptedAnalysisRevision(*analysis)
	if revision != "" && revision != accepted {
		return FrozenTestProfile{}, ErrTestStylePublicationConflict
	}
	var samples []Sample
	if target == LanguageKorean && NormalizedOrigin(analysis.Origin) != OriginSynthetic {
		samples, err = s.samples.ListSampleBodies(ctx, user, id)
		if err != nil {
			return FrozenTestProfile{}, err
		}
	}
	projection, err := projectAcceptedProfile(*analysis, samples, topic, target, "")
	if err != nil {
		return FrozenTestProfile{}, err
	}
	return FrozenTestProfile{Voice: found, Analysis: *analysis, Projection: projection, Revision: accepted}, nil
}

// ProjectSyntheticTestStyle consumes only a validated fictional style factory
// result. Personal source material cannot enter this unpublished contender path.
func ProjectSyntheticTestStyle(frozen FrozenWritingStyle, target Language) (PromptProfile, error) {
	if !target.Valid() {
		return PromptProfile{}, ErrLanguageRequired
	}
	if NormalizedOrigin(frozen.Analysis.Origin) != OriginSynthetic || len(frozen.Analysis.AcceptedMaterials) != 0 || len(frozen.Analysis.AcceptedSources) != 0 || len(frozen.Analysis.MaterialIDs) != 0 || frozen.Revision != AcceptedAnalysisRevision(frozen.Analysis) {
		return PromptProfile{}, ErrTestStylePublicationConflict
	}
	return projectAcceptedProfile(frozen.Analysis, nil, "", target, "")
}
