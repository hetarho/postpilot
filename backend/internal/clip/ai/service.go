package ai

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/url"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/llm"
)

type Models interface {
	Resolve(llm.ModelRef) (llm.ModelInfo, bool)
	Complete(context.Context, llm.ModelRef, llm.Request) (llm.Response, error)
}
type CaptionSizer interface {
	CaptionSize(context.Context, string, clip.Caption) (float64, float64, error)
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
	if stage == llm.StageNameObserve && (!info.Vision || !info.VideoInput) {
		return info, llm.ErrUnsupported
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
	u, err := url.Parse(input.URL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil || u.Fragment != "" {
		return clip.ChunkAnalysis{}, llm.Usage{}, clip.ErrInvalid
	}
	info, err := s.model(model, llm.StageNameObserve)
	if err != nil {
		return clip.ChunkAnalysis{}, llm.Usage{}, stageError("analyze", err)
	}
	system, user := BuildObservePrompt(input)
	request := llm.Request{System: system, Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.VideoPart(input.URL, "video/mp4"), llm.TextPart(user)}}}, Stage: llm.StageNameObserve, Reasoning: s.cfg.ObserveReasoning, MaxTokens: s.cfg.ObserveCompletionTokens}
	if info.StructuredOutput {
		request.JSONSchema = ChunkSchema()
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
	system, user := BuildPlanPrompt(input, s.cfg.Render.FadeMS)
	request := llm.Request{System: system, Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart(user)}}}, Stage: llm.StageNameWrite, Reasoning: s.cfg.PlanReasoning, MaxTokens: s.cfg.PlanCompletionTokens}
	if info.StructuredOutput {
		request.JSONSchema = PlanSchema()
	}
	response, err := s.models.Complete(ctx, model, request)
	if err != nil {
		return clip.EditPlan{}, response.Usage, stageError("plan", err)
	}
	result, err := parsePlan(s.cfg, input, response.Text)
	if err != nil {
		return clip.EditPlan{}, response.Usage, stageError("plan", llm.ResponseParseError(response, err))
	}
	canvas, _ := clip.ClipCanvas(input.Ratio)
	byID := map[string]clip.SourceAnalysis{}
	for _, a := range input.Analyses {
		byID[a.Source.ID] = a
	}
	for i, cut := range result.Cuts {
		if strings.TrimSpace(cut.Copy.Text) == "" {
			continue
		}
		width, height, err := s.captions.CaptionSize(ctx, input.Ratio, cut.Copy)
		if err != nil {
			if errors.Is(err, clip.ErrInvalid) {
				err = llm.ErrBadOutput
			}
			return clip.EditPlan{}, response.Usage, stageError("plan", err)
		}
		position, err := clip.PickCopyPosition(canvas, cut.Copy.Position, width, height, clip.CaptionAvoid(canvas, cut, byID[cut.SourceID]))
		if err != nil {
			return clip.EditPlan{}, response.Usage, stageError("plan", err)
		}
		result.Cuts[i].Copy.Position = position
	}
	return result, response.Usage, nil
}
func within(value string, minimum, maximum int) bool {
	return utf8.ValidString(value) && utf8.RuneCountInString(value) >= minimum && utf8.RuneCountInString(value) <= maximum
}
func validateInput(cfg Config, in clip.PlanningInput) error {
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
	if plan.Ratio != in.Ratio || math.Abs(float64(plan.DurationMS)-float64(in.TargetDurationMS)) > float64(cfg.TargetToleranceMS) {
		return llm.ErrBadOutput
	}
	sources := make([]clip.RenderSource, 0, len(in.Analyses))
	for _, a := range in.Analyses {
		sources = append(sources, a.Source.RenderSource)
	}
	if err := clip.ValidateEditPlan(cfg.Render, plan, sources); err != nil {
		if errors.Is(err, clip.ErrCopyTooLong) {
			return err
		}
		return llm.ErrBadOutput
	}
	for _, cut := range plan.Cuts {
		if !slices.Contains(in.Template.CopyStyles, cut.Copy.Style) || (cut.Copy.Accent != "" && cut.Copy.Accent != in.Template.Accent) {
			return llm.ErrBadOutput
		}
	}
	return nil
}
