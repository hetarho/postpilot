package authoring

import (
	"encoding/json"
	"fmt"
	"github.com/postpilot/backend/internal/llm"
	"strings"
)

func authoringDescriptor(kind Kind, mode Mode) llm.RequestComposition {
	return llm.RequestComposition{Stage: "setting-authoring", Mode: string(kind) + "/" + string(mode), PromptVersion: "authoring-request-v1", SchemaVersion: "authoring-response-v1", Composer: "authoring.Service.Run/modelMessage", Parser: "authoring.Service.parseResponse/Targets.Validate", Consumer: "authoring operation output and unpublished candidate/draft", Activation: "explicit admitted recommend or refine; frozen kind, count, draft and recent completed conversation", SourceFiles: []string{"internal/authoring/run.go", "internal/authoring/ports.go"}, Output: llm.OutputContractInspection{Name: "authoring-artifacts", Version: "authoring-response-v1"}}
}

// RequestCompositions is a code-owned inventory; it reads no account and executes no work.
func RequestCompositions() []llm.RequestComposition {
	var result []llm.RequestComposition
	for _, kind := range []Kind{PostTemplate, VideoTemplate, PostGuideline, VideoGuideline, WritingVoice} {
		for _, mode := range []Mode{Recommend, Refine} {
			c := authoringComposition(operationInput{Kind: kind, Mode: mode, CandidateCount: 4, Guide: "Synthetic kind grammar guide", Purpose: "Synthetic setting purpose", Prompt: "Synthetic owner direction", SourceContext: "Synthetic style context", ReferencePost: "Synthetic structural reference", Selected: &artifactWire{Name: "Synthetic draft", Body: "Synthetic draft body"}, History: []historyWire{{Request: "Synthetic prior request", Reply: "Synthetic prior AI reply"}}})
			result = append(result, *c)
		}
	}
	return result
}
func authoringComposition(in operationInput) *llm.RequestComposition {
	out := authoringDescriptor(in.Kind, in.Mode)
	out.Output.Schema = string(responseSchema(in.Mode, in.CandidateCount))
	estimate := int64(in.PromptTokens)
	if estimate > 0 {
		out.ReferenceTokenEstimate = &estimate
	}
	out.Fragments = []llm.RequestFragment{{ID: "authoring-contract", Role: llm.InspectionRoleSystem, Authorship: llm.FragmentAuthorshipCode, MaterialRole: "setting-grammar-output-and-fictional-style-scene-contract", Text: authoringSystem, SourceFiles: []string{"internal/authoring/run.go"}}}
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
