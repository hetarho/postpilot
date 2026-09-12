package job

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/postpilot/backend/internal/llm"
)

// A terminal write is one SQLite update. It gets a fresh, bounded context so a
// shutdown arriving after the handler has completed cannot relabel completed work as
// an interrupted job on the next boot.
const finishTimeout = 5 * time.Second

// Run consumes queued rows until its context is cancelled. One Run call is one worker.
func (q *Queue) Run(ctx context.Context) {
	ticker := time.NewTicker(q.pollInterval)
	defer ticker.Stop()

	q.drain(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-q.wake:
			q.drain(ctx)
		case <-ticker.C:
			q.drain(ctx)
		}
	}
}

func (q *Queue) drain(ctx context.Context) {
	for ctx.Err() == nil {
		found, err := q.store.PickNextQueued(ctx, q.now())
		if errors.Is(err, ErrNotFound) {
			return
		}
		if err != nil {
			slog.Error("pick queued job failed", "err", err)
			return
		}
		q.run(ctx, found)
	}
}

func (q *Queue) run(ctx context.Context, found Job) {
	started := time.Now()
	stage, stageStarted := "queued", started
	clipJob := found.Kind == KindGenerateClip || found.Kind == KindRenderClip
	if clipJob {
		slog.Info("clip job started", "job", found.ID, "kind", found.Kind)
		defer func() {
			slog.Info("clip job stopped", "job", found.ID, "kind", found.Kind, "stage", safeClipStage(stage), "stage_elapsed_ms", time.Since(stageStarted).Milliseconds(), "elapsed_ms", time.Since(started).Milliseconds())
		}()
	}
	handler := q.handler(found.Kind)
	var runErr error
	if handler == nil {
		runErr = errHandlerMissing
	} else {
		runErr = callHandler(ctx, handler, found, func(next string, done, total int) {
			if clipJob && next != stage {
				slog.Info("clip stage changed", "job", found.ID, "stage", safeClipStage(next), "previous_stage", safeClipStage(stage), "previous_elapsed_ms", time.Since(stageStarted).Milliseconds(), "elapsed_ms", time.Since(started).Milliseconds())
				stage, stageStarted = next, time.Now()
			}
			if err := q.store.UpdateProgress(ctx, found.ID, next, done, total, q.now()); err != nil && ctx.Err() == nil {
				slog.Error("update job progress failed", "job", found.ID, "err", err)
			}
		})
	}

	// A handler that observed the worker cancellation did not complete and deliberately
	// leaves the row running for the next boot sweep. A successful handler (or an
	// ordinary failure) has reached a terminal result even if shutdown raced its return,
	// so that result must still be committed.
	if ctx.Err() != nil && errors.Is(runErr, ctx.Err()) {
		return
	}

	status := StatusDone
	var failure *Failure
	if runErr != nil {
		status = StatusFailed
		normalized := failureFromError(runErr)
		failure = &normalized
		logJobFailure(found, normalized, runErr)
	}
	finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), finishTimeout)
	defer cancel()
	terminalAt := q.now()
	if err := q.store.Finish(finishCtx, found.ID, status, failure, terminalAt); err != nil {
		slog.Error("finish job failed", "job", found.ID, "status", status, "err", err)
		// The write may have committed despite returning an error. Only a persisted
		// terminal outcome can authorize settlement; otherwise recovery owns the hold.
		persisted, readErr := q.store.GetByID(finishCtx, found.ID)
		if readErr != nil || (persisted.Status != StatusDone && persisted.Status != StatusFailed) {
			return
		}
		status = persisted.Status
		terminalAt = time.Time{}
		if persisted.FinishedAt != nil {
			terminalAt = *persisted.FinishedAt
		}
	}
	// Settling after the terminal write, on the same detached context: the job's ledger
	// rows are all written by now, so this is the first moment the hold can be reconciled
	// against what the work actually cost. A failure here strands credits until the boot
	// sweep, which is why it must not also fail the job.
	if q.admitter != nil && found.Kind != KindRenderClip {
		q.admitter.Settle(finishCtx, found.ID, status)
	}
	if !terminalAt.IsZero() {
		if err := q.notifyTerminal(finishCtx, found, terminalAt); err != nil {
			slog.Warn("job terminal resource release pending recovery", "job", found.ID)
		}
	}
}

func safeClipStage(stage string) string {
	switch stage {
	case "queued", "prepare", "analyze", "plan", "render", "save", "cleanup":
		return stage
	}
	return "unknown"
}

func callHandler(ctx context.Context, handler Handler, found Job, progress Progress) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			attrs := []any{"job", found.ID, "kind", found.Kind}
			if found.Kind != KindGenerateClip && found.Kind != KindRenderClip {
				attrs = append(attrs, "panic", recovered)
			}
			slog.Error("job handler panicked", attrs...)
			err = errHandlerPanicked
		}
	}()
	return handler(ctx, found, progress)
}

// Clip failures may wrap subprocess stderr, media paths or provider bodies. Log
// only normalized metadata even if an unexpected handler bypasses StageFailure.
func logJobFailure(found Job, failure Failure, err error) {
	attrs := []any{"job", found.ID, "kind", found.Kind, "reason", failure.Reason}
	if found.Kind != KindGenerateClip && found.Kind != KindRenderClip {
		attrs = append(attrs, "err", err)
	} else {
		var media interface {
			MediaOperation() string
			MediaFailureClass() string
			MediaElapsedMS() int64
		}
		if errors.As(err, &media) {
			switch operation := media.MediaOperation(); operation {
			case "typeset", "probe", "loudness", "encode_final", "compose_audio", "correct_audio", "render_cut", "compose_video", "sample", "prepare", "decode":
				attrs = append(attrs, "media_operation", operation)
			}
			switch class := media.MediaFailureClass(); class {
			case "workspace_limit", "output_limit", "timeout", "canceled", "process_exit", "process_signal", "command_failed", "validation_failed":
				attrs = append(attrs, "media_error_class", class)
			}
			if ms := media.MediaElapsedMS(); ms >= 0 && ms <= (24*time.Hour).Milliseconds() {
				attrs = append(attrs, "media_elapsed_ms", ms)
			}
		}
		var staged interface{ FailureStage() string }
		if errors.As(err, &staged) {
			switch stage := staged.FailureStage(); stage {
			case "prepare", "analyze", "plan", "render", "save", "cleanup":
				attrs = append(attrs, "stage", stage)
			}
		}
		if diagnostic, ok := llm.DiagnosticOf(err); ok {
			attrs = append(attrs, "operation", diagnostic.Operation, "error_class", diagnostic.Class)
			if diagnostic.HTTPStatus != 0 {
				attrs = append(attrs, "http_status", diagnostic.HTTPStatus)
			}
			if diagnostic.UpstreamCode != 0 {
				attrs = append(attrs, "upstream_code", diagnostic.UpstreamCode)
			}
			if diagnostic.RequestID != "" {
				attrs = append(attrs, "request_id", diagnostic.RequestID)
			}
		}
		var output interface{ OutputValidationCode() string }
		if errors.As(err, &output) {
			// Do not trust arbitrary error implementations or reflected field names.
			switch code := output.OutputValidationCode(); code {
			case "output_encoding_or_size", "output_json", "output_shape", "output_field_type",
				"plan_required", "plan_cut_fields", "plan_caption_fields", "plan_source",
				"plan_caption_time", "plan_volume", "plan_ratio", "plan_target_duration",
				"plan_style", "plan_accent", "caption_measurement", "plan_cut_count",
				"plan_duration_range", "plan_source_metadata", "plan_cut_identity",
				"plan_cut_range", "plan_cut_fade", "plan_cut_transition", "plan_focal", "plan_copy_format",
				"plan_duration_limit", "plan_timeline", "plan_copy_lines",
				"plan_copy_chars", "plan_copy_exposure", "plan_copy_keyword",
				"plan_layout_safe_area", "plan_layout_size", "plan_layout_overlap",
				"plan_layout_motion", "plan_layout_anchor_step", "plan_layout_frequency",
				"plan_layout_disclosure", "plan_layout_kind", "plan_layout_contrast",
				"plan_hook", "plan_chip_count", "plan_chip_label", "plan_copy_count",
				"plan_copy_second_cut", "plan_copy_sequence", "plan_copy_classes":
				attrs = append(attrs, "output_validation", code)
			}
		}
	}
	slog.Error("job failed", attrs...)
}
