package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
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
	// candidates drops a deleted project's link from its 영상 지침 candidates; bound with the
	// generation side, nil records nothing.
	candidates clip.GuidelineCandidates
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
	for _, n := range []int{limits.NameChars, limits.FieldCount, limits.LabelChars, limits.PromptChars, limits.TitleChars, limits.AnswerChars, limits.InstructionChars, limits.MinDurationMS, limits.MaxDurationMS} {
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

func (s *Service) ListTemplates(ctx context.Context, user string) ([]clip.VideoTemplate, error) {
	return s.store.ListTemplates(ctx, user)
}

// CreateTemplate saves a name and an outline body; a request without a body is
// refused, because a template is its outline and nothing else (CLIP-4, CLIP-14).
// CreateTemplate saves a video template, and its starting design selection when one is given
// (CLIP-166); without one the template starts at the shared defaults.
func (s *Service) CreateTemplate(ctx context.Context, user string, recipe clip.Recipe, design ...clip.TemplateDesign) (clip.VideoTemplate, error) {
	recipe.Name = strings.TrimSpace(recipe.Name)
	if !clip.BoundedText(recipe.Name, 1, s.limits.NameChars) || strings.TrimSpace(recipe.CompositionBody) == "" || len(design) > 1 {
		return clip.VideoTemplate{}, clip.ErrInvalid
	}
	var chosen clip.TemplateDesign
	if len(design) == 1 {
		chosen = design[0]
	}
	if !chosen.Valid() {
		return clip.VideoTemplate{}, clip.ErrInvalid
	}
	if err := s.authoredBody(recipe.CompositionBody); err != nil {
		return clip.VideoTemplate{}, err
	}
	now := s.now()
	t := clip.VideoTemplate{ID: newID(), UserID: user, Recipe: recipe, Design: chosen, CreatedAt: now, UpdatedAt: now}
	if err := s.store.InsertTemplate(ctx, t); err != nil {
		return clip.VideoTemplate{}, err
	}
	return t, nil
}

func (s *Service) UpdateTemplate(ctx context.Context, user, id string, p clip.TemplatePatch) (clip.VideoTemplate, error) {
	if _, err := s.store.GetTemplate(ctx, user, id); err != nil {
		return clip.VideoTemplate{}, err
	}
	if p.Name != nil {
		name := strings.TrimSpace(*p.Name)
		if !clip.BoundedText(name, 1, s.limits.NameChars) {
			return clip.VideoTemplate{}, clip.ErrInvalid
		}
		p.Name = &name
	}
	// The design selection answers to the project's own rule (CLIP-166); empty is the default.
	patched := clip.TemplateDesign{}
	if p.IntroPreset != nil {
		patched.IntroPreset = *p.IntroPreset
	}
	if p.OutroPreset != nil {
		patched.OutroPreset = *p.OutroPreset
	}
	if p.CaptionStyles != nil {
		patched.CaptionStyles = *p.CaptionStyles
	}
	if !patched.Valid() {
		return clip.VideoTemplate{}, clip.ErrInvalid
	}
	if p.CompositionBody != nil {
		if strings.TrimSpace(*p.CompositionBody) == "" {
			return clip.VideoTemplate{}, clip.ErrInvalid
		}
		if err := s.authoredBody(*p.CompositionBody); err != nil {
			return clip.VideoTemplate{}, err
		}
	}
	return s.store.UpdateTemplate(ctx, user, id, p, s.now())
}

func (s *Service) DeleteTemplate(ctx context.Context, user, id string) (int, error) {
	return s.store.DeleteTemplate(ctx, user, id)
}

func (s *Service) ListProjects(ctx context.Context, user string) ([]clip.Project, error) {
	return s.store.ListProjects(ctx, user)
}

// ListProjectSummaries is the directory's read: every project without its plan
// or analysis.
func (s *Service) ListProjectSummaries(ctx context.Context, user string) ([]clip.Project, error) {
	return s.store.ListProjectSummaries(ctx, user)
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
	if input.Disclosure != "" && !clip.ValidDisclosure(input.Disclosure) {
		return clip.Project{}, clip.ErrInvalid
	}
	if !clip.BoundedText(input.Instruction, 0, s.limits.InstructionChars) {
		return clip.Project{}, clip.ErrInvalid
	}
	// The pace and the accent are the PROJECT's alone and start unset, which is
	// the shared default; a template gives only the presets and the styles (CLIP-139, CLIP-166).
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
	// The two presets and the allowed styles are the project's own. Chosen with a template, they
	// start at that template's selection (CLIP-168), otherwise at the new-project defaults; an
	// explicit value wins either way. The ids are stored, so a later change of defaults — or of
	// the template — never restyles this project (CLIP-111).
	defaults := composition.DefaultDesign()
	intro, outro, styles := template.Design.IntroPreset, template.Design.OutroPreset, template.Design.CaptionStyles
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
	p := clip.Project{ID: newID(), UserID: user, Title: title, VideoTemplateID: input.VideoTemplateID, Ratio: input.Ratio, Language: input.Language, Disclosure: input.Disclosure, HideDisclosure: input.HideDisclosure, Instruction: input.Instruction, CaptionPace: pace, Accent: accent, IntroPreset: intro, OutroPreset: outro, CaptionStyles: styles, TargetDurationMS: input.TargetDurationMS, CreatedAt: now, UpdatedAt: now}
	var err error
	p.Composition, err = s.projectComposition(template, input.CompositionInputs, p)
	if err != nil {
		return clip.Project{}, err
	}

	regions, err := clip.SeedProjectRegions(p, nil)
	if err != nil {
		return clip.Project{}, err
	}
	if input.IntroPreset != nil {
		regions.Intro.Enabled = true
	}
	if input.OutroPreset != nil {
		regions.Outro.Enabled = true
	}
	if err := clip.ApplyRegionPatch(&regions.Intro, input.IntroRegion, s.limits); err != nil {
		return clip.Project{}, err
	}
	if err := clip.ApplyRegionPatch(&regions.Outro, input.OutroRegion, s.limits); err != nil {
		return clip.Project{}, err
	}
	if err := clip.ValidateOwnerRegions(regions, p.DesignSelection().RegionPresets(), p.Ratio); err != nil {
		return clip.Project{}, err
	}
	p.Regions = &regions
	if input.Dubbing != nil {
		chosen, e := s.resolveDubbing(ctx, user, *input.Dubbing)
		if e != nil {
			return clip.Project{}, e
		}
		p.Dubbing = chosen
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
	if p.Dubbing != nil {
		chosen, e := s.resolveDubbing(ctx, user, *p.Dubbing)
		if e != nil {
			return clip.Project{}, e
		}
		p.Dubbing = &chosen
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
	// An authored composition owns its own disclosure field and leaves this
	// column empty, and an owner edit has to be able to write that back —
	// otherwise the project can never be saved again after creation (CDS-5).
	authored := old.Composition != nil
	if p.Disclosure != nil && !clip.ValidDisclosure(*p.Disclosure) && !(authored && *p.Disclosure == "") {
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
		// Choosing a template takes its design selection in the same write, silently: the owner
		// saw that design in the template's preview (CLIP-168). Clearing to 없음 moves nothing.
		if t.ID != "" {
			defaults := composition.DefaultDesign()
			intro, outro, styles := t.Design.IntroPreset, t.Design.OutroPreset, t.Design.CaptionStyles
			if intro == "" {
				intro = defaults.Intro
			}
			if outro == "" {
				outro = defaults.Outro
			}
			if styles == nil {
				styles = []string{}
			}
			p.IntroPreset, p.OutroPreset, p.CaptionStyles = &intro, &outro, &styles
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
			c.Snapshot = clip.CompositionSnapshot{Version: clip.CompositionVersion, Body: current.CompositionBody, TemplateID: current.ID}
		}
		limits := s.limits.Composition
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
		p.Composition = &c
	}
	if p.Composition != nil {
		v := old.EditPlanRevision
		p.ExpectedCompositionRevision = &v
	}

	// The service owns seeding; callers submit presence-aware edits only.
	p.Regions = nil
	if p.IntroRegion != nil || p.OutroRegion != nil || p.IntroPreset != nil || p.OutroPreset != nil || p.Composition != nil {
		regions := clip.EffectiveProjectRegions(old)
		if p.ExpectedRegionRevision != nil && *p.ExpectedRegionRevision != regions.Revision {
			return clip.Project{}, clip.ErrPlanConflict
		}
		// Words a writer left in the plan's template entries are the generated
		// slots' own before any edit lands on them (CLIP-190).
		if old.EditPlan != "" {
			if plan, e := clip.DecodeEditPlan(old.EditPlan); e == nil {
				clip.AbsorbWrittenRegions(&regions, plan)
			}
		}
		revision := regions.Revision
		p.ExpectedRegionRevision = &revision
		next := old
		if p.Composition != nil {
			next.Composition = p.Composition
		}
		// Choosing a preset switches its region on (CLIP-111); a save carrying
		// the preset the region already renders in chooses nothing, so ①'s
		// autosave never turns a region back on.
		current := old.DesignSelection().RegionPresets()
		if p.IntroPreset != nil {
			next.IntroPreset = *p.IntroPreset
			regions.Intro.Enabled = regions.Intro.Enabled || *p.IntroPreset != current.Intro
		}
		if p.OutroPreset != nil {
			next.OutroPreset = *p.OutroPreset
			regions.Outro.Enabled = regions.Outro.Enabled || *p.OutroPreset != current.Outro
		}
		if p.VideoTemplateID != nil && *p.VideoTemplateID != "" && *p.VideoTemplateID != old.VideoTemplateID {
			regions, err = clip.SeedProjectRegions(next, &regions)
			if err != nil {
				return clip.Project{}, err
			}
		} else if p.CompositionInputs != nil {
			clip.RefreshRegionBindings(&regions, *p.CompositionInputs)
		}
		clip.EnsureRegionSlots(&regions, next)
		if err := clip.ApplyRegionPatch(&regions.Intro, p.IntroRegion, s.limits); err != nil {
			return clip.Project{}, err
		}
		if err := clip.ApplyRegionPatch(&regions.Outro, p.OutroRegion, s.limits); err != nil {
			return clip.Project{}, err
		}
		p.Regions = &regions
	}
	return s.store.UpdateProject(ctx, user, id, p, s.now())
}

func (s *Service) DeleteProject(ctx context.Context, user, id string) error {
	if s.sources != nil {
		if err := s.sources.PrepareProjectDelete(ctx, user, id); err != nil {
			return err
		}
	}
	if err := s.store.DeleteProject(ctx, user, id); err != nil {
		return err
	}
	// After the row is gone, and never failing the delete: the candidates keep their text and
	// only lose the link (GUIDE-13).
	if s.candidates != nil {
		if err := s.candidates.DetachProject(ctx, user, id); err != nil {
			slog.WarnContext(ctx, "could not detach a deleted clip project from its guideline candidates", "project", id, "err", err)
		}
	}
	return nil
}

func (s *Service) resolveDubbing(ctx context.Context, user string, in clip.DubbingOptions) (clip.DubbingOptions, error) {
	in.BindingDigest = ""
	if !in.Enabled && in.VoiceID == "" {
		return in, nil
	}
	if in.VoiceID == "" || s.generation == nil {
		return in, clip.ErrInvalid
	}
	b, e := s.generation.voices.ResolveClipVoice(ctx, user, in.VoiceID)
	if e != nil {
		return in, e
	}
	in.BindingDigest = b.Digest
	return in, nil
}
