package main

import (
	"context"
	"errors"
	"strings"

	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/memory"
	"github.com/postpilot/backend/internal/post"
	"github.com/postpilot/backend/internal/provider"
)

// memoryModels hands the memory context the account's ANALYZE selection and the provider
// call. It is the same resolution the voice context uses — extraction reads finished prose,
// which is what that stage is for, so no new per-account setting appears (MEM-13).
type memoryModels struct {
	selections *provider.Service
	// The METERED registry, exactly as the voice adapter takes it: an extraction is a model
	// call the account pays for, and a call made through the raw registry would spend
	// credits the ledger never sees.
	registry meteredRegistry
}

func (a memoryModels) AnalyzeModel(ctx context.Context, userID string) (llm.ModelRef, bool, error) {
	selections, err := a.selections.GetSelections(ctx, userID)
	if err != nil {
		return llm.ModelRef{}, false, err
	}
	for _, selection := range selections {
		if selection.Stage != provider.StageAnalyze || selection.Missing {
			continue
		}
		info, ok := a.registry.Lookup(selection.Ref)
		if !ok || info.Disabled {
			continue
		}
		return selection.Ref, true, nil
	}
	return llm.ModelRef{}, false, nil
}

func (a memoryModels) Resolve(ref llm.ModelRef) (llm.ModelInfo, bool) { return a.registry.Lookup(ref) }

func (a memoryModels) Complete(ctx context.Context, ref llm.ModelRef, request llm.Request) (llm.Response, error) {
	return a.registry.Complete(ctx, ref, request)
}

// memoryPosts hands the memory context the finished post to read, ownership already
// checked. The canonical block array is flattened HERE: the memory context speaks in text,
// and the post context never learns that an extraction exists (ARCHITECTURE §2.2).
type memoryPosts struct{ service *post.Service }

func (a memoryPosts) ExtractionSource(ctx context.Context, userID, slug string) (memory.ExtractionSource, error) {
	found, err := a.service.Get(ctx, userID, slug)
	if err != nil {
		if errors.Is(err, post.ErrNotFound) {
			return memory.ExtractionSource{}, memory.ErrNotFound
		}
		return memory.ExtractionSource{}, err
	}
	return memory.ExtractionSource{
		PostSlug: found.Slug, Title: found.Title, Memo: found.Memo,
		Body: extractionBody(found.Content),
	}, nil
}

// extractionBody is the canonical content as prose. IMAGE and VIDEO blocks contribute their
// caption and nothing else: a filename is not a fact about the author's world, and the
// pixels never reach this path at all (MEM-14).
func extractionBody(content *post.PostContent) string {
	if content == nil {
		return ""
	}
	var out strings.Builder
	write := func(line string) {
		if strings.TrimSpace(line) == "" {
			return
		}
		if out.Len() > 0 {
			out.WriteString("\n\n")
		}
		out.WriteString(line)
	}
	write(content.Summary)
	for _, block := range content.Blocks {
		switch block.Type {
		case post.BlockText, post.BlockHeading, post.BlockQuote:
			write(block.Content)
		case post.BlockList:
			write(strings.Join(block.Items, "\n"))
		case post.BlockImage, post.BlockVideo:
			write(block.Caption)
		}
	}
	return out.String()
}

// memoryExtractions is the durable job the extraction runs as. Every one of the three calls
// is the queue's, and the credit gate is the queue's own enqueue seam (QUOTA-13) — this
// adapter prices nothing and charges nothing.
type memoryExtractions struct{ queue *job.Queue }

func (a memoryExtractions) Enqueue(ctx context.Context, request memory.ExtractionRequest) (string, error) {
	payload, err := memory.EncodeExtractionSource(request.Source)
	if err != nil {
		return "", err
	}
	// The post dimension alone: an extraction belongs to the post it reads, and the guard
	// is the ordinary one-job-at-a-time-per-post rule every post-addressed kind carries.
	subjects, guards := postVoiceWork(job.KindExtractMemory, request.UserID, request.PostSlug, "")
	id, err := a.queue.Enqueue(ctx, job.NewJob{
		Kind: job.KindExtractMemory, UserID: request.UserID, Subjects: subjects, Guards: guards,
		WriteModel: request.Model, Payload: payload,
	})
	var active *job.ErrAlreadyInProgress
	if errors.As(err, &active) {
		return "", &memory.ExtractionInProgressError{ActiveID: active.ActiveID}
	}
	return id, err
}

func (a memoryExtractions) SaveCandidates(ctx context.Context, jobID string, payload []byte) error {
	return a.queue.SaveResult(ctx, jobID, payload)
}

func (a memoryExtractions) Candidates(ctx context.Context, userID, jobID string) ([]byte, error) {
	found, err := a.queue.Result(ctx, userID, jobID)
	if err != nil {
		if errors.Is(err, job.ErrNotFound) {
			return nil, memory.ErrNotFound
		}
		return nil, err
	}
	// A job that has not finished carries its INPUT, not its result. Answering that as an
	// empty candidate list would read as "this post yielded nothing", which is a different
	// thing the user would act on.
	if found.Kind != job.KindExtractMemory || found.Status != job.StatusDone {
		return nil, memory.ErrExtractionNotReady
	}
	return found.Payload, nil
}
