package generation

import (
	"context"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/post"
)

// RequestCaptureStore owns the post's private execution witnesses and their
// source/result fences. Generation neither stores accounting nor rebuilds history.
type RequestCaptureStore interface {
	WritePostRequestCapture(context.Context, post.RequestCaptureRun, post.RequestCaptureCall, llm.RequestInspection) error
	FinishPostRequestCapture(context.Context, post.RequestCaptureRun, *post.RequestCaptureCompletion) error
	ReadPostRequestInspection(context.Context, string, string, string, llm.InspectionStatus) (llm.RequestInspection, error)
}

// RequestInspectionModels shares Registry's pure effective-option resolution.
// It has no dispatch method, media reader or queue admission behavior.
type RequestInspectionModels interface {
	PreparePostRequest(context.Context, string, llm.ModelRef, llm.Request) (llm.RequestInspection, error)
}

type RequestInspectionSelections interface {
	ModelForInspection(context.Context, string, string) (llm.ModelRef, bool, error)
}

type RequestInspectionDependencies struct {
	Captures   RequestCaptureStore
	Models     RequestInspectionModels
	Selections RequestInspectionSelections
}

// NewInspectedService leaves the admitted legacy service usable and requires all
// production inspection collaborators explicitly, like NewOriginService.
func NewInspectedService(service *Service, deps RequestInspectionDependencies) *Service {
	if service == nil || deps.Captures == nil || deps.Models == nil || deps.Selections == nil {
		panic("generation: request inspection collaborators are required")
	}
	copy := *service
	copy.inspection = &deps
	return &copy
}
