package generation

import (
	"context"
	"fmt"

	"github.com/postpilot/backend/internal/llm"
)

type writingTestRequestCaptureKey struct{}
type writingTestRequestCapture func(context.Context, llm.Request, []Image, llm.Response, error) error

// Test evidence belongs to the same private checkpoint and purge fence as the
// admitted test, never the source post's request store. Commit the actual adapter
// witness before parsing its answer so failures and interrupted replies remain
// honest without reconstructing or repeating a provider call.
func withWritingTestRequestCapture(ctx context.Context, checkpoint *WritingTestCheckpoint, save SaveWritingTestCheckpoint) context.Context {
	capture := writingTestRequestCapture(func(ctx context.Context, request llm.Request, images []Image, response llm.Response, callErr error) error {
		stage, mode := "", ""
		if request.Composition != nil {
			stage, mode = request.Composition.Stage, request.Composition.Mode
		}
		inspection := llm.UnavailableRequestInspection(stage, mode)
		inspection.UnavailableReason = "execution_witness_unavailable"
		if response.Inspection != nil && response.Inspection.Status == llm.InspectionCaptured && response.Inspection.Validate() == nil {
			inspection = llm.CloneRequestInspection(*response.Inspection)
		} else if value, ok := llm.RequestInspectionFromError(callErr); ok && value.Status == llm.InspectionCaptured && value.Validate() == nil {
			inspection = value
		}
		if inspection.Status == llm.InspectionCaptured {
			inspection.CallID = fmt.Sprintf("%s:%d:%d", checkpoint.SnapshotHash, checkpoint.Index, len(checkpoint.RequestInspections)+1)
			inspection.Attachments = nil
			for _, image := range images {
				if image.ID == "" {
					continue
				}
				kind := "photo"
				if image.Kind == AttachmentVideo {
					kind = "video"
				}
				inspection.Attachments = append(inspection.Attachments, llm.InspectionAttachment{ID: image.ID, Kind: kind})
			}
		}
		if err := inspection.Validate(); err != nil {
			return err
		}
		checkpoint.RequestInspections = append(checkpoint.RequestInspections, inspection)
		return save(context.WithoutCancel(ctx), cloneWritingTestCheckpoint(*checkpoint))
	})
	return context.WithValue(ctx, writingTestRequestCaptureKey{}, capture)
}
