package store

import (
	"context"
	"database/sql"
	"errors"
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/store/sqlc"
	"reflect"
	"time"
)

func reuseSpeechAssets(ctx context.Context, q *sqlc.Queries, owner, project string, p *clip.EditPlan) (bool, error) {
	if p.Narration == nil || p.Narration.BindingDigest == "" {
		return false, nil
	}
	changed := false
	for i, segment := range p.Narration.Segments {
		if clip.CompatibleSpeech(p.Narration, segment) {
			continue
		}
		row, err := q.FindClipSpeechAsset(ctx, sqlc.FindClipSpeechAssetParams{OwnerID: owner, ProjectID: project, InputHash: segment.InputHash, BindingDigest: p.Narration.BindingDigest})
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return false, err
		}
		a, err := speechAsset(row)
		if err != nil {
			return false, err
		}
		if a.Text != segment.Text || a.Speech.VoiceID != p.Narration.VoiceID {
			return false, clip.ErrInvalid
		}
		p.Narration.Segments[i].Speech = &a.Speech
		changed = true
	}
	return changed, nil
}

func (s *Store) InsertSpeechAsset(ctx context.Context, a clip.SpeechAsset) error {
	if a.ID == "" || a.Speech.AssetID != a.ID || a.OwnerID == "" || a.ProjectID == "" || a.ObjectKey == "" || a.CreatedAt.IsZero() || a.Speech.InputHash != clip.SpokenInputHash(a.Text) {
		return clip.ErrInvalid
	}
	test := clip.EditPlan{Narration: &clip.NarrationPlan{VolumePermille: 1000, Segments: []clip.SpokenSegment{{ID: "spoken-1", Text: a.Text, TextRevision: 1, InputHash: a.Speech.InputHash, EndMS: max(1, a.Speech.DurationMS()), Speech: &a.Speech}}}}
	if err := clip.ValidateNarration(test); err != nil {
		return err
	}
	raw, err := clip.EncodeSpeechReference(a.Speech)
	if err != nil {
		return err
	}
	_, err = transact(ctx, s, func(q *sqlc.Queries) (struct{}, error) {
		return struct{}{}, affected(q.InsertClipSpeechAsset(ctx, sqlc.InsertClipSpeechAssetParams{ID: a.ID, OwnerID: a.OwnerID, ProjectID: a.ProjectID, ObjectKey: a.ObjectKey, InputText: a.Text, InputHash: a.Speech.InputHash, BindingDigest: a.Speech.BindingDigest, SpeechJson: raw, CreatedAt: stamp(a.CreatedAt), ProjectCheck: a.ProjectID, OwnerCheck: a.OwnerID}))
	})
	return err
}
func speechAsset(row sqlc.ClipSpeechAsset) (clip.SpeechAsset, error) {
	audio, err := clip.DecodeSpeechReference(row.SpeechJson)
	if err != nil {
		return clip.SpeechAsset{}, err
	}
	created, err := time.Parse(time.RFC3339Nano, row.CreatedAt)
	if err != nil {
		return clip.SpeechAsset{}, err
	}
	return clip.SpeechAsset{ID: row.ID, OwnerID: row.OwnerID, ProjectID: row.ProjectID, ObjectKey: row.ObjectKey, Text: row.InputText, Speech: audio, CreatedAt: created}, nil
}
func getSpeechAsset(ctx context.Context, q *sqlc.Queries, owner, project, id string) (clip.SpeechAsset, error) {
	row, err := q.GetClipSpeechAsset(ctx, sqlc.GetClipSpeechAssetParams{ID: id, OwnerID: owner, ProjectID: project})
	if err != nil {
		return clip.SpeechAsset{}, dbError(err)
	}
	return speechAsset(row)
}
func (s *Store) GetSpeechAsset(ctx context.Context, owner, project, id string) (clip.SpeechAsset, error) {
	return getSpeechAsset(ctx, s.read, owner, project, id)
}
func (s *Store) FindSpeechAsset(ctx context.Context, owner, project, hash, binding string) (clip.SpeechAsset, error) {
	row, err := s.read.FindClipSpeechAsset(ctx, sqlc.FindClipSpeechAssetParams{OwnerID: owner, ProjectID: project, InputHash: hash, BindingDigest: binding})
	if err != nil {
		return clip.SpeechAsset{}, dbError(err)
	}
	return speechAsset(row)
}
func validateSpeechAssets(ctx context.Context, q *sqlc.Queries, owner, project string, p clip.EditPlan) error {
	if err := clip.ValidateNarration(p); err != nil {
		return err
	}
	if p.Narration == nil {
		return nil
	}
	for _, s := range p.Narration.Segments {
		if s.Speech != nil {
			a, err := getSpeechAsset(ctx, q, owner, project, s.Speech.AssetID)
			if err != nil {
				return err
			}
			if !reflect.DeepEqual(a.Speech, *s.Speech) {
				return clip.ErrInvalid
			}
		}
	}
	return nil
}

// Publication uses the same writer-transaction revision CAS as an owner save.
// The asset row is immutable, and is independently checked inside that CAS.
func (s *Store) PublishSpeechAsset(ctx context.Context, owner, project, job string, revision int, segment, hash, binding, asset string) (clip.Project, error) {
	p, err := s.GetProject(ctx, owner, project)
	if err != nil {
		return clip.Project{}, err
	}
	a, err := s.GetSpeechAsset(ctx, owner, project, asset)
	if err != nil {
		return clip.Project{}, err
	}
	plan, err := clip.DecodeEditPlan(p.EditPlan)
	if err != nil {
		return clip.Project{}, err
	}
	plan, err = clip.PublishSpeech(plan, revision, p.EditPlanRevision, segment, hash, binding, a.Speech)
	if err != nil {
		return clip.Project{}, err
	}
	raw, err := clip.EncodeEditPlan(plan)
	if err != nil {
		return clip.Project{}, err
	}
	return s.saveCorrection(ctx, owner, project, job, revision, raw, nil, false)
}
