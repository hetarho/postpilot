package generation

import (
	"context"
	"reflect"
	"strings"

	"github.com/postpilot/backend/internal/llm"
)

func validWritingTestCount(count int) bool {
	return count == 2 || count == 4 || count == 8 || count == 16
}
func validWritingTestFactor(factor, stage string) bool {
	if factor == "model" {
		return stage == "observe" || stage == "write"
	}
	return stage == "" && (factor == "voice" || factor == "template" || factor == "guideline")
}

func validWritingTestReference(ref WritingTestReference, factor string) bool {
	switch ref.SourceKind {
	case "model":
		return factor == "model" && ref.Model.ProviderID != "" && ref.Model.ModelID != "" && ref.SettingID == "" && ref.SettingRevision == "" && ref.SettingKind == "" && ref.AuthoringSessionID == "" && ref.AuthoringCandidateID == "" && ref.AuthoringRevision == 0
	case "setting":
		return factor != "model" && ref.SettingKind == factor && ref.SettingID != "" && ref.SettingRevision != "" && ref.Model == (llm.ModelRef{}) && ref.AuthoringSessionID == "" && ref.AuthoringCandidateID == "" && ref.AuthoringRevision == 0
	case "authoring_candidate":
		return factor != "model" && ref.AuthoringSessionID != "" && ref.AuthoringCandidateID != "" && ref.AuthoringRevision != 0 && ref.Model == (llm.ModelRef{}) && ref.SettingID == "" && ref.SettingRevision == "" && (ref.SettingKind == "" || ref.SettingKind == factor)
	}
	return false
}

func (f *WritingTestFactory) FreezeWritingTest(ctx context.Context, request WritingTestSnapshotRequest) (WritingTestSnapshot, error) {
	if !validWritingTestCount(request.Count) || len(request.Variants) != request.Count {
		return WritingTestSnapshot{}, ErrWritingTestCount
	}
	if !validWritingTestFactor(request.Factor, request.ModelStage) {
		return WritingTestSnapshot{}, ErrWritingTestFactor
	}
	if request.UserID == "" || request.InputRevision < 0 || request.ContentRevision < 0 {
		return WritingTestSnapshot{}, ErrWritingTestMaterial
	}
	var material WritingTestMaterialRequest
	if err := decodeWritingTestJSON(request.CommonMaterial, &material); err != nil {
		return WritingTestSnapshot{}, err
	}
	if !material.TargetLanguage.Valid() {
		return WritingTestSnapshot{}, ErrLanguageRequired
	}
	if material.TagCount <= 0 || material.TargetLength != nil && *material.TargetLength <= 0 {
		return WritingTestSnapshot{}, ErrWritingTestMaterial
	}
	refs := make([]WritingTestReference, request.Count)
	for index, raw := range request.Variants {
		if err := decodeWritingTestJSON(raw, &refs[index]); err != nil || !validWritingTestReference(refs[index], request.Factor) {
			return WritingTestSnapshot{}, ErrWritingTestReference
		}
	}
	source, err := f.deps.Sources.ResolveWritingTestSource(ctx, request.UserID, request.SourcePostSlug, request.InputRevision, request.ContentRevision, material)
	if err != nil {
		return WritingTestSnapshot{}, err
	}
	if source.Post.UserID != request.UserID || source.Post.Slug != request.SourcePostSlug {
		return WritingTestSnapshot{}, ErrWritingTestMaterial
	}
	if request.SourcePostSlug != "" && (source.InputRevision != request.InputRevision || source.ContentRevision != request.ContentRevision || source.Revision == "" || source.AssignmentsHash == "") {
		return WritingTestSnapshot{}, ErrWritingTestRevision
	}
	if err := validateWritingTestAttachments(source); err != nil {
		return WritingTestSnapshot{}, err
	}
	post := fromSnapshotPost(toSnapshotPost(source.Post))
	post.TargetLanguage, post.TargetLength, post.TagCount = material.TargetLanguage, copyOptionalInt(material.TargetLength), material.TagCount
	post.TemplateID, post.TemplateAnswers = material.TemplateID, append([]TemplateAnswer(nil), material.TemplateAnswers...)
	post.UseMemory, post.QualityRuleIDs = material.UseMemory, copyTexts(material.QualityRules)
	post.Content, post.ContentLanguage, post.Storyline, post.FollowStoryline = nil, nil, nil, nil
	post.Template, post.Guidelines, post.DefaultGuidelines, post.Memories, post.QualityRules = nil, nil, nil, nil, nil
	post.StockGuidelines = nil
	post.WriteNativeEffort, post.Voice = false, VoiceRef{}
	if material.Fictional {
		post.Memo = "가상 시나리오(사용자의 실제 경험이 아님)\n" + post.Memo
	}
	if strings.TrimSpace(source.Post.Memo) == "" && len(post.Images) == 0 && !writingTestHasFacts(material.TemplateAnswers) {
		return WritingTestSnapshot{}, ErrWritingTestMaterial
	}
	common := writingTestCommon{Version: writingTestSnapshotVersion, Factor: request.Factor, ModelStage: request.ModelStage, Post: post, Profile: Profile{NoVoice: true}, Fictional: material.Fictional, BatchSize: f.service.batchSize, Reasoning: f.service.reasoning, PromptVersion: writingTestPromptVersion, SchemaVersion: writingTestSchemaVersion(), SourceRevision: source.Revision, AssignmentsHash: source.AssignmentsHash, InputRevision: source.InputRevision, ContentRevision: source.ContentRevision, Attachments: source.Attachments}
	if common.BatchSize <= 0 {
		return WritingTestSnapshot{}, ErrWritingTestMaterial
	}
	common.QualityRuleIDs = copyTexts(material.QualityRules)
	if request.Factor != "voice" && material.VoiceID != "" {
		voice, err := f.deps.Profiles.ResolveWritingTestProfile(ctx, request.UserID, WritingTestReference{SourceKind: "setting", SettingKind: "voice", SettingID: material.VoiceID}, nil, post.TargetLanguage, post.Title+" "+post.Memo)
		if err != nil {
			return WritingTestSnapshot{}, err
		}
		if err := validateWritingTestVoice(voice, material.VoiceID); err != nil {
			return WritingTestSnapshot{}, err
		}
		common.Profile, post.Voice, common.VoiceRevision = voice.Profile, voice.Voice, voice.Revision
	}
	if request.Factor != "template" && material.TemplateID != "" {
		template, err := f.deps.Templates.ResolveWritingTestTemplate(ctx, request.UserID, WritingTestReference{SourceKind: "setting", SettingKind: "template", SettingID: material.TemplateID}, nil)
		if err != nil {
			return WritingTestSnapshot{}, err
		}
		if template.ID != material.TemplateID || template.Revision == "" {
			return WritingTestSnapshot{}, ErrWritingTestRevision
		}
		brief, err := f.deps.Templates.RenderWritingTestTemplate(ctx, request.UserID, template, len(photosOf(post.Images)) > 0, material.TemplateAnswers)
		if err != nil {
			return WritingTestSnapshot{}, err
		}
		post.Template, common.TemplateRevision, common.RequiredFields = &brief, template.Revision, template.Fields
	}
	// All common retrieval happens before variants, never after selecting their scope or defaults.
	post.Memories, err = f.service.freezeMemories(ctx, post)
	if err != nil {
		return WritingTestSnapshot{}, err
	}
	common.Rules, err = f.deps.Guidelines.FreezeWritingTestGuidelines(ctx, request.UserID, material.TemplateID, source.Post.Field, post.TargetLanguage, len(post.Memories) > 0)
	if err != nil {
		return WritingTestSnapshot{}, err
	}
	post.DefaultGuidelines = copyTexts(common.Rules.Defaults)
	post.StockGuidelines = cloneStockGuidelines(common.Rules.Stock)
	for _, rule := range common.Rules.Owner {
		if rule.ID == "" || rule.Revision == "" || strings.TrimSpace(rule.Text) == "" {
			return WritingTestSnapshot{}, ErrWritingTestMaterial
		}
		post.Guidelines = append(post.Guidelines, rule.Text)
	}
	post.QualityRules, err = f.service.freezeQualityRules(ctx, post)
	if err != nil {
		return WritingTestSnapshot{}, err
	}
	var fixedWriter WritingTestModel
	if request.Factor != "model" || request.ModelStage != "write" {
		fixedWriter, err = f.prepareModel(ctx, request.UserID, "write", material.WriteModel, post.Images)
		if err != nil {
			return WritingTestSnapshot{}, err
		}
		post.WriteNativeEffort, common.WritePromptTokens = fixedWriter.Info.ReasoningNativeEffort, fixedWriter.PromptTokens
	}
	var sharedObserver WritingTestModel
	if request.Factor == "model" && request.ModelStage == "observe" {
		if len(post.Images) == 0 {
			return WritingTestSnapshot{}, ErrWritingTestMaterial
		}
		common.Prepared = true
	} else {
		files, seed := freezeObserveSelection(post.Images, source.Post.Observations, material.ObserveFiles)
		common.ObserveFiles, common.Observations = &files, seed
		calls := f.service.observeCalls(observeTargets(post.Images, &files))
		common.Prepared = calls == 0
		if calls > 0 {
			sharedObserver, err = f.prepareModel(ctx, request.UserID, "observe", material.ObserveModel, post.Images)
			if err != nil {
				return WritingTestSnapshot{}, err
			}
			common.ObserveModel, common.ObservePromptTokens, common.ObserveCompletionTokens = material.ObserveModel, sharedObserver.PromptTokens, f.service.budget.Observation()
			common.ObserveStructuredOutput = sharedObserver.Info.StructuredOutput
		}
	}
	post.Observations = nil
	common.Post = post
	variants := make([]writingTestVariant, len(refs))
	seen := map[string]bool{}
	for index, ref := range refs {
		variant := writingTestVariant{Reference: ref, WriteModel: material.WriteModel, ObserveModel: common.ObserveModel, ObservePromptTokens: common.ObservePromptTokens, ObserveCompletionTokens: common.ObserveCompletionTokens, ObserveStructuredOutput: common.ObserveStructuredOutput, WritePromptTokens: fixedWriter.PromptTokens, WriteStructuredOutput: fixedWriter.Info.StructuredOutput}
		variant.Snapshot = writeSnapshot{Prepared: common.Prepared, TargetLanguage: post.TargetLanguage, ObserveModel: testModelString(common.ObserveModel), ObserveFiles: common.ObserveFiles, Post: post, Profile: common.Profile, Observations: common.Observations, SnapshotOnly: true}
		var prepared *WritingTestPreparedSetting
		if ref.SourceKind == "authoring_candidate" {
			candidate, err := f.deps.Candidates.ResolveWritingTestCandidate(ctx, request.UserID, ref)
			if err != nil {
				return WritingTestSnapshot{}, err
			}
			if candidate.Kind != request.Factor || candidate.Revision == "" || !candidate.Synthetic {
				return WritingTestSnapshot{}, ErrWritingTestReference
			}
			prepared, variant.Synthetic = &candidate, true
		}
		switch request.Factor {
		case "model":
			info, err := f.prepareModel(ctx, request.UserID, request.ModelStage, ref.Model, post.Images)
			if err != nil {
				return WritingTestSnapshot{}, err
			}
			variant.Revision = ref.Model.String()
			variant.SemanticKey = writingTestSemanticKey(struct {
				Stage string
				Ref   llm.ModelRef
			}{request.ModelStage, ref.Model})
			if request.ModelStage == "write" {
				variant.WriteModel, variant.WritePromptTokens, variant.WriteStructuredOutput = ref.Model, info.PromptTokens, info.Info.StructuredOutput
				variant.Snapshot.Post.WriteNativeEffort = info.Info.ReasoningNativeEffort
			} else {
				variant.ObserveModel, variant.ObservePromptTokens, variant.ObserveCompletionTokens = ref.Model, info.PromptTokens, f.service.budget.Observation()
				variant.ObserveStructuredOutput = info.Info.StructuredOutput
				variant.Snapshot.ObserveModel, variant.Snapshot.ObserveFiles, variant.Snapshot.Observations, variant.Snapshot.Prepared = ref.Model.String(), nil, nil, false
			}
		case "voice":
			voice, err := f.deps.Profiles.ResolveWritingTestProfile(ctx, request.UserID, ref, prepared, post.TargetLanguage, post.Title+" "+post.Memo)
			if err != nil {
				return WritingTestSnapshot{}, err
			}
			if err := validateWritingTestVoice(voice, ref.SettingID); err != nil {
				return WritingTestSnapshot{}, err
			}
			if ref.SourceKind == "setting" && voice.Revision != ref.SettingRevision {
				return WritingTestSnapshot{}, ErrWritingTestRevision
			}
			if prepared != nil && (!voice.Synthetic || voice.Voice.ID != "" || len(voice.Profile.Sources) > 0) {
				return WritingTestSnapshot{}, ErrWritingTestReference
			}
			variant.Snapshot.Profile, variant.Snapshot.Post.Voice, variant.Revision, variant.Payload, variant.Synthetic = voice.Profile, voice.Voice, voice.Revision, voice.Payload, voice.Synthetic
			variant.SemanticKey = writingTestSemanticKey(struct {
				Text     string
				Excerpts []string
				Portable bool
			}{voice.Profile.Text, voice.Profile.Excerpts, voice.Profile.Portable})
		case "template":
			template, err := f.deps.Templates.ResolveWritingTestTemplate(ctx, request.UserID, ref, prepared)
			if err != nil {
				return WritingTestSnapshot{}, err
			}
			if template.Revision == "" || ref.SourceKind == "setting" && (template.Revision != ref.SettingRevision || template.ID != ref.SettingID) {
				return WritingTestSnapshot{}, ErrWritingTestRevision
			}
			brief, err := f.deps.Templates.RenderWritingTestTemplate(ctx, request.UserID, template, len(photosOf(post.Images)) > 0, material.TemplateAnswers)
			if err != nil {
				return WritingTestSnapshot{}, err
			}
			variant.Snapshot.Post.Template, variant.Revision, variant.Payload = &brief, template.Revision, template.Payload
			variant.SemanticKey = writingTestSemanticKey(struct {
				Body, Title string
				Facts       []TemplateFact
			}{brief.Body, brief.TitleArea, brief.Facts})
			common.RequiredFields = mergeWritingTestFields(common.RequiredFields, template.Fields)
		case "guideline":
			rule, err := f.deps.Guidelines.ResolveWritingTestGuideline(ctx, request.UserID, ref, prepared)
			if err != nil {
				return WritingTestSnapshot{}, err
			}
			if rule.Revision == "" || strings.TrimSpace(rule.Text) == "" || ref.SourceKind == "setting" && (rule.ID != ref.SettingID || rule.Revision != ref.SettingRevision) {
				return WritingTestSnapshot{}, ErrWritingTestRevision
			}
			rules := copyTexts(post.Guidelines)
			slot := len(rules)
			if material.GuidelineSlotID != "" {
				slot = -1
				for index, current := range common.Rules.Owner {
					if current.ID == material.GuidelineSlotID {
						slot = index
						break
					}
				}
				if slot < 0 {
					return WritingTestSnapshot{}, ErrWritingTestReference
				}
			}
			if slot == len(rules) {
				rules = append(rules, rule.Text)
			} else {
				rules[slot] = rule.Text
			}
			variant.Snapshot.Post.Guidelines, variant.Revision, variant.Payload = rules, rule.Revision, rule.Payload
			variant.SemanticKey = writingTestSemanticKey(strings.TrimSpace(rule.Text))
		}
		if seen[variant.SemanticKey] {
			return WritingTestSnapshot{}, ErrWritingTestDuplicate
		}
		seen[variant.SemanticKey] = true
		variant.WriteCompletionTokens = f.service.budget.Write(post.TargetLength, variant.Snapshot.Post.WriteNativeEffort)
		if variant.WriteCompletionTokens <= 0 || variant.WritePromptTokens <= 0 {
			return WritingTestSnapshot{}, ErrWritingTestMaterial
		}
		variants[index] = variant
	}
	if err := validateWritingTestFacts(common.RequiredFields, material.TemplateAnswers); err != nil {
		return WritingTestSnapshot{}, err
	}
	if strings.TrimSpace(source.Post.Memo) == "" && len(post.Images) == 0 {
		for _, variant := range variants {
			if variant.Snapshot.Post.Template == nil || !writingTestTemplateHasFacts(*variant.Snapshot.Post.Template) {
				return WritingTestSnapshot{}, ErrWritingTestMaterial
			}
		}
	}
	commonRaw, err := encodeWritingTestCommon(common)
	if err != nil {
		return WritingTestSnapshot{}, err
	}
	out := WritingTestSnapshot{Common: commonRaw, Variants: make([][]byte, len(variants)), PromptVersion: common.PromptVersion, AssignmentsHash: common.AssignmentsHash}
	for index, variant := range variants {
		out.Variants[index], err = encodeWritingTestVariant(variant)
		if err != nil {
			return WritingTestSnapshot{}, err
		}
	}
	out.Hash = writingTestHash(out.Common, out.Variants)
	calls, err := f.PlanWritingTest(out)
	if err != nil {
		return WritingTestSnapshot{}, err
	}
	for _, call := range calls {
		if call.Stage == "observe" {
			out.ObserveCalls += call.Count
		}
	}
	return out, nil
}

func (f *WritingTestFactory) PlanWritingTest(snapshot WritingTestSnapshot) ([]WritingTestCall, error) {
	common, variants, err := decodeWritingTestSnapshot(snapshot)
	if err != nil {
		return nil, err
	}
	var calls []WritingTestCall
	if !observerWritingTest(common) && !common.Prepared {
		targets, _ := frozenObserveSelection(common.Post.Images, common.ObserveFiles, common.Observations)
		calls = appendWritingTestCall(calls, WritingTestCall{Ref: common.ObserveModel, Stage: "observe", Count: len(writingTestBatches(targets, common.BatchSize)), PromptTokens: common.ObservePromptTokens, CompletionTokens: common.ObserveCompletionTokens})
	}
	for _, variant := range variants {
		if observerWritingTest(common) {
			calls = appendWritingTestCall(calls, WritingTestCall{Ref: variant.ObserveModel, Stage: "observe", Count: len(writingTestBatches(variant.Snapshot.Post.Images, common.BatchSize)), PromptTokens: variant.ObservePromptTokens, CompletionTokens: variant.ObserveCompletionTokens})
		}
		calls = appendWritingTestCall(calls, WritingTestCall{Ref: variant.WriteModel, Stage: "write", Count: 1, PromptTokens: variant.WritePromptTokens, CompletionTokens: variant.WriteCompletionTokens})
	}
	return calls, nil
}

func (f *WritingTestFactory) prepareModel(ctx context.Context, user, stage string, ref llm.ModelRef, images []Image) (WritingTestModel, error) {
	if ref.ProviderID == "" || ref.ModelID == "" {
		return WritingTestModel{}, ErrWritingTestReference
	}
	value, err := f.deps.Models.PrepareWritingTestModel(ctx, user, stage, ref)
	if err != nil {
		return value, err
	}
	if value.Info.Ref != ref || value.Info.Disabled || !value.Info.ServesStage(stage) || value.PromptTokens <= 0 {
		return WritingTestModel{}, ErrWritingTestReference
	}
	if stage == "observe" {
		if !value.Info.Vision {
			return WritingTestModel{}, ErrObserveModelRequired
		}
		if len(videosOf(images)) > 0 && (!value.Info.VideoInput || !value.Info.VideoDelivery.SignedVideoURL) {
			return WritingTestModel{}, &VideoUnsupportedError{Model: ref.String()}
		}
		if f.service.budget.Observation() <= 0 {
			return WritingTestModel{}, ErrWritingTestMaterial
		}
	}
	return value, nil
}

func validateWritingTestAttachments(source WritingTestSource) error {
	if len(source.Attachments) != len(source.Post.Images) {
		return ErrWritingTestMaterial
	}
	ids, names := map[string]bool{}, map[string]bool{}
	for index, attachment := range source.Attachments {
		image := attachment.Image
		if attachment.ID == "" || attachment.Fingerprint == "" || image.Filename == "" || image.Key == "" || strings.Contains(image.Key, "://") || ids[attachment.ID] || names[image.Filename] || !reflect.DeepEqual(image, source.Post.Images[index]) {
			return ErrWritingTestMaterial
		}
		if image.Kind != "" && image.Kind != AttachmentPhoto && image.Kind != AttachmentVideo {
			return ErrWritingTestMaterial
		}
		ids[attachment.ID], names[image.Filename] = true, true
	}
	return nil
}
func validateWritingTestVoice(voice WritingTestVoice, expected string) error {
	if voice.Revision == "" || voice.Profile.NoVoice || strings.TrimSpace(voice.Profile.Text) == "" || voice.Voice.Deleted || !voice.Voice.Made || expected != "" && voice.Voice.ID != expected {
		return ErrVoiceNotMade
	}
	return nil
}
func writingTestHasFacts(answers []TemplateAnswer) bool {
	for _, answer := range answers {
		if answer.Enabled && strings.TrimSpace(answer.Text) != "" {
			return true
		}
	}
	return false
}

func writingTestTemplateHasFacts(brief TemplateBrief) bool {
	for _, fact := range brief.Facts {
		if strings.TrimSpace(fact.Value) != "" {
			return true
		}
	}
	return false
}
func mergeWritingTestFields(fields, next []WritingTestTemplateField) []WritingTestTemplateField {
	for _, field := range next {
		found := false
		for index, current := range fields {
			if current.Label == field.Label && current.Flavor == field.Flavor {
				fields[index].Required = fields[index].Required || field.Required
				found = true
				break
			}
		}
		if !found {
			fields = append(fields, field)
		}
	}
	return fields
}
func validateWritingTestFacts(fields []WritingTestTemplateField, answers []TemplateAnswer) error {
	values := map[string]TemplateAnswer{}
	for _, answer := range answers {
		if answer.Label == "" {
			return ErrWritingTestMaterial
		}
		if _, exists := values[answer.Label]; exists {
			return ErrWritingTestMaterial
		}
		values[answer.Label] = answer
	}
	for _, field := range fields {
		if field.Label == "" || field.Flavor != "write" && field.Flavor != "verbatim" {
			return ErrWritingTestMaterial
		}
		if field.Required {
			value, exists := values[field.Label]
			if !exists || !value.Enabled || strings.TrimSpace(value.Text) == "" {
				return &RequiredTemplateAnswerError{Label: field.Label}
			}
		}
	}
	return nil
}
