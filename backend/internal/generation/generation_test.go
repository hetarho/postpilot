package generation

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/llm"
)

var (
	observeRef = llm.ModelRef{ProviderID: "provider", ModelID: "observe"}
	writeRef   = llm.ModelRef{ProviderID: "provider", ModelID: "write"}
)

func TestValidateBlocks(t *testing.T) {
	input := []Block{
		{Type: BlockText, Content: "valid"},
		{Type: BlockHeading, Content: "heading", Level: 99},
		{Type: BlockImage, File: "IMG_1.jpg", Alt: "alt", Caption: "caption"},
		{Type: BlockQuote, Content: "quote"},
		{Type: BlockList, Items: []string{"one"}},
		{Type: BlockText, Content: "bad", Items: []string{"not allowed"}},
		{Type: "VIDEO", Content: "unknown"},
		{Type: BlockText},
		{Type: BlockList, Items: []string{""}},
	}
	got := ValidateBlocks(input)
	if len(got) != 5 {
		t.Fatalf("got %d valid blocks, want 5: %+v", len(got), got)
	}
	if got[1].Level != 2 {
		t.Errorf("heading level = %d, want clamped 2", got[1].Level)
	}
}

func TestFilterAttachmentsIsExact(t *testing.T) {
	content := PostContent{Blocks: []Block{
		{Type: BlockText, Content: "keep"},
		{Type: BlockImage, File: "IMG_1.jpg"},
		{Type: BlockImage, File: "img_1.jpg"},
		{Type: BlockImage, File: "IMG_999.jpg"},
	}}
	got := FilterAttachments(content, []string{"IMG_1.jpg"})
	if len(got.Blocks) != 2 || got.Blocks[1].File != "IMG_1.jpg" {
		t.Fatalf("filtered blocks = %+v", got.Blocks)
	}
}

func TestParseContentFallbacksAndBadOutput(t *testing.T) {
	plain := `{"title":"제목","summary":"요약","tags":["a","b","c"],"blocks":[{"type":"TEXT","content":"본문"}]}`
	for name, raw := range map[string]string{
		"plain":         plain,
		"json fence":    "```json\n" + plain + "\n```",
		"bare fence":    "```\n" + plain + "\n```",
		"prose wrapper": "결과입니다.\n" + plain + "\n끝",
	} {
		t.Run(name, func(t *testing.T) {
			content, err := ParseContent(raw)
			if err != nil || content.Title != "제목" || len(content.Blocks) != 1 {
				t.Fatalf("content=%+v err=%v", content, err)
			}
		})
	}
	raw := strings.Repeat("가", BadOutputErrorHeadChars+20)
	_, err := ParseContent(raw)
	var bad *ErrBadOutput
	if !errors.As(err, &bad) {
		t.Fatalf("err = %v, want ErrBadOutput", err)
	}
	if !strings.HasPrefix(err.Error(), badOutputPrefix) || len([]rune(bad.Head)) != BadOutputErrorHeadChars {
		t.Fatalf("bad output error = %q", err)
	}
	if _, err := ParseContent(`{"title":"missing required fields"}`); err == nil {
		t.Fatal("schema-incomplete content was accepted")
	}
}

func TestBuildWritePromptOrderAndRules(t *testing.T) {
	system, user := BuildWritePrompt(Profile{
		Styleguide: "STYLE", Excerpts: []string{"EXCERPT-1", "EXCERPT-2"}, Rules: "RULES",
	}, []Observation{{File: "IMG_1.jpg", Scene: "바다"}}, "MEMO", "TITLE", []string{"IMG_1.jpg", "IMG_2.jpg"})
	positions := []int{
		strings.Index(system, "STYLE"), strings.Index(system, "EXCERPT-1"),
		strings.Index(system, "EXCERPT-2"), strings.Index(system, "RULES"),
	}
	for i := 1; i < len(positions); i++ {
		if positions[i-1] < 0 || positions[i] <= positions[i-1] {
			t.Fatalf("profile order wrong: %v\n%s", positions, system)
		}
	}
	for _, required := range []string{"하나의 문단마다 TEXT 블록 하나", "목록에 없는 이미지를 절대", "3–6개"} {
		if !strings.Contains(system, required) {
			t.Errorf("system prompt missing %q", required)
		}
	}
	for _, required := range []string{"TITLE", "MEMO", "IMG_1.jpg, IMG_2.jpg"} {
		if !strings.Contains(user, required) {
			t.Errorf("user prompt missing %q", required)
		}
	}
	if !strings.Contains(user, `"visible_text"`) || strings.Contains(user, `"VisibleText"`) {
		t.Fatalf("observation JSON is not model-facing: %s", user)
	}
}

func TestObserveBatchesIncrementallyAndMatchesFilenames(t *testing.T) {
	post := PostInput{Slug: "post", UserID: "alice"}
	for i := 1; i <= 9; i++ {
		post.Images = append(post.Images, Image{Filename: fmt.Sprintf("IMG_%d.jpg", i), Key: fmt.Sprintf("key-%d", i)})
	}
	posts := &fakePosts{input: post}
	models := newFakeModels()
	models.complete = func(_ llm.ModelRef, request llm.Request) (llm.Response, error) {
		files := strings.TrimPrefix(request.Messages[0].Parts[len(request.Messages[0].Parts)-1].Text, "files: ")
		parts := strings.Split(files, ", ")
		items := make([]string, 0, len(parts)+1)
		for _, file := range parts {
			if file == "IMG_2.jpg" {
				continue
			}
			items = append(items, fmt.Sprintf(`{"file":%q,"scene":"seen","mood":"","visible_text":"","objects":[],"people_present":false}`, file))
		}
		items = append(items, `{"file":"NOT_ATTACHED.jpg","scene":"extra","mood":"","visible_text":"","objects":[],"people_present":false}`)
		return llm.Response{Text: `{"observations":[` + strings.Join(items, ",") + `]}`}, nil
	}
	svc := NewService(posts, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, &fakeJobs{}, 4)
	var progress []string
	got, err := svc.observe(context.Background(), post, observeRef, func(stage string, done, total int) {
		progress = append(progress, fmt.Sprintf("%s:%d/%d", stage, done, total))
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(models.calls) != 3 {
		t.Fatalf("observation calls = %d, want 3", len(models.calls))
	}
	if !reflect.DeepEqual(progress, []string{"observe:4/9", "observe:8/9", "observe:9/9"}) {
		t.Fatalf("progress = %v", progress)
	}
	if len(posts.observationWrites) != 3 || len(posts.observationWrites[0]) != 4 || len(posts.observationWrites[1]) != 8 || len(posts.observationWrites[2]) != 9 {
		t.Fatalf("incremental writes = %#v", posts.observationWrites)
	}
	if len(got) != 9 || got[1].File != "IMG_2.jpg" || got[1].Scene != "" {
		t.Fatalf("matched observations = %+v", got)
	}
	for _, call := range models.calls {
		if call.request.JSONSchema == nil {
			t.Error("structured observe model did not receive schema")
		}
	}
}

func TestGenerateWithNoPhotosSkipsObserveAndPersistsReviewInput(t *testing.T) {
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Title: "가제", Memo: "메모"}}
	models := newFakeModels()
	models.complete = func(_ llm.ModelRef, request llm.Request) (llm.Response, error) {
		if request.HasImages() {
			t.Fatal("zero-photo generation made an observation call")
		}
		if !strings.Contains(request.Messages[0].Parts[0].Text, "첨부 사진이 없습니다") {
			t.Error("no-photo prompt missing")
		}
		return llm.Response{Text: `{"title":"완성","summary":"요약","tags":["a","b","c"],"blocks":[{"type":"TEXT","content":"본문"}]}`}, nil
	}
	svc := NewService(posts, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, &fakeJobs{}, 4)
	var progress []string
	err := svc.Generate(context.Background(), GenerateJob{UserID: "alice", PostSlug: "post", WriteModel: writeRef.String()}, func(stage string, done, total int) {
		progress = append(progress, fmt.Sprintf("%s:%d/%d", stage, done, total))
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(models.calls) != 1 || len(posts.contents) != 1 || len(posts.contents[0].Blocks) != 1 {
		t.Fatalf("calls=%d contents=%+v", len(models.calls), posts.contents)
	}
	if len(posts.observationWrites) != 1 || len(posts.observationWrites[0]) != 0 {
		t.Fatalf("observations not cleared: %#v", posts.observationWrites)
	}
	if !reflect.DeepEqual(progress, []string{"observe:0/0", "write:0/1", "write:1/1"}) {
		t.Fatalf("progress=%v", progress)
	}
}

func TestWriteStructuredAndPlainFallback(t *testing.T) {
	for _, structured := range []bool{true, false} {
		t.Run(fmt.Sprintf("structured=%v", structured), func(t *testing.T) {
			models := newFakeModels()
			info := models.infos[writeRef]
			info.StructuredOutput = structured
			models.infos[writeRef] = info
			models.complete = func(_ llm.ModelRef, request llm.Request) (llm.Response, error) {
				if (request.JSONSchema != nil) != structured {
					t.Fatalf("schema present = %v", request.JSONSchema != nil)
				}
				raw := `{"title":"t","summary":"s","tags":["a","b","c"],"blocks":[{"type":"TEXT","content":"ok"}]}`
				if !structured {
					raw = "```json\n" + raw + "\n```"
				}
				return llm.Response{Text: raw}, nil
			}
			svc := NewService(&fakePosts{}, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, &fakeJobs{}, 4)
			content, err := svc.write(context.Background(), PostInput{UserID: "alice"}, nil, writeRef)
			if err != nil || len(content.Blocks) != 1 {
				t.Fatalf("content=%+v err=%v", content, err)
			}
		})
	}
}

func TestQueuedZeroPhotoGenerationIgnoresPhotosAttachedAfterStart(t *testing.T) {
	posts := &fakePosts{input: PostInput{
		Slug: "post", UserID: "alice", Memo: "memo",
		Images: []Image{{Filename: "late.jpg", Key: "late-key"}},
	}}
	models := newFakeModels()
	models.complete = func(_ llm.ModelRef, request llm.Request) (llm.Response, error) {
		if request.HasImages() || !strings.Contains(request.Messages[0].Parts[0].Text, "첨부 사진이 없습니다") {
			t.Fatalf("late photo entered zero-photo job: %+v", request)
		}
		return llm.Response{Text: `{"title":"t","summary":"s","tags":["a","b","c"],"blocks":[{"type":"TEXT","content":"ok"}]}`}, nil
	}
	err := NewService(posts, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, &fakeJobs{}, 4).Generate(
		context.Background(), GenerateJob{UserID: "alice", PostSlug: "post", WriteModel: writeRef.String()},
		func(string, int, int) {},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(models.calls) != 1 {
		t.Fatalf("provider calls = %d, want write only", len(models.calls))
	}
}

func TestProviderTimeoutHasClearStageReason(t *testing.T) {
	models := newFakeModels()
	models.complete = func(llm.ModelRef, llm.Request) (llm.Response, error) {
		return llm.Response{}, context.DeadlineExceeded
	}
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice"}}
	err := NewService(posts, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, &fakeJobs{}, 4).Generate(
		context.Background(), GenerateJob{UserID: "alice", PostSlug: "post", WriteModel: writeRef.String()},
		func(string, int, int) {},
	)
	if err == nil || !strings.Contains(err.Error(), "글 작성 모델 호출 시간이 초과됐어요") {
		t.Fatalf("timeout error = %v", err)
	}
}

func TestStartGenerationPreconditionsAndEnqueueOnly(t *testing.T) {
	basePost := PostInput{Slug: "post", UserID: "alice", Images: []Image{{Filename: "a.jpg"}}}
	for name, tc := range map[string]struct {
		mutate  func(*StartRequest, *fakePosts, *fakeModels)
		wantErr error
	}{
		"write required":               {func(r *StartRequest, _ *fakePosts, _ *fakeModels) { r.WriteModel = "" }, ErrWriteModelRequired},
		"observe required with photos": {func(r *StartRequest, _ *fakePosts, _ *fakeModels) { r.ObserveModel = "" }, ErrObserveModelRequired},
		"observe must see": {func(_ *StartRequest, _ *fakePosts, m *fakeModels) {
			info := m.infos[observeRef]
			info.Vision = false
			m.infos[observeRef] = info
		}, ErrObserveModelRequired},
		"zero photos ignores observe": {func(r *StartRequest, p *fakePosts, _ *fakeModels) { p.input.Images = nil; r.ObserveModel = "" }, nil},
	} {
		t.Run(name, func(t *testing.T) {
			posts := &fakePosts{input: basePost}
			models := newFakeModels()
			jobs := &fakeJobs{id: "job-1"}
			request := StartRequest{UserID: "alice", PostSlug: "post", ObserveModel: observeRef.String(), WriteModel: writeRef.String()}
			tc.mutate(&request, posts, models)
			svc := NewService(posts, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, jobs, 4)
			id, err := svc.Start(context.Background(), request)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("id=%q err=%v, want %v", id, err, tc.wantErr)
			}
			wantEnqueues := 1
			if tc.wantErr != nil {
				wantEnqueues = 0
			}
			if jobs.enqueues != wantEnqueues {
				t.Fatalf("enqueues=%d, want %d", jobs.enqueues, wantEnqueues)
			}
			if len(models.calls) != 0 {
				t.Fatal("Start called a provider")
			}
		})
	}

	posts, models := &fakePosts{input: basePost}, newFakeModels()
	jobs := &fakeJobs{err: &JobAlreadyInProgressError{ActiveID: "active"}}
	_, err := NewService(posts, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, jobs, 4).Start(context.Background(), StartRequest{
		UserID: "alice", PostSlug: "post", ObserveModel: observeRef.String(), WriteModel: writeRef.String(),
	})
	var active *JobAlreadyInProgressError
	if !errors.As(err, &active) || active.ActiveID != "active" {
		t.Fatalf("active err=%v", err)
	}
}

type fakePosts struct {
	input             PostInput
	err               error
	reads             int
	observationWrites [][]Observation
	contents          []PostContent
}

func (f *fakePosts) AttachedImages(context.Context, string, string) (PostInput, error) {
	f.reads++
	return f.input, f.err
}
func (f *fakePosts) SetObservations(_ context.Context, _, _ string, values []Observation) error {
	f.observationWrites = append(f.observationWrites, append([]Observation(nil), values...))
	return nil
}
func (f *fakePosts) SetGeneratedContent(_ context.Context, _, _ string, value PostContent) error {
	f.contents = append(f.contents, value)
	f.input.Content = &value
	return nil
}

type fakeProfiles struct {
	profile Profile
	calls   int
}

func (f fakeProfiles) ProfileForPrompt(context.Context, string) (Profile, error) {
	return f.profile, nil
}

type fakeRules struct {
	lines []string
	err   error
}

func (f *fakeRules) AppendRule(_ context.Context, _ string, line string) error {
	f.lines = append(f.lines, line)
	return f.err
}

type fakeImages struct{}

func (fakeImages) Read(_ context.Context, key string) ([]byte, error) { return []byte(key), nil }

type modelCall struct {
	ref     llm.ModelRef
	request llm.Request
}
type fakeModels struct {
	infos    map[llm.ModelRef]llm.ModelInfo
	calls    []modelCall
	complete func(llm.ModelRef, llm.Request) (llm.Response, error)
}

func newFakeModels() *fakeModels {
	return &fakeModels{infos: map[llm.ModelRef]llm.ModelInfo{
		observeRef: {Ref: observeRef, Vision: true, StructuredOutput: true},
		writeRef:   {Ref: writeRef, StructuredOutput: true},
	}}
}
func (f *fakeModels) Resolve(ref llm.ModelRef) (llm.ModelInfo, bool) {
	info, ok := f.infos[ref]
	return info, ok
}
func (f *fakeModels) Complete(_ context.Context, ref llm.ModelRef, request llm.Request) (llm.Response, error) {
	f.calls = append(f.calls, modelCall{ref: ref, request: request})
	return f.complete(ref, request)
}

type fakeJobs struct {
	id        string
	err       error
	enqueues  int
	revisions []StartRevisionRequest
	payloads  [][]byte
}

func (f *fakeJobs) EnqueueGeneration(context.Context, StartRequest) (string, error) {
	f.enqueues++
	return f.id, f.err
}
func (f *fakeJobs) EnqueueRevision(_ context.Context, request StartRevisionRequest, payload []byte) (string, error) {
	f.enqueues++
	f.revisions = append(f.revisions, request)
	f.payloads = append(f.payloads, append([]byte(nil), payload...))
	return f.id, f.err
}
func (f fakeJobs) GetGeneration(context.Context, string, string) (*JobSummary, error) {
	return nil, ErrNotFound
}
