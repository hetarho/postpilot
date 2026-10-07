package memory

import "github.com/postpilot/backend/internal/llm"

func memoryDescriptor() llm.RequestComposition {
	return llm.RequestComposition{Stage: "memory-extraction", Mode: "propose", PromptVersion: "memory-extraction-v1", SchemaVersion: "memory-candidates-v1", Composer: "memory.Service.Extract/extractionInput", Parser: "memory.Service.parseCandidates", Consumer: "private extraction job candidates; explicit owner Create only", Activation: "explicit admitted extraction over frozen post title/memo/canonical prose; no automatic save", SourceFiles: []string{"internal/memory/extraction.go", "internal/memory/schemas.go"}, Output: llm.OutputContractInspection{Name: "memory-candidates", Version: "memory-candidates-v1", Schema: string(MemoryCandidatesSchema())}}
}
func RequestCompositions() []llm.RequestComposition {
	return []llm.RequestComposition{*memoryComposition(ExtractionSource{PostSlug: "synthetic-post", Title: "Synthetic title", Memo: "Synthetic owner memo", Body: "Synthetic canonical prose"})}
}
func memoryComposition(source ExtractionSource) *llm.RequestComposition {
	out := memoryDescriptor()
	out.Fragments = []llm.RequestFragment{
		{ID: "extraction-contract", Role: llm.InspectionRoleSystem, Authorship: llm.FragmentAuthorshipCode, MaterialRole: "memory-proposal-contract", Text: ExtractionPrompt, SourceFiles: []string{"internal/memory/extraction.go"}},
		{ID: "title-heading", Role: llm.InspectionRoleUser, Authorship: llm.FragmentAuthorshipCode, MaterialRole: "extraction-input-envelope", Text: "제목: ", SourceFiles: []string{"internal/memory/extraction.go"}},
		{ID: "source-title", Role: llm.InspectionRoleUser, Authorship: llm.FragmentAuthorshipAccount, MaterialRole: "frozen-post-title", Text: source.Title},
		{ID: "memo-heading", Role: llm.InspectionRoleUser, Authorship: llm.FragmentAuthorshipCode, MaterialRole: "extraction-input-envelope", Text: "\n메모: ", SourceFiles: []string{"internal/memory/extraction.go"}},
		{ID: "source-memo", Role: llm.InspectionRoleUser, Authorship: llm.FragmentAuthorshipAccount, MaterialRole: "explicit-owner-material", Text: source.Memo},
		{ID: "body-heading", Role: llm.InspectionRoleUser, Authorship: llm.FragmentAuthorshipCode, MaterialRole: "extraction-input-envelope", Text: "\n본문:\n", SourceFiles: []string{"internal/memory/extraction.go"}},
		{ID: "source-content", Role: llm.InspectionRoleUser, Authorship: llm.FragmentAuthorshipAccount, MaterialRole: "canonical-post-prose-unconfirmed-origin", Text: source.Body},
	}
	return &out
}
