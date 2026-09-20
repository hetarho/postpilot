package memory

import _ "embed"

//go:embed schemas/memory_candidates.schema.json
var memoryCandidatesSchema []byte

// MemoryCandidatesSchema is the response contract for the extract_memory completion,
// attached only when the resolved model declares structured output (mirrors
// generation/schemas.go and voice/schemas.go).
//
// The kinds are STRING enum values, never integers. A numeric enum anywhere in a response
// schema makes Gemini answer `{}` for the whole object — measured on this codebase, and the
// cause of a clip outage — so a kind that travelled as a number here would take every
// candidate with it.
func MemoryCandidatesSchema() []byte { return append([]byte(nil), memoryCandidatesSchema...) }
