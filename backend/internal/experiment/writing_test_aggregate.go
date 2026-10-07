package experiment

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"slices"
	"strings"
	"time"
)

func WritingTestStartFingerprint(request TestStart) string {
	request.RequestKey, request.QuoteKey = "", ""
	raw, _ := json.Marshal(request)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
func WritingTestRetryFingerprint(request TestRetry) string {
	request.RequestKey, request.QuoteKey = "", ""
	request.ExpectedRevision = 0
	request.CandidateIDs = slices.Clone(request.CandidateIDs)
	slices.Sort(request.CandidateIDs)
	raw, _ := json.Marshal(request)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
func ValidWritingTestPlan(request TestStart, plan TestPlan) error {
	if err := ValidateTestShape(request); err != nil {
		return err
	}
	if !Language(request.Input.TargetLanguage).Valid() || len(plan.Snapshot.Common) == 0 || plan.Snapshot.Hash == "" || plan.Snapshot.PromptVersion == "" || len(plan.Snapshot.Variants) != request.Count || len(plan.Calls) == 0 || plan.EstimateCredits < 0 {
		return ErrTestMaterialInvalid
	}
	keys := map[string]bool{}
	for index, variant := range plan.Snapshot.Variants {
		if variant.Reference != request.Entrants[index] || len(variant.Content) == 0 || variant.Revision == "" || variant.SemanticKey == "" {
			return ErrTestEntrant
		}
		if keys[variant.SemanticKey] {
			return ErrTestDuplicate
		}
		keys[variant.SemanticKey] = true
	}
	for _, call := range plan.Calls {
		if call.Ref.ProviderID == "" || call.Ref.ModelID == "" || (call.Stage != StageObserve && call.Stage != StageWrite) || call.Count <= 0 || call.PromptTokens <= 0 || call.CompletionTokens <= 0 {
			return ErrTestMaterialInvalid
		}
	}
	return nil
}

// BuildWritingTest assigns opaque identities and one unbiased shuffle. Persistence
// resolves duplicate request keys before storing this aggregate, so retries reuse it.
func BuildWritingTest(request TestStart, plan TestPlan, at time.Time) (WritingTest, error) {
	if err := ValidWritingTestPlan(request, plan); err != nil {
		return WritingTest{}, err
	}
	out := WritingTest{ID: newID(), UserID: request.UserID, Kind: "knockout", Factor: request.Factor, ModelStage: request.ModelStage, Count: request.Count, Status: TestQueued, Input: request.Input, SourcePostSlug: request.Input.SourcePostSlug, CommonSnapshot: append([]byte(nil), plan.Snapshot.Common...), CommonHash: plan.Snapshot.Hash, PromptVersion: plan.Snapshot.PromptVersion, CreatedAt: at.UTC(), UpdatedAt: at.UTC(), ReservedCredits: plan.EstimateCredits}
	if request.Count == 2 {
		out.Kind = "ab"
	}
	positions := make([]int, request.Count)
	for index := range positions {
		positions[index] = index
	}
	for index := len(positions) - 1; index > 0; index-- {
		pick, err := rand.Int(rand.Reader, big.NewInt(int64(index+1)))
		if err != nil {
			return WritingTest{}, err
		}
		positions[index], positions[int(pick.Int64())] = positions[int(pick.Int64())], positions[index]
	}
	for index, variant := range plan.Snapshot.Variants {
		label := variant.Label
		if label == "" {
			label = variant.Reference.SettingID
		}
		if label == "" && variant.Reference.SourceKind == "model" {
			label = variant.Reference.Model.String()
		}
		if label == "" && variant.Reference.SourceKind == "authoring_candidate" {
			label = "Prepared setting"
		}
		out.Candidates = append(out.Candidates, TestCandidate{ID: newID(), UserID: out.UserID, TestID: out.ID, SeedPosition: positions[index], SnapshotIndex: index, Ref: variant.Reference, SourceRevision: variant.Revision, SemanticKey: variant.SemanticKey, FrozenVariant: append([]byte(nil), variant.Content...), Status: string(TestCandidatePending), Identity: &TestCandidateIdentity{Label: label, Ref: variant.Reference, Synthetic: variant.Synthetic}})
	}
	return out, nil
}

// OpenWritingTestMatches is the only all-success barrier. It invents no losses or byes.
func OpenWritingTestMatches(test *WritingTest) error {
	if len(test.Candidates) != test.Count || !ValidTestCount(test.Count) || test.PurgeFence != 0 {
		return ErrTestStateInvalid
	}
	for _, candidate := range test.Candidates {
		if candidate.Status != string(TestCandidateSucceeded) || len(candidate.Output) == 0 {
			return ErrTestStateInvalid
		}
	}
	if len(test.Matches) != 0 {
		return nil
	}
	ordered := slices.Clone(test.Candidates)
	slices.SortFunc(ordered, func(a, b TestCandidate) int { return a.SeedPosition - b.SeedPosition })
	for index := 0; index < len(ordered); index += 2 {
		test.Matches = append(test.Matches, TestMatch{ID: newID(), Round: 1, Index: index / 2, LeftID: ordered[index].ID, RightID: ordered[index+1].ID})
	}
	test.Status = TestReview
	return nil
}

func AdvanceWritingTestMatches(test *WritingTest, round int) error {
	winners := []TestMatch{}
	for _, match := range test.Matches {
		if match.Round == round {
			if match.WinnerID == "" {
				return nil
			}
			winners = append(winners, match)
		}
	}
	if len(winners) == 0 {
		return ErrTestMatchInvalid
	}
	slices.SortFunc(winners, func(a, b TestMatch) int { return a.Index - b.Index })
	if len(winners) == 1 {
		test.WinnerID = winners[0].WinnerID
		test.Status = TestCompleted
		return nil
	}
	for _, match := range test.Matches {
		if match.Round == round+1 {
			return nil
		}
	}
	for index := 0; index < len(winners); index += 2 {
		test.Matches = append(test.Matches, TestMatch{ID: newID(), Round: round + 1, Index: index / 2, LeftID: winners[index].WinnerID, RightID: winners[index+1].WinnerID})
	}
	return nil
}

// ProjectWritingTest is safe even if an RPC forgets a particular private member.
// Supplier accounting is never public, and a bracket reveals identity only at its boundary.
func ProjectWritingTest(test WritingTest) WritingTest {
	revealed := test.Status == TestCompleted || test.Status == TestCancelled
	out := test
	out.CommonSnapshot = nil
	out.CommonHash, out.PromptVersion = "", ""
	out.Input = TestInput{TargetLanguage: test.Input.TargetLanguage, Fictional: test.Input.Fictional}
	if !revealed {
		out.SourcePostSlug = ""
		out.ConfirmedCredits, out.ReservedCredits = 0, 0
	}
	if out.Failure != nil {
		out.Failure = &Failure{Reason: out.Failure.Reason}
	}
	out.Candidates = slices.Clone(test.Candidates)
	for index, candidate := range out.Candidates {
		projection := candidate.Project(revealed, "Candidate "+strings.TrimSpace(candidate.ID))
		out.Candidates[index].FrozenVariant, out.Candidates[index].Accounting = nil, nil
		out.Candidates[index].SeedPosition, out.Candidates[index].SnapshotIndex = 0, 0
		out.Candidates[index].Ref = TestEntrantRef{}
		out.Candidates[index].SourceRevision, out.Candidates[index].SemanticKey = "", ""
		out.Candidates[index].Identity = projection.Identity
		out.Candidates[index].Failure = projection.Failure
		out.Candidates[index].Usage = nil
		if projection.Usage != nil {
			out.Candidates[index].Usage = &Usage{PromptTokens: projection.Usage.PromptTokens, CompletionTokens: projection.Usage.CompletionTokens, LatencyMS: projection.Usage.LatencyMS}
		}
		out.Candidates[index].Output = projection.Output
	}
	out.Publications = slices.Clone(test.Publications)
	for index := range out.Publications {
		out.Publications[index].Fingerprint = ""
	}
	return out
}
