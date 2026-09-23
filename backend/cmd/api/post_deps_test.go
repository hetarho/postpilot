package main

import (
	"context"
	"time"

	"github.com/postpilot/backend/internal/guideline"
	"github.com/postpilot/backend/internal/post"
	"github.com/postpilot/backend/internal/voice"
)

func testPostLimits() post.Limits {
	return post.Limits{PutTTL: time.Minute, GetTTL: time.Minute, MaxImageBytes: 1 << 20, MaxPhotos: 30, AnswerLabelMax: 40, AnswerValueMax: 500}
}

// testPostDeps wires a post service to a real voice service and neutral everything else.
func testPostDeps(voiceSvc *voice.Service) post.Deps {
	return testPostDepsWithLinks(voiceSvc, noDetach{})
}

func testPostDepsWithLinks(voiceSvc *voice.Service, links post.GuidelineCandidateDetacher) post.Deps {
	return post.Deps{
		Jobs:           noActiveJobs{},
		Voices:         postVoices{service: voiceSvc},
		Experiments:    noExperiments{},
		ContentPurger:  noPurge{},
		CandidateLinks: links,
		MemoryLinks:    noDetach{},
		Fields:         blogFields{},
	}
}

type noActiveJobs struct{}

func (noActiveJobs) ActiveForPost(context.Context, string) (*post.ActiveJob, error) { return nil, nil }

type noExperiments struct{}

func (noExperiments) PendingForPost(context.Context, string, string) (string, error) { return "", nil }

type noDetach struct{}

func (noDetach) DetachPost(context.Context, string, string) error { return nil }

// lateLinks detaches through a guideline service that is constructed after the post
// service holding it.
type lateLinks struct{ svc **guideline.Service }

func (l lateLinks) DetachPost(ctx context.Context, userID, postSlug string) error {
	return postCandidateLinks{service: *l.svc}.DetachPost(ctx, userID, postSlug)
}
