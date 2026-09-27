// Package ai maps the provider-neutral LLM boundary to clip domain contracts.
package ai

import (
	"bytes"
	_ "embed"
	"encoding/json"
)

//go:embed schemas/chunk.schema.json
var chunkSchema []byte

//go:embed schemas/flow.schema.json
var flowSchema []byte

//go:embed schemas/narration.schema.json
var narrationSchema []byte

//go:embed schemas/storyline.schema.json
var storylineSchema []byte

func compactContract(value []byte) string {
	var out bytes.Buffer
	if err := json.Compact(&out, value); err != nil {
		panic(err)
	}
	return out.String()
}

// revisionFlowSchema is the flow contract a revision's flow rewrite answers: the same
// document without the storyline, which a revision never writes (CLIP-131, CLIP-178).
var revisionFlowSchema = withoutProperty(flowSchema, "storyline")

// withoutProperty is a code-owned contract with one top-level property and its requirement
// removed.
func withoutProperty(contract []byte, key string) []byte {
	var doc map[string]any
	if err := json.Unmarshal(contract, &doc); err != nil {
		panic(err)
	}
	delete(doc["properties"].(map[string]any), key)
	required := []any{}
	for _, name := range doc["required"].([]any) {
		if name != key {
			required = append(required, name)
		}
	}
	doc["required"] = required
	out, err := json.Marshal(doc)
	if err != nil {
		panic(err)
	}
	return out
}

var flowPromptSchema = compactContract(flowSchema)
var revisionFlowPromptSchema = compactContract(revisionFlowSchema)
var narrationPromptSchema = compactContract(narrationSchema)
var storylinePromptSchema = compactContract(storylineSchema)
var chunkPromptSchema = compactContract(chunkSchema)

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

// Google's schema subset accepts enum only on a string. A numeric enum does not
// merely lose its bound there: the object carrying it comes back EMPTY, which
// reaches us as an output_shape refusal the model cannot correct. The allowed
// set stays in the prompt and in the local validator, where it is enforced.
func (s *outputShape) dropUnsupportedEnums() {
	if s == nil {
		return
	}
	if s.Type != "string" {
		s.Enum = nil
	}
	for _, child := range s.Properties {
		child.dropUnsupportedEnums()
	}
	s.Items.dropUnsupportedEnums()
}

func structuralSchema(contract []byte) []byte {
	var shape outputShape
	if err := json.Unmarshal(contract, &shape); err != nil {
		panic(err) // Embedded, code-owned contracts; never provider/user input.
	}
	shape.dropUnsupportedEnums()
	out, err := json.Marshal(shape)
	if err != nil {
		panic(err)
	}
	return out
}

var chunkOutputSchema = structuralSchema(chunkSchema)

func ChunkSchema() []byte { return append([]byte(nil), chunkOutputSchema...) }

var flowOutputSchema = structuralSchema(flowSchema)

func FlowSchema() []byte { return append([]byte(nil), flowOutputSchema...) }

var revisionFlowOutputSchema = structuralSchema(revisionFlowSchema)

// RevisionFlowSchema is the structural shape of a revision's flow answer: no storyline.
func RevisionFlowSchema() []byte { return append([]byte(nil), revisionFlowOutputSchema...) }

var narrationOutputSchema = structuralSchema(narrationSchema)

func NarrationSchema() []byte { return append([]byte(nil), narrationOutputSchema...) }

var storylineOutputSchema = structuralSchema(storylineSchema)

// StorylineSchema is the structural shape of the storyline call's answer (CLIP-177).
func StorylineSchema() []byte { return append([]byte(nil), storylineOutputSchema...) }
