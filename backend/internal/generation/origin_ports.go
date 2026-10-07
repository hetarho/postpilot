package generation

import (
	"context"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/post"
)

// OriginPostCompletion is one complete canonical result and its independent
// annotations. A missing or invalid Origins never licenses rejecting valid content.
// Publication assigns the result revision/hash atomically in the post context.
type OriginPostCompletion struct {
	Content                 PostContent
	Language                Language
	Annotations             *WriteAnnotations
	Origins                 *post.OriginReview
	ExpectedContentRevision int64
}

// OriginPostPublisher is the completion behavior needed by origin-aware writing.
// It does not extend Posts: existing generation consumers retain their contract.
type OriginPostPublisher interface {
	PublishGeneratedResult(context.Context, string, string, OriginPostCompletion) (post.OriginResultIdentity, error)
}

// RequestInspectionCapture persists only the effective product projection, after
// execution has resolved model conditions. Its implementation owns deletion fences.
type RequestInspectionCapture interface {
	CapturePostRequest(context.Context, string, string, post.OriginResultIdentity, llm.RequestInspection) error
}
