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
var legacyFlowSchema []byte

//go:embed schemas/narration.schema.json
var legacyNarrationSchema []byte

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
// document without the storyline and the intro/outro words that come with it, which a
// revision never writes (CLIP-131, CLIP-178, CLIP-188).
// Legacy responses may echo computed duration and unused caption IDs/cuts. New
// requests omit them; the local parser still admits their original typed shape.
var flowSchema = withoutProperty(legacyFlowSchema, "duration_ms")
var flowParseSchema = withoutRequirement(legacyFlowSchema, "duration_ms")
var revisionFlowSchema = withoutProperty(withoutProperty(flowSchema, "storyline"), "region_slots")
var revisionFlowParseSchema = withoutProperty(withoutProperty(flowParseSchema, "storyline"), "region_slots")
var narrationSchema = withoutNestedProperty(withoutProperty(legacyNarrationSchema, "cuts"), "id", "properties", "captions", "items")
var narrationParseSchema = withoutRequirementAt(legacyNarrationSchema, "id", "properties", "captions", "items")

// withoutProperty is a code-owned contract with one top-level property and its requirement
// removed.
func withoutProperty(contract []byte, key string) []byte {
	return withoutNestedProperty(contract, key)
}

func withoutNestedProperty(contract []byte, key string, path ...string) []byte {
	return changeSchema(contract, path, func(doc map[string]any) {
		delete(doc["properties"].(map[string]any), key)
		removeRequirement(doc, key)
	})
}

func withoutRequirement(contract []byte, key string) []byte {
	return withoutRequirementAt(contract, key)
}

func withoutRequirementAt(contract []byte, key string, path ...string) []byte {
	return changeSchema(contract, path, func(doc map[string]any) { removeRequirement(doc, key) })
}

func removeRequirement(doc map[string]any, key string) {
	required := []any{}
	for _, name := range doc["required"].([]any) {
		if name != key {
			required = append(required, name)
		}
	}
	doc["required"] = required
}

func changeSchema(contract []byte, path []string, change func(map[string]any)) []byte {
	var doc map[string]any
	if err := json.Unmarshal(contract, &doc); err != nil {
		panic(err)
	}
	target := doc
	for _, key := range path {
		target = target[key].(map[string]any)
	}
	change(target)
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
