package generation

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"slices"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/post"
)

type requestCaptureContextKey struct{}
type requestCaptureContext struct {
	run         post.RequestCaptureRun
	sequence    int
	completion  *post.RequestCaptureCompletion
	attachments []Image
}

func (s *Service) beginRequestCapture(ctx context.Context, jobID string, input PostInput) (context.Context, func()) {
	if s.inspection == nil || jobID == "" {
		return ctx, func() {}
	}
	state := &requestCaptureContext{run: post.RequestCaptureRun{JobID: jobID, UserID: input.UserID, PostSlug: input.Slug, InputRevision: input.InputRevision, ContentRevision: input.ContentRevision, SourceFingerprint: input.SourceFingerprint, PlanFingerprint: input.StorylineFingerprint}, attachments: append([]Image(nil), input.Images...)}
	ctx = context.WithValue(ctx, requestCaptureContextKey{}, state)
	return ctx, func() {
		if err := s.inspection.Captures.FinishPostRequestCapture(context.WithoutCancel(ctx), state.run, state.completion); err != nil {
			// No prompt, source material or storage error is ordinary log content.
			slog.WarnContext(ctx, "post request capture binding unavailable", "job_id", jobID)
		}
	}
}

func bindRequestCaptureResult(ctx context.Context, identity post.OriginResultIdentity) {
	if state, ok := ctx.Value(requestCaptureContextKey{}).(*requestCaptureContext); ok {
		if state.completion == nil {
			state.completion = &post.RequestCaptureCompletion{}
		}
		state.completion.Result = &identity
	}
}

func bindRequestCapturePlan(ctx context.Context, fingerprint string) {
	if state, ok := ctx.Value(requestCaptureContextKey{}).(*requestCaptureContext); ok {
		if state.completion == nil {
			state.completion = &post.RequestCaptureCompletion{}
		}
		state.completion.PlanFingerprint = &fingerprint
	}
}

func captureAttachments(ctx context.Context, shown []string) []Image {
	state, ok := ctx.Value(requestCaptureContextKey{}).(*requestCaptureContext)
	if !ok {
		return nil
	}
	set := nameSet(shown)
	var result []Image
	for _, image := range state.attachments {
		if _, included := set[image.Filename]; included {
			result = append(result, image)
		}
	}
	return result
}

func (s *Service) bindPublishedRequestCapturePlan(ctx context.Context, userID, slug string, expected Storyline) bool {
	if _, ok := ctx.Value(requestCaptureContextKey{}).(*requestCaptureContext); !ok {
		return true
	}
	current, err := s.posts.AttachedImages(context.WithoutCancel(ctx), userID, slug)
	if err == nil && current.Storyline != nil && !current.StorylineEditedByHand && reflect.DeepEqual(current.Storyline.Paragraphs, expected.Paragraphs) && reflect.DeepEqual(current.Storyline.MadeWith, expected.MadeWith) {
		bindRequestCapturePlan(ctx, current.StorylineFingerprint)
		return true
	}
	return false
}

// completePostRequest records only the actual Registry invocation witness. A
// missing witness remains unavailable; preparing the request here would invent
// history. Capture failures cannot discard canonical output or retry a paid call.
func (s *Service) completePostRequest(ctx context.Context, model llm.ModelRef, request llm.Request, attachments []Image) (llm.Response, error) {
	response, err := s.models.Complete(ctx, model, request)
	state, ok := ctx.Value(requestCaptureContextKey{}).(*requestCaptureContext)
	if !ok || s.inspection == nil {
		return response, err
	}
	state.sequence++
	stage, mode := "", ""
	if request.Composition != nil {
		stage, mode = request.Composition.Stage, request.Composition.Mode
	}
	witness := llm.UnavailableRequestInspection(stage, mode)
	witness.UnavailableReason = "No safe execution witness was retained for this attempted call."
	if response.Inspection != nil && response.Inspection.Status == llm.InspectionCaptured && response.Inspection.Validate() == nil {
		witness = *response.Inspection
	} else if fromError, found := llm.RequestInspectionFromError(err); found && fromError.Status == llm.InspectionCaptured && fromError.Validate() == nil {
		witness = fromError
	}
	call := post.RequestCaptureCall{ID: fmt.Sprintf("%s:%d", state.run.JobID, state.sequence), Sequence: state.sequence}
	for _, image := range attachments {
		if image.ID == "" {
			continue
		}
		kind := string(AttachmentPhoto)
		if image.Kind == AttachmentVideo {
			kind = string(AttachmentVideo)
		}
		call.AttachmentIDs = append(call.AttachmentIDs, image.ID)
		call.AttachmentKinds = append(call.AttachmentKinds, kind)
	}
	if captureErr := s.inspection.Captures.WritePostRequestCapture(context.WithoutCancel(ctx), state.run, call, witness); captureErr != nil {
		if state.completion == nil {
			state.completion = &post.RequestCaptureCompletion{}
		}
		if !slices.Contains(state.completion.UnavailableStages, stage) {
			state.completion.UnavailableStages = append(state.completion.UnavailableStages, stage)
		}
		state.completion.UnavailableReason = "capture_persistence_failed"
		slog.WarnContext(ctx, "post request capture unavailable", "job_id", state.run.JobID, "call_id", call.ID)
	}
	return response, err
}
