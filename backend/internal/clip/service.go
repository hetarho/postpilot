package clip

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/clip/design"
)

type Service struct {
	store      Store
	limits     Limits
	sources    *SourceService
	generation *GenerationService
}

func (s *Service) SetGeneration(g *GenerationService) { s.generation = g }
func (s *Service) SetSources(sources *SourceService)  { s.sources = sources }

func NewService(store Store, limits Limits) *Service {
	for _, n := range []int{limits.NameChars, limits.GuidanceChars, limits.FieldCount, limits.LabelChars, limits.PromptChars, limits.TitleChars, limits.AnswerChars, limits.MinDurationMS, limits.MaxDurationMS} {
		if n <= 0 {
			panic("clip: limits must be positive")
		}
	}
	if limits.MinDurationMS > limits.MaxDurationMS {
		panic("clip: inverted duration limits")
	}
	return &Service{store: store, limits: limits}
}
func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}
func bounded(s string, min, max int) bool {
	n := utf8.RuneCountInString(s)
	return utf8.ValidString(s) && n >= min && n <= max
}

// Empty is neutral; the seven others are the CDS-15 palette, one per project.
func ValidAccent(s string) bool {
	if s == "" {
		return true
	}
	_, ok := design.Accent[s]
	return ok
}

// A template approves a subset of the four CDS-22 styles and must always keep
// 깔끔하게: every CDS fallback — a 메모 over its limit, a third 크게 강조, a
// 형광펜 with two keywords, a pairing under 4.5:1 — lands on it.
func ValidCopyStyles(values []string) bool {
	if len(values) == 0 || len(values) > len(design.Styles) || !slices.Contains(values, "clean") {
		return false
	}
	seen := map[string]bool{}
	for _, s := range values {
		if _, ok := design.Styles[s]; seen[s] || !ok {
			return false
		}
		seen[s] = true
	}
	return true
}
func (s *Service) fields(values []InformationField) ([]InformationField, error) {
	if len(values) > s.limits.FieldCount {
		return nil, ErrInvalid
	}
	out := make([]InformationField, 0, len(values))
	seen := map[string]bool{}
	for _, f := range values {
		f.Label = strings.TrimSpace(f.Label)
		f.Prompt = strings.TrimSpace(f.Prompt)
		if seen[f.Label] || !bounded(f.Label, 1, s.limits.LabelChars) || !bounded(f.Prompt, 1, s.limits.PromptChars) {
			return nil, ErrInvalid
		}
		seen[f.Label] = true
		out = append(out, f)
	}
	return out, nil
}
func (s *Service) answers(values []Answer) ([]Answer, error) {
	out := make([]Answer, 0, len(values))
	seen := map[string]bool{}
	for _, a := range values {
		a.Label = strings.TrimSpace(a.Label)
		if seen[a.Label] || !bounded(a.Label, 1, s.limits.LabelChars) || !bounded(a.Text, 0, s.limits.AnswerChars) {
			return nil, ErrInvalid
		}
		seen[a.Label] = true
		out = append(out, a)
	}
	return out, nil
}
func (s *Service) ListTemplates(ctx context.Context, user string) ([]VideoTemplate, error) {
	return s.store.ListTemplates(ctx, user)
}
func (s *Service) CreateTemplate(ctx context.Context, user string, recipe Recipe) (VideoTemplate, error) {
	recipe.Name = strings.TrimSpace(recipe.Name)
	if recipe.CompositionBody != "" {
		if !bounded(recipe.Name, 1, s.limits.NameChars) {
			return VideoTemplate{}, ErrInvalid
		}
		var err error
		recipe, err = s.authoredRecipe(recipe)
		if err != nil {
			return VideoTemplate{}, err
		}
		now := time.Now()
		t := VideoTemplate{ID: newID(), UserID: user, Recipe: recipe, CreatedAt: now, UpdatedAt: now}
		if err = s.store.InsertTemplate(ctx, t); err != nil {
			return VideoTemplate{}, err
		}
		return t, nil
	}
	if !bounded(recipe.Name, 1, s.limits.NameChars) || !bounded(recipe.CutGuidance, 0, s.limits.GuidanceChars) || !ValidCopyStyles(recipe.CopyStyles) || !ValidAccent(recipe.Accent) || !ValidPreset(recipe.Preset) || !ValidCaptionPace(recipe.CaptionPace) {
		return VideoTemplate{}, ErrInvalid
	}
	fields, err := s.fields(recipe.InformationFields)
	if err != nil {
		return VideoTemplate{}, err
	}
	recipe.InformationFields = fields
	recipe.CompositionBody = LegacyCompositionBody(recipe)
	recipe.CompositionLegacy = true
	now := time.Now()
	t := VideoTemplate{ID: newID(), UserID: user, Recipe: recipe, CreatedAt: now, UpdatedAt: now}
	if err := s.store.InsertTemplate(ctx, t); err != nil {
		return VideoTemplate{}, err
	}
	return t, nil
}
func (s *Service) UpdateTemplate(ctx context.Context, user, id string, p TemplatePatch) (VideoTemplate, error) {
	old, err := s.store.GetTemplate(ctx, user, id)
	if err != nil {
		return VideoTemplate{}, err
	}
	if p.Name != nil {
		name := strings.TrimSpace(*p.Name)
		if !bounded(name, 1, s.limits.NameChars) {
			return VideoTemplate{}, ErrInvalid
		}
		p.Name = &name
	}
	if p.CompositionBody != nil {
		r := old.Recipe
		r.CompositionBody = *p.CompositionBody
		r, err = s.authoredRecipe(r)
		if err != nil {
			return VideoTemplate{}, err
		}
		p.CutGuidance = &r.CutGuidance
		p.Accent = &r.Accent
		p.Preset = &r.Preset
		p.CaptionPace = &r.CaptionPace
		p.InformationFields = &r.InformationFields
		p.CopyStyles = &r.CopyStyles
		return s.store.UpdateTemplate(ctx, user, id, p, time.Now())
	}
	if old.CompositionBody != "" && !old.CompositionLegacy {
		if p.CutGuidance != nil || p.Accent != nil || p.Preset != nil || p.CaptionPace != nil || p.InformationFields != nil || p.CopyStyles != nil {
			return VideoTemplate{}, ErrInvalid
		}
		return s.store.UpdateTemplate(ctx, user, id, p, time.Now())
	}
	if p.CutGuidance != nil && !bounded(*p.CutGuidance, 0, s.limits.GuidanceChars) {
		return VideoTemplate{}, ErrInvalid
	}
	if p.Accent != nil && !ValidAccent(*p.Accent) {
		return VideoTemplate{}, ErrInvalid
	}
	if p.CaptionPace != nil && !ValidCaptionPace(*p.CaptionPace) {
		return VideoTemplate{}, ErrInvalid
	}
	if p.CopyStyles != nil && !ValidCopyStyles(*p.CopyStyles) {
		return VideoTemplate{}, ErrInvalid
	}
	// The empty preset is readable but never writable: a template that names one
	// must name one of the five (CDS-50).
	if p.Preset != nil && !ValidPreset(*p.Preset) {
		return VideoTemplate{}, ErrInvalid
	}
	if p.InformationFields != nil {
		fields, err := s.fields(*p.InformationFields)
		if err != nil {
			return VideoTemplate{}, err
		}
		p.InformationFields = &fields
	}
	return s.store.UpdateTemplate(ctx, user, id, p, time.Now())
}
func (s *Service) DeleteTemplate(ctx context.Context, user, id string) (int, error) {
	return s.store.DeleteTemplate(ctx, user, id)
}
func (s *Service) ListProjects(ctx context.Context, user string) ([]Project, error) {
	return s.store.ListProjects(ctx, user)
}
func (s *Service) GetProject(ctx context.Context, user, id string) (Project, error) {
	p, err := s.store.GetProject(ctx, user, id)
	if err != nil || p.Result == nil || s.generation == nil {
		return p, err
	}
	p.Result.ViewURL, err = s.generation.objects.PresignRead(ctx, p.Result.Key, "clip.mp4", false, s.generation.cfg.ReadTTL)
	if err != nil {
		return Project{}, err
	}
	p.Result.DownloadURL, err = s.generation.objects.PresignRead(ctx, p.Result.Key, "clip.mp4", true, s.generation.cfg.ReadTTL)
	return p, err
}
func (s *Service) duration(ms int) bool {
	return ms >= s.limits.MinDurationMS && ms <= s.limits.MaxDurationMS
}
func (s *Service) CreateProject(ctx context.Context, user string, input ProjectInput) (Project, error) {
	template, err := s.store.GetTemplate(ctx, user, input.VideoTemplateID)
	if err != nil {
		return Project{}, err
	}
	title := strings.TrimSpace(input.Title)
	if !bounded(title, 1, s.limits.TitleChars) || !s.duration(input.TargetDurationMS) || !slices.Contains([]string{"vertical", "horizontal", "square"}, input.Ratio) {
		return Project{}, ErrInvalid
	}
	// Empty disclosure is allowed at creation — the owner chooses it before
	// starting, and the generation gate is what refuses a clip without one.
	if input.Disclosure != "" && !ValidDisclosure(input.Disclosure) || !ValidCTA(input.CTA) {
		return Project{}, ErrInvalid
	}
	answers, err := s.answers(input.Answers)
	if err != nil {
		return Project{}, err
	}
	now := time.Now()
	p := Project{ID: newID(), UserID: user, Title: title, VideoTemplateID: input.VideoTemplateID, Ratio: input.Ratio, Disclosure: input.Disclosure, HideDisclosure: input.HideDisclosure, CTA: input.CTA, TargetDurationMS: input.TargetDurationMS, Answers: answers, CreatedAt: now, UpdatedAt: now}
	p.Composition, err = s.projectComposition(template, input.CompositionInputs, p)
	if err != nil {
		return Project{}, err
	}
	if input.CompositionInputs != nil && p.Composition.Snapshot.Legacy {
		p.Answers = legacyAnswers(p.Answers, template.InformationFields, p.Composition.Inputs.Values)
	}
	if err := s.store.InsertProject(ctx, p); err != nil {
		return Project{}, err
	}
	return p, nil
}
func (s *Service) UpdateProject(ctx context.Context, user, id string, p ProjectPatch) (Project, error) {
	if s.generation != nil {
		active, err := s.generation.jobs.Active(ctx, user, id)
		if err != nil {
			return Project{}, err
		}
		if active != nil {
			return Project{}, ErrBusy
		}
	}
	old, err := s.store.GetProject(ctx, user, id)
	if err != nil {
		return Project{}, err
	}
	if p.Title != nil {
		title := strings.TrimSpace(*p.Title)
		if !bounded(title, 1, s.limits.TitleChars) {
			return Project{}, ErrInvalid
		}
		p.Title = &title
	}
	if p.TargetDurationMS != nil && !s.duration(*p.TargetDurationMS) {
		return Project{}, ErrInvalid
	}
	if p.Disclosure != nil && !ValidDisclosure(*p.Disclosure) {
		return Project{}, ErrInvalid
	}
	if p.CTA != nil && !ValidCTA(*p.CTA) {
		return Project{}, ErrInvalid
	}
	if p.VideoTemplateID != nil {
		if _, err := s.store.GetTemplate(ctx, user, *p.VideoTemplateID); err != nil {
			return Project{}, err
		}
	}
	answers, err := s.answers(p.Answers)
	if err != nil {
		return Project{}, err
	}
	p.Answers = answers
	p.Composition = nil
	p.ExpectedCompositionRevision = nil
	if p.VideoTemplateID != nil && *p.VideoTemplateID != old.VideoTemplateID {
		t, e := s.store.GetTemplate(ctx, user, *p.VideoTemplateID)
		if e != nil {
			return Project{}, e
		}
		next := old
		next.VideoTemplateID = t.ID
		p.Composition, err = s.projectComposition(t, p.CompositionInputs, next)
		if err != nil {
			return Project{}, err
		}
	} else if p.CompositionInputs != nil {
		if old.Composition == nil {
			return Project{}, ErrInvalid
		}
		c := *old.Composition
		c.Inputs = *p.CompositionInputs
		limits := s.limits.Composition
		if c.Snapshot.Legacy {
			limits = LegacyCompositionLimits(limits)
		}
		d, e := composition.Parse(c.Snapshot.Body, limits)
		if e != nil {
			return Project{}, e
		}
		if e := ValidateCompositionInputs(d, c.Inputs, s.limits.Composition, false); e != nil {
			return Project{}, e
		}
		if e := ValidateSourceAssociations(old, c.Inputs.Associations); e != nil {
			return Project{}, e
		}
		if c.Snapshot.Legacy && c.Snapshot.LegacyRecipe != nil {
			next := old
			next.Answers = legacyAnswers(old.Answers, c.Snapshot.LegacyRecipe.InformationFields, c.Inputs.Values)
			if p.Disclosure != nil {
				next.Disclosure = *p.Disclosure
			}
			if p.HideDisclosure != nil {
				next.HideDisclosure = *p.HideDisclosure
			}
			if p.CTA != nil {
				next.CTA = *p.CTA
			}
			p.Answers = next.Answers
			updated := LegacyProjectComposition(next, *c.Snapshot.LegacyRecipe)
			updated.Snapshot.TemplateID = c.Snapshot.TemplateID
			c = updated
		}
		p.Composition = &c
	}
	if p.Composition != nil {
		v := old.EditPlanRevision
		p.ExpectedCompositionRevision = &v
	}
	return s.store.UpdateProject(ctx, user, id, p, time.Now())
}
func (s *Service) DeleteProject(ctx context.Context, user, id string) error {
	if s.sources != nil {
		if err := s.sources.PrepareProjectDelete(ctx, user, id); err != nil {
			return err
		}
	}
	return s.store.DeleteProject(ctx, user, id)
}

// RequiredAnswers is the generation gate, separate from saving an unfinished form.
func RequiredAnswers(t VideoTemplate, p Project, limits ...composition.Limits) error {
	if t.CompositionBody != "" && !t.CompositionLegacy {
		if len(limits) != 1 {
			return ErrInvalid
		}
		_, err := GenerationComposition(t, p, limits[0])
		return err
	}
	answers := map[string]string{}
	for _, a := range p.Answers {
		answers[a.Label] = a.Text
	}
	for _, f := range t.InformationFields {
		if strings.TrimSpace(answers[f.Label]) == "" {
			return fmt.Errorf("%w: missing template answer", ErrInvalid)
		}
	}
	return ApprovalGate(t, p)
}

// ValidDisclosure and ValidCTA accept only the fixed ids; the phrases themselves
// are code-owned and never editable (CDS-31), and an empty CTA means "the
// template preset's" (CDS-29, CDS-51).
func ValidDisclosure(s string) bool {
	_, ok := design.Disclosure[s]
	return ok
}
func ValidCTA(s string) bool {
	if s == "" {
		return true
	}
	_, ok := design.CTA[s]
	return ok
}
func ValidPreset(s string) bool {
	_, ok := design.Presets[s]
	return ok
}

// MissingFactsError names the reserved labels a clip still needs, so the refusal
// can say which ones rather than that something is missing.
type MissingFactsError struct{ Labels []string }

func (e *MissingFactsError) Error() string {
	return "clip needs more on-screen facts: " + strings.Join(e.Labels, ", ")
}

// ApprovalGate is what CDS-1 and CDS-5 make checkable before a single credit is
// reserved: a clip renders its disclosure badge and at least two verifiable
// facts, so a clip without them cannot be started or even quoted.
func ApprovalGate(t VideoTemplate, p Project) error {
	if !ValidDisclosure(p.Disclosure) {
		return ErrDisclosureRequired
	}
	answers := map[string]string{}
	for _, a := range p.Answers {
		answers[a.Label] = a.Text
	}
	present, missing := 0, []string{}
	for _, label := range design.Fact.Minimum.Labels {
		if strings.TrimSpace(answers[label]) != "" {
			present++
			continue
		}
		missing = append(missing, label)
	}
	if present < design.Fact.Minimum.Count {
		return &MissingFactsError{Labels: missing}
	}
	return nil
}
