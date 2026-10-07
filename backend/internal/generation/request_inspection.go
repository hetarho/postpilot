package generation

import (
	"context"
	"fmt"
	"strings"

	"github.com/postpilot/backend/internal/llm"
)

func postInspectionStage(stage string) (string, bool) {
	switch stage {
	case "observe", "post-observation":
		return "observe", true
	case "plan", "storyline", "post-storyline":
		return "plan", true
	case "write", "post-writing":
		return "write", true
	case "write-from-storyline":
		return "write-from-storyline", true
	case "revise", "post-revision":
		return "revise", true
	default:
		return "", false
	}
}

func inspectionUnavailable(stage, reason string) llm.RequestInspection {
	out := llm.UnavailableRequestInspection(stage, "")
	out.UnavailableReason = reason
	return out
}

// ReadPostRequestInspection never admits a job or calls a model. Captured reads
// are exclusively post-owned stored history; current/prepared views use current
// eligible selection, enqueue material reads and the actual execution assemblers.
func (s *Service) ReadPostRequestInspection(ctx context.Context, userID, slug, stage string, status llm.InspectionStatus) (llm.RequestInspection, error) {
	if s.inspection == nil {
		return inspectionUnavailable(stage, "Request inspection is not configured."), nil
	}
	if status == llm.InspectionCaptured {
		if normalized, valid := postInspectionStage(stage); valid {
			switch normalized {
			case "observe":
				stage = "post-observation"
			case "plan":
				stage = "post-storyline"
			case "write", "write-from-storyline":
				stage = "post-writing"
			case "revise":
				stage = "post-revision"
			}
		}
		return s.inspection.Captures.ReadPostRequestInspection(ctx, userID, slug, stage, status)
	}
	// Ownership is checked before interpreting stage or eligibility. This is the
	// source-only work read: it has no media presigning or mutation behavior.
	post, err := s.posts.AttachedImages(ctx, userID, slug)
	if err != nil {
		return llm.RequestInspection{}, err
	}
	selectedStage, valid := postInspectionStage(stage)
	if !valid || status != llm.InspectionCurrent && status != llm.InspectionPrepared {
		return llm.RequestInspection{}, fmt.Errorf("%w: unsupported post stage or view", llm.ErrInvalidInspection)
	}
	if post.Published && status == llm.InspectionPrepared {
		return inspectionUnavailable(stage, "The published post is locked for new generation."), nil
	}
	purpose := llm.StageNameWrite
	if selectedStage == "observe" {
		purpose = llm.StageNameObserve
	}
	model, found, err := s.inspection.Selections.ModelForInspection(ctx, userID, purpose)
	if err != nil {
		return llm.RequestInspection{}, err
	}
	if !found || !modelEnabled(s.models, model, purpose) {
		return inspectionUnavailable(stage, "No current eligible model is selected for this stage."), nil
	}
	info, _ := s.models.Resolve(model)
	post.OriginProtocolVersion = s.originProtocol
	post.WriteNativeEffort = info.ReasoningNativeEffort
	request, attachments, reason, err := s.preparePostInspection(ctx, post, selectedStage, status, model)
	if err != nil {
		// Material reads can return owner-safe domain errors. They are not captured
		// history, and an invalid preview must not mutate or repair its source.
		return inspectionUnavailable(stage, "Current stage material cannot be prepared."), nil
	}
	if reason != "" {
		return inspectionUnavailable(stage, reason), nil
	}
	out, err := s.inspection.Models.PreparePostRequest(ctx, userID, model, request)
	if err != nil {
		return inspectionUnavailable(stage, "The selected model cannot prepare the current stage conditions."), nil
	}
	if out.Status == llm.InspectionUnavailable {
		out.UnavailableReason = "The current composer has no safe prepared projection."
		return out, nil
	}
	out.Status = status
	out.Omissions = append(out.Omissions, llm.RequestOmission{ID: "execution-admission", Reason: "This unissued view uses the stored selection and configured plan eligibility. Live provider eligibility, credit admission and a hold are checked only when the owner starts work.", Activation: "current configuration or prepared preview", SourceFiles: []string{"internal/generation/request_inspection.go"}})
	for _, image := range attachments {
		if image.ID == "" {
			continue
		}
		kind := string(AttachmentPhoto)
		if image.Kind == AttachmentVideo {
			kind = string(AttachmentVideo)
		}
		out.Attachments = append(out.Attachments, llm.InspectionAttachment{ID: image.ID, Kind: kind})
	}
	return out, out.Validate()
}

func (s *Service) preparePostInspection(ctx context.Context, post PostInput, stage string, status llm.InspectionStatus, model llm.ModelRef) (llm.Request, []Image, string, error) {
	if stage == "observe" {
		return s.prepareObserveInspection(post, model)
	}
	language := post.TargetLanguage
	if stage == "revise" {
		if post.Content == nil {
			return llm.Request{}, nil, "Revision requires current generated content.", nil
		}
		if post.ContentLanguage == nil || !post.ContentLanguage.Valid() {
			return llm.Request{}, nil, "Revision requires a known current content language.", nil
		}
		language = *post.ContentLanguage
		if status == llm.InspectionPrepared {
			return llm.Request{}, nil, "A prepared revision requires an explicit owner edit instruction.", nil
		}
	}
	if !language.Valid() {
		return llm.Request{}, nil, "A supported content language is required for this stage.", nil
	}
	voiceID, err := activeVoice(post)
	if err != nil {
		return llm.Request{}, nil, "", err
	}
	if stage == "write-from-storyline" && (post.Storyline == nil || len(post.Storyline.Paragraphs) == 0) {
		return llm.Request{}, nil, "Writing from a storyline requires a current stored plan.", nil
	}
	if stage != "revise" {
		observations := attachedObservations(post.Images, post.Observations)
		needed := post.Images
		if stage == "write-from-storyline" {
			needed = heldAttachments(post.Images, post.Storyline.Paragraphs)
		}
		if len(heldObservations(needed, observations)) != len(needed) && status == llm.InspectionPrepared {
			return llm.Request{}, nil, "Some attachments require observation before this request can be prepared.", nil
		}
		post.Observations = observations
	}
	if stage == "plan" {
		material, err := s.freezeStorylineMaterial(ctx, post)
		if err != nil {
			return llm.Request{}, nil, "", err
		}
		photos, videos := AttachmentNames(post.Images)
		input := StorylinePromptInput{Language: language, Title: post.Title, Memo: post.Memo, Photos: photos, Videos: videos, Observations: post.Observations, Template: material.Template, DefaultGuidelines: material.DefaultGuidelines, StockGuidelines: material.StockGuidelines, Guidelines: material.Guidelines, Memories: material.Memories}
		request, _ := preparePlanRequest(input, s.originProtocol, s.budget.Storyline(post.WriteNativeEffort), originAttachmentIDs(post.Images), nil)
		request = s.prepareStorylineCall(request, model, post.WriteNativeEffort, s.originProtocol)
		addUnobservedInspectionOmission(&request, post)
		return request, post.Images, "", nil
	}
	profile, err := s.profileForTopic(ctx, post.UserID, voiceID, language, post.Title+" "+post.Memo, contentTags(post.Content))
	if err != nil {
		return llm.Request{}, nil, "", err
	}
	if stage == "revise" {
		brief, err := s.freezeTemplate(ctx, post, false)
		if err != nil {
			return llm.Request{}, nil, "", err
		}
		guidelines, err := s.freezeGuidelines(ctx, post, language, false)
		if err != nil {
			return llm.Request{}, nil, "", err
		}
		payload := revisionPayloadJSON{OriginProtocolVersion: s.originProtocol, ContentLanguage: language, Template: encodeTemplate(brief), DefaultGuidelines: guidelines.Defaults, StockGuidelines: encodeStockGuidelines(guidelines.Stock), Guidelines: guidelines.Owner, TagCount: resolveTagCount(post.TagCount), WriteNativeEffort: post.WriteNativeEffort}
		if s.originProtocol == OriginProtocolVersion {
			payload.CompletionTokens = s.budget.Revise(contentChars(post.Content)+OriginCompletionExtraChars, post.TargetLength, post.WriteNativeEffort)
		}
		request, _ := s.prepareRevisionRequest(post, profile, payload, model)
		request.Composition.Omissions = append(request.Composition.Omissions, llm.RequestOmission{ID: "revision-instruction", Reason: "Current configuration has no prospective owner edit instruction; a prepared revision is unavailable until one is supplied.", Activation: "current configuration only"})
		return request, post.Images, "", nil
	}
	material, err := s.freezeWriteMaterial(ctx, post)
	if err != nil {
		return llm.Request{}, nil, "", err
	}
	post = material.onto(post)
	if stage == "write-from-storyline" {
		post.FollowStoryline, post.FollowStorylineOrigins = cloneParagraphs(post.Storyline.Paragraphs), clonePlanOrigins(post.Storyline.Origins)
		post.Images = heldAttachments(post.Images, post.FollowStoryline)
		post.Observations = heldObservations(post.Images, post.Observations)
	}
	if s.originProtocol == OriginProtocolVersion {
		post.OriginCompletionTokens = s.budget.Write(OriginBudgetTarget(post.TargetLength), post.WriteNativeEffort)
	}
	request, _ := s.prepareWriteRequest(post, profile, post.Observations, model)
	addUnobservedInspectionOmission(&request, post)
	return request, post.Images, "", nil
}

func addUnobservedInspectionOmission(request *llm.Request, post PostInput) {
	if len(post.Observations) != len(post.Images) {
		request.Composition.Omissions = append(request.Composition.Omissions, llm.RequestOmission{ID: "pending-observation", Reason: "Current configuration includes attachments with no reusable observation. Future observation output is unknown and this configuration is not an executable prepared request.", Activation: "current configuration only"})
	}
}

// Media placeholders preserve capability/part-kind resolution without reading
// bytes or minting a signed link. Only the safe owning composition is projected.
func (s *Service) prepareObserveInspection(post PostInput, model llm.ModelRef) (llm.Request, []Image, string, error) {
	if len(post.Images) == 0 {
		return llm.Request{}, nil, "The post has no attachments to observe.", nil
	}
	if err := s.refuseVideoBlindObserveModel(post.Images, model); err != nil {
		return llm.Request{}, nil, "The selected observation model cannot inspect the attached media kinds.", nil
	}
	photos, videos := photosOf(post.Images), videosOf(post.Images)
	var request llm.Request
	var attachments []Image
	if len(photos) > 0 {
		attachments = photos[:min(s.batchSize, len(photos))]
		parts := make([]llm.Part, 0, 2*len(attachments)+1)
		var filenames []string
		for _, image := range attachments {
			parts = append(parts, llm.TextPart("file: "+image.Filename), llm.ImagePart([]byte{0}, "image/jpeg"))
			filenames = append(filenames, image.Filename)
		}
		parts = append(parts, llm.TextPart("files: "+strings.Join(filenames, ", ")))
		request, _ = s.preparePhotoObservationRequest(parts, attachments, model, s.originProtocol)
	} else {
		attachments = videos[:1]
		request, _ = s.prepareVideoObservationRequest(videos[0], "https://request-preview.invalid/media-placeholder", model, s.originProtocol)
	}
	if remaining := s.observeCalls(post.Images) - 1; remaining > 0 {
		request.Composition.Omissions = append(request.Composition.Omissions, llm.RequestOmission{ID: "other-observation-calls", Reason: fmt.Sprintf("This is the first prospective observation call. %d other batch/video calls have separate requests and are excluded from this preview.", remaining), Activation: "first prospective batch or video"})
	}
	return request, attachments, "", nil
}
