package analysisquality

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/ai"
	"github.com/postpilot/backend/internal/clip/media"
	"github.com/postpilot/backend/internal/llm"
)

type Verifier func(context.Context, Input, []byte) (clip.AnalysisCopyVerification, error)

func key(id string, replicate int) string { return fmt.Sprintf("%s/%d", id, replicate) }

type replayModels struct {
	record Record
	calls  int
	policy llm.CallPolicy
}

func (m *replayModels) Resolve(ref llm.ModelRef) (llm.ModelInfo, bool) {
	return llm.ModelInfo{Vision: true, VideoInput: true, StructuredOutput: m.policy.StructuredOutput, Stages: []string{llm.StageNameObserve}}, ref == m.policy.Ref
}
func (m *replayModels) Complete(ctx context.Context, ref llm.ModelRef, req llm.Request) (llm.Response, error) {
	if ctx.Err() != nil {
		return llm.Response{}, ctx.Err()
	}
	if m.calls != 0 || req.Execution == nil || !req.Execution.Matches(ref, req) || req.Execution.Call != m.policy {
		return llm.Response{}, ErrInput
	}
	m.calls++
	if m.record.Failure != "" {
		return m.record.Response, errors.New("analysis_quality_provider_failure_replayed")
	}
	return m.record.Response, nil
}

// Run has no network-capable dependency or live mode. A verifier returns real
// measured copy bounds; unit tests can inject synthetic measurements, which
// remain explicitly synthetic and cannot qualify a profile.
func Run(ctx context.Context, c Corpus, root, mode string, now time.Time, verify Verifier) (Report, error) {
	r := Report{Version: Version, Format: Format, Mode: mode, Status: "refused", CreatedAt: now, AnalysisContract: clip.AnalysisContractVersion, Corpus: c,
		Prompts: map[string]Prompt{}, Verifications: map[string]clip.AnalysisCopyVerification{}, ProviderControls: map[string]string{
			"processing":                       "frozen llm inline static path; endpoint support rechecked only in an admitted live call",
			"fileFps":                          "15; independent of provider sampling",
			"effectiveSamplingFps":             "unknown; not exposed by normalized response",
			"effectiveInternalResolution":      "unknown; not exposed by normalized response",
			"temperatureSeedTopP":              "not sent; provider defaults unknown",
			"customSamplingFpsMediaResolution": "unsupported; not sent",
			"servedModelRevision":              "unknown; not exposed by normalized response"},
		Qualification: Qualification{MissingGates: []string{"current_live_semantic_comparison", "explicit_cumulative_live_approval", "trusted_live_admission_accounting", "actual_provider_internal_controls", "human_review_of_actual_comparison"}},
		Limits:        []string{"Offline replay proves contract/scoring structure only, not current semantic accuracy.", "Original metadata and encoder telemetry have manifest-declared provenance; originals are hashed, not independently decoded by this runner.", "All current arms use production copy bounds; a higher-resolution reference needs a separately reviewed bounded validator and admission.", "Semantic matching and quality classification require human span-grounded annotations; no automatic judge is supplied.", "Budget primitive tests use stubs; live model registration, account admission and accounting are not wired.", "Small corpus/repetition counts establish no population-level statistical guarantee."}}
	if !slices.Contains([]string{"inspect", "replay", "audit"}, mode) || Validate(c, now) != nil || !privateRoot(root) || verify == nil {
		return r, ErrInput
	}
	if ctx.Err() != nil {
		return r, ctx.Err()
	}
	r.PlanDigest = planDigest(c)
	var truth []struct {
		ID        string
		Labels    []Label
		Original  string
		Annotator string
		At        time.Time
	}
	cases := map[string]Case{}
	for _, cs := range c.Cases {
		truth = append(truth, struct {
			ID        string
			Labels    []Label
			Original  string
			Annotator string
			At        time.Time
		}{cs.ID, cs.Labels, cs.Original.SHA256, cs.Annotator, cs.AnnotatedAt})
		cases[cs.ID] = cs
		if e := checkPrivate(root, cs.Original, MaxOriginalBytes); e != nil {
			return r, ErrInput
		}
	}
	r.TruthDigest = digest(truth)
	// Validate every immutable file/key before measurement or model construction.
	// Re-read one bounded copy at a time rather than retaining the entire corpus.
	var total int64
	for _, in := range c.Inputs {
		total += in.File.Bytes
		if total > 128<<20 {
			return r, ErrInput
		}
		if _, e := readPrivate(root, in.File, 8<<20); e != nil {
			return r, ErrInput
		}
		r.Prompts[in.ID] = promptFor(c, in, cases[in.CaseID])
	}
	records := map[string]Replay{}
	total = 0
	for _, rp := range c.Replays {
		total += rp.File.Bytes
		if total > 16<<20 {
			return r, ErrInput
		}
		if _, e := loadRecord(root, rp, c.Origin, r.Prompts[rp.InputID]); e != nil {
			return r, ErrInput
		}
		records[key(rp.InputID, rp.Replicate)] = rp
	}
	for _, in := range c.Inputs {
		data, e := readPrivate(root, in.File, 8<<20)
		if e != nil {
			return r, ErrInput
		}
		v, e := verify(ctx, in, data)
		if e != nil || clip.ValidateAnalysisCopyVerification(in.Copy, v, clip.DefaultMediaConfig(clip.Environment{})) != nil {
			return r, ErrInput
		}
		r.Verifications[in.ID] = v
	}
	if mode == "inspect" {
		r.Status = "inspected_offline"
		return r, nil
	}
	for _, in := range c.Inputs {
		cs := cases[in.CaseID]
		for rep := 0; rep < c.Replicates; rep++ {
			at := Attempt{InputID: in.ID, Replicate: rep, Origin: c.Origin, Prompt: r.Prompts[in.ID], Status: "unrun"}
			rp, ok := records[key(in.ID, rep)]
			if !ok {
				r.Attempts = append(r.Attempts, at)
				continue
			}
			record, e := loadRecord(root, rp, c.Origin, at.Prompt)
			if e != nil {
				return r, ErrInput
			}
			at.ResponseSHA256 = rp.File.SHA256
			at.RecordedUsage = record.Response.Usage
			at.FinishReason = record.Response.FinishReason
			models := &replayModels{record: record, policy: c.Policy}
			service, e := ai.New(models, ai.DefaultConfig(clip.Environment{}))
			if e != nil {
				return r, ErrInput
			}
			input := clip.ChunkInput{Language: c.Language, Source: cs.Source, Index: in.Copy.Index, OffsetMS: in.Copy.OffsetMS, DurationMS: in.Copy.DurationMS, Policy: c.Policy,
				Video: llm.InlineVideo{MIME: "video/mp4", Size: in.File.Bytes, DurationMS: int64(in.Copy.DurationMS), Sampling: llm.VideoSamplingFixed,
					Open: func(callCtx context.Context) (io.ReadCloser, error) {
						if callCtx.Err() != nil {
							return nil, callCtx.Err()
						}
						data, e := readPrivate(root, in.File, 8<<20)
						if e != nil {
							return nil, e
						}
						return io.NopCloser(strings.NewReader(string(data))), nil
					}}}
			parsed, _, e := service.ObserveChunk(ctx, c.Policy.Ref, input)
			if e != nil {
				at.Status = "parse_failed"
				if record.Failure != "" {
					at.Status = "provider_failure_replayed"
				}
				if d, ok := clip.DiagnosticFromError(e); ok {
					d.Check = clip.SafeAttemptCheck(d.Check)
					d.Phase = clip.SafeAttemptPhase(d.Phase)
					d.Values = clip.SafeAttemptValues(d.Values)
					at.Diagnostic = &d
				}
			} else {
				at.Status = "parsed"
				at.Parsed = &parsed
				at.ParsedSHA256 = digest(parsed)
			}
			r.Attempts = append(r.Attempts, at)
		}
	}
	r.OutputsDigest = digest(r.Attempts)
	if e := score(&r); e != nil {
		r.Status = "invalid_assessment"
		return r, e
	}
	r.Status = "replayed_offline"
	return r, nil
}

func loadRecord(root string, rp Replay, origin string, prompt Prompt) (Record, error) {
	var result Record
	data, e := readPrivate(root, rp.File, MaxResponseBytes)
	if e != nil || strict(data, &result, MaxResponseBytes) != nil || result.Version != Version || result.Origin != origin || result.Key != prompt.ReplayKey || !slices.Contains([]string{"", "provider_failure"}, result.Failure) || !slices.Contains([]string{"", "stop", "length", "content_filter"}, result.Response.FinishReason) || len(result.Response.Text) > MaxResponseBytes || !validUsage(result.Response.Usage) {
		return Record{}, ErrInput
	}
	return result, nil
}

// LocalVerifier reuses the bounded production EOF/frame/audio verifier on a
// private workspace copy. No media subprocess sees a corpus path or URL.
func LocalVerifier(workRoot, ffmpeg, ffprobe string) (Verifier, error) {
	env := clip.Environment{WorkRoot: workRoot, FFmpegPath: ffmpeg, FFprobePath: ffprobe, WorkStaleAge: time.Hour, MediaTimeout: clip.AnalysisVerificationTimeout}
	cfg := clip.DefaultMediaConfig(env)
	cfg.PreparedMaxBytes = cfg.AnalysisMaxBytes
	cfg.WorkspaceMaxBytes = clip.AnalysisVerificationWorkspaceBytes
	adapter, e := media.New(cfg, nil)
	if e != nil {
		return nil, ErrInput
	}
	return func(ctx context.Context, in Input, data []byte) (out clip.AnalysisCopyVerification, err error) {
		err = adapter.WithWorkspace(ctx, "analysis-quality", func(ws clip.MediaWorkspace) error {
			path := filepath.Join(ws.Path, "copy.mp4")
			if hash(data) != in.File.SHA256 || int64(len(data)) != in.File.Bytes {
				return ErrInput
			}
			if e := os.WriteFile(path, data, 0600); e != nil {
				return ErrOutput
			}
			var e error
			out, e = adapter.VerifyAnalysisCopy(ctx, ws, path, in.Copy)
			return e
		})
		return out, err
	}, nil
}

// Execute validates locally, writes exclusive private evidence, and returns a
// summary that cannot leak response, truth, rights, routes or filesystem paths.
func Execute(ctx context.Context, args []string, out io.Writer, now time.Time) error {
	return execute(ctx, args, out, now, LocalVerifier)
}

func safeError(e error) error {
	for _, known := range []error{ErrLive, ErrInput, ErrBudget, ErrUncertain, ErrOutput} {
		if errors.Is(e, known) {
			return known
		}
	}
	return ErrInput
}
