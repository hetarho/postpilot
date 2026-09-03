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
	observeRef          = llm.ModelRef{ProviderID: "provider", ModelID: "observe"}
	writeRef            = llm.ModelRef{ProviderID: "provider", ModelID: "write"}
	writeRefB           = llm.ModelRef{ProviderID: "provider", ModelID: "write-b"}
	testReasoningPolicy = ReasoningPolicy{Observe: llm.ReasoningLow, Write: llm.ReasoningLow}
	// liveVoice is the post's active voice in every fixture; voice_test.go covers the
	// deleted and reassigned cases.
	liveVoice = VoiceRef{ID: "voice-live", Name: "기본 말투", SourceLanguage: LanguageKorean}
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
	if !errors.Is(err, llm.ErrBadOutput) {
		t.Fatalf("bad output must normalize to llm.ErrBadOutput: %v", err)
	}
	if _, err := ParseContent(`{"title":"missing required fields"}`); err == nil {
		t.Fatal("schema-incomplete content was accepted")
	}
}

func TestBuildWritePromptOrderAndRules(t *testing.T) {
	system, user := BuildWritePrompt(Profile{
		Styleguide: "STYLE", ActiveRules: "ACTIVE", Excerpts: []string{"EXCERPT-1", "EXCERPT-2"}, Rules: "RULES",
	}, []Observation{{File: "IMG_1.jpg", Scene: "바다"}}, "MEMO", "TITLE", []string{"IMG_1.jpg", "IMG_2.jpg"}, nil, nil, nil)
	positions := []int{
		strings.Index(system, "STYLE"), strings.Index(system, "ACTIVE"), strings.Index(system, "EXCERPT-1"),
		strings.Index(system, "EXCERPT-2"), strings.Index(system, "RULES"),
	}
	for i := 1; i < len(positions); i++ {
		if positions[i-1] < 0 || positions[i] <= positions[i-1] {
			t.Fatalf("profile order wrong: %v\n%s", positions, system)
		}
	}
	for _, required := range []string{"하나의 문단마다 TEXT 블록 하나", "목록에 없는 이미지를 절대", "3–6개", "고유 사실, 주제, 문구를 복사하지", "같은 종결어미를 2문장보다 많이"} {
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
	if strings.Contains(system, "목표 길이") || strings.Contains(system, "1200") {
		t.Fatalf("absent target leaked a numeric constraint: %s", system)
	}
	target := 777
	withTarget, _ := BuildWritePrompt(Profile{}, nil, "", "", nil, &target, nil, nil)
	if !strings.Contains(withTarget, "목표 길이: 약 777자") {
		t.Fatalf("configured target missing: %s", withTarget)
	}
}

func TestGenerationPayloadPreservesOptionalTargetPresence(t *testing.T) {
	for _, target := range []*int{nil, intPointer(100), intPointer(10_000)} {
		raw, err := EncodeGenerationPayload(GenerationOptions{TargetLanguage: LanguageKorean, TargetLength: target})
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := DecodeGenerationPayload(raw)
		if err != nil || (target == nil) != (decoded.TargetLength == nil) || target != nil && *target != *decoded.TargetLength {
			t.Fatalf("target=%v raw=%s decoded=%v err=%v", target, raw, decoded.TargetLength, err)
		}
		if decoded.Purpose != nil {
			t.Fatalf("absent purpose decoded as %+v", decoded.Purpose)
		}
	}
}

func intPointer(value int) *int { return &value }

func TestObserveBatchesIncrementallyAndMatchesFilenames(t *testing.T) {
	post := PostInput{Slug: "post", UserID: "alice", Voice: liveVoice}
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
	svc := NewService(posts, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, &fakeJobs{}, 4, testReasoningPolicy)
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
		if call.request.Reasoning != llm.ReasoningLow {
			t.Errorf("observe reasoning = %q, want low", call.request.Reasoning)
		}
	}
}

func TestReasoningPolicyAndFailedUsageReachExperimentCandidates(t *testing.T) {
	models := newFakeModels()
	models.complete = func(ref llm.ModelRef, _ llm.Request) (llm.Response, error) {
		if ref == writeRef {
			return llm.Response{Usage: llm.Usage{PromptTokens: 11, CompletionTokens: 7, CostMicrousd: 5, CostReported: true}}, errors.New("write failed after billing")
		}
		return llm.Response{Usage: llm.Usage{PromptTokens: 13, CompletionTokens: 3, CostMicrousd: 2, CostReported: true}}, errors.New("observe failed after billing")
	}
	svc := NewService(&fakePosts{}, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, &fakeJobs{}, 4, testReasoningPolicy)
	_, writeUsage, writeErr := svc.writeCandidate(context.Background(), PostInput{}, Profile{}, nil, writeRef)
	_, observeUsage, observeErr := svc.observeCandidate(context.Background(), PostInput{
		Images: []Image{{Filename: "IMG.jpg", Key: "key"}},
	}, observeRef, func(string, int, int) {}, false)
	if writeErr == nil || writeUsage.PromptTokens != 11 || writeUsage.CompletionTokens != 7 || !writeUsage.CostReported {
		t.Fatalf("write usage/error = %+v / %v", writeUsage, writeErr)
	}
	if observeErr == nil || observeUsage.PromptTokens != 13 || observeUsage.CompletionTokens != 3 || !observeUsage.CostReported {
		t.Fatalf("observe usage/error = %+v / %v", observeUsage, observeErr)
	}
	if len(models.calls) != 2 || models.calls[0].request.Reasoning != llm.ReasoningLow ||
		models.calls[1].request.Reasoning != llm.ReasoningLow {
		t.Fatalf("reasoning calls = %+v", models.calls)
	}
}

func TestLengthLimitedPartialJSONIsOutputTruncated(t *testing.T) {
	for _, test := range []struct {
		name string
		run  func(*Service) error
	}{
		{
			name: "write",
			run: func(svc *Service) error {
				_, _, err := svc.writeCandidate(context.Background(), PostInput{}, Profile{}, nil, writeRef)
				return err
			},
		},
		{
			name: "observe",
			run: func(svc *Service) error {
				_, _, err := svc.observeCandidate(context.Background(), PostInput{
					Images: []Image{{Filename: "IMG.jpg", Key: "key"}},
				}, observeRef, func(string, int, int) {}, false)
				return err
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			models := newFakeModels()
			models.complete = func(llm.ModelRef, llm.Request) (llm.Response, error) {
				return llm.Response{
					Text:         `{"partial":`,
					FinishReason: "length",
					Usage:        llm.Usage{PromptTokens: 11, CompletionTokens: 8192},
				}, nil
			}
			svc := NewService(&fakePosts{}, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, &fakeJobs{}, 4, testReasoningPolicy)
			err := test.run(svc)
			if !errors.Is(err, llm.ErrOutputTruncated) || errors.Is(err, llm.ErrBadOutput) {
				t.Fatalf("err = %v, want only ErrOutputTruncated", err)
			}
		})
	}
}

func TestGenerateWithNoPhotosSkipsObserveAndPersistsReviewInput(t *testing.T) {
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: liveVoice, Title: "가제", Memo: "메모"}}
	models := newFakeModels()
	models.complete = func(_ llm.ModelRef, request llm.Request) (llm.Response, error) {
		if request.HasImages() {
			t.Fatal("zero-photo generation made an observation call")
		}
		if !strings.Contains(request.System, NaturalnessBaseline) {
			t.Error("ordinary generation lost the naturalness baseline")
		}
		if !strings.Contains(request.Messages[0].Parts[0].Text, "첨부 사진이 없습니다") {
			t.Error("no-photo prompt missing")
		}
		return llm.Response{Text: `{"title":"완성","summary":"요약","tags":["a","b","c"],"blocks":[{"type":"TEXT","content":"본문"}]}`}, nil
	}
	svc := NewService(posts, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, &fakeJobs{}, 4, testReasoningPolicy)
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

func TestGenerateUsesFrozenTargetInsteadOfLaterPostOption(t *testing.T) {
	current := 1600
	frozen := 850
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: liveVoice, TargetLength: &current}}
	models := newFakeModels()
	models.complete = func(_ llm.ModelRef, request llm.Request) (llm.Response, error) {
		if !strings.Contains(request.System, "목표 길이: 약 850자") || strings.Contains(request.System, "1600") {
			t.Fatalf("prompt did not use frozen target: %s", request.System)
		}
		return llm.Response{Text: `{"title":"t","summary":"s","tags":["a","b","c"],"blocks":[{"type":"TEXT","content":"ok"}]}`}, nil
	}
	svc := NewService(posts, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, &fakeJobs{}, 4, testReasoningPolicy)
	if err := svc.Generate(context.Background(), GenerateJob{UserID: "alice", PostSlug: "post", WriteModel: writeRef.String(), TargetLength: &frozen}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
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
			svc := NewService(&fakePosts{}, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, &fakeJobs{}, 4, testReasoningPolicy)
			content, err := svc.write(context.Background(), PostInput{UserID: "alice", Voice: liveVoice, TargetLanguage: LanguageKorean}, nil, writeRef)
			if err != nil || len(content.Blocks) != 1 {
				t.Fatalf("content=%+v err=%v", content, err)
			}
		})
	}
}

func TestQueuedZeroPhotoGenerationIgnoresPhotosAttachedAfterStart(t *testing.T) {
	posts := &fakePosts{input: PostInput{
		Slug: "post", UserID: "alice", Voice: liveVoice, Memo: "memo",
		Images: []Image{{Filename: "late.jpg", Key: "late-key"}},
	}}
	models := newFakeModels()
	models.complete = func(_ llm.ModelRef, request llm.Request) (llm.Response, error) {
		if request.HasImages() || !strings.Contains(request.Messages[0].Parts[0].Text, "첨부 사진이 없습니다") {
			t.Fatalf("late photo entered zero-photo job: %+v", request)
		}
		return llm.Response{Text: `{"title":"t","summary":"s","tags":["a","b","c"],"blocks":[{"type":"TEXT","content":"ok"}]}`}, nil
	}
	err := NewService(posts, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, &fakeJobs{}, 4, testReasoningPolicy).Generate(
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
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: liveVoice}}
	err := NewService(posts, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, &fakeJobs{}, 4, testReasoningPolicy).Generate(
		context.Background(), GenerateJob{UserID: "alice", PostSlug: "post", WriteModel: writeRef.String()},
		func(string, int, int) {},
	)
	if err == nil || !strings.Contains(err.Error(), "글 작성 모델 호출 시간이 초과됐어요") {
		t.Fatalf("timeout error = %v", err)
	}
}

func TestStartGenerationPreconditionsAndEnqueueOnly(t *testing.T) {
	basePost := PostInput{Slug: "post", UserID: "alice", Voice: liveVoice, Images: []Image{{Filename: "a.jpg"}}}
	for name, tc := range map[string]struct {
		mutate  func(*StartRequest, *fakePosts, *fakeModels)
		wantErr error
	}{
		"write required":               {func(r *StartRequest, _ *fakePosts, _ *fakeModels) { r.WriteModel = "" }, ErrWriteModelRequired},
		"observe required with photos": {func(r *StartRequest, _ *fakePosts, _ *fakeModels) { r.ObserveModel = "" }, ErrObserveModelRequired},
		"observe must see": {func(_ *StartRequest, _ *fakePosts, m *fakeModels) {
			info := m.infos[observeRef]
			// Losing vision reaches this boundary as lost observe membership: the catalog
			// drops the stage the moment the capability its registration was gated on goes.
			info.Stages = []string{llm.StageNameWrite, llm.StageNameAnalyze}
			m.infos[observeRef] = info
		}, ErrObserveModelRequired},
		"target must be positive": {func(r *StartRequest, _ *fakePosts, _ *fakeModels) {
			invalid := 0
			r.TargetLength = &invalid
		}, ErrInvalidTargetLength},
		"zero photos ignores observe": {func(r *StartRequest, p *fakePosts, _ *fakeModels) { p.input.Images = nil; r.ObserveModel = "" }, nil},
	} {
		t.Run(name, func(t *testing.T) {
			posts := &fakePosts{input: basePost}
			models := newFakeModels()
			jobs := &fakeJobs{id: "job-1"}
			request := StartRequest{UserID: "alice", PostSlug: "post", ObserveModel: observeRef.String(), WriteModel: writeRef.String()}
			tc.mutate(&request, posts, models)
			svc := NewService(posts, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, jobs, 4, testReasoningPolicy)
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
	_, err := NewService(posts, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, jobs, 4, testReasoningPolicy).Start(context.Background(), StartRequest{
		UserID: "alice", PostSlug: "post", ObserveModel: observeRef.String(), WriteModel: writeRef.String(),
	})
	var active *JobAlreadyInProgressError
	if !errors.As(err, &active) || active.ActiveID != "active" {
		t.Fatalf("active err=%v", err)
	}
}

func TestWriteExperimentUsesOnePreparedSnapshotAndDoesNotApplyBeforeChoice(t *testing.T) {
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: liveVoice, Title: "가제", Memo: "같은 메모"}}
	models := newFakeModels()
	models.infos[writeRefB] = llm.ModelInfo{Ref: writeRefB, StructuredOutput: true}
	models.complete = func(ref llm.ModelRef, _ llm.Request) (llm.Response, error) {
		return llm.Response{
			Text:  fmt.Sprintf(`{"title":%q,"summary":"요약","tags":["a","b","c"],"blocks":[{"type":"TEXT","content":"본문"}]}`, ref.ModelID),
			Usage: llm.Usage{PromptTokens: 10, CompletionTokens: 2, CostMicrousd: 3, CostReported: true},
		}, nil
	}
	svc := NewService(posts, fakeProfiles{profile: Profile{Styleguide: "말투"}}, &fakeRules{}, models, fakeImages{}, &fakeJobs{}, 4, testReasoningPolicy)
	raw, err := svc.SnapshotWriteInput(context.Background(), "alice", "post", llm.ModelRef{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := svc.PrepareWriteInput(context.Background(), raw, func(string, int, int) {})
	if err != nil {
		t.Fatal(err)
	}
	left, leftUsage, err := svc.RunWriteCandidate(context.Background(), prepared, writeRef)
	if err != nil {
		t.Fatal(err)
	}
	right, _, err := svc.RunWriteCandidate(context.Background(), prepared, writeRefB)
	if err != nil {
		t.Fatal(err)
	}
	if len(posts.contents) != 0 {
		t.Fatalf("candidate completion wrote canonical content: %+v", posts.contents)
	}
	if left.Title == right.Title || len(models.calls) != 2 || !reflect.DeepEqual(models.calls[0].request, models.calls[1].request) {
		t.Fatalf("writers did not receive equal request snapshots: left=%+v right=%+v calls=%+v", left, right, models.calls)
	}
	if !strings.Contains(models.calls[0].request.System, NaturalnessBaseline) {
		t.Fatal("write comparison candidates lost the naturalness baseline")
	}
	if !leftUsage.CostReported || leftUsage.CostMicrousd != 3 {
		t.Fatalf("candidate usage = %+v", leftUsage)
	}
	if err := svc.ApplyWriteWinner(context.Background(), "alice", "post", right, prepared); err != nil {
		t.Fatal(err)
	}
	if len(posts.contents) != 1 || posts.contents[0].Title != "write-b" {
		t.Fatalf("winner apply = %+v", posts.contents)
	}
}

func TestOrdinaryGenerationAndRevisionRefuseAnUnresolvedWriteExperiment(t *testing.T) {
	posts := &fakePosts{input: PostInput{
		Slug: "post", UserID: "alice", Voice: liveVoice, Content: revisionContent("existing"),
	}}
	jobs := &fakeJobs{id: "should-not-enqueue"}
	rules := &fakeRules{}
	svc := NewService(posts, fakeProfiles{}, rules, newFakeModels(), fakeImages{}, jobs, 4, testReasoningPolicy)
	svc.SetPendingExperimentFinder(fakePendingExperiments{id: "experiment-pending"})

	_, err := svc.Start(context.Background(), StartRequest{
		UserID: "alice", PostSlug: "post", WriteModel: writeRef.String(),
	})
	var active *JobAlreadyInProgressError
	if !errors.As(err, &active) || active.ActiveID != "experiment-pending" {
		t.Fatalf("ordinary generation error = %v", err)
	}
	_, err = svc.StartRevision(context.Background(), StartRevisionRequest{
		UserID: "alice", PostSlug: "post", Instruction: "더 짧게",
		WriteModel: writeRef.String(), SaveAsRule: true,
	})
	if !errors.As(err, &active) || active.ActiveID != "experiment-pending" {
		t.Fatalf("revision error = %v", err)
	}
	if jobs.enqueues != 0 || len(rules.lines) != 0 {
		t.Fatalf("blocked starts mutated state: jobs=%d rules=%v", jobs.enqueues, rules.lines)
	}
}

func TestWriteExperimentObservesPhotosExactlyOnceBeforeTwoWriters(t *testing.T) {
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: liveVoice, Memo: "memo", Images: []Image{{Filename: "IMG_1.jpg", Key: "key"}}}}
	models := newFakeModels()
	models.infos[writeRefB] = llm.ModelInfo{Ref: writeRefB, StructuredOutput: true}
	models.complete = func(_ llm.ModelRef, request llm.Request) (llm.Response, error) {
		if request.HasImages() {
			return llm.Response{Text: `{"observations":[{"file":"IMG_1.jpg","scene":"바다","mood":"","visible_text":"","objects":[],"people_present":false}]}`}, nil
		}
		return llm.Response{Text: `{"title":"글","summary":"요약","tags":["a","b","c"],"blocks":[{"type":"TEXT","content":"본문"}]}`}, nil
	}
	svc := NewService(posts, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, &fakeJobs{}, 4, testReasoningPolicy)
	raw, err := svc.SnapshotWriteInput(context.Background(), "alice", "post", observeRef, nil)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := svc.PrepareWriteInput(context.Background(), raw, func(string, int, int) {})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.RunWriteCandidate(context.Background(), prepared, writeRef); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.RunWriteCandidate(context.Background(), prepared, writeRefB); err != nil {
		t.Fatal(err)
	}
	imageCalls := 0
	for _, call := range models.calls {
		if call.request.HasImages() {
			imageCalls++
		}
	}
	if len(models.calls) != 3 || imageCalls != 1 || len(posts.observationWrites) != 1 || len(posts.contents) != 0 {
		t.Fatalf("calls=%d imageCalls=%d observationWrites=%d contents=%d", len(models.calls), imageCalls, len(posts.observationWrites), len(posts.contents))
	}
}

type fakePosts struct {
	input                    PostInput
	err                      error
	reads                    int
	observationWrites        [][]Observation
	contents                 []PostContent
	contentLanguages         []Language
	preserveMissingLanguages bool
}

func (f *fakePosts) AttachedImages(context.Context, string, string) (PostInput, error) {
	f.reads++
	value := f.input
	if value.TargetLanguage == "" && !f.preserveMissingLanguages {
		value.TargetLanguage = LanguageKorean
	}
	if value.Voice.SourceLanguage == "" && !f.preserveMissingLanguages {
		value.Voice.SourceLanguage = LanguageKorean
	}
	if value.Content != nil && value.ContentLanguage == nil && !f.preserveMissingLanguages {
		language := LanguageKorean
		value.ContentLanguage = &language
	}
	return value, f.err
}
func (f *fakePosts) SetObservations(_ context.Context, _, _ string, values []Observation) error {
	f.observationWrites = append(f.observationWrites, append([]Observation(nil), values...))
	return nil
}
func (f *fakePosts) SetGeneratedContent(_ context.Context, _, _ string, value PostContent, language Language) error {
	f.contents = append(f.contents, value)
	f.contentLanguages = append(f.contentLanguages, language)
	f.input.Content = &value
	f.input.ContentLanguage = &language
	return nil
}

type fakeProfiles struct {
	profile Profile
	calls   int
}

func (f fakeProfiles) ProfileForPrompt(_ context.Context, _, _ string, target Language) (Profile, error) {
	profile := f.profile
	if profile.TargetLanguage == "" {
		profile.TargetLanguage = target
	}
	if profile.SourceLanguage == "" {
		profile.SourceLanguage = target
	}
	return profile, nil
}

type fakeRules struct {
	lines  []string
	voices []string
	err    error
}

func (f *fakeRules) AppendRule(_ context.Context, _, voiceID string, line string) error {
	f.lines = append(f.lines, line)
	f.voices = append(f.voices, voiceID)
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
		observeRef: {Ref: observeRef, Vision: true, StructuredOutput: true, Stages: []string{llm.StageNameObserve, llm.StageNameWrite, llm.StageNameAnalyze}},
		writeRef:   {Ref: writeRef, StructuredOutput: true, Stages: []string{llm.StageNameWrite, llm.StageNameAnalyze}},
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
	id          string
	err         error
	enqueues    int
	revisions   []StartRevisionRequest
	payloads    [][]byte
	generations []StartRequest
}

func (f *fakeJobs) EnqueueGeneration(_ context.Context, request StartRequest) (string, error) {
	f.enqueues++
	f.generations = append(f.generations, request)
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

type fakePendingExperiments struct {
	id  string
	err error
}

func (f fakePendingExperiments) PendingForPost(context.Context, string, string) (string, error) {
	return f.id, f.err
}
