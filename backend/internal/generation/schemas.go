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

func ObservationsSchema() []byte      { return append([]byte(nil), observationsSchema...) }
func VideoObservationsSchema() []byte { return append([]byte(nil), videoObservationsSchema...) }
func PostContentSchema() []byte       { return append([]byte(nil), postContentSchema...) }
