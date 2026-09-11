package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"unicode/utf8"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
	"github.com/postpilot/backend/internal/llm"
)

type Models interface {
	Resolve(llm.ModelRef) (llm.ModelInfo, bool)
	Complete(context.Context, llm.ModelRef, llm.Request) (llm.Response, error)
}
type CaptionSizer interface {
	CaptionSize(context.Context, string, clip.Caption) (float64, float64, error)
	// FixedElements is the disclosure badge and this cut's chips, already placed.
	// Copy yields to them and never displaces them (CDS-45), so the selector has
	// to see them before it chooses an anchor.
	FixedElements(ctx context.Context, ratio, disclosure string, labels []string, answers []clip.Answer) (clip.Manifest, error)
	// CardElements is the hook and ending cards, measured and placed. Copy
	// yields to them exactly as it does to the badge (CDS-28, CDS-29, CDS-45):
	// nothing shows under a card, so the selector needs their regions before it
	// chooses an anchor for the first and the last cut.
	CardElements(ctx context.Context, plan clip.EditPlan) (clip.Manifest, error)
	// Layout lays the whole composed plan out and gates it on the design
	// system's verifier (CDS-52), touching no source pixel. The composer runs it
	// on its own result so a residual violation is a COMPOSITION failure with
	// the check named, not a surprise at render time.
	Layout(ctx context.Context, plan clip.EditPlan, sources []clip.RenderSource) (clip.Manifest, error)
}
type Config struct {
	Analysis                                                                                          clip.AnalysisLimits
	Render                                                                                            clip.RenderConfig
	Template                                                                                          clip.Limits
	ObserveCompletionTokens, PlanCompletionTokens, MaxResponseBytes, MaxCutIDRunes, TargetToleranceMS int
	ObserveReasoning, PlanReasoning                                                                   llm.ReasoningEffort
}

// Budgets is copied into the durable job's planned calls before any provider work.
// The request path reads these same values, never the registry's default budget.
type Budgets = clip.CompletionBudgets
type Service struct {
	models   Models
	captions CaptionSizer
	cfg      Config
}

func New(models Models, captions CaptionSizer, cfg Config) (*Service, error) {
	for _, n := range []int{cfg.Analysis.ChunkMS, cfg.Analysis.MaxSources, cfg.Analysis.MaxSourceDurationMS, cfg.Analysis.MaxSegments, cfg.Analysis.MaxTextRunes, cfg.Analysis.MaxSubjects, cfg.ObserveCompletionTokens, cfg.PlanCompletionTokens, cfg.MaxResponseBytes, cfg.MaxCutIDRunes, cfg.TargetToleranceMS, cfg.Render.MaxCuts, cfg.Render.MaxCopyRunes, cfg.Template.NameChars, cfg.Template.AnswerChars} {
		if n <= 0 {
			return nil, errors.New("invalid clip AI configuration")
		}
	}
	if models == nil || captions == nil || !cfg.ObserveReasoning.Valid() || !cfg.PlanReasoning.Valid() {
		return nil, errors.New("clip AI requires models and caption measurement")
	}
	return &Service{models, captions, cfg}, nil
}
func (s *Service) Budgets() Budgets {
	return Budgets{Observe: s.cfg.ObserveCompletionTokens, Plan: s.cfg.PlanCompletionTokens}
}

type StageError struct {
	Stage string
	Cause error
}

func (e *StageError) Error() string        { return fmt.Sprintf("clip %s: %v", e.Stage, e.Cause) }
func (e *StageError) Unwrap() error        { return e.Cause }
func (e *StageError) Failure() llm.Failure { return llm.NormalizeFailure(e.Cause) }
func stageError(stage string, err error) error {
	if err == nil {
		return nil
	}
	return &StageError{stage, err}
}
func (s *Service) model(ref llm.ModelRef, stage string) (llm.ModelInfo, error) {
	info, ok := s.models.Resolve(ref)
	if !ok || !info.ServesStage(stage) {
		return info, llm.ErrModelUnavailable
	}
	if info.Disabled {
		if info.DisabledReason == llm.DisabledReasonDelisted {
			return info, llm.ErrModelUnavailable
		}
		return info, llm.ErrProviderDisabled
	}
	if stage == llm.StageNameObserve && (!info.Vision || !info.VideoInput || !info.VideoDelivery.InlineStaticVideo) {
		return info, clip.ErrModelInputUnsupported
	}
	return info, nil
}
func (s *Service) ValidateModels(observe, write llm.ModelRef) error {
	if _, err := s.model(observe, llm.StageNameObserve); err != nil {
		return stageError("analyze", err)
	}
	_, err := s.model(write, llm.StageNameWrite)
	return stageError("plan", err)
}
func (s *Service) ObserveChunk(ctx context.Context, model llm.ModelRef, input clip.ChunkInput) (clip.ChunkAnalysis, llm.Usage, error) {
	if err := ctx.Err(); err != nil {
		return clip.ChunkAnalysis{}, llm.Usage{}, err
	}
	if err := clip.ValidateChunkInput(s.cfg.Analysis, input); err != nil {
		return clip.ChunkAnalysis{}, llm.Usage{}, err
	}
	info, err := s.model(model, llm.StageNameObserve)
	if err != nil {
		return clip.ChunkAnalysis{}, llm.Usage{}, stageError("analyze", err)
	}
	execution, err := executionPolicy(input.Policy, model, llm.StageNameObserve, s.cfg.ObserveCompletionTokens, llm.ExecutionInlineStatic)
	if err != nil {
		return clip.ChunkAnalysis{}, llm.Usage{}, err
	}
	system, user := BuildObservePrompt(input)
	request := llm.Request{System: system, Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.InlineVideoPart(input.Video), llm.TextPart(user)}}}, Stage: llm.StageNameObserve, Reasoning: input.Policy.Reasoning, DisableReasoning: input.Policy.DisableReasoning, MaxTokens: input.Policy.CompletionTokens, Execution: execution}
	if info.StructuredOutput {
		request.JSONSchema = ChunkSchema()
	}
	if !execution.Matches(model, request) || input.Video.Size > 8<<20 || !boundedPrompt(system, user, request.JSONSchema, llm.ExecutionInlineStatic) {
		return clip.ChunkAnalysis{}, llm.Usage{}, clip.ErrInvalid
	}
	response, err := s.models.Complete(ctx, model, request)
	if err != nil {
		return clip.ChunkAnalysis{}, response.Usage, stageError("analyze", err)
	}
	result, err := parseChunk(s.cfg, input, response.Text)
	return result, response.Usage, stageError("analyze", llm.ResponseParseError(response, err))
}
func (s *Service) Plan(ctx context.Context, model llm.ModelRef, input clip.PlanningInput) (clip.EditPlan, llm.Usage, error) {
	if err := ctx.Err(); err != nil {
		return clip.EditPlan{}, llm.Usage{}, err
	}
	if err := validateInput(s.cfg, input); err != nil {
		return clip.EditPlan{}, llm.Usage{}, err
	}
	info, err := s.model(model, llm.StageNameWrite)
	if err != nil {
		return clip.EditPlan{}, llm.Usage{}, stageError("plan", err)
	}
	execution, err := executionPolicy(input.Policy, model, llm.StageNameWrite, s.cfg.PlanCompletionTokens, llm.ExecutionTextOnly)
	if err != nil {
		return clip.EditPlan{}, llm.Usage{}, err
	}
	system, user := BuildPlanPrompt(input, s.cfg.Render.FadeMS)
	request := llm.Request{System: system, Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart(user)}}}, Stage: llm.StageNameWrite, Reasoning: input.Policy.Reasoning, DisableReasoning: input.Policy.DisableReasoning, MaxTokens: input.Policy.CompletionTokens, Execution: execution}
	if info.StructuredOutput {
		request.JSONSchema = PlanSchema()
	}
	if !boundedPrompt(system, user, request.JSONSchema, llm.ExecutionTextOnly) {
		return clip.EditPlan{}, llm.Usage{}, clip.ErrInvalid
	}
	response, err := s.models.Complete(ctx, model, request)
	if err != nil {
		return clip.EditPlan{}, response.Usage, stageError("plan", err)
	}
	result, err := parsePlan(s.cfg, input, response.Text)
	if err != nil {
		return clip.EditPlan{}, response.Usage, stageError("plan", llm.ResponseParseError(response, err))
	}
	if err := s.compose(ctx, input, &result); err != nil {
		return clip.EditPlan{}, response.Usage, stageError("plan", err)
	}
	if err := validatePlan(s.cfg, input, result); err != nil {
		return clip.EditPlan{}, response.Usage, stageError("plan", llm.ResponseParseError(response, err))
	}
	return result, response.Usage, nil
}

// compose is where every placement decision is made — by the CDS tables, never
// by the model. It measures each candidate plate through the renderer's own
// shaping, so the anchor it picks is the anchor that actually fits.
func (s *Service) compose(ctx context.Context, input clip.PlanningInput, plan *clip.EditPlan) error {
	canvas, _ := clip.ClipCanvas(input.Ratio)
	byID := map[string]clip.SourceAnalysis{}
	for _, a := range input.Analyses {
		byID[a.Source.ID] = a
	}
	measured := func(c clip.Caption) (clip.Region, bool, error) {
		width, height, err := s.captions.CaptionSize(ctx, input.Ratio, c)
		if err != nil {
			if errors.Is(err, clip.ErrInvalid) {
				return clip.Region{}, false, outputError("caption_measurement")
			}
			return clip.Region{}, false, err
		}
		if width <= 0 || height <= 0 {
			return clip.Region{}, false, nil
		}
		region, err := clip.PlaceCopy(canvas, c.Anchor, c.Align, width, height)
		return region, err == nil, nil
	}
	// The plan's own preset and facts decide which chips a cut can carry.
	plan.Preset, plan.Facts, plan.Disclosure = input.Template.Preset, input.Answers, input.Disclosure
	plan.CTA, plan.Accent = input.CTA, input.Template.Accent
	// CDS-41 may lengthen a cut to fit its copy, but the timeline is already
	// reconciled to the owner's approved target, so the extension may only use
	// the slack that target still allows.
	slack := min(s.cfg.TargetToleranceMS-abs(plan.DurationMS-input.TargetDurationMS), s.cfg.Render.MaxDurationMS-plan.DurationMS)
	// The hook is dropped rather than shown ungrounded (CDS-42), and it has to be
	// settled before the cards are measured, since a card without a hook
	// sentence is no card at all (CDS-28).
	// Two lines of nine, which is what the hook card sets (CDS-28).
	if !clip.Grounded(plan.Hook, input.Answers) || clip.CopyChars(plan.Hook) > 2*design.Type["hook"].Chars {
		plan.Hook = ""
	}
	cards, err := s.captions.CardElements(ctx, *plan)
	if err != nil {
		return err
	}
	history, previous := design.StyleHistory{}, ""
	// The badge and the chips are placed before any copy, and copy yields to
	// them (CDS-45); T107's cards join this list.
	for i, cut := range plan.Cuts {
		analysis := byID[cut.SourceID]
		scene, readable := clip.CutScene(cut, analysis)
		written := clip.Written{Answers: input.Answers}
		if i < len(plan.Written) {
			written = plan.Written[i]
			written.Answers = input.Answers
		}
		// A cut may only be extended inside the segment it already came from, and
		// never past the source or the target's remaining slack.
		limit := cut.EndMS
		for _, seg := range analysis.Segments {
			if seg.StartMS <= cut.StartMS && seg.EndMS > limit {
				limit = seg.EndMS
			}
		}
		limit = min(limit, analysis.Source.Info.DurationMS, cut.EndMS+max(0, slack))
		// CDS-37's ceiling holds through the exposure extension too: a cut may
		// pass its scene's maximum only by what the copy's own minimum needs.
		_, maximum := design.CutBounds(scene)
		limit = min(limit, cut.StartMS+max(maximum, clip.MinExposureMS(written.Text)+design.Timing.SubExtendMS+2*design.Timing.CopyLeadMS))
		placed, err := s.captions.FixedElements(ctx, input.Ratio, input.Disclosure, plan.ChipLabels(cut), input.Answers)
		if err != nil {
			return err
		}
		// The two cards cover the middle of the first and the last cut, so a
		// copy on those cuts has to go somewhere else (CDS-45).
		for _, e := range cards {
			if e.Cut == i {
				placed = append(placed, e)
			}
		}
		composed, decision, err := clip.Compose(canvas, cut, written, scene, readable, clip.CutSubject(canvas, cut, analysis), placed, input.Template.CopyStyles, input.Template.Accent, history, previous, limit, measured)
		if err != nil {
			return err
		}
		grown := composed.EndMS - cut.EndMS
		slack, plan.DurationMS = slack-grown, plan.DurationMS+grown
		plan.Cuts[i] = composed
		plan.Decisions = append(plan.Decisions, decision)
		// Both copies count against CDS-40's frequency guards, and the anchor a
		// later cut steps from is the LAST one this cut showed.
		for _, copy := range composed.Placed() {
			history = append(history, copy.Style)
			previous = copy.Anchor
		}
	}
	// The guards above make V13 and V14 hold by construction; this is what
	// catches anything they do not, before a single byte is downloaded.
	sources := make([]clip.RenderSource, 0, len(input.Analyses))
	for _, a := range input.Analyses {
		sources = append(sources, a.Source.RenderSource)
	}
	plan.Styles = input.Template.CopyStyles
	_, err = s.captions.Layout(ctx, *plan, sources)
	return err
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
func executionPolicy(p llm.CallPolicy, ref llm.ModelRef, stage string, budget int, delivery llm.ExecutionDelivery) (*llm.ExecutionPolicy, error) {
	if !p.Valid() || !p.Pricing.Valid() || p.Pricing.Delivery != delivery || p.Ref != ref || p.Stage != stage || p.CompletionTokens != budget {
		return nil, clip.ErrPricingUnavailable
	}
	return &llm.ExecutionPolicy{Call: p, Delivery: delivery, NoFallback: true, RequireParameters: true}, nil
}

func boundedPrompt(system, user string, schema json.RawMessage, delivery llm.ExecutionDelivery) bool {
	// Account for JSON escaping and reserve request/routing overhead. Never
	// silently truncate observations or make an extra paid summarization call.
	data, err := json.Marshal(struct {
		System, User string
		Schema       json.RawMessage
	}{system, user, schema})
	limit := llm.ClipInputUnits - 2048
	if delivery == llm.ExecutionInlineStatic {
		limit -= 20000
	}
	return err == nil && len(data) <= limit
}
func within(value string, minimum, maximum int) bool {
	return utf8.ValidString(value) && utf8.RuneCountInString(value) >= minimum && utf8.RuneCountInString(value) <= maximum
}
func validateSettings(cfg Config, in clip.PlanningInput) error {
	if _, err := clip.ClipCanvas(in.Ratio); err != nil {
		return err
	}
	if in.TargetDurationMS < cfg.Render.MinDurationMS || in.TargetDurationMS > cfg.Render.MaxDurationMS || !clip.ValidCopyStyles(in.Template.CopyStyles) || !clip.ValidAccent(in.Template.Accent) || !within(in.Template.Name, 1, cfg.Template.NameChars) || !within(in.Template.CutGuidance, 0, cfg.Template.GuidanceChars) || len(in.Template.InformationFields) > cfg.Template.FieldCount || len(in.Answers) != len(in.Template.InformationFields) {
		return clip.ErrInvalid
	}
	fields := map[string]bool{}
	for _, f := range in.Template.InformationFields {
		if fields[f.Label] || !within(f.Label, 1, cfg.Template.LabelChars) || !within(f.Prompt, 1, cfg.Template.PromptChars) {
			return clip.ErrInvalid
		}
		fields[f.Label] = true
	}
	for _, a := range in.Answers {
		if !fields[a.Label] || !within(a.Text, 0, cfg.Template.AnswerChars) {
			return clip.ErrInvalid
		}
		delete(fields, a.Label)
	}
	return nil
}

func validateInput(cfg Config, in clip.PlanningInput) error {
	if err := validateSettings(cfg, in); err != nil {
		return err
	}
	sources := make([]clip.AnalysisSource, 0, len(in.Analyses))
	for _, a := range in.Analyses {
		sources = append(sources, a.Source)
	}
	if err := clip.ValidateAnalysisSources(cfg.Analysis, sources); err != nil {
		return err
	}
	for _, a := range in.Analyses {
		limits := cfg.Analysis
		limits.MaxSegments *= (a.Source.Info.DurationMS + limits.ChunkMS - 1) / limits.ChunkMS
		if err := clip.ValidateSegments(limits, a.Segments, 0, a.Source.Info.DurationMS); err != nil {
			return err
		}
	}
	return nil
}
func validatePlan(cfg Config, in clip.PlanningInput, plan clip.EditPlan) error {
	if plan.Ratio != in.Ratio {
		return outputError("plan_ratio")
	}
	// The compiler holds the target from above; from below it may deliver
	// less when the footage ran out, so only an overrun is the model's error.
	if plan.DurationMS-in.TargetDurationMS > cfg.TargetToleranceMS {
		return outputError("plan_target_duration")
	}
	sources := make([]clip.RenderSource, 0, len(in.Analyses))
	for _, a := range in.Analyses {
		sources = append(sources, a.Source.RenderSource)
	}
	if err := clip.ValidateEditPlan(cfg.Render, plan, sources); err != nil {
		if errors.Is(err, clip.ErrCopyTooLong) {
			return err
		}
		return fmt.Errorf("%w: %w", llm.ErrBadOutput, err)
	}
	for _, cut := range plan.Cuts {
		// A cut whose copy the compiler dropped has no style and no accent to
		// check (CDS-41's last fallback).
		for _, copy := range cut.Placed() {
			if !slices.Contains(in.Template.CopyStyles, copy.Style) {
				return outputError("plan_style")
			}
			if copy.Accent != "" && copy.Accent != in.Template.Accent {
				return outputError("plan_accent")
			}
		}
	}
	return nil
}
