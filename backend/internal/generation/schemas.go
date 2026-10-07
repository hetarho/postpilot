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

// A run along the stored storyline answers the write's members without `storyline`: the post
// follows the one it already holds (GEN-70).
//
//go:embed schemas/write_along_storyline_answer.schema.json
var writeAlongStorylineAnswerSchema []byte

// A storyline job's answer is the storyline alone, in the write answer's paragraph shape
// (GEN-68, GEN-69).
//
//go:embed schemas/storyline_answer.schema.json
var storylineAnswerSchema []byte

// Exact pre-origin contracts remain available only to already frozen legacy
// protocol calls. Their byte hashes are part of paid checkpoint admission.
//
//go:embed schemas/legacy/observations.schema.json
var legacyObservationsSchema []byte

//go:embed schemas/legacy/video_observations.schema.json
var legacyVideoObservationsSchema []byte

//go:embed schemas/legacy/post_content.schema.json
var legacyPostContentSchema []byte

//go:embed schemas/legacy/write_answer.schema.json
var legacyWriteAnswerSchema []byte

//go:embed schemas/legacy/write_along_storyline_answer.schema.json
var legacyWriteAlongStorylineAnswerSchema []byte

//go:embed schemas/legacy/storyline_answer.schema.json
var legacyStorylineAnswerSchema []byte

func ObservationsSchema() []byte      { return append([]byte(nil), observationsSchema...) }
func VideoObservationsSchema() []byte { return append([]byte(nil), videoObservationsSchema...) }
func PostContentSchema() []byte       { return append([]byte(nil), postContentSchema...) }
func WriteAnswerSchema() []byte       { return append([]byte(nil), writeAnswerSchema...) }
func StorylineAnswerSchema() []byte   { return append([]byte(nil), storylineAnswerSchema...) }
func WriteAlongStorylineAnswerSchema() []byte {
	return append([]byte(nil), writeAlongStorylineAnswerSchema...)
}

func LegacyObservationsSchema() []byte { return append([]byte(nil), legacyObservationsSchema...) }
func LegacyVideoObservationsSchema() []byte {
	return append([]byte(nil), legacyVideoObservationsSchema...)
}
func LegacyPostContentSchema() []byte { return append([]byte(nil), legacyPostContentSchema...) }
func LegacyWriteAnswerSchema() []byte { return append([]byte(nil), legacyWriteAnswerSchema...) }
func LegacyWriteAlongStorylineAnswerSchema() []byte {
	return append([]byte(nil), legacyWriteAlongStorylineAnswerSchema...)
}
func LegacyStorylineAnswerSchema() []byte { return append([]byte(nil), legacyStorylineAnswerSchema...) }
