package post

import (
	"context"

	"github.com/postpilot/backend/internal/llm"
)

// OriginReviewReader checks ownership and reads only evidence tied to the current
// result. No capture returns nil; readers must not infer history from current inputs.
type OriginReviewReader interface {
	ReadOriginReview(context.Context, string, string, OriginResultIdentity) (*OriginReview, error)
}

// RequestInspectionReader is a read-only owner-scoped product view. Missing or
// purged historical capture returns InspectionUnavailable without restoration.
type RequestInspectionReader interface {
	ReadPostRequestInspection(context.Context, string, string, string, llm.InspectionStatus) (llm.RequestInspection, error)
}

// PostRequestCaptureReader returns every available call of the selected current
// stage in call order. Missing/private-purged history is an empty selection.
type PostRequestCaptureReader interface {
	ReadPostRequestCaptures(context.Context, string, string, string) ([]llm.RequestInspection, error)
}
