package experiment

import "fmt"

func StatusAfterCandidates(candidates []Candidate) (Status, error) {
	succeeded := 0
	failed := 0
	for _, candidate := range candidates {
		switch candidate.Status {
		case CandidateSucceeded:
			succeeded++
		case CandidateFailed:
			failed++
		}
	}
	switch {
	case succeeded == 2:
		return StatusReview, nil
	case succeeded == 1 && failed == 1:
		return StatusPartial, nil
	case failed == 2:
		return StatusFailed, nil
	default:
		return "", fmt.Errorf("%w: candidate completion is not terminal", ErrInvalidState)
	}
}

func ValidateVerdict(found Experiment, candidateID string, allowSingle bool) (Candidate, error) {
	if found.Status != StatusReview && found.Status != StatusPartial {
		return Candidate{}, ErrInvalidState
	}
	for _, candidate := range found.Candidates {
		if candidate.ID != candidateID {
			continue
		}
		if candidate.Status != CandidateSucceeded {
			return Candidate{}, ErrInvalidState
		}
		if found.Status == StatusPartial && !allowSingle {
			return Candidate{}, ErrInvalidState
		}
		if found.Status == StatusReview && allowSingle {
			return Candidate{}, ErrInvalidState
		}
		return candidate, nil
	}
	return Candidate{}, ErrCandidateNotFound
}
