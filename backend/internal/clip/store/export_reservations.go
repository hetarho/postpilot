package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/store/sqlc"
)

// exportWrite runs all counter and reservation changes on the same writer. A
// tx-bound Store joins its caller's transaction (the result/job terminal saga).
func (s *Store) exportWrite(ctx context.Context, fn func(sqlc.DBTX) error) error {
	if s.writer == nil {
		return fn(s.raw)
	}
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ReserveExport(ctx context.Context, user, project string, revision int, id string, at time.Time) error {
	if user == "" || project == "" || revision <= 0 || id == "" {
		return clip.ErrInvalid
	}
	return s.exportWrite(ctx, func(db sqlc.DBTX) error {
		// Acquire SQLite's single writer position before the first read. A
		// read-then-upgrade transaction can fail with BUSY_SNAPSHOT under
		// concurrent admission even when capacity remains.
		if _, err := db.ExecContext(ctx, `UPDATE server_export_windows SET reserved=reserved
			WHERE user_id=? AND window_start<=? AND window_end>?`, user, stamp(at), stamp(at)); err != nil {
			return err
		}
		var owner, existingProject, state string
		var existingRevision int
		err := db.QueryRowContext(ctx, `SELECT user_id,project_id,plan_revision,state FROM server_export_reservations WHERE id=?`, id).Scan(&owner, &existingProject, &existingRevision, &state)
		if err == nil {
			if owner == user && existingProject == project && existingRevision == revision && state == "reserved" {
				return nil
			}
			return clip.ErrInvalid
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		window, ok, err := (&Store{read: sqlc.New(db), write: sqlc.New(db)}).CurrentExportWindow(ctx, user, at)
		if err != nil {
			return err
		}
		if !ok {
			return &clip.ExportAllowanceError{}
		}
		if window.Used+window.Reserved >= window.Allowance {
			return &clip.ExportAllowanceError{Window: window}
		}
		updated, err := db.ExecContext(ctx, `UPDATE server_export_windows SET reserved=reserved+1
			WHERE user_id=? AND coverage_id=? AND window_start=? AND window_end>?
			AND used+reserved<allowance AND refund_request_id IS NULL`, user, window.CoverageID, stamp(window.Start), stamp(at))
		if err != nil {
			return err
		}
		n, err := updated.RowsAffected()
		if err != nil {
			return err
		}
		if n != 1 {
			return &clip.ExportAllowanceError{Window: window}
		}
		_, err = db.ExecContext(ctx, `INSERT INTO server_export_reservations
			(id,user_id,project_id,plan_revision,coverage_id,window_start,state,created_at)
			VALUES (?,?,?,?,?,?,'reserved',?)`, id, user, project, revision, window.CoverageID, stamp(window.Start), stamp(at))
		return err
	})
}

func (s *Store) BindExport(ctx context.Context, reservationID, jobID string) error {
	if reservationID == "" || jobID == "" {
		return clip.ErrInvalid
	}
	return s.exportWrite(ctx, func(db sqlc.DBTX) error {
		result, err := db.ExecContext(ctx, `UPDATE server_export_reservations SET job_id=?
			WHERE id=? AND state='reserved' AND (job_id IS NULL OR job_id=?)`, jobID, reservationID, jobID)
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n != 1 {
			return clip.ErrInvalid
		}
		return nil
	})
}

func (s *Store) CommitExport(ctx context.Context, candidate clip.AttemptResult) error {
	if candidate.Result.RenderKind() != clip.RenderServer || candidate.Result.Key == "" {
		return nil
	}
	return s.exportWrite(ctx, func(db sqlc.DBTX) error {
		var id, owner, project, coverage, start, state string
		var revision int
		err := db.QueryRowContext(ctx, `SELECT id,user_id,project_id,plan_revision,coverage_id,window_start,state
			FROM server_export_reservations WHERE job_id=?`, candidate.JobID).
			Scan(&id, &owner, &project, &revision, &coverage, &start, &state)
		if errors.Is(err, sql.ErrNoRows) { // operator and pre-quota jobs have no reservation
			return nil
		}
		if err != nil {
			return err
		}
		if owner != candidate.UserID || project != candidate.ProjectID || revision != candidate.ExpectedRevision {
			return clip.ErrInvalid
		}
		if state == "committed" {
			return nil
		}
		if state != "reserved" {
			return clip.ErrInvalid
		}
		updated, err := db.ExecContext(ctx, `UPDATE server_export_windows SET reserved=reserved-1,used=used+1
			WHERE user_id=? AND coverage_id=? AND window_start=? AND reserved>0`, owner, coverage, start)
		if err != nil {
			return err
		}
		n, err := updated.RowsAffected()
		if err != nil {
			return err
		}
		if n != 1 {
			return errors.New("commit export: missing origin reservation")
		}
		_, err = db.ExecContext(ctx, `UPDATE server_export_reservations SET state='committed' WHERE id=? AND state='reserved'`, id)
		return err
	})
}

func (s *Store) ReleaseExport(ctx context.Context, idOrJob string) error {
	if idOrJob == "" {
		return nil
	}
	return s.exportWrite(ctx, func(db sqlc.DBTX) error {
		if _, err := db.ExecContext(ctx, `UPDATE server_export_reservations SET state=state WHERE id=? OR job_id=?`, idOrJob, idOrJob); err != nil {
			return err
		}
		var id, owner, coverage, start, state string
		var jobID sql.NullString
		err := db.QueryRowContext(ctx, `SELECT id,user_id,coverage_id,window_start,state,job_id
			FROM server_export_reservations WHERE id=? OR job_id=?`, idOrJob, idOrJob).
			Scan(&id, &owner, &coverage, &start, &state, &jobID)
		if errors.Is(err, sql.ErrNoRows) || state == "released" || state == "committed" {
			return nil
		}
		if err != nil {
			return err
		}
		if jobID.Valid {
			var status string
			var ready int
			err := db.QueryRowContext(ctx, `SELECT status,dispatch_ready FROM generation_jobs WHERE id=?`, jobID.String).Scan(&status, &ready)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			if err == nil && (status == "running" || status == "queued" && ready == 1) {
				return nil
			}
		}
		result, err := db.ExecContext(ctx, `UPDATE server_export_windows SET reserved=reserved-1
			WHERE user_id=? AND coverage_id=? AND window_start=? AND reserved>0`, owner, coverage, start)
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n != 1 {
			return errors.New("release export: missing origin reservation")
		}
		_, err = db.ExecContext(ctx, `UPDATE server_export_reservations SET state='released' WHERE id=? AND state='reserved'`, id)
		return err
	})
}

// RecoverExports releases an unbound pre-queue hold or a hold whose job ended
// without committing a result. Running and queued jobs keep their origin slot.
func (s *Store) RecoverExports(ctx context.Context) error {
	rows, err := s.raw.QueryContext(ctx, `SELECT r.id FROM server_export_reservations r
		LEFT JOIN generation_jobs j ON j.id=r.job_id
		WHERE r.state='reserved' AND ((r.job_id IS NULL AND r.created_at<?)
		OR j.id IS NULL AND r.created_at<? OR j.status IN ('done','failed','cancelled'))`,
		stamp(time.Now().Add(-2*time.Minute)), stamp(time.Now().Add(-2*time.Minute)))
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := s.ReleaseExport(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

var _ clip.ExportReservations = (*Store)(nil)
