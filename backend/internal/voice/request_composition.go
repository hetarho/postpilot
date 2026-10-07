package voice

import (
	"encoding/json"
	"fmt"
	"github.com/postpilot/backend/internal/llm"
	"strings"
)

func voiceDescriptor(mode string) llm.RequestComposition {
	c := llm.RequestComposition{Stage: "writing-style", Mode: mode, PromptVersion: "writing-style-" + mode + "-v1", SchemaVersion: "writing-style-" + mode + "-v1"}
	switch mode {
	case "analyze":
		c.Composer = "voice.analysisRequest/analysisInput"
		c.Parser = "voice.parseAIPart"
		c.Consumer = "private accepted personal impression, tics, signature phrases and verified quotations; measured habits remain product-owned"
		c.Activation = "explicit admitted analysis of frozen owner-authored writing revisions; generated examples and posts excluded"
		c.SourceFiles = []string{"internal/voice/analysis.go", "internal/voice/corpus.go", "internal/voice/request_composition.go", "internal/voice/schemas.go"}
		c.Output = llm.OutputContractInspection{Name: "voice-analysis", Version: c.SchemaVersion, Schema: string(VoiceAnalysisSchema())}
	case "recommend":
		c.Composer = "voice.candidateRequest"
		c.Parser = "voice.parseCandidates"
		c.Consumer = "unpublished synthetic writing-style candidates"
		c.Activation = "explicit admitted recommendation of 2, 4, 8 or 16 candidates over code-owned fictional scene"
		c.SourceFiles = []string{"internal/voice/candidates.go", "internal/voice/candidate_schema.go", "internal/voice/request_composition.go"}
		c.Output = llm.OutputContractInspection{Name: "writing-candidates", Version: c.SchemaVersion}
	case "legacy-check":
		c.Composer = "voice.Service.writePiece"
		c.Parser = "voice.Service.writePiece nonempty text"
		c.Consumer = "legacy admitted check/reflection comparison pieces"
		c.Activation = "admitted-only old check_voice or frozen reflection work; new standalone helper admission retired"
		c.SourceFiles = []string{"internal/voice/check.go", "internal/voice/check_handler.go", "internal/voice/reflection.go", "internal/voice/request_composition.go"}
		c.Omissions = []llm.RequestOmission{{ID: "new-check-admission", Reason: "standalone voice checks and reflection starts are retired", Activation: "retained admitted work only", SourceFiles: []string{"internal/voice/check.go"}}}
		c.Output = llm.OutputContractInspection{Name: "writing-piece", Version: c.SchemaVersion}
	}
	return c
}
func RequestCompositions() []llm.RequestComposition {
	return []llm.RequestComposition{*analysisComposition(Fingerprint{}, []Sample{{ID: "synthetic-sample", Body: "합성 예시 학습 글이에요."}}), *candidateComposition(candidateInput{Count: 4, Directions: []string{"Synthetic plain", "Synthetic gentle", "Synthetic concise", "Synthetic lively"}, Scene: "A fictional walk and tea"}), *checkComposition("Synthetic frozen style", Prompt{Key: "synthetic-photo-question", Photo: true, Scene: "A fictional situation", Text: "Synthetic catalog question"})}
}
func analysisComposition(counted Fingerprint, samples []Sample) *llm.RequestComposition {
	c := voiceDescriptor("analyze")
	c.Fragments = []llm.RequestFragment{{ID: "analysis-contract", Role: llm.InspectionRoleSystem, Authorship: llm.FragmentAuthorshipCode, MaterialRole: "accepted-corpus-analysis-contract", Text: analysisPrompt, SourceFiles: []string{"internal/voice/analysis.go"}}, {ID: "counted-habits", Role: llm.InspectionRoleUser, Authorship: llm.FragmentAuthorshipCode, MaterialRole: "product-measurements", Text: "[제품이 센 습관]\n" + strings.Join(countedLines(counted), "\n") + "\n\n[학습 글]\n", SourceFiles: []string{"internal/voice/fingerprint.go"}}}
	for i, sample := range samples {
		refs := []string(nil)
		if sample.ID != "" {
			refs = []string{sample.ID}
		}
		if i > 0 {
			c.Fragments = append(c.Fragments, llm.RequestFragment{ID: fmt.Sprintf("sample-separator-%d", i), Role: llm.InspectionRoleUser, Authorship: llm.FragmentAuthorshipCode, MaterialRole: "corpus-envelope", Text: "\n\n"})
		}
		c.Fragments = append(c.Fragments, llm.RequestFragment{ID: fmt.Sprintf("sample-heading-%d", i), Role: llm.InspectionRoleUser, Authorship: llm.FragmentAuthorshipCode, MaterialRole: "corpus-envelope", Text: fmt.Sprintf("===== 학습 글 %d: ", i+1)})
		titleAuthor := llm.FragmentAuthorshipAccount
		if sample.Kind == SampleKindAnswer {
			titleAuthor = llm.FragmentAuthorshipCode
		}
		c.Fragments = append(c.Fragments, llm.RequestFragment{ID: fmt.Sprintf("sample-title-%d", i), Role: llm.InspectionRoleUser, Authorship: titleAuthor, MaterialRole: "sample-label-or-catalog-question", Text: sample.Title()}, llm.RequestFragment{ID: fmt.Sprintf("sample-heading-end-%d", i), Role: llm.InspectionRoleUser, Authorship: llm.FragmentAuthorshipCode, MaterialRole: "corpus-envelope", Text: " =====\n"})
		c.Fragments = append(c.Fragments, llm.RequestFragment{ID: fmt.Sprintf("accepted-sample-%d", i), Role: llm.InspectionRoleUser, Authorship: llm.FragmentAuthorshipAccount, MaterialRole: "accepted-owner-writing", Text: ProseText(sample.Body), SourceRefs: refs, SourceFiles: []string{"internal/voice/analysis.go"}})
	}
	n := int64(promptTokens(llm.Request{System: analysisPrompt, Messages: []llm.Message{{Parts: []llm.Part{llm.TextPart(analysisInput(counted, samples))}}}}))
	c.ReferenceTokenEstimate = &n
	return &c
}
func candidateComposition(in candidateInput) *llm.RequestComposition {
	c := voiceDescriptor("recommend")
	count, _ := NormalizeCandidateCount(in.Count)
	c.Output.Schema = string(WritingCandidateSchema(count))
	raw, _ := json.Marshal(struct {
		Directions []string `json:"directions"`
		Scene      string   `json:"scene"`
	}{in.Directions, in.Scene})
	c.Fragments = []llm.RequestFragment{{ID: "candidate-contract", Role: llm.InspectionRoleSystem, Authorship: llm.FragmentAuthorshipCode, MaterialRole: "synthetic-style-candidate-contract", Text: fmt.Sprintf(candidateSystem, count, count, count, count), SourceFiles: []string{"internal/voice/candidates.go"}}, {ID: "fictional-directions-and-scene", Role: llm.InspectionRoleUser, Authorship: llm.FragmentAuthorshipCode, MaterialRole: "fictional-example-not-owner-fact", Text: string(raw), SourceFiles: []string{"internal/voice/candidates.go"}}}

	return &c
}
func checkComposition(projection string, prompt Prompt) *llm.RequestComposition {
	c := voiceDescriptor("legacy-check")
	refs := []string(nil)
	if prompt.Key != "" {
		refs = []string{prompt.Key}
	}
	c.Fragments = []llm.RequestFragment{{ID: "frozen-style", Role: llm.InspectionRoleSystem, Authorship: llm.FragmentAuthorshipAccount, MaterialRole: "frozen-style-only-context-not-post-facts", Text: projection}, {ID: "catalog-task", Role: llm.InspectionRoleUser, Authorship: llm.FragmentAuthorshipCode, MaterialRole: "catalog-example-task-not-owner-experience", Text: "[상황]\n" + prompt.Scene + "\n[문항]\n" + prompt.Text + "\n[요청]\n" + checkRequest, SourceRefs: refs, SourceFiles: []string{"internal/voice/prompts.go", "internal/voice/check_handler.go"}}}
	if prompt.Photo {
		c.Fragments = append(c.Fragments, llm.RequestFragment{ID: "photo", Role: llm.InspectionRoleUser, Authorship: llm.FragmentAuthorshipAccount, MaterialRole: "attached-photo", SourceRefs: refs, Activation: "catalog question requires an owned private photo"})
	}
	return &c
}
