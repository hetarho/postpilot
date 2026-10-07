package memory

import "github.com/postpilot/backend/internal/llm"

func memoryDescriptor() llm.RequestComposition {
	return llm.RequestComposition{Stage: "memory-extraction", Mode: "propose", PromptVersion: "memory-extraction-v2", SchemaVersion: "memory-candidates-v1", Composer: "memory.Service.Extract/extractionInput", Parser: "memory.Service.parseCandidates", Consumer: "private text/kind/tags proposals; only explicit owner checkbox approval creates stored memories", Activation: "explicit admitted extraction over JSON-fenced frozen post title/memo/canonical prose; no automatic approval or save", SourceFiles: []string{"internal/memory/extraction.go", "internal/memory/request_composition.go", "internal/memory/schemas.go"}, Output: llm.OutputContractInspection{Name: "memory-candidates", Version: "memory-candidates-v1", Schema: string(MemoryCandidatesSchema())}}
}
func RequestCompositions() []llm.RequestComposition {
	return []llm.RequestComposition{*memoryComposition(ExtractionSource{PostSlug: "synthetic-post", Title: "Synthetic title", Memo: "Synthetic owner memo", Body: "Synthetic canonical prose"})}
}
func memoryComposition(source ExtractionSource) *llm.RequestComposition {
	out := memoryDescriptor()
	out.Fragments = []llm.RequestFragment{
		{ID: "extraction-contract", Role: llm.InspectionRoleSystem, Authorship: llm.FragmentAuthorshipCode, MaterialRole: "memory-proposal-contract", Text: ExtractionPrompt, SourceFiles: []string{"internal/memory/extraction.go"}},
		{ID: "title-heading", Role: llm.InspectionRoleUser, Authorship: llm.FragmentAuthorshipCode, MaterialRole: "extraction-input-envelope", Text: extractionSourceEnvelope + `{"title":`, SourceFiles: []string{"internal/memory/extraction.go"}},
		{ID: "source-title", Role: llm.InspectionRoleUser, Authorship: llm.FragmentAuthorshipAccount, MaterialRole: "canonical-post-title-unconfirmed-origin", Text: extractionData(source.Title)},
		{ID: "memo-heading", Role: llm.InspectionRoleUser, Authorship: llm.FragmentAuthorshipCode, MaterialRole: "extraction-input-envelope", Text: `,"memo":`, SourceFiles: []string{"internal/memory/extraction.go"}},
		{ID: "source-memo", Role: llm.InspectionRoleUser, Authorship: llm.FragmentAuthorshipAccount, MaterialRole: "explicit-owner-material", Text: extractionData(source.Memo)},
		{ID: "body-heading", Role: llm.InspectionRoleUser, Authorship: llm.FragmentAuthorshipCode, MaterialRole: "extraction-input-envelope", Text: `,"body":`, SourceFiles: []string{"internal/memory/extraction.go"}},
		{ID: "source-content", Role: llm.InspectionRoleUser, Authorship: llm.FragmentAuthorshipAccount, MaterialRole: "canonical-post-prose-unconfirmed-origin", Text: extractionData(source.Body)},
		{ID: "source-end", Role: llm.InspectionRoleUser, Authorship: llm.FragmentAuthorshipCode, MaterialRole: "extraction-input-envelope", Text: `}`, SourceFiles: []string{"internal/memory/extraction.go"}},
	}
	return &out
}
