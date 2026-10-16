package store_test

import (
	"context"
	"errors"
	"github.com/postpilot/backend/internal/clip"
	clipapp "github.com/postpilot/backend/internal/clip/app"
	"github.com/postpilot/backend/internal/llm"
	"testing"
	"time"
)

type linkedTextFixture struct {
	linkedPostsFixture
	text *clip.LinkedPostText
}

func (p *linkedTextFixture) Text(ctx context.Context, user, slug string) (clip.LinkedPostDocument, error) {
	meta, err := p.Get(ctx, user, slug)
	return clip.LinkedPostDocument{CreatedAt: meta.CreatedAt, Text: p.text}, err
}
func attachWritingPost(t *testing.T, h *generationHarness) *linkedTextFixture {
	t.Helper()
	at := time.Date(2026, 10, 15, 1, 2, 3, 0, time.UTC)
	reader := &linkedTextFixture{linkedPostsFixture: linkedPostsFixture{"alice:story": {Slug: "story", CreatedAt: at, ContentReady: true}}, text: &clip.LinkedPostText{Slug: "story", Title: "owner title", Sections: []clip.LinkedPostSection{{Heading: "식사", Text: "원래 후기"}}}}
	h.projects.WithLinkedPosts(reader)
	slug := "story"
	if _, err := h.projects.UpdateProject(t.Context(), "alice", h.project.ID, clip.ProjectPatch{LinkedPostSlug: &slug}); err != nil {
		t.Fatal(err)
	}
	return reader
}
func TestLinkedPostGenerationApprovalAndWorkerUseTheSameFrozenText(t *testing.T) {
	h := generationSetup(t)
	reader := attachWritingPost(t, h)
	q := quote(t, h)
	if q.Pricing.LinkedPostDigest == "" || q.Pricing.MaxCredits != 107 {
		t.Fatal("missing material digest or changed fixed writing price", q.Pricing)
	}
	reader.text.Sections[0].Text = "고친 후기"
	if _, err := accept(h, q); !errors.Is(err, clip.ErrQuoteChanged) {
		t.Fatal("post edit reused approval", err)
	}
	assertNoQuoteWork(t, h)
	current := quote(t, h)
	id, err := accept(h, current)
	if err != nil {
		t.Fatal("linked quote failed its store transaction fence", err)
	}
	h.planner.id = id
	reader.text.Sections[0].Text = "작업 시작 뒤 수정"
	if err = h.run(t); err != nil {
		t.Fatal(err)
	}
	if h.planner.input.LinkedPost == nil || h.planner.input.LinkedPost.Sections[0].Text != "고친 후기" || h.planner.flows != 1 || h.planner.narrations != 1 {
		t.Fatal("worker failed to keep approved text", h.planner.input.LinkedPost)
	}
}
func TestLinkedPostGenerationOversizeRefusesBeforeHold(t *testing.T) {
	h := generationSetup(t)
	attachWritingPost(t, h)
	h.start(t)
	h.planner.preparationErr = clip.ErrInputTooLarge
	if err := h.run(t); !errors.Is(err, clip.ErrInputTooLarge) {
		t.Fatal(err)
	}
	if len(h.admitter.calls) != 0 || h.planner.observe != 0 || h.planner.flows != 0 || h.planner.narrations != 0 {
		t.Fatal("oversized linked request held or paid for work")
	}
}

type refusingLinkedAllowance struct{ *plannerFake }

func (p refusingLinkedAllowance) ValidateRevision(_ llm.ModelRef, in clip.RevisionInput) error {
	if in.LinkedPost == nil {
		return errors.New("revision lost frozen post before allowance")
	}
	return clip.ErrInputTooLarge
}
func (p refusingLinkedAllowance) ValidateStorylineRequest(_ llm.ModelRef, in clip.StorylineInput) error {
	if in.LinkedPost == nil {
		return errors.New("storyline request lost frozen post before allowance")
	}
	return clip.ErrInputTooLarge
}
func TestLinkedPostRevisionAllowancesRefuseBeforeAnyNewHold(t *testing.T) {
	for _, kind := range []string{"revision", "storyline-request"} {
		t.Run(kind, func(t *testing.T) {
			h := generationSetup(t)
			attachWritingPost(t, h)
			if kind == "revision" {
				h.start(t)
				if err := h.run(t); err != nil {
					t.Fatal(err)
				}
			} else {
				runStoryline(t, h)
			}
			held := len(h.admitter.calls)
			h.service = clipapp.NewGenerationService(h.store, h.projects, h.sources, h.objects, h.media, refusingLinkedAllowance{h.planner}, h.renderer, h.clipJobs(), h.cfg, generationDeps(generationFinisher{h.store}, &quotePricing{}, nil))
			if kind == "revision" {
				startRevision(t, h, "글의 결론을 살려줘", clip.RevisionNarration)
			} else {
				q, err := h.service.QuoteStorylineRevision(t.Context(), "alice", h.project.ID, "글의 결론을 살려줘", "p/o", "p/w")
				if err != nil {
					t.Fatal(err)
				}
				id, err := h.service.StartStorylineRevision(t.Context(), "alice", h.project.ID, "글의 결론을 살려줘", "p/o", "p/w", approve(q))
				if err != nil {
					t.Fatal(err)
				}
				h.planner.id = id
			}
			if err := h.run(t); !errors.Is(err, clip.ErrInputTooLarge) {
				t.Fatal("allowance not checked before hold", err)
			}
			if len(h.admitter.calls) != held {
				t.Fatal("refused writing request took another hold")
			}
		})
	}
}

func TestLinkedPostRevisionApprovalAndWorkerFreeze(t *testing.T) {
	for _, kind := range []string{"revision", "storyline-request"} {
		t.Run(kind, func(t *testing.T) {
			h := generationSetup(t)
			reader := attachWritingPost(t, h)
			if kind == "revision" {
				h.start(t)
				if err := h.run(t); err != nil {
					t.Fatal(err)
				}
			} else {
				runStoryline(t, h)
			}
			var quoteCall func() (clip.GenerationQuote, error)
			var startCall func(clip.GenerationQuote) (string, error)
			if kind == "revision" {
				quoteCall = func() (clip.GenerationQuote, error) {
					return h.service.QuoteRevision(t.Context(), "alice", h.project.ID, "후기대로", clip.RevisionNarration, "p/o", "p/w")
				}
				startCall = func(q clip.GenerationQuote) (string, error) {
					return h.service.StartRevision(t.Context(), "alice", h.project.ID, "후기대로", clip.RevisionNarration, "p/o", "p/w", approve(q))
				}
			} else {
				quoteCall = func() (clip.GenerationQuote, error) {
					return h.service.QuoteStorylineRevision(t.Context(), "alice", h.project.ID, "후기대로", "p/o", "p/w")
				}
				startCall = func(q clip.GenerationQuote) (string, error) {
					return h.service.StartStorylineRevision(t.Context(), "alice", h.project.ID, "후기대로", "p/o", "p/w", approve(q))
				}
			}
			q, err := quoteCall()
			if err != nil {
				t.Fatal(err)
			}
			held := len(h.admitter.calls)
			reader.text.Sections[0].Text = "견적 뒤 수정"
			if _, err = startCall(q); !errors.Is(err, clip.ErrQuoteChanged) {
				t.Fatal("edited text reused approval", err)
			}
			if len(h.admitter.calls) != held {
				t.Fatal("changed quote acquired credits")
			}
			q, err = quoteCall()
			if err != nil {
				t.Fatal(err)
			}
			id, err := startCall(q)
			if err != nil {
				t.Fatal(err)
			}
			h.planner.id = id
			reader.text.Sections[0].Text = "시작 뒤 수정"
			if err = h.run(t); err != nil {
				t.Fatal(err)
			}
			var frozen *clip.LinkedPostText
			if kind == "revision" {
				frozen = h.planner.revisions[len(h.planner.revisions)-1].LinkedPost
			} else {
				frozen = h.planner.storylines[len(h.planner.storylines)-1].LinkedPost
			}
			if frozen == nil || frozen.Sections[0].Text != "견적 뒤 수정" {
				t.Fatal("worker reread the edited post", frozen)
			}
		})
	}
}
