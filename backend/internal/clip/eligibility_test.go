package clip

import (
	"errors"
	"testing"
)

// The wire's unspecified value is not a status, and eligible is never a default.
func TestEligibilityParsingRejectsUnspecified(t *testing.T) {
	for _, value := range []string{"", "ELIGIBLE", "unspecified", "maybe"} {
		if _, err := ParseEligibility(value); !errors.Is(err, ErrInvalid) {
			t.Fatalf("%q parsed", value)
		}
	}
	for _, s := range []EligibilityStatus{EligibilityEligible, EligibilityVideoInputAbsent, EligibilityInlineEndpointUnavailable, EligibilityRequiredParametersUnsupported, EligibilityPriceCeilingUnavailable} {
		if got, err := ParseEligibility(string(s)); err != nil || got != s {
			t.Fatal(s, err)
		}
	}
	if EligibilityEligible.FailureReason() != "" {
		t.Fatal("eligible has a failure reason")
	}
}
