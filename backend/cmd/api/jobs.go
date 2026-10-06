package main

import (
	"context"
	"errors"
	"github.com/postpilot/backend/internal/authoring"
	"strings"
	"time"

	"github.com/postpilot/backend/internal/clip"
	clipapp "github.com/postpilot/backend/internal/clip/app"

	"github.com/postpilot/backend/internal/experiment"
	"github.com/postpilot/backend/internal/generation"
	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/memory"
	"github.com/postpilot/backend/internal/post"
	"github.com/postpilot/backend/internal/template"
	"github.com/postpilot/backend/internal/usage"
	"github.com/postpilot/backend/internal/voice"
	"github.com/postpilot/backend/internal/voice/spoken"
)

// registerJobs binds every job kind to the context that owns it. `metered` stamps the
// account and job on the context once, at this seam, so no handler below carries the
// ledger's identity through its own call graph.
func registerJobs(c *contexts) {
	q := c.jobs
	voiceSvc, generationSvc, experimentSvc := c.voice, c.generation, c.experiment
	// The payload is the snapshot of 학습 글 the start froze (VOICE-22).
	q.Register(job.KindAnalyzeVoice, metered(func(ctx context.Context, found job.Job, progress job.Progress) error {
		materialIDs, err := voice.DecodeAnalysisSnapshot(found.Payload)
		if err != nil {
			return err
		}
		return voiceSvc.Analyze(ctx, voice.AnalysisJob{
			UserID: found.UserID, VoiceID: found.Subject(voice.JobSubject), WriteModel: found.WriteModel,
			MaterialIDs: materialIDs,
		}, voice.Progress(progress))
	}))
	// 검증 (VOICE-43): the payload names the check the job writes back to.
	q.Register(job.KindCheckVoice, metered(func(ctx context.Context, found job.Job, progress job.Progress) error {
		checkID := strings.TrimSpace(string(found.Payload))
		if checkID == "" {
			return errors.New("voice check payload is missing")
		}
		return voiceSvc.CheckVoice(ctx, voice.CheckJob{
			UserID: found.UserID, VoiceID: found.Subject(voice.JobSubject), CheckID: checkID, WriteModel: found.WriteModel,
		}, voice.Progress(progress))
	}))
	// The one job the memory context owns. It reads the post frozen onto its own row and
	// writes its candidates back onto it; it touches no memory table at all (MEM-14).
	q.Register(job.KindExtractMemory, metered(func(ctx context.Context, found job.Job, progress job.Progress) error {
		slug := found.Subject(post.JobSubject)
		if slug == "" {
			return job.ErrInvalidTarget
		}
		source, err := memory.DecodeExtractionSource(found.Payload)
		if err != nil {
			return err
		}
		return c.memory.Extract(ctx, memory.ExtractionJob{
			ID: found.ID, UserID: found.UserID, PostSlug: slug, Model: found.WriteModel, Source: source,
		}, progress)
	}))
	q.Register(job.KindModelExperiment, metered(func(ctx context.Context, found job.Job, progress job.Progress) error {
		experimentID := strings.TrimSpace(string(found.Payload))
		if experimentID == "" {
			return errors.New("model experiment payload is missing")
		}
		return experimentSvc.Handle(ctx, experimentID, experiment.Progress(progress))
	}))
	q.Register(job.KindGenerate, metered(func(ctx context.Context, found job.Job, progress job.Progress) error {
		run, err := generateJob(found)
		if err != nil {
			return err
		}
		return generationSvc.Generate(ctx, run, generation.Progress(progress))
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
	q.Register(job.KindStoryline, metered(func(ctx context.Context, found job.Job, progress job.Progress) error {
		slug := found.Subject(post.JobSubject)
		if slug == "" {
			return job.ErrInvalidTarget
		}
		return generationSvc.WriteStoryline(ctx, generation.StorylineJob{
			UserID: found.UserID, PostSlug: slug, ObserveModel: found.ObserveModel, WriteModel: found.WriteModel,
			Payload: found.Payload,
		}, generation.Progress(progress))
	}))
	q.Register(job.KindReviseStoryline, metered(func(ctx context.Context, found job.Job, progress job.Progress) error {
		slug := found.Subject(post.JobSubject)
		if slug == "" {
			return job.ErrInvalidTarget
		}
		return generationSvc.ReviseStoryline(ctx, generation.StorylineRevisionJob{
			UserID: found.UserID, PostSlug: slug, WriteModel: found.WriteModel, Payload: found.Payload,
		}, generation.Progress(progress))
	}))
	// The template request (TMPL-58) writes its answer back onto its own row; the draft it is
	// for lives in the browser, so the job has no subject at all.
	q.Register(job.KindTemplateRequest, metered(func(ctx context.Context, found job.Job, progress job.Progress) error {
		return c.template.RunRequest(ctx, template.RequestRun{
			ID: found.ID, UserID: found.UserID, WriteModel: found.WriteModel, Payload: found.Payload,
		}, progress)
	}))
	q.Register(job.KindWritingVoiceCandidates, metered(func(ctx context.Context, found job.Job, progress job.Progress) error {
		return c.voiceCandidates.Run(ctx, voice.CandidateRun{ID: found.ID, UserID: found.UserID, WriteModel: found.WriteModel, Payload: found.Payload}, voice.Progress(progress))
	}))
	q.Register(authoring.JobKind, metered(func(ctx context.Context, found job.Job, progress job.Progress) error {
		return c.authoring.Run(ctx, authoring.Run{ID: found.ID, UserID: found.UserID, WriteModel: found.WriteModel, Payload: found.Payload}, func(stage string, done, total int) { progress(stage, done, total) })
	}))
	registerClipJobs(q, c.clipGeneration, c.clipSources)
	q.Register(clip.JobKindSpeech, metered(c.clipSpeech.Run))
	q.OnTerminal(clip.JobKindSpeech, func(ctx context.Context, j job.Job, _ time.Time) error { return c.clipSpeech.OnTerminal(ctx, j) })
	for _, kind := range []string{spoken.JobKindDesign, spoken.JobKindConfirm, spoken.JobKindProbe} {
		q.Register(kind, metered(c.spokenGeneration.Run))
		q.OnTerminal(kind, func(ctx context.Context, j job.Job, _ time.Time) error { return c.spokenGeneration.OnTerminal(ctx, j) })
	}
}

// registerClipJobs binds the clip kinds and releases each attempt's held sources when its
// job ends, whatever the outcome.
func registerClipJobs(q *job.Queue, service *clipapp.GenerationService, sources *clipapp.SourceService) {
	q.Register(clip.JobKindGenerate, metered(func(ctx context.Context, j job.Job, progress job.Progress) error {
		return service.Run(ctx, j.UserID, j.ID, j.Subject(clip.JobSubject), j.Payload, progress)
	}))
	q.Register(clip.JobKindRender, metered(func(ctx context.Context, j job.Job, progress job.Progress) error {
		return service.RunRender(ctx, j.UserID, j.ID, j.Subject(clip.JobSubject), j.Payload, progress)
	}))
	// The sampling a browser render waits on (CLIP-192).
	q.Register(clip.JobKindSampleBrowserRender, metered(func(ctx context.Context, j job.Job, progress job.Progress) error {
		return service.RunBrowserSampling(ctx, j.UserID, j.ID, j.Subject(clip.JobSubject), j.Payload, progress)
	}))
	q.Register(clip.JobKindRevise, metered(func(ctx context.Context, j job.Job, progress job.Progress) error {
		return service.RunRevision(ctx, j.UserID, j.ID, j.Subject(clip.JobSubject), j.Payload, progress)
	}))
	// The storyline call and the storyline request (CLIP-177, CLIP-181).
	q.Register(clip.JobKindStoryline, metered(func(ctx context.Context, j job.Job, progress job.Progress) error {
		return service.RunStoryline(ctx, j.UserID, j.ID, j.Subject(clip.JobSubject), j.Payload, progress)
	}))
	q.Register(clip.JobKindReviseStoryline, metered(func(ctx context.Context, j job.Job, progress job.Progress) error {
		return service.RunStorylineRevision(ctx, j.UserID, j.ID, j.Subject(clip.JobSubject), j.Payload, progress)
	}))
	for _, kind := range []string{clip.JobKindGenerate, clip.JobKindRender, clip.JobKindSampleBrowserRender, clip.JobKindRevise, clip.JobKindStoryline, clip.JobKindReviseStoryline} {
		terminalKind := kind
		q.OnTerminal(kind, func(ctx context.Context, j job.Job, at time.Time) error {
			if terminalKind == clip.JobKindGenerate {
				if e := service.FinishInitialSpeech(ctx, j); e != nil {
					return e
				}
			}
			if terminalKind == clip.JobKindRender {
				if err := service.ReleaseExport(ctx, j.ID); err != nil {
					return err
				}
			}
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

// generateJob maps a stored generate job onto the run the worker executes: the subjects name the
// post and the voice, the row names the models, and the payload crosses opaque — generation
// froze it and only generation reads it. It is its own function so a test drives exactly the
// mapping the worker uses.
func generateJob(found job.Job) (generation.GenerateJob, error) {
	slug := found.Subject(post.JobSubject)
	if slug == "" {
		return generation.GenerateJob{}, job.ErrInvalidTarget
	}
	return generation.GenerateJob{
		UserID: found.UserID, PostSlug: slug, VoiceID: found.Subject(voice.JobSubject),
		ObserveModel: found.ObserveModel, WriteModel: found.WriteModel, Payload: found.Payload,
	}, nil
}
