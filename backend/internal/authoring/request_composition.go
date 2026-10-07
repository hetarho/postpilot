package authoring

import (
	"encoding/json"
	"fmt"
	"github.com/postpilot/backend/internal/llm"
	"strings"
)

func authoringDescriptor(kind Kind, mode Mode) llm.RequestComposition {
	return llm.RequestComposition{Stage: "setting-authoring", Mode: string(kind) + "/" + string(mode), PromptVersion: "authoring-request-v2", SchemaVersion: "authoring-response-v2", Composer: "authoring.Service.Run/modelMessage", Parser: "authoring.Service.parseResponse/Targets.Validate", Consumer: "authoring operation output and unpublished candidate/draft", Activation: "explicit admitted recommend or refine; frozen kind, count, draft and recent completed conversation", SourceFiles: []string{"internal/authoring/run.go", "internal/authoring/ports.go", "internal/authoring/request_composition.go"}, Output: llm.OutputContractInspection{Name: "authoring-artifacts", Version: "authoring-response-v2"}}
}

// RequestCompositions is a code-owned inventory; it reads no account and executes no work.
func RequestCompositions(guides ...map[Kind]string) []llm.RequestComposition {
	var result []llm.RequestComposition
	for _, kind := range []Kind{PostTemplate, VideoTemplate, PostGuideline, VideoGuideline, WritingVoice} {
		for _, mode := range []Mode{Recommend, Refine} {
			guide := "Synthetic kind grammar guide"
			if len(guides) > 0 {
				guide = guides[0][kind]
			}
			body := "구체적인 작성 방향"
			description := ""
			reference := ""
			switch kind {
			case PostTemplate:
				body = "<write>방문 이유와 첫인상</write>"
				description = "글 구조의 설명"
				reference = "Synthetic structural reference"
			case VideoTemplate:
				body = `<clip version="1"><stage name="방문">방문 장면의 흐름</stage></clip>`
			case WritingVoice:
				body = strings.Repeat("가상의 산책과 차 한 잔을 한국어 말투로 이야기해요. ", 10)
				description = "가상 말투의 설명"
			}
			c := authoringComposition(operationInput{Kind: kind, Mode: mode, CandidateCount: 4, Guide: guide, Purpose: "Synthetic setting purpose", Prompt: "Synthetic owner direction", SourceContext: "Synthetic style context", ReferencePost: reference, Selected: &artifactWire{Name: "Synthetic draft", Description: description, Body: body}, History: []historyWire{{Request: "Synthetic prior request", Reply: "Synthetic prior AI reply"}}})
			result = append(result, *c)
		}
	}
	return result
}
func authoringComposition(in operationInput) *llm.RequestComposition {
	out := authoringDescriptor(in.Kind, in.Mode)
	switch in.Kind {
	case PostTemplate:
		out.SourceFiles = append(out.SourceFiles, "internal/template/authoring.go", "internal/template/guide.go")
	case VideoTemplate:
		out.SourceFiles = append(out.SourceFiles, "internal/clip/app/authoring.go")
	case PostGuideline, VideoGuideline:
		out.SourceFiles = append(out.SourceFiles, "internal/guideline/authoring.go")
	case WritingVoice:
		out.SourceFiles = append(out.SourceFiles, "internal/voice/authoring.go")
	}
	out.Output.Schema = string(kindResponseSchema(in.Kind, in.Mode, in.CandidateCount))
	estimate := int64(in.PromptTokens)
	if estimate > 0 {
		out.ReferenceTokenEstimate = &estimate
	}
	out.Fragments = authoringRules(in)
	out.Omissions = []llm.RequestOmission{{ID: "other-setting-kinds", Reason: "only selected kind grammar and consumed fields apply", Activation: string(in.Kind), SourceFiles: []string{"internal/authoring/run.go"}}, {ID: "unsupported-modes", Reason: "only explicit recommend/refine are admitted; inspecting creates no helper start or publication", Activation: "mode admission", SourceFiles: []string{"internal/authoring/service.go"}}}
	if in.Mode == Refine {
		out.Omissions = append(out.Omissions, llm.RequestOmission{ID: "recommendation-batch", Reason: "candidate count and new recommendation structures are not refinement outputs", Activation: string(in.Mode), SourceFiles: []string{"internal/authoring/run.go"}})
	}
	if in.Kind != WritingVoice {
		out.Omissions = append(out.Omissions, llm.RequestOmission{ID: "style-source", Reason: "accepted personal/synthetic style context applies only to writing style authoring", Activation: string(in.Kind), SourceFiles: []string{"internal/authoring/run.go"}})
	}
	user := modelMessage(in)
	decoder := json.NewDecoder(strings.NewReader(user))
	_, _ = decoder.Token()
	last := int64(0)
	framing := 0
	for decoder.More() {
		keyToken, _ := decoder.Token()
		key, _ := keyToken.(string)
		start := decoder.InputOffset()
		for start < int64(len(user)) && (user[start] == ':' || user[start] == ' ' || user[start] == '\n') {
			start++
		}
		var raw json.RawMessage
		if decoder.Decode(&raw) != nil {
			break
		}
		end := decoder.InputOffset()
		out.Fragments = append(out.Fragments, llm.RequestFragment{ID: fmt.Sprintf("envelope-%d", framing), Role: llm.InspectionRoleUser, Authorship: llm.FragmentAuthorshipCode, MaterialRole: "authoring-input-envelope", Text: user[last:start], SourceFiles: []string{"internal/authoring/run.go"}})
		framing++
		author, material := llm.FragmentAuthorshipAccount, "setting-direction"
		switch key {
		case "candidate_count", "kind", "mode":
			author = llm.FragmentAuthorshipCode
			material = "frozen-product-mode-count"
		case "guide":
			author = llm.FragmentAuthorshipCode
			material = "kind-specific-format-guide"
		case "request":
			material = "explicit-recommendation-or-refinement"
		case "source_style":
			material = "style-source-context-unconfirmed"
		case "reference_post":
			material = "structure-reference-not-event-evidence"
		case "draft":
			material = "unpublished-setting"
		case "recent_conversation":
			material = "completed-owner-requests-and-ai-replies"
		}
		out.Fragments = append(out.Fragments, llm.RequestFragment{ID: key, Role: llm.InspectionRoleUser, Authorship: author, MaterialRole: material, Text: user[start:end], SourceFiles: []string{"internal/authoring/run.go"}})
		last = end
	}
	if last < int64(len(user)) {
		out.Fragments = append(out.Fragments, llm.RequestFragment{ID: "envelope-tail", Role: llm.InspectionRoleUser, Authorship: llm.FragmentAuthorshipCode, MaterialRole: "authoring-input-envelope", Text: user[last:], SourceFiles: []string{"internal/authoring/run.go"}})
	}

	return &out
}
