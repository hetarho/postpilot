package voice

import (
	_ "embed"
	"encoding/json"
	"strconv"
)

//go:embed schemas/writing_voice_candidates.schema.json
var writingCandidateSchema []byte

func WritingCandidateSchema(counts ...int) []byte {
	count, err := requestedCandidateCount(counts)
	if err != nil {
		return nil
	}
	schema := append([]byte(nil), writingCandidateSchema...)
	if count == CandidateCount {
		return schema
	}
	var root, properties, candidates map[string]json.RawMessage
	if json.Unmarshal(schema, &root) != nil || json.Unmarshal(root["properties"], &properties) != nil || json.Unmarshal(properties["candidates"], &candidates) != nil {
		return nil
	}
	candidates["minItems"] = json.RawMessage(strconv.Itoa(count))
	candidates["maxItems"] = json.RawMessage(strconv.Itoa(count))
	raw, err := json.Marshal(candidates)
	if err != nil {
		return nil
	}
	properties["candidates"] = raw
	raw, err = json.Marshal(properties)
	if err != nil {
		return nil
	}
	root["properties"] = raw
	raw, err = json.Marshal(root)
	if err != nil {
		return nil
	}
	return raw
}
