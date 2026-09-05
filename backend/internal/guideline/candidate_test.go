package guideline

import (
	"errors"
	"strings"
	"testing"
)

func TestDecideRecordingSkipsWhatTheUserAlreadyRuledOn(t *testing.T) {
	for name, tc := range map[string]struct {
		existing  CandidateStatus
		guideline bool
		pending   int
		max       int
		want      CandidateRecording
	}{
		"first sighting":                  {"", false, 0, 2, RecordCandidateInsert},
		"repeat of a pending candidate":   {CandidateStatusPending, false, 1, 2, RecordCandidateCount},
		"already approved":                {CandidateStatusApproved, false, 0, 2, RecordCandidateSkip},
		"already dismissed":               {CandidateStatusDismissed, false, 0, 2, RecordCandidateSkip},
		"already a saved guideline":       {"", true, 0, 2, RecordCandidateSkip},
		"pending queue full":              {"", false, 2, 2, RecordCandidateSkip},
		"a repeat still counts when full": {CandidateStatusPending, false, 2, 2, RecordCandidateCount},
	} {
		t.Run(name, func(t *testing.T) {
			got := DecideRecording(tc.existing, tc.guideline, tc.pending, tc.max)
			if got != tc.want {
				t.Fatalf("DecideRecording = %v, want %v", got, tc.want)
			}
		})
	}
}

// The bound split is the whole point of a candidate: an instruction the guideline bound would
// refuse is still recorded, so the correction is not lost before the user can shorten it.
func TestValidCandidateTextStoresAtTheRevisionBound(t *testing.T) {
	long := strings.Repeat("가", 400)
	got, err := validCandidateText("  " + long + "  ")
	if err != nil {
		t.Fatalf("400 characters refused: %v", err)
	}
	if got != long {
		t.Fatal("the recorded text is not the trimmed instruction verbatim")
	}
	if _, err := validCandidateText(strings.Repeat("가", CandidateTextMaxChars+1)); err == nil {
		t.Fatal("past the revision bound was accepted")
	}
	if _, err := validCandidateText("   "); !errors.Is(err, ErrCandidateTextInvalid) {
		t.Fatalf("blank err = %v", err)
	}
}

// Verbatim means verbatim: only the surrounding whitespace goes. Nothing normalizes case,
// punctuation, inner spacing or wording, because the dedupe below is exact-after-trim.
func TestValidCandidateTextChangesNothingButTheEdges(t *testing.T) {
	const raw = "여기  너무 광고 같아!! (특히 마지막 문단)"
	got, err := validCandidateText("\n\t" + raw + " \n")
	if err != nil {
		t.Fatal(err)
	}
	if got != raw {
		t.Fatalf("recorded %q, want %q", got, raw)
	}
}
