package generation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/llm"
)

func revisionContent(text string, blocks ...Block) *PostContent {
	return &PostContent{
		Title: "제목", Summary: "요약", Tags: []string{"a", "b", "c"},
		Blocks: append([]Block{{Type: BlockText, Content: text}}, blocks...),
	}
}

func mustRevisionPayload(t *testing.T, instruction string, save bool) []byte {
	t.Helper()
	payload, err := encodeRevisionPayload(instruction, save)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func TestBuildRevisePromptKeepsProfileFirstAndStatesMinimalChange(t *testing.T) {
	system, user := BuildRevisePrompt(Profile{
		Styleguide: "STYLE", ActiveRules: "ACTIVE", Excerpts: []string{"EXCERPT-1", "EXCERPT-2"}, Rules: "RULES", EndingMaxConsecutive: 2,
	}, *revisionContent("CURRENT"), []string{"IMG_1.jpg"}, "INSTRUCTION")
	whole := system + "\n" + user
	positions := []int{
		strings.Index(whole, "STYLE"), strings.Index(whole, "ACTIVE"), strings.Index(whole, "EXCERPT-1"),
		strings.Index(whole, "EXCERPT-2"), strings.Index(whole, "RULES"),
		strings.Index(whole, "CURRENT"), strings.Index(whole, "INSTRUCTION"),
	}
	for i := 1; i < len(positions); i++ {
		if positions[i-1] < 0 || positions[i] <= positions[i-1] {
			t.Fatalf("revision prompt order wrong: %v\n%s", positions, whole)
		}
	}
	for _, required := range []string{
		"요청과 무관한 문장은 글자 그대로", "제목, 한 줄 요약, 태그", "완전한 PostContent",
		"파일명을 바꾸거나 새 이미지를 만들지", "IMG_1.jpg", `"type":"TEXT"`,
		"고유 사실, 주제, 문구를 복사하지", "같은 종결어미를 2문장보다 많이",
	} {
		if !strings.Contains(whole, required) {
			t.Errorf("revision prompt missing %q", required)
		}
	}
	if strings.Contains(system, "목표 길이") || strings.Contains(system, "1200") {
		t.Fatalf("revision added a hidden length target: %s", system)
	}
	target := 940
	configured, _ := BuildRevisePrompt(Profile{}, *revisionContent("CURRENT"), nil, "INSTRUCTION", &target)
	if !strings.Contains(configured, "목표 길이: 약 940자") {
		t.Fatalf("configured revision target missing: %s", configured)
	}
}

type recordingProfiles struct {
	profile Profile
	calls   int
}

func (f *recordingProfiles) ProfileForPrompt(context.Context, string) (Profile, error) {
	f.calls++
	return f.profile, nil
}

func TestFiveRevisionsReinjectProfileAndPersistEveryResult(t *testing.T) {
	posts := &fakePosts{input: PostInput{
		Slug: "post", UserID: "alice", Content: revisionContent("pass-0"),
	}}
	profiles := &recordingProfiles{profile: Profile{
		Styleguide: "STYLE", Excerpts: []string{"EXCERPT"}, Rules: "RULE",
	}}
	models := newFakeModels()
	models.complete = func(_ llm.ModelRef, _ llm.Request) (llm.Response, error) {
		pass := len(models.calls)
		return llm.Response{Text: fmt.Sprintf(
			`{"title":"제목","summary":"요약","tags":["a","b","c"],"blocks":[{"type":"TEXT","content":"pass-%d"}]}`,
			pass,
		)}, nil
	}
	svc := NewService(posts, profiles, &fakeRules{}, models, fakeImages{}, &fakeJobs{}, 4)

	for pass := 1; pass <= 5; pass++ {
		instruction := fmt.Sprintf("INSTRUCTION-%d", pass)
		if err := svc.Revise(context.Background(), RevisionJob{
			UserID: "alice", PostSlug: "post", WriteModel: writeRef.String(),
			Payload: mustRevisionPayload(t, instruction, false),
		}, func(string, int, int) {}); err != nil {
			t.Fatalf("pass %d: %v", pass, err)
		}
		request := models.calls[pass-1].request
		whole := request.System + "\n" + request.Messages[0].Parts[0].Text
		for _, expected := range []string{"STYLE", "EXCERPT", "RULE", instruction} {
			if !strings.Contains(whole, expected) {
				t.Errorf("pass %d missing %q", pass, expected)
			}
		}
		if request.JSONSchema == nil {
			t.Errorf("pass %d did not request structured output", pass)
		}
	}
	if profiles.calls != 5 || len(posts.contents) != 5 {
		t.Fatalf("profile calls=%d content writes=%d", profiles.calls, len(posts.contents))
	}
}

func TestRevisionUsesSharedValidationAndAttachmentFilterAndKeepsImageOrder(t *testing.T) {
	posts := &fakePosts{input: PostInput{
		Slug: "post", UserID: "alice", Content: revisionContent("before"),
		Images: []Image{{Filename: "A.jpg"}, {Filename: "B.jpg"}},
	}}
	models := newFakeModels()
	models.complete = func(_ llm.ModelRef, _ llm.Request) (llm.Response, error) {
		return llm.Response{Text: `{"title":"제목","summary":"요약","tags":["a","b","c"],"blocks":[
          {"type":"IMAGE","file":"B.jpg"},
          {"type":"TEXT","content":"invalid","items":["forbidden"]},
          {"type":"IMAGE","file":"RENAMED.jpg"},
          {"type":"IMAGE","file":"A.jpg"}
        ]}`}, nil
	}
	svc := NewService(posts, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, &fakeJobs{}, 4)
	var progress []string
	err := svc.Revise(context.Background(), RevisionJob{
		UserID: "alice", PostSlug: "post", WriteModel: writeRef.String(),
		Payload: mustRevisionPayload(t, "사진 순서 바꿔줘", false),
	}, func(stage string, done, total int) {
		progress = append(progress, fmt.Sprintf("%s:%d/%d", stage, done, total))
	})
	if err != nil {
		t.Fatal(err)
	}
	got := posts.contents[0].Blocks
	if len(got) != 2 || got[0].File != "B.jpg" || got[1].File != "A.jpg" {
		t.Fatalf("revised blocks = %+v", got)
	}
	if fmt.Sprint(progress) != "[write:0/1 write:1/1]" {
		t.Fatalf("progress = %v", progress)
	}
}

func TestRevisionRefiltersAgainstAttachmentsAfterProviderCall(t *testing.T) {
	posts := &fakePosts{input: PostInput{
		Slug: "post", UserID: "alice", Content: revisionContent("before"),
		Images: []Image{{Filename: "A.jpg"}, {Filename: "B.jpg"}},
	}}
	models := newFakeModels()
	models.complete = func(_ llm.ModelRef, _ llm.Request) (llm.Response, error) {
		// B is deleted while the provider is producing its answer.
		posts.input.Images = []Image{{Filename: "A.jpg"}}
		return llm.Response{Text: `{"title":"제목","summary":"요약","tags":["a","b","c"],"blocks":[{"type":"IMAGE","file":"B.jpg"},{"type":"IMAGE","file":"A.jpg"}]}`}, nil
	}
	svc := NewService(posts, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, &fakeJobs{}, 4)

	err := svc.Revise(context.Background(), RevisionJob{
		UserID: "alice", PostSlug: "post", WriteModel: writeRef.String(),
		Payload: mustRevisionPayload(t, "사진 순서 바꿔줘", false),
	}, func(string, int, int) {})
	if err != nil {
		t.Fatal(err)
	}
	if posts.reads != 2 {
		t.Fatalf("attachment reads = %d, want prompt and pre-persist snapshots", posts.reads)
	}
	got := posts.contents[0].Blocks
	if len(got) != 1 || got[0].File != "A.jpg" {
		t.Fatalf("revised blocks after concurrent delete = %+v", got)
	}
}

type linkedRules struct {
	profile *Profile
	lines   []string
}

func (f *linkedRules) AppendRule(_ context.Context, _ string, line string) error {
	f.lines = append(f.lines, line)
	f.profile.Rules = strings.TrimSpace(f.profile.Rules + "\n" + strings.TrimSpace(line))
	return nil
}

func TestStartRevisionSavesRuleBeforeEnqueueAndNewWritePromptSeesIt(t *testing.T) {
	profile := Profile{Styleguide: "STYLE", Excerpts: []string{"EXCERPT"}, Rules: "OLD"}
	rules := &linkedRules{profile: &profile}
	jobs := &fakeJobs{id: "revision-job"}
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Content: revisionContent("body")}}
	svc := NewService(posts, fakeProfiles{}, rules, newFakeModels(), fakeImages{}, jobs, 4)

	id, err := svc.StartRevision(context.Background(), StartRevisionRequest{
		UserID: "alice", PostSlug: "post", Instruction: "  존댓말로  ",
		SaveAsRule: true, WriteModel: writeRef.String(),
	})
	if err != nil || id != "revision-job" {
		t.Fatalf("StartRevision id=%q err=%v", id, err)
	}
	if len(rules.lines) != 1 || rules.lines[0] != "존댓말로" || len(jobs.revisions) != 1 {
		t.Fatalf("rules=%v revisions=%v", rules.lines, jobs.revisions)
	}
	payload, err := parseRevisionPayload(jobs.payloads[0])
	if err != nil || payload.Instruction != "존댓말로" || !payload.SaveAsRule {
		t.Fatalf("payload=%+v err=%v", payload, err)
	}
	system, _ := BuildWritePrompt(profile, nil, "memo", "title", nil)
	if strings.Index(system, "존댓말로") <= strings.Index(system, "EXCERPT") {
		t.Fatalf("saved rule is not after excerpts: %s", system)
	}
}

func TestStartRevisionPreconditionsDoNotEnqueue(t *testing.T) {
	base := StartRevisionRequest{
		UserID: "alice", PostSlug: "post", Instruction: "더 짧게", WriteModel: writeRef.String(),
	}
	for name, tc := range map[string]struct {
		mutate  func(*StartRevisionRequest, *fakePosts, *fakeModels, *fakeJobs)
		wantErr error
	}{
		"empty instruction": {func(r *StartRevisionRequest, _ *fakePosts, _ *fakeModels, _ *fakeJobs) { r.Instruction = "  " }, ErrRevisionInstructionRequired},
		"instruction too long": {func(r *StartRevisionRequest, _ *fakePosts, _ *fakeModels, _ *fakeJobs) {
			r.Instruction = strings.Repeat("가", RevisionInstructionMaxChars+1)
		}, ErrRevisionInstructionTooLong},
		"content required":     {func(_ *StartRevisionRequest, p *fakePosts, _ *fakeModels, _ *fakeJobs) { p.input.Content = nil }, ErrRevisionContentRequired},
		"foreign post":         {func(_ *StartRevisionRequest, p *fakePosts, _ *fakeModels, _ *fakeJobs) { p.err = ErrForbidden }, ErrForbidden},
		"write model required": {func(r *StartRevisionRequest, _ *fakePosts, _ *fakeModels, _ *fakeJobs) { r.WriteModel = "" }, ErrWriteModelRequired},
		"active job": {func(_ *StartRevisionRequest, _ *fakePosts, _ *fakeModels, j *fakeJobs) {
			j.err = &JobAlreadyInProgressError{ActiveID: "active"}
		}, nil},
	} {
		t.Run(name, func(t *testing.T) {
			request := base
			posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Content: revisionContent("body")}}
			models, jobs := newFakeModels(), &fakeJobs{id: "job"}
			tc.mutate(&request, posts, models, jobs)
			_, err := NewService(posts, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, jobs, 4).
				StartRevision(context.Background(), request)
			if name == "active job" {
				var active *JobAlreadyInProgressError
				if !errors.As(err, &active) || active.ActiveID != "active" {
					t.Fatalf("active error = %v", err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
			if jobs.enqueues != 0 {
				t.Fatalf("enqueues = %d", jobs.enqueues)
			}
		})
	}
}

func TestStartRevisionWithoutSaveDoesNotAppendRule(t *testing.T) {
	rules := &fakeRules{}
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Content: revisionContent("body")}}
	_, err := NewService(posts, fakeProfiles{}, rules, newFakeModels(), fakeImages{}, &fakeJobs{id: "job"}, 4).
		StartRevision(context.Background(), StartRevisionRequest{
			UserID: "alice", PostSlug: "post", Instruction: "더 짧게",
			WriteModel: writeRef.String(), SaveAsRule: false,
		})
	if err != nil || len(rules.lines) != 0 {
		t.Fatalf("rules=%v err=%v", rules.lines, err)
	}
}
