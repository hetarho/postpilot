package generation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/llm"
)

var videoObserveRef = llm.ModelRef{ProviderID: "provider", ModelID: "watcher"}

// fakeLinker is the URL minting a video reaches a model by. It records what it signed, which
// is how a test proves the bytes never entered the process.
type fakeLinker struct {
	keys []string
	ttls []time.Duration
	err  error
}

func (f *fakeLinker) PresignGet(_ context.Context, key string, ttl time.Duration) (string, error) {
	f.keys = append(f.keys, key)
	f.ttls = append(f.ttls, ttl)
	if f.err != nil {
		return "", f.err
	}
	return "https://storage.example/" + key + "?signed", nil
}

func videoModels() *fakeModels {
	models := newFakeModels()
	models.infos[videoObserveRef] = llm.ModelInfo{
		Ref: videoObserveRef, Vision: true, VideoInput: true, StructuredOutput: true,
		Stages: []string{llm.StageNameObserve, llm.StageNameWrite, llm.StageNameAnalyze},
	}
	return models
}

func photo(name string) Image {
	return Image{Filename: name, Key: "posts/p/" + name, Kind: AttachmentPhoto, ContentType: "image/jpeg"}
}

func clip(name string) Image {
	return Image{
		Filename: name, Key: "posts/p/" + name, Kind: AttachmentVideo,
		ContentType: "video/mp4", DurationMs: 8000,
	}
}

// answers one observation entry per file named on the `files:` line, with the video fields set
// when the call carried a video part.
func observationAnswer(request llm.Request) llm.Response {
	parts := request.Messages[0].Parts
	files := strings.Split(strings.TrimPrefix(parts[len(parts)-1].Text, "files: "), ", ")
	items := make([]string, 0, len(files))
	for _, file := range files {
		if request.HasVideos() {
			items = append(items, fmt.Sprintf(
				`{"file":%q,"scene":"바다","mood":"","visible_text":"","objects":[],"people_present":false,"events":["파도가 친다"],"speech":"좋다"}`, file))
			continue
		}
		items = append(items, fmt.Sprintf(
			`{"file":%q,"scene":"seen","mood":"","visible_text":"","objects":[],"people_present":false}`, file))
	}
	return llm.Response{Text: `{"observations":[` + strings.Join(items, ",") + `]}`}
}

func videoService(t *testing.T, posts *fakePosts, models *fakeModels, linker *fakeLinker) *Service {
	t.Helper()
	svc := NewService(posts, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, &fakeJobs{}, 4, testReasoningPolicy, testBudget)
	svc.SetVideoLinker(linker, 10*time.Minute)
	return svc
}

// VIDEO-8: the photo batches run first and unchanged, then ONE call per video — never batched,
// never mixed with photos. VIDEO-10: the clip reaches the model as a signed URL, and no part of
// the call carries its bytes.
func TestObserveRunsOneCallPerVideoAfterThePhotoBatches(t *testing.T) {
	posts := &fakePosts{}
	models := videoModels()
	models.complete = func(_ llm.ModelRef, request llm.Request) (llm.Response, error) {
		return observationAnswer(request), nil
	}
	linker := &fakeLinker{}
	post := PostInput{Slug: "p", UserID: "alice", Images: []Image{
		photo("IMG_1.jpg"), photo("IMG_2.jpg"), photo("IMG_3.jpg"), photo("IMG_4.jpg"), photo("IMG_5.jpg"),
		clip("a.mp4"), clip("b.mp4"),
	}}
	posts.input = post

	var progress []string
	got, err := videoService(t, posts, models, linker).observe(
		context.Background(), post, post.Images, nil, videoObserveRef,
		func(stage string, done, total int) {
			progress = append(progress, fmt.Sprintf("%s:%d/%d", stage, done, total))
		})
	if err != nil {
		t.Fatal(err)
	}

	// ceil(5/4) photo batches + one call per video.
	if len(models.calls) != 4 {
		t.Fatalf("observation calls = %d, want 2 photo batches + 2 videos", len(models.calls))
	}
	for i, call := range models.calls[:2] {
		if call.request.HasVideos() {
			t.Errorf("photo batch %d carried a video part", i)
		}
		if call.request.System != ObservePrompt {
			t.Errorf("photo batch %d used the wrong system prompt", i)
		}
	}
	for i, call := range models.calls[2:] {
		if !call.request.HasVideos() || call.request.HasImages() {
			t.Errorf("video call %d = images:%v videos:%v, want a video part alone", i, call.request.HasImages(), call.request.HasVideos())
		}
		if call.request.System != ObserveVideoPrompt {
			t.Errorf("video call %d did not use the video observe prompt", i)
		}
		// One video per call, so exactly one file is named.
		parts := call.request.Messages[0].Parts
		if files := strings.TrimPrefix(parts[len(parts)-1].Text, "files: "); strings.Contains(files, ",") {
			t.Errorf("video call %d named more than one file: %q", i, files)
		}
		if call.request.MaxTokens != testBudget.Observation() {
			t.Errorf("video call %d budget = %d, want one full batch's", i, call.request.MaxTokens)
		}
	}

	// The URL is minted per call, for that clip's key, at the view-URL lifetime.
	if len(linker.keys) != 2 || linker.keys[0] != "posts/p/a.mp4" || linker.keys[1] != "posts/p/b.mp4" {
		t.Fatalf("signed keys = %v", linker.keys)
	}
	if linker.ttls[0] != 10*time.Minute {
		t.Errorf("url ttl = %v", linker.ttls[0])
	}
	part := models.calls[2].request.Messages[0].Parts[0]
	if !strings.HasPrefix(part.VideoURL, "https://storage.example/posts/p/a.mp4") || part.MIME != "video/mp4" {
		t.Errorf("video part = %+v", part)
	}

	// One progress scale over both stages, and the snapshot grows once per call.
	if strings.Join(progress, " ") != "observe:4/7 observe:5/7 observe:6/7 observe:7/7" {
		t.Fatalf("progress = %v", progress)
	}
	if len(posts.observationWrites) != 4 {
		t.Fatalf("incremental writes = %d, want one per call", len(posts.observationWrites))
	}

	// The video's own fields survive the round trip; a photo's entry has neither.
	byFile := map[string]Observation{}
	for _, observation := range got {
		byFile[observation.File] = observation
	}
	if video := byFile["a.mp4"]; len(video.Events) != 1 || video.Speech != "좋다" {
		t.Errorf("video observation = %+v", video)
	}
	if photo := byFile["IMG_1.jpg"]; len(photo.Events) != 0 || photo.Speech != "" {
		t.Errorf("photo observation carried video fields: %+v", photo)
	}
}

// A failure names the stage the user is watching, not the photo one.
func TestVideoObservationFailureNamesTheVideoStage(t *testing.T) {
	models := videoModels()
	models.complete = func(_ llm.ModelRef, request llm.Request) (llm.Response, error) {
		if request.HasVideos() {
			return llm.Response{}, errors.New("provider is down")
		}
		return observationAnswer(request), nil
	}
	post := PostInput{Slug: "p", UserID: "alice", Images: []Image{clip("a.mp4")}}
	_, err := videoService(t, &fakePosts{}, models, &fakeLinker{}).observe(
		context.Background(), post, post.Images, nil, videoObserveRef, func(string, int, int) {})
	if err == nil || !strings.Contains(err.Error(), "영상 관찰") {
		t.Fatalf("error = %v, want the video stage named", err)
	}
}

// VIDEO-11: the capability is a per-run check against the post's own attachments, refused
// before anything is enqueued and naming the model so the picker can be fixed.
func TestStartRefusesAVideoBlindObserveModel(t *testing.T) {
	posts := &fakePosts{}
	models := videoModels()
	jobs := &fakeJobs{id: "job"}
	svc := NewService(posts, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, jobs, 4, testReasoningPolicy, testBudget)
	posts.input = PostInput{
		Slug: "p", UserID: "alice", Voice: VoiceRef{ID: "voice"},
		Images: []Image{photo("IMG_1.jpg"), clip("a.mp4")},
	}

	_, err := svc.Start(context.Background(), StartRequest{
		UserID: "alice", PostSlug: "p",
		ObserveModel: observeRef.String(), WriteModel: writeRef.String(),
	})
	if !errors.Is(err, ErrVideoUnsupported) {
		t.Fatalf("error = %v, want ErrVideoUnsupported", err)
	}
	var unsupported *VideoUnsupportedError
	if !errors.As(err, &unsupported) || unsupported.Model != observeRef.String() {
		t.Fatalf("refusal did not name the model: %v", err)
	}
	if jobs.enqueues != 0 {
		t.Fatal("a refused start still enqueued")
	}

	// The same post with a model that can watch is accepted.
	if _, err := svc.Start(context.Background(), StartRequest{
		UserID: "alice", PostSlug: "p",
		ObserveModel: videoObserveRef.String(), WriteModel: writeRef.String(),
	}); err != nil {
		t.Fatalf("a video-capable observe model was refused: %v", err)
	}

	// A post with photos only is unaffected by any of this.
	posts.input.Images = []Image{photo("IMG_1.jpg")}
	if _, err := svc.Start(context.Background(), StartRequest{
		UserID: "alice", PostSlug: "p",
		ObserveModel: observeRef.String(), WriteModel: writeRef.String(),
	}); err != nil {
		t.Fatalf("a photo-only post was refused: %v", err)
	}

	// And a post with videos and NO observe model hears the simpler thing first.
	posts.input.Images = []Image{clip("a.mp4")}
	if _, err := svc.Start(context.Background(), StartRequest{
		UserID: "alice", PostSlug: "p", ObserveModel: "", WriteModel: writeRef.String(),
	}); !errors.Is(err, ErrObserveModelRequired) {
		t.Fatalf("error = %v, want ErrObserveModelRequired", err)
	}
}

// VIDEO-13: the hold prices one call per frozen video beside the photo batches, and a
// reuse-everything run plans none of either.
func TestStartPlansOneObserveCallPerFrozenVideo(t *testing.T) {
	posts := &fakePosts{}
	models := videoModels()
	jobs := &fakeJobs{id: "job"}
	svc := NewService(posts, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, jobs, 4, testReasoningPolicy, testBudget)
	images := []Image{
		photo("IMG_1.jpg"), photo("IMG_2.jpg"), photo("IMG_3.jpg"), photo("IMG_4.jpg"), photo("IMG_5.jpg"),
		clip("a.mp4"), clip("b.mp4"),
	}
	posts.input = PostInput{Slug: "p", UserID: "alice", Voice: VoiceRef{ID: "voice"}, Images: images}

	if _, err := svc.Start(context.Background(), StartRequest{
		UserID: "alice", PostSlug: "p",
		ObserveModel: videoObserveRef.String(), WriteModel: writeRef.String(),
	}); err != nil {
		t.Fatal(err)
	}
	if got := jobs.generations[0].ObserveCalls; got != 4 {
		t.Fatalf("planned calls = %d, want 2 photo batches + 2 videos", got)
	}

	// Everything already observed and nothing picked: no call is planned at all.
	posts.input.Observations = []Observation{
		{File: "IMG_1.jpg", Scene: "s"}, {File: "IMG_2.jpg", Scene: "s"}, {File: "IMG_3.jpg", Scene: "s"},
		{File: "IMG_4.jpg", Scene: "s"}, {File: "IMG_5.jpg", Scene: "s"},
		// A video entry is carried by what only a clip has.
		{File: "a.mp4", Events: []string{"파도"}}, {File: "b.mp4", Speech: "좋다"},
	}
	none := []string{}
	if _, err := svc.Start(context.Background(), StartRequest{
		UserID: "alice", PostSlug: "p", ObserveFiles: &none,
		ObserveModel: videoObserveRef.String(), WriteModel: writeRef.String(),
	}); err != nil {
		t.Fatal(err)
	}
	if got := jobs.generations[1].ObserveCalls; got != 0 {
		t.Fatalf("reuse-everything planned %d calls, want 0", got)
	}

	// Picking only the video plans exactly its one call.
	one := []string{"a.mp4"}
	if _, err := svc.Start(context.Background(), StartRequest{
		UserID: "alice", PostSlug: "p", ObserveFiles: &one,
		ObserveModel: videoObserveRef.String(), WriteModel: writeRef.String(),
	}); err != nil {
		t.Fatal(err)
	}
	if got := jobs.generations[2].ObserveCalls; got != 1 {
		t.Fatalf("one video planned %d calls, want 1", got)
	}
}

// The picker's contract treats a clip like a photo, by filename — including the reusable rule,
// which for a video may rest on the two fields only a clip has.
func TestObserveSelectionTreatsAVideoLikeAPhoto(t *testing.T) {
	images := []Image{photo("IMG_1.jpg"), clip("a.mp4"), clip("b.mp4")}
	stored := []Observation{
		{File: "IMG_1.jpg", Scene: "바다"},
		// Nothing a photo would count as eyesight, but a clip described by its motion alone.
		{File: "a.mp4", Events: []string{"파도가 친다"}},
	}

	// b.mp4 has nothing to reuse, so it is forced into the selection whatever the picker said.
	none := []string{}
	targets := resolveObserveSelection(images, stored, &none)
	if len(targets) != 1 || targets[0].Filename != "b.mp4" {
		t.Fatalf("targets = %+v, want only the unobserved clip", targets)
	}

	files, snapshot := freezeObserveSelection(images, stored, &none)
	if len(files) != 1 || files[0] != "b.mp4" {
		t.Fatalf("frozen files = %v", files)
	}
	if len(snapshot) != 2 {
		t.Fatalf("frozen snapshot = %+v, want both reusable entries", snapshot)
	}

	frozenTargets, seed := frozenObserveSelection(images, &files, snapshot)
	if len(frozenTargets) != 1 || frozenTargets[0].Filename != "b.mp4" || len(seed) != 2 {
		t.Fatalf("frozen selection = %+v / %+v", frozenTargets, seed)
	}

	// The merged snapshot never shrinks and keeps both kinds in post order.
	merged := mergeObservations(images, seed, []Observation{{File: "b.mp4", Speech: "좋다"}})
	if len(merged) != 3 || merged[0].File != "IMG_1.jpg" || merged[1].File != "a.mp4" || merged[2].File != "b.mp4" {
		t.Fatalf("merged = %+v", merged)
	}
}

// VIDEO-12: the writer is told the two filename lists separately and sees the clips' own
// observations — and a post with no video gets exactly the prompt it got before videos existed.
func TestWritePromptSeparatesVideoMaterialAndLeavesAPhotoOnlyPostUntouched(t *testing.T) {
	observations := []Observation{
		{File: "IMG_1.jpg", Scene: "바다"},
		{File: "a.mp4", Scene: "해변", Events: []string{"파도가 친다"}, Speech: "좋다"},
	}
	system, user := BuildWritePromptForLanguage(LanguageKorean, goldenProfile(), observations,
		"MEMO", "TITLE", []string{"IMG_1.jpg"}, []string{"a.mp4"}, nil, nil, nil)

	for _, want := range []string{
		"첨부 사진 파일명(정확히 일치해야 함): IMG_1.jpg",
		"첨부 영상 파일명(정확히 일치해야 함): a.mp4",
		"사진 관찰: ",
		"영상 관찰: ",
		"파도가 친다",
		"좋다",
	} {
		if !strings.Contains(user, want) {
			t.Errorf("per-post material is missing %q:\n%s", want, user)
		}
	}
	// The video's own fields are not sent for the photo, which has none.
	photoSection := user[strings.Index(user, "사진 관찰: "):strings.Index(user, "첨부 영상")]
	if strings.Contains(photoSection, "events") || strings.Contains(photoSection, "speech") {
		t.Errorf("the photo observations carried video fields:\n%s", photoSection)
	}
	if !strings.Contains(system, "VIDEO 블록은 첨부 영상 파일명만") {
		t.Error("the write prompt did not ask for VIDEO blocks")
	}

	// And with no video: the fixed prompt and the per-post material are what they always were.
	plainSystem, plainUser := BuildWritePromptForLanguage(LanguageKorean, goldenProfile(), observations[:1],
		"MEMO", "TITLE", []string{"IMG_1.jpg"}, nil, nil, nil, nil)
	if strings.Contains(plainSystem, "VIDEO") || strings.Contains(plainUser, "영상") {
		t.Errorf("a photo-only post was told about videos:\n%s\n%s", plainSystem, plainUser)
	}
	if !strings.HasPrefix(plainUser, "[이번 글]\n가제: TITLE\n메모: MEMO\n첨부 파일명(정확히 일치해야 함): IMG_1.jpg\n사진 관찰: ") {
		t.Errorf("the no-video material moved:\n%s", plainUser)
	}
}

// A block names one KIND of attachment. A filename is unique across both, so naming the other
// kind's file is a block the writer got wrong, and keeping it would render the wrong element.
func TestFilterAttachmentsKeepsTheTwoKindsApart(t *testing.T) {
	content := PostContent{Blocks: []Block{
		{Type: BlockText, Content: "본문"},
		{Type: BlockImage, File: "IMG_1.jpg"},
		{Type: BlockVideo, File: "a.mp4"},
		{Type: BlockImage, File: "a.mp4"},
		{Type: BlockVideo, File: "IMG_1.jpg"},
		{Type: BlockVideo, File: "ghost.mp4"},
	}}
	got := FilterAttachments(content, []string{"IMG_1.jpg"}, []string{"a.mp4"})
	if len(got.Blocks) != 3 {
		t.Fatalf("blocks = %+v, want the text, the image and the video", got.Blocks)
	}
	if got.Blocks[1].Type != BlockImage || got.Blocks[1].File != "IMG_1.jpg" ||
		got.Blocks[2].Type != BlockVideo || got.Blocks[2].File != "a.mp4" {
		t.Fatalf("blocks = %+v", got.Blocks)
	}
}

// The attachment list stays ONE list; these are the two views of it the call sites need.
func TestAttachmentNamesSplitsTheKinds(t *testing.T) {
	photos, videos := AttachmentNames([]Image{photo("IMG_1.jpg"), clip("a.mp4"), {Filename: "legacy.jpg"}})
	// An empty Kind reads as a photo — every entry built before videos existed is one.
	if strings.Join(photos, ",") != "IMG_1.jpg,legacy.jpg" || strings.Join(videos, ",") != "a.mp4" {
		t.Fatalf("photos = %v, videos = %v", photos, videos)
	}
}

// A run with a clip and no way to sign it fails its call rather than falling back to something:
// there is no other way to deliver a video.
func TestObserveVideoWithoutALinkerFails(t *testing.T) {
	models := videoModels()
	models.complete = func(_ llm.ModelRef, request llm.Request) (llm.Response, error) {
		return observationAnswer(request), nil
	}
	svc := NewService(&fakePosts{}, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, &fakeJobs{}, 4, testReasoningPolicy, testBudget)
	post := PostInput{Slug: "p", UserID: "alice", Images: []Image{clip("a.mp4")}}
	if _, err := svc.observe(context.Background(), post, post.Images, nil, videoObserveRef, func(string, int, int) {}); err == nil {
		t.Fatal("a video observation ran with no linker configured")
	}
	if len(models.calls) != 0 {
		t.Fatal("a provider was called with no URL to send")
	}
}
