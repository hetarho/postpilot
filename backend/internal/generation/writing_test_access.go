package generation

import (
	"context"
	"fmt"

	"github.com/postpilot/backend/internal/llm"
)

// CheckWritingTestAccess rechecks only calls that a new admission will issue.
// Completed work remains readable even if its former model is unavailable.
// This gate neither resolves new input nor replaces any frozen variant.
func (f *WritingTestFactory) CheckWritingTestAccess(ctx context.Context, snapshot WritingTestSnapshot, calls []WritingTestCall) error {
	common, variants, err := decodeWritingTestSnapshot(snapshot)
	if err != nil {
		return err
	}
	if err := checkWritingTestVersions(common); err != nil {
		return err
	}
	full, err := f.PlanWritingTest(snapshot)
	if err != nil {
		return err
	}
	type identity struct {
		Ref                llm.ModelRef
		Stage              string
		Prompt, Completion int
	}
	available := make(map[identity]int, len(full))
	for _, call := range full {
		available[identity{call.Ref, call.Stage, call.PromptTokens, call.CompletionTokens}] += call.Count
	}
	if len(calls) == 0 {
		return ErrWritingTestMaterial
	}
	for _, call := range calls {
		key := identity{call.Ref, call.Stage, call.PromptTokens, call.CompletionTokens}
		if call.Count <= 0 || available[key] < call.Count {
			return ErrWritingTestMaterial
		}
		available[key] -= call.Count
		if err := ctx.Err(); err != nil {
			return err
		}
		switch call.Stage {
		case llm.StageNameWrite:
			model, err := f.prepareModel(ctx, common.Post.UserID, call.Stage, call.Ref, nil)
			if err != nil {
				return err
			}
			current, err := f.service.requireWriteModel(call.Ref.String())
			if err != nil {
				return err
			}
			for _, variant := range variants {
				if variant.WriteModel == call.Ref && variant.WriteStructuredOutput && (!model.Info.StructuredOutput || !current.StructuredOutput) {
					return llm.ErrUnsupported
				}
			}
		case llm.StageNameObserve:
			if observerWritingTest(common) {
				for _, variant := range variants {
					if variant.ObserveModel == call.Ref {
						if err := f.checkWritingTestObserver(ctx, common.Post.UserID, call.Ref, variant.Snapshot.Post.Images, variant.ObserveStructuredOutput); err != nil {
							return err
						}
					}
				}
			} else {
				targets, _ := frozenObserveSelection(common.Post.Images, common.ObserveFiles, common.Observations)
				if err := f.checkWritingTestObserver(ctx, common.Post.UserID, call.Ref, targets, common.ObserveStructuredOutput); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("%w: unsupported call stage", ErrWritingTestMaterial)
		}
	}
	return nil
}
