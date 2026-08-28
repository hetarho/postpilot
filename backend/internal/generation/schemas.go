package generation

import _ "embed"

//go:embed schemas/observations.schema.json
var observationsSchema []byte

//go:embed schemas/post_content.schema.json
var postContentSchema []byte

func ObservationsSchema() []byte { return append([]byte(nil), observationsSchema...) }
func PostContentSchema() []byte  { return append([]byte(nil), postContentSchema...) }
