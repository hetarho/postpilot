package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
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
	FixedElements(ctx context.Context, ratio, disclosure string, labels []string, answers []clip.Answer, hideDisclosure ...bool) (clip.Manifest, error)
	Layout(ctx context.Context, plan clip.EditPlan, sources []clip.RenderSource) (clip.EditPlan, clip.Manifest, error)
}
type Config struct {
	Analysis                                                                                          clip.AnalysisLimits
	Render                                                                                            clip.RenderConfig
	Template                                                                                          clip.Limits
	ObserveCompletionTokens, PlanCompletionTokens, MaxResponseBytes, MaxCutIDRunes, TargetToleranceMS int
	// One budget per writing call (CLIP-135). Both are generous first and come
	// down on measured usage; PlanCompletionTokens stays for the legacy writer.
	FlowCompletionTokens, NarrationCompletionTokens int
	ObserveReasoning, PlanReasoning                 llm.ReasoningEffort
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
	for _, n := range []int{cfg.Analysis.ChunkMS, cfg.Analysis.MaxSources, cfg.Analysis.MaxSourceDurationMS, cfg.Analysis.MaxSegments, cfg.Analysis.MaxTextRunes, cfg.Analysis.MaxSubjects, cfg.ObserveCompletionTokens, cfg.PlanCompletionTokens, cfg.FlowCompletionTokens, cfg.NarrationCompletionTokens, cfg.MaxResponseBytes, cfg.MaxCutIDRunes, cfg.TargetToleranceMS, cfg.Render.MaxCuts, cfg.Render.MaxCopyRunes, cfg.Template.NameChars, cfg.Template.AnswerChars} {
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
	return Budgets{Observe: s.cfg.ObserveCompletionTokens, Flow: s.cfg.FlowCompletionTokens, Narration: s.cfg.NarrationCompletionTokens}
}
func (s *Service) CompositionPlanVersion() int { return clip.CompositionPlanVersion }

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
	// The catalog's raw modality is the only gate here; whether a current endpoint
	// takes the bounded inline clip is proven when the price is frozen and
	// rechecked before the call (CLIP-30). No delivery-profile flag admits or
	// refuses a model.
	if stage == llm.StageNameObserve {
		if !info.VideoInput {
			return info, llm.ErrVideoInputAbsent
		}
		if !info.Vision {
			return info, clip.ErrModelInputUnsupported
		}
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
	// The schema goes when the FROZEN policy says so: a quote priced for the
	// parser fallback never executes with a schema parameter nobody qualified,
	// and a model that lost the capability since the quote is refused, not
	// silently downgraded.
	if execution.Call.StructuredOutput {
		if !info.StructuredOutput {
			return clip.ChunkAnalysis{}, llm.Usage{}, clip.ErrPricingUnavailable
		}
		request.JSONSchema = ChunkSchema()
	}
	if !execution.Matches(model, request) || input.Video.Size > 8<<20 {
		return clip.ChunkAnalysis{}, llm.Usage{}, clip.ErrInvalid
	}
	if err := validatePrompt(system, user, request.JSONSchema, llm.ExecutionInlineStatic, input.Policy.InputTokenLimit()); err != nil {
		return clip.ChunkAnalysis{}, llm.Usage{}, err
	}
	result, usage, err := completeValidated(ctx, s, model, request, user, input.Policy, func(raw string) (clip.ChunkAnalysis, error) { return parseChunk(s.cfg, input, raw) })
	return result, usage, stageError("analyze", err)
}

// Flow is the FIRST writing call of a generation (CLIP-135): it writes the
// footage flow — which observed spans play, in what order, at what rate — and
// returns a plan holding those cuts and the template regions the server can
// already state in full. It writes no caption: the narration is written over
// this flow once the server has resolved it into exact output intervals.
func (s *Service) Flow(ctx context.Context, model llm.ModelRef, input clip.PlanningInput) (clip.EditPlan, llm.Usage, error) {
	if err := ctx.Err(); err != nil {
		return clip.EditPlan{}, llm.Usage{}, err
	}
	if !nativeComposition(input) {
		return clip.EditPlan{}, llm.Usage{}, stageError("flow", clip.ErrInvalid)
	}
	if err := validateInput(s.cfg, input); err != nil {
		return clip.EditPlan{}, llm.Usage{}, err
	}
	info, err := s.model(model, llm.StageNameWrite)
	if err != nil {
		return clip.EditPlan{}, llm.Usage{}, stageError("flow", err)
	}
	execution, err := executionPolicy(input.Policy, model, llm.StageNameWrite, s.cfg.FlowCompletionTokens, llm.ExecutionTextOnly)
	if err != nil {
		return clip.EditPlan{}, llm.Usage{}, err
	}
	system, user := buildFlowPrompt(input, s.cfg.Render.FadeMS, compositionLimits(s.cfg, input))
	request := llm.Request{System: system, Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart(user)}}}, Stage: llm.StageNameWrite, Reasoning: input.Policy.Reasoning, DisableReasoning: input.Policy.DisableReasoning, MaxTokens: input.Policy.CompletionTokens, Execution: execution}
	if execution.Call.StructuredOutput {
		if !info.StructuredOutput {
			return clip.EditPlan{}, llm.Usage{}, clip.ErrPricingUnavailable
		}
		request.JSONSchema = FlowSchema()
	}
	if err := validatePrompt(system, user, request.JSONSchema, llm.ExecutionTextOnly, input.Policy.InputTokenLimit()); err != nil {
		return clip.EditPlan{}, llm.Usage{}, err
	}
	result, usage, err := completeValidated(ctx, s, model, request, user, input.Policy, func(raw string) (clip.EditPlan, error) {
		return parseFlowPlan(s.cfg, input, raw)
	})
	if err != nil {
		return clip.EditPlan{}, usage, stageError("flow", err)
	}
	if err := validatePlan(s.cfg, input, result); err != nil {
		return clip.EditPlan{}, usage, stageError("flow", planFailure(err, input, result, "validation", 0))
	}
	return result, usage, nil
}

// Narrate is the SECOND writing call of a generation (CLIP-135): it writes what
// is said over the flow the server has already resolved — captions on absolute
// output intervals, and the template's own generated slot rows. It may not
// change a cut, and the flow it is given is the flow it writes over.
func (s *Service) Narrate(ctx context.Context, model llm.ModelRef, input clip.NarrationInput) (clip.EditPlan, llm.Usage, error) {
	if err := ctx.Err(); err != nil {
		return clip.EditPlan{}, llm.Usage{}, err
	}
	if !nativeComposition(input.PlanningInput) || input.Flow.Portable == nil || len(input.Flow.Cuts) == 0 || input.Flow.DurationMS <= 0 {
		return clip.EditPlan{}, llm.Usage{}, stageError("narrate", clip.ErrInvalid)
	}
	if err := validateInput(s.cfg, input.PlanningInput); err != nil {
		return clip.EditPlan{}, llm.Usage{}, err
	}
	info, err := s.model(model, llm.StageNameWrite)
	if err != nil {
		return clip.EditPlan{}, llm.Usage{}, stageError("narrate", err)
	}
	execution, err := executionPolicy(input.Policy, model, llm.StageNameWrite, s.cfg.NarrationCompletionTokens, llm.ExecutionTextOnly)
	if err != nil {
		return clip.EditPlan{}, llm.Usage{}, err
	}
	system, user := buildNarrationPrompt(input, compositionLimits(s.cfg, input.PlanningInput))
	request := llm.Request{System: system, Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart(user)}}}, Stage: llm.StageNameWrite, Reasoning: input.Policy.Reasoning, DisableReasoning: input.Policy.DisableReasoning, MaxTokens: input.Policy.CompletionTokens, Execution: execution}
	if execution.Call.StructuredOutput {
		if !info.StructuredOutput {
			return clip.EditPlan{}, llm.Usage{}, clip.ErrPricingUnavailable
		}
		request.JSONSchema = NarrationSchema()
	}
	if err := validatePrompt(system, user, request.JSONSchema, llm.ExecutionTextOnly, input.Policy.InputTokenLimit()); err != nil {
		return clip.EditPlan{}, llm.Usage{}, err
	}
	result, usage, err := completeValidated(ctx, s, model, request, user, input.Policy, func(raw string) (clip.EditPlan, error) {
		return parseNarration(s.cfg, input, raw)
	})
	if err != nil {
		return clip.EditPlan{}, usage, stageError("narrate", err)
	}
	if err := validatePlan(s.cfg, input.PlanningInput, result); err != nil {
		return clip.EditPlan{}, usage, stageError("narrate", planFailure(err, input.PlanningInput, result, "validation", 0))
	}
	return result, usage, nil
}

func (s *Service) Plan(ctx context.Context, model llm.ModelRef, input clip.PlanningInput) (clip.EditPlan, llm.Usage, error) {
	if err := ctx.Err(); err != nil {
		return clip.EditPlan{}, llm.Usage{}, err
	}
	// A composition is written by the two calls the assembly contract names —
	// Flow, then Narrate (CLIP-135). This writer is what remains for the legacy
	// non-native payloads, which carry no composition snapshot at all.
	if nativeComposition(input) {
		return clip.EditPlan{}, llm.Usage{}, stageError("plan", clip.ErrCompositionUnavailable)
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
	system, user := BuildPlanPrompt(input, s.cfg.Render.FadeMS, compositionLimits(s.cfg, input))
	request := llm.Request{System: system, Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart(user)}}}, Stage: llm.StageNameWrite, Reasoning: input.Policy.Reasoning, DisableReasoning: input.Policy.DisableReasoning, MaxTokens: input.Policy.CompletionTokens, Execution: execution}
	if execution.Call.StructuredOutput {
		if !info.StructuredOutput {
			return clip.EditPlan{}, llm.Usage{}, clip.ErrPricingUnavailable
		}
		request.JSONSchema = PlanSchema()
		if nativeComposition(input) {
			request.JSONSchema = CompositionPlanSchema()
		}
	}
	if err := validatePrompt(system, user, request.JSONSchema, llm.ExecutionTextOnly, input.Policy.InputTokenLimit()); err != nil {
		return clip.EditPlan{}, llm.Usage{}, err
	}
	result, usage, err := completeValidated(ctx, s, model, request, user, input.Policy, func(raw string) (clip.EditPlan, error) {
		if nativeComposition(input) {
			return parseCompositionPlan(s.cfg, input, raw)
		}
		return parsePlan(s.cfg, input, raw)
	})
	if err != nil {
		return clip.EditPlan{}, usage, stageError("plan", err)
	}
	if !nativeComposition(input) {
		if err := s.compose(ctx, input, &result); err != nil {
			return clip.EditPlan{}, usage, stageError("plan", err)
		}
	}
	if !nativeComposition(input) && result.DurationMS-input.TargetDurationMS > s.cfg.TargetToleranceMS {
		trimGeneratedOverrun(&result, input.TargetDurationMS)
	}
	removeInvalidGeneratedCopies(s.cfg, input, &result)
	clip.RecomputePlanNotices(&result, input.TargetDurationMS, s.cfg.TargetToleranceMS)
	if err := validatePlan(s.cfg, input, result); err != nil {
		return clip.EditPlan{}, usage, stageError("plan", planFailure(err, input, result, "validation", 0))
	}
	return result, usage, nil
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
	plan.HideDisclosure = input.HideDisclosure
	plan.Preset, plan.Facts, plan.Disclosure = input.Template.Preset, input.Answers, input.Disclosure
	plan.CTA, plan.Accent = input.CTA, input.Template.Accent
	// CDS-41 may lengthen a cut to fit its copy, but the timeline is already
	// reconciled to the owner's approved target, so the extension may only use
	// the slack that target still allows.
	slack := min(s.cfg.TargetToleranceMS-abs(plan.DurationMS-input.TargetDurationMS), s.cfg.Render.MaxDurationMS-plan.DurationMS)
	previous := ""
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
		_, maximum := design.CutBounds(scene, plan.Preset)
		limit = min(limit, cut.StartMS+max(maximum, clip.MinExposureMS(written.Text)+design.Timing.SubExtendMS+2*design.Timing.CopyLeadMS))
		placed, err := s.captions.FixedElements(ctx, input.Ratio, input.Disclosure, plan.ChipLabels(cut), input.Answers, input.HideDisclosure)
		if err != nil {
			return err
		}
		written.Pace = input.Template.CaptionPace
		composed, decision, err := clip.Compose(canvas, cut, written, scene, readable, clip.CutSubject(canvas, cut, analysis), placed, input.Template.Accent, previous, limit, measured)
		if err != nil {
			return err
		}
		grown := composed.EndMS - cut.EndMS
		slack, plan.DurationMS = slack-grown, plan.DurationMS+grown
		plan.Cuts[i] = composed
		plan.Decisions = append(plan.Decisions, decision)
		// The next cut steps from the last caption this cut showed.
		for _, copy := range composed.Placed() {
			previous = copy.Anchor
		}
	}
	// The guards above make V13 and V14 hold by construction; this is what
	// catches anything they do not, before a single byte is downloaded.
	sources := make([]clip.RenderSource, 0, len(input.Analyses))
	for _, a := range input.Analyses {
		sources = append(sources, a.Source.RenderSource)
	}
	repaired, _, err := s.captions.Layout(ctx, *plan, sources)
	if err != nil {
		return err
	}
	// The ladder may have moved a style or an anchor or dropped a copy; the plan
	// that is stored and rendered is the one that verified.
	*plan = repaired
	return nil
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

// PromptBytes is the encoded size validatePrompt measures, so a caller can
// state an allowance in the same units the refusal reports.
func PromptBytes(system, user string, schema json.RawMessage) int {
	data, err := json.Marshal(struct {
		System, User string
		Schema       json.RawMessage
	}{system, user, schema})
	if err != nil {
		return 0
	}
	return len(data)
}

func validatePrompt(system, user string, schema json.RawMessage, delivery llm.ExecutionDelivery, inputLimit int) error {
	// The conservative encoded-byte bound is part of the approved input budget.
	// Do not discard completed observations or authored content to make it fit.
	data, err := json.Marshal(struct {
		System, User string
		Schema       json.RawMessage
	}{system, user, schema})
	limit := inputLimit - 2048
	if delivery == llm.ExecutionInlineStatic {
		limit -= 20000
	}
	if err != nil || limit < 0 || len(data) > limit {
		return clip.WithAttemptDiagnostic(clip.ErrInputTooLarge, clip.AttemptDiagnostic{Check: "input_prompt_limit", Phase: "input", Values: clip.SafeAttemptValues(map[string]int{"input_bytes": len(data), "input_limit_bytes": max(0, limit), "system_bytes": len(system), "content_bytes": len(user), "schema_bytes": len(schema)})})
	}
	return nil
}
func inputFailure(err error, check string, source int) error {
	if d, ok := clip.DiagnosticFromError(err); ok {
		d.Values = clip.SafeAttemptValues(d.Values)
		if source > 0 {
			d.Values["source"] = source
		}
		return clip.WithAttemptDiagnostic(err, d)
	}
	values := map[string]int{}
	if source > 0 {
		values["source"] = source
	}
	return clip.WithAttemptDiagnostic(err, clip.AttemptDiagnostic{Check: check, Phase: "input", Values: values})
}
func within(value string, minimum, maximum int) bool {
	return utf8.ValidString(value) && utf8.RuneCountInString(value) >= minimum && utf8.RuneCountInString(value) <= maximum
}
func validateSettings(cfg Config, in clip.PlanningInput) error {
	if _, err := clip.ClipCanvas(in.Ratio); err != nil {
		return err
	}
	if nativeComposition(in) {
		if in.TargetDurationMS < cfg.Render.MinDurationMS || in.TargetDurationMS > cfg.Render.MaxDurationMS || in.Composition.Snapshot.Version != clip.CompositionVersion || !within(in.Template.Name, 1, cfg.Template.NameChars) || !in.Composition.Snapshot.Legacy && in.Template.CompositionBody != in.Composition.Snapshot.Body {
			return clip.ErrInvalid
		}
		doc, problem := composition.Parse(in.Composition.Snapshot.Body, compositionLimits(cfg, in))
		if problem != nil {
			return problem
		}
		return clip.ValidateCompositionInputs(doc, in.Composition.Inputs, compositionLimits(cfg, in), !in.Composition.Snapshot.Legacy)
	}
	if in.TargetDurationMS < cfg.Render.MinDurationMS || in.TargetDurationMS > cfg.Render.MaxDurationMS || !clip.ValidCaptionPace(in.Template.CaptionPace) || !clip.ValidAccent(in.Template.Accent) || !within(in.Template.Name, 1, cfg.Template.NameChars) || !within(in.Template.CutGuidance, 0, cfg.Template.GuidanceChars) || len(in.Template.InformationFields) > cfg.Template.FieldCount || len(in.Answers) != len(in.Template.InformationFields) {
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
		return inputFailure(err, "input_settings", 0)
	}
	sources := make([]clip.AnalysisSource, 0, len(in.Analyses))
	for _, a := range in.Analyses {
		sources = append(sources, a.Source)
	}
	if err := clip.ValidateAnalysisSources(cfg.Analysis, sources); err != nil {
		return inputFailure(err, "input_sources", 0)
	}
	for index, a := range in.Analyses {
		limits := cfg.Analysis
		limits.MaxSegments *= (a.Source.Info.DurationMS + limits.ChunkMS - 1) / limits.ChunkMS
		if err := clip.ValidateSegments(limits, a.Segments, 0, a.Source.Info.DurationMS); err != nil {
			return inputFailure(err, "input_sources", index+1)
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
			if copy.Accent != "" && copy.Accent != in.Template.Accent {
				return outputError("plan_accent")
			}
		}
	}
	return nil
}
