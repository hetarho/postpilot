package generation

import (
	"bytes"
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/llm"
)

const storylineAnswer = `{"storyline":[{"text":"가게 앞을 보여줍니다.","files":["IMG_1.jpg","ghost.jpg"]},{"text":"커피를 이야기합니다.","files":["IMG_2.jpg"]}]}`

var storylineObservations = map[string]string{
	"IMG_1.jpg": `{"file":"IMG_1.jpg","scene":"가게","mood":"","visible_text":"","objects":[],"people_present":false}`,
	"IMG_2.jpg": `{"file":"IMG_2.jpg","scene":"커피","mood":"","visible_text":"","objects":[],"people_present":false}`,
}

// storylineModels answers an observation call with an entry for every photo it names, and every
// other call with answer.
func storylineModels(answer string) *fakeModels {
	models := newFakeModels()
	models.complete = func(_ llm.ModelRef, request llm.Request) (llm.Response, error) {
		if !request.HasImages() {
			return llm.Response{Text: answer}, nil
		}
		var entries []string
		for _, file := range []string{"IMG_1.jpg", "IMG_2.jpg"} {
			for _, message := range request.Messages {
				for _, part := range message.Parts {
					if strings.Contains(part.Text, file) {
						entries = append(entries, storylineObservations[file])
					}
				}
			}
		}
		return llm.Response{Text: `{"observations":[` + strings.Join(entries, ",") + `]}`}, nil
	}
	return models
}

func storylinePost() PostInput {
	return PostInput{
		Slug: "post", UserID: "alice", Voice: liveVoice, Title: "성수 카페", Memo: "라떼가 맛있었다",
		Images: []Image{{Filename: "IMG_1.jpg", Key: "k1"}, {Filename: "IMG_2.jpg", Key: "k2"}},
	}
}

func storylineService(posts *fakePosts, jobs *fakeJobs, models *fakeModels, deps Deps) *Service {
	return NewService(posts, fakeProfiles{}, models, fakeImages{}, jobs, 4, testReasoningPolicy, testBudget, deps)
}

// GEN-68: a storyline start checks Start's preconditions in Start's order and refuses before
// anything is frozen, held or queued.
func TestStartStorylineChecksStartsPreconditions(t *testing.T) {
	videoPost := storylinePost()
	videoPost.Images = append(videoPost.Images, Image{Filename: "clip.mp4", Key: "k3", Kind: AttachmentVideo})
	published := storylinePost()
	published.Published = true
	noLanguage := storylinePost()
	noLanguage.TargetLanguage = ""
	deleted := storylinePost()
	deleted.Voice = deletedVoice
	for name, test := range map[string]struct {
		input        PostInput
		keepLanguage bool
		observe      string
		write        string
		want         error
	}{
		"published":           {input: published, observe: observeRef.String(), write: writeRef.String(), want: ErrPostPublished},
		"no target language":  {input: noLanguage, keepLanguage: true, observe: observeRef.String(), write: writeRef.String(), want: ErrLanguageRequired},
		"a deleted voice":     {input: deleted, observe: observeRef.String(), write: writeRef.String(), want: ErrVoiceDeleted},
		"no write model":      {input: storylinePost(), observe: observeRef.String(), write: "", want: ErrWriteModelRequired},
		"no observe model":    {input: storylinePost(), observe: "", write: writeRef.String(), want: ErrObserveModelRequired},
		"a video-blind model": {input: videoPost, observe: observeRef.String(), write: writeRef.String(), want: ErrVideoUnsupported},
	} {
		posts := &fakePosts{input: test.input, preserveMissingLanguages: test.keepLanguage}
		jobs := &fakeJobs{id: "job"}
		models := storylineModels(storylineAnswer)
		deps := testDeps()
		svc := storylineService(posts, jobs, models, deps)
		_, err := svc.StartStoryline(context.Background(), StartStorylineRequest{
			UserID: "alice", PostSlug: "post", ObserveModel: test.observe, WriteModel: test.write,
		})
		if !errors.Is(err, test.want) {
			t.Errorf("%s: err = %v, want %v", name, err, test.want)
		}
		if jobs.enqueues != 0 || len(models.calls) != 0 {
			t.Errorf("%s: a refused start queued %d and called %d", name, jobs.enqueues, len(models.calls))
		}
	}
}

// GEN-68, GEN-8, GEN-9: the start freezes the target language, the brief, both 지침 groups, the
// opted-in memories, the picker's selection and the reusable snapshot, and prices the calls
// the frozen set will take.
func TestStartStorylineFreezesTheMaterialAndTheSelection(t *testing.T) {
	input := storylinePost()
	input.TemplateID = "tmpl"
	input.UseMemory = true
	input.TargetLanguage = LanguageEnglish
	input.Observations = []Observation{{File: "IMG_1.jpg", Scene: "가게", Model: observeRef.String()}}
	posts := &fakePosts{input: input}
	jobs := &fakeJobs{id: "job"}
	deps := testDeps()
	deps.Templates = &fakeTemplateBriefs{brief: *testBrief()}
	guidelines := &fakeGuidelines{texts: []string{"CCTV를 언급하지 않기"}, defaults: []string{"메모의 이름으로 쓰세요"}}
	deps.Guidelines = guidelines
	deps.Memories = &recordingMemories{texts: []string{"매운 음식을 못 먹는다"}}
	svc := storylineService(posts, jobs, storylineModels(storylineAnswer), deps)

	reobserve := []string{"IMG_2.jpg"}
	id, err := svc.StartStoryline(context.Background(), StartStorylineRequest{
		UserID: "alice", PostSlug: "post", ObserveModel: observeRef.String(), WriteModel: writeRef.String(),
		ObserveFiles: &reobserve,
	})
	if err != nil || id != "job" || len(jobs.storylineStarts) != 1 {
		t.Fatalf("start = %q, %v (%d enqueued)", id, err, len(jobs.storylineStarts))
	}
	request := jobs.storylineStarts[0]
	if request.TargetLanguage != LanguageEnglish || request.VoiceID != liveVoice.ID || request.ObserveCalls != 1 {
		t.Fatalf("request = %+v", request)
	}
	options, err := decodeStorylinePayload(jobs.storylinePayloads[0])
	if err != nil {
		t.Fatal(err)
	}
	if options.TargetLanguage != LanguageEnglish || options.Template == nil || options.Template.Name != testBrief().Name {
		t.Fatalf("options = %+v", options)
	}
	// GEN-73: the storyline prompt carries the frozen [기억], so it asked for the 지침 with it.
	if !guidelines.askedMemories {
		t.Fatal("a storyline carrying memories asked for the 지침 without them")
	}
	if !reflect.DeepEqual(options.Guidelines, []string{"CCTV를 언급하지 않기"}) || !reflect.DeepEqual(options.DefaultGuidelines, []string{"메모의 이름으로 쓰세요"}) ||
		!reflect.DeepEqual(options.Memories, []string{"매운 음식을 못 먹는다"}) {
		t.Fatalf("material = %+v / %+v / %+v", options.Guidelines, options.DefaultGuidelines, options.Memories)
	}
	if options.ObserveFiles == nil || !reflect.DeepEqual(*options.ObserveFiles, []string{"IMG_2.jpg"}) ||
		len(options.Observations) != 1 || options.Observations[0].File != "IMG_1.jpg" {
		t.Fatalf("selection = %v, snapshot %+v", options.ObserveFiles, options.Observations)
	}

	// A post with no attachment freezes no selection and names no observe model.
	posts.input.Images = nil
	if _, err := svc.StartStoryline(context.Background(), StartStorylineRequest{UserID: "alice", PostSlug: "post", WriteModel: writeRef.String()}); err != nil {
		t.Fatal(err)
	}
	bare, err := decodeStorylinePayload(jobs.storylinePayloads[1])
	if err != nil || bare.ObserveFiles != nil || jobs.storylineStarts[1].ObserveModel != "" || jobs.storylineStarts[1].ObserveCalls != 0 {
		t.Fatalf("a bare start froze %+v (%+v, %v)", bare, jobs.storylineStarts[1], err)
	}
}

// GEN-69: the storyline request is trimmed and bounded, needs a storyline to rewrite, keeps
// Start's other preconditions without the observe model, and freezes the request, the stored
// paragraphs, the material and every stored observation.
func TestStartStorylineRevisionFreezesTheRequestAndTheStoredStoryline(t *testing.T) {
	stored := &Storyline{Paragraphs: []StorylineParagraph{{Text: "가게 앞", Files: []string{"IMG_1.jpg"}}}, MadeWith: []string{"IMG_1.jpg"}}
	withStoryline := storylinePost()
	withStoryline.Storyline = stored
	withStoryline.Observations = []Observation{{File: "IMG_1.jpg", Scene: "가게"}}
	published := withStoryline
	published.Published = true
	for name, test := range map[string]struct {
		input   PostInput
		request string
		write   string
		want    error
	}{
		"an empty request":   {input: withStoryline, request: "   ", write: writeRef.String(), want: ErrRevisionInstructionRequired},
		"a too long request": {input: withStoryline, request: strings.Repeat("가", RevisionInstructionMaxChars+1), write: writeRef.String(), want: ErrRevisionInstructionTooLong},
		"no storyline":       {input: storylinePost(), request: "짧게", write: writeRef.String(), want: ErrStorylineMissing},
		"published":          {input: published, request: "짧게", write: writeRef.String(), want: ErrPostPublished},
		"no write model":     {input: withStoryline, request: "짧게", want: ErrWriteModelRequired},
	} {
		jobs := &fakeJobs{id: "job"}
		svc := storylineService(&fakePosts{input: test.input}, jobs, storylineModels(storylineAnswer), testDeps())
		if _, err := svc.StartStorylineRevision(context.Background(), StartStorylineRevisionRequest{
			UserID: "alice", PostSlug: "post", Request: test.request, WriteModel: test.write,
		}); !errors.Is(err, test.want) || jobs.enqueues != 0 {
			t.Errorf("%s: err = %v (%d enqueued), want %v", name, err, jobs.enqueues, test.want)
		}
	}

	jobs := &fakeJobs{id: "job"}
	svc := storylineService(&fakePosts{input: withStoryline}, jobs, storylineModels(storylineAnswer), testDeps())
	if _, err := svc.StartStorylineRevision(context.Background(), StartStorylineRevisionRequest{
		UserID: "alice", PostSlug: "post", Request: "  둘째 문단을 줄여 주세요 ", WriteModel: writeRef.String(),
	}); err != nil {
		t.Fatal(err)
	}
	options, err := decodeStorylineRevisionPayload(jobs.storylineRequestPayloads[0])
	if err != nil {
		t.Fatal(err)
	}
	if options.Request != "둘째 문단을 줄여 주세요" || !reflect.DeepEqual(options.Storyline, stored.Paragraphs) ||
		len(options.Observations) != 1 || options.TargetLanguage != LanguageKorean {
		t.Fatalf("options = %+v", options)
	}
}

// Both payloads survive their bytes, and one without a language reads as Korean (GEN-30).
func TestStorylinePayloadsRoundTrip(t *testing.T) {
	files := []string{}
	options := storylineOptions{
		TargetLanguage: LanguageEnglish,
		storylineMaterial: storylineMaterial{
			Template:   &TemplateBrief{Name: "여행", Body: "# 제목", Facts: []TemplateFact{{Label: "장소", Value: "제주"}}},
			Guidelines: []string{"g"}, DefaultGuidelines: []string{"d"}, Memories: []string{"m"},
		},
		ObserveFiles: &files, Observations: []Observation{{File: "IMG_1.jpg", Scene: "가게"}},
		WriteNativeEffort: true,
	}
	raw, err := encodeStorylinePayload(options)
	if err != nil {
		t.Fatal(err)
	}
	back, err := decodeStorylinePayload(raw)
	if err != nil || !reflect.DeepEqual(back, options) {
		t.Fatalf("storyline payload = %+v, %v\nwant %+v", back, err, options)
	}
	request := storylineRevisionOptions{
		TargetLanguage: LanguageKorean, Request: "짧게",
		Storyline:         []StorylineParagraph{{Text: "가게 앞", Files: []string{"IMG_1.jpg"}}},
		storylineMaterial: storylineMaterial{Guidelines: []string{"g"}},
		Observations:      []Observation{{File: "IMG_1.jpg", Scene: "가게"}},
		WriteNativeEffort: true,
	}
	raw, err = encodeStorylineRevisionPayload(request)
	if err != nil {
		t.Fatal(err)
	}
	backRequest, err := decodeStorylineRevisionPayload(raw)
	if err != nil || !reflect.DeepEqual(backRequest, request) {
		t.Fatalf("storyline request payload = %+v, %v\nwant %+v", backRequest, err, request)
	}
	legacy, err := decodeStorylinePayload([]byte(`{"observe_files":null}`))
	if err != nil || legacy.TargetLanguage != LanguageKorean || legacy.WriteNativeEffort {
		t.Fatalf("a payload without a language or a native-effort flag = %+v, %v", legacy, err)
	}
}

// GEN-22: a storyline start freezes the write model's native-effort flag, as Start does, and
// both storyline calls size their budget from the frozen flag: the 8,192 floor for an ordinary
// model, doubled for one that reasons inside the cap — even after the catalog flag flips.
func TestStorylineCallsSendTheBudgetTheStartFroze(t *testing.T) {
	for _, nativeEffort := range []bool{false, true} {
		want := 8192
		if nativeEffort {
			want = 16384
		}
		input := storylinePost()
		input.Images = nil
		input.Storyline = &Storyline{Paragraphs: []StorylineParagraph{{Text: "가게 앞"}}}
		posts := &fakePosts{input: input}
		jobs := &fakeJobs{id: "job"}
		models := storylineModels(`{"storyline":[{"text":"메모만으로 짭니다.","files":[]}]}`)
		info := models.infos[writeRef]
		info.ReasoningNativeEffort = nativeEffort
		models.infos[writeRef] = info
		svc := storylineService(posts, jobs, models, testDeps())
		if _, err := svc.StartStoryline(context.Background(), StartStorylineRequest{UserID: "alice", PostSlug: "post", WriteModel: writeRef.String()}); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.StartStorylineRevision(context.Background(), StartStorylineRevisionRequest{
			UserID: "alice", PostSlug: "post", Request: "짧게", WriteModel: writeRef.String(),
		}); err != nil {
			t.Fatal(err)
		}
		if jobs.storylineStarts[0].WriteNativeEffort != nativeEffort || jobs.storylineRequests[0].WriteNativeEffort != nativeEffort {
			t.Fatalf("native effort %v: the starts froze %v and %v", nativeEffort, jobs.storylineStarts[0].WriteNativeEffort, jobs.storylineRequests[0].WriteNativeEffort)
		}
		// The operator flips the catalog flag between the enqueue and the run: the call still
		// sends what the hold priced.
		info.ReasoningNativeEffort = !nativeEffort
		models.infos[writeRef] = info
		if err := svc.WriteStoryline(context.Background(), StorylineJob{
			UserID: "alice", PostSlug: "post", WriteModel: writeRef.String(), Payload: jobs.storylinePayloads[0],
		}, func(string, int, int) {}); err != nil {
			t.Fatal(err)
		}
		if err := svc.ReviseStoryline(context.Background(), StorylineRevisionJob{
			UserID: "alice", PostSlug: "post", WriteModel: writeRef.String(), Payload: jobs.storylineRequestPayloads[0],
		}, func(string, int, int) {}); err != nil {
			t.Fatal(err)
		}
		if len(models.calls) != 2 {
			t.Fatalf("native effort %v: %d calls, want one per storyline job", nativeEffort, len(models.calls))
		}
		for i, call := range models.calls {
			if call.request.MaxTokens != want {
				t.Errorf("native effort %v: call %d sent %d tokens, want %d", nativeEffort, i, call.request.MaxTokens, want)
			}
		}
	}
}

// GEN-68: the handler observes as a generation does, then makes one call on the write model at
// low reasoning with the storyline budget and schema, and stores the storyline alone.
func TestWriteStorylineObservesThenMakesOneCall(t *testing.T) {
	posts := &fakePosts{input: storylinePost()}
	jobs := &fakeJobs{id: "job"}
	models := storylineModels(storylineAnswer)
	svc := storylineService(posts, jobs, models, testDeps())
	if _, err := svc.StartStoryline(context.Background(), StartStorylineRequest{
		UserID: "alice", PostSlug: "post", ObserveModel: observeRef.String(), WriteModel: writeRef.String(),
	}); err != nil {
		t.Fatal(err)
	}
	var stages []string
	err := svc.WriteStoryline(context.Background(), StorylineJob{
		UserID: "alice", PostSlug: "post", ObserveModel: observeRef.String(), WriteModel: writeRef.String(),
		Payload: jobs.storylinePayloads[0],
	}, func(stage string, done, total int) {
		stages = append(stages, stage)
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(posts.observationWrites) == 0 || len(posts.contents) != 0 {
		t.Fatalf("observation writes %d, content writes %d", len(posts.observationWrites), len(posts.contents))
	}
	last := models.calls[len(models.calls)-1]
	for _, call := range models.calls[:len(models.calls)-1] {
		if !call.request.HasImages() {
			t.Fatalf("a call before the storyline call sent no photo: %+v", call.request)
		}
	}
	if last.ref != writeRef || last.request.HasImages() || last.request.MaxTokens != 8192 ||
		last.request.Reasoning != llm.ReasoningLow || !bytes.Equal(last.request.JSONSchema, LegacyStorylineAnswerSchema()) {
		t.Fatalf("storyline call = %+v", last.request)
	}
	if !strings.HasPrefix(last.request.System, koreanStorylineTask) || strings.Contains(last.request.System, "말투 프로필") {
		t.Fatalf("storyline system prompt:\n%s", last.request.System)
	}
	want := Storyline{
		Paragraphs: []StorylineParagraph{{Text: "가게 앞을 보여줍니다.", Files: []string{"IMG_1.jpg"}}, {Text: "커피를 이야기합니다.", Files: []string{"IMG_2.jpg"}}},
		MadeWith:   []string{"IMG_1.jpg", "IMG_2.jpg"},
	}
	if len(posts.storylines) != 1 || !reflect.DeepEqual(posts.storylines[0], want) {
		t.Fatalf("stored %+v, want %+v", posts.storylines, want)
	}
	if stages[len(stages)-2] != "storyline" || stages[len(stages)-1] != "storyline" || stages[0] != "observe" {
		t.Fatalf("stages = %v", stages)
	}
}

// GEN-56, POST-74: a post published after the start calls no provider, and one published while
// the call ran refuses the write; a bad answer fails as GEN-20 says and stores nothing.
func TestWriteStorylineRefusesAPublishedPostAndABadAnswer(t *testing.T) {
	start := func(posts *fakePosts, models *fakeModels) (*Service, []byte) {
		jobs := &fakeJobs{id: "job"}
		svc := storylineService(posts, jobs, models, testDeps())
		if _, err := svc.StartStoryline(context.Background(), StartStorylineRequest{UserID: "alice", PostSlug: "post", WriteModel: writeRef.String()}); err != nil {
			t.Fatal(err)
		}
		return svc, jobs.storylinePayloads[0]
	}
	bare := storylinePost()
	bare.Images = nil
	run := func(svc *Service, payload []byte) error {
		return svc.WriteStoryline(context.Background(), StorylineJob{UserID: "alice", PostSlug: "post", WriteModel: writeRef.String(), Payload: payload}, func(string, int, int) {})
	}

	posts := &fakePosts{input: bare}
	models := storylineModels(`{"storyline":[{"text":"메모만으로 짭니다.","files":[]}]}`)
	svc, payload := start(posts, models)
	posts.input.Published = true
	if err := run(svc, payload); !errors.Is(err, ErrPostPublished) || len(models.calls) != 0 {
		t.Fatalf("published at the run: %v, %d calls", err, len(models.calls))
	}

	posts = &fakePosts{input: bare, storylineErr: ErrPostPublished}
	svc, payload = start(posts, storylineModels(`{"storyline":[{"text":"메모만으로 짭니다.","files":[]}]}`))
	if err := run(svc, payload); !errors.Is(err, ErrPostPublished) {
		t.Fatalf("published at the write: %v", err)
	}

	for name, answer := range map[string]string{
		"not json":       "스토리라인입니다",
		"no storyline":   `{"title":"제목"}`,
		"all paragraphs": `{"storyline":[{"text":"  ","files":["ghost.jpg"]}]}`,
	} {
		posts = &fakePosts{input: bare}
		svc, payload = start(posts, storylineModels(answer))
		if err := run(svc, payload); !errors.Is(err, llm.ErrBadOutput) || len(posts.storylines) != 0 {
			t.Errorf("%s: err = %v, stored %d", name, err, len(posts.storylines))
		}
	}
}

// GEN-69, GUIDE-7: the request rewrites the stored storyline from the frozen material, shows the
// current storyline and the request, keeps what it was made with, observes nothing and records
// no guideline candidate.
func TestReviseStorylineRewritesTheStoredStoryline(t *testing.T) {
	input := storylinePost()
	input.Storyline = &Storyline{Paragraphs: []StorylineParagraph{{Text: "가게 앞", Files: []string{"IMG_1.jpg", "IMG_2.jpg"}}}, MadeWith: []string{"IMG_1.jpg", "IMG_2.jpg"}}
	input.Observations = []Observation{{File: "IMG_1.jpg", Scene: "가게"}, {File: "IMG_2.jpg", Scene: "커피"}}
	posts := &fakePosts{input: input}
	jobs := &fakeJobs{id: "job"}
	models := storylineModels(storylineAnswer)
	candidates := &fakeCandidates{}
	deps := testDeps()
	deps.Candidates = candidates
	svc := storylineService(posts, jobs, models, deps)
	if _, err := svc.StartStorylineRevision(context.Background(), StartStorylineRevisionRequest{
		UserID: "alice", PostSlug: "post", Request: "커피를 따로 떼어 주세요", WriteModel: writeRef.String(),
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.ReviseStoryline(context.Background(), StorylineRevisionJob{
		UserID: "alice", PostSlug: "post", WriteModel: writeRef.String(), Payload: jobs.storylineRequestPayloads[0],
	}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	if len(models.calls) != 1 || models.calls[0].request.HasImages() {
		t.Fatalf("calls = %d", len(models.calls))
	}
	call := models.calls[0].request
	user := call.Messages[0].Parts[0].Text
	if !strings.Contains(call.System, koreanStorylineRequestRule) ||
		!strings.Contains(user, "[현재 스토리라인]\n{\"storyline\":[{\"text\":\"가게 앞\",\"files\":[\"IMG_1.jpg\",\"IMG_2.jpg\"]}]}") ||
		!strings.HasSuffix(user, "[수정 요청]\n커피를 따로 떼어 주세요") {
		t.Fatalf("request prompt:\n%s\n---\n%s", call.System, user)
	}
	want := Storyline{
		Paragraphs: []StorylineParagraph{{Text: "가게 앞을 보여줍니다.", Files: []string{"IMG_1.jpg"}}, {Text: "커피를 이야기합니다.", Files: []string{"IMG_2.jpg"}}},
		MadeWith:   []string{"IMG_1.jpg", "IMG_2.jpg"},
	}
	if len(posts.storylines) != 1 || !reflect.DeepEqual(posts.storylines[0], want) {
		t.Fatalf("stored %+v", posts.storylines)
	}
	if len(posts.observationWrites) != 0 || len(posts.contents) != 0 || len(candidates.recorded) != 0 {
		t.Fatalf("observations %d, contents %d, candidates %d", len(posts.observationWrites), len(posts.contents), len(candidates.recorded))
	}
}

// GEN-68: the storyline is the whole answer, so a missing or empty one is bad output; the
// paragraphs follow the write's bounds.
func TestParseStorylineAnswer(t *testing.T) {
	got, err := ParseStorylineAnswer("```json\n"+storylineAnswer+"\n```", []string{"IMG_1.jpg", "IMG_2.jpg"})
	if err != nil || len(got) != 2 || !reflect.DeepEqual(got[0].Files, []string{"IMG_1.jpg"}) {
		t.Fatalf("parsed %+v, %v", got, err)
	}
	for _, raw := range []string{`{}`, `{"storyline":null}`, `{"storyline":"문단"}`, `{"storyline":[]}`} {
		if _, err := ParseStorylineAnswer(raw, nil); !errors.Is(err, llm.ErrBadOutput) {
			t.Errorf("%s: err = %v, want bad output", raw, err)
		}
	}
}

func storylineGoldenInput() StorylinePromptInput {
	return StorylinePromptInput{
		Language: LanguageKorean, Title: "가제 TITLE", Memo: "MEMO 본문",
		Photos: []string{"IMG_1.jpg"}, Observations: goldenObservations(),
		Template: testBrief(), DefaultGuidelines: []string{"메모의 이름으로 쓰세요"}, Guidelines: []string{"CCTV를 언급하지 않기"},
		Memories: []string{"매운 음식을 못 먹는다"},
	}
}

// GEN-68, GEN-69: both prompts, pinned. Set STORYLINE_GOLDEN_REGEN=1 to rewrite them after an
// accepted wording change.
func TestStorylinePromptsMatchTheirGoldens(t *testing.T) {
	request := storylineGoldenInput()
	request.Current = []StorylineParagraph{{Text: "가게 앞을 보여줍니다.", Files: []string{"IMG_1.jpg"}}}
	request.Request = "둘째 문단을 줄여 주세요"
	english := storylineGoldenInput()
	english.Language = LanguageEnglish
	for name, input := range map[string]StorylinePromptInput{
		"storyline_prompt.golden":         storylineGoldenInput(),
		"storyline_request_prompt.golden": request,
		"storyline_prompt_english.golden": english,
	} {
		system, user := BuildStorylinePromptForLanguage(input)
		if os.Getenv("STORYLINE_GOLDEN_REGEN") == "1" {
			if err := os.WriteFile("testdata/"+name, []byte(system+"\n@@USER@@\n"+user+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		wantSystem, wantUser := loadGolden(t, name)
		if system != wantSystem || user != wantUser {
			t.Errorf("%s drifted:\n--- system ---\n%s\n--- user ---\n%s", name, system, user)
		}
	}
}

// GEN-68: the storyline prompt names no voice, orders its sections as the write does, and
// closes [작문 지침] with the storyline's own precedence sentence.
func TestTheStorylinePromptHasNoVoiceAndItsOwnPrecedence(t *testing.T) {
	system, user := BuildStorylinePromptForLanguage(storylineGoldenInput())
	for _, gone := range []string{"말투 프로필", "[스타일가이드]", "JSON의 title", guidelinePrecedence} {
		if strings.Contains(system, gone) {
			t.Errorf("the storyline prompt carries %q", gone)
		}
	}
	template := strings.Index(system, "[글 템플릿:")
	guidelines := strings.Index(system, "[작문 지침]")
	if template < 0 || guidelines < template || !strings.HasSuffix(system, storylineGuidelinePrecedence) {
		t.Fatalf("section order or closing:\n%s", system)
	}
	if !strings.HasPrefix(user, "[이번 글]\n가제: 가제 TITLE\n메모: MEMO 본문\n[기억]") || !strings.Contains(user, "첨부 파일명(정확히 일치해야 함): IMG_1.jpg") {
		t.Fatalf("user half:\n%s", user)
	}
	english, _ := BuildStorylinePromptForLanguage(StorylinePromptInput{Language: LanguageEnglish})
	if !strings.HasPrefix(english, englishStorylineTask) || strings.Contains(english, "Then write the post") {
		t.Fatalf("english storyline prompt:\n%s", english)
	}
}

func alongStorylinePost() PostInput {
	input := storylinePost()
	input.Images = append(input.Images, Image{Filename: "IMG_3.jpg", Key: "k3"})
	// IMG_1 has a reusable observation; IMG_2 has none; IMG_3 is not in the storyline at all.
	input.Observations = []Observation{{File: "IMG_1.jpg", Scene: "가게", Model: observeRef.String()}, {File: "IMG_3.jpg", Scene: "간판", Model: observeRef.String()}}
	input.Storyline = &Storyline{
		Paragraphs: []StorylineParagraph{{Text: "가게 앞을 보여줍니다.", Files: []string{"IMG_1.jpg"}}, {Text: "커피를 이야기합니다.", Files: []string{"IMG_2.jpg"}}},
		MadeWith:   []string{"IMG_1.jpg", "IMG_2.jpg", "IMG_3.jpg"},
	}
	return input
}

// GEN-70: a run along the storyline needs one, takes no picker, freezes the paragraphs, and
// observes exactly the held attachments with no reusable observation.
func TestAFromStorylineStartFreezesTheStorylineAndItsObserveSet(t *testing.T) {
	jobs := &fakeJobs{id: "job"}
	posts := &fakePosts{input: alongStorylinePost()}
	svc := storylineService(posts, jobs, storylineModels(storylineAnswer), testDeps())
	files := []string{"IMG_3.jpg"}
	if _, err := svc.Start(context.Background(), StartRequest{
		UserID: "alice", PostSlug: "post", ObserveModel: observeRef.String(), WriteModel: writeRef.String(),
		FromStoryline: true, ObserveFiles: &files,
	}); !errors.Is(err, ErrStorylineReobserve) || posts.reads != 0 {
		t.Fatalf("a picker beside the storyline: %v (%d reads)", err, posts.reads)
	}
	bare := storylinePost()
	if _, err := storylineService(&fakePosts{input: bare}, jobs, storylineModels(storylineAnswer), testDeps()).Start(context.Background(), StartRequest{
		UserID: "alice", PostSlug: "post", ObserveModel: observeRef.String(), WriteModel: writeRef.String(), FromStoryline: true,
	}); !errors.Is(err, ErrStorylineMissing) {
		t.Fatalf("a post without a storyline: %v", err)
	}

	if _, err := svc.Start(context.Background(), StartRequest{
		UserID: "alice", PostSlug: "post", ObserveModel: observeRef.String(), WriteModel: writeRef.String(), FromStoryline: true,
	}); err != nil {
		t.Fatal(err)
	}
	options, err := decodeGenerationPayload(jobs.generatePayloads[0])
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(options.FollowStoryline, posts.input.Storyline.Paragraphs) {
		t.Fatalf("followed = %+v", options.FollowStoryline)
	}
	if options.ObserveFiles == nil || !reflect.DeepEqual(*options.ObserveFiles, []string{"IMG_2.jpg"}) || len(options.Observations) != 2 {
		t.Fatalf("observe set = %v, snapshot %+v", options.ObserveFiles, options.Observations)
	}
	if jobs.generations[0].ObserveCalls != 1 {
		t.Fatalf("priced %d observe calls, want the one", jobs.generations[0].ObserveCalls)
	}
}

// GEN-70, GEN-2, GEN-71: the write is shown only what the storyline holds, an attachment it does
// not hold never reaches the prompt or the post, and the stored storyline is left as it was.
func TestAFromStorylineRunShowsOnlyTheHeldAttachments(t *testing.T) {
	jobs := &fakeJobs{id: "job"}
	posts := &fakePosts{input: alongStorylinePost()}
	models := storylineModels(`{"title":"성수 카페","summary":"s","tags":["a"],"nouns":["카페"],"blocks":[` +
		`{"type":"TEXT","content":"가게 앞"},{"type":"IMAGE","file":"IMG_1.jpg"},{"type":"IMAGE","file":"IMG_3.jpg"},{"type":"IMAGE","file":"IMG_2.jpg"}]}`)
	svc := storylineService(posts, jobs, models, testDeps())
	if _, err := svc.Start(context.Background(), StartRequest{
		UserID: "alice", PostSlug: "post", ObserveModel: observeRef.String(), WriteModel: writeRef.String(), FromStoryline: true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Generate(context.Background(), GenerateJob{
		UserID: "alice", PostSlug: "post", VoiceID: liveVoice.ID, ObserveModel: observeRef.String(), WriteModel: writeRef.String(),
		Payload: jobs.generatePayloads[0],
	}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	write := models.calls[len(models.calls)-1].request
	user := write.Messages[0].Parts[0].Text
	if strings.Contains(user, "IMG_3.jpg") || !strings.Contains(user, "IMG_1.jpg") || !strings.Contains(user, "IMG_2.jpg") {
		t.Fatalf("the write was shown:\n%s", user)
	}
	if !strings.HasSuffix(user, "[스토리라인]\n1. 가게 앞을 보여줍니다. (파일: IMG_1.jpg)\n2. 커피를 이야기합니다. (파일: IMG_2.jpg)") {
		t.Fatalf("the storyline section:\n%s", user)
	}
	if !bytes.Equal(write.JSONSchema, LegacyWriteAlongStorylineAnswerSchema()) || !strings.Contains(write.System, koreanWriteAlongStorylineRule) ||
		strings.Contains(write.System, koreanStorylineRule) || strings.Contains(write.System, `"storyline":`) {
		t.Fatalf("the storyline-path write asked:\n%s", write.System)
	}
	content := posts.contents[len(posts.contents)-1]
	for _, block := range content.Blocks {
		if block.File == "IMG_3.jpg" {
			t.Fatal("an attachment the storyline does not hold reached the post")
		}
	}
	annotations := posts.annotations[len(posts.annotations)-1]
	if annotations == nil || annotations.Storyline != nil || !reflect.DeepEqual(annotations.Nouns, []string{"카페"}) {
		t.Fatalf("annotations = %+v, want the nouns and the storyline kept", annotations)
	}
}

// GEN-70: the storyline path differs from the direct write only where it asks for no storyline
// and follows [스토리라인]; its prompt is pinned.
func TestTheStorylinePathRulesAndPrompt(t *testing.T) {
	for name, pair := range map[string][2]string{
		"Korean": {WritePrompt, writeAlongStorylinePrompt}, "English": {englishWritePrompt, englishWriteAlongStorylinePrompt},
	} {
		direct, along := pair[0], pair[1]
		if along == direct || strings.Contains(along, `"storyline":`) || strings.Count(along, "스토리라인]") < 1 {
			t.Errorf("%s: the storyline-path rules:\n%s", name, along)
		}
	}
	input := WritePromptInput{
		Language: LanguageKorean, Profile: goldenProfile(), Observations: goldenObservations(), Memo: "MEMO 본문", Title: "가제 TITLE",
		Photos: []string{"IMG_1.jpg"}, TagCount: 4,
		FollowStoryline: []StorylineParagraph{{Text: "바다를\n보여줍니다.", Files: []string{"IMG_1.jpg"}}, {Text: "마무리합니다."}},
	}
	system, user := BuildWritePromptForLanguage(input)
	if os.Getenv("STORYLINE_GOLDEN_REGEN") == "1" {
		if err := os.WriteFile("testdata/write_prompt_along_storyline.golden", []byte(system+"\n@@USER@@\n"+user+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	wantSystem, wantUser := loadGolden(t, "write_prompt_along_storyline.golden")
	if system != wantSystem || user != wantUser {
		t.Errorf("the storyline-path prompt drifted:\n--- system ---\n%s\n--- user ---\n%s", system, user)
	}
}
