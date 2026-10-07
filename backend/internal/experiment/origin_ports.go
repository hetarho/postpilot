package experiment

import (
	"context"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/post"
)

// OriginOutputApplication carries the winning result's evidence with the already
// established publication mutation, receipt and input/content revision fences.
type OriginOutputApplication struct {
	OutputApplication
	Origins *post.OriginReview
}

type OriginTestPostPublication interface {
	ApplyOriginTestResult(context.Context, OriginOutputApplication) (PublicationReceipt, error)
}

// TestRequestInspection reads private entrant evidence only after ownership,
// payload-retention and blind-comparison identity fences have been applied.
type TestRequestInspection interface {
	ReadTestRequestInspection(context.Context, string, string, string, string, llm.InspectionStatus) (llm.RequestInspection, error)
}
