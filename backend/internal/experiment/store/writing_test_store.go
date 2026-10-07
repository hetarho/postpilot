package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/postpilot/backend/internal/experiment"
	"github.com/postpilot/backend/internal/experiment/store/sqlc"
)

type writingTestContext struct {
	Input            experiment.TestInput
	RetentionSeconds int64
}
type writingTestIdentity struct {
	SnapshotIndex int
	Ref           experiment.TestEntrantRef
	Label         string
	Synthetic     bool
}

func encodeTest(value any) ([]byte, error) { return json.Marshal(value) }
func attemptID(user, key string) string {
	sum := sha256.Sum256([]byte("writing-test-attempt\x00" + user + "\x00" + key))
	return hex.EncodeToString(sum[:])
}
func testStoreError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return experiment.ErrTestNotFound
	}
	return err
}
func (s *Store) withWritingTestTx(ctx context.Context, fn func(*sqlc.Queries) error) error {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = fn(sqlc.New(tx)); err != nil {
		return err
	}
	return tx.Commit()
}
func loadWritingTest(ctx context.Context, q *sqlc.Queries, user, id string) (experiment.WritingTest, error) {
	row, err := q.GetWritingTest(ctx, sqlc.GetWritingTestParams{UserID: user, ID: id})
	if err != nil {
		return experiment.WritingTest{}, testStoreError(err)
	}
	created, err := time.Parse(time.RFC3339Nano, row.CreatedAt)
	if err != nil {
		return experiment.WritingTest{}, err
	}
	updated, err := time.Parse(time.RFC3339Nano, row.UpdatedAt)
	if err != nil {
		return experiment.WritingTest{}, err
	}
	var info writingTestContext
	if err = json.Unmarshal([]byte(row.Context), &info); err != nil {
		return experiment.WritingTest{}, err
	}
	found := experiment.WritingTest{ID: row.ID, UserID: row.UserID, Kind: row.Kind, Factor: experiment.TestFactor(row.Factor), ModelStage: experiment.Stage(row.ModelStage), Count: int(row.Count), Status: experiment.TestStatus(row.Status), Revision: uint32(row.Revision), SourcePostSlug: row.SourcePostSlug.String, Input: info.Input, CommonSnapshot: slices.Clone(row.CommonSnapshot), CommonHash: row.CommonHash, PromptVersion: row.PromptVersion, PurgeFence: uint64(row.PurgeFence), JobID: row.JobID, WinnerID: row.WinnerCandidateID.String, ConfirmedCredits: int(row.ConfirmedCredits), ReservedCredits: int(row.ReservedCredits), CreatedAt: created, UpdatedAt: updated, ContentExpiresAt: parseOptional(row.ContentExpiresAt)}
	if row.FailureReason != "" {
		found.Failure = &experiment.Failure{Reason: row.FailureReason}
	}
	candidates, err := q.ListWritingTestCandidates(ctx, sqlc.ListWritingTestCandidatesParams{UserID: user, TestID: id})
	if err != nil {
		return experiment.WritingTest{}, err
	}
	for _, row := range candidates {
		var identity writingTestIdentity
		if err = json.Unmarshal([]byte(row.SourceID), &identity); err != nil {
			return experiment.WritingTest{}, err
		}
		candidate := experiment.TestCandidate{ID: row.ID, UserID: row.UserID, TestID: row.TestID, SeedPosition: int(row.SeedPosition), SnapshotIndex: identity.SnapshotIndex, Ref: identity.Ref, SourceRevision: row.SourceRevision, SemanticKey: row.SemanticKey, FrozenVariant: slices.Clone(row.FrozenVariant), Output: slices.Clone(row.Output), Accounting: slices.Clone(row.Accounting), Status: row.Status, Identity: &experiment.TestCandidateIdentity{Label: identity.Label, Ref: identity.Ref, Synthetic: identity.Synthetic}}
		if row.FailureReason != "" {
			candidate.Failure = &experiment.Failure{Reason: row.FailureReason}
		}
		if len(row.Accounting) > 0 {
			var usage experiment.Usage
			if err = json.Unmarshal(row.Accounting, &usage); err != nil {
				return experiment.WritingTest{}, err
			}
			candidate.Usage = &usage
		}
		found.Candidates = append(found.Candidates, candidate)
	}
	matches, err := q.ListWritingTestMatches(ctx, sqlc.ListWritingTestMatchesParams{UserID: user, TestID: id})
	if err != nil {
		return experiment.WritingTest{}, err
	}
	for _, row := range matches {
		found.Matches = append(found.Matches, experiment.TestMatch{ID: row.ID, Round: int(row.Round), Index: int(row.MatchIndex), LeftID: row.LeftCandidateID, RightID: row.RightCandidateID, WinnerID: row.WinnerCandidateID.String, DecisionKey: row.DecisionKey.String})
	}
	publications, err := q.ListWritingTestPublications(ctx, sqlc.ListWritingTestPublicationsParams{UserID: user, TestID: id})
	if err != nil {
		return experiment.WritingTest{}, err
	}
	for _, row := range publications {
		found.Publications = append(found.Publications, experiment.TestPublication{ID: row.ID, UserID: row.UserID, TestID: row.TestID, WinnerID: row.WinnerCandidateID, Action: row.Action, RequestKey: row.RequestKey, TargetID: row.TargetID, Status: row.Status, Fingerprint: row.Fingerprint})
	}
	return found, nil
}
func updateWritingTest(ctx context.Context, q *sqlc.Queries, found *experiment.WritingTest, expected uint32) error {
	row, err := q.GetWritingTest(ctx, sqlc.GetWritingTestParams{UserID: found.UserID, ID: found.ID})
	if err != nil {
		return testStoreError(err)
	}
	var info writingTestContext
	if err = json.Unmarshal([]byte(row.Context), &info); err != nil {
		return err
	}
	info.Input = found.Input
	encoded, err := encodeTest(info)
	if err != nil {
		return err
	}
	failure := ""
	if found.Failure != nil {
		failure = found.Failure.Reason
	}
	changed, err := q.UpdateWritingTest(ctx, sqlc.UpdateWritingTestParams{Status: string(found.Status), Revision: int64(found.Revision), Context: string(encoded), PurgeFence: int64(found.PurgeFence), JobID: found.JobID, WinnerCandidateID: nullString(found.WinnerID), ConfirmedCredits: int64(found.ConfirmedCredits), ReservedCredits: int64(found.ReservedCredits), FailureReason: failure, UpdatedAt: formatTime(found.UpdatedAt), ContentExpiresAt: nullTime(found.ContentExpiresAt), CommonSnapshot: found.CommonSnapshot, SourcePostSlug: nullString(found.SourcePostSlug), UserID: found.UserID, ID: found.ID, Revision_2: int64(expected)})
	if err != nil {
		return err
	}
	if changed != 1 {
		return experiment.ErrTestRevisionConflict
	}
	return nil
}
func insertTestMatches(ctx context.Context, q *sqlc.Queries, found experiment.WritingTest, start int) error {
	for _, match := range found.Matches[start:] {
		if err := q.InsertWritingTestMatch(ctx, sqlc.InsertWritingTestMatchParams{ID: match.ID, UserID: found.UserID, TestID: found.ID, Round: int64(match.Round), MatchIndex: int64(match.Index), LeftCandidateID: match.LeftID, RightCandidateID: match.RightID}); err != nil {
			return err
		}
	}
	return nil
}
func (s *Store) PutTestQuote(ctx context.Context, quote experiment.TestQuote) error {
	encoded, err := encodeTest(quote)
	if err != nil {
		return err
	}
	plan, err := encodeTest(quote.Plan)
	if err != nil {
		return err
	}
	return s.write.InsertWritingTestQuote(ctx, sqlc.InsertWritingTestQuoteParams{ID: quote.Key, UserID: quote.UserID, TestID: nullString(quote.TestID), SourcePostSlug: nullString(quote.Start.Input.SourcePostSlug), Fingerprint: quote.Fingerprint, Request: string(encoded), Plan: plan, EstimatedCredits: int64(quote.Credits), Free: boolValue(quote.Free), CreatedAt: formatTime(quote.CreatedAt), ExpiresAt: formatTime(quote.ExpiresAt)})
}
func loadTestQuote(ctx context.Context, q *sqlc.Queries, user, id string) (experiment.TestQuote, error) {
	row, err := q.GetWritingTestQuote(ctx, sqlc.GetWritingTestQuoteParams{UserID: user, ID: id})
	if err != nil {
		return experiment.TestQuote{}, testStoreError(err)
	}
	var quote experiment.TestQuote
	if err = json.Unmarshal([]byte(row.Request), &quote); err != nil {
		return experiment.TestQuote{}, err
	}
	quote.Key, quote.UserID, quote.TestID, quote.Fingerprint = row.ID, row.UserID, row.TestID.String, row.Fingerprint
	quote.Credits, quote.Free = int(row.EstimatedCredits), row.Free == 1
	quote.ExpiresAt, _ = time.Parse(time.RFC3339Nano, row.ExpiresAt)
	quote.ConsumedRequestKey = row.ConsumedRequestKey
	if len(row.Plan) > 0 {
		if err = json.Unmarshal(row.Plan, &quote.Plan); err != nil {
			return experiment.TestQuote{}, err
		}
	} else {
		quote.Plan = experiment.TestPlan{}
	}
	return quote, nil
}
func (s *Store) GetTestQuote(ctx context.Context, user, id string) (experiment.TestQuote, error) {
	return loadTestQuote(ctx, s.read, user, id)
}
func (s *Store) GetTest(ctx context.Context, user, id string) (experiment.WritingTest, error) {
	return loadWritingTest(ctx, s.read, user, id)
}
func (s *Store) TestByRequest(ctx context.Context, user, key string) (experiment.WritingTest, error) {
	row, err := s.read.GetWritingTestAttemptByRequest(ctx, sqlc.GetWritingTestAttemptByRequestParams{UserID: user, RequestKey: key})
	if err != nil {
		return experiment.WritingTest{}, testStoreError(err)
	}
	return s.GetTest(ctx, user, row.TestID)
}
func (s *Store) ListTests(ctx context.Context, user string, limit int, cursor string) ([]experiment.WritingTest, string, error) {
	if user == "" {
		return nil, "", experiment.ErrTestNotFound
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	at, id := "", ""
	if cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil {
			return nil, "", experiment.ErrTestOperation
		}
		at, id, _ = strings.Cut(string(raw), "\x00")
		if _, err = time.Parse(time.RFC3339Nano, at); err != nil || id == "" {
			return nil, "", experiment.ErrTestOperation
		}
	}
	rows, err := s.read.ListWritingTests(ctx, sqlc.ListWritingTestsParams{UserID: user, CursorTime: at, CursorID: id, PageLimit: int64(limit + 1)})
	if err != nil {
		return nil, "", err
	}
	next := ""
	if len(rows) > limit {
		last := rows[limit-1]
		next = base64.RawURLEncoding.EncodeToString([]byte(last.CreatedAt + "\x00" + last.ID))
		rows = rows[:limit]
	}
	result := make([]experiment.WritingTest, 0, len(rows))
	for _, row := range rows {
		test, err := s.GetTest(ctx, user, row.ID)
		if err != nil {
			return nil, "", err
		}
		test.CommonSnapshot = nil
		for i := range test.Candidates {
			test.Candidates[i].Output = nil
			test.Candidates[i].FrozenVariant = nil
		}
		result = append(result, test)
	}
	return result, next, nil
}
func (s *Store) AdmitTest(ctx context.Context, request experiment.TestStart, plan experiment.TestPlan) (experiment.WritingTest, bool, error) {
	var found experiment.WritingTest
	fresh := false
	err := s.withWritingTestTx(ctx, func(q *sqlc.Queries) error {
		old, err := q.GetWritingTestAttemptByRequest(ctx, sqlc.GetWritingTestAttemptByRequestParams{UserID: request.UserID, RequestKey: request.RequestKey})
		if err == nil {
			if old.Fingerprint != experiment.WritingTestStartFingerprint(request) {
				return experiment.ErrTestOperation
			}
			found, err = loadWritingTest(ctx, q, request.UserID, old.TestID)
			return err
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		quote, err := loadTestQuote(ctx, q, request.UserID, request.QuoteKey)
		if err != nil || quote.TestID != "" || quote.Fingerprint != experiment.WritingTestStartFingerprint(request) || !quote.ExpiresAt.After(time.Now()) || quote.ConsumedRequestKey != "" || quote.Plan.Snapshot.Hash == "" {
			return experiment.ErrTestQuoteRequired
		}
		expectedRaw, _ := encodeTest(quote.Plan)
		actualRaw, _ := encodeTest(plan)
		if !slices.Equal(expectedRaw, actualRaw) {
			return experiment.ErrTestQuoteRequired
		}
		found, err = experiment.BuildWritingTest(request, plan, time.Now())
		if err != nil {
			return err
		}
		info := writingTestContext{Input: found.Input, RetentionSeconds: int64(quote.Retention / time.Second)}
		if info.RetentionSeconds <= 0 {
			info.RetentionSeconds = int64((30 * 24 * time.Hour) / time.Second)
		}
		encoded, err := encodeTest(info)
		if err != nil {
			return err
		}
		err = q.InsertWritingTest(ctx, sqlc.InsertWritingTestParams{ID: found.ID, UserID: found.UserID, OperationKey: request.RequestKey, Fingerprint: experiment.WritingTestStartFingerprint(request), Kind: found.Kind, Factor: string(found.Factor), ModelStage: string(found.ModelStage), Count: int64(found.Count), Status: string(found.Status), SourcePostSlug: nullString(found.SourcePostSlug), Context: string(encoded), CommonSnapshot: found.CommonSnapshot, CommonHash: found.CommonHash, PromptVersion: found.PromptVersion, ReservedCredits: int64(found.ReservedCredits), CreatedAt: formatTime(found.CreatedAt), UpdatedAt: formatTime(found.UpdatedAt)})
		if err != nil {
			return err
		}
		ids := []string{}
		for _, candidate := range found.Candidates {
			identity := writingTestIdentity{SnapshotIndex: candidate.SnapshotIndex, Ref: candidate.Ref, Label: candidate.Identity.Label, Synthetic: candidate.Identity.Synthetic}
			encoded, err := encodeTest(identity)
			if err != nil {
				return err
			}
			if err = q.InsertWritingTestCandidate(ctx, sqlc.InsertWritingTestCandidateParams{ID: candidate.ID, UserID: found.UserID, TestID: found.ID, SeedPosition: int64(candidate.SeedPosition), SourceKind: candidate.Ref.SourceKind, SourceID: string(encoded), SourceRevision: candidate.SourceRevision, SemanticKey: candidate.SemanticKey, FrozenVariant: candidate.FrozenVariant, Status: candidate.Status}); err != nil {
				return err
			}
			ids = append(ids, candidate.ID)
		}
		if err = insertTestAttempt(ctx, q, found, request.RequestKey, quote.Key, quote.Fingerprint, ids, false); err != nil {
			return err
		}
		changed, err := q.ConsumeWritingTestQuote(ctx, sqlc.ConsumeWritingTestQuoteParams{ConsumedRequestKey: request.RequestKey, ConsumedTestID: found.ID, UserID: found.UserID, ID: quote.Key})
		if err != nil {
			return err
		}
		if changed != 1 {
			return experiment.ErrTestQuoteRequired
		}
		fresh = true
		found, err = loadWritingTest(ctx, q, found.UserID, found.ID)
		return err
	})
	return found, fresh, err
}
func insertTestAttempt(ctx context.Context, q *sqlc.Queries, found experiment.WritingTest, key, quote, fingerprint string, ids []string, nonMetered bool) error {
	encoded, err := encodeTest(ids)
	if err != nil {
		return err
	}
	return q.InsertWritingTestAttempt(ctx, sqlc.InsertWritingTestAttemptParams{ID: attemptID(found.UserID, key), UserID: found.UserID, TestID: found.ID, RequestKey: key, Fingerprint: fingerprint, Epoch: int64(found.Revision), PurgeFence: int64(found.PurgeFence), QuoteID: quote, JobID: attemptID(found.UserID, "job\x00"+key), NonMetered: boolValue(nonMetered), Status: "prepared", CandidateIds: string(encoded), CreatedAt: formatTime(found.UpdatedAt), UpdatedAt: formatTime(found.UpdatedAt)})
}
func (s *Store) ReserveTestRetry(ctx context.Context, r experiment.TestRetry, plan experiment.TestPlan) (experiment.WritingTest, bool, error) {
	var found experiment.WritingTest
	fresh := false
	err := s.withWritingTestTx(ctx, func(q *sqlc.Queries) error {
		old, err := q.GetWritingTestAttemptByRequest(ctx, sqlc.GetWritingTestAttemptByRequestParams{UserID: r.UserID, RequestKey: r.RequestKey})
		if err == nil {
			if old.TestID != r.TestID || old.Fingerprint != experiment.WritingTestRetryFingerprint(r) {
				return experiment.ErrTestOperation
			}
			found, err = loadWritingTest(ctx, q, r.UserID, r.TestID)
			return err
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		found, err = loadWritingTest(ctx, q, r.UserID, r.TestID)
		if err != nil {
			return err
		}
		if found.Revision != r.ExpectedRevision {
			return experiment.ErrTestRevisionConflict
		}
		if found.PurgeFence != 0 || (found.Status != experiment.TestPartial && found.Status != experiment.TestFailed) {
			return experiment.ErrTestStateInvalid
		}
		quote, err := loadTestQuote(ctx, q, r.UserID, r.QuoteKey)
		if err != nil || quote.TestID != r.TestID || quote.Fingerprint != experiment.WritingTestRetryFingerprint(r) || quote.Retry.ExpectedRevision != r.ExpectedRevision || quote.ConsumedRequestKey != "" || !quote.ExpiresAt.After(time.Now()) {
			return experiment.ErrTestQuoteRequired
		}
		encoded, _ := encodeTest(plan)
		expected, _ := encodeTest(quote.Plan)
		if !slices.Equal(encoded, expected) || plan.Snapshot.Hash != found.CommonHash || !slices.Equal(plan.Snapshot.Common, found.CommonSnapshot) {
			return experiment.ErrTestQuoteRequired
		}
		if err := experiment.ValidateWritingTestRetryPlan(found, plan); err != nil {
			return err
		}
		if len(plan.Calls) == 0 && (plan.EstimateCredits != 0 || !plan.Free) {
			return experiment.ErrTestMaterialInvalid
		}
		originalWork, err := workForTest(ctx, q, r.UserID, r.TestID)
		if err != nil {
			return err
		}
		originalSnapshot, _ := encodeTest(originalWork.Plan.Snapshot)
		retrySnapshot, _ := encodeTest(plan.Snapshot)
		if !slices.Equal(originalSnapshot, retrySnapshot) {
			return experiment.ErrTestMaterialInvalid
		}
		if len(r.CandidateIDs) == 0 {
			return experiment.ErrTestEntrant
		}
		seen := map[string]bool{}
		for _, id := range r.CandidateIDs {
			if seen[id] {
				return experiment.ErrTestDuplicate
			}
			seen[id] = true
			valid := false
			for i := range found.Candidates {
				candidate := &found.Candidates[i]
				if candidate.ID == id {
					valid = candidate.Status == string(experiment.TestCandidateFailed)
					if valid {
						candidate.Status = string(experiment.TestCandidatePending)
						candidate.Failure = nil
						if err = updateTestCandidate(ctx, q, *candidate); err != nil {
							return err
						}
					}
					break
				}
			}
			if !valid {
				return experiment.ErrTestEntrant
			}
		}
		previous := found.Revision
		found.Revision++
		found.Status = experiment.TestQueued
		found.JobID = ""
		found.Failure = nil
		found.ReservedCredits = plan.EstimateCredits
		found.ContentExpiresAt = nil
		found.UpdatedAt = time.Now().UTC()
		if err = updateWritingTest(ctx, q, &found, previous); err != nil {
			return err
		}
		if err = insertTestAttempt(ctx, q, found, r.RequestKey, quote.Key, quote.Fingerprint, r.CandidateIDs, len(plan.Calls) == 0); err != nil {
			return err
		}
		changed, err := q.ConsumeWritingTestQuote(ctx, sqlc.ConsumeWritingTestQuoteParams{ConsumedRequestKey: r.RequestKey, ConsumedTestID: r.TestID, UserID: r.UserID, ID: r.QuoteKey})
		if err != nil {
			return err
		}
		if changed != 1 {
			return experiment.ErrTestQuoteRequired
		}
		fresh = true
		return nil
	})
	return found, fresh, err
}
func updateTestCandidate(ctx context.Context, q *sqlc.Queries, c experiment.TestCandidate) error {
	failure := ""
	if c.Failure != nil {
		failure = c.Failure.Reason
	}
	finished := sql.NullString{}
	started := sql.NullString{}
	if c.Status == string(experiment.TestCandidateRunning) {
		started = nullString(formatTime(time.Now()))
	}
	if c.Status == string(experiment.TestCandidateSucceeded) || c.Status == string(experiment.TestCandidateFailed) || c.Status == string(experiment.TestCandidateCancelled) {
		finished = nullString(formatTime(time.Now()))
	}
	changed, err := q.UpdateWritingTestCandidate(ctx, sqlc.UpdateWritingTestCandidateParams{Status: c.Status, Output: c.Output, Accounting: c.Accounting, FailureReason: failure, StartedAt: started, FinishedAt: finished, UserID: c.UserID, TestID: c.TestID, ID: c.ID})
	if err != nil {
		return err
	}
	if changed != 1 {
		return experiment.ErrTestEntrant
	}
	return nil
}
func (s *Store) DecideMatch(ctx context.Context, r experiment.MatchDecision) (experiment.WritingTest, error) {
	var found experiment.WritingTest
	if r.UserID == "" || r.TestID == "" || r.RequestKey == "" || r.MatchID == "" || r.WinnerCandidateID == "" {
		return found, experiment.ErrTestOperation
	}
	err := s.withWritingTestTx(ctx, func(q *sqlc.Queries) error {
		decision, err := q.GetWritingTestDecision(ctx, sqlc.GetWritingTestDecisionParams{UserID: r.UserID, DecisionKey: nullString(r.RequestKey)})
		if err == nil {
			if decision.TestID != r.TestID || decision.ID != r.MatchID || decision.WinnerCandidateID.String != r.WinnerCandidateID {
				return experiment.ErrTestDecisionConflict
			}
			found, err = loadWritingTest(ctx, q, r.UserID, r.TestID)
			return err
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		found, err = loadWritingTest(ctx, q, r.UserID, r.TestID)
		if err != nil {
			return err
		}
		if found.Revision != r.ExpectedRevision {
			return experiment.ErrTestRevisionConflict
		}
		if found.Status != experiment.TestReview || found.PurgeFence != 0 {
			return experiment.ErrTestStateInvalid
		}
		index := -1
		for i, match := range found.Matches {
			if match.ID == r.MatchID {
				index = i
				break
			}
		}
		if index < 0 {
			return experiment.ErrTestMatchInvalid
		}
		match := &found.Matches[index]
		if match.WinnerID != "" {
			return experiment.ErrTestDecisionConflict
		}
		if r.WinnerCandidateID != match.LeftID && r.WinnerCandidateID != match.RightID {
			return experiment.ErrTestMatchInvalid
		}
		changed, err := q.DecideWritingTestMatch(ctx, sqlc.DecideWritingTestMatchParams{WinnerCandidateID: nullString(r.WinnerCandidateID), DecisionKey: nullString(r.RequestKey), DecidedAt: nullString(formatTime(time.Now())), UserID: r.UserID, TestID: r.TestID, ID: r.MatchID})
		if err != nil {
			return err
		}
		if changed != 1 {
			return experiment.ErrTestDecisionConflict
		}
		match.WinnerID, match.DecisionKey = r.WinnerCandidateID, r.RequestKey
		count := len(found.Matches)
		if err = experiment.AdvanceWritingTestMatches(&found, match.Round); err != nil {
			return err
		}
		if err = insertTestMatches(ctx, q, found, count); err != nil {
			return err
		}
		previous := found.Revision
		found.Revision++
		found.UpdatedAt = time.Now().UTC()
		if found.Status == experiment.TestCompleted {
			if err = setTestExpiry(ctx, q, &found); err != nil {
				return err
			}
		}
		return updateWritingTest(ctx, q, &found, previous)
	})
	return found, err
}
func setTestExpiry(ctx context.Context, q *sqlc.Queries, found *experiment.WritingTest) error {
	row, err := q.GetWritingTest(ctx, sqlc.GetWritingTestParams{UserID: found.UserID, ID: found.ID})
	if err != nil {
		return err
	}
	var info writingTestContext
	if err = json.Unmarshal([]byte(row.Context), &info); err != nil {
		return err
	}
	retention := time.Duration(info.RetentionSeconds) * time.Second
	if retention <= 0 {
		retention = 30 * 24 * time.Hour
	}
	expires := found.UpdatedAt.Add(retention)
	found.ContentExpiresAt = &expires
	return nil
}
func (s *Store) CancelTest(ctx context.Context, r experiment.TestMutation) (experiment.WritingTest, error) {
	var found experiment.WritingTest
	if r.UserID == "" || r.TestID == "" || r.RequestKey == "" {
		return found, experiment.ErrTestOperation
	}
	fingerprint := attemptID(r.UserID, r.TestID+"\x00cancel")
	err := s.withWritingTestTx(ctx, func(q *sqlc.Queries) error {
		old, err := q.GetWritingTestOperation(ctx, sqlc.GetWritingTestOperationParams{UserID: r.UserID, OperationKey: r.RequestKey})
		if err == nil {
			if old.TestID != r.TestID || old.Fingerprint != fingerprint {
				return experiment.ErrTestOperation
			}
			found, err = loadWritingTest(ctx, q, r.UserID, r.TestID)
			return err
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		found, err = loadWritingTest(ctx, q, r.UserID, r.TestID)
		if err != nil {
			return err
		}
		if found.Revision != r.ExpectedRevision {
			return experiment.ErrTestRevisionConflict
		}
		if found.Status == experiment.TestCompleted || found.Status == experiment.TestCancelled {
			return experiment.ErrTestStateInvalid
		}
		previous := found.Revision
		found.Revision++
		found.Status = experiment.TestCancelled
		found.UpdatedAt = time.Now().UTC()
		if err = setTestExpiry(ctx, q, &found); err != nil {
			return err
		}
		if err = cancelOpenAttempts(ctx, q, found); err != nil {
			return err
		}
		for i := range found.Candidates {
			c := &found.Candidates[i]
			if c.Status == string(experiment.TestCandidatePending) || c.Status == string(experiment.TestCandidateRunning) {
				c.Status = string(experiment.TestCandidateCancelled)
				if err = updateTestCandidate(ctx, q, *c); err != nil {
					return err
				}
			}
		}
		if err = updateWritingTest(ctx, q, &found, previous); err != nil {
			return err
		}
		return q.InsertWritingTestOperation(ctx, sqlc.InsertWritingTestOperationParams{UserID: r.UserID, OperationKey: r.RequestKey, TestID: r.TestID, Action: "cancel", Fingerprint: fingerprint, CreatedAt: formatTime(found.UpdatedAt)})
	})
	return found, err
}
func cancelOpenAttempts(ctx context.Context, q *sqlc.Queries, found experiment.WritingTest) error {
	attempts, err := q.ListWritingTestAttempts(ctx, sqlc.ListWritingTestAttemptsParams{UserID: found.UserID, TestID: found.ID})
	if err != nil {
		return err
	}
	for _, attempt := range attempts {
		if attempt.Status == "prepared" || attempt.Status == "queued" || attempt.Status == "running" {
			if err = updateAttempt(ctx, q, attempt, "cancelled", attempt.JobID, attempt.ConfirmedCredits, attempt.Settled, "", found.UpdatedAt); err != nil {
				return err
			}
		}
	}
	return nil
}
func updateAttempt(ctx context.Context, q *sqlc.Queries, attempt sqlc.WritingTestAttempt, status, jobID string, credits, settled int64, failure string, at time.Time) error {
	changed, err := q.UpdateWritingTestAttempt(ctx, sqlc.UpdateWritingTestAttemptParams{Status: status, JobID: jobID, ConfirmedCredits: credits, Settled: settled, FailureReason: failure, UpdatedAt: formatTime(at), UserID: attempt.UserID, TestID: attempt.TestID, ID: attempt.ID, Status_2: attempt.Status})
	if err != nil {
		return err
	}
	if changed != 1 {
		return experiment.ErrTestStateInvalid
	}
	return nil
}
