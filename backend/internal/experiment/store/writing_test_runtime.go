package store

import (
	"context"
	"encoding/json"
	"slices"
	"time"

	"github.com/postpilot/backend/internal/experiment"
	"github.com/postpilot/backend/internal/experiment/store/sqlc"
)

func workForTest(ctx context.Context, q *sqlc.Queries, user, id string) (experiment.TestExecutionWork, error) {
	found, err := loadWritingTest(ctx, q, user, id)
	if err != nil {
		return experiment.TestExecutionWork{}, err
	}
	attempts, err := q.ListWritingTestAttempts(ctx, sqlc.ListWritingTestAttemptsParams{UserID: user, TestID: id})
	if err != nil {
		return experiment.TestExecutionWork{}, err
	}
	if len(attempts) == 0 {
		return experiment.TestExecutionWork{}, experiment.ErrTestStateInvalid
	}
	// Epoch orders attempts independently of wall-clock precision and random ids.
	slices.SortFunc(attempts, func(a, b sqlc.WritingTestAttempt) int {
		if a.Epoch < b.Epoch {
			return -1
		}
		if a.Epoch > b.Epoch {
			return 1
		}
		return 0
	})
	attempt := attempts[len(attempts)-1]
	quote, err := loadTestQuote(ctx, q, user, attempt.QuoteID)
	if err != nil {
		return experiment.TestExecutionWork{}, err
	}
	work := experiment.TestExecutionWork{Fence: experiment.TestExecutionFence{UserID: user, TestID: id, JobID: attempt.JobID, RequestKey: attempt.RequestKey, Revision: uint32(attempt.Epoch), PurgeFence: uint64(attempt.PurgeFence), NonMetered: attempt.NonMetered == 1}, Test: found, Plan: quote.Plan, CandidateIndices: map[string]int{}, CandidateCheckpoints: map[string][]byte{}}
	if err = json.Unmarshal([]byte(attempt.CandidateIds), &work.CandidateIDs); err != nil {
		return experiment.TestExecutionWork{}, err
	}
	for _, candidate := range found.Candidates {
		for index, variant := range quote.Plan.Snapshot.Variants {
			if candidate.Ref == variant.Reference {
				work.CandidateIndices[candidate.ID] = index
				break
			}
		}
	}
	for _, prior := range attempts {
		checkpoints, err := q.ListWritingTestCheckpoints(ctx, sqlc.ListWritingTestCheckpointsParams{UserID: user, TestID: id, AttemptID: prior.ID})
		if err != nil {
			return experiment.TestExecutionWork{}, err
		}
		for _, checkpoint := range checkpoints {
			if checkpoint.SlotID == "shared" {
				work.SharedCheckpoint = slices.Clone(checkpoint.Checkpoint)
			} else {
				work.CandidateCheckpoints[checkpoint.SlotID] = slices.Clone(checkpoint.Checkpoint)
			}
		}
	}
	return work, nil
}
func (s *Store) PreparedTestWork(ctx context.Context, user, id string) (experiment.TestExecutionWork, error) {
	return workForTest(ctx, s.read, user, id)
}
func executionAttempt(ctx context.Context, q *sqlc.Queries, fence experiment.TestExecutionFence) (sqlc.WritingTestAttempt, error) {
	attempt, err := q.GetWritingTestAttemptByRequest(ctx, sqlc.GetWritingTestAttemptByRequestParams{UserID: fence.UserID, RequestKey: fence.RequestKey})
	if err != nil {
		return sqlc.WritingTestAttempt{}, testStoreError(err)
	}
	if attempt.TestID != fence.TestID || attempt.JobID != fence.JobID || attempt.Epoch != int64(fence.Revision) || attempt.PurgeFence != int64(fence.PurgeFence) || (attempt.NonMetered == 1) != fence.NonMetered {
		return sqlc.WritingTestAttempt{}, experiment.ErrTestStateInvalid
	}
	return attempt, nil
}
func liveExecution(ctx context.Context, q *sqlc.Queries, fence experiment.TestExecutionFence) (experiment.WritingTest, sqlc.WritingTestAttempt, error) {
	attempt, err := executionAttempt(ctx, q, fence)
	if err != nil {
		return experiment.WritingTest{}, attempt, err
	}
	found, err := loadWritingTest(ctx, q, fence.UserID, fence.TestID)
	if err != nil {
		return found, attempt, err
	}
	if found.PurgeFence != fence.PurgeFence || found.PurgeFence != 0 || found.JobID != fence.JobID || found.Status == experiment.TestCancelled || !(attempt.Status == "queued" || attempt.Status == "running") {
		return found, attempt, experiment.ErrTestStateInvalid
	}
	attempts, err := q.ListWritingTestAttempts(ctx, sqlc.ListWritingTestAttemptsParams{UserID: fence.UserID, TestID: fence.TestID})
	if err != nil {
		return found, attempt, err
	}
	for _, other := range attempts {
		if other.Epoch > attempt.Epoch {
			return found, attempt, experiment.ErrTestStateInvalid
		}
	}
	return found, attempt, nil
}
func (s *Store) BindTestJob(ctx context.Context, fence experiment.TestExecutionFence, jobID string) (experiment.WritingTest, error) {
	var found experiment.WritingTest
	err := s.withWritingTestTx(ctx, func(q *sqlc.Queries) error {
		if jobID == "" || jobID != fence.JobID {
			return experiment.ErrTestOperation
		}
		attempt, err := executionAttempt(ctx, q, fence)
		if err != nil {
			return err
		}
		found, err = loadWritingTest(ctx, q, fence.UserID, fence.TestID)
		if err != nil {
			return err
		}
		if found.PurgeFence != 0 || found.PurgeFence != fence.PurgeFence || found.Status == experiment.TestCancelled {
			return experiment.ErrTestStateInvalid
		}
		if attempt.Status == "queued" || attempt.Status == "running" || attempt.Status == "done" || attempt.Status == "failed" {
			if found.JobID == jobID {
				return nil
			}
			return experiment.ErrTestStateInvalid
		}
		if attempt.Status != "prepared" || found.Status != experiment.TestQueued || found.Revision != fence.Revision {
			return experiment.ErrTestStateInvalid
		}
		if err = updateAttempt(ctx, q, attempt, "queued", jobID, attempt.ConfirmedCredits, attempt.Settled, "", time.Now()); err != nil {
			return err
		}
		previous := found.Revision
		found.Revision++
		found.JobID = jobID
		found.UpdatedAt = time.Now().UTC()
		return updateWritingTest(ctx, q, &found, previous)
	})
	return found, err
}
func (s *Store) BeginTestExecution(ctx context.Context, fence experiment.TestExecutionFence) (experiment.TestExecutionWork, error) {
	var work experiment.TestExecutionWork
	err := s.withWritingTestTx(ctx, func(q *sqlc.Queries) error {
		found, attempt, err := liveExecution(ctx, q, fence)
		if err != nil {
			return err
		}
		if attempt.Status == "running" {
			return experiment.ErrTestRunning
		}
		if attempt.Status != "queued" || found.Status != experiment.TestQueued {
			return experiment.ErrTestStateInvalid
		}
		if err = updateAttempt(ctx, q, attempt, "running", attempt.JobID, attempt.ConfirmedCredits, attempt.Settled, "", time.Now()); err != nil {
			return err
		}
		previous := found.Revision
		found.Revision++
		found.Status = experiment.TestRunning
		found.UpdatedAt = time.Now().UTC()
		if err = updateWritingTest(ctx, q, &found, previous); err != nil {
			return err
		}
		work, err = workForTest(ctx, q, fence.UserID, fence.TestID)
		return err
	})
	return work, err
}
func (s *Store) SaveTestCheckpoint(ctx context.Context, fence experiment.TestExecutionFence, slot string, checkpoint []byte) error {
	if len(checkpoint) == 0 || len(checkpoint) > 8<<20 {
		return experiment.ErrTestMaterialInvalid
	}
	return s.withWritingTestTx(ctx, func(q *sqlc.Queries) error {
		found, attempt, err := liveExecution(ctx, q, fence)
		if err != nil {
			return err
		}
		if attempt.Status != "running" {
			return experiment.ErrTestStateInvalid
		}
		if slot != "shared" {
			var ids []string
			if err = json.Unmarshal([]byte(attempt.CandidateIds), &ids); err != nil {
				return err
			}
			if !slices.Contains(ids, slot) {
				return experiment.ErrTestEntrant
			}
			for _, candidate := range found.Candidates {
				if candidate.ID == slot && candidate.Status == string(experiment.TestCandidatePending) {
					candidate.Status = string(experiment.TestCandidateRunning)
					if err = updateTestCandidate(ctx, q, candidate); err != nil {
						return err
					}
					break
				}
			}
		}
		return q.UpsertWritingTestCheckpoint(ctx, sqlc.UpsertWritingTestCheckpointParams{UserID: fence.UserID, TestID: fence.TestID, AttemptID: attempt.ID, SlotID: slot, Checkpoint: slices.Clone(checkpoint), UpdatedAt: formatTime(time.Now())})
	})
}
func (s *Store) CompleteTestCandidate(ctx context.Context, fence experiment.TestExecutionFence, id string, output, accounting []byte, failure *experiment.Failure) error {
	if len(accounting) > 1<<20 {
		return experiment.ErrTestMaterialInvalid
	}
	if len(output) > 0 {
		if _, err := experiment.DecodeTestOutput(output); err != nil {
			return experiment.ErrTestMaterialInvalid
		}
	}
	return s.withWritingTestTx(ctx, func(q *sqlc.Queries) error {
		found, attempt, err := liveExecution(ctx, q, fence)
		if err != nil {
			return err
		}
		if attempt.Status != "running" {
			return experiment.ErrTestStateInvalid
		}
		var ids []string
		if err = json.Unmarshal([]byte(attempt.CandidateIds), &ids); err != nil {
			return err
		}
		if !slices.Contains(ids, id) {
			return experiment.ErrTestEntrant
		}
		for _, candidate := range found.Candidates {
			if candidate.ID == id {
				if candidate.Status == string(experiment.TestCandidateSucceeded) {
					if slices.Equal(candidate.Output, output) && slices.Equal(candidate.Accounting, accounting) {
						return nil
					}
					return experiment.ErrTestStateInvalid
				}
				if candidate.Status != string(experiment.TestCandidatePending) && candidate.Status != string(experiment.TestCandidateRunning) {
					return experiment.ErrTestStateInvalid
				}
				candidate.Output = slices.Clone(output)
				candidate.Accounting = slices.Clone(accounting)
				candidate.Failure = failure
				candidate.Status = string(experiment.TestCandidateSucceeded)
				if failure != nil || len(output) == 0 {
					candidate.Status = string(experiment.TestCandidateFailed)
					candidate.Output = nil
					if failure == nil {
						candidate.Failure = &experiment.Failure{Reason: experiment.FailureReasonUnknown}
					}
				}
				if len(accounting) > 0 {
					var usage experiment.Usage
					if err = json.Unmarshal(accounting, &usage); err != nil {
						return experiment.ErrTestMaterialInvalid
					}
				}
				return updateTestCandidate(ctx, q, candidate)
			}
		}
		return experiment.ErrTestEntrant
	})
}
func (s *Store) FinishTestExecution(ctx context.Context, fence experiment.TestExecutionFence, credits int, failure *experiment.Failure) (experiment.WritingTest, error) {
	var found experiment.WritingTest
	if credits < 0 {
		return found, experiment.ErrTestMaterialInvalid
	}
	err := s.withWritingTestTx(ctx, func(q *sqlc.Queries) error {
		var attempt sqlc.WritingTestAttempt
		var err error
		found, attempt, err = liveExecution(ctx, q, fence)
		if err != nil {
			return err
		}
		if attempt.Status != "running" {
			return experiment.ErrTestStateInvalid
		}
		successes := 0
		for i := range found.Candidates {
			candidate := &found.Candidates[i]
			if candidate.Status == string(experiment.TestCandidateSucceeded) {
				successes++
			} else if candidate.Status == string(experiment.TestCandidatePending) || candidate.Status == string(experiment.TestCandidateRunning) {
				candidate.Status = string(experiment.TestCandidateFailed)
				candidate.Failure = failure
				if failure == nil {
					candidate.Failure = &experiment.Failure{Reason: experiment.FailureReasonUnknown}
				}
				if err = updateTestCandidate(ctx, q, *candidate); err != nil {
					return err
				}
			}
		}
		status := "failed"
		found.Status = experiment.TestFailed
		if successes > 0 {
			found.Status = experiment.TestPartial
		}
		if successes == found.Count {
			if err = experiment.OpenWritingTestMatches(&found); err != nil {
				return err
			}
			if err = insertTestMatches(ctx, q, found, 0); err != nil {
				return err
			}
			status = "done"
			found.Failure = nil
		} else {
			found.Failure = failure
		}
		if err = updateAttempt(ctx, q, attempt, status, attempt.JobID, attempt.ConfirmedCredits, attempt.Settled, "", time.Now()); err != nil {
			return err
		}
		previous := found.Revision
		found.Revision++
		found.UpdatedAt = time.Now().UTC()
		if found.Status == experiment.TestPartial || found.Status == experiment.TestFailed {
			if err = setTestExpiry(ctx, q, &found); err != nil {
				return err
			}
		}
		return updateWritingTest(ctx, q, &found, previous)
	})
	return found, err
}
func (s *Store) ConfirmTestSettlement(ctx context.Context, fence experiment.TestExecutionFence, credits int) error {
	if fence.NonMetered && credits != 0 {
		return experiment.ErrTestStateInvalid
	}
	if credits < 0 {
		return experiment.ErrTestMaterialInvalid
	}
	return s.withWritingTestTx(ctx, func(q *sqlc.Queries) error {
		attempt, err := executionAttempt(ctx, q, fence)
		if err != nil {
			return err
		}
		if attempt.Settled == 1 {
			if attempt.ConfirmedCredits != int64(credits) {
				return experiment.ErrTestStateInvalid
			}
			return nil
		}
		if attempt.Status == "prepared" || attempt.Status == "queued" || attempt.Status == "running" {
			return experiment.ErrTestRunning
		}
		found, err := loadWritingTest(ctx, q, fence.UserID, fence.TestID)
		if err != nil {
			return err
		}
		if err = updateAttempt(ctx, q, attempt, attempt.Status, attempt.JobID, int64(credits), 1, attempt.FailureReason, time.Now()); err != nil {
			return err
		}
		previous := found.Revision
		found.Revision++
		found.ConfirmedCredits += credits
		if found.JobID == fence.JobID {
			found.ReservedCredits = 0
		}
		found.UpdatedAt = time.Now().UTC()
		return updateWritingTest(ctx, q, &found, previous)
	})
}
func (s *Store) RejectTestAdmission(ctx context.Context, fence experiment.TestExecutionFence, failure *experiment.Failure) error {
	return s.withWritingTestTx(ctx, func(q *sqlc.Queries) error {
		attempt, err := executionAttempt(ctx, q, fence)
		if err != nil {
			return err
		}
		if attempt.Status != "prepared" {
			return experiment.ErrTestStateInvalid
		}
		found, err := loadWritingTest(ctx, q, fence.UserID, fence.TestID)
		if err != nil {
			return err
		}
		if found.PurgeFence != 0 {
			return experiment.ErrTestStateInvalid
		}
		if err = updateAttempt(ctx, q, attempt, "failed", attempt.JobID, 0, 1, "", time.Now()); err != nil {
			return err
		}
		for i := range found.Candidates {
			candidate := &found.Candidates[i]
			if candidate.Status == string(experiment.TestCandidatePending) {
				candidate.Status = string(experiment.TestCandidateFailed)
				candidate.Failure = failure
				if err = updateTestCandidate(ctx, q, *candidate); err != nil {
					return err
				}
			}
		}
		previous := found.Revision
		found.Revision++
		found.Status = experiment.TestFailed
		found.Failure = failure
		found.ReservedCredits = 0
		found.UpdatedAt = time.Now().UTC()
		if err = setTestExpiry(ctx, q, &found); err != nil {
			return err
		}
		return updateWritingTest(ctx, q, &found, previous)
	})
}
func (s *Store) RecoverInterruptedTests(ctx context.Context) error {
	return s.withWritingTestTx(ctx, func(q *sqlc.Queries) error {
		attempts, err := q.ListInterruptedWritingTestAttempts(ctx)
		if err != nil {
			return err
		}
		for _, attempt := range attempts {
			found, err := loadWritingTest(ctx, q, attempt.UserID, attempt.TestID)
			if err != nil {
				return err
			}
			if found.PurgeFence != 0 || found.Status == experiment.TestCancelled {
				if err = updateAttempt(ctx, q, attempt, "cancelled", attempt.JobID, attempt.ConfirmedCredits, attempt.Settled, "", time.Now()); err != nil {
					return err
				}
				continue
			}
			successes := 0
			for i := range found.Candidates {
				candidate := &found.Candidates[i]
				if candidate.Status == string(experiment.TestCandidateSucceeded) {
					successes++
				} else if candidate.Status == string(experiment.TestCandidatePending) || candidate.Status == string(experiment.TestCandidateRunning) {
					candidate.Status = string(experiment.TestCandidateFailed)
					candidate.Failure = &experiment.Failure{Reason: experiment.FailureReasonInterrupted}
					if err = updateTestCandidate(ctx, q, *candidate); err != nil {
						return err
					}
				}
			}
			previous := found.Revision
			found.Revision++
			found.Status = experiment.TestFailed
			if successes > 0 {
				found.Status = experiment.TestPartial
			}
			found.Failure = &experiment.Failure{Reason: experiment.FailureReasonInterrupted}
			found.UpdatedAt = time.Now().UTC()
			attemptStatus := "uncertain"
			failureReason := experiment.FailureReasonInterrupted
			if successes == found.Count {
				if err = experiment.OpenWritingTestMatches(&found); err != nil {
					return err
				}
				if err = insertTestMatches(ctx, q, found, 0); err != nil {
					return err
				}
				found.Failure = nil
				attemptStatus = "done"
				failureReason = ""
			} else if err = setTestExpiry(ctx, q, &found); err != nil {
				return err
			}
			if err = updateAttempt(ctx, q, attempt, attemptStatus, attempt.JobID, attempt.ConfirmedCredits, attempt.Settled, failureReason, found.UpdatedAt); err != nil {
				return err
			}
			if err = updateWritingTest(ctx, q, &found, previous); err != nil {
				return err
			}
		}
		return nil
	})
}
func purgeTest(ctx context.Context, q *sqlc.Queries, found experiment.WritingTest) error {
	if found.PurgeFence != 0 {
		return nil
	}
	previous := found.Revision
	found.Revision++
	found.PurgeFence++
	if found.Status == experiment.TestQueued || found.Status == experiment.TestRunning || found.Status == experiment.TestReview {
		found.Status = experiment.TestCancelled
	}
	found.CommonSnapshot = nil
	found.SourcePostSlug = ""
	found.Input = experiment.TestInput{TargetLanguage: found.Input.TargetLanguage, Fictional: found.Input.Fictional}
	found.UpdatedAt = time.Now().UTC()
	if err := cancelOpenAttempts(ctx, q, found); err != nil {
		return err
	}
	if err := q.PurgeWritingTestCandidate(ctx, sqlc.PurgeWritingTestCandidateParams{UserID: found.UserID, TestID: found.ID}); err != nil {
		return err
	}
	if err := q.PurgeWritingTestCheckpoints(ctx, sqlc.PurgeWritingTestCheckpointsParams{UserID: found.UserID, TestID: found.ID}); err != nil {
		return err
	}
	if err := q.PurgeWritingTestQuoteForTest(ctx, sqlc.PurgeWritingTestQuoteForTestParams{UserID: found.UserID, TestID: nullString(found.ID), ConsumedTestID: found.ID}); err != nil {
		return err
	}
	return updateWritingTest(ctx, q, &found, previous)
}
func (s *Store) PurgeWritingTestPost(ctx context.Context, user, slug string) error {
	return s.withWritingTestTx(ctx, func(q *sqlc.Queries) error {
		rows, err := q.ListWritingTestsForSource(ctx, sqlc.ListWritingTestsForSourceParams{UserID: user, SourcePostSlug: nullString(slug)})
		if err != nil {
			return err
		}
		for _, row := range rows {
			found, err := loadWritingTest(ctx, q, user, row.ID)
			if err != nil {
				return err
			}
			if err = purgeTest(ctx, q, found); err != nil {
				return err
			}
		}
		return q.PurgeWritingTestQuotesForSource(ctx, sqlc.PurgeWritingTestQuotesForSourceParams{UserID: user, SourcePostSlug: nullString(slug)})
	})
}
func (s *Store) PurgeExpiredWritingTests(ctx context.Context, before time.Time) (int64, error) {
	var count int64
	err := s.withWritingTestTx(ctx, func(q *sqlc.Queries) error {
		rows, err := q.ListWritingTestsForPurge(ctx, nullString(formatTime(before)))
		if err != nil {
			return err
		}
		for _, row := range rows {
			found, err := loadWritingTest(ctx, q, row.UserID, row.ID)
			if err != nil {
				return err
			}
			if err = purgeTest(ctx, q, found); err != nil {
				return err
			}
			count++
		}
		return q.PurgeExpiredWritingTestQuotes(ctx, formatTime(before))
	})
	return count, err
}

var _ experiment.WritingTestStorage = (*Store)(nil)
var _ experiment.WritingTestRuntimeStore = (*Store)(nil)

func (s *Store) PendingTestSettlements(ctx context.Context) ([]experiment.TestExecutionFence, error) {
	attempts, err := s.read.ListUnsettledWritingTestAttempts(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]experiment.TestExecutionFence, 0, len(attempts))
	for _, attempt := range attempts {
		result = append(result, experiment.TestExecutionFence{UserID: attempt.UserID, TestID: attempt.TestID, JobID: attempt.JobID, RequestKey: attempt.RequestKey, Revision: uint32(attempt.Epoch), PurgeFence: uint64(attempt.PurgeFence), NonMetered: attempt.NonMetered == 1})
	}
	return result, nil
}
func (s *Store) EndTestExecution(ctx context.Context, fence experiment.TestExecutionFence, outcome experiment.TestExecutionOutcome, failure *experiment.Failure) error {
	if outcome != experiment.TestExecutionSucceeded && outcome != experiment.TestExecutionFailed && outcome != experiment.TestExecutionCancelled {
		return experiment.ErrTestStateInvalid
	}
	return s.withWritingTestTx(ctx, func(q *sqlc.Queries) error {
		attempt, err := executionAttempt(ctx, q, fence)
		if err != nil {
			return err
		}
		if attempt.Status != "prepared" && attempt.Status != "queued" && attempt.Status != "running" && outcome != experiment.TestExecutionCancelled {
			return nil
		}
		found, err := loadWritingTest(ctx, q, fence.UserID, fence.TestID)
		if err != nil {
			return err
		}
		attemptStatus := "failed"
		if outcome == experiment.TestExecutionCancelled {
			attemptStatus = "cancelled"
		}
		reason := ""
		if failure != nil {
			reason = failure.Reason
		}
		attempts, err := q.ListWritingTestAttempts(ctx, sqlc.ListWritingTestAttemptsParams{UserID: fence.UserID, TestID: fence.TestID})
		if err != nil {
			return err
		}
		current := true
		for _, other := range attempts {
			if other.Epoch > attempt.Epoch {
				current = false
			}
		}
		if !current || found.PurgeFence != 0 || found.PurgeFence != fence.PurgeFence || found.Status == experiment.TestCancelled || found.Status == experiment.TestCompleted {
			if attempt.Status != "prepared" && attempt.Status != "queued" && attempt.Status != "running" {
				return nil
			}
			return updateAttempt(ctx, q, attempt, attemptStatus, attempt.JobID, attempt.ConfirmedCredits, attempt.Settled, reason, time.Now())
		}
		var ids []string
		if err = json.Unmarshal([]byte(attempt.CandidateIds), &ids); err != nil {
			return err
		}
		successes := 0
		for i := range found.Candidates {
			candidate := &found.Candidates[i]
			if candidate.Status == string(experiment.TestCandidateSucceeded) {
				successes++
				continue
			}
			if !slices.Contains(ids, candidate.ID) {
				continue
			}
			if candidate.Status == string(experiment.TestCandidatePending) || candidate.Status == string(experiment.TestCandidateRunning) {
				candidate.Status = string(experiment.TestCandidateFailed)
				if outcome == experiment.TestExecutionCancelled {
					candidate.Status = string(experiment.TestCandidateCancelled)
				}
				candidate.Output = nil
				candidate.Failure = failure
				if failure == nil {
					candidate.Failure = &experiment.Failure{Reason: experiment.FailureReasonUnknown}
				}
				if err = updateTestCandidate(ctx, q, *candidate); err != nil {
					return err
				}
			}
		}
		previous := found.Revision
		found.Revision++
		found.UpdatedAt = time.Now().UTC()
		found.JobID = fence.JobID
		found.Failure = failure
		found.Status = experiment.TestFailed
		if successes > 0 {
			found.Status = experiment.TestPartial
		}
		if outcome == experiment.TestExecutionCancelled {
			found.Status = experiment.TestCancelled
		} else if successes == found.Count {
			oldMatches := len(found.Matches)
			if err = experiment.OpenWritingTestMatches(&found); err != nil {
				return err
			}
			if err = insertTestMatches(ctx, q, found, oldMatches); err != nil {
				return err
			}
			found.Failure = nil
			attemptStatus = "done"
			reason = ""
		}
		if found.Status != experiment.TestReview {
			if err = setTestExpiry(ctx, q, &found); err != nil {
				return err
			}
		}
		if err = updateAttempt(ctx, q, attempt, attemptStatus, attempt.JobID, attempt.ConfirmedCredits, attempt.Settled, reason, found.UpdatedAt); err != nil {
			return err
		}
		return updateWritingTest(ctx, q, &found, previous)
	})
}

var _ experiment.WritingTestExecutionTerminalStore = (*Store)(nil)
var _ experiment.WritingTestSettlementStore = (*Store)(nil)
