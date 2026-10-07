package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/postpilot/backend/internal/authoring"
	"github.com/postpilot/backend/internal/clip"
	clipai "github.com/postpilot/backend/internal/clip/ai"
	clipapp "github.com/postpilot/backend/internal/clip/app"
	"github.com/postpilot/backend/internal/generation"
	"github.com/postpilot/backend/internal/guideline"
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
		generation.RequestCompositions(), authoring.RequestCompositions(authoringInventoryGuides()),
		voice.RequestCompositions(), memory.RequestCompositions(),
		template.RequestCompositions(), clipai.RequestCompositions(),
		spokenapp.RequestCompositions(), clipapp.RequestCompositions(),
	} {
		result = append(result, compositions...)
	}
	return result
}

// Synthetic ceilings are fixture values, not a claim about current deployment.
// Each owning composer remains the single source of grammar and field rules.
func authoringInventoryGuides() map[authoring.Kind]string {
	templateLimits := template.Limits{NameMaxChars: 100, DescriptionMaxChars: 200, BodyMaxChars: 5000, TitleAreaMaxChars: 500, MaxPerAccount: 20, PhotoRowMax: 3, AskLabelMaxChars: 100, AskMaxPerBody: 20, TargetLengthMin: 100, TargetLengthMax: 10000, TagCountMin: 1, TagCountMax: 10}
	guidelineLimits := guideline.Limits{TitleMaxChars: 100, TextMaxChars: 2000, MaxPerAccount: 20}
	return map[authoring.Kind]string{
		authoring.PostTemplate:   template.AuthoringGuide(templateLimits),
		authoring.VideoTemplate:  clipapp.AuthoringGuide(clip.DefaultLimits()),
		authoring.PostGuideline:  guideline.AuthoringGuide(guideline.KindPost, guidelineLimits),
		authoring.VideoGuideline: guideline.AuthoringGuide(guideline.KindClip, guidelineLimits),
		authoring.WritingVoice:   voice.WritingStyleAuthoringGuide(),
	}
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
