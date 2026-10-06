package voice

import _ "embed"

//go:embed schemas/writing_voice_candidates.schema.json
var writingCandidateSchema []byte

func WritingCandidateSchema() []byte { return append([]byte(nil), writingCandidateSchema...) }
