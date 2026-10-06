package store

import (
	"context"
	"encoding/json"
	"reflect"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/store/sqlc"
)

func browserRender(row sqlc.ClipBrowserRender) (clip.BrowserRender, error) {
	r := clip.BrowserRender{ID: row.ID, UserID: row.UserID, ProjectID: row.ProjectID, Revision: int(row.PlanRevision), Ratio: row.Ratio, DurationMS: int(row.DurationMs), Audio: row.HasAudio != 0, UploadBytes: row.UploadBytes}
	var err error
	r.Speech, r.Composition, err = decodeBrowserComposition(row.SpeechJson)
	if err != nil {
		return r, err
	}
	r.CreatedAt, err = time.Parse(time.RFC3339Nano, row.CreatedAt)
	if err == nil && row.StoredAt.Valid {
		var at time.Time
		at, err = time.Parse(time.RFC3339Nano, row.StoredAt.String)
		r.StoredAt = &at
	}
	if err == nil && row.CancelledAt.Valid {
		var at time.Time
		at, err = time.Parse(time.RFC3339Nano, row.CancelledAt.String)
		r.CancelledAt = &at
	}
	if err == nil && row.VerdictJson.Valid {
		err = json.Unmarshal([]byte(row.VerdictJson.String), &r.Verdict)
	}
	r.SampleJobID = row.SampleJobID.String
	if err == nil && row.SampledAt.Valid {
		var at time.Time
		at, err = time.Parse(time.RFC3339Nano, row.SampledAt.String)
		r.SampledAt = &at
		if err == nil && row.GroundsJson.Valid {
			err = json.Unmarshal([]byte(row.GroundsJson.String), &r.Grounds)
		}
	}
	return r, err
}

// Recheck ownership, revision and activity in the same transaction as the
// record. Layout has no write lock, so a save may win while it is being checked.
func checkBrowserProject(ctx context.Context, q *sqlc.Queries, r clip.BrowserRender) error {
	return checkBrowserProjectExcept(ctx, q, r, "")
}

// checkBrowserProjectExcept is the same check made from inside a render's own
// sampling job, which is the project's one active job while it runs.
func checkBrowserProjectExcept(ctx context.Context, q *sqlc.Queries, r clip.BrowserRender, self string) error {
	if r.CancelledAt != nil {
		return clip.ErrSourceState
	}
	p, err := getProject(ctx, q, r.UserID, r.ProjectID)
	if err != nil {
		return err
	}
	if p.Finalized != nil {
		return clip.ErrFinalized
	}
	access, err := q.SourceProjectAccess(ctx, sqlc.SourceProjectAccessParams{ID: r.ProjectID, UserID: r.UserID})
	if err != nil {
		return err
	}
	if access.Deleting != 0 || access.SourceAccessRevokedAt.Valid {
		return clip.ErrSourceState
	}
	if p.EditPlanRevision != r.Revision {
		return clip.ErrPlanConflict
	}
	var busy int64
	if self == "" {
		busy, err = q.HasActiveClipJob(ctx, nullable(r.ProjectID))
	} else {
		busy, err = q.HasOtherActiveClipJob(ctx, sqlc.HasOtherActiveClipJobParams{ProjectID: nullable(r.ProjectID), JobID: self})
	}
	if err != nil {
		return err
	}
	if busy != 0 {
		return clip.ErrBusy
	}
	return nil
}

// BindBrowserRenderSampling names the job a live render's grounds come from;
// naming the same job again is a no-op and naming another is a conflict.
func (s *Store) BindBrowserRenderSampling(ctx context.Context, user, render, job string) error {
	n, err := s.write.BindBrowserRenderSampling(ctx, sqlc.BindBrowserRenderSamplingParams{JobID: nullable(job), ID: render, UserID: user})
	if err != nil {
		return dbError(err)
	}
	if n == 0 {
		if _, err := s.GetBrowserRender(ctx, user, render); err != nil {
			return err
		}
		return clip.ErrPlanConflict
	}
	return nil
}

// SaveBrowserRenderGrounds keeps what the render's own sampling job measured,
// once, while the render is live at its revision and the project unchanged.
func (s *Store) SaveBrowserRenderGrounds(ctx context.Context, user, render, job string, revision int, grounds []clip.SampledGround, now time.Time) error {
	if grounds == nil {
		grounds = []clip.SampledGround{}
	}
	raw, err := json.Marshal(grounds)
	if err != nil {
		return err
	}
	_, err = transact(ctx, s, func(q *sqlc.Queries) (struct{}, error) {
		row, err := q.GetBrowserRender(ctx, sqlc.GetBrowserRenderParams{ID: render, UserID: user})
		if err != nil {
			return struct{}{}, err
		}
		r, err := browserRender(row)
		if err != nil {
			return struct{}{}, err
		}
		if r.SampleJobID != job || r.Revision != revision {
			return struct{}{}, clip.ErrPlanConflict
		}
		if r.SampledAt != nil {
			var stored []clip.SampledGround
			if json.Unmarshal([]byte(row.GroundsJson.String), &stored) != nil || !reflect.DeepEqual(append([]clip.SampledGround{}, stored...), grounds) {
				return struct{}{}, clip.ErrPlanConflict
			}
			return struct{}{}, nil
		}
		if err := checkBrowserProjectExcept(ctx, q, r, job); err != nil {
			return struct{}{}, err
		}
		if err := renewProjectSources(ctx, q, user, r.ProjectID, now); err != nil {
			return struct{}{}, err
		}
		n, err := q.SaveBrowserRenderGrounds(ctx, sqlc.SaveBrowserRenderGroundsParams{GroundsJson: nullable(string(raw)), SampledAt: nullable(stamp(now)), ID: render, UserID: user, JobID: nullable(job), PlanRevision: int64(revision)})
		return struct{}{}, affected(n, err)
	})
	return err
}

func (s *Store) BeginBrowserRender(ctx context.Context, r clip.BrowserRender) error {
	speech, err := encodeBrowserComposition(r)
	if err != nil {
		return err
	}
	_, err = transact(ctx, s, func(q *sqlc.Queries) (struct{}, error) {
		if err := checkBrowserProject(ctx, q, r); err != nil {
			return struct{}{}, err
		}
		p, err := getProject(ctx, q, r.UserID, r.ProjectID)
		if err != nil {
			return struct{}{}, err
		}
		if plan, e := clip.DecodeEditPlan(p.EditPlan); e == nil {
			if err = clip.NarrationReadiness(plan); err != nil {
				return struct{}{}, err
			}
			if !reflect.DeepEqual(clip.RequestedSpeech(plan), r.Speech) {
				return struct{}{}, clip.ErrInvalidMedia
			}
			if err = validateSpeechAssets(ctx, q, r.UserID, r.ProjectID, plan); err != nil {
				return struct{}{}, err
			}
		} else if len(r.Speech) > 0 {
			return struct{}{}, clip.ErrInvalidMedia
		}
		if err := renewProjectSources(ctx, q, r.UserID, r.ProjectID, r.CreatedAt); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, q.BeginBrowserRender(ctx, sqlc.BeginBrowserRenderParams{SpeechJson: speech, ID: r.ID, UserID: r.UserID, ProjectID: r.ProjectID, PlanRevision: int64(r.Revision), Ratio: r.Ratio, DurationMs: int64(r.DurationMS), HasAudio: flag(r.Audio), CreatedAt: stamp(r.CreatedAt)})
	})
	return err
}

func (s *Store) GetBrowserRender(ctx context.Context, user, id string) (clip.BrowserRender, error) {
	r, err := s.read.GetBrowserRender(ctx, sqlc.GetBrowserRenderParams{ID: id, UserID: user})
	if err != nil {
		return clip.BrowserRender{}, dbError(err)
	}
	return browserRender(r)
}

func (s *Store) SaveBrowserRenderVerdict(ctx context.Context, user, id string, verdict clip.RenderVerdict, now time.Time) error {
	_, err := transact(ctx, s, func(q *sqlc.Queries) (struct{}, error) {
		row, err := q.GetBrowserRender(ctx, sqlc.GetBrowserRenderParams{ID: id, UserID: user})
		if err != nil {
			return struct{}{}, err
		}
		r, err := browserRender(row)
		if err != nil {
			return struct{}{}, err
		}
		if err := checkBrowserProject(ctx, q, r); err != nil {
			return struct{}{}, err
		}
		if r.Verdict != nil {
			if !reflect.DeepEqual(*r.Verdict, verdict) {
				return struct{}{}, clip.ErrPlanConflict
			}
			return struct{}{}, nil
		}
		raw, err := json.Marshal(verdict)
		if err != nil {
			return struct{}{}, err
		}
		if err := renewProjectSources(ctx, q, user, r.ProjectID, now); err != nil {
			return struct{}{}, err
		}
		n, err := q.SaveBrowserRenderVerdict(ctx, sqlc.SaveBrowserRenderVerdictParams{ID: id, UserID: user, VerdictJson: nullable(string(raw)), ReportedAt: nullable(stamp(now))})
		return struct{}{}, affected(n, err)
	})
	return err
}
