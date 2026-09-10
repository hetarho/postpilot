package rpc

import (
	"testing"

	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/provider"
)

// T092/MODEL-57: a model's level is per stage, so the wire carries one entry per stage that
// has one — and none for a stage the operator has not graded. The projection walks the
// STAGES, so a level for a stage the model does not serve can never reach the client.
func TestStageLevelProjection(t *testing.T) {
	wire := toProtoModel(provider.CatalogModel{Info: llm.ModelInfo{
		Stages: []string{llm.StageNameObserve, llm.StageNameWrite},
		Levels: map[string]string{
			llm.StageNameObserve: "value",
			// Registered to write but never graded: absent, not "".
			llm.StageNameAnalyze: "top",
		},
	}})

	if len(wire.GetLevels()) != 1 {
		t.Fatalf("levels = %v, want only the graded served stage", wire.GetLevels())
	}
	got := wire.GetLevels()[0]
	if got.GetStage() != postpilotv1.Stage_STAGE_OBSERVE || got.GetLevel() != "value" {
		t.Fatalf("level = %v, want observe/value", got)
	}
	if len(wire.GetStages()) != 2 {
		t.Fatalf("stages = %v, the level projection must not drop a stage", wire.GetStages())
	}
}

// A model nobody has graded carries no levels at all, and is otherwise a normal entry —
// the level gates nothing (MODEL-58).
func TestStageLevelProjection_UnlevelledModelIsUnaffected(t *testing.T) {
	wire := toProtoModel(provider.CatalogModel{
		Info:       llm.ModelInfo{Stages: []string{llm.StageNameWrite}, Label: "x"},
		Affordable: true,
	})
	if len(wire.GetLevels()) != 0 {
		t.Fatalf("levels = %v, want none", wire.GetLevels())
	}
	if len(wire.GetStages()) != 1 || !wire.GetAffordable() {
		t.Fatalf("an unlevelled model lost something: %+v", wire)
	}
}
