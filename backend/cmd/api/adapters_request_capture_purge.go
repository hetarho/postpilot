package main

import (
	"context"
	"database/sql"

	"github.com/postpilot/backend/internal/job"
	jobstore "github.com/postpilot/backend/internal/job/store"
	"github.com/postpilot/backend/internal/post"
)

// The job store owns its SQL; the post purge binds these reads to its writer
// transaction so an issued first callback cannot miss the durable erasure fence.
type postRequestCaptureJobs struct{}

func (postRequestCaptureJobs) ActivePostRequestCaptureJobs(ctx context.Context, tx *sql.Tx, userID, slug string) ([]string, error) {
	reader := jobstore.NewTx(tx, jobKinds())
	var ids []string
	for _, kind := range ordinaryPostWriteKinds() {
		run, err := reader.ActiveFor(ctx, job.Subject{Dimension: post.JobSubject, ID: slug}, job.Filter{UserID: userID, Kind: kind})
		if err != nil {
			return nil, err
		}
		if run != nil {
			ids = append(ids, run.ID)
		}
	}
	return ids, nil
}
