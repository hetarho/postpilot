package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/postpilot/backend/internal/template"
	"github.com/postpilot/backend/internal/template/store/sqlc"
)

func (s *Store) CountAuthoringTargets(ctx context.Context, user string) (int, error) {
	n, err := s.read.CountTemplates(ctx, user)
	return int(n), err
}

func (s *Store) PublishAuthoring(ctx context.Context, user string, in template.AuthoringPublication, newID string, at time.Time, max int) (template.Template, error) {
	if user == "" || strings.TrimSpace(in.Key.Key) == "" || strings.TrimSpace(in.Key.SessionID) == "" {
		return template.Template{}, template.ErrAuthoringConflict
	}
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return template.Template{}, err
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)
	if err := q.LockAuthoringPublication(ctx); err != nil {
		return template.Template{}, err
	}
	prior, err := q.GetAuthoringPublication(ctx, sqlc.GetAuthoringPublicationParams{UserID: user, SessionID: in.Key.SessionID, Revision: int64(in.Key.Revision)})
	if err == nil {
		if prior.PublicationKey != in.Key.Key {
			return template.Template{}, template.ErrAuthoringConflict
		}
		current, err := s.get(ctx, q, user, prior.TargetID)
		if err != nil {
			return template.Template{}, err
		}
		return current, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return template.Template{}, err
	}
	stamp := formatTime(at)
	id := in.TargetID
	if id == "" {
		count, err := q.CountTemplates(ctx, user)
		if err != nil {
			return template.Template{}, err
		}
		if int(count) >= max {
			return template.Template{}, template.ErrTooMany
		}
		id = newID
		if err := q.InsertTemplate(ctx, sqlc.InsertTemplateParams{ID: id, UserID: user, Name: in.Draft.Name, Description: in.Draft.Description, Body: in.Draft.Body, TitleArea: in.Draft.TitleArea, CreatedAt: stamp, UpdatedAt: stamp}); err != nil {
			if isUniqueViolation(err) {
				return template.Template{}, template.ErrDuplicateName
			}
			return template.Template{}, err
		}
	} else {
		current, err := s.get(ctx, q, user, id)
		if err != nil {
			return template.Template{}, err
		}
		if in.TargetVersion == "" || template.AuthoringVersion(current) != in.TargetVersion {
			return template.Template{}, template.ErrAuthoringConflict
		}
		for _, write := range []func() error{
			func() error {
				_, err := q.UpdateTemplateName(ctx, sqlc.UpdateTemplateNameParams{ID: id, UserID: user, Name: in.Draft.Name, UpdatedAt: stamp})
				return err
			},
			func() error {
				_, err := q.UpdateTemplateDescription(ctx, sqlc.UpdateTemplateDescriptionParams{ID: id, UserID: user, Description: in.Draft.Description, UpdatedAt: stamp})
				return err
			},
			func() error {
				_, err := q.UpdateTemplateBody(ctx, sqlc.UpdateTemplateBodyParams{ID: id, UserID: user, Body: in.Draft.Body, UpdatedAt: stamp})
				return err
			},
			func() error {
				_, err := q.UpdateTemplateTitleArea(ctx, sqlc.UpdateTemplateTitleAreaParams{ID: id, UserID: user, TitleArea: in.Draft.TitleArea, UpdatedAt: stamp})
				return err
			},
		} {
			if err := write(); err != nil {
				if isUniqueViolation(err) {
					return template.Template{}, template.ErrDuplicateName
				}
				return template.Template{}, err
			}
		}
	}
	if err := q.InsertAuthoringPublication(ctx, sqlc.InsertAuthoringPublicationParams{UserID: user, SessionID: in.Key.SessionID, Revision: int64(in.Key.Revision), PublicationKey: in.Key.Key, TargetID: id, CreatedAt: stamp}); err != nil {
		return template.Template{}, fmt.Errorf("record template publication: %w", err)
	}
	current, err := s.get(ctx, q, user, id)
	if err != nil {
		return template.Template{}, err
	}
	return current, tx.Commit()
}

var _ template.AuthoringStore = (*Store)(nil)
