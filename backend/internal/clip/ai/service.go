package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/llm"
)

type Models interface {
	Resolve(llm.ModelRef) (llm.ModelInfo, bool)
	Complete(context.Context, llm.ModelRef, llm.Request) (llm.Response, error)
}
type Config struct {
	Analysis                                                                    clip.AnalysisLimits
	Render                                                                      clip.RenderConfig
	Template                                                                    clip.Limits
	ObserveCompletionTokens, MaxResponseBytes, MaxCutIDRunes, TargetToleranceMS int
	// One budget per writing call (CLIP-135). Both are generous first and come
	// down on measured usage.
	FlowCompletionTokens, NarrationCompletionTokens int
	ObserveReasoning, PlanReasoning                 llm.ReasoningEffort
}

// Budgets is copied into the durable job's planned calls before any provider work.
// The request path reads these same values, never the registry's default budget.
type Budgets = clip.CompletionBudgets
type Service struct {
	models Models
	cfg    Config
}

func New(models Models, cfg Config) (*Service, error) {
	for _, n := range []int{cfg.Analysis.ChunkMS, cfg.Analysis.MaxSources, cfg.Analysis.MaxSourceDurationMS, cfg.Analysis.MaxSegments, cfg.Analysis.MaxTextRunes, cfg.Analysis.MaxSubjects, cfg.ObserveCompletionTokens, cfg.FlowCompletionTokens, cfg.NarrationCompletionTokens, cfg.MaxResponseBytes, cfg.MaxCutIDRunes, cfg.TargetToleranceMS, cfg.Render.MaxCuts, cfg.Render.MaxCopyRunes, cfg.Template.NameChars, cfg.Template.AnswerChars} {
		if n <= 0 {
			return nil, errors.New("invalid clip AI configuration")
		}
	}
	if models == nil || !cfg.ObserveReasoning.Valid() || !cfg.PlanReasoning.Valid() {
		return nil, errors.New("clip AI requires models")
	}
	return &Service{models, cfg}, nil
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
	request := llm.Request{Composition: clipComposition("observe", system, user, clip.VideoGuidelines{}, []string{input.Source.ID}), System: system, Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.InlineVideoPart(input.Video), llm.TextPart(user)}}}, Stage: llm.StageNameObserve, Reasoning: input.Policy.Reasoning, DisableReasoning: input.Policy.DisableReasoning, MaxTokens: input.Policy.CompletionTokens, Execution: execution}
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
	mode := flowMode(input)
	system, user := BuildFlowPrompt(input, s.cfg.Render.FadeMS, compositionLimits(s.cfg, input))
	request := llm.Request{Composition: clipComposition(mode, system, user, input.Guidelines, planningSourceRefs(input)), System: system, Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart(user)}}}, Stage: llm.StageNameWrite, Reasoning: input.Policy.Reasoning, DisableReasoning: input.Policy.DisableReasoning, MaxTokens: input.Policy.CompletionTokens, Execution: execution}
	if execution.Call.StructuredOutput {
		if !info.StructuredOutput {
			return clip.EditPlan{}, llm.Usage{}, clip.ErrPricingUnavailable
		}
		// Built from a storyline, the flow writes none of its own (CLIP-178).
		request.JSONSchema = FlowSchema()
		if input.FollowStoryline != nil {
			request.JSONSchema = RevisionFlowSchema()
		}
	}
	if err := validatePrompt(system, user, request.JSONSchema, llm.ExecutionTextOnly, input.Policy.InputTokenLimit()); err != nil {
		return clip.EditPlan{}, llm.Usage{}, err
	}
	result, usage, err := completeValidated(ctx, s, model, request, user, input.Policy, func(raw string) (clip.EditPlan, error) {
		return parseFlowPlan(s.cfg, input, raw, input.FollowStoryline == nil)
	})
	if err != nil {
		return clip.EditPlan{}, usage, stageError("flow", err)
	}
	if err := validatePlan(s.cfg, input, result); err != nil {
		return clip.EditPlan{}, usage, stageError("flow", planFailure(err, input, result, "validation", 0))
	}
	return result, usage, nil
}

// Revise answers one owner-written revision request (CLIP-131). It is the same
// two writing calls, on the same contracts, given the plan as the owner's own
// edits left it: a flow target rewrites the footage and then the narration over
// it, because captions timed against cuts that moved are timed against nothing;
// a narration target rewrites what is said and leaves every cut alone.
func (s *Service) Revise(ctx context.Context, model llm.ModelRef, input clip.RevisionInput) (clip.EditPlan, llm.Usage, error) {
	if err := ctx.Err(); err != nil {
		return clip.EditPlan{}, llm.Usage{}, err
	}
	if !nativeComposition(input.PlanningInput) || !clip.ValidRevisionTarget(input.Target) ||
		input.Current.Portable == nil || len(input.Current.Cuts) == 0 || input.Current.DurationMS <= 0 ||
		!within(input.Request, 1, s.cfg.Template.InstructionChars) {
		return clip.EditPlan{}, llm.Usage{}, stageError("flow", clip.ErrInvalid)
	}
	if input.Current.Narration != nil {
		return s.reviseSpoken(ctx, model, input)
	}
	usage := llm.Usage{}
	flow := input.Current
	if input.Target != clip.RevisionNarration {
		written, spent, err := s.write(ctx, model, input.PlanningInput, "flow", "flow-revision", s.cfg.FlowCompletionTokens, RevisionFlowSchema(), func() (string, string) {
			return buildFlowRevisionPrompt(input, s.cfg.Render.FadeMS, compositionLimits(s.cfg, input.PlanningInput))
		}, func(raw string) (clip.EditPlan, error) { return parseFlowPlan(s.cfg, input.PlanningInput, raw, false) })
		usage = addUsage(usage, spent)
		if err != nil {
			return clip.EditPlan{}, usage, err
		}
		flow = written
	}
	result, spent, err := s.write(ctx, model, input.PlanningInput, "narrate", "narration-revision", s.cfg.NarrationCompletionTokens, NarrationSchema(), func() (string, string) {
		return buildNarrationRevisionPrompt(input, flow, compositionLimits(s.cfg, input.PlanningInput))
	}, func(raw string) (clip.EditPlan, error) {
		return parseNarration(s.cfg, clip.NarrationInput{PlanningInput: input.PlanningInput, Flow: flow}, raw)
	})
	usage = addUsage(usage, spent)
	if err != nil {
		return clip.EditPlan{}, usage, err
	}
	if err := validatePlan(s.cfg, input.PlanningInput, result); err != nil {
		return clip.EditPlan{}, usage, stageError("narrate", planFailure(err, input.PlanningInput, result, "validation", 0))
	}
	return result, usage, nil
}

// addUsage sums what two calls of one revision spent. A provider that reported
// no cost for either leaves the pair reporting none, rather than claiming zero.
func addUsage(a, b llm.Usage) llm.Usage {
	return llm.Usage{
		PromptTokens:     a.PromptTokens + b.PromptTokens,
		CompletionTokens: a.CompletionTokens + b.CompletionTokens,
		ReasoningTokens:  a.ReasoningTokens + b.ReasoningTokens,
		CostMicrousd:     a.CostMicrousd + b.CostMicrousd,
		CostReported:     a.CostReported || b.CostReported,
	}
}

// write is the one path every writing call takes: the frozen policy, the model's
// own structured-output capability, the input allowance, and CLIP-94's bounded
// correction loop around one parse.
func (s *Service) write(ctx context.Context, model llm.ModelRef, in clip.PlanningInput, stage, mode string, budget int, schema []byte, prompt func() (string, string), parse func(string) (clip.EditPlan, error)) (clip.EditPlan, llm.Usage, error) {
	if err := validateInput(s.cfg, in); err != nil {
		return clip.EditPlan{}, llm.Usage{}, err
	}
	info, err := s.model(model, llm.StageNameWrite)
	if err != nil {
		return clip.EditPlan{}, llm.Usage{}, stageError(stage, err)
	}
	execution, err := executionPolicy(in.Policy, model, llm.StageNameWrite, budget, llm.ExecutionTextOnly)
	if err != nil {
		return clip.EditPlan{}, llm.Usage{}, err
	}
	system, user := prompt()
	request := llm.Request{Composition: clipComposition(mode, system, user, in.Guidelines, planningSourceRefs(in)), System: system, Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart(user)}}}, Stage: llm.StageNameWrite, Reasoning: in.Policy.Reasoning, DisableReasoning: in.Policy.DisableReasoning, MaxTokens: in.Policy.CompletionTokens, Execution: execution}
	if execution.Call.StructuredOutput {
		if !info.StructuredOutput {
			return clip.EditPlan{}, llm.Usage{}, clip.ErrPricingUnavailable
		}
		request.JSONSchema = schema
	}
	if err := validatePrompt(system, user, request.JSONSchema, llm.ExecutionTextOnly, in.Policy.InputTokenLimit()); err != nil {
		return clip.EditPlan{}, llm.Usage{}, err
	}
	result, usage, err := completeValidated(ctx, s, model, request, user, in.Policy, parse)
	return result, usage, stageError(stage, err)
}

// Storyline is the storyline call (CLIP-177, CLIP-181): one writing call over the observations
// that sets the storyline — or rewrites the current one as the request asks — and stops there.
// It is corrected like the flow call (CLIP-94).
func (s *Service) Storyline(ctx context.Context, model llm.ModelRef, input clip.StorylineInput) (clip.Storyline, llm.Usage, error) {
	if err := ctx.Err(); err != nil {
		return clip.Storyline{}, llm.Usage{}, err
	}
	if !nativeComposition(input.PlanningInput) || len(input.Analyses) == 0 ||
		input.Request != "" && (input.Current == nil || !within(input.Request, 1, s.cfg.Template.InstructionChars)) {
		return clip.Storyline{}, llm.Usage{}, stageError("storyline", clip.ErrInvalid)
	}
	if err := validateInput(s.cfg, input.PlanningInput); err != nil {
		return clip.Storyline{}, llm.Usage{}, err
	}
	info, err := s.model(model, llm.StageNameWrite)
	if err != nil {
		return clip.Storyline{}, llm.Usage{}, stageError("storyline", err)
	}
	execution, err := executionPolicy(input.Policy, model, llm.StageNameWrite, s.cfg.FlowCompletionTokens, llm.ExecutionTextOnly)
	if err != nil {
		return clip.Storyline{}, llm.Usage{}, err
	}
	mode := "storyline"
	if input.Request != "" {
		mode = "storyline-revision"
	}
	system, user := BuildStorylinePrompt(input, compositionLimits(s.cfg, input.PlanningInput))
	request := llm.Request{Composition: clipComposition(mode, system, user, input.Guidelines, planningSourceRefs(input.PlanningInput)), System: system, Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart(user)}}}, Stage: llm.StageNameWrite, Reasoning: input.Policy.Reasoning, DisableReasoning: input.Policy.DisableReasoning, MaxTokens: input.Policy.CompletionTokens, Execution: execution}
	if execution.Call.StructuredOutput {
		if !info.StructuredOutput {
			return clip.Storyline{}, llm.Usage{}, clip.ErrPricingUnavailable
		}
		request.JSONSchema = StorylineSchema()
	}
	if err := validatePrompt(system, user, request.JSONSchema, llm.ExecutionTextOnly, input.Policy.InputTokenLimit()); err != nil {
		return clip.Storyline{}, llm.Usage{}, err
	}
	result, usage, err := completeValidated(ctx, s, model, request, user, input.Policy, func(raw string) (clip.Storyline, error) {
		return parseStoryline(s.cfg, input, raw)
	})
	return result, usage, stageError("storyline", err)
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
	system, user := BuildNarrationPrompt(input, compositionLimits(s.cfg, input.PlanningInput))
	request := llm.Request{Composition: clipComposition("narration", system, user, input.Guidelines, planningSourceRefs(input.PlanningInput)), System: system, Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart(user)}}}, Stage: llm.StageNameWrite, Reasoning: input.Policy.Reasoning, DisableReasoning: input.Policy.DisableReasoning, MaxTokens: input.Policy.CompletionTokens, Execution: execution}
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
		if in.TargetDurationMS < cfg.Render.MinDurationMS || in.TargetDurationMS > cfg.Render.MaxDurationMS || in.Composition.Snapshot.Version != clip.CompositionVersion {
			return clip.ErrInvalid
		}
		// With no template attached the recipe is absent and the document is
		// the server's own empty one (CLIP-5); with one, it must still name the
		// template and match the body that template froze.
		if in.Composition.NoTemplate() {
			if in.Template.Name != "" || in.Template.CompositionBody != "" || in.Composition.Snapshot.Body != clip.EmptyCompositionBody() {
				return clip.ErrInvalid
			}
		} else if !within(in.Template.Name, 1, cfg.Template.NameChars) || in.Template.CompositionBody != in.Composition.Snapshot.Body {
			return clip.ErrInvalid
		}
		doc, problem := composition.Parse(in.Composition.Snapshot.Body, compositionLimits(cfg, in))
		if problem != nil {
			return problem
		}
		return clip.ValidateCompositionInputs(doc, in.Composition.Inputs, compositionLimits(cfg, in), true)
	}
	// Every generation freezes a composition (CLIP-5): the no-template document
	// or its template's outline. A payload without one is refused.
	return clip.ErrCompositionUnavailable
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
	return nil
}
