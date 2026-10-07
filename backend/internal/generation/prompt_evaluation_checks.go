package generation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/post"
)

type PromptEvaluationCheck struct {
	CaseID string `json:"case_id"`
	ID     string `json:"id"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail"`
}

func evaluationContentWire(content PostContent) map[string]any {
	blocks := make([]map[string]any, 0, len(content.Blocks))
	for _, block := range content.Blocks {
		items, files := block.Items, block.Files
		if items == nil {
			items = []string{}
		}
		if files == nil {
			files = []string{}
		}
		blocks = append(blocks, map[string]any{"type": string(block.Type), "content": block.Content, "level": block.Level, "file": block.File, "files": files, "layout": block.Layout, "alt": block.Alt, "caption": block.Caption, "items": items})
	}
	return map[string]any{"title": content.Title, "summary": content.Summary, "tags": content.Tags, "blocks": blocks}
}

func syntheticPromptEvaluationResponse(mode string, protocol int) string {
	material := syntheticPromptEvaluationMaterial()
	var core any
	var origins []originCandidateJSON
	zero, one := 0, 1
	switch mode {
	case "photo-observation", "full-test-photo-observation", "video-observation", "full-test-video-observation":
		observations := material.post.Observations[:2]
		video := mode == "video-observation" || mode == "full-test-video-observation"
		if video {
			observations = material.post.Observations[2:]
		}
		wire := make([]map[string]any, 0, len(observations))
		for i, observation := range observations {
			row := map[string]any{"file": observation.File, "scene": observation.Scene, "mood": observation.Mood, "visible_text": observation.VisibleText, "objects": []string{}, "people_present": observation.PeoplePresent}
			if video {
				row["events"], row["speech"] = observation.Events, observation.Speech
			} else {
				row["rotation"] = observation.Rotation
			}
			wire = append(wire, row)
			origins = append(origins, originCandidateJSON{File: observation.File, Field: originFieldJSON{Kind: "observation_scene"}, Quote: observation.Scene, Category: post.OriginPhotoInterpretation, SourceRefs: []string{fmt.Sprintf("media.%d", i)}})
		}
		core = map[string]any{"observations": wire}
	case "storyline-create", "storyline-rewrite":
		core = storylineForPrompt(material.plan)
		origins = []originCandidateJSON{{Field: originFieldJSON{Kind: "storyline_paragraph", ParagraphIndex: &zero}, Quote: "맛있었다는 감상", Category: post.OriginOwnerInput, SourceRefs: []string{"current.memo"}}}
	case "revision":
		content := *material.post.Content
		content.Blocks = slices.Clone(content.Blocks)
		content.Blocks[0].Content = strings.Replace(content.Blocks[0].Content, "저는 맛있었어요.", "제 입맛에는 맛있었어요.", 1)
		core = evaluationContentWire(content)
		origins = []originCandidateJSON{{Field: originFieldJSON{Kind: "block_content", BlockIndex: &zero}, Quote: "제 입맛에는 맛있었어요.", Category: post.OriginOwnerInput, SourceRefs: []string{"current.edit"}}}
	default:
		content := *material.post.Content
		content.Blocks = slices.Clone(content.Blocks)
		// Deliberate semantic traps: compatible references and valid ranges do
		// not prove rank or chronology. These are fixed adversarial outputs,
		// never results observed from a model and never semantic passing claims.
		content.Blocks[0].Content = "저는 맛있었어요. 창가 옆 초록 식물이 보였어요. 전국 1위이고 먼저 창가로 옮겼어요."
		wire := evaluationContentWire(content)
		wire["nouns"] = []string{"초록창가", "식물", "컵"}
		if mode != "frozen-storyline" {
			wire["storyline"] = storylineForPrompt(material.plan)["storyline"]
		}
		core = wire
		origins = []originCandidateJSON{
			{Field: originFieldJSON{Kind: "block_content", BlockIndex: &zero}, Quote: "저는 맛있었어요.", Category: post.OriginOwnerInput, SourceRefs: []string{"current.memo"}},
			{Field: originFieldJSON{Kind: "block_content", BlockIndex: &zero}, Quote: "창가 옆 초록 식물", Category: post.OriginPhotoInterpretation, SourceRefs: []string{"current.visual.0.0"}},
			{Field: originFieldJSON{Kind: "block_content", BlockIndex: &zero}, Quote: "전국 1위", Category: post.OriginOwnerInput, SourceRefs: []string{"current.memo"}},
			{Field: originFieldJSON{Kind: "block_content", BlockIndex: &zero}, Quote: "먼저 창가로 옮겼어요.", Category: post.OriginAIAdded, SourceRefs: []string{}},
			{Field: originFieldJSON{Kind: "block_content", BlockIndex: &one}, Quote: "좋아요😊", Category: post.OriginOwnerInput, SourceRefs: []string{"current.memo"}},
			{Field: originFieldJSON{Kind: "block_content", BlockIndex: &one}, Quote: "좋아요😊", Occurrence: &zero, Category: post.OriginOwnerInput, SourceRefs: []string{"current.memo"}},
			{Field: originFieldJSON{Kind: "block_content", BlockIndex: &one}, Quote: "좋아요😊", Occurrence: &one, Category: post.OriginOwnerInput, SourceRefs: []string{"current.memo"}},
			{Field: originFieldJSON{Kind: "tag", TagIndex: &zero}, Quote: " 초록창가 ", Category: post.OriginOwnerInput, SourceRefs: []string{"fabricated.source"}},
			{Field: originFieldJSON{Kind: "summary"}, Quote: "존재하지 않는 구절", Category: post.OriginOwnerInput, SourceRefs: []string{"current.memo"}},
		}
	}
	raw := marshalPromptJSON(core)
	if protocol == OriginProtocolVersion {
		wire := make([]map[string]any, 0, len(origins))
		for _, candidate := range origins {
			row := map[string]any{"field": candidate.Field, "quote": candidate.Quote, "category": candidate.Category, "source_refs": candidate.SourceRefs}
			if candidate.File != "" {
				row["file"] = candidate.File
			}
			if candidate.Occurrence != nil {
				row["occurrence"] = *candidate.Occurrence
			}
			wire = append(wire, row)
		}
		raw = strings.TrimSuffix(raw, "}") + `,"origins":` + marshalPromptJSON(wire) + "}"
	}
	return raw
}

func promptEvaluationSources(request llm.Request) ([]post.OriginSource, error) {
	for _, fragment := range request.Composition.Fragments {
		if fragment.ID != "origin-supplied-catalog" {
			continue
		}
		_, raw, found := strings.Cut(fragment.Text, "\n\n[Result-local supplied origin material]\n")
		if !found {
			return nil, fmt.Errorf("missing actual origin material wrapper")
		}
		var data struct {
			Sources []originSourceJSON `json:"sources"`
		}
		if err := json.Unmarshal([]byte(raw), &data); err != nil {
			return nil, err
		}
		return decodeOriginSources(data.Sources), nil
	}
	return nil, nil
}

// CheckPromptEvaluationCases verifies composition and fixed synthetic response
// structure only. It neither judges source truth/prose nor computes a winner,
// usable model-response rate, provider usage, cost or statistical superiority.
func CheckPromptEvaluationCases(cases []PromptEvaluationCase) []PromptEvaluationCheck {
	var checks []PromptEvaluationCheck
	baselines := map[string]PromptEvaluationCase{}
	for _, item := range cases {
		if item.InstructionLanguage == PromptInstructionsKorean {
			baselines[item.Scenario] = item
		}
	}
	for _, item := range cases {
		add := func(id string, passed bool, detail string) {
			checks = append(checks, PromptEvaluationCheck{CaseID: item.ID, ID: id, Passed: passed, Detail: detail})
		}
		inspection, err := llm.PreparedRequestInspection(item.Request)
		add("actual-composition", err == nil && inspection.Status == llm.InspectionPrepared && inspection.IssuedAt == nil && inspection.Conditions == nil && inspection.Measures.ProviderPromptTokens == nil, "Unissued actual application composition; no runtime usage or dispatch claim.")
		if err != nil || item.Request.Composition == nil {
			continue
		}
		var system, user, actualUser strings.Builder
		for _, fragment := range inspection.Fragments {
			if fragment.Role == llm.InspectionRoleSystem {
				system.WriteString(fragment.Text)
			} else if fragment.Role == llm.InspectionRoleUser {
				user.WriteString(fragment.Text)
			}
		}
		for _, message := range item.Request.Messages {
			for _, part := range message.Parts {
				if !part.IsImage() && !part.IsVideo() {
					actualUser.WriteString(part.Text)
				}
			}
		}
		add("ordered-fragment-bytes", system.String() == item.Request.System && user.String() == actualUser.String(), "System/User ordered fragments reproduce the actual prompt bytes.")
		add("actual-schema-identity", bytes.Equal(item.Request.JSONSchema, []byte(inspection.Output.Schema)), "Execution preparation selected the declared exact schema bytes for this admitted protocol.")
		baseline, found := baselines[item.Scenario]
		compiled, compileErr := compilePromptEvaluationRequest(baseline.Request, item.InstructionLanguage)
		add("instruction-condition-parity", found && compileErr == nil && reflect.DeepEqual(compiled, item.Request) && baseline.OutputLanguage == LanguageKorean && item.OutputLanguage == baseline.OutputLanguage && baseline.ModelRef == item.ModelRef && baseline.OriginProtocolVersion == item.OriginProtocolVersion, "Only named common task/format fragments change; Korean material, policies, examples, output target, schema, model, cap, effort and origin catalog remain fixed.")
		add("synthetic-response-provenance", item.ResponseProvenance == "fixed-synthetic-output-not-model-response" && item.SyntheticResponse == syntheticPromptEvaluationResponse(inspection.Mode, item.OriginProtocolVersion), "Fixed adversarial response bytes exercise parsers; they are not observations of model performance.")
		checkPromptEvaluationResponse(item, add)
	}
	return checks
}

func checkPromptEvaluationResponse(item PromptEvaluationCase, add func(string, bool, string)) {
	mode := item.Request.Composition.Mode
	material := syntheticPromptEvaluationMaterial()
	photos, videos := AttachmentNames(material.post.Images)
	files := append(slices.Clone(photos), videos...)
	sources, sourceErr := promptEvaluationSources(item.Request)
	add("bounded-actual-source-catalog", sourceErr == nil && (item.OriginProtocolVersion == 0 || len(sources) > 0), "Read actual request-local catalog IDs; no source is inferred from arbitrary prose.")
	switch mode {
	case "photo-observation", "full-test-photo-observation", "video-observation", "full-test-video-observation":
		observations, err := parseObservations(item.SyntheticResponse)
		video := mode == "video-observation" || mode == "full-test-video-observation"
		expected, kind := 2, AttachmentPhoto
		if video {
			expected, kind = 1, AttachmentVideo
		}
		add("actual-response-parser", err == nil && len(observations) == expected, "Actual observation parser accepts the fixed complete synthetic output.")
		if err == nil && item.OriginProtocolVersion == OriginProtocolVersion {
			valid := true
			for _, observation := range observations {
				review := ValidateObservationOrigins(observation, sources, kind)
				valid = valid && len(review.Spans) == 1 && review.Spans[0].Quote == observation.Scene
			}
			add("identified-observation-reference-range", valid, "Exact file-local media IDs and quote ranges validate; synthetic media supplies no proof that the scene is true.")
		}
	case "storyline-create", "storyline-rewrite":
		paragraphs, candidates, err := ParseStorylineAnswerWithOrigins(item.SyntheticResponse, files)
		add("actual-response-parser", err == nil && reflect.DeepEqual(paragraphs, material.plan), "Actual storyline parser accepts plan-only output and all exact attachment names.")
		if err == nil && item.OriginProtocolVersion == OriginProtocolVersion {
			review := ValidatePlanOrigins(paragraphs, sources, candidates)
			add("plan-reference-range", len(review.Spans) == 1 && review.Spans[0].Quote == "맛있었다는 감상", "Exact plan-local quote and catalog reference validate; plan approval supplies no new event facts.")
		}
	case "revision":
		content, candidates, err := ParseRevisionContentWithOrigins(item.SyntheticResponse, material.post.TagCount, *material.post.Content)
		add("actual-response-parser", err == nil, "Actual revision parser accepts the complete fixed replacement content.")
		if err != nil {
			return
		}
		expected := *material.post.Content
		expected.Blocks = slices.Clone(expected.Blocks)
		expected.Blocks[0].Content = strings.Replace(expected.Blocks[0].Content, "저는 맛있었어요.", "제 입맛에는 맛있었어요.", 1)
		add("requested-only-revision-fixture", reflect.DeepEqual(*content, expected), "The fixed replacement changes exactly the requested phrase and preserves title, summary, tag whitespace/order, quotations, emoji and other blocks; this does not prove a model obeyed the request.")
		add("revision-excludes-memory-material", !strings.Contains(actualEvaluationUserText(item.Request), material.post.Memories[0]), "Actual revision preparation reads no selected memory material or new observation.")
		if item.OriginProtocolVersion == OriginProtocolVersion {
			resolved := ResolveWriteOriginCandidates(*content, sources, candidates)
			add("edit-reference-range", len(resolved.Review.Spans) == 1 && resolved.Review.Spans[0].Quote == "제 입맛에는 맛있었어요.", "Requested phrase binds to the exact supplied current.edit catalog ID and Unicode range.")
		}
	default:
		var answer *WriteAnswer
		var err error
		if mode == "frozen-storyline" {
			answer, err = ParseWriteAlongStorylineAnswer(item.SyntheticResponse, material.post.TagCount, files)
		} else {
			answer, err = ParseWriteAnswer(item.SyntheticResponse, material.post.TagCount, files)
		}
		add("actual-response-parser", err == nil, "Actual writing parser accepts the fixed complete canonical content and mode-specific plan shape.")
		if err != nil {
			return
		}
		add("sparse-tags-under-cap", reflect.DeepEqual(answer.Content.Tags, material.post.Content.Tags) && len(answer.Content.Tags) < material.post.TagCount, "One fixed fixture tag is accepted below six; no padding to the cap. Grounding remains human review.")
		zeroTag := strings.Replace(item.SyntheticResponse, `"tags":[" 초록창가 "]`, `"tags":[]`, 1)
		zeroAnswer, zeroErr := ParseContent(zeroTag, material.post.TagCount)
		add("zero-tags-under-cap", zeroErr == nil && len(zeroAnswer.Tags) == 0, "The actual content parser accepts zero tags.")
		add("repeated-quote-emoji-bytes", answer.Content.Blocks[1].Content == "좋아요😊 좋아요😊", "Repeated Korean quotation and emoji survive actual parsing byte-for-byte.")
		if item.OriginProtocolVersion == OriginProtocolVersion {
			resolved := ResolveWriteOriginCandidates(answer.Content, sources, answer.OriginCandidates)
			issues := map[post.OriginIssueCode]bool{}
			for _, issue := range resolved.Issues {
				issues[issue.Code] = true
			}
			add("invalid-reference-ambiguous-quote-rejection", issues[post.OriginIssueUnknownSource] && issues[post.OriginIssueAmbiguous] && issues[post.OriginIssueQuote], "Invented catalog IDs, undisambiguated repeated quotes and absent exact quotes are rejected without discarding canonical content.")
			valid, repeated, owner, visual := true, 0, false, false
			for _, span := range resolved.Review.Spans {
				text, exists := post.OriginFieldText(originPostContent(answer.Content), span.Field)
				runes := []rune(text)
				valid = valid && exists && span.Start >= 0 && span.End <= len(runes) && span.End > span.Start && string(runes[span.Start:span.End]) == span.Quote && span.End-span.Start == utf8.RuneCountInString(span.Quote)
				if span.Quote == "좋아요😊" {
					repeated++
				}
				owner = owner || span.Quote == "저는 맛있었어요." && span.Category == post.OriginOwnerInput
				visual = visual || span.Quote == "창가 옆 초록 식물" && span.Category == post.OriginPhotoInterpretation
			}
			add("mixed-source-unicode-occurrences", valid && repeated == 2 && owner && visual, "Different meaning ranges in one sentence retain separate compatible labels; occurrence 0/1 resolves two emoji-bearing quotes without byte offsets.")
			semanticTrap := false
			for _, span := range resolved.Review.Spans {
				semanticTrap = semanticTrap || span.Quote == "전국 1위" && span.Category == post.OriginOwnerInput
			}
			add("compatible-label-does-not-certify-truth", semanticTrap, "Deliberately unsupported rank uses a structurally compatible memo ID and passes reference/range checks. Human review must assess source support; this passing check makes no semantic-support claim.")
		}
	}
}

func actualEvaluationUserText(request llm.Request) string {
	var out strings.Builder
	for _, message := range request.Messages {
		for _, part := range message.Parts {
			if !part.IsImage() && !part.IsVideo() {
				out.WriteString(part.Text)
			}
		}
	}
	return out.String()
}
