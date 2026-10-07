package generation

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/postpilot/backend/internal/llm"
)

// Versions identify the owning composer, independently of the private full-test
// snapshot protocol. Inspection does not change that protocol or its schema bytes.
const postPromptCompositionVersion = "post-stage-contracts-v2"

func schemaInspection(name string, schema []byte) llm.OutputContractInspection {
	version := fmt.Sprintf("sha256:%x", sha256.Sum256(schema))
	return llm.OutputContractInspection{Name: name, Version: version, Schema: string(schema)}
}

func generationDescriptor(mode string) llm.RequestComposition {
	out := llm.RequestComposition{
		Stage: "post-writing", Mode: mode, PromptVersion: postPromptCompositionVersion,
		Composer:    "generation.ComposeWriteRequest/BuildWritePromptForLanguage",
		Parser:      "generation.ParseWriteAnswer/ValidateBlocks/FilterAttachments",
		Consumer:    "generated post content, storyline and machine baseline; private full-test candidate output",
		Activation:  "admitted ordinary generation or full writing test; immutable accepted profile and frozen settings; explicit target language",
		SourceFiles: []string{"internal/generation/request_composition.go", "internal/generation/prompts.go", "internal/generation/material_composition.go", "internal/generation/block_field_contract.go", "internal/generation/write.go", "internal/generation/parse.go", "internal/generation/schemas.go"},
		Output:      schemaInspection("WriteAnswer", WriteAnswerSchema()),
	}
	switch mode {
	case "frozen-storyline":
		out.Output = schemaInspection("WriteAlongStorylineAnswer", WriteAlongStorylineAnswerSchema())
		out.Activation = "admitted ordinary generation with stored nonempty storyline; returns content only and preserves stored storyline"
	case "photo-observation", "full-test-photo-observation":
		out.Stage, out.Composer = "post-observation", "generation.composePhotoObservationRequest/Service.observeCandidate"
		out.Parser, out.Consumer = "generation.parseObservations/matchObservations/mergeObservations", "ordered attachment observations; ordinary post or private full-test checkpoint"
		out.Activation = "selected photos; one call per configured batch; full-test shared or entrant-specific observation plan"
		out.SourceFiles = []string{"internal/generation/request_composition.go", "internal/generation/observe.go", "internal/generation/prompts.go", "internal/generation/schemas.go"}
		out.Output = schemaInspection("Observations", ObservationsSchema())
	case "video-observation", "full-test-video-observation":
		out.Stage, out.Composer = "post-observation", "generation.composeVideoObservationRequest/Service.observeVideo"
		out.Parser, out.Consumer = "generation.parseObservations/matchObservations/mergeObservations", "source-time video observations; ordinary post or private full-test checkpoint"
		out.Activation = "one selected video per call; signed media link minted only for dispatch and omitted from inspection"
		out.SourceFiles = []string{"internal/generation/request_composition.go", "internal/generation/observe.go", "internal/generation/prompts.go", "internal/generation/schemas.go"}
		out.Output = schemaInspection("VideoObservations", VideoObservationsSchema())
	case "storyline-create", "storyline-rewrite":
		out.Stage, out.Composer = "post-storyline", "generation.ComposeStorylineRequest/BuildStorylinePromptForLanguage/Service.storylineCall"
		out.Parser, out.Consumer = "generation.ParseStorylineAnswer", "stored storyline paragraphs and attachment placement; does not write canonical prose"
		out.Activation = "admitted storyline create after optional observation; rewrite uses frozen current plan and explicit edit request without observation"
		out.SourceFiles = []string{"internal/generation/request_composition.go", "internal/generation/storyline_prompts.go", "internal/generation/storyline.go", "internal/generation/material_composition.go", "internal/generation/schemas.go", "internal/generation/parse.go"}
		out.Output = schemaInspection("StorylineAnswer", StorylineAnswerSchema())
	case "revision":
		out.Stage, out.Composer = "post-revision", "generation.composeRevisionRequest/buildRevisePrompt/Service.Revise"
		out.Parser, out.Consumer = "generation.ParseRevisionContent/ValidateBlocks/FilterAttachments", "complete replacement PostContent; unchanged tags and unrelated scope preserved"
		out.Activation = "admitted revision with current PostContent, explicit owner edit, frozen accepted profile and settings; no observation or memory retrieval"
		out.SourceFiles = []string{"internal/generation/request_composition.go", "internal/generation/revise.go", "internal/generation/revise_handler.go", "internal/generation/material_composition.go", "internal/generation/block_field_contract.go", "internal/generation/schemas.go", "internal/generation/parse.go"}
		out.Output = schemaInspection("PostContent", PostContentSchema())
	}
	out.SchemaVersion = out.Output.Version
	return out
}

// RequestCompositions is the discoverable current code inventory. It performs no
// read, admission, provider call or account access. Each entry also seeds its real
// assembler, so this list is not a second prompt authority.
func RequestCompositions() []llm.RequestComposition {
	fixtures := RequestCompositionFixtures()
	out := make([]llm.RequestComposition, 0, len(fixtures))
	for _, request := range fixtures {
		out = append(out, *request.Composition)
	}
	return out
}

// RequestCompositionFixtures uses only synthetic developer material and the
// production assemblers. It grants no account access and does not call a model.
func RequestCompositionFixtures() []llm.Request {
	input := WritePromptInput{Language: LanguageKorean, Profile: Profile{Text: "Synthetic accepted style projection", Excerpts: []string{"Synthetic fictional style example"}}, Title: "Synthetic post", Memo: "Explicit synthetic owner material", TagCount: 3, StockGuidelines: []StockGuideline{{Key: "fixture-facts", Text: "Use supplied synthetic evidence", Applicability: []StockRuleApplicability{{Stage: "write", Outputs: []string{"prose"}}, {Stage: "revise", Outputs: []string{"prose"}}, {Stage: "storyline", Outputs: []string{"plan"}}}}, {Key: "fixture-tags", Text: "Synthetic final tag rule", Applicability: []StockRuleApplicability{{Stage: "write", Outputs: []string{"tags"}}}}}, Guidelines: []string{"Synthetic owner instruction"}, Memories: []string{"Synthetic selected memory fact"}, QualityRules: []string{"Synthetic publication rule"}, Photos: []string{"fixture.jpg"}, Observations: []Observation{{File: "fixture.jpg", Scene: "Synthetic visual interpretation"}}, Template: &TemplateBrief{Name: "Synthetic form", BodyParts: []TemplateMaterialPart{{Kind: "write", Text: "Introduce the supplied subject"}, {Kind: "answer_write", Text: "Describe this field", Label: "Subject", Parts: []TemplateMaterialPart{{Kind: "fact", Text: "Synthetic field fact"}}}}, TitleParts: []TemplateMaterialPart{}}}
	direct := ComposeWriteRequest(input)
	input.FollowStoryline = []StorylineParagraph{{Text: "Proposed arrangement", Files: []string{"fixture.jpg"}}}
	following := ComposeWriteRequest(input)
	input.FollowStoryline = nil
	fullTest := ComposeWriteRequest(input)
	fullTest.Composition.Mode = "full-test-direct"
	photoParts := []llm.Part{llm.TextPart("file: fixture.jpg"), llm.ImagePart([]byte("synthetic media omitted from inspection"), "image/jpeg"), llm.TextPart("files: fixture.jpg")}
	plan := StorylinePromptInput{Language: LanguageKorean, Title: input.Title, Memo: input.Memo, Photos: input.Photos, Observations: input.Observations, Template: input.Template, StockGuidelines: input.StockGuidelines, Guidelines: input.Guidelines, Memories: input.Memories}
	create := ComposeStorylineRequest(plan)
	plan.Current, plan.Request = []StorylineParagraph{{Text: "Synthetic existing AI plan", Files: []string{"fixture.jpg"}}}, "Change the proposed arrangement"
	rewrite := ComposeStorylineRequest(plan)
	return []llm.Request{direct, following, fullTest, composePhotoObservationRequest(photoParts, input.Photos, false), composeVideoObservationRequest("https://synthetic.invalid/private-link", "video/mp4", "fixture.mp4", false), composePhotoObservationRequest(photoParts, input.Photos, true), composeVideoObservationRequest("https://synthetic.invalid/private-link", "video/mp4", "fixture.mp4", true), create, rewrite, composeRevisionRequest(LanguageKorean, Profile{NoVoice: true}, PostContent{Title: "Synthetic current title", Summary: "Synthetic summary", Tags: []string{}, Blocks: []Block{{Type: BlockText, Content: "Current material with unconfirmed historical origins"}}}, input.Photos, input.Photos, nil, "Change the requested scope", nil, 3, input.Template, FrozenGuidelines{Stock: []StockGuideline{}})}
}

type promptMaterial struct {
	id, text, role, prefix string
	// An exact offset is used where the emitted container can itself contain
	// arbitrary owner delimiters. Otherwise prefix must be a code-owned boundary.
	offset *int
	author llm.FragmentAuthorship
}

func materialAt(id, role, text string, offset int) promptMaterial {
	return promptMaterial{id: id, role: role, text: text, offset: &offset}
}

// compositionSection splits only explicitly supplied values in their owning
// emitted section. It never interprets owner sentences or reconstructs legacy
// template roles. Code framing remains code, even in a User message. Concatenating
// each role's Text reproduces the exact prompt passed to the model.
func compositionSection(out *llm.RequestComposition, id string, role llm.InspectionRole, text, file string, materials ...promptMaterial) {
	position, frame := 0, 0
	appendFragment := func(fragmentID string, author llm.FragmentAuthorship, materialRole, value string) {
		if value == "" {
			return
		}
		out.Fragments = append(out.Fragments, llm.RequestFragment{ID: fragmentID, Role: role, Authorship: author, MaterialRole: materialRole, Text: value, SourceFiles: []string{file}, Activation: out.Activation})
	}
	for _, material := range materials {
		if material.text == "" {
			continue
		}
		start := -1
		if material.offset != nil {
			start = *material.offset
		} else if material.prefix != "" {
			if offset := strings.Index(text[position:], material.prefix+material.text); offset >= 0 {
				start = position + offset + len(material.prefix)
			}
		}
		if start < position || start+len(material.text) > len(text) || text[start:start+len(material.text)] != material.text {
			continue // the owning composer omitted this value
		}
		appendFragment(fmt.Sprintf("%s.frame.%d", id, frame), llm.FragmentAuthorshipCode, "code-framing-and-stage-contract", text[position:start])
		frame++
		author := material.author
		if author == "" {
			author = llm.FragmentAuthorshipAccount
		}
		appendFragment(id+"."+material.id, author, material.role, material.text)
		position = start + len(material.text)
	}
	appendFragment(fmt.Sprintf("%s.frame.%d", id, frame), llm.FragmentAuthorshipCode, "code-framing-and-stage-contract", text[position:])
}

func textMaterials(id, role string, values []string, indent bool) []promptMaterial {
	out := make([]promptMaterial, 0, len(values))
	for i, text := range values {
		if indent {
			text = strings.ReplaceAll(text, "\n", "\n  ")
		}
		out = append(out, promptMaterial{id: fmt.Sprintf("%s.%d", id, i), text: text, role: role, prefix: "\n- "})
	}
	return out
}

func profileComposition(out *llm.RequestComposition, language Language, profile Profile, length *int) string {
	var section strings.Builder
	writeProfileSection(&section, language, profile, length)
	materials := []promptMaterial{}
	if !profile.NoVoice {
		offset := 0
		if profile.Text != "" {
			materials = append(materials, materialAt("accepted-profile", "accepted-style-projection-not-post-facts", profile.Text, 2))
			offset = 2 + len(profile.Text)
		}
		if !profile.Portable {
			offset += len("\n\n[글 예시 발췌]")
			for i, excerpt := range profile.Excerpts {
				excerpt = marshalPromptJSON(excerpt)
				offset += len(fmt.Sprintf("\n%d. ", i+1))
				materials = append(materials, materialAt(fmt.Sprintf("example.%d", i), "style-example-not-post-facts", excerpt, offset))
				offset += len(excerpt)
			}
		}
	} else {
		out.Omissions = append(out.Omissions, llm.RequestOmission{ID: "voice-profile", Reason: "no voice selected; no profile or examples read", Activation: "NoVoice", SourceFiles: []string{"internal/generation/prompts.go"}})
	}
	compositionSection(out, "voice", llm.InspectionRoleSystem, section.String(), "internal/generation/prompts.go", materials...)
	return section.String()
}

func templateMaterials(brief *TemplateBrief, title bool) []promptMaterial {
	if brief == nil {
		return nil
	}
	known := brief.BodyParts != nil || brief.TitleParts != nil
	name := brief.Name
	if known {
		name = marshalPromptJSON(name)
	}
	materials := []promptMaterial{materialAt("name", "setting-label-not-post-facts", name, len("\n\n[글 템플릿: "))}
	var parts func(string, []TemplateMaterialPart)
	parts = func(path string, values []TemplateMaterialPart) {
		for i, value := range values {
			id := fmt.Sprintf("%s.%d", path, i)
			materials = append(materials, promptMaterial{id: id + ".text", text: marshalPromptJSON(value.Text), prefix: `"text":`, role: "template-" + value.Kind})
			if value.Label != "" {
				materials = append(materials, promptMaterial{id: id + ".label", text: marshalPromptJSON(value.Label), prefix: `"label":`, role: "template-field-scope"})
			}
			parts(id+".parts", value.Parts)
		}
	}
	if title {
		if known && brief.TitleParts != nil {
			parts("title", brief.TitleParts)
		} else {
			value := brief.TitleArea
			if known {
				value = marshalPromptJSON(value)
			}
			prefix := "\n---\n"
			if known {
				prefix = "\n[역할 미확인 보관 제목 형식]\n"
			}
			materials = append(materials, promptMaterial{id: "legacy-title", text: value, role: "legacy-template-role-unknown", prefix: prefix})
		}
	}
	if known && brief.BodyParts != nil {
		parts("body", brief.BodyParts)
	} else {
		value := brief.Body
		if known {
			value = marshalPromptJSON(value)
		}
		prefix := "\n---\n"
		if known {
			prefix = "\n[역할 미확인 보관 본문 형식]\n"
		}
		materials = append(materials, promptMaterial{id: "legacy-body", text: value, role: "legacy-template-role-unknown", prefix: prefix})
	}
	return materials
}

func templateComposition(out *llm.RequestComposition, brief *TemplateBrief, titleInstruction string, noVoice, plan bool) string {
	var section strings.Builder
	if brief == nil {
		out.Omissions = append(out.Omissions, llm.RequestOmission{ID: "template", Reason: "no frozen template selected", Activation: "template absent", SourceFiles: []string{"internal/generation/prompts.go"}})
		return ""
	}
	if plan {
		writeTemplateForm(&section, brief, "")
	} else {
		writeTemplateSection(&section, brief, titleInstruction, noVoice)
	}
	compositionSection(out, "template", llm.InspectionRoleSystem, section.String(), "internal/generation/material_composition.go", templateMaterials(brief, !plan)...)
	return section.String()
}

func guidelineComposition(out *llm.RequestComposition, stock []StockGuideline, legacy, owner []string, stage string, noVoice bool) string {
	var section strings.Builder
	selected := selectStockGuidelines(stock, legacy, stage)
	if stage == "storyline" {
		writeGuidelinesSectionClosedBy(&section, selected, owner, storylineGuidelinePrecedence)
	} else {
		writeGuidelinesSection(&section, selected, owner, noVoice)
	}
	// Selection uses the same declared stage/output policy as the real composer.
	for _, rule := range stock {
		if len(selectStockGuidelines([]StockGuideline{rule}, nil, stage)) > 0 {
			out.SelectedRuleIDs = append(out.SelectedRuleIDs, rule.Key)
		} else {
			out.Omissions = append(out.Omissions, llm.RequestOmission{ID: "stock." + rule.Key, Reason: "frozen stock rule does not apply to this stage's declared outputs", Activation: stage, SourceFiles: []string{"internal/generation/material_composition.go"}})
		}
	}
	if len(selected) == 0 && len(owner) == 0 {
		out.Omissions = append(out.Omissions, llm.RequestOmission{ID: "guidelines", Reason: "no applicable frozen stock or owner lines", Activation: stage + " declared outputs", SourceFiles: []string{"internal/generation/material_composition.go"}})
	}
	materials := textMaterials("owner-line", "owner-writing-instruction-not-event-evidence", owner, true)
	if len(materials) > 0 {
		materials[0].prefix = "\n" + ownerGuidelinesLabel + "\n- "
	}
	// Legacy stock text has no declared key/applicability. Preserve that unknown
	// identity rather than matching its wording to the current stock registry.
	if stock == nil {
		legacyMaterials := textMaterials("legacy-stock", "legacy-rule-identity-unknown", legacy, true)
		if len(legacyMaterials) > 0 {
			legacyMaterials[0].prefix = "\n" + defaultGuidelinesLabel + "\n- "
		}
		for i := range legacyMaterials {
			legacyMaterials[i].author = llm.FragmentAuthorshipCode
		}
		materials = append(legacyMaterials, materials...)
	}
	compositionSection(out, "guidelines", llm.InspectionRoleSystem, section.String(), "internal/generation/prompts.go", materials...)
	return section.String()
}

func postUserComposition(out *llm.RequestComposition, title, memo string, memories, photos, videos []string, observations []Observation, portraits map[string]bool) {
	compositionSection(out, "post-brief", llm.InspectionRoleUser, fmt.Sprintf("[이번 글]\n가제: %s\n메모: %s\n", title, memo), "internal/generation/prompts.go", materialAt("title", "owner-title-hint", title, len("[이번 글]\n가제: ")), materialAt("memo", "owner-supplied-post-material", memo, len("[이번 글]\n가제: ")+len(title)+len("\n메모: ")))
	compositionSection(out, "memories", llm.InspectionRoleUser, memorySection(memories), "internal/generation/prompts.go", textMaterials("fact", "selected-memory-fact", memories, false)...)
	if len(memories) == 0 {
		out.Omissions = append(out.Omissions, llm.RequestOmission{ID: "memories", Reason: "no opted-in matching frozen memory material", Activation: "memory selection", SourceFiles: []string{"internal/generation/prompts.go"}})
	}
	compositionSection(out, "attachments", llm.InspectionRoleUser, attachmentMaterial(photos, videos, observations, portraits), "internal/generation/prompts.go")
	if len(photos)+len(videos) > 0 {
		last := &out.Fragments[len(out.Fragments)-1]
		last.MaterialRole = "identified-media-observations-not-owner-impressions"
		last.SourceRefs = append(append([]string(nil), photos...), videos...)
	}
}

// ComposeWriteRequest is used by execution and can be inspected locally with
// llm.PreparedRequestInspection. Registry.Prepare adds effective options without
// admitting or dispatching work. Budget/reasoning and structured capability are
// still supplied by the executing owner at its existing seam.
func ComposeWriteRequest(input WritePromptInput) llm.Request {
	system, user := BuildWritePromptForLanguage(input)
	mode := "direct"
	if len(input.FollowStoryline) > 0 {
		mode = "frozen-storyline"
	}
	out := generationDescriptor(mode)
	// Build known suffixes before recording the fixed prefix so role order exactly
	// follows the actual prompt, including code frames surrounding account values.
	var suffix llm.RequestComposition
	suffix.Activation = out.Activation
	profile := profileComposition(&suffix, input.Language, input.Profile, input.TargetLength)
	template := templateComposition(&suffix, input.Template, templateTitleInstruction, input.Profile.NoVoice, false)
	guidelines := guidelineComposition(&suffix, input.StockGuidelines, input.DefaultGuidelines, input.Guidelines, "write", input.Profile.NoVoice)
	compositionSection(&out, "write-contract", llm.InspectionRoleSystem, system[:len(system)-len(profile)-len(template)-len(guidelines)], "internal/generation/prompts.go")
	out.Fragments, out.Omissions, out.SelectedRuleIDs = append(out.Fragments, suffix.Fragments...), suffix.Omissions, suffix.SelectedRuleIDs
	compositionSection(&out, "quality", llm.InspectionRoleUser, qualityRulesSection(input.QualityRules), "internal/generation/prompts.go", textMaterials("rule", "selected-published-quality-rule", input.QualityRules, false)...)
	postUserComposition(&out, input.Title, input.Memo, input.Memories, input.Photos, input.Videos, input.Observations, input.Portraits)
	planMaterials := make([]promptMaterial, 0, len(input.FollowStoryline))
	planOffset := len("\n\n[스토리라인]")
	for i, paragraph := range input.FollowStoryline {
		planOffset += len(fmt.Sprintf("\n%d. ", i+1))
		text := strings.Join(strings.Fields(paragraph.Text), " ")
		planMaterials = append(planMaterials, materialAt(fmt.Sprintf("paragraph.%d", i), "approved-ai-plan-not-event-evidence", text, planOffset))
		planOffset += len(text)
		if len(paragraph.Files) > 0 {
			planOffset += len(" (파일: " + strings.Join(paragraph.Files, ", ") + ")")
		}
	}
	compositionSection(&out, "stored-plan", llm.InspectionRoleUser, storylineSection(input.FollowStoryline), "internal/generation/prompts.go", planMaterials...)
	return llm.Request{System: system, Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart(user)}}}, Stage: llm.StageNameWrite, Composition: &out}
}

func ComposeStorylineRequest(input StorylinePromptInput) llm.Request {
	system, user := BuildStorylinePromptForLanguage(input)
	mode := "storyline-create"
	if input.Request != "" {
		mode = "storyline-rewrite"
	}
	out := generationDescriptor(mode)
	var suffix llm.RequestComposition
	suffix.Activation = out.Activation
	template := templateComposition(&suffix, input.Template, "", true, true)
	guidelines := guidelineComposition(&suffix, input.StockGuidelines, input.DefaultGuidelines, input.Guidelines, "storyline", true)
	compositionSection(&out, "plan-contract", llm.InspectionRoleSystem, system[:len(system)-len(template)-len(guidelines)], "internal/generation/storyline_prompts.go")
	out.Fragments, out.Omissions, out.SelectedRuleIDs = append(out.Fragments, suffix.Fragments...), suffix.Omissions, suffix.SelectedRuleIDs
	postUserComposition(&out, input.Title, input.Memo, input.Memories, input.Photos, input.Videos, input.Observations, nil)
	if input.Request != "" {
		compositionSection(&out, "plan-edit", llm.InspectionRoleUser, fmt.Sprintf("\n\n[현재 스토리라인]\n%s\n\n[수정 요청]\n%s", marshalPromptJSON(storylineForPrompt(input.Current)), input.Request), "internal/generation/storyline_prompts.go", materialAt("current", "current-ai-plan-not-event-evidence", marshalPromptJSON(storylineForPrompt(input.Current)), len("\n\n[현재 스토리라인]\n")), materialAt("request", "owner-plan-edit-and-explicit-new-facts", input.Request, len("\n\n[현재 스토리라인]\n")+len(marshalPromptJSON(storylineForPrompt(input.Current)))+len("\n\n[수정 요청]\n")))
	}
	return llm.Request{System: system, Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart(user)}}}, Stage: llm.StageNameWrite, Composition: &out}
}

func composeRevisionRequest(language Language, profile Profile, content PostContent, filenames, photos []string, portraits map[string]bool, instruction string, targetLength *int, tagCount int, template *TemplateBrief, guidelines FrozenGuidelines) llm.Request {
	system, user := buildRevisePrompt(language, profile, content, filenames, photos, portraits, instruction, targetLength, tagCount, template, guidelines)
	out := generationDescriptor("revision")
	var suffix llm.RequestComposition
	suffix.Activation = out.Activation
	profileSection := profileComposition(&suffix, language, profile, targetLength)
	templateSection := templateComposition(&suffix, template, reviseTemplateTitleInstruction, profile.NoVoice, false)
	guidelineSection := guidelineComposition(&suffix, guidelines.Stock, guidelines.Defaults, guidelines.Owner, "revise", profile.NoVoice)
	scope := ""
	if guidelineSection != "" {
		scope = "\n" + reviseGuidelineScope
	}
	compositionSection(&out, "revision-contract", llm.InspectionRoleSystem, system[:len(system)-len(profileSection)-len(templateSection)-len(guidelineSection)-len(scope)], "internal/generation/revise.go")
	out.Fragments, out.Omissions, out.SelectedRuleIDs = append(out.Fragments, suffix.Fragments...), suffix.Omissions, suffix.SelectedRuleIDs
	compositionSection(&out, "revision-guideline-scope", llm.InspectionRoleSystem, scope, "internal/generation/revise.go")
	compositionSection(&out, "revision-material", llm.InspectionRoleUser, user, "internal/generation/revise.go", materialAt("current-content", "current-post-content-historical-evidence-unconfirmed", marshalPromptJSON(contentForPrompt(content)), len("[현재 PostContent]\n")), materialAt("request", "owner-edit-instruction-and-explicit-new-facts", instruction, len(user)-len(instruction)))
	out.Omissions = append(out.Omissions, llm.RequestOmission{ID: "memories-and-observation", Reason: "revision performs neither memory retrieval nor observation", Activation: "revision", SourceFiles: []string{"internal/generation/revise_handler.go"}})
	return llm.Request{System: system, Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart(user)}}}, Stage: llm.StageNameWrite, Composition: &out}
}

func composePhotoObservationRequest(parts []llm.Part, filenames []string, fullTest bool) llm.Request {
	mode := "photo-observation"
	if fullTest {
		mode = "full-test-photo-observation"
	}
	out := generationDescriptor(mode)
	compositionSection(&out, "photo-contract", llm.InspectionRoleSystem, ObservePrompt, "internal/generation/prompts.go")
	for i, filename := range filenames {
		compositionSection(&out, fmt.Sprintf("photo-label.%d", i), llm.InspectionRoleUser, "file: "+filename, "internal/generation/observe.go", materialAt("filename", "attachment-identifier", filename, len("file: ")))
		out.Fragments = append(out.Fragments, llm.RequestFragment{ID: fmt.Sprintf("photo-media.%d", i), Role: llm.InspectionRoleUser, Authorship: llm.FragmentAuthorshipAccount, MaterialRole: "photo-evidence-media-body-redacted", SourceRefs: []string{filename}, SourceFiles: []string{"internal/generation/observe.go"}, Activation: "selected batch item"})
	}
	compositionSection(&out, "photo-file-set", llm.InspectionRoleUser, "files: "+strings.Join(filenames, ", "), "internal/generation/observe.go")
	return llm.Request{System: ObservePrompt, Messages: []llm.Message{{Role: llm.RoleUser, Parts: parts}}, Stage: llm.StageNameObserve, Composition: &out}
}

func composeVideoObservationRequest(url, contentType, filename string, fullTest bool) llm.Request {
	mode := "video-observation"
	if fullTest {
		mode = "full-test-video-observation"
	}
	out := generationDescriptor(mode)
	compositionSection(&out, "video-contract", llm.InspectionRoleSystem, ObserveVideoPrompt, "internal/generation/prompts.go")
	out.Fragments = append(out.Fragments, llm.RequestFragment{ID: "video-media", Role: llm.InspectionRoleUser, Authorship: llm.FragmentAuthorshipAccount, MaterialRole: "video-evidence-signed-link-and-body-redacted", SourceRefs: []string{filename}, SourceFiles: []string{"internal/generation/observe.go"}, Activation: "one selected video"})
	compositionSection(&out, "video-file-set", llm.InspectionRoleUser, "files: "+filename, "internal/generation/observe.go", materialAt("filename", "attachment-identifier", filename, len("files: ")))
	return llm.Request{System: ObserveVideoPrompt, Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.VideoPart(url, contentType), llm.TextPart("files: " + filename)}}}, Stage: llm.StageNameObserve, Composition: &out}
}
