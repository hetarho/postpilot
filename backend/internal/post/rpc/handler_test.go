package rpc

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/auth"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/post"
)

// Every domain error the service can raise must have a code. An unmapped one becomes
// Internal, which tells the client to retry something that will never succeed.
func TestToConnectErrorMapsEveryDomainError(t *testing.T) {
	cases := []struct {
		name   string
		op     string
		err    error
		code   connect.Code
		reason string
	}{
		{"post missing", "get post", post.ErrNotFound, connect.CodeNotFound, "POST_NOT_FOUND"},
		{"upload missing", "confirm upload", post.ErrNotFound, connect.CodeNotFound, "UPLOAD_NOT_FOUND"},
		{"forbidden", "get post", post.ErrForbidden, connect.CodePermissionDenied, "POST_FORBIDDEN"},
		{"filename", "create upload", post.ErrDuplicateFilename, connect.CodeAlreadyExists, "POST_FILENAME_TAKEN"},
		{"object", "confirm upload", post.ErrObjectMissing, connect.CodeFailedPrecondition, "UPLOAD_OBJECT_MISSING"},
		{"image", "confirm upload", post.ErrInvalidImage, connect.CodeInvalidArgument, "UPLOAD_INVALID"},
		{"busy", "save draft", post.ErrPostBusy, connect.CodeFailedPrecondition, "POST_BUSY"},
		{"no storyline", "save draft", post.ErrStorylineMissing, connect.CodeFailedPrecondition, "POST_STORYLINE_MISSING"},
		{"storyline edit", "save draft", post.ErrStorylineInvalid, connect.CodeInvalidArgument, "POST_STORYLINE_INVALID"},
		{"stale", "save post content", post.ErrStaleContentRevision, connect.CodeAborted, "POST_CONTENT_STALE"},
		{"baseline", "finalize post", post.ErrNoMachineBaseline, connect.CodeFailedPrecondition, "POST_MACHINE_BASELINE_REQUIRED"},
		{"not finalized", "publish", post.ErrPostNotFinalized, connect.CodeFailedPrecondition, "POST_NOT_FINALIZED"},
		{"invalid content", "save post content", &post.InvalidContentError{Reason: "private authored content"}, connect.CodeInvalidArgument, "POST_CONTENT_INVALID"},
		{"tag count", "save post generation options", post.ErrInvalidTagCount, connect.CodeInvalidArgument, "POST_TAG_COUNT_INVALID"},
		{"voice required", "save draft", post.ErrVoiceRequired, connect.CodeInvalidArgument, "VOICE_REQUIRED"},
		{"voice missing", "save draft", post.ErrVoiceNotFound, connect.CodeNotFound, "VOICE_NOT_FOUND"},
		{"template missing", "save draft", post.ErrTemplateNotFound, connect.CodeNotFound, "PURPOSE_NOT_FOUND"},
		{"voice deleted", "save draft", post.ErrVoiceDeleted, connect.CodeFailedPrecondition, "VOICE_DELETED"},
		{"language", "save draft", post.ErrLanguageRequired, connect.CodeInvalidArgument, "POST_TARGET_LANGUAGE_REQUIRED"},
		{"video ceiling", "create upload", post.ErrTooManyVideos, connect.CodeFailedPrecondition, "POST_VIDEO_LIMIT"},
		{"video container", "create upload", post.ErrUnsupportedVideo, connect.CodeInvalidArgument, "UPLOAD_VIDEO_UNSUPPORTED"},
		{"video", "confirm upload", post.ErrInvalidVideo, connect.CodeInvalidArgument, "UPLOAD_VIDEO_INVALID"},
		{"answer invalid", "save draft", post.ErrTemplateAnswerInvalid, connect.CodeInvalidArgument, "POST_TEMPLATE_ANSWER_INVALID"},
		{"published", "save post content", post.ErrPostPublished, connect.CodeFailedPrecondition, "POST_PUBLISHED_LOCKED"},
		{"published url", "save published url", post.ErrPublishedURLInvalid, connect.CodeInvalidArgument, "POST_PUBLISHED_URL_INVALID"},
		{"field missing", "save draft", post.ErrFieldNotFound, connect.CodeNotFound, "POST_FIELD_NOT_FOUND"},
		{"quality rule", "save post generation options", post.ErrQualityRuleInvalid, connect.CodeInvalidArgument, "POST_QUALITY_RULE_INVALID"},
		{"list request", "list posts", post.ErrInvalidListRequest, connect.CodeInvalidArgument, "POST_LIST_REQUEST_INVALID"},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			// Wrapped, as the service actually returns them, and bare, as a handler raises
			// one before the service is called.
			for shape, err := range map[string]error{
				"wrapped": errors.Join(errors.New("private context"), test.err),
				"bare":    test.err,
			} {
				got := toConnectError(test.op, err)
				if connect.CodeOf(got) != test.code {
					t.Errorf("%s: code = %v, want %v", shape, connect.CodeOf(got), test.code)
				}
				detail := postAppErrorDetail(t, got)
				if detail.GetReason() != test.reason || !reflect.DeepEqual(detail.GetParams(), map[string]string(nil)) {
					t.Errorf("%s: detail = %#v, want reason %q", shape, detail, test.reason)
				}
				if strings.Contains(got.Error(), "private") {
					t.Errorf("%s: private detail leaked: %v", shape, got)
				}
			}
		})
	}
}

// The address is saved only for the session's own account, and the request gives a caller
// nowhere to claim one. What a signed-in save does is pinned by the service tests.
func TestSavePostPublishedUrlRequiresASession(t *testing.T) {
	handler := NewHandler(nil)
	request := connect.NewRequest(&postpilotv1.SavePostPublishedUrlRequest{Slug: "20260924-jeju", Url: "https://blog.naver.com/alice/1"})

	_, err := handler.SavePostPublishedUrl(context.Background(), request)
	if connect.CodeOf(err) != connect.CodeUnauthenticated || postAppErrorDetail(t, err).GetReason() != "AUTH_REQUIRED" {
		t.Fatalf("anonymous = %v", err)
	}

	fields := (&postpilotv1.SavePostPublishedUrlRequest{}).ProtoReflect().Descriptor().Fields()
	for i := 0; i < fields.Len(); i++ {
		switch name := string(fields.Get(i).Name()); name {
		case "user_id", "account_id", "owner_id":
			t.Fatalf("SavePostPublishedUrlRequest carries %s", name)
		}
	}
}

// A published post carries its address and when it was recorded, and it can no longer be
// finalized: its way back is clearing the address (POST-75).
func TestToProtoPostCarriesThePublication(t *testing.T) {
	at := time.Date(2026, 9, 24, 3, 4, 5, 0, time.UTC)
	content := &post.PostContent{Title: "제주 3일"}
	published := toProtoPost(post.Post{
		Slug: "20260924-jeju", Status: post.StatusPublished, Content: content,
		PublishedURL: "https://blog.naver.com/alice/223000000000", PublishedAt: &at,
	})
	if published.GetPublishedUrl() != "https://blog.naver.com/alice/223000000000" || published.GetPublishedAt() != "2026-09-24T03:04:05Z" {
		t.Fatalf("publication = %q at %q", published.GetPublishedUrl(), published.GetPublishedAt())
	}
	if published.GetCanFinalize() {
		t.Fatal("a published post offers finalization")
	}
	finalized := toProtoPost(post.Post{Slug: "20260924-jeju", Status: post.StatusFinalized, Content: content})
	if finalized.GetPublishedUrl() != "" || finalized.GetPublishedAt() != "" || !finalized.GetCanFinalize() {
		t.Fatalf("an unpublished post = %q at %q, can finalize %v", finalized.GetPublishedUrl(), finalized.GetPublishedAt(), finalized.GetCanFinalize())
	}
}

// POST-99: the storyline reaches the wire with what was added and what was taken out, read
// against the post's attachments as they stand; a post without one carries none.
func TestToProtoPostCarriesTheStoryline(t *testing.T) {
	storied := toProtoPost(post.Post{
		Slug:   "20260928-seongsu",
		Images: []post.Image{{Filename: "a.jpg"}, {Filename: "b.jpg"}, {Filename: "new.jpg"}},
		Videos: []post.Video{{Filename: "clip.mp4"}},
		Storyline: &post.Storyline{
			Paragraphs:   []post.StorylineParagraph{{Text: "가게 앞", Files: []string{"a.jpg", "clip.mp4"}}, {Text: "마무리"}},
			EditedByHand: true,
			MadeWith:     []string{"a.jpg", "b.jpg", "clip.mp4"},
		},
	})
	story := storied.GetStoryline()
	if story == nil || len(story.GetParagraphs()) != 2 || !story.GetEditedByHand() ||
		!reflect.DeepEqual(story.GetParagraphs()[0].GetFiles(), []string{"a.jpg", "clip.mp4"}) || story.GetParagraphs()[1].GetText() != "마무리" {
		t.Fatalf("storyline = %+v", story)
	}
	if !reflect.DeepEqual(story.GetAddedFiles(), []string{"new.jpg"}) || !reflect.DeepEqual(story.GetTakenOutFiles(), []string{"b.jpg"}) {
		t.Fatalf("added %v, taken out %v", story.GetAddedFiles(), story.GetTakenOutFiles())
	}
	if none := toProtoPost(post.Post{Slug: "20260928-plain"}); none.GetStoryline() != nil {
		t.Fatalf("a post without a storyline carries %+v", none.GetStoryline())
	}
}

// An unexpected failure must not put a SQL string or a bucket name on the wire.
func TestToConnectErrorHidesUnexpectedDetail(t *testing.T) {
	got := toConnectError("save draft", errors.New("no such table: posts (file /data/postpilot.db)"))

	if connect.CodeOf(got) != connect.CodeInternal {
		t.Errorf("code = %v, want internal", connect.CodeOf(got))
	}
	if got.Error() != "internal: save draft failed" {
		t.Errorf("message = %q, want it to carry no detail", got.Error())
	}
	if detail := postAppErrorDetail(t, got); detail.GetReason() != "UNKNOWN_FAILURE" || len(detail.GetParams()) != 0 {
		t.Errorf("detail = %#v", detail)
	}
}

func TestDraftLanguageAndContentDecodeFailuresAreStable(t *testing.T) {
	handler := NewHandler(nil)
	ctx := auth.WithUser(context.Background(), "alice")
	for name, test := range map[string]struct {
		language postpilotv1.ContentLanguage
		reason   string
	}{
		"missing": {postpilotv1.ContentLanguage_CONTENT_LANGUAGE_UNSPECIFIED, "POST_TARGET_LANGUAGE_REQUIRED"},
		"unknown": {postpilotv1.ContentLanguage(999), "POST_TARGET_LANGUAGE_UNSUPPORTED"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := handler.SavePostDraft(ctx, connect.NewRequest(&postpilotv1.SavePostDraftRequest{TargetLanguage: &test.language}))
			if connect.CodeOf(err) != connect.CodeInvalidArgument || postAppErrorDetail(t, err).GetReason() != test.reason {
				t.Fatalf("error = %v, detail = %#v", err, postAppErrorDetail(t, err))
			}
		})
	}

	_, err := handler.SavePostContent(ctx, connect.NewRequest(&postpilotv1.SavePostContentRequest{}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument || postAppErrorDetail(t, err).GetReason() != "POST_CONTENT_INVALID" {
		t.Fatalf("content error = %v", err)
	}
}

func TestActiveJobProjectsStructuredFailureWithoutDeprecatedRawError(t *testing.T) {
	params := map[string]string{"safe": "value"}
	mapped := toProtoActiveJob(&post.ActiveJob{Failure: &post.Failure{
		Reason: "MODEL_UNAVAILABLE", Params: params, TechnicalDetail: "provider detail",
	}})
	if mapped.GetFailure().GetReason() != "MODEL_UNAVAILABLE" ||
		mapped.GetFailure().GetParams()["safe"] != "value" ||
		mapped.GetFailure().GetTechnicalDetail() != "provider detail" || mapped.GetError() != "" {
		t.Fatalf("mapped failure = %+v", mapped)
	}
	params["safe"] = "mutated"
	if mapped.GetFailure().GetParams()["safe"] != "value" {
		t.Fatalf("proto params alias domain map: %#v", mapped.GetFailure().GetParams())
	}
}

func postAppErrorDetail(t *testing.T, err error) *postpilotv1.AppErrorDetail {
	t.Helper()
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		t.Fatalf("error type = %T", err)
	}
	if len(connectErr.Details()) != 1 {
		t.Fatalf("details = %d, want 1", len(connectErr.Details()))
	}
	value, valueErr := connectErr.Details()[0].Value()
	if valueErr != nil {
		t.Fatalf("decode detail: %v", valueErr)
	}
	detail, ok := value.(*postpilotv1.AppErrorDetail)
	if !ok {
		t.Fatalf("detail type = %T", value)
	}
	return detail
}

// The wire enum is the only thing that decides the kind, and its zero value is a photo —
// which is what every client shipped before videos existed sends.
func TestAttachmentKindFromProto(t *testing.T) {
	cases := map[postpilotv1.AttachmentKind]post.AttachmentKind{
		postpilotv1.AttachmentKind_ATTACHMENT_KIND_UNSPECIFIED: post.AttachmentPhoto,
		postpilotv1.AttachmentKind_ATTACHMENT_KIND_PHOTO:       post.AttachmentPhoto,
		postpilotv1.AttachmentKind_ATTACHMENT_KIND_VIDEO:       post.AttachmentVideo,
	}
	for wire, want := range cases {
		if got := attachmentKindFromProto(wire); got != want {
			t.Errorf("attachmentKindFromProto(%v) = %q, want %q", wire, got, want)
		}
	}
}

// A VIDEO block has to survive the round trip through the wire enum, or a saved post
// would come back as an unknown block type and fail its own validator.
func TestVideoBlockTypeRoundTrips(t *testing.T) {
	if got := toProtoBlockType(post.BlockVideo); got != postpilotv1.BlockType_VIDEO {
		t.Fatalf("toProtoBlockType = %v, want VIDEO", got)
	}
	if got := fromProtoBlockType(postpilotv1.BlockType_VIDEO); got != post.BlockVideo {
		t.Fatalf("fromProtoBlockType = %q, want %q", got, post.BlockVideo)
	}
}

// A video observation carries what a still frame cannot, and the projection has to take
// both fields along with it.
func TestObservationProjectionCarriesEventsAndSpeech(t *testing.T) {
	got := toProtoObservation(post.Observation{
		File: "clip.mp4", Scene: "바다", Events: []string{"파도가 친다"}, Speech: "좋다",
	})
	if !reflect.DeepEqual(got.GetEvents(), []string{"파도가 친다"}) || got.GetSpeech() != "좋다" {
		t.Errorf("observation = %+v", got)
	}
}

// A video projects its container type and duration; a photo has neither, and its message
// has no room for them.
func TestVideoProjection(t *testing.T) {
	got := toProtoVideo(post.Video{
		ID: "v1", Filename: "clip.mp4", Width: 1920, Height: 1080, Bytes: 42,
		ViewURL: "https://storage.example/clip", DurationMs: 5_000, ContentType: "video/mp4",
	})
	if got.GetId() != "v1" || got.GetFilename() != "clip.mp4" || got.GetDurationMs() != 5_000 ||
		got.GetContentType() != "video/mp4" || got.GetViewUrl() != "https://storage.example/clip" {
		t.Errorf("video = %+v", got)
	}
}

// The typed answer refusal carries the numbers the message needs, so it is asserted apart
// from the parameterless sentinels above.
func TestToConnectErrorCarriesTheAnswerBound(t *testing.T) {
	got := toConnectError("save draft", errors.Join(errors.New("private context"),
		&post.TemplateAnswerTooLongError{Field: "text", Chars: 501, Max: 500}))
	if connect.CodeOf(got) != connect.CodeInvalidArgument {
		t.Fatalf("code = %v", connect.CodeOf(got))
	}
	detail := postAppErrorDetail(t, got)
	want := map[string]string{"field": "text", "max": "500", "actual": "501"}
	if detail.GetReason() != "POST_TEMPLATE_ANSWER_TOO_LONG" || !reflect.DeepEqual(detail.GetParams(), want) {
		t.Errorf("detail = %#v, want %q with %v", detail, "POST_TEMPLATE_ANSWER_TOO_LONG", want)
	}
	if strings.Contains(got.Error(), "private") {
		t.Error("the wrapped context reached the client")
	}
}

// The request's answers travel inward as-is, and an empty list is "no answer in this save"
// rather than a clear (POST-62).
func TestTemplateAnswersCrossTheWireBothWays(t *testing.T) {
	if fromProtoTemplateAnswers(nil) != nil {
		t.Error("an absent list should carry nothing inward")
	}
	if got := fromProtoTemplateAnswers([]*postpilotv1.TemplateAnswer{}); got != nil {
		t.Errorf("an empty list should carry nothing inward, got %+v", got)
	}
	inward := fromProtoTemplateAnswers([]*postpilotv1.TemplateAnswer{
		{Label: "총평 별점", Text: "4.5점", Enabled: true},
		{Label: "방문일", Enabled: false},
	})
	want := []post.TemplateAnswer{
		{Label: "총평 별점", Text: "4.5점", Enabled: true},
		{Label: "방문일", Text: "", Enabled: false},
	}
	if !reflect.DeepEqual(inward, want) {
		t.Errorf("inward = %+v, want %+v", inward, want)
	}
	outward := toProtoTemplateAnswers(want)
	if len(outward) != 2 || outward[0].GetLabel() != "총평 별점" || outward[0].GetText() != "4.5점" ||
		!outward[0].GetEnabled() || outward[1].GetEnabled() {
		t.Errorf("outward = %+v", outward)
	}
	// A post with no answers sends an empty list, never a nil the client has to guard.
	if got := toProtoTemplateAnswers(nil); got == nil || len(got) != 0 {
		t.Errorf("no answers should marshal as an empty list, got %+v", got)
	}
}

// The list narrows by tag in the browser (POST-65), so the summary mapper has to carry the
// tags across the seam — and a post with none must arrive as an empty repeated field rather
// than as a single blank tag the client would then render.
func TestToProtoSummaryCarriesTags(t *testing.T) {
	tagged := toProtoSummary(post.Summary{
		Slug: "20260828-jeju", Title: "제주 3일", Status: "review",
		UpdatedAt: time.Date(2026, 8, 28, 11, 58, 0, 0, time.UTC),
		Tags:      []string{"제주", "카페"},
	})
	if want := []string{"제주", "카페"}; !reflect.DeepEqual(tagged.GetTags(), want) {
		t.Errorf("tags = %v, want %v", tagged.GetTags(), want)
	}

	untagged := toProtoSummary(post.Summary{Slug: "20260820-busan", Status: "draft"})
	if len(untagged.GetTags()) != 0 {
		t.Errorf("tags = %v, want none", untagged.GetTags())
	}
}

// The numbers and names the surface needs travel as params: the range a length missed, how many
// image places still name a detached photo, a storyline paragraph's ceiling and the file a
// storyline edit may not place (POST-13, POST-20, POST-96).
func TestTheRangeAndTheMissingCountTravelAsParams(t *testing.T) {
	for _, tc := range []struct {
		err    error
		code   connect.Code
		reason string
		want   map[string]string
	}{
		{&post.TargetLengthError{Min: 100, Max: 10_000}, connect.CodeInvalidArgument, "POST_TARGET_LENGTH_INVALID", map[string]string{"min": "100", "max": "10000"}},
		{&post.PhotoMissingError{Count: 2}, connect.CodeFailedPrecondition, "POST_PHOTO_MISSING", map[string]string{"count": "2"}},
		{&post.StorylineTextTooLongError{Max: 1000}, connect.CodeInvalidArgument, "POST_STORYLINE_INVALID", map[string]string{"max": "1000"}},
		{&post.StorylineFileUnknownError{File: "later.jpg"}, connect.CodeInvalidArgument, "POST_STORYLINE_FILE_UNKNOWN", map[string]string{"file": "later.jpg"}},
	} {
		mapped := toConnectError("op", errors.Join(errors.New("private context"), tc.err))
		if connect.CodeOf(mapped) != tc.code {
			t.Fatalf("%v code = %v, want %v", tc.err, connect.CodeOf(mapped), tc.code)
		}
		detail := postAppErrorDetail(t, mapped)
		if detail.GetReason() != tc.reason || !reflect.DeepEqual(detail.GetParams(), tc.want) {
			t.Fatalf("%v detail = %v %v, want %s %v", tc.err, detail.GetReason(), detail.GetParams(), tc.reason, tc.want)
		}
	}
}

// POST-96: an absent edit keeps the stored storyline; a present one arrives whole.
func TestTheStorylineEditTravelsWhole(t *testing.T) {
	if fromProtoStorylineEdit(nil) != nil {
		t.Fatal("an absent edit arrived as one")
	}
	got := fromProtoStorylineEdit(&postpilotv1.StorylineEdit{Paragraphs: []*postpilotv1.StorylineParagraph{
		{Text: "가게 앞", Files: []string{"a.jpg"}}, {Text: "마무리"},
	}})
	want := &post.StorylineEdit{Paragraphs: []post.StorylineParagraph{{Text: "가게 앞", Files: []string{"a.jpg"}}, {Text: "마무리"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("edit = %+v, want %+v", got, want)
	}
}
