package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/postpilot/backend/internal/publishing"
)

func (s *Store) RetirementSnapshot(ctx context.Context) (publishing.RetirementSnapshot, error) {
	var snapshot publishing.RetirementSnapshot
	if err := s.readDB().QueryRowContext(ctx, `
		SELECT CAST(tstamp AS TEXT)
		FROM goose_db_version
		WHERE version_id=? AND is_applied=1
		ORDER BY id DESC LIMIT 1`, publishing.RetirementMigrationVersion).Scan(&snapshot.CutoffAt); err != nil {
		if err == sql.ErrNoRows {
			return snapshot, fmt.Errorf("retirement migration %d is not applied", publishing.RetirementMigrationVersion)
		}
		return snapshot, err
	}
	rows, err := s.readDB().QueryContext(ctx, `
		SELECT id,user_id,label,platform_account_id,platform_account_label,COALESCE(revoked_at,'')
		FROM publishing_agents ORDER BY user_id,id`)
	if err != nil {
		return snapshot, err
	}
	for rows.Next() {
		var agent publishing.RetirementAgent
		if err := rows.Scan(&agent.ID, &agent.AccountID, &agent.Label, &agent.PlatformAccountID, &agent.PlatformAccountLabel, &agent.RevokedAt); err != nil {
			rows.Close()
			return snapshot, err
		}
		snapshot.Agents = append(snapshot.Agents, agent)
	}
	if err := rows.Close(); err != nil {
		return snapshot, err
	}
	rows, err = s.readDB().QueryContext(ctx, `
		SELECT id,user_id,post_slug,post_created_at,status,stage,
		       COALESCE(committed_at,''),COALESCE(published_at,''),
		       COALESCE(error_reason,''),COALESCE(platform_post_url,'')
		FROM publish_jobs ORDER BY user_id,created_at,id`)
	if err != nil {
		return snapshot, err
	}
	for rows.Next() {
		var job publishing.RetirementJob
		if err := rows.Scan(&job.ID, &job.AccountID, &job.PostSlug, &job.PostCreatedAt, &job.Status, &job.Stage, &job.CommittedAt, &job.PublishedAt, &job.FailureReason, &job.PlatformPostURL); err != nil {
			rows.Close()
			return snapshot, err
		}
		snapshot.Jobs = append(snapshot.Jobs, job)
	}
	if err := rows.Close(); err != nil {
		return snapshot, err
	}
	for query, target := range map[string]*int64{
		`SELECT COUNT(*) FROM publishing_pairings`: &snapshot.Pairings,
		`SELECT COUNT(*) FROM publish_job_ids`:     &snapshot.Reservations,
		`SELECT COUNT(*) FROM publish_assets`:      &snapshot.Assets,
	} {
		if err := s.readDB().QueryRowContext(ctx, query).Scan(target); err != nil {
			return snapshot, err
		}
	}
	return snapshot, nil
}

func (s *Store) readDB() *sql.DB {
	return s.reader
}
