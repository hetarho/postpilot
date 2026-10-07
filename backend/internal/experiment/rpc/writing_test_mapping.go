package rpc

import (
	"fmt"

	"github.com/postpilot/backend/internal/experiment"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
)

func writingStartFromProto(user string, p *v1.WritingTestPlan) (experiment.TestStart, error) {
	out := experiment.TestStart{UserID: user, Count: int(p.GetCount())}
	switch p.GetFactor() {
	case v1.WritingTestFactor_WRITING_TEST_FACTOR_MODEL:
		out.Factor = experiment.FactorModel
	case v1.WritingTestFactor_WRITING_TEST_FACTOR_VOICE:
		out.Factor = experiment.FactorVoice
	case v1.WritingTestFactor_WRITING_TEST_FACTOR_TEMPLATE:
		out.Factor = experiment.FactorTemplate
	case v1.WritingTestFactor_WRITING_TEST_FACTOR_GUIDELINE:
		out.Factor = experiment.FactorGuideline
	default:
		return out, experiment.ErrTestFactor
	}
	switch p.GetModelStage() {
	case v1.WritingTestStage_WRITING_TEST_STAGE_UNSPECIFIED:
	case v1.WritingTestStage_WRITING_TEST_STAGE_OBSERVE:
		out.ModelStage = experiment.StageObserve
	case v1.WritingTestStage_WRITING_TEST_STAGE_WRITE:
		out.ModelStage = experiment.StageWrite
	default:
		return out, experiment.ErrTestFactor
	}
	for _, item := range p.GetEntrants() {
		ref, err := writingEntrantFromProto(item)
		if err != nil {
			return out, err
		}
		out.Entrants = append(out.Entrants, ref)
	}
	c := p.GetContext()
	if c == nil {
		return out, experiment.ErrTestMaterialInvalid
	}
	out.Input = experiment.TestInput{SourcePostSlug: c.GetSourcePostSlug(), InputRevision: c.GetExpectedInputRevision(), ContentRevision: c.GetExpectedContentRevision(), ObserveModel: fromProtoRef(c.GetObserveModel()), WriteModel: fromProtoRef(c.GetWriteModel()), VoiceID: c.GetVoiceId(), TemplateID: c.GetTemplateId(), GuidelineSlotID: c.GetGuidelineSlotId(), TargetLength: int(c.GetTargetLength()), TagCount: int(c.GetTagCount()), UseMemory: c.GetUseMemory()}
	switch c.GetTargetLanguage() {
	case v1.ContentLanguage_CONTENT_LANGUAGE_KOREAN:
		out.Input.TargetLanguage = "ko"
	case v1.ContentLanguage_CONTENT_LANGUAGE_ENGLISH:
		out.Input.TargetLanguage = "en"
	default:
		return out, experiment.ErrTestMaterialInvalid
	}
	if m := c.GetMaterial(); m != nil {
		out.Input.Material = m.GetText()
		out.Input.Fictional = m.GetFictional()
		out.Input.AttachmentIDs = append([]string(nil), m.GetAttachmentIds()...)
		for _, a := range m.GetTemplateAnswers() {
			if a == nil {
				return out, experiment.ErrTestMaterialInvalid
			}
			out.Input.TemplateAnswers = append(out.Input.TemplateAnswers, experiment.TestAnswer{Label: a.GetLabel(), Answer: a.GetText(), Enabled: a.GetEnabled()})
		}
	}
	for _, metric := range c.GetQualityRules() {
		var key string
		switch metric {
		case v1.QualityMetric_QUALITY_METRIC_TITLE_SATURATION:
			key = "title_saturation"
		case v1.QualityMetric_QUALITY_METRIC_CROSS_POST_PHRASES:
			key = "cross_post_phrases"
		case v1.QualityMetric_QUALITY_METRIC_IN_POST_REPETITION:
			key = "in_post_repetition"
		case v1.QualityMetric_QUALITY_METRIC_COMPOSITION:
			key = "composition"
		default:
			return out, experiment.ErrTestMaterialInvalid
		}
		out.Input.QualityRules = append(out.Input.QualityRules, key)
	}
	shape := out
	shape.RequestKey = "transport-shape"
	if err := experiment.ValidateTestShape(shape); err != nil {
		return out, err
	}
	return out, nil
}
func writingEntrantFromProto(p *v1.WritingTestEntrant) (experiment.TestEntrantRef, error) {
	var out experiment.TestEntrantRef
	if p == nil {
		return out, experiment.ErrTestEntrant
	}
	switch source := p.GetSource().(type) {
	case *v1.WritingTestEntrant_Model:
		out.SourceKind = "model"
		out.Model = fromProtoRef(source.Model)
	case *v1.WritingTestEntrant_Setting:
		if source.Setting == nil {
			return out, experiment.ErrTestEntrant
		}
		out.SourceKind = "setting"
		kind, err := writingSettingKindFromProto(source.Setting.GetKind())
		if err != nil {
			return out, err
		}
		out.SettingKind = kind
		out.SettingID = source.Setting.GetId()
		out.SettingRevision = source.Setting.GetRevision()
	case *v1.WritingTestEntrant_AuthoringCandidate:
		if source.AuthoringCandidate == nil {
			return out, experiment.ErrTestEntrant
		}
		out.SourceKind = "authoring_candidate"
		out.AuthoringSessionID = source.AuthoringCandidate.GetSessionId()
		out.AuthoringCandidateID = source.AuthoringCandidate.GetCandidateId()
		out.AuthoringRevision = source.AuthoringCandidate.GetRevision()
	default:
		return out, experiment.ErrTestEntrant
	}
	return out, nil
}
func writingSettingKindFromProto(k v1.ConfigurationKind) (string, error) {
	switch k {
	case v1.ConfigurationKind_CONFIGURATION_KIND_WRITING_VOICE:
		return "voice", nil
	case v1.ConfigurationKind_CONFIGURATION_KIND_POST_TEMPLATE:
		return "template", nil
	case v1.ConfigurationKind_CONFIGURATION_KIND_POST_GUIDELINE:
		return "guideline", nil
	}
	return "", experiment.ErrTestEntrant
}
func writingEntrantToProto(ref experiment.TestEntrantRef) (*v1.WritingTestEntrant, error) {
	out := &v1.WritingTestEntrant{}
	switch ref.SourceKind {
	case "model":
		out.Source = &v1.WritingTestEntrant_Model{Model: toProtoRef(ref.Model)}
	case "setting":
		var kind v1.ConfigurationKind
		switch ref.SettingKind {
		case "voice":
			kind = v1.ConfigurationKind_CONFIGURATION_KIND_WRITING_VOICE
		case "template":
			kind = v1.ConfigurationKind_CONFIGURATION_KIND_POST_TEMPLATE
		case "guideline":
			kind = v1.ConfigurationKind_CONFIGURATION_KIND_POST_GUIDELINE
		default:
			return nil, experiment.ErrTestEntrant
		}
		out.Source = &v1.WritingTestEntrant_Setting{Setting: &v1.WritingTestSettingRef{Kind: kind, Id: ref.SettingID, Revision: ref.SettingRevision}}
	case "authoring_candidate":
		out.Source = &v1.WritingTestEntrant_AuthoringCandidate{AuthoringCandidate: &v1.WritingTestAuthoringRef{SessionId: ref.AuthoringSessionID, CandidateId: ref.AuthoringCandidateID, Revision: ref.AuthoringRevision}}
	default:
		return nil, experiment.ErrTestEntrant
	}
	return out, nil
}
func writingActionFromProto(p v1.WritingTestPublicationAction) (experiment.TestPublicationAction, error) {
	switch p {
	case v1.WritingTestPublicationAction_WRITING_TEST_PUBLICATION_ACTION_SAVE_SETTING:
		return experiment.TestSaveSetting, nil
	case v1.WritingTestPublicationAction_WRITING_TEST_PUBLICATION_ACTION_USE_SETTING:
		return experiment.TestUseSetting, nil
	case v1.WritingTestPublicationAction_WRITING_TEST_PUBLICATION_ACTION_ADOPT_MODEL:
		return experiment.TestAdoptModel, nil
	case v1.WritingTestPublicationAction_WRITING_TEST_PUBLICATION_ACTION_APPLY_OUTPUT:
		return experiment.TestApplyOutput, nil
	}
	return "", experiment.ErrTestOperation
}
func writingPublicationToProto(p experiment.TestPublication) (*v1.WritingTestPublication, error) {
	out := &v1.WritingTestPublication{Id: p.ID, TestId: p.TestID, WinnerCandidateId: p.WinnerID, RequestKey: p.RequestKey, TargetId: p.TargetID}
	switch experiment.TestPublicationAction(p.Action) {
	case experiment.TestSaveSetting:
		out.Action = v1.WritingTestPublicationAction_WRITING_TEST_PUBLICATION_ACTION_SAVE_SETTING
	case experiment.TestUseSetting:
		out.Action = v1.WritingTestPublicationAction_WRITING_TEST_PUBLICATION_ACTION_USE_SETTING
	case experiment.TestAdoptModel:
		out.Action = v1.WritingTestPublicationAction_WRITING_TEST_PUBLICATION_ACTION_ADOPT_MODEL
	case experiment.TestApplyOutput:
		out.Action = v1.WritingTestPublicationAction_WRITING_TEST_PUBLICATION_ACTION_APPLY_OUTPUT
	default:
		return nil, experiment.ErrTestOperation
	}
	switch experiment.TestPublicationStatus(p.Status) {
	case experiment.TestPublicationPending:
		out.Status = v1.WritingTestPublicationStatus_WRITING_TEST_PUBLICATION_STATUS_PENDING
	case experiment.TestPublicationConfirmed:
		out.Status = v1.WritingTestPublicationStatus_WRITING_TEST_PUBLICATION_STATUS_CONFIRMED
	case experiment.TestPublicationConflict:
		out.Status = v1.WritingTestPublicationStatus_WRITING_TEST_PUBLICATION_STATUS_CONFLICT
	default:
		return nil, experiment.ErrTestStateInvalid
	}
	return out, nil
}
func writingTestToProto(t experiment.WritingTest) (*v1.WritingTest, error) {
	out := &v1.WritingTest{Id: t.ID, Revision: t.Revision, Count: int32(t.Count), SourcePostSlug: t.SourcePostSlug, JobId: t.JobID, WinnerCandidateId: t.WinnerID, CreatedAt: formatTime(t.CreatedAt), UpdatedAt: formatTime(t.UpdatedAt), ContentExpiresAt: formatOptional(t.ContentExpiresAt), Fictional: t.Input.Fictional, ConfirmedCredits: int64(t.ConfirmedCredits), ReservedCredits: int64(t.ReservedCredits), Revealed: t.Status == experiment.TestCompleted || t.Status == experiment.TestCancelled}
	switch t.Factor {
	case experiment.FactorModel:
		out.Factor = v1.WritingTestFactor_WRITING_TEST_FACTOR_MODEL
	case experiment.FactorVoice:
		out.Factor = v1.WritingTestFactor_WRITING_TEST_FACTOR_VOICE
	case experiment.FactorTemplate:
		out.Factor = v1.WritingTestFactor_WRITING_TEST_FACTOR_TEMPLATE
	case experiment.FactorGuideline:
		out.Factor = v1.WritingTestFactor_WRITING_TEST_FACTOR_GUIDELINE
	default:
		return nil, experiment.ErrTestFactor
	}
	switch t.ModelStage {
	case "":
	case experiment.StageObserve:
		out.ModelStage = v1.WritingTestStage_WRITING_TEST_STAGE_OBSERVE
	case experiment.StageWrite:
		out.ModelStage = v1.WritingTestStage_WRITING_TEST_STAGE_WRITE
	default:
		return nil, experiment.ErrTestFactor
	}
	switch t.Status {
	case experiment.TestQueued:
		out.Status = v1.WritingTestStatus_WRITING_TEST_STATUS_QUEUED
	case experiment.TestRunning:
		out.Status = v1.WritingTestStatus_WRITING_TEST_STATUS_RUNNING
	case experiment.TestPartial:
		out.Status = v1.WritingTestStatus_WRITING_TEST_STATUS_PARTIAL
	case experiment.TestReview:
		out.Status = v1.WritingTestStatus_WRITING_TEST_STATUS_REVIEW
	case experiment.TestCompleted:
		out.Status = v1.WritingTestStatus_WRITING_TEST_STATUS_COMPLETED
	case experiment.TestCancelled:
		out.Status = v1.WritingTestStatus_WRITING_TEST_STATUS_CANCELLED
	case experiment.TestFailed:
		out.Status = v1.WritingTestStatus_WRITING_TEST_STATUS_FAILED
	default:
		return nil, experiment.ErrTestStateInvalid
	}
	switch t.Input.TargetLanguage {
	case "ko":
		out.TargetLanguage = v1.ContentLanguage_CONTENT_LANGUAGE_KOREAN
	case "en":
		out.TargetLanguage = v1.ContentLanguage_CONTENT_LANGUAGE_ENGLISH
	default:
		return nil, experiment.ErrTestMaterialInvalid
	}
	if t.Failure != nil {
		out.Failure = failureToProto(&experiment.Failure{Reason: t.Failure.Reason})
	}
	for i, c := range t.Candidates {
		p := c.Project(out.Revealed, fmt.Sprintf("%c", 'A'+i))
		wire := &v1.WritingTestCandidate{Id: p.ID, DisplayLabel: p.DisplayLabel}
		switch experiment.TestCandidateStatus(p.Status) {
		case experiment.TestCandidatePending:
			wire.Status = v1.WritingTestCandidateStatus_WRITING_TEST_CANDIDATE_STATUS_PENDING
		case experiment.TestCandidateRunning:
			wire.Status = v1.WritingTestCandidateStatus_WRITING_TEST_CANDIDATE_STATUS_RUNNING
		case experiment.TestCandidateSucceeded:
			wire.Status = v1.WritingTestCandidateStatus_WRITING_TEST_CANDIDATE_STATUS_SUCCEEDED
		case experiment.TestCandidateFailed:
			wire.Status = v1.WritingTestCandidateStatus_WRITING_TEST_CANDIDATE_STATUS_FAILED
		case experiment.TestCandidateCancelled:
			wire.Status = v1.WritingTestCandidateStatus_WRITING_TEST_CANDIDATE_STATUS_CANCELLED
		default:
			return nil, experiment.ErrTestStateInvalid
		}
		if len(p.Output) > 0 {
			value, err := experiment.DecodeTestOutput(p.Output)
			if err != nil {
				return nil, err
			}
			wire.Output, wire.Storyline, err = writingOutputToProto(value)
			if err != nil {
				return nil, err
			}
		}
		wire.Failure = failureToProto(p.Failure)
		if p.Identity != nil {
			ref, err := writingEntrantToProto(p.Identity.Ref)
			if err != nil {
				return nil, err
			}
			wire.Identity = &v1.WritingTestIdentity{Label: p.Identity.Label, Source: ref, Synthetic: p.Identity.Synthetic}
		}
		if p.Usage != nil {
			wire.Usage = &v1.WritingTestUsage{PromptTokens: p.Usage.PromptTokens, CompletionTokens: p.Usage.CompletionTokens, LatencyMs: p.Usage.LatencyMS}
		}
		out.Candidates = append(out.Candidates, wire)
	}
	for _, m := range t.Matches {
		out.Matches = append(out.Matches, &v1.WritingTestMatch{Id: m.ID, Round: int32(m.Round), Index: int32(m.Index), LeftCandidateId: m.LeftID, RightCandidateId: m.RightID, WinnerCandidateId: m.WinnerID})
	}
	for _, p := range t.Publications {
		wire, err := writingPublicationToProto(p)
		if err != nil {
			return nil, err
		}
		out.Publications = append(out.Publications, wire)
	}
	return out, nil
}
func writingOutputToProto(value experiment.TestOutput) (*v1.PostContent, *v1.Storyline, error) {
	out := &v1.PostContent{Title: value.Content.Title, Summary: value.Content.Summary, Tags: value.Content.Tags}
	for _, b := range value.Content.Blocks {
		kind, ok := v1.BlockType_value[b.Type]
		if !ok || kind == 0 {
			return nil, nil, experiment.ErrTestOutputIncompatible
		}
		layout := v1.GalleryLayout_GALLERY_LAYOUT_UNSPECIFIED
		switch b.Layout {
		case "":
		case "COLLAGE":
			layout = v1.GalleryLayout_GALLERY_LAYOUT_COLLAGE
		case "SLIDE":
			layout = v1.GalleryLayout_GALLERY_LAYOUT_SLIDE
		default:
			return nil, nil, experiment.ErrTestOutputIncompatible
		}

		out.Blocks = append(out.Blocks, &v1.Block{Type: v1.BlockType(kind), Content: b.Content, Level: b.Level, File: b.File, Alt: b.Alt, Caption: b.Caption, Items: b.Items, Files: b.Files, Layout: layout})
	}
	var story *v1.Storyline
	if value.Storyline != nil {
		story = &v1.Storyline{}
		for _, p := range value.Storyline.Paragraphs {
			story.Paragraphs = append(story.Paragraphs, &v1.StorylineParagraph{Text: p.Text, Files: p.Files})
		}
	}
	return out, story, nil
}
