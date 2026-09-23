package generation

import _ "embed"

//go:embed schemas/observations.schema.json
var observationsSchema []byte

// The video schema is the photo one plus the two fields only a clip can carry, both REQUIRED
// so a model that heard nothing answers with an empty string rather than omitting the key
// (VIDEO-9).
//
//go:embed schemas/video_observations.schema.json
var videoObservationsSchema []byte

//go:embed schemas/post_content.schema.json
var postContentSchema []byte

// The write answer is the post content plus `nouns` (GEN-55). A revision keeps the stored
// nouns, so only the write pass asks for this one.
//
//go:embed schemas/write_answer.schema.json
var writeAnswerSchema []byte

// The write answer with replacement candidates beside the nouns (GEN-48), asked for only when
// the run froze 분야 phrases. Its surface enum is strings alone: a numeric enum makes Gemini
// answer an empty object for the whole schema.
//
//go:embed schemas/write_answer_replacements.schema.json
var writeAnswerReplacementsSchema []byte

func ObservationsSchema() []byte      { return append([]byte(nil), observationsSchema...) }
func VideoObservationsSchema() []byte { return append([]byte(nil), videoObservationsSchema...) }
func PostContentSchema() []byte       { return append([]byte(nil), postContentSchema...) }
func WriteAnswerSchema() []byte       { return append([]byte(nil), writeAnswerSchema...) }
func WriteAnswerReplacementsSchema() []byte {
	return append([]byte(nil), writeAnswerReplacementsSchema...)
}
