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

// The write answer is the post content plus `nouns` (GEN-55) and `storyline` (GEN-67). A
// revision keeps both as stored, so only the write pass asks for this one.
//
//go:embed schemas/write_answer.schema.json
var writeAnswerSchema []byte

// A storyline job's answer is the storyline alone, in the write answer's paragraph shape
// (GEN-68, GEN-69).
//
//go:embed schemas/storyline_answer.schema.json
var storylineAnswerSchema []byte

func ObservationsSchema() []byte      { return append([]byte(nil), observationsSchema...) }
func VideoObservationsSchema() []byte { return append([]byte(nil), videoObservationsSchema...) }
func PostContentSchema() []byte       { return append([]byte(nil), postContentSchema...) }
func WriteAnswerSchema() []byte       { return append([]byte(nil), writeAnswerSchema...) }
func StorylineAnswerSchema() []byte   { return append([]byte(nil), storylineAnswerSchema...) }
