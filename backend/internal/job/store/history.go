package store

import (
	"context"
	"encoding/json"

	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/job/store/sqlc"
)

func (s *Store) LatestForSubjects(ctx context.Context, userID, dimension string, ids, kinds []string) (map[string]job.Job, error) {
	switch dimension {
	case dimensionPost, dimensionVoice, dimensionProject, dimensionExperiment:
	default:
		return nil, job.ErrInvalidTarget
	}
	if userID == "" {
		return nil, job.ErrInvalidTarget
	}
	out := map[string]job.Job{}
	if len(ids) == 0 {
		return out, nil
	}
	for _, id := range ids {
		if id == "" {
			return nil, job.ErrInvalidTarget
		}
	}
	rawIDs, err := json.Marshal(ids)
	if err != nil {
		return nil, err
	}
	if kinds == nil {
		kinds = []string{}
	}
	rawKinds, err := json.Marshal(kinds)
	if err != nil {
		return nil, err
	}
	var rows []sqlc.GenerationJob
	switch dimension {
	case dimensionPost:
		rows, err = s.read.LatestPostJobs(ctx, sqlc.LatestPostJobsParams{UserID: userID, Kinds: string(rawKinds), SubjectIds: string(rawIDs)})
	case dimensionVoice:
		rows, err = s.read.LatestVoiceJobs(ctx, sqlc.LatestVoiceJobsParams{UserID: userID, Kinds: string(rawKinds), SubjectIds: string(rawIDs)})
	case dimensionProject:
		rows, err = s.read.LatestProjectJobs(ctx, sqlc.LatestProjectJobsParams{UserID: userID, Kinds: string(rawKinds), SubjectIds: string(rawIDs)})
	case dimensionExperiment:
		rows, err = s.read.LatestExperimentJobs(ctx, sqlc.LatestExperimentJobsParams{UserID: userID, Kinds: string(rawKinds), SubjectIds: string(rawIDs)})
	}

	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		found, err := toJob(row)
		if err != nil {
			return nil, err
		}
		out[found.Subject(dimension)] = found
	}
	return out, nil
}
