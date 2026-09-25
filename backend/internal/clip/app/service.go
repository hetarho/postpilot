package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"slices"
	"strings"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
)

type Service struct {
	finalizer  clip.ProjectFinalizer
	store      clip.Store
	limits     clip.Limits
	sources    *SourceService
	generation *GenerationService
	// now is the service clock: every stored timestamp comes from here so a test can
	// hold time still (the same seam GenerationService and SourceService carry).
	now func() time.Time
}

// NewService wires the project context. sources and finalizer are required (ARCH-40):
// deleting a project fences its sources and confirming a result is a saga over two
// contexts' tables, and a service built without either would skip both silently. The
// generation side is bound by NewGenerationService, which needs this service first.
func NewService(store clip.Store, limits clip.Limits, sources *SourceService, finalizer clip.ProjectFinalizer) *Service {
	if sources == nil || finalizer == nil {
		panic("clip: sources and finalizer are required")
	}
	for _, n := range []int{limits.NameChars, limits.GuidanceChars, limits.FieldCount, limits.LabelChars, limits.PromptChars, limits.TitleChars, limits.AnswerChars, limits.InstructionChars, limits.MinDurationMS, limits.MaxDurationMS} {
		if n <= 0 {
			panic("clip: limits must be positive")
		}
	}
	if limits.MinDurationMS > limits.MaxDurationMS {
		panic("clip: inverted duration limits")
	}
	return &Service{store: store, limits: limits, sources: sources, finalizer: finalizer, now: time.Now}
}

func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}

func (s *Service) fields(values []clip.InformationField) ([]clip.InformationField, error) {
	if len(values) > s.limits.FieldCount {
		return nil, clip.ErrInvalid
	}
	out := make([]clip.InformationField, 0, len(values))
	seen := map[string]bool{}
	for _, f := range values {
		f.Label = strings.TrimSpace(f.Label)
		f.Prompt = strings.TrimSpace(f.Prompt)
		if seen[f.Label] || !clip.BoundedText(f.Label, 1, s.limits.LabelChars) || !clip.BoundedText(f.Prompt, 1, s.limits.PromptChars) {
			return nil, clip.ErrInvalid
		}
		seen[f.Label] = true
		out = append(out, f)
	}
	return out, nil
}

func (s *Service) answers(values []clip.Answer) ([]clip.Answer, error) {
	out := make([]clip.Answer, 0, len(values))
	seen := map[string]bool{}
	for _, a := range values {
		a.Label = strings.TrimSpace(a.Label)
		if seen[a.Label] || !clip.BoundedText(a.Label, 1, s.limits.LabelChars) || !clip.BoundedText(a.Text, 0, s.limits.AnswerChars) {
			return nil, clip.ErrInvalid
		}
		seen[a.Label] = true
		out = append(out, a)
	}
	return out, nil
}

func (s *Service) ListTemplates(ctx context.Context, user string) ([]clip.VideoTemplate, error) {
	return s.store.ListTemplates(ctx, user)
}

func (s *Service) CreateTemplate(ctx context.Context, user string, recipe clip.Recipe) (clip.VideoTemplate, error) {
	recipe.Name = strings.TrimSpace(recipe.Name)
	if recipe.CompositionBody != "" {
		if !clip.BoundedText(recipe.Name, 1, s.limits.NameChars) {
			return clip.VideoTemplate{}, clip.ErrInvalid
		}
		var err error
		recipe, err = s.authoredRecipe(recipe)
		if err != nil {
			return clip.VideoTemplate{}, err
		}
		now := s.now()
		t := clip.VideoTemplate{ID: newID(), UserID: user, Recipe: recipe, CreatedAt: now, UpdatedAt: now}
		if err = s.store.InsertTemplate(ctx, t); err != nil {
			return clip.VideoTemplate{}, err
		}
		return t, nil
	}
	if !clip.BoundedText(recipe.Name, 1, s.limits.NameChars) || !clip.BoundedText(recipe.CutGuidance, 0, s.limits.GuidanceChars) || !clip.ValidAccent(recipe.Accent) || !clip.ValidPreset(recipe.Preset) || !clip.ValidCaptionPace(recipe.CaptionPace) {
		return clip.VideoTemplate{}, clip.ErrInvalid
	}
	fields, err := s.fields(recipe.InformationFields)
	if err != nil {
		return clip.VideoTemplate{}, err
	}
	recipe.InformationFields = fields
	recipe.CompositionBody = clip.LegacyCompositionBody(recipe)
	recipe.CompositionLegacy = true
	now := s.now()
	t := clip.VideoTemplate{ID: newID(), UserID: user, Recipe: recipe, CreatedAt: now, UpdatedAt: now}
	if err := s.store.InsertTemplate(ctx, t); err != nil {
		return clip.VideoTemplate{}, err
	}
	return t, nil
}

func (s *Service) UpdateTemplate(ctx context.Context, user, id string, p clip.TemplatePatch) (clip.VideoTemplate, error) {
	old, err := s.store.GetTemplate(ctx, user, id)
	if err != nil {
		return clip.VideoTemplate{}, err
	}
	if p.Name != nil {
		name := strings.TrimSpace(*p.Name)
		if !clip.BoundedText(name, 1, s.limits.NameChars) {
			return clip.VideoTemplate{}, clip.ErrInvalid
		}
		p.Name = &name
	}
	if p.CompositionBody != nil {
		r := old.Recipe
		r.CompositionBody = *p.CompositionBody
		r, err = s.authoredRecipe(r)
		if err != nil {
			return clip.VideoTemplate{}, err
		}
		p.CutGuidance = &r.CutGuidance
		p.Accent = &r.Accent
		p.Preset = &r.Preset
		p.CaptionPace = &r.CaptionPace
		p.InformationFields = &r.InformationFields
		return s.store.UpdateTemplate(ctx, user, id, p, s.now())
	}
	if old.CompositionBody != "" && !old.CompositionLegacy {
		if p.CutGuidance != nil || p.Accent != nil || p.Preset != nil || p.CaptionPace != nil || p.InformationFields != nil {
			return clip.VideoTemplate{}, clip.ErrInvalid
		}
		return s.store.UpdateTemplate(ctx, user, id, p, s.now())
	}
	if p.CutGuidance != nil && !clip.BoundedText(*p.CutGuidance, 0, s.limits.GuidanceChars) {
		return clip.VideoTemplate{}, clip.ErrInvalid
	}
	if p.Accent != nil && !clip.ValidAccent(*p.Accent) {
		return clip.VideoTemplate{}, clip.ErrInvalid
	}
	if p.CaptionPace != nil && !clip.ValidCaptionPace(*p.CaptionPace) {
		return clip.VideoTemplate{}, clip.ErrInvalid
	}
	// The empty preset is readable but never writable: a template that names one
	// must name one of the five (CDS-50).
	if p.Preset != nil && !clip.ValidPreset(*p.Preset) {
		return clip.VideoTemplate{}, clip.ErrInvalid
	}
	if p.InformationFields != nil {
		fields, err := s.fields(*p.InformationFields)
		if err != nil {
			return clip.VideoTemplate{}, err
		}
		p.InformationFields = &fields
	}
	return s.store.UpdateTemplate(ctx, user, id, p, s.now())
}

func (s *Service) DeleteTemplate(ctx context.Context, user, id string) (int, error) {
	return s.store.DeleteTemplate(ctx, user, id)
}

func (s *Service) ListProjects(ctx context.Context, user string) ([]clip.Project, error) {
	return s.store.ListProjects(ctx, user)
}

func (s *Service) GetProject(ctx context.Context, user, id string) (clip.Project, error) {
	p, err := s.store.GetProject(ctx, user, id)
	if err != nil {
		return p, err
	}
	// What the owner asked the AI for, read where the OWNER reads the project
	// (CLIP-133). The run paths take the project from the store directly, so no
	// job pays for this read.
	if p.Requests, err = s.store.ListProjectRequests(ctx, user, id); err != nil {
		return clip.Project{}, err
	}
	if p.Result == nil || s.generation == nil {
		return p, nil
	}
	p.Result.ViewURL, err = s.generation.objects.PresignRead(ctx, p.Result.Key, "clip.mp4", false, s.generation.cfg.ReadTTL)
	if err != nil {
		return clip.Project{}, err
	}
	p.Result.DownloadURL, err = s.generation.objects.PresignRead(ctx, p.Result.Key, "clip.mp4", true, s.generation.cfg.ReadTTL)
	return p, err
}

func (s *Service) duration(ms int) bool {
	return ms >= s.limits.MinDurationMS && ms <= s.limits.MaxDurationMS
}

// 0 means the owner has not chosen yet; any written value stays BoundedText.
func (s *Service) durationOrUnset(ms int) bool { return ms == 0 || s.duration(ms) }

func (s *Service) CreateProject(ctx context.Context, user string, input clip.ProjectInput) (clip.Project, error) {
	if !clip.ValidLanguage(input.Language) {
		return clip.Project{}, clip.ErrInvalid
	}
	// A template is a preset, not a precondition (CLIP-5): `/clips/new` may mint
	// a project with none, and everything a template would have supplied starts
	// at the shared defaults instead.
	var template clip.VideoTemplate
	if input.VideoTemplateID != "" {
		var err error
		if template, err = s.store.GetTemplate(ctx, user, input.VideoTemplateID); err != nil {
			return clip.Project{}, err
		}
	}
	title := strings.TrimSpace(input.Title)
	// An unset duration is allowed at creation — the owner chooses it in ①
	// beside the sources it measures, and the generation gate is what refuses a
	// clip without one (CLIP-130, CLIP-7).
	if !clip.BoundedText(title, 1, s.limits.TitleChars) || !s.durationOrUnset(input.TargetDurationMS) || !slices.Contains([]string{"vertical", "horizontal", "square"}, input.Ratio) {
		return clip.Project{}, clip.ErrInvalid
	}
	// Empty disclosure is allowed at creation — the owner chooses it before
	// starting, and the generation gate is what refuses a clip without one.
	if input.Disclosure != "" && !clip.ValidDisclosure(input.Disclosure) || !clip.ValidCTA(input.CTA) {
		return clip.Project{}, clip.ErrInvalid
	}
	if !clip.BoundedText(input.Instruction, 0, s.limits.InstructionChars) {
		return clip.Project{}, clip.ErrInvalid
	}
	answers, err := s.answers(input.Answers)
	if err != nil {
		return clip.Project{}, err
	}
	// The pace and the accent are the PROJECT's alone and start unset, which is
	// the shared default: a template carries none of the five design values any
	// more (CLIP-14, CLIP-139).
	pace, accent := "", ""
	if input.CaptionPace != nil {
		pace = *input.CaptionPace
	}
	if input.Accent != nil {
		accent = *input.Accent
	}
	if !clip.ValidCaptionPace(pace) || !clip.ValidAccent(accent) {
		return clip.Project{}, clip.ErrInvalid
	}
	// The two presets and the allowed styles are the project's alone and start
	// at the new-project defaults whether or not a template was chosen: a
	// template carries no design at all (CLIP-14, CLIP-139). The ids are stored,
	// so a later change of defaults never restyles this project (CLIP-111).
	defaults := composition.DefaultDesign()
	intro, outro, styles := defaults.Intro, defaults.Outro, []string(nil)
	if input.IntroPreset != nil {
		intro = *input.IntroPreset
	}
	if input.OutroPreset != nil {
		outro = *input.OutroPreset
	}
	if input.CaptionStyles != nil {
		styles = *input.CaptionStyles
	}
	if intro == "" {
		intro = defaults.Intro
	}
	if outro == "" {
		outro = defaults.Outro
	}
	if !clip.ValidIntroPreset(intro) || !clip.ValidOutroPreset(outro) || !clip.ValidCaptionStyles(styles) {
		return clip.Project{}, clip.ErrInvalid
	}
	now := s.now()
	p := clip.Project{ID: newID(), UserID: user, Title: title, VideoTemplateID: input.VideoTemplateID, Ratio: input.Ratio, Language: input.Language, Disclosure: input.Disclosure, HideDisclosure: input.HideDisclosure, CTA: input.CTA, Instruction: input.Instruction, CaptionPace: pace, Accent: accent, IntroPreset: intro, OutroPreset: outro, CaptionStyles: styles, TargetDurationMS: input.TargetDurationMS, Answers: answers, CreatedAt: now, UpdatedAt: now}
	p.Composition, err = s.projectComposition(template, input.CompositionInputs, p)
	if err != nil {
		return clip.Project{}, err
	}
	if input.CompositionInputs != nil && p.Composition.Snapshot.Legacy {
		p.Answers = legacyAnswers(p.Answers, template.InformationFields, p.Composition.Inputs.Values)
	}
	if err := s.store.InsertProject(ctx, p); err != nil {
		return clip.Project{}, err
	}
	return p, nil
}

func (s *Service) UpdateProject(ctx context.Context, user, id string, p clip.ProjectPatch) (clip.Project, error) {
	if s.generation != nil {
		active, err := s.generation.jobs.Active(ctx, user, id)
		if err != nil {
			return clip.Project{}, err
		}
		if active != nil {
			return clip.Project{}, clip.ErrBusy
		}
	}
	old, err := s.store.GetProject(ctx, user, id)
	if err != nil {
		return clip.Project{}, err
	}
	if old.Finalized != nil {
		return clip.Project{}, clip.ErrFinalized
	}
	if p.Title != nil {
		title := strings.TrimSpace(*p.Title)
		if !clip.BoundedText(title, 1, s.limits.TitleChars) {
			return clip.Project{}, clip.ErrInvalid
		}
		p.Title = &title
	}
	if p.TargetDurationMS != nil && !s.durationOrUnset(*p.TargetDurationMS) {
		return clip.Project{}, clip.ErrInvalid
	}
	// An instruction is optional and clearable; only its length is refused.
	if p.Instruction != nil && !clip.BoundedText(*p.Instruction, 0, s.limits.InstructionChars) {
		return clip.Project{}, clip.ErrInvalid
	}
	// The retired picker's value is the campaign identity only where nothing
	// else declares one, so a legacy project still may not clear it (CDS-5). An
	// authored composition owns its own disclosure field and leaves this column
	// empty, and an owner edit has to be able to write that back — otherwise the
	// project can never be saved again after creation.
	authored := old.Composition != nil && !old.Composition.Snapshot.Legacy
	if p.Disclosure != nil && !clip.ValidDisclosure(*p.Disclosure) && !(authored && *p.Disclosure == "") {
		return clip.Project{}, clip.ErrInvalid
	}
	if p.CTA != nil && !clip.ValidCTA(*p.CTA) {
		return clip.Project{}, clip.ErrInvalid
	}
	// Either one only changes how the SAME plan renders — how its phrases split
	// and which colour one word takes — so both are clearable and neither
	// invalidates an observation or a written plan (CLIP-139).
	if p.CaptionPace != nil && !clip.ValidCaptionPace(*p.CaptionPace) || p.Accent != nil && !clip.ValidAccent(*p.Accent) {
		return clip.Project{}, clip.ErrInvalid
	}
	// The presets and the allowed styles are the same kind of change: the plan,
	// the observations and the writing calls all stand, and only the rendered
	// result goes stale (CLIP-139, CLIP-142).
	if p.IntroPreset != nil && !clip.ValidIntroPreset(*p.IntroPreset) || p.OutroPreset != nil && !clip.ValidOutroPreset(*p.OutroPreset) {
		return clip.Project{}, clip.ErrInvalid
	}
	if p.CaptionStyles != nil && !clip.ValidCaptionStyles(*p.CaptionStyles) {
		return clip.Project{}, clip.ErrInvalid
	}
	// An empty id is the owner choosing 없음, which detaches the template and
	// leaves every value it seeded exactly where it is (CLIP-5, CLIP-139).
	if p.VideoTemplateID != nil && *p.VideoTemplateID != "" {
		if _, err := s.store.GetTemplate(ctx, user, *p.VideoTemplateID); err != nil {
			return clip.Project{}, err
		}
	}
	answers, err := s.answers(p.Answers)
	if err != nil {
		return clip.Project{}, err
	}
	p.Answers = answers
	p.Composition = nil
	p.ExpectedCompositionRevision = nil
	if p.VideoTemplateID != nil && *p.VideoTemplateID != old.VideoTemplateID {
		var t clip.VideoTemplate
		if *p.VideoTemplateID != "" {
			var e error
			if t, e = s.store.GetTemplate(ctx, user, *p.VideoTemplateID); e != nil {
				return clip.Project{}, e
			}
		}
		next := old
		next.VideoTemplateID = t.ID
		p.Composition, err = s.projectComposition(t, p.CompositionInputs, next)
		if err != nil {
			return clip.Project{}, err
		}
	} else if p.CompositionInputs != nil {
		if old.Composition == nil {
			return clip.Project{}, clip.ErrInvalid
		}
		c := *old.Composition
		c.Inputs = *p.CompositionInputs
		// Applying current template inputs is a meaningful setup edit. Keep the
		// previous rendered plan frozen, but validate new field IDs against the
		// current authored template rather than the prior generation snapshot.
		if old.VideoTemplateID != "" {
			current, e := s.store.GetTemplate(ctx, user, old.VideoTemplateID)
			if e != nil {
				return clip.Project{}, e
			}
			if current.CompositionBody != "" && !current.CompositionLegacy {
				c.Snapshot = clip.CompositionSnapshot{Version: clip.CompositionVersion, Body: current.CompositionBody, TemplateID: current.ID}
			}
		}
		limits := s.limits.Composition
		if c.Snapshot.Legacy {
			limits = clip.LegacyCompositionLimits(limits)
		}
		d, e := composition.Parse(c.Snapshot.Body, limits)
		if e != nil {
			return clip.Project{}, e
		}
		if e := clip.ValidateCompositionInputs(d, c.Inputs, s.limits.Composition, false); e != nil {
			return clip.Project{}, e
		}
		if e := clip.ValidateSourceAssociations(old, c.Inputs.Associations); e != nil {
			return clip.Project{}, e
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
			updated := clip.LegacyProjectComposition(next, *c.Snapshot.LegacyRecipe)
			updated.Snapshot.TemplateID = c.Snapshot.TemplateID
			c = updated
		}
		p.Composition = &c
	}
	if p.Composition != nil {
		v := old.EditPlanRevision
		p.ExpectedCompositionRevision = &v
	}
	return s.store.UpdateProject(ctx, user, id, p, s.now())
}

func (s *Service) DeleteProject(ctx context.Context, user, id string) error {
	if s.sources != nil {
		if err := s.sources.PrepareProjectDelete(ctx, user, id); err != nil {
			return err
		}
	}
	return s.store.DeleteProject(ctx, user, id)
}
