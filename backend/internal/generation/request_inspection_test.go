package generation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/post"
)

type inspectionCaptureFixture struct {
	runs        []post.RequestCaptureRun
	calls       []post.RequestCaptureCall
	inspections []llm.RequestInspection
	finished    []*post.RequestCaptureCompletion
	writeErr    error
	reads       int
	readStage   string
}

func (f *inspectionCaptureFixture) WritePostRequestCapture(ctx context.Context, run post.RequestCaptureRun, call post.RequestCaptureCall, inspection llm.RequestInspection) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	f.runs = append(f.runs, run)
	f.calls = append(f.calls, call)
	f.inspections = append(f.inspections, inspection)
	return f.writeErr
}
func (f *inspectionCaptureFixture) FinishPostRequestCapture(ctx context.Context, _ post.RequestCaptureRun, completion *post.RequestCaptureCompletion) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	f.finished = append(f.finished, completion)
	return nil
}
func (f *inspectionCaptureFixture) ReadPostRequestInspection(_ context.Context, _, _, stage string, _ llm.InspectionStatus) (llm.RequestInspection, error) {
	f.reads++
	f.readStage = stage
	return inspectionUnavailable(stage, "No retained capture matches this result."), nil
}

type inspectionPreparationFixture struct {
	requests []llm.Request
	refs     []llm.ModelRef
	users    []string
}

func (f *inspectionPreparationFixture) PreparePostRequest(_ context.Context, userID string, model llm.ModelRef, request llm.Request) (llm.RequestInspection, error) {
	f.requests = append(f.requests, request)
	f.refs = append(f.refs, model)
	f.users = append(f.users, userID)
	result, err := llm.PreparedRequestInspection(request)
	if err != nil {
		return result, err
	}
	budget := int64(request.MaxTokens)
	effort, structured := llm.ReasoningHigh, request.JSONSchema != nil
	result.Conditions = &llm.EffectiveRequestConditions{Model: &model, MaxCompletionTokens: &budget, ReasoningEffort: &effort, StructuredOutput: &structured}
	return result, result.Validate()
}

type inspectionSelectionsFixture struct{ observe, write llm.ModelRef }

func (f inspectionSelectionsFixture) ModelForInspection(_ context.Context, _, stage string) (llm.ModelRef, bool, error) {
	if stage == llm.StageNameObserve {
		return f.observe, f.observe.ModelID != "", nil
	}
	return f.write, f.write.ModelID != "", nil
}

type inspectionForbiddenMedia struct{}

func (inspectionForbiddenMedia) Read(context.Context, string) ([]byte, error) {
	panic("preview read media")
}
func (inspectionForbiddenMedia) PresignGet(context.Context, string, time.Duration) (string, error) {
	panic("preview minted a media link")
}

type inspectionPublisherFixture struct{ posts *fakePosts }

func (f *inspectionPublisherFixture) PublishGeneratedResult(_ context.Context, _, _ string, result OriginPostCompletion) (post.OriginResultIdentity, error) {
	f.posts.input.Content = &result.Content
	f.posts.input.ContentRevision++
	if result.Annotations != nil && result.Annotations.Storyline != nil {
		f.posts.input.Storyline = result.Annotations.Storyline
		f.posts.input.StorylineFingerprint = "published-plan"
		f.posts.input.StorylineEditedByHand = false
	}
	identity := OriginContentIdentity(result.Content)
	identity.ContentRevision = f.posts.input.ContentRevision
	return identity, nil
}
func (f *inspectionPublisherFixture) PublishStorylineResult(_ context.Context, _, _ string, result OriginStorylineCompletion) error {
	f.posts.input.Storyline = &result.Storyline
	f.posts.input.StorylineFingerprint = "published-plan"
	f.posts.input.StorylineEditedByHand = false
	return nil
}

func inspectedFixtureService(posts *fakePosts, models *fakeModels, jobs *fakeJobs, capture *inspectionCaptureFixture, prepare *inspectionPreparationFixture) *Service {
	pub := &inspectionPublisherFixture{posts: posts}
	service := NewOriginService(NewService(posts, fakeProfiles{}, models, fakeImages{}, jobs, 2, testReasoningPolicy, testBudget, testDeps()), pub, pub)
	return NewInspectedService(service, RequestInspectionDependencies{Captures: capture, Models: prepare, Selections: inspectionSelectionsFixture{observe: videoObserveRef, write: writeRef}})
}

func TestPostInspectionPreviewsUseStageAssemblyWithoutExecutionOrMediaAccess(t *testing.T) {
	language := LanguageEnglish
	base := PostInput{UserID: "alice", Slug: "p", TargetLanguage: language, ContentLanguage: &language, Memo: "Owner fact", Content: &PostContent{Title: "Current", Blocks: []Block{{Type: BlockText, Content: "Current prose"}}}}
	for _, tc := range []struct {
		stage                       string
		status                      llm.InspectionStatus
		media                       []Image
		expectedStage, expectedMode string
	}{
		{"write", llm.InspectionPrepared, nil, "post-writing", "direct"},
		{"plan", llm.InspectionPrepared, nil, "post-storyline", "storyline-create"},
		{"revise", llm.InspectionCurrent, nil, "post-revision", "revision"},
		{"observe", llm.InspectionPrepared, []Image{{ID: "photo-1", Filename: "one.jpg", Key: "secret/storage/key", Kind: AttachmentPhoto}}, "post-observation", "photo-observation"},
		{"observe", llm.InspectionPrepared, []Image{{ID: "video-1", Filename: "one.mp4", Key: "secret/storage/key", Kind: AttachmentVideo}}, "post-observation", "video-observation"},
	} {
		t.Run(tc.stage+"/"+tc.expectedMode, func(t *testing.T) {
			input := base
			input.Images = tc.media
			posts, models, jobs := &fakePosts{input: input}, videoModels(), &fakeJobs{}
			prepare, capture := &inspectionPreparationFixture{}, &inspectionCaptureFixture{}
			service := inspectedFixtureService(posts, models, jobs, capture, prepare)
			service.images, service.videos = inspectionForbiddenMedia{}, inspectionForbiddenMedia{}
			models.complete = func(llm.ModelRef, llm.Request) (llm.Response, error) { panic("preview dispatched") }
			result, err := service.ReadPostRequestInspection(t.Context(), "alice", "p", tc.stage, tc.status)
			if err != nil || result.Status != tc.status || result.Stage != tc.expectedStage || result.Mode != tc.expectedMode {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			if len(prepare.requests) != 1 || prepare.users[0] != "alice" || result.Conditions == nil || *result.Conditions.ReasoningEffort != llm.ReasoningHigh {
				t.Fatal("pure effective preparation was not used")
			}
			if len(models.calls) != 0 || jobs.enqueues != 0 || len(posts.contents)+len(posts.observationWrites)+len(posts.storylines) != 0 || len(capture.calls) != 0 {
				t.Fatal("preview performed execution or mutation")
			}
			raw, _ := json.Marshal(result)
			if strings.Contains(string(raw), "secret/storage/key") || strings.Contains(string(raw), "request-preview.invalid") {
				t.Fatal("private media reached inspection")
			}
			if len(tc.media) > 0 && (len(result.Attachments) != 1 || result.Attachments[0].ID != tc.media[0].ID || result.Attachments[0].Kind != string(tc.media[0].Kind)) {
				t.Fatal("safe media incarnation is absent")
			}
			if tc.stage == "observe" && (strings.Contains(string(raw), "Owner fact") || strings.Contains(string(raw), "Current prose")) {
				t.Fatal("observation received writing context")
			}
		})
	}
}

func TestPostInspectionUnavailableConditionsDoNotSynthesizePastOrFutureMaterial(t *testing.T) {
	for _, tc := range []struct {
		name, stage string
		status      llm.InspectionStatus
		input       PostInput
	}{
		{"revision instruction", "revise", llm.InspectionPrepared, PostInput{Content: &PostContent{Title: "Old"}}},
		{"pending observation", "write", llm.InspectionPrepared, PostInput{Images: []Image{photo("a.jpg")}}},
		{"missing plan", "write-from-storyline", llm.InspectionPrepared, PostInput{}},
		{"published", "write", llm.InspectionPrepared, PostInput{Published: true}},
		{"no media", "observe", llm.InspectionPrepared, PostInput{}},
		{"legacy historical", "write", llm.InspectionCaptured, PostInput{Memo: "new source must not become old prompt"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			posts, prepare, capture := &fakePosts{input: tc.input}, &inspectionPreparationFixture{}, &inspectionCaptureFixture{}
			service := inspectedFixtureService(posts, videoModels(), &fakeJobs{}, capture, prepare)
			result, err := service.ReadPostRequestInspection(t.Context(), "alice", "p", tc.stage, tc.status)
			if err != nil || result.Status != llm.InspectionUnavailable || result.UnavailableReason == "" || len(result.Fragments) != 0 || len(prepare.requests) != 0 {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			if tc.status == llm.InspectionCaptured && (capture.reads != 1 || capture.readStage != "post-writing" || posts.reads != 0) {
				t.Fatal("historical read used current composer/source")
			}
		})
	}
}

func TestPostInspectionChecksOwnershipBeforeUnknownView(t *testing.T) {
	posts := &fakePosts{err: post.ErrForbidden}
	service := inspectedFixtureService(posts, videoModels(), &fakeJobs{}, &inspectionCaptureFixture{}, &inspectionPreparationFixture{})
	if _, err := service.ReadPostRequestInspection(t.Context(), "bob", "p", "unknown", llm.InspectionPrepared); !errors.Is(err, post.ErrForbidden) {
		t.Fatal(err)
	}
}

func fixtureIssuedResponse(t *testing.T, request llm.Request, response llm.Response) llm.Response {
	t.Helper()
	prepared, err := llm.PreparedRequestInspection(request)
	if err != nil {
		t.Fatal(err)
	}
	witness, err := llm.CapturedRequestInspection(prepared, time.Unix(123, 0), llm.Usage{PromptTokens: 11, CompletionTokens: 7})
	if err != nil {
		t.Fatal(err)
	}
	response.Inspection = &witness
	return response
}

func TestPostGenerationCapturesEveryActualPhotoBatchVideoAndWriteBeforeParsing(t *testing.T) {
	posts := &fakePosts{input: PostInput{UserID: "alice", Slug: "p", SourceFingerprint: "source", StorylineFingerprint: "plan", InputRevision: 3, ContentRevision: 7, TargetLanguage: LanguageEnglish}}
	for i := 0; i < 5; i++ {
		posts.input.Images = append(posts.input.Images, Image{ID: fmt.Sprint(i), Filename: fmt.Sprintf("%d.jpg", i), Key: "raw-photo-key", Kind: AttachmentPhoto})
	}
	posts.input.Images = append(posts.input.Images, Image{ID: "video", Filename: "v.mp4", Key: "raw-video-key", Kind: AttachmentVideo})
	models, jobs, capture, prepare := videoModels(), &fakeJobs{id: "job"}, &inspectionCaptureFixture{}, &inspectionPreparationFixture{}
	service := inspectedFixtureService(posts, models, jobs, capture, prepare)
	service.videos = &fakeLinker{}
	models.complete = func(_ llm.ModelRef, request llm.Request) (llm.Response, error) {
		if request.Stage == llm.StageNameObserve {
			return fixtureIssuedResponse(t, request, observationAnswer(request)), nil
		}
		return fixtureIssuedResponse(t, request, llm.Response{Text: `{"title":"Title","summary":"Summary","tags":[],"blocks":[{"type":"TEXT","content":"Good prose"}],"storyline":[{"text":"Plan","files":[]}],"nouns":[]}`}), nil
	}
	if _, err := service.Start(t.Context(), StartRequest{UserID: "alice", PostSlug: "p", ObserveModel: videoObserveRef.String(), WriteModel: writeRef.String()}); err != nil {
		t.Fatal(err)
	}
	run := jobs.queued(0)
	run.ID = "job"
	if err := service.Generate(t.Context(), run, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	if len(models.calls) != 5 || len(capture.calls) != 5 || len(prepare.requests) != 0 {
		t.Fatalf("complete=%d capture=%d prepare=%d", len(models.calls), len(capture.calls), len(prepare.requests))
	}
	for i, call := range capture.calls {
		if call.Sequence != i+1 || call.ID != fmt.Sprintf("job:%d", i+1) || capture.runs[i].SourceFingerprint != "source" || capture.runs[i].InputRevision != 3 || capture.runs[i].ContentRevision != 7 || capture.inspections[i].Status != llm.InspectionCaptured {
			t.Fatalf("call=%+v run=%+v", call, capture.runs[i])
		}
		if !reflect.DeepEqual(capture.inspections[i], *modelsResponseWitness(t, models.calls[i].request)) {
			t.Fatal("capture differs from actual invocation witness")
		}
	}
	if len(capture.calls[0].AttachmentIDs) != 2 || capture.calls[3].AttachmentKinds[0] != "video" || len(capture.calls[4].AttachmentIDs) != 6 {
		t.Fatal("batch/video safe identities differ")
	}
	if len(capture.finished) != 1 || capture.finished[0] == nil || capture.finished[0].Result == nil {
		t.Fatal("successful result did not bind issued calls")
	}
	raw, _ := json.Marshal(capture.inspections)
	if strings.Contains(string(raw), "raw-photo-key") || strings.Contains(string(raw), "raw-video-key") || strings.Contains(string(raw), "signed") && strings.Contains(string(raw), "storage.example") {
		t.Fatal("media leaked through captured projection")
	}
}

func modelsResponseWitness(t *testing.T, request llm.Request) *llm.RequestInspection {
	return fixtureIssuedResponse(t, request, llm.Response{}).Inspection
}

func TestPostCaptureRetainsFailedInterruptedAndMalformedCallWithoutRepair(t *testing.T) {
	for _, mode := range []string{"parse failure", "provider failure", "interrupted", "missing witness", "invalid witness", "capture failure"} {
		t.Run(mode, func(t *testing.T) {
			posts, jobs, capture, prepare := &fakePosts{input: PostInput{UserID: "alice", Slug: "p", TargetLanguage: LanguageEnglish}}, &fakeJobs{id: "job"}, &inspectionCaptureFixture{}, &inspectionPreparationFixture{}
			models := videoModels()
			service := inspectedFixtureService(posts, models, jobs, capture, prepare)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			models.complete = func(_ llm.ModelRef, request llm.Request) (llm.Response, error) {
				response := fixtureIssuedResponse(t, request, llm.Response{Text: `{"title":"Title","summary":"Summary","tags":[],"blocks":[{"type":"TEXT","content":"Canonical survives"}],"storyline":[{"text":"Plan","files":[]}],"nouns":[]}`})
				switch mode {
				case "parse failure":
					response.Text = "broken JSON"
				case "provider failure":
					return llm.Response{}, llm.WithRequestInspectionError(errors.New("provider failed"), *response.Inspection)
				case "interrupted":
					cancel()
					return llm.Response{}, llm.WithRequestInspectionError(context.Canceled, *response.Inspection)
				case "missing witness":
					response.Inspection = nil
				case "invalid witness":
					response.Inspection.Version = -1
				case "capture failure":
					capture.writeErr = errors.New("storage unavailable")
				}
				return response, nil
			}
			if _, err := service.Start(t.Context(), StartRequest{UserID: "alice", PostSlug: "p", WriteModel: writeRef.String()}); err != nil {
				t.Fatal(err)
			}
			run := jobs.queued(0)
			run.ID = "job"
			err := service.Generate(ctx, run, func(string, int, int) {})
			failed := mode == "parse failure" || mode == "provider failure" || mode == "interrupted"
			if failed != (err != nil) {
				t.Fatalf("err=%v", err)
			}
			if len(models.calls) != 1 || len(capture.calls) != 1 || len(capture.finished) != 1 || len(prepare.requests) != 0 || jobs.enqueues != 1 {
				t.Fatal("capture was lost or attempted repair/replay")
			}
			want := llm.InspectionCaptured
			if mode == "missing witness" || mode == "invalid witness" {
				want = llm.InspectionUnavailable
			}
			if capture.inspections[0].Status != want {
				t.Fatal(capture.inspections[0])
			}
			if failed && capture.finished[0] != nil {
				t.Fatal("failed execution bound a result")
			}
			if !failed && (capture.finished[0] == nil || capture.finished[0].Result == nil) {
				t.Fatal("valid canonical output was discarded")
			}
			if mode == "capture failure" && (len(capture.finished[0].UnavailableStages) != 1 || capture.finished[0].UnavailableReason == "") {
				t.Fatal("capture storage failure could expose an earlier result witness")
			}
		})
	}
}

func TestPostPlanAndRevisionHandlersCaptureTheirActualFrozenRequests(t *testing.T) {
	for _, mode := range []string{"storyline-create", "storyline-rewrite", "revision"} {
		t.Run(mode, func(t *testing.T) {
			language, length := LanguageEnglish, 1200
			posts := &fakePosts{input: PostInput{UserID: "alice", Slug: "p", SourceFingerprint: "source", StorylineFingerprint: "original-plan", TargetLanguage: language, ContentLanguage: &language, TargetLength: &length, Content: &PostContent{Title: "Current title", Blocks: []Block{{Type: BlockText, Content: "Current material"}}}, Storyline: &Storyline{Paragraphs: []StorylineParagraph{{Text: "Stored plan"}}}}}
			models, jobs, capture, prepare := videoModels(), &fakeJobs{id: "job"}, &inspectionCaptureFixture{}, &inspectionPreparationFixture{}
			service := inspectedFixtureService(posts, models, jobs, capture, prepare)
			models.complete = func(_ llm.ModelRef, request llm.Request) (llm.Response, error) {
				if request.Composition.Mode != mode {
					t.Fatalf("actual mode=%s", request.Composition.Mode)
				}
				answer := `{"storyline":[{"text":"Result plan","files":[]}]}`
				if mode == "revision" {
					answer = `{"title":"Current title","summary":"","tags":[],"blocks":[{"type":"TEXT","content":"Revised material"}]}`
				}
				return fixtureIssuedResponse(t, request, llm.Response{Text: answer}), nil
			}
			var err error
			var expectedCap int
			switch mode {
			case "storyline-create":
				_, err = service.StartStoryline(t.Context(), StartStorylineRequest{UserID: "alice", PostSlug: "p", WriteModel: writeRef.String()})
				if err == nil {
					expectedCap = jobs.storylineStarts[0].CompletionTokens
					err = service.WriteStoryline(t.Context(), StorylineJob{ID: "job", UserID: "alice", PostSlug: "p", WriteModel: writeRef.String(), Payload: jobs.storylinePayloads[0]}, func(string, int, int) {})
				}
			case "storyline-rewrite":
				_, err = service.StartStorylineRevision(t.Context(), StartStorylineRevisionRequest{UserID: "alice", PostSlug: "p", WriteModel: writeRef.String(), Request: "Keep the supplied edit"})
				if err == nil {
					expectedCap = jobs.storylineRequests[0].CompletionTokens
					err = service.ReviseStoryline(t.Context(), StorylineRevisionJob{ID: "job", UserID: "alice", PostSlug: "p", WriteModel: writeRef.String(), Payload: jobs.storylineRequestPayloads[0]}, func(string, int, int) {})
				}
			case "revision":
				_, err = service.StartRevision(t.Context(), StartRevisionRequest{UserID: "alice", PostSlug: "p", WriteModel: writeRef.String(), Instruction: "Keep the supplied edit"})
				if err == nil {
					expectedCap = jobs.revisions[0].CompletionTokens
					newLength := 600
					posts.input.TargetLength = &newLength
					err = service.Revise(t.Context(), RevisionJob{ID: "job", UserID: "alice", PostSlug: "p", WriteModel: writeRef.String(), Payload: jobs.payloads[0]}, func(string, int, int) {})
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(models.calls) != 1 || len(capture.calls) != 1 || capture.inspections[0].Mode != mode || capture.inspections[0].Status != llm.InspectionCaptured || len(prepare.requests) != 0 || models.calls[0].request.MaxTokens != expectedCap {
				t.Fatal("handler did not capture the one actual frozen request")
			}
			if !reflect.DeepEqual(capture.inspections[0], *modelsResponseWitness(t, models.calls[0].request)) {
				t.Fatal("captured request changed after invocation")
			}
			if mode != "storyline-create" {
				var text strings.Builder
				for _, fragment := range capture.inspections[0].Fragments {
					text.WriteString(fragment.Text)
				}
				if !strings.Contains(text.String(), "Keep the supplied edit") {
					t.Fatal("frozen explicit request missing")
				}
			}
		})
	}
}

func TestPreparedWriteMatchesExecutionAndLaterCaptureKeepsAdmittedBudget(t *testing.T) {
	length := 1200
	posts := &fakePosts{input: PostInput{UserID: "alice", Slug: "p", TargetLanguage: LanguageEnglish, TargetLength: &length, Memo: "Owner fact"}}
	models, jobs, capture, prepare := videoModels(), &fakeJobs{id: "job"}, &inspectionCaptureFixture{}, &inspectionPreparationFixture{}
	service := inspectedFixtureService(posts, models, jobs, capture, prepare)
	if _, err := service.ReadPostRequestInspection(t.Context(), "alice", "p", "write", llm.InspectionPrepared); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Start(t.Context(), StartRequest{UserID: "alice", PostSlug: "p", WriteModel: writeRef.String(), TargetLength: &length}); err != nil {
		t.Fatal(err)
	}
	models.complete = func(_ llm.ModelRef, request llm.Request) (llm.Response, error) {
		return fixtureIssuedResponse(t, request, llm.Response{Text: `{"title":"Title","summary":"Summary","tags":[],"blocks":[{"type":"TEXT","content":"Good prose"}],"storyline":[{"text":"Plan","files":[]}],"nouns":[]}`}), nil
	}
	newLength := 600
	posts.input.TargetLength = &newLength
	run := jobs.queued(0)
	run.ID = "job"
	if err := service.Generate(t.Context(), run, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(prepare.requests[0], models.calls[0].request) {
		t.Fatal("preview and actual execution diverged under the same admitted material/options")
	}
	if models.calls[0].request.MaxTokens != jobs.generations[0].CompletionTokens || models.calls[0].request.MaxTokens == service.budget.Write(OriginBudgetTarget(&newLength), false) {
		t.Fatal("later current length replaced the admitted cap")
	}
}

type inspectionRacingPosts struct{ *fakePosts }

func (f inspectionRacingPosts) AttachedImages(ctx context.Context, userID, slug string) (PostInput, error) {
	if f.reads == 2 {
		f.input.StorylineEditedByHand = true
		f.input.StorylineFingerprint = "owner-edited-plan"
		f.input.Storyline = &Storyline{Paragraphs: []StorylineParagraph{{Text: "An independently edited plan"}}}
	}
	return f.fakePosts.AttachedImages(ctx, userID, slug)
}

func TestPostCaptureDoesNotBindAPlanEditedAfterCanonicalPublication(t *testing.T) {
	posts, models, jobs := &fakePosts{input: PostInput{UserID: "alice", Slug: "p", TargetLanguage: LanguageEnglish}}, videoModels(), &fakeJobs{}
	capture := &inspectionCaptureFixture{}
	service := inspectedFixtureService(posts, models, jobs, capture, &inspectionPreparationFixture{})
	service.posts = inspectionRacingPosts{posts}
	models.complete = func(_ llm.ModelRef, request llm.Request) (llm.Response, error) {
		return fixtureIssuedResponse(t, request, llm.Response{Text: `{"title":"Canonical title","summary":"Summary","tags":[],"blocks":[{"type":"TEXT","content":"Canonical survives"}],"storyline":[{"text":"Issued plan","files":[]}],"nouns":[]}`}), nil
	}
	payload := mustGeneratePayload(t, generationOptions{TargetLanguage: LanguageEnglish, OriginProtocolVersion: OriginProtocolVersion, CompletionTokens: 16384, Profile: &Profile{NoVoice: true}})
	if err := service.Generate(t.Context(), GenerateJob{ID: "job", UserID: "alice", PostSlug: "p", WriteModel: writeRef.String(), Payload: payload}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	if len(models.calls) != 1 || posts.input.Content == nil || posts.input.Content.Title != "Canonical title" || len(capture.finished) != 1 || capture.finished[0] != nil {
		t.Fatal("a changed plan was retargeted or canonical output was discarded/replayed")
	}
}
