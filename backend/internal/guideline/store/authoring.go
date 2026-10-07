package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/postpilot/backend/internal/guideline"
	"github.com/postpilot/backend/internal/guideline/store/sqlc"
	"strings"
	"time"
)

func (s *Store) CountAuthoringTargets(ctx context.Context, user string, kind guideline.Kind) (int, error) {
	n, err := s.read.CountGuidelines(ctx, sqlc.CountGuidelinesParams{UserID: user, Kind: string(kind)})
	return int(n), err
}
func (s *Store) PublishAuthoring(ctx context.Context, user string, in guideline.AuthoringPublication, newID string, at time.Time, max int) (guideline.Guideline, error) {
	if user == "" || in.Key.Key == "" || in.Key.SessionID == "" || !in.Kind.Valid() {
		return guideline.Guideline{}, guideline.ErrAuthoringConflict
	}
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return guideline.Guideline{}, err
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)
	if err := q.LockAuthoringPublication(ctx); err != nil {
		return guideline.Guideline{}, err
	}
	prior, err := q.GetAuthoringPublication(ctx, sqlc.GetAuthoringPublicationParams{UserID: user, SessionID: in.Key.SessionID, Revision: int64(in.Key.Revision)})
	if err == nil {
		if prior.PublicationKey != in.Key.Key || prior.Kind != string(in.Kind) {
			return guideline.Guideline{}, guideline.ErrAuthoringConflict
		}
		current, err := s.get(ctx, q, user, prior.TargetID)
		if err != nil {
			return guideline.Guideline{}, err
		}
		return current, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return guideline.Guideline{}, err
	}
	stamp := formatTime(at)
	id := in.TargetID
	if id == "" {
		count, err := q.CountGuidelines(ctx, sqlc.CountGuidelinesParams{UserID: user, Kind: string(in.Kind)})
		if err != nil {
			return guideline.Guideline{}, err
		}
		if int(count) >= max {
			return guideline.Guideline{}, &guideline.AccountCapError{Max: max}
		}
		id = newID
		if err := q.InsertGuideline(ctx, sqlc.InsertGuidelineParams{ID: id, UserID: user, Kind: string(in.Kind), Title: in.Draft.Name, Text: in.Draft.Body, Scope: string(guideline.ScopeGlobal), CreatedAt: stamp, UpdatedAt: stamp}); err != nil {
			if isDuplicateText(err) {
				return guideline.Guideline{}, guideline.ErrDuplicateText
			}
			return guideline.Guideline{}, err
		}
	} else {
		current, err := s.get(ctx, q, user, id)
		if err != nil {
			return guideline.Guideline{}, err
		}
		if current.Kind != in.Kind {
			return guideline.Guideline{}, guideline.ErrNotFound
		}
		if in.TargetVersion == "" || guideline.AuthoringVersion(current) != in.TargetVersion {
			return guideline.Guideline{}, guideline.ErrAuthoringConflict
		}
		if _, err := q.UpdateGuidelineTitle(ctx, sqlc.UpdateGuidelineTitleParams{ID: id, UserID: user, Title: in.Draft.Name, UpdatedAt: stamp}); err != nil {
			return guideline.Guideline{}, err
		}
		if _, err := q.UpdateGuidelineText(ctx, sqlc.UpdateGuidelineTextParams{ID: id, UserID: user, Text: in.Draft.Body, UpdatedAt: stamp}); err != nil {
			if isDuplicateText(err) {
				return guideline.Guideline{}, guideline.ErrDuplicateText
			}
			return guideline.Guideline{}, err
		}
	}
	if in.Scope != nil {
		if err := replaceAuthoringScope(ctx, q, user, in.Kind, id, *in.Scope, stamp); err != nil {
			return guideline.Guideline{}, err
		}
	}
	if err := approve(ctx, q, user, in.Kind, guideline.CandidateApproval{Text: in.Draft.Body}); err != nil {
		return guideline.Guideline{}, err
	}
	if err := q.InsertAuthoringPublication(ctx, sqlc.InsertAuthoringPublicationParams{UserID: user, SessionID: in.Key.SessionID, Revision: int64(in.Key.Revision), PublicationKey: in.Key.Key, TargetID: id, Kind: string(in.Kind), CreatedAt: stamp}); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique constraint") {
			return guideline.Guideline{}, guideline.ErrAuthoringConflict
		}
		return guideline.Guideline{}, fmt.Errorf("record guideline publication: %w", err)
	}
	current, err := s.get(ctx, q, user, id)
	if err != nil {
		return guideline.Guideline{}, err
	}
	return current, tx.Commit()
}

var _ guideline.AuthoringStore = (*Store)(nil)

func replaceAuthoringScope(ctx context.Context, q *sqlc.Queries, user string, kind guideline.Kind, id string, scope guideline.ScopePatch, at string) error {
	if _, err := q.UpdateGuidelineScope(ctx, sqlc.UpdateGuidelineScopeParams{ID: id, UserID: user, Scope: string(scope.Scope), UpdatedAt: at}); err != nil {
		return err
	}
	if err := q.DeleteGuidelineScope(ctx, sqlc.DeleteGuidelineScopeParams{GuidelineID: id, UserID: user}); err != nil {
		return err
	}
	if err := q.DeleteGuidelineVideoTemplates(ctx, sqlc.DeleteGuidelineVideoTemplatesParams{GuidelineID: id, UserID: user}); err != nil {
		return err
	}
	if err := q.DeleteGuidelineFieldLinks(ctx, sqlc.DeleteGuidelineFieldLinksParams{GuidelineID: id, UserID: user}); err != nil {
		return err
	}
	if err := insertScope(ctx, q, user, kind, id, scope.TemplateIDs); err != nil {
		return err
	}
	return insertFields(ctx, q, user, id, scope.Fields)
}
