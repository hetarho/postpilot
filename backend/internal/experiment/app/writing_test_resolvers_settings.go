package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/postpilot/backend/internal/authoring"
	"github.com/postpilot/backend/internal/experiment"
	"github.com/postpilot/backend/internal/generation"
	"github.com/postpilot/backend/internal/guideline"
	"github.com/postpilot/backend/internal/template"
	"github.com/postpilot/backend/internal/voice"
)

type preparedSettingSource struct {
	Version   int                       `json:"version"`
	Candidate authoring.FrozenCandidate `json:"candidate"`
}

func (r *WritingTestResolvers) ResolveWritingTestCandidate(ctx context.Context, user string, ref generation.WritingTestReference) (generation.WritingTestPreparedSetting, error) {
	frozen, err := r.dependencies.Candidates.FreezeCandidate(ctx, user, authoring.OwnedCandidateRef{SessionID: ref.AuthoringSessionID, CandidateID: ref.AuthoringCandidateID, Revision: ref.AuthoringRevision})
	if err != nil {
		return generation.WritingTestPreparedSetting{}, candidateReferenceError(err)
	}
	kind := factorForKind(frozen.Kind)
	if kind == "" || ref.SettingKind != "" && ref.SettingKind != kind || !frozen.Synthetic {
		return generation.WritingTestPreparedSetting{}, experiment.ErrTestEntrant
	}
	raw, err := json.Marshal(preparedSettingSource{Version: 1, Candidate: frozen})
	if err != nil {
		return generation.WritingTestPreparedSetting{}, err
	}
	a := frozen.Artifact
	return generation.WritingTestPreparedSetting{Kind: kind, Revision: fmt.Sprint(ref.AuthoringRevision), Name: a.Name, Description: a.Description, Body: a.Body, TitleArea: a.TitleArea, Payload: raw, Synthetic: true}, nil
}
func decodePreparedSetting(p *generation.WritingTestPreparedSetting, kind authoring.Kind) (authoring.FrozenCandidate, error) {
	if p == nil || len(p.Payload) == 0 || len(p.Payload) > 2<<20 {
		return authoring.FrozenCandidate{}, experiment.ErrTestEntrant
	}
	d := json.NewDecoder(bytes.NewReader(p.Payload))
	d.DisallowUnknownFields()
	var edge preparedSettingSource
	if d.Decode(&edge) != nil || d.Decode(new(any)) != io.EOF || edge.Version != 1 || edge.Candidate.Kind != kind || !edge.Candidate.Synthetic || factorForKind(kind) != p.Kind {
		return authoring.FrozenCandidate{}, experiment.ErrTestEntrant
	}
	return edge.Candidate, nil
}
func factorForKind(k authoring.Kind) string {
	switch k {
	case authoring.WritingVoice:
		return "voice"
	case authoring.PostTemplate:
		return "template"
	case authoring.PostGuideline:
		return "guideline"
	}
	return ""
}
func candidateReferenceError(err error) error {
	switch {
	case errors.Is(err, authoring.ErrNotFound):
		return experiment.ErrTestNotFound
	case errors.Is(err, authoring.ErrDraftInvalid), errors.Is(err, authoring.ErrInvalid):
		return experiment.ErrTestEntrant
	}
	return err
}
func (r *WritingTestResolvers) ResolveWritingTestProfile(ctx context.Context, user string, ref generation.WritingTestReference, p *generation.WritingTestPreparedSetting, target generation.Language, topic string) (generation.WritingTestVoice, error) {
	var frozen voice.FrozenWritingStyle
	var projection voice.PromptProfile
	var selected voice.Voice
	if p == nil {
		found, err := r.dependencies.Voices.FreezeTestProfile(ctx, user, ref.SettingID, ref.SettingRevision, voice.Language(target), topic)
		if err != nil {
			return generation.WritingTestVoice{}, err
		}
		frozen = voice.FrozenWritingStyle{Draft: voice.WritingStyleDraft{Name: found.Voice.Name, Description: found.Analysis.AI.Impression, Sample: found.Analysis.SyntheticSample}, Analysis: found.Analysis, Revision: found.Revision}
		projection, selected = found.Projection, found.Voice
	} else {
		source, err := decodePreparedSetting(p, authoring.WritingVoice)
		if err != nil {
			return generation.WritingTestVoice{}, err
		}
		draft := voice.WritingStyleDraft{Name: source.Artifact.Name, Description: source.Artifact.Description, Sample: source.Artifact.Body}
		// The source does not claim an original provider model. ReferenceAt belongs
		// to its immutable session and never substitutes today's time during retry.
		analysis, err := r.dependencies.Styles.PrepareWritingStyle(draft, "", source.ReferenceAt)
		if err != nil {
			return generation.WritingTestVoice{}, err
		}
		frozen = voice.FrozenWritingStyle{Draft: draft, Analysis: analysis, Revision: voice.AcceptedAnalysisRevision(analysis)}
		projection, err = voice.ProjectSyntheticTestStyle(frozen, voice.Language(target))
		if err != nil {
			return generation.WritingTestVoice{}, err
		}
		selected = voice.Voice{Name: draft.Name, Made: true}
	}
	payload, err := json.Marshal(frozen)
	if err != nil {
		return generation.WritingTestVoice{}, err
	}
	profile := generation.Profile{Text: projection.Text, Excerpts: slices.Clone(projection.Excerpts), Portable: projection.Portable}
	for _, source := range projection.AcceptedSources {
		profile.Sources = append(profile.Sources, generation.ProfileSource{SampleID: source.SampleID, ContentRevision: source.ContentRevision})
	}
	return generation.WritingTestVoice{Voice: generation.VoiceRef{ID: selected.ID, Name: selected.Name, Made: selected.Made, Deleted: selected.Deleted()}, Profile: profile, Revision: frozen.Revision, Synthetic: voice.NormalizedOrigin(frozen.Analysis.Origin) == voice.OriginSynthetic, Payload: payload}, nil
}
func readPreparedNumber(raw *string) (*int, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, nil
	}
	value, err := strconv.Atoi(strings.TrimSpace(*raw))
	if err != nil {
		return nil, experiment.ErrTestMaterialInvalid
	}
	return &value, nil
}
func (r *WritingTestResolvers) ResolveWritingTestTemplate(ctx context.Context, user string, ref generation.WritingTestReference, p *generation.WritingTestPreparedSetting) (generation.WritingTestTemplate, error) {
	var frozen template.TestSnapshot
	if p == nil {
		draft, numbers, version, err := r.dependencies.Templates.SeedWithNumbers(ctx, user, ref.SettingID)
		if err != nil {
			return generation.WritingTestTemplate{}, err
		}
		if ref.SettingRevision != "" && ref.SettingRevision != version {
			return generation.WritingTestTemplate{}, experiment.ErrTestRevisionConflict
		}
		frozen = template.TestSnapshot{TargetID: ref.SettingID, TargetVersion: version, Draft: draft, Numbers: numbers}
	} else {
		source, err := decodePreparedSetting(p, authoring.PostTemplate)
		if err != nil {
			return generation.WritingTestTemplate{}, err
		}
		length, err := readPreparedNumber(source.Artifact.TargetLength)
		if err != nil {
			return generation.WritingTestTemplate{}, err
		}
		tags, err := readPreparedNumber(source.Artifact.TagCount)
		if err != nil {
			return generation.WritingTestTemplate{}, err
		}
		frozen = template.TestSnapshot{TargetID: source.TargetID, TargetVersion: source.TargetVersion, Draft: template.Draft{Name: source.Artifact.Name, Description: source.Artifact.Description, Body: source.Artifact.Body, TitleArea: source.Artifact.TitleArea}, Numbers: template.Numbers{TargetLength: length, TagCount: tags}}
	}
	draft, err := r.dependencies.Templates.Validate(frozen.Draft)
	if err != nil {
		return generation.WritingTestTemplate{}, err
	}
	frozen.Draft = draft
	if err := r.dependencies.Templates.ValidateNumbers(frozen.Numbers); err != nil {
		return generation.WritingTestTemplate{}, err
	}
	asks, err := r.dependencies.Templates.InputFields(frozen.Draft)
	if err != nil {
		return generation.WritingTestTemplate{}, err
	}
	var fields []generation.WritingTestTemplateField
	for _, ask := range asks {
		flavor := "verbatim"
		if ask.Text != "" {
			flavor = "write"
		}
		fields = append(fields, generation.WritingTestTemplateField{Label: template.Decode(ask.Label), Flavor: flavor, Required: ask.Required})
	}
	raw, err := template.EncodeTestSnapshot(frozen)
	if err != nil {
		return generation.WritingTestTemplate{}, err
	}
	revision := frozen.TargetVersion
	if p != nil {
		revision = p.Revision
	}
	return generation.WritingTestTemplate{ID: ref.SettingID, Name: frozen.Draft.Name, Revision: revision, Fields: fields, Payload: raw}, nil
}
func (r *WritingTestResolvers) RenderWritingTestTemplate(_ context.Context, _ string, t generation.WritingTestTemplate, photos bool, answers []generation.TemplateAnswer) (generation.TemplateBrief, error) {
	frozen, err := template.DecodeTestSnapshot(t.Payload)
	if err != nil {
		return generation.TemplateBrief{}, experiment.ErrTestEntrant
	}
	input := make([]template.Answer, len(answers))
	for i, a := range answers {
		input[i] = template.Answer{Label: a.Label, Text: a.Text, Enabled: a.Enabled}
	}
	rendered, err := r.dependencies.Templates.RenderedForNewWrite(frozen.Draft, photos, input)
	if err != nil {
		var missing *template.RequiredAnswerError
		if errors.As(err, &missing) {
			return generation.TemplateBrief{}, &generation.RequiredTemplateAnswerError{Label: missing.Label}
		}
		return generation.TemplateBrief{}, err
	}
	brief := generation.TemplateBrief{Name: rendered.Name, Body: rendered.Body, TitleArea: rendered.TitleArea}
	for _, fact := range rendered.Facts {
		brief.Facts = append(brief.Facts, generation.TemplateFact{Label: fact.Label, Value: fact.Value})
	}
	return brief, nil
}
func (r *WritingTestResolvers) FreezeWritingTestGuidelines(ctx context.Context, user, templateID, field string, target generation.Language, memory bool) (generation.WritingTestRules, error) {
	frozen, err := r.dependencies.Guidelines.TestRules(ctx, user, templateID, field, guideline.Language(target), memory)
	if err != nil {
		return generation.WritingTestRules{}, err
	}
	result := generation.WritingTestRules{Defaults: slices.Clone(frozen.Defaults)}
	for _, rule := range frozen.Owner {
		result.Owner = append(result.Owner, generation.WritingTestRule{ID: rule.ID, Name: rule.Title, Text: rule.Text, Revision: guideline.AuthoringVersion(rule)})
	}
	return result, nil
}
func (r *WritingTestResolvers) ResolveWritingTestGuideline(ctx context.Context, user string, ref generation.WritingTestReference, p *generation.WritingTestPreparedSetting) (generation.WritingTestRule, error) {
	var frozen guideline.TestSnapshot
	var revision string
	if p == nil {
		draft, scope, version, err := r.dependencies.GuidelineAuthoring.SeedWithScope(ctx, user, guideline.KindPost, ref.SettingID)
		if err != nil {
			return generation.WritingTestRule{}, err
		}
		if ref.SettingRevision != "" && ref.SettingRevision != version {
			return generation.WritingTestRule{}, experiment.ErrTestRevisionConflict
		}
		frozen = guideline.TestSnapshot{TargetID: ref.SettingID, TargetVersion: version, Kind: guideline.KindPost, Draft: draft, Scope: scope}
		revision = version
	} else {
		source, err := decodePreparedSetting(p, authoring.PostGuideline)
		if err != nil {
			return generation.WritingTestRule{}, err
		}
		scope := guideline.ScopePatch{}
		if source.Artifact.Scope != nil {
			scope.Scope = guideline.Scope(*source.Artifact.Scope)
		}
		scope.TemplateIDs = slices.Clone(source.Artifact.TemplateIDs)
		scope.Fields = slices.Clone(source.Artifact.Fields)
		frozen = guideline.TestSnapshot{TargetID: source.TargetID, TargetVersion: source.TargetVersion, Kind: guideline.KindPost, Draft: guideline.AuthoringDraft{Name: source.Artifact.Name, Body: source.Artifact.Body}, Scope: scope}
		revision = p.Revision
	}
	draft, err := r.dependencies.GuidelineAuthoring.Validate(frozen.Draft)
	if err != nil {
		return generation.WritingTestRule{}, err
	}
	frozen.Draft = draft
	raw, err := guideline.EncodeTestSnapshot(frozen)
	if err != nil {
		return generation.WritingTestRule{}, err
	}
	return generation.WritingTestRule{ID: ref.SettingID, Name: draft.Name, Text: draft.Body, Revision: revision, Payload: raw}, nil
}
