package template

import (
	"github.com/postpilot/backend/internal/llm"
	"strings"
)

func templateRequestDescriptor(mode string) llm.RequestComposition {
	return llm.RequestComposition{Stage: "template-request", Mode: mode, PromptVersion: "template-request-v1", SchemaVersion: "template-request-answer-v1", Composer: "template.Service.RunRequest/requestSystem/requestMessage/correctionMessage", Parser: "template.Service.checkAnswer", Consumer: "private legacy request result; explicit draft application", Activation: "admitted-only frozen legacy template request; new helper admission retired; correction only for permitted grammar/field failure", SourceFiles: []string{"internal/template/request.go", "internal/template/request_prompts.go", "internal/template/guide.go"}, Output: llm.OutputContractInspection{Name: "template-request-answer", Version: "template-request-answer-v1", Schema: string(RequestAnswerSchema())}}
}
func RequestCompositions() []llm.RequestComposition {
	// Explicit synthetic ceilings describe a fixture, never current deployment settings.
	service := &Service{limits: Limits{NameMaxChars: 100, DescriptionMaxChars: 200, BodyMaxChars: 5000, TitleAreaMaxChars: 500, MaxPerAccount: 20, PhotoRowMax: 3, AskLabelMaxChars: 100, AskMaxPerBody: 20, TargetLengthMin: 100, TargetLengthMax: 10000, TagCountMin: 1, TagCountMax: 10}}
	in := requestInput{Language: LanguageKorean, Text: "Synthetic layout direction", Draft: Draft{Name: "Synthetic draft", Body: "<write>Synthetic topic</write>"}, Sample: &Sample{Title: "Synthetic sample", Text: "Synthetic structural reference"}}
	system, err := service.requestSystem(in.Language)
	if err != nil {
		panic(err)
	}
	base := []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart(requestMessage(in))}}}
	correction := append(append([]llm.Message(nil), base...), llm.Message{Role: llm.RoleAssistant, Parts: []llm.Part{llm.TextPart("Synthetic invalid candidate")}}, llm.Message{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart(correctionMessage(in.Language, ErrNameRequired))}})
	return []llm.RequestComposition{*templateRequestComposition(in, system, base), *templateRequestComposition(in, system, correction)}
}

func templateRequestComposition(in requestInput, system string, messages []llm.Message) *llm.RequestComposition {
	mode := "request"
	if len(messages) > 1 {
		mode = "correction"
	}
	c := templateRequestDescriptor(mode)
	c.Fragments = []llm.RequestFragment{{ID: "format-and-response-contract", Role: llm.InspectionRoleSystem, Authorship: llm.FragmentAuthorshipCode, MaterialRole: "template-grammar-and-output-contract", Text: system, SourceFiles: []string{"internal/template/request_prompts.go", "internal/template/guide.go"}}}
	words := requestCopy[in.Language]
	appendFragment := func(id string, author llm.FragmentAuthorship, material, text string) {
		c.Fragments = append(c.Fragments, llm.RequestFragment{ID: id, Role: llm.InspectionRoleUser, Authorship: author, MaterialRole: material, Text: text, SourceFiles: []string{"internal/template/request_prompts.go"}})
	}
	field := func(id, prefix, value, material string) {
		appendFragment(id+"-heading", llm.FragmentAuthorshipCode, "template-input-envelope", prefix)
		if strings.TrimSpace(value) == "" {
			appendFragment(id, llm.FragmentAuthorshipCode, "empty-product-placeholder", words.empty)
		} else {
			appendFragment(id, llm.FragmentAuthorshipAccount, material, value)
		}
	}
	appendFragment("request-heading", llm.FragmentAuthorshipCode, "template-input-envelope", words.request+"\n")
	if in.Text == "" {
		appendFragment("owner-request", llm.FragmentAuthorshipCode, "empty-product-placeholder", words.none)
	} else {
		appendFragment("owner-request", llm.FragmentAuthorshipAccount, "explicit-template-direction", in.Text)
	}
	field("draft-name", "\n\n"+words.draft+"\n"+words.name+": ", in.Draft.Name, "unpublished-template-draft")
	field("draft-description", "\n"+words.description+": ", in.Draft.Description, "unpublished-template-draft")
	field("draft-title", "\n"+words.titleArea+":\n", in.Draft.TitleArea, "unpublished-template-draft")
	field("draft-body", "\n"+words.body+":\n", in.Draft.Body, "unpublished-template-draft")
	if in.Sample != nil {
		field("sample-title", "\n\n"+words.sample+"\n"+words.sampleTitle+": ", in.Sample.Title, "structural-reference-not-event-evidence")
		appendFragment("sample-body-heading", llm.FragmentAuthorshipCode, "template-input-envelope", "\n")
		appendFragment("sample-body", llm.FragmentAuthorshipAccount, "structural-reference-not-event-evidence", in.Sample.Text)
	}

	if len(messages) > 1 {
		c.Fragments = append(c.Fragments, llm.RequestFragment{ID: "previous-ai-candidate", Role: llm.InspectionRoleAssistant, Authorship: llm.FragmentAuthorshipAccount, MaterialRole: "previous-ai-candidate-not-owner-fact", Text: messages[1].Parts[0].Text}, llm.RequestFragment{ID: "validation-feedback", Role: llm.InspectionRoleUser, Authorship: llm.FragmentAuthorshipCode, MaterialRole: "permitted-grammar-field-correction", Text: messages[2].Parts[0].Text, SourceFiles: []string{"internal/template/request_prompts.go"}, Activation: "previous response failed permitted check; not cancelled, filtered or truncated"})
	}
	return &c
}
