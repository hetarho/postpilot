// Package ai maps the provider-neutral LLM boundary to clip domain contracts.
package ai

import (
	_ "embed"
	"encoding/json"
)

//go:embed schemas/chunk.schema.json
var chunkSchema []byte

//go:embed schemas/plan.schema.json
var planSchema []byte

//go:embed schemas/composition-plan.schema.json
var compositionPlanSchema []byte

// Provider grammars receive the closed structural shape, not every domain
// bound. The full contracts still live in the prompts and are checked locally.
// Sending nested array/numeric bounds rejected otherwise valid video requests
// on Gemini; removing string-length bounds alone did not fix that rejection.
type outputShape struct {
	Type                 string                  `json:"type"`
	Properties           map[string]*outputShape `json:"properties,omitempty"`
	Required             []string                `json:"required,omitempty"`
	Items                *outputShape            `json:"items,omitempty"`
	Enum                 []json.RawMessage       `json:"enum,omitempty"`
	AdditionalProperties *bool                   `json:"additionalProperties,omitempty"`
}

func structuralSchema(contract []byte) []byte {
	var shape outputShape
	if err := json.Unmarshal(contract, &shape); err != nil {
		panic(err) // Embedded, code-owned contracts; never provider/user input.
	}
	out, err := json.Marshal(shape)
	if err != nil {
		panic(err)
	}
	return out
}

var chunkOutputSchema = structuralSchema(chunkSchema)
var planOutputSchema = structuralSchema(planSchema)

func ChunkSchema() []byte { return append([]byte(nil), chunkOutputSchema...) }
func PlanSchema() []byte  { return append([]byte(nil), planOutputSchema...) }

var compositionPlanOutputSchema = structuralSchema(compositionPlanSchema)

func CompositionPlanSchema() []byte { return append([]byte(nil), compositionPlanOutputSchema...) }
