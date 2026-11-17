package app

import (
	"context"
	"errors"
	"testing"

	"github.com/postpilot/backend/internal/experiment"
	"github.com/postpilot/backend/internal/provider"
)

// A photo observation test produced no post: applying its output is refused before any
// target is touched, while its champion is adopted as the observe stage's model.
func TestObservationTestOffersModelAdoptionAndNoOutputApplication(t *testing.T) {
	raw, err := experiment.EncodeTestOutput(experiment.TestOutput{Observations: []experiment.TestOutputObservation{{File: "one.jpg", Scene: "scene"}}})
	if err != nil {
		t.Fatal(err)
	}
	p, store, snap, target, in := publicationFixture(t, experiment.FactorModel)
	store.test.Output, store.test.ModelStage, store.test.Candidates[0].Output = experiment.OutputObservation, experiment.StageObserve, raw
	snap.description.ModelStage = string(experiment.StageObserve)
	request := experiment.OutputApplication{TestMutation: in.TestMutation, WinnerID: in.WinnerID, InputRevision: 6, ContentRevision: 7}
	if _, _, err := p.ApplyOutput(context.Background(), request); !errors.Is(err, experiment.ErrTestOutputIncompatible) || target.calls != 0 {
		t.Fatalf("observation output applied err=%v calls=%d", err, target.calls)
	}
	if _, _, err := p.SaveWinner(context.Background(), in); err != nil {
		t.Fatalf("adopt observe champion: %v", err)
	}
	if target.model.Stage != provider.Stage(experiment.StageObserve) || target.model.Ref.ModelID != "winner-observer" {
		t.Fatalf("adoption=%+v", target.model)
	}
}
