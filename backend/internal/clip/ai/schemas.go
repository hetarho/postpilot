// Package ai maps the provider-neutral LLM boundary to clip domain contracts.
package ai

import _ "embed"

//go:embed schemas/chunk.schema.json
var chunkSchema []byte

//go:embed schemas/plan.schema.json
var planSchema []byte

func ChunkSchema() []byte { return append([]byte(nil), chunkSchema...) }
func PlanSchema() []byte  { return append([]byte(nil), planSchema...) }
