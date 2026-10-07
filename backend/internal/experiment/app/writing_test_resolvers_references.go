package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/postpilot/backend/internal/experiment"
	"github.com/postpilot/backend/internal/generation"
	"github.com/postpilot/backend/internal/llm"
)

func (r *WritingTestResolvers) ResolveTestVariant(ctx context.Context, user string, factor experiment.TestFactor, ref experiment.TestEntrantRef) (experiment.FrozenTestVariant, error) {
	out := experiment.FrozenTestVariant{Reference: ref}
	if user == "" || !factor.Valid() {
		return out, experiment.ErrTestEntrant
	}
	if factor == experiment.FactorModel {
		if ref.SourceKind != "model" {
			return out, experiment.ErrTestEntrant
		}
		info, ok := r.dependencies.Models.RegisteredTestModel(llm.ModelRef{ProviderID: ref.Model.ProviderID, ModelID: ref.Model.ModelID})
		if !ok || info.Disabled || !info.ServesStage("observe") && !info.ServesStage("write") {
			return out, experiment.ErrTestEntrant
		}
		out.Label = info.Label
		out.Revision = ref.Model.String()
		out.Content, _ = json.Marshal(ref.Model)
		out.SemanticKey = "model:" + ref.Model.String()
		return out, nil
	}
	generationRef := toGenerationTestReference(ref)
	var prepared *generation.WritingTestPreparedSetting
	switch ref.SourceKind {
	case "setting":
		if ref.SettingKind != string(factor) || ref.SettingID == "" || ref.SettingRevision == "" {
			return out, experiment.ErrTestEntrant
		}
	case "authoring_candidate":
		value, err := r.ResolveWritingTestCandidate(ctx, user, generationRef)
		if err != nil {
			return out, err
		}
		if value.Kind != string(factor) {
			return out, experiment.ErrTestEntrant
		}
		prepared = &value
	default:
		return out, experiment.ErrTestEntrant
	}
	switch factor {
	case experiment.FactorVoice:
		voice, err := r.ResolveWritingTestProfile(ctx, user, generationRef, prepared, generation.LanguageKorean, "")
		if err != nil {
			return out, err
		}
		out.Content, out.Revision, out.Label, out.Synthetic = voice.Payload, voice.Revision, voice.Voice.Name, voice.Synthetic
	case experiment.FactorTemplate:
		template, err := r.ResolveWritingTestTemplate(ctx, user, generationRef, prepared)
		if err != nil {
			return out, err
		}
		out.Content, out.Revision, out.Label, out.Synthetic = template.Payload, template.Revision, template.Name, prepared != nil
	case experiment.FactorGuideline:
		rule, err := r.ResolveWritingTestGuideline(ctx, user, generationRef, prepared)
		if err != nil {
			return out, err
		}
		out.Content, out.Revision, out.Label, out.Synthetic = rule.Payload, rule.Revision, rule.Name, prepared != nil
	default:
		return out, experiment.ErrTestFactor
	}
	sum := sha256.Sum256(out.Content)
	out.SemanticKey = fmt.Sprintf("%s:%s", factor, hex.EncodeToString(sum[:]))
	return out, nil
}
