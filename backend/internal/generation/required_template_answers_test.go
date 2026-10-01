package generation

import (
	"context"
	"errors"
	"testing"

	"github.com/postpilot/backend/internal/llm"
)

func TestNewWriteStartsRefuseMissingRequiredTemplateAnswerBeforeEnqueue(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(*PostInput)
		start func(*Service) error
	}{
		{"direct generation", nil, func(s *Service) error {
			_, err := s.Start(context.Background(), StartRequest{UserID: "alice", PostSlug: "post", WriteModel: writeRef.String()})
			return err
		}},
		{"generation from storyline", func(post *PostInput) {
			post.Storyline = &Storyline{Paragraphs: []StorylineParagraph{{Text: "도입"}}}
		}, func(s *Service) error {
			_, err := s.Start(context.Background(), StartRequest{UserID: "alice", PostSlug: "post", WriteModel: writeRef.String(), FromStoryline: true})
			return err
		}},
		{"storyline creation", nil, func(s *Service) error {
			_, err := s.StartStoryline(context.Background(), StartStorylineRequest{UserID: "alice", PostSlug: "post", WriteModel: writeRef.String()})
			return err
		}},
		{"storyline rewrite", func(post *PostInput) {
			post.Storyline = &Storyline{Paragraphs: []StorylineParagraph{{Text: "도입"}}}
		}, func(s *Service) error {
			_, err := s.StartStorylineRevision(context.Background(), StartStorylineRevisionRequest{UserID: "alice", PostSlug: "post", Request: "더 자세히", WriteModel: writeRef.String()})
			return err
		}},
		{"write comparison snapshot", nil, func(s *Service) error {
			_, err := s.SnapshotWriteInput(context.Background(), "alice", "post", llm.ModelRef{}, nil, nil, false)
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			post := PostInput{Slug: "post", UserID: "alice", Voice: liveVoice, TemplateID: "tmpl"}
			if tc.setup != nil {
				tc.setup(&post)
			}
			posts := &fakePosts{input: post}
			jobs := &fakeJobs{id: "job"}
			models := newFakeModels()
			briefs := &fakeTemplateBriefs{newWriteErr: &RequiredTemplateAnswerError{Label: "직접 겪은 일"}}
			svc := templateAwareService(t, briefs, posts, jobs, models)
			err := tc.start(svc)
			var missing *RequiredTemplateAnswerError
			if !errors.As(err, &missing) || missing.Label != "직접 겪은 일" {
				t.Fatalf("start error = %v, want required answer label", err)
			}
			if jobs.enqueues != 0 || briefs.newWriteCalls != 1 || briefs.calls != 0 || len(models.calls) != 0 {
				t.Fatalf("side effects: jobs=%d new-write=%d revision=%d provider=%d", jobs.enqueues, briefs.newWriteCalls, briefs.calls, len(models.calls))
			}
		})
	}
}

func TestExistingContentRevisionIgnoresNewRequiredTemplateAnswer(t *testing.T) {
	posts := &fakePosts{input: PostInput{
		Slug: "post", UserID: "alice", Voice: liveVoice, TemplateID: "tmpl", Content: revisionContent("기존 본문"),
	}}
	jobs := &fakeJobs{id: "revision-job"}
	briefs := &fakeTemplateBriefs{brief: *testBrief(), newWriteErr: &RequiredTemplateAnswerError{Label: "직접 겪은 일"}}
	svc := templateAwareService(t, briefs, posts, jobs, newFakeModels())
	id, err := svc.StartRevision(context.Background(), StartRevisionRequest{
		UserID: "alice", PostSlug: "post", Instruction: "문장을 다듬어 줘", WriteModel: writeRef.String(),
	})
	if err != nil || id != "revision-job" || jobs.enqueues != 1 || briefs.newWriteCalls != 0 || briefs.calls != 1 {
		t.Fatalf("revision = id:%q err:%v jobs:%d new-write:%d revision:%d", id, err, jobs.enqueues, briefs.newWriteCalls, briefs.calls)
	}
}
