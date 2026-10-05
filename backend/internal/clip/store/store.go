// Package store maps clip aggregates to owned SQLite rows.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/clip/store/sqlc"
)

const timeLayout = "2006-01-02T15:04:05.000000000Z07:00"

type Store struct {
	writer      *sql.DB
	read, write *sqlc.Queries
	raw         sqlc.DBTX
}

func New(writer, reader *sql.DB) *Store {
	return &Store{writer: writer, read: sqlc.New(reader), write: sqlc.New(writer), raw: writer}
}

func NewTx(tx *sql.Tx) *Store {
	return &Store{read: sqlc.New(tx), write: sqlc.New(tx), raw: tx}
}

var _ clip.Store = (*Store)(nil)

func disclosureFlag(hidden bool) int64 { return flag(hidden) }

// flag is the one bool→SQLite encoding; every such column is CHECKed to 0 or 1.
func flag(on bool) int64 {
	if on {
		return 1
	}
	return 0
}

func stamp(t time.Time) string         { return t.UTC().Format(timeLayout) }
func nullable(s string) sql.NullString { return sql.NullString{String: s, Valid: s != ""} }
func dbError(err error) error {
	if err != nil && strings.Contains(err.Error(), "clip finalized") {
		return clip.ErrFinalized
	}
	if err != nil && strings.Contains(err.Error(), "clip busy") {
		return clip.ErrBusy
	}
	if errors.Is(err, sql.ErrNoRows) {
		return clip.ErrNotFound
	}
	if err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed: video_templates.user_id, video_templates.name") {
		return clip.ErrDuplicateName
	}
	if err != nil && strings.Contains(err.Error(), "FOREIGN KEY constraint failed") {
		return clip.ErrNotFound
	}
	return err
}
func affected(n int64, err error) error {
	if err != nil {
		return dbError(err)
	}
	if n == 0 {
		return clip.ErrNotFound
	}
	return nil
}
func transact[T any](ctx context.Context, s *Store, f func(*sqlc.Queries) (T, error)) (T, error) {
	if s.writer == nil {
		return f(s.write)
	}
	var zero T
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return zero, err
	}
	defer tx.Rollback()
	value, err := f(s.write.WithTx(tx))
	if err != nil {
		return zero, dbError(err)
	}
	if err = tx.Commit(); err != nil {
		return zero, dbError(err)
	}
	return value, nil
}

func encodeStyles(styles []string) string {
	if styles == nil {
		styles = []string{}
	}
	b, _ := json.Marshal(styles)
	return string(b)
}
func strictJSON(value string, out any) error {
	dec := json.NewDecoder(strings.NewReader(value))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return err
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("invalid trailing JSON")
	}
	return nil
}

// A template row is its name and its outline body. A row without a readable
// body is corrupt rather than a template of some older kind (CLIP-4).
func templateRow(r sqlc.VideoTemplate) (clip.VideoTemplate, error) {
	created, err := time.Parse(time.RFC3339Nano, r.CreatedAt)
	if err != nil {
		return clip.VideoTemplate{}, err
	}
	updated, err := time.Parse(time.RFC3339Nano, r.UpdatedAt)
	if err != nil {
		return clip.VideoTemplate{}, err
	}
	if !r.CompositionBody.Valid {
		return clip.VideoTemplate{}, errors.New("stored template has no outline body")
	}
	styles, err := decodeCaptionStyles(r.AllowedCaptionStyles)
	if err != nil {
		return clip.VideoTemplate{}, err
	}
	t := clip.VideoTemplate{ID: r.ID, UserID: r.UserID, Recipe: clip.Recipe{Name: r.Name, CompositionBody: r.CompositionBody.String}, Design: clip.TemplateDesign{IntroPreset: r.IntroPreset, OutroPreset: r.OutroPreset, CaptionStyles: styles}, CreatedAt: created, UpdatedAt: updated}
	if _, e := composition.ReadStored(t.CompositionBody, clip.DefaultCompositionLimits()); e != nil {
		return t, e
	}
	return t, nil
}
func getTemplate(ctx context.Context, q *sqlc.Queries, user, id string) (clip.VideoTemplate, error) {
	r, err := q.GetVideoTemplate(ctx, sqlc.GetVideoTemplateParams{ID: id, UserID: user})
	if err != nil {
		return clip.VideoTemplate{}, dbError(err)
	}
	t, err := templateRow(r)
	if err != nil {
		return t, err
	}
	n, err := q.CountTemplateProjects(ctx, sqlc.CountTemplateProjectsParams{VideoTemplateID: nullable(id), UserID: user})
	t.ProjectCount = int(n)
	return t, err
}
func (s *Store) GetTemplate(ctx context.Context, user, id string) (clip.VideoTemplate, error) {
	return getTemplate(ctx, s.read, user, id)
}
func (s *Store) ListTemplates(ctx context.Context, user string) ([]clip.VideoTemplate, error) {
	rows, err := s.read.ListVideoTemplates(ctx, user)
	if err != nil {
		return nil, err
	}
	out := make([]clip.VideoTemplate, 0, len(rows))
	for _, r := range rows {
		t, err := templateRow(r)
		if err != nil {
			return nil, err
		}
		n, err := s.read.CountTemplateProjects(ctx, sqlc.CountTemplateProjectsParams{VideoTemplateID: nullable(t.ID), UserID: user})
		if err != nil {
			return nil, err
		}
		t.ProjectCount = int(n)
		out = append(out, t)
	}
	return out, nil
}
func (s *Store) InsertTemplate(ctx context.Context, t clip.VideoTemplate) error {
	_, err := transact(ctx, s, func(q *sqlc.Queries) (struct{}, error) {
		return struct{}{}, q.InsertVideoTemplate(ctx, sqlc.InsertVideoTemplateParams{ID: t.ID, UserID: t.UserID, Name: t.Name, CompositionBody: nullable(t.CompositionBody), IntroPreset: t.Design.IntroPreset, OutroPreset: t.Design.OutroPreset, AllowedCaptionStyles: encodeCaptionStyles(t.Design.CaptionStyles), CreatedAt: stamp(t.CreatedAt), UpdatedAt: stamp(t.UpdatedAt)})
	})
	return err
}
func (s *Store) UpdateTemplate(ctx context.Context, user, id string, p clip.TemplatePatch, now time.Time) (clip.VideoTemplate, error) {
	return transact(ctx, s, func(q *sqlc.Queries) (clip.VideoTemplate, error) {
		if _, err := getTemplate(ctx, q, user, id); err != nil {
			return clip.VideoTemplate{}, err
		}
		if err := freezeTemplateProjects(ctx, q, user, id); err != nil {
			return clip.VideoTemplate{}, err
		}
		if p.Name != nil {
			if err := affected(q.UpdateVideoTemplateName(ctx, sqlc.UpdateVideoTemplateNameParams{Name: *p.Name, UpdatedAt: stamp(now), ID: id, UserID: user})); err != nil {
				return clip.VideoTemplate{}, err
			}
		}
		if p.IntroPreset != nil || p.OutroPreset != nil || p.CaptionStyles != nil {
			current, err := getTemplate(ctx, q, user, id)
			if err != nil {
				return clip.VideoTemplate{}, err
			}
			design := current.Design
			if p.IntroPreset != nil {
				design.IntroPreset = *p.IntroPreset
			}
			if p.OutroPreset != nil {
				design.OutroPreset = *p.OutroPreset
			}
			if p.CaptionStyles != nil {
				design.CaptionStyles = *p.CaptionStyles
			}
			if e := affected(q.UpdateVideoTemplateDesign(ctx, sqlc.UpdateVideoTemplateDesignParams{IntroPreset: design.IntroPreset, OutroPreset: design.OutroPreset, AllowedCaptionStyles: encodeCaptionStyles(design.CaptionStyles), UpdatedAt: stamp(now), ID: id, UserID: user})); e != nil {
				return clip.VideoTemplate{}, e
			}
		}
		if p.CompositionBody != nil {
			if e := affected(q.SaveTemplateComposition(ctx, sqlc.SaveTemplateCompositionParams{CompositionBody: nullable(*p.CompositionBody), UpdatedAt: stamp(now), ID: id, UserID: user})); e != nil {
				return clip.VideoTemplate{}, e
			}
		}
		return getTemplate(ctx, q, user, id)
	})
}
func (s *Store) DeleteTemplate(ctx context.Context, user, id string) (int, error) {
	return transact(ctx, s, func(q *sqlc.Queries) (int, error) {
		n, err := q.CountTemplateProjects(ctx, sqlc.CountTemplateProjectsParams{VideoTemplateID: nullable(id), UserID: user})
		if err != nil {
			return 0, err
		}
		projects, e := q.ProjectsForTemplate(ctx, sqlc.ProjectsForTemplateParams{VideoTemplateID: nullable(id), UserID: user})
		if e != nil {
			return 0, e
		}
		for _, row := range projects {
			if row.FinalizedAt.Valid {
				continue
			}
			p, e := getProject(ctx, q, user, row.ID)
			if e != nil {
				return 0, e
			}
			if e = saveComposition(ctx, q, p); e != nil {
				return 0, e
			}
		}
		if err := affected(q.DeleteVideoTemplate(ctx, sqlc.DeleteVideoTemplateParams{ID: id, UserID: user})); err != nil {
			return 0, err
		}
		return int(n), nil
	})
}

// The allowed styles are one JSON array in one column: the layout reads them
// whole, nothing queries across projects, and the set is small and closed
// (CDS-80). An empty array is a selection of none, which resolves to the
// default style alone rather than to no captions at all.
// captionSetRestyles is whether a new AI set changes how the saved plan draws
// any caption, which is what moves its revision and leaves a render stale. Only
// a caption naming no style, or one the product no longer carries, takes the
// set's first entry (CaptionStyleOf); every other caption keeps its drawing, so
// a render of it stays current (CLIP-191). A plan this build cannot read counts
// as restyled, as every set change did before.
func captionSetRestyles(p clip.Project, next []string) bool {
	if p.EditPlan == "" || slices.Equal(p.CaptionStyles, next) {
		return false
	}
	plan, err := clip.DecodeEditPlan(p.EditPlan)
	if err != nil || plan.Portable == nil {
		return true
	}
	before, after := clip.ResolvedCaptionStyles(p.CaptionStyles), clip.ResolvedCaptionStyles(next)
	for _, text := range plan.Portable.Elements {
		if text.Resolved.Element.Role != "caption" {
			continue
		}
		was, _ := clip.CaptionStyleOf(text, before)
		now, _ := clip.CaptionStyleOf(text, after)
		if was != now {
			return true
		}
	}
	return false
}

func encodeCaptionStyles(styles []string) string {
	if styles == nil {
		styles = []string{}
	}
	raw, err := json.Marshal(styles)
	if err != nil {
		return "[]"
	}
	return string(raw)
}
func decodeCaptionStyles(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var styles []string
	if err := strictJSON(raw, &styles); err != nil {
		return nil, fmt.Errorf("stored caption styles: %w", err)
	}
	return styles, nil
}

func projectRow(r sqlc.ClipProject) (clip.Project, error) {
	created, err := time.Parse(time.RFC3339Nano, r.CreatedAt)
	if err != nil {
		return clip.Project{}, err
	}
	updated, err := time.Parse(time.RFC3339Nano, r.UpdatedAt)
	if err != nil {
		return clip.Project{}, err
	}
	styles, err := decodeCaptionStyles(r.AllowedCaptionStyles)
	if err != nil {
		return clip.Project{}, err
	}
	p := clip.Project{Dubbing: clip.DubbingOptions{Enabled: r.DubbingEnabled != 0, VoiceID: r.DubbingVoiceID, BindingDigest: r.DubbingBindingDigest}, ID: r.ID, UserID: r.UserID, Title: r.Title, VideoTemplateID: r.VideoTemplateID.String, Ratio: r.Ratio, Language: r.Language, Disclosure: r.Disclosure, HideDisclosure: r.HideDisclosure != 0, Instruction: r.Instruction, CaptionPace: r.CaptionPace, Accent: r.Accent, IntroPreset: r.IntroPreset, OutroPreset: r.OutroPreset, CaptionStyles: styles, TargetDurationMS: int(r.TargetDurationMs), Analysis: r.AnalysisJson.String, EditPlan: r.EditPlanJson.String, EditPlanRevision: int(r.EditPlanRevision), RenderedPlanRevision: int(r.RenderedPlanRevision), GeneratedPlanRevision: int(r.GeneratedPlanRevision), CreatedAt: created, UpdatedAt: updated}
	if r.ResultKey.Valid {
		at, err := time.Parse(time.RFC3339Nano, r.ResultCreatedAt.String)
		if err != nil {
			return clip.Project{}, err
		}
		p.Result = &clip.Result{Kind: clip.RenderKind(r.RenderKind), ID: r.ResultID.String, Key: r.ResultKey.String, ContentType: r.ResultContentType.String, Bytes: r.ResultBytes.Int64, DurationMS: int(r.ResultDurationMs.Int64), CreatedAt: at}
	}
	if r.FinalizedAt.Valid {
		at, err := time.Parse(time.RFC3339Nano, r.FinalizedAt.String)
		if err != nil {
			return clip.Project{}, err
		}
		p.Finalized = &clip.Finalization{At: at, PlanRevision: int(r.FinalizedPlanRevision.Int64), ResultID: r.ResultID.String}
	}
	p.Composition, err = decodeComposition(r.CompositionSnapshotJson.String, r.CompositionInputsJson.String)
	if err != nil {
		return p, err
	}
	if p.Regions, err = decodeRegions(r.RegionsJson.String); err != nil {
		return p, err
	}
	if p.Regions == nil {
		regions := clip.EffectiveProjectRegions(p)
		p.Regions = &regions
	}
	if p.Storyline, err = clip.DecodeStoryline(r.StorylineJson.String); err != nil {
		return p, err
	}
	return p, nil
}
func getProject(ctx context.Context, q *sqlc.Queries, user, id string) (clip.Project, error) {
	r, err := q.GetClipProject(ctx, sqlc.GetClipProjectParams{ID: id, UserID: user})
	if err != nil {
		return clip.Project{}, dbError(err)
	}
	p, err := projectRow(r)
	if err != nil {
		return p, err
	}
	if err = hydrateComposition(ctx, q, &p); err != nil {
		return p, err
	}
	return p, nil
}
func (s *Store) GetProject(ctx context.Context, user, id string) (clip.Project, error) {
	return getProject(ctx, s.read, user, id)
}
func (s *Store) ListProjects(ctx context.Context, user string) ([]clip.Project, error) {
	rows, err := s.read.ListClipProjects(ctx, user)
	if err != nil {
		return nil, err
	}
	out := make([]clip.Project, 0, len(rows))
	for _, r := range rows {
		p, err := projectRow(r)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

// ListProjectSummaries is ListProjects without each project's plan and
// analysis, which a directory row never reads.
func (s *Store) ListProjectSummaries(ctx context.Context, user string) ([]clip.Project, error) {
	rows, err := s.read.ListClipProjectSummaries(ctx, user)
	if err != nil {
		return nil, err
	}
	out := make([]clip.Project, 0, len(rows))
	for _, r := range rows {
		p, err := projectRow(sqlc.ClipProject{ID: r.ID, UserID: r.UserID, Title: r.Title, VideoTemplateID: r.VideoTemplateID, Ratio: r.Ratio, TargetDurationMs: r.TargetDurationMs,
			ResultKey: r.ResultKey, ResultContentType: r.ResultContentType, ResultBytes: r.ResultBytes, ResultDurationMs: r.ResultDurationMs, ResultCreatedAt: r.ResultCreatedAt,
			EditPlanRevision: r.EditPlanRevision, RenderedPlanRevision: r.RenderedPlanRevision, GeneratedPlanRevision: r.GeneratedPlanRevision, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
			Disclosure: r.Disclosure, HideDisclosure: r.HideDisclosure, CompositionInputsJson: r.CompositionInputsJson, CompositionSnapshotJson: r.CompositionSnapshotJson,
			ResultID: r.ResultID, FinalizedAt: r.FinalizedAt, FinalizedPlanRevision: r.FinalizedPlanRevision, Language: r.Language, Instruction: r.Instruction,
			CaptionPace: r.CaptionPace, Accent: r.Accent, IntroPreset: r.IntroPreset, OutroPreset: r.OutroPreset, AllowedCaptionStyles: r.AllowedCaptionStyles,
			DubbingEnabled: r.DubbingEnabled, DubbingVoiceID: r.DubbingVoiceID, DubbingBindingDigest: r.DubbingBindingDigest, RenderKind: r.RenderKind, StorylineJson: r.StorylineJson, RegionsJson: r.RegionsJson})
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}
func (s *Store) InsertProject(ctx context.Context, p clip.Project) error {
	_, err := transact(ctx, s, func(q *sqlc.Queries) (struct{}, error) {
		err := q.InsertClipProject(ctx, sqlc.InsertClipProjectParams{ID: p.ID, UserID: p.UserID, Title: p.Title, VideoTemplateID: nullable(p.VideoTemplateID), Ratio: p.Ratio, Language: p.Language, TargetDurationMs: int64(p.TargetDurationMS), Disclosure: p.Disclosure, HideDisclosure: disclosureFlag(p.HideDisclosure), Instruction: p.Instruction, CaptionPace: p.CaptionPace, Accent: p.Accent, IntroPreset: p.IntroPreset, OutroPreset: p.OutroPreset, AllowedCaptionStyles: encodeCaptionStyles(p.CaptionStyles), CreatedAt: stamp(p.CreatedAt), UpdatedAt: stamp(p.UpdatedAt)})
		if err == nil {
			err = saveComposition(ctx, q, p)
		}
		if err == nil {
			err = saveRegions(ctx, q, p)
		}
		if err == nil {
			err = affected(q.UpdateClipDubbing(ctx, sqlc.UpdateClipDubbingParams{ID: p.ID, UserID: p.UserID, DubbingEnabled: disclosureFlag(p.Dubbing.Enabled), DubbingVoiceID: p.Dubbing.VoiceID, DubbingBindingDigest: p.Dubbing.BindingDigest, UpdatedAt: stamp(p.UpdatedAt)}))
		}
		return struct{}{}, err
	})
	return err
}
func (s *Store) UpdateProject(ctx context.Context, user, id string, p clip.ProjectPatch, now time.Time) (clip.Project, error) {
	return transact(ctx, s, func(q *sqlc.Queries) (clip.Project, error) {
		before, err := getProject(ctx, q, user, id)
		if err != nil {
			return clip.Project{}, err
		}
		if before.Finalized != nil {
			return clip.Project{}, clip.ErrFinalized
		}
		if p.Composition != nil || p.Regions != nil || p.Dubbing != nil {
			active, e := q.HasActiveClipJob(ctx, nullable(id))
			if e != nil {
				return clip.Project{}, e
			}
			if active > 0 {
				return clip.Project{}, clip.ErrBusy
			}
		}
		if p.Composition != nil && p.ExpectedCompositionRevision != nil && before.EditPlanRevision != *p.ExpectedCompositionRevision {
			return clip.Project{}, clip.ErrPlanConflict
		}
		// The owner's storyline edit (CLIP-178): refused while a job holds the project, since
		// a storyline job would overwrite it and a build reads it; marked edited by hand.
		if p.Storyline != nil {
			active, e := q.HasActiveClipJob(ctx, nullable(id))
			if e != nil {
				return clip.Project{}, e
			}
			if active > 0 {
				return clip.Project{}, clip.ErrBusy
			}
			analyses, e := clip.RetainedObservations(before)
			if e != nil {
				return clip.Project{}, e
			}
			edited, e := clip.ApplyStorylineEdit(before.Storyline, *p.Storyline, analyses)
			if e != nil {
				return clip.Project{}, e
			}
			raw, e := clip.EncodeStoryline(edited)
			if e != nil {
				return clip.Project{}, e
			}
			if e := affected(q.SetClipStoryline(ctx, sqlc.SetClipStorylineParams{StorylineJson: nullable(raw), UserID: user, ID: id})); e != nil {
				return clip.Project{}, e
			}
		}

		if p.Regions != nil {
			if p.ExpectedRegionRevision == nil {
				return clip.Project{}, clip.ErrPlanConflict
			}
			next := p.Regions.Clone()
			next.Revision = *p.ExpectedRegionRevision
			if err := saveRegionState(ctx, q, before, next, now, false); err != nil {
				return clip.Project{}, err
			}
		}
		if p.Dubbing != nil {
			if err := affected(q.UpdateClipDubbing(ctx, sqlc.UpdateClipDubbingParams{ID: id, UserID: user, DubbingEnabled: disclosureFlag(p.Dubbing.Enabled), DubbingVoiceID: p.Dubbing.VoiceID, DubbingBindingDigest: p.Dubbing.BindingDigest, UpdatedAt: stamp(now)})); err != nil {
				return clip.Project{}, err
			}
		}
		if p.Title != nil {
			if err := affected(q.UpdateClipTitle(ctx, sqlc.UpdateClipTitleParams{Title: *p.Title, UpdatedAt: stamp(now), ID: id, UserID: user})); err != nil {
				return clip.Project{}, err
			}
		}
		if p.VideoTemplateID != nil {
			if err := affected(q.UpdateClipVideoTemplateID(ctx, sqlc.UpdateClipVideoTemplateIDParams{VideoTemplateID: nullable(*p.VideoTemplateID), UpdatedAt: stamp(now), ID: id, UserID: user})); err != nil {
				return clip.Project{}, err
			}
		}
		if p.TargetDurationMS != nil {
			if err := affected(q.UpdateClipTargetDurationMS(ctx, sqlc.UpdateClipTargetDurationMSParams{TargetDurationMs: int64(*p.TargetDurationMS), UpdatedAt: stamp(now), ID: id, UserID: user})); err != nil {
				return clip.Project{}, err
			}
		}
		if p.HideDisclosure != nil {
			if err := affected(q.UpdateClipHideDisclosure(ctx, sqlc.UpdateClipHideDisclosureParams{HideDisclosure: disclosureFlag(*p.HideDisclosure), UpdatedAt: stamp(now), ID: id, UserID: user})); err != nil {
				return clip.Project{}, err
			}
		}
		if p.Disclosure != nil {
			if err := affected(q.UpdateClipDisclosure(ctx, sqlc.UpdateClipDisclosureParams{Disclosure: *p.Disclosure, UpdatedAt: stamp(now), ID: id, UserID: user})); err != nil {
				return clip.Project{}, err
			}
		}
		if p.Instruction != nil {
			if err := affected(q.UpdateClipInstruction(ctx, sqlc.UpdateClipInstructionParams{Instruction: *p.Instruction, UpdatedAt: stamp(now), ID: id, UserID: user})); err != nil {
				return clip.Project{}, err
			}
		}
		// Both bump the plan revision where a plan exists, so the rendered
		// result goes stale without the plan itself changing (CLIP-139).
		if p.CaptionPace != nil {
			if err := affected(q.UpdateClipCaptionPace(ctx, sqlc.UpdateClipCaptionPaceParams{CaptionPace: *p.CaptionPace, UpdatedAt: stamp(now), ID: id, UserID: user})); err != nil {
				return clip.Project{}, err
			}
		}
		if p.Accent != nil {
			if err := affected(q.UpdateClipAccent(ctx, sqlc.UpdateClipAccentParams{Accent: *p.Accent, UpdatedAt: stamp(now), ID: id, UserID: user})); err != nil {
				return clip.Project{}, err
			}
		}
		if p.IntroPreset != nil {
			if err := affected(q.UpdateClipIntroPreset(ctx, sqlc.UpdateClipIntroPresetParams{IntroPreset: *p.IntroPreset, UpdatedAt: stamp(now), ID: id, UserID: user})); err != nil {
				return clip.Project{}, err
			}
		}
		if p.OutroPreset != nil {
			if err := affected(q.UpdateClipOutroPreset(ctx, sqlc.UpdateClipOutroPresetParams{OutroPreset: *p.OutroPreset, UpdatedAt: stamp(now), ID: id, UserID: user})); err != nil {
				return clip.Project{}, err
			}
		}
		if p.CaptionStyles != nil {
			step := int64(0)
			if captionSetRestyles(before, *p.CaptionStyles) {
				step = 1
			}
			if err := affected(q.UpdateClipAllowedCaptionStyles(ctx, sqlc.UpdateClipAllowedCaptionStylesParams{AllowedCaptionStyles: encodeCaptionStyles(*p.CaptionStyles), RevisionStep: step, UpdatedAt: stamp(now), ID: id, UserID: user})); err != nil {
				return clip.Project{}, err
			}
		}
		// Every region edit, preset change and seeding reaches the plan in this
		// same write, and the project's regions are the only words it draws there
		// (CLIP-188).
		if p.Regions != nil {
			if err := syncPlanRegions(ctx, q, before, now); err != nil {
				return clip.Project{}, err
			}
		}
		next, e := getProject(ctx, q, user, id)
		if e != nil {
			return next, e
		}
		if p.Composition != nil {
			next.Composition = p.Composition
			if !reflect.DeepEqual(before.Composition, p.Composition) {
				if e = affected(q.TouchCompositionRevision(ctx, sqlc.TouchCompositionRevisionParams{UpdatedAt: stamp(now), ID: id, UserID: user})); e != nil {
					return next, e
				}
			}
		}
		if e = saveComposition(ctx, q, next); e != nil {
			return next, e
		}
		next.UpdatedAt = before.UpdatedAt
		if !reflect.DeepEqual(before, next) {
			if e = renewProjectSources(ctx, q, user, id, now); e != nil {
				return next, e
			}
		}

		return getProject(ctx, q, user, id)
	})
}
func (s *Store) DeleteProject(ctx context.Context, user, id string) error {
	_, err := transact(ctx, s, func(q *sqlc.Queries) (struct{}, error) {
		p, err := getProject(ctx, q, user, id)
		if err != nil {
			return struct{}{}, err
		}
		if p.Result != nil {
			if err = q.EnqueueObjectDeletion(ctx, sqlc.EnqueueObjectDeletionParams{ObjectKey: p.Result.Key, CreatedAt: stamp(time.Now())}); err != nil {
				return struct{}{}, err
			}
		}
		batches, err := q.ListProjectSourceBatches(ctx, sqlc.ListProjectSourceBatchesParams{ProjectID: id, UserID: user})
		if err != nil {
			return struct{}{}, err
		}
		for _, b := range batches {
			if b.State != "cleanup_pending" {
				return struct{}{}, clip.ErrSourceState
			}
		}
		return struct{}{}, affected(q.DeleteClipProject(ctx, sqlc.DeleteClipProjectParams{ID: id, UserID: user}))
	})
	return err
}
