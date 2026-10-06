package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/store/sqlc"
	"strings"
	"time"
)

func (s *Store) PublishAuthoringTemplate(ctx context.Context, user string, in clip.AuthoringPublication, newID string, at time.Time) (clip.VideoTemplate, error) {
	if user == "" || in.Key.Key == "" || in.Key.SessionID == "" {
		return clip.VideoTemplate{}, clip.ErrAuthoringConflict
	}
	return transact(ctx, s, func(q *sqlc.Queries) (clip.VideoTemplate, error) {
		if err := q.LockAuthoringPublication(ctx); err != nil {
			return clip.VideoTemplate{}, err
		}
		prior, err := q.GetAuthoringPublication(ctx, sqlc.GetAuthoringPublicationParams{UserID: user, SessionID: in.Key.SessionID, Revision: int64(in.Key.Revision)})
		if err == nil {
			if prior.PublicationKey != in.Key.Key {
				return clip.VideoTemplate{}, clip.ErrAuthoringConflict
			}
			return getTemplate(ctx, q, user, prior.TargetID)
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return clip.VideoTemplate{}, err
		}
		stamp := stamp(at)
		id := in.TargetID
		if id == "" {
			id = newID
			if err := q.InsertVideoTemplate(ctx, sqlc.InsertVideoTemplateParams{ID: id, UserID: user, Name: in.Recipe.Name, CompositionBody: nullable(in.Recipe.CompositionBody), AllowedCaptionStyles: encodeCaptionStyles(nil), CreatedAt: stamp, UpdatedAt: stamp}); err != nil {
				if strings.Contains(strings.ToLower(err.Error()), "unique constraint") {
					return clip.VideoTemplate{}, clip.ErrDuplicateName
				}
				return clip.VideoTemplate{}, err
			}
		} else {
			current, err := getTemplate(ctx, q, user, id)
			if err != nil {
				return clip.VideoTemplate{}, err
			}
			if in.TargetVersion == "" || clip.AuthoringTemplateVersion(current) != in.TargetVersion {
				return clip.VideoTemplate{}, clip.ErrAuthoringConflict
			}
			if err := freezeTemplateProjects(ctx, q, user, id); err != nil {
				return clip.VideoTemplate{}, err
			}
			if _, err := q.UpdateVideoTemplateName(ctx, sqlc.UpdateVideoTemplateNameParams{ID: id, UserID: user, Name: in.Recipe.Name, UpdatedAt: stamp}); err != nil {
				if strings.Contains(strings.ToLower(err.Error()), "unique constraint") {
					return clip.VideoTemplate{}, clip.ErrDuplicateName
				}
				return clip.VideoTemplate{}, err
			}
			if _, err := q.SaveTemplateComposition(ctx, sqlc.SaveTemplateCompositionParams{ID: id, UserID: user, CompositionBody: nullable(in.Recipe.CompositionBody), UpdatedAt: stamp}); err != nil {
				return clip.VideoTemplate{}, err
			}
		}
		if err := q.InsertAuthoringPublication(ctx, sqlc.InsertAuthoringPublicationParams{UserID: user, SessionID: in.Key.SessionID, Revision: int64(in.Key.Revision), PublicationKey: in.Key.Key, TargetID: id, CreatedAt: stamp}); err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "unique constraint") {
				return clip.VideoTemplate{}, clip.ErrAuthoringConflict
			}
			return clip.VideoTemplate{}, fmt.Errorf("record video template publication: %w", err)
		}
		return getTemplate(ctx, q, user, id)
	})
}

var _ clip.AuthoringStore = (*Store)(nil)
