package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/postpilot/backend/internal/authoring"
	clipai "github.com/postpilot/backend/internal/clip/ai"
	clipapp "github.com/postpilot/backend/internal/clip/app"
	"github.com/postpilot/backend/internal/generation"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/memory"
	"github.com/postpilot/backend/internal/template"
	"github.com/postpilot/backend/internal/voice"
	spokenapp "github.com/postpilot/backend/internal/voice/spoken/app"
)

// Each context supplies the same descriptors its actual request composers use.
// This aggregate performs no configuration, customer read, admission or dispatch.
func productRequestCompositions() []llm.RequestComposition {
	var result []llm.RequestComposition
	for _, compositions := range [][]llm.RequestComposition{
		generation.RequestCompositions(), authoring.RequestCompositions(),
		voice.RequestCompositions(), memory.RequestCompositions(),
		template.RequestCompositions(), clipai.RequestCompositions(),
		spokenapp.RequestCompositions(), clipapp.RequestCompositions(),
	} {
		result = append(result, compositions...)
	}
	return result
}

func runPromptInventory(args []string, output io.Writer) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: api prompt-inventory")
	}
	inventory := struct {
		Version      int                      `json:"version"`
		Kind         string                   `json:"kind"`
		Material     string                   `json:"material"`
		SourceRoot   string                   `json:"source_root"`
		Compositions []llm.RequestComposition `json:"compositions"`
	}{Version: 1, Kind: "code", Material: "synthetic", SourceRoot: "backend", Compositions: productRequestCompositions()}
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	return encoder.Encode(inventory)
}
