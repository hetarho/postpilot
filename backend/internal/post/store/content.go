package store

import (
	"encoding/json"
	"fmt"

	"github.com/postpilot/backend/internal/post"
)

// These wire structs are the database anti-corruption boundary. Domain types stay
// free of serialization tags while stored JSON keeps the canonical protojson names.
type blockJSON struct {
	Type    string   `json:"type,omitempty"`
	Content string   `json:"content,omitempty"`
	Level   int32    `json:"level,omitempty"`
	File    string   `json:"file,omitempty"`
	Alt     string   `json:"alt,omitempty"`
	Caption string   `json:"caption,omitempty"`
	Items   []string `json:"items,omitempty"`
}

type contentJSON struct {
	Title   string      `json:"title,omitempty"`
	Summary string      `json:"summary,omitempty"`
	Tags    []string    `json:"tags,omitempty"`
	Blocks  []blockJSON `json:"blocks,omitempty"`
}

type observationJSON struct {
	File          string   `json:"file,omitempty"`
	Scene         string   `json:"scene,omitempty"`
	Mood          string   `json:"mood,omitempty"`
	VisibleText   string   `json:"visibleText,omitempty"`
	Objects       []string `json:"objects,omitempty"`
	PeoplePresent bool     `json:"peoplePresent,omitempty"`
	// A row written before provenance existed decodes with this empty — unknown, not an
	// error. No migration: the column is a JSON document the post context owns.
	Model string `json:"model,omitempty"`
}

func marshalContent(content post.PostContent) (string, error) {
	wire := contentJSON{Title: content.Title, Summary: content.Summary, Tags: content.Tags}
	for _, block := range content.Blocks {
		wire.Blocks = append(wire.Blocks, blockJSON{
			Type: string(block.Type), Content: block.Content, Level: block.Level,
			File: block.File, Alt: block.Alt, Caption: block.Caption, Items: block.Items,
		})
	}
	data, err := json.Marshal(wire)
	return string(data), err
}

func unmarshalContent(data string) (*post.PostContent, error) {
	if data == "" {
		return nil, nil
	}
	var wire contentJSON
	if err := json.Unmarshal([]byte(data), &wire); err != nil {
		return nil, fmt.Errorf("decode content: %w", err)
	}
	content := &post.PostContent{Title: wire.Title, Summary: wire.Summary, Tags: wire.Tags}
	for _, block := range wire.Blocks {
		content.Blocks = append(content.Blocks, post.Block{
			Type: post.BlockType(block.Type), Content: block.Content, Level: block.Level,
			File: block.File, Alt: block.Alt, Caption: block.Caption, Items: block.Items,
		})
	}
	return content, nil
}

func marshalObservations(observations []post.Observation) (string, error) {
	wire := make([]observationJSON, 0, len(observations))
	for _, observation := range observations {
		wire = append(wire, observationJSON{
			File: observation.File, Scene: observation.Scene, Mood: observation.Mood,
			VisibleText: observation.VisibleText, Objects: observation.Objects,
			PeoplePresent: observation.PeoplePresent, Model: observation.Model,
		})
	}
	data, err := json.Marshal(wire)
	return string(data), err
}

func unmarshalObservations(data string) ([]post.Observation, error) {
	if data == "" {
		return nil, nil
	}
	var wire []observationJSON
	if err := json.Unmarshal([]byte(data), &wire); err != nil {
		return nil, fmt.Errorf("decode observations: %w", err)
	}
	out := make([]post.Observation, 0, len(wire))
	for _, observation := range wire {
		out = append(out, post.Observation{
			File: observation.File, Scene: observation.Scene, Mood: observation.Mood,
			VisibleText: observation.VisibleText, Objects: observation.Objects,
			PeoplePresent: observation.PeoplePresent, Model: observation.Model,
		})
	}
	return out, nil
}
