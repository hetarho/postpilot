package main

import (
	"context"
	"errors"
	"strings"

	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/post"
	"github.com/postpilot/backend/internal/template"
)

// templateModels is the METERED registry, as every other context that makes a model call
// takes it: a template request is a call the account pays for (QUOTA-67).
type templateModels struct{ registry meteredRegistry }

func (a templateModels) Resolve(ref llm.ModelRef) (llm.ModelInfo, bool) {
	return a.registry.Lookup(ref)
}

func (a templateModels) Complete(ctx context.Context, ref llm.ModelRef, request llm.Request) (llm.Response, error) {
	return a.registry.Complete(ctx, ref, request)
}

// templateSamples hands the template context a post as a sample, ownership already checked and
// the canonical blocks already flattened HERE: the template context speaks in text, and the post
// context never learns that a template request exists (TMPL-64).
type templateSamples struct{ service *post.Service }

func (a templateSamples) RequestSample(ctx context.Context, userID, slug string) (template.Sample, error) {
	found, err := a.service.Get(ctx, userID, slug)
	if err != nil {
		if errors.Is(err, post.ErrNotFound) {
			return template.Sample{}, template.ErrSampleUnavailable
		}
		return template.Sample{}, err
	}
	text := sampleText(found.Content)
	if text == "" {
		return template.Sample{}, template.ErrSampleUnavailable
	}
	title := found.Title
	if found.Content != nil && strings.TrimSpace(found.Content.Title) != "" {
		title = found.Content.Title
	}
	return template.Sample{Title: strings.TrimSpace(title), Text: text}, nil
}

// sampleText is a post's shape as plain text: its prose blocks as written and every photo as a
// position, `[사진]`, so the order and the photo places survive while nothing about a photo does
// — no file, caption or video, and no summary or tags (TMPL-64).
func sampleText(content *post.PostContent) string {
	if content == nil {
		return ""
	}
	lines := make([]string, 0, len(content.Blocks))
	for _, block := range content.Blocks {
		switch block.Type {
		case post.BlockText:
			if text := strings.TrimSpace(block.Content); text != "" {
				lines = append(lines, text)
			}
		case post.BlockHeading:
			if text := strings.TrimSpace(block.Content); text != "" {
				marker := "## "
				if block.Level >= 3 {
					marker = "### "
				}
				lines = append(lines, marker+text)
			}
		case post.BlockQuote:
			if text := strings.TrimSpace(block.Content); text != "" {
				lines = append(lines, "> "+text)
			}
		case post.BlockList:
			items := make([]string, 0, len(block.Items))
			for _, item := range block.Items {
				if item = strings.TrimSpace(item); item != "" {
					items = append(items, "- "+item)
				}
			}
			if len(items) > 0 {
				lines = append(lines, strings.Join(items, "\n"))
			}
		case post.BlockImage:
			lines = append(lines, "[사진]")
		}
	}
	return strings.Join(lines, "\n\n")
}

// templateRequestJobs is the durable job a template request runs as. The credit gate is the
// queue's own enqueue seam (QUOTA-13): the planned calls are the first one and every correction
// allowed, on the write stage, each at the request's own completion cap (QUOTA-67).
type templateRequestJobs struct{ queue *job.Queue }

func (a templateRequestJobs) EnqueueRequest(ctx context.Context, request template.RequestJob) (string, error) {
	// No subject and no guard: the queue's unattached rule keeps one per account and kind.
	id, err := a.queue.Enqueue(ctx, job.NewJob{
		Kind: job.KindTemplateRequest, UserID: request.UserID, WriteModel: request.WriteModel, Payload: request.Payload,
		PricingCalls: []job.PlannedCall{{
			Ref: request.WriteModel, Stage: llm.StageNameWrite, Count: request.Calls, CompletionTokens: request.CompletionTokens,
		}},
	})
	if errors.Is(err, job.ErrActiveConflict) {
		return "", template.ErrRequestRunning
	}
	return id, err
}

func (a templateRequestJobs) SaveRequestResult(ctx context.Context, jobID string, payload []byte) error {
	return a.queue.SaveResult(ctx, jobID, payload)
}

func (a templateRequestJobs) RequestPayload(ctx context.Context, userID, jobID string) ([]byte, error) {
	found, err := a.queue.Result(ctx, userID, jobID)
	if err != nil {
		if errors.Is(err, job.ErrNotFound) {
			return nil, template.ErrNotFound
		}
		return nil, err
	}
	if found.Kind != job.KindTemplateRequest {
		return nil, template.ErrNotFound
	}
	// Until it is done the row holds the request's INPUT, not an answer.
	if found.Status != job.StatusDone {
		return nil, template.ErrRequestNotReady
	}
	return found.Payload, nil
}

// templateEstimates prices one template request the way the post estimate prices a post: the
// registry's catalog prices at the current eligible rate, credits only (QUOTA-40, QUOTA-65).
type templateEstimates struct{ rates estimateRates }

func (a templateEstimates) CallCredits(ctx context.Context, info llm.ModelInfo, promptTokens, completionTokens int64) (int, bool) {
	rate, err := a.rates.SelectRate(ctx)
	if err != nil {
		return 0, false
	}
	return plan.CallCreditsAt(catalogPricer(info), rate, promptTokens, completionTokens)
}
