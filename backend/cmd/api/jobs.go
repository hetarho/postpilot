package main

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/postpilot/backend/internal/clip"
	clipapp "github.com/postpilot/backend/internal/clip/app"

	"github.com/postpilot/backend/internal/experiment"
	"github.com/postpilot/backend/internal/generation"
	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/post"
	"github.com/postpilot/backend/internal/usage"
	"github.com/postpilot/backend/internal/voice"
)

// registerJobs binds every job kind to the context that owns it. `metered` stamps the
// account and job on the context once, at this seam, so no handler below carries the
// ledger's identity through its own call graph.
func registerJobs(c *contexts) {
	q := c.jobs
	voiceSvc, generationSvc, experimentSvc := c.voice, c.generation, c.experiment
	q.Register(job.KindAnalyzeVoice, metered(func(ctx context.Context, found job.Job, progress job.Progress) error {
		return voiceSvc.Analyze(ctx, voice.AnalysisJob{
			UserID: found.UserID, VoiceID: found.Subject(voice.JobSubject), WriteModel: found.WriteModel,
		}, voice.Progress(progress))
	}))
	q.Register(job.KindLearnVoice, metered(func(ctx context.Context, found job.Job, progress job.Progress) error {
		return voiceSvc.Learn(ctx, voice.LearningJob{UserID: found.UserID, EventID: strings.TrimSpace(string(found.Payload)), WriteModel: found.WriteModel}, voice.Progress(progress))
	}))
	q.Register(job.KindCompareVoiceRule, metered(func(ctx context.Context, found job.Job, progress job.Progress) error {
		return voiceSvc.CompareRule(ctx, found.UserID, strings.TrimSpace(string(found.Payload)), found.WriteModel, voice.Progress(progress))
	}))
	q.Register(job.KindValidateVoiceProfile, metered(func(ctx context.Context, found job.Job, progress job.Progress) error {
		return voiceSvc.ValidateProfile(ctx, found.UserID, strings.TrimSpace(string(found.Payload)), voice.Progress(progress))
	}))
	q.Register(job.KindSeedVoice, metered(func(ctx context.Context, found job.Job, progress job.Progress) error {
		return voiceSvc.Seed(ctx, voice.SeedJob{
			UserID: found.UserID, VoiceID: found.Subject(voice.JobSubject), Description: string(found.Payload), WriteModel: found.WriteModel,
		}, voice.Progress(progress))
	}))
	q.Register(job.KindModelExperiment, metered(func(ctx context.Context, found job.Job, progress job.Progress) error {
		experimentID := strings.TrimSpace(string(found.Payload))
		if experimentID == "" {
			return errors.New("model experiment payload is missing")
		}
		return experimentSvc.Handle(ctx, experimentID, experiment.Progress(progress))
	}))
	q.Register(job.KindGenerate, metered(func(ctx context.Context, found job.Job, progress job.Progress) error {
		slug := found.Subject(post.JobSubject)
		if slug == "" {
			return job.ErrInvalidTarget
		}
		options, err := generation.DecodeGenerationPayload(found.Payload)
		if err != nil {
			return err
		}
		return generationSvc.Generate(ctx, generation.GenerateJob{
			UserID: found.UserID, PostSlug: slug, VoiceID: found.Subject(voice.JobSubject),
			ObserveModel: found.ObserveModel, WriteModel: found.WriteModel,
			TargetLanguage: options.TargetLanguage, TargetLength: options.TargetLength, TagCount: options.TagCount, Template: options.Template,
			Guidelines: options.Guidelines, ObserveFiles: options.ObserveFiles, Observations: options.Observations,
			WriteNativeEffort: options.WriteNativeEffort,
		}, generation.Progress(progress))
	}))
	q.Register(job.KindRevise, metered(func(ctx context.Context, found job.Job, progress job.Progress) error {
		slug := found.Subject(post.JobSubject)
		if slug == "" {
			return job.ErrInvalidTarget
		}
		return generationSvc.Revise(ctx, generation.RevisionJob{
			UserID: found.UserID, PostSlug: slug, VoiceID: found.Subject(voice.JobSubject), WriteModel: found.WriteModel,
			Payload: found.Payload,
		}, generation.Progress(progress))
	}))
	registerClipJobs(q, c.clipGeneration, c.clipSources)
}

// registerClipJobs binds the three clip kinds and releases each attempt's held sources
// when its job ends, whatever the outcome.
func registerClipJobs(q *job.Queue, service *clipapp.GenerationService, sources *clipapp.SourceService) {
	q.Register(clip.JobKindGenerate, metered(func(ctx context.Context, j job.Job, progress job.Progress) error {
		return service.Run(ctx, j.UserID, j.ID, j.Subject(clip.JobSubject), j.Payload, progress)
	}))
	q.Register(clip.JobKindRender, metered(func(ctx context.Context, j job.Job, progress job.Progress) error {
		return service.RunRender(ctx, j.UserID, j.ID, j.Subject(clip.JobSubject), j.Payload, progress)
	}))
	q.Register(clip.JobKindRevise, metered(func(ctx context.Context, j job.Job, progress job.Progress) error {
		return service.RunRevision(ctx, j.UserID, j.ID, j.Subject(clip.JobSubject), j.Payload, progress)
	}))
	for _, kind := range []string{clip.JobKindGenerate, clip.JobKindRender, clip.JobKindRevise} {
		q.OnTerminal(kind, func(ctx context.Context, j job.Job, at time.Time) error {
			return sources.ReleaseAttempt(ctx, j.UserID, j.ID, at)
		})
	}
}

// metered attributes every provider call a job handler makes to the account and the job
// that caused it. Stamped once here, at the worker seam, so no handler below has to carry
// the ledger's identity through its own call graph.
func metered(handler job.Handler) job.Handler {
	return func(ctx context.Context, found job.Job, progress job.Progress) error {
		return handler(usage.WithWork(ctx, usage.Work{
			UserID: found.UserID, Kind: found.Kind, JobID: found.ID,
			ObserveModel: found.ObserveModel, WriteModel: found.WriteModel,
		}), found, progress)
	}
}
