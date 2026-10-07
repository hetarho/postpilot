package app

import (
	"context"

	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/post"
)

type PostJobs struct {
	queue             *job.Queue
	ordinary, content []string
}

func NewPostJobs(queue *job.Queue, ordinary, content []string) *PostJobs {
	if queue == nil || len(ordinary) == 0 || len(content) == 0 {
		panic("experiment/app: post jobs and explicit ordinary/content kinds are required")
	}
	return &PostJobs{queue: queue, ordinary: append([]string(nil), ordinary...), content: append([]string(nil), content...)}
}
func (a *PostJobs) ActiveForPost(ctx context.Context, slug string) (*post.ActiveJob, error) {
	found, err := a.queue.ActiveFor(ctx, job.Subject{Dimension: post.JobSubject, ID: slug}, job.Filter{})
	return a.project(found), err
}
func (a *PostJobs) ActiveOrdinaryForPost(ctx context.Context, user, slug string) (*post.ActiveJob, error) {
	for _, kind := range a.ordinary {
		found, err := a.queue.ActiveFor(ctx, job.Subject{Dimension: post.JobSubject, ID: slug}, job.Filter{UserID: user, Kind: kind})
		if err != nil {
			return nil, err
		}
		if found != nil {
			return a.project(found), nil
		}
	}
	return nil, nil
}
func (a *PostJobs) LatestOrdinaryForPosts(ctx context.Context, user string, slugs []string) (map[string]post.ActiveJob, error) {
	found, err := a.queue.LatestForSubjects(ctx, user, post.JobSubject, slugs, a.ordinary)
	if err != nil {
		return nil, err
	}
	result := make(map[string]post.ActiveJob, len(found))
	for slug, j := range found {
		result[slug] = *a.project(&j)
	}
	return result, nil
}
func (a *PostJobs) project(found *job.JobSummary) *post.ActiveJob {
	if found == nil {
		return nil
	}
	writes := false
	for _, kind := range a.content {
		if kind == found.Kind {
			writes = true
			break
		}
	}
	var failure *post.Failure
	if found.Failure != nil {
		failure = &post.Failure{Reason: found.Failure.Reason, Params: found.Failure.Params, TechnicalDetail: found.Failure.TechnicalDetail}
	}
	return &post.ActiveJob{ID: found.ID, Kind: found.Kind, Status: found.Status, Stage: found.Stage, ProgressDone: found.ProgressDone, ProgressTotal: found.ProgressTotal, Failure: failure,
		PostSlug: found.Subject(post.JobSubject), ObserveModel: found.ObserveModel, WriteModel: found.WriteModel, TargetLanguage: post.Language(found.TargetLanguage), CreatedAt: found.CreatedAt, UpdatedAt: found.UpdatedAt, WritesContent: writes}
}
