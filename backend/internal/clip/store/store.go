// Package store maps clip aggregates to owned SQLite rows.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/clip/store/sqlc"
	"github.com/postpilot/backend/internal/platform/config"
	"io"
	"reflect"
	"strings"
	"time"
)

const timeLayout = "2006-01-02T15:04:05.000000000Z07:00"

type Store struct {
	writer      *sql.DB
	read, write *sqlc.Queries
}

func New(writer, reader *sql.DB) *Store {
	return &Store{writer: writer, read: sqlc.New(reader), write: sqlc.New(writer)}
}

var _ clip.Store = (*Store)(nil)

func disclosureFlag(hidden bool) int64 {
	if hidden {
		return 1
	}
	return 0
}

func stamp(t time.Time) string         { return t.UTC().Format(timeLayout) }
func nullable(s string) sql.NullString { return sql.NullString{String: s, Valid: s != ""} }
func dbError(err error) error {
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

type fieldJSON struct {
	Label  string `json:"label"`
	Prompt string `json:"prompt"`
}

func encodeFields(fields []clip.InformationField) string {
	out := make([]fieldJSON, 0, len(fields))
	for _, f := range fields {
		out = append(out, fieldJSON{f.Label, f.Prompt})
	}
	b, _ := json.Marshal(out)
	return string(b)
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
func templateRow(r sqlc.VideoTemplate) (clip.VideoTemplate, error) {
	var fields []fieldJSON
	var styles []string
	if err := strictJSON(r.InformationFields, &fields); err != nil {
		return clip.VideoTemplate{}, fmt.Errorf("stored information fields: %w", err)
	}
	if err := strictJSON(r.CopyStyles, &styles); err != nil {
		return clip.VideoTemplate{}, fmt.Errorf("stored copy styles: %w", err)
	}
	if fields == nil || (!r.CompositionBody.Valid || r.CompositionLegacy != 0) && !clip.ValidCopyStyles(styles) || !clip.ValidAccent(r.Accent.String) || !clip.ValidCaptionPace(r.CaptionPace) {
		return clip.VideoTemplate{}, errors.New("stored recipe must contain JSON arrays")
	}
	created, err := time.Parse(time.RFC3339Nano, r.CreatedAt)
	if err != nil {
		return clip.VideoTemplate{}, err
	}
	updated, err := time.Parse(time.RFC3339Nano, r.UpdatedAt)
	if err != nil {
		return clip.VideoTemplate{}, err
	}
	t := clip.VideoTemplate{ID: r.ID, UserID: r.UserID, Recipe: clip.Recipe{Name: r.Name, CutGuidance: r.CutGuidance, CopyStyles: styles, Accent: r.Accent.String, Preset: r.Preset, CaptionPace: r.CaptionPace}, CreatedAt: created, UpdatedAt: updated}
	seen := make(map[string]bool, len(fields))
	for _, f := range fields {
		if (!r.CompositionBody.Valid || r.CompositionLegacy != 0) && (strings.TrimSpace(f.Label) == "" || strings.TrimSpace(f.Prompt) == "" || seen[f.Label]) {
			return clip.VideoTemplate{}, errors.New("malformed stored information field")
		}
		seen[f.Label] = true
		t.InformationFields = append(t.InformationFields, clip.InformationField{Label: f.Label, Prompt: f.Prompt})
	}
	t.CompositionBody = r.CompositionBody.String
	t.CompositionLegacy = r.CompositionLegacy != 0 || !r.CompositionBody.Valid
	if !r.CompositionBody.Valid {
		t.CompositionBody = clip.LegacyCompositionBody(t.Recipe)
	}
	limits := config.ClipCompositionLimits()
	if t.CompositionLegacy {
		limits = clip.LegacyCompositionLimits(limits)
	}
	if _, e := composition.Parse(t.CompositionBody, limits); e != nil {
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
		if e := q.InsertVideoTemplate(ctx, sqlc.InsertVideoTemplateParams{ID: t.ID, UserID: t.UserID, Name: t.Name, InformationFields: encodeFields(t.InformationFields), CutGuidance: t.CutGuidance, CopyStyles: encodeStyles(t.CopyStyles), Accent: nullable(t.Accent), Preset: t.Preset, CaptionPace: t.CaptionPace, CreatedAt: stamp(t.CreatedAt), UpdatedAt: stamp(t.UpdatedAt)}); e != nil {
			return struct{}{}, e
		}
		if t.CompositionBody != "" {
			if e := affected(q.SaveTemplateComposition(ctx, sqlc.SaveTemplateCompositionParams{CompositionBody: nullable(t.CompositionBody), CompositionLegacy: disclosureFlag(t.CompositionLegacy), ID: t.ID, UserID: t.UserID})); e != nil {
				return struct{}{}, e
			}
		}
		return struct{}{}, nil
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
		if p.InformationFields != nil {
			if err := affected(q.UpdateVideoTemplateInformationFields(ctx, sqlc.UpdateVideoTemplateInformationFieldsParams{InformationFields: encodeFields(*p.InformationFields), UpdatedAt: stamp(now), ID: id, UserID: user})); err != nil {
				return clip.VideoTemplate{}, err
			}
		}
		if p.CutGuidance != nil {
			if err := affected(q.UpdateVideoTemplateCutGuidance(ctx, sqlc.UpdateVideoTemplateCutGuidanceParams{CutGuidance: *p.CutGuidance, UpdatedAt: stamp(now), ID: id, UserID: user})); err != nil {
				return clip.VideoTemplate{}, err
			}
		}
		if p.CaptionPace != nil {
			if err := affected(q.UpdateVideoTemplateCaptionPace(ctx, sqlc.UpdateVideoTemplateCaptionPaceParams{CaptionPace: *p.CaptionPace, UpdatedAt: stamp(now), ID: id, UserID: user})); err != nil {
				return clip.VideoTemplate{}, err
			}
		}
		if p.CopyStyles != nil {
			if err := affected(q.UpdateVideoTemplateCopyStyles(ctx, sqlc.UpdateVideoTemplateCopyStylesParams{CopyStyles: encodeStyles(*p.CopyStyles), UpdatedAt: stamp(now), ID: id, UserID: user})); err != nil {
				return clip.VideoTemplate{}, err
			}
		}
		if p.Accent != nil {
			if err := affected(q.UpdateVideoTemplateAccent(ctx, sqlc.UpdateVideoTemplateAccentParams{Accent: nullable(*p.Accent), UpdatedAt: stamp(now), ID: id, UserID: user})); err != nil {
				return clip.VideoTemplate{}, err
			}
		}
		if p.Preset != nil {
			if err := affected(q.UpdateVideoTemplatePreset(ctx, sqlc.UpdateVideoTemplatePresetParams{Preset: *p.Preset, UpdatedAt: stamp(now), ID: id, UserID: user})); err != nil {
				return clip.VideoTemplate{}, err
			}
		}
		if p.CompositionBody != nil {
			if e := affected(q.SaveTemplateComposition(ctx, sqlc.SaveTemplateCompositionParams{CompositionBody: nullable(*p.CompositionBody), CompositionLegacy: 0, ID: id, UserID: user})); e != nil {
				return clip.VideoTemplate{}, e
			}
		}
		t, e := getTemplate(ctx, q, user, id)
		if e != nil {
			return t, e
		}
		if t.CompositionLegacy {
			t.CompositionBody = clip.LegacyCompositionBody(t.Recipe)
			if e = affected(q.SaveTemplateComposition(ctx, sqlc.SaveTemplateCompositionParams{CompositionBody: nullable(t.CompositionBody), CompositionLegacy: 1, ID: id, UserID: user})); e != nil {
				return t, e
			}
		}
		return t, nil
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
func projectRow(r sqlc.ClipProject) (clip.Project, error) {
	created, err := time.Parse(time.RFC3339Nano, r.CreatedAt)
	if err != nil {
		return clip.Project{}, err
	}
	updated, err := time.Parse(time.RFC3339Nano, r.UpdatedAt)
	if err != nil {
		return clip.Project{}, err
	}
	p := clip.Project{ID: r.ID, UserID: r.UserID, Title: r.Title, VideoTemplateID: r.VideoTemplateID.String, Ratio: r.Ratio, Disclosure: r.Disclosure, HideDisclosure: r.HideDisclosure != 0, CTA: r.Cta, TargetDurationMS: int(r.TargetDurationMs), Analysis: r.AnalysisJson.String, EditPlan: r.EditPlanJson.String, EditPlanRevision: int(r.EditPlanRevision), RenderedPlanRevision: int(r.RenderedPlanRevision), CreatedAt: created, UpdatedAt: updated}
	if r.ResultKey.Valid {
		at, err := time.Parse(time.RFC3339Nano, r.ResultCreatedAt.String)
		if err != nil {
			return clip.Project{}, err
		}
		p.Result = &clip.Result{Key: r.ResultKey.String, ContentType: r.ResultContentType.String, Bytes: r.ResultBytes.Int64, DurationMS: int(r.ResultDurationMs.Int64), CreatedAt: at}
	}
	p.Composition, err = decodeComposition(r.CompositionSnapshotJson.String, r.CompositionInputsJson.String)
	if err != nil {
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
	answers, err := q.ListClipAnswers(ctx, sqlc.ListClipAnswersParams{ProjectID: id, UserID: user})
	if err != nil {
		return p, err
	}
	for _, a := range answers {
		p.Answers = append(p.Answers, clip.Answer{Label: a.Label, Text: a.Answer})
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
func saveAnswers(ctx context.Context, q *sqlc.Queries, user, id string, answers []clip.Answer, now time.Time) error {
	for _, a := range answers {
		if err := q.UpsertClipAnswer(ctx, sqlc.UpsertClipAnswerParams{ProjectID: id, UserID: user, Label: a.Label, Answer: a.Text, UpdatedAt: stamp(now)}); err != nil {
			return err
		}
	}
	return nil
}
func (s *Store) InsertProject(ctx context.Context, p clip.Project) error {
	_, err := transact(ctx, s, func(q *sqlc.Queries) (struct{}, error) {
		err := q.InsertClipProject(ctx, sqlc.InsertClipProjectParams{ID: p.ID, UserID: p.UserID, Title: p.Title, VideoTemplateID: nullable(p.VideoTemplateID), Ratio: p.Ratio, TargetDurationMs: int64(p.TargetDurationMS), Disclosure: p.Disclosure, HideDisclosure: disclosureFlag(p.HideDisclosure), Cta: p.CTA, CreatedAt: stamp(p.CreatedAt), UpdatedAt: stamp(p.UpdatedAt)})
		if err == nil {
			err = saveAnswers(ctx, q, p.UserID, p.ID, p.Answers, p.UpdatedAt)
		}
		if err == nil {
			err = saveComposition(ctx, q, p)
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
		if p.Composition != nil {
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
		if p.CTA != nil {
			if err := affected(q.UpdateClipCTA(ctx, sqlc.UpdateClipCTAParams{Cta: *p.CTA, UpdatedAt: stamp(now), ID: id, UserID: user})); err != nil {
				return clip.Project{}, err
			}
		}
		if len(p.Answers) > 0 {
			if err := saveAnswers(ctx, q, user, id, p.Answers, now); err != nil {
				return clip.Project{}, err
			}
			if err := affected(q.TouchClip(ctx, sqlc.TouchClipParams{UpdatedAt: stamp(now), ID: id, UserID: user})); err != nil {
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
		} else if next.Composition != nil && next.Composition.Snapshot.Legacy && (len(p.Answers) > 0 || p.Disclosure != nil || p.HideDisclosure != nil || p.CTA != nil) {
			recipe := clip.Recipe{CopyStyles: []string{"clean"}}
			if before.Composition != nil && before.Composition.Snapshot.LegacyRecipe != nil {
				recipe = *before.Composition.Snapshot.LegacyRecipe
			}
			c := clip.LegacyProjectComposition(next, recipe)
			c.Snapshot.TemplateID = before.Composition.Snapshot.TemplateID
			next.Composition = &c
		}
		if e = saveComposition(ctx, q, next); e != nil {
			return next, e
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
