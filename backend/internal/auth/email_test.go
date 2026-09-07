package auth

import (
	"strings"
	"testing"
)

func TestNormalizeEmail(t *testing.T) {
	for raw, want := range map[string]string{
		"A@B.com":              "a@b.com",
		" a@b.com ":            "a@b.com",
		"name+tag@sub.example": "name+tag@sub.example",
	} {
		got, err := NormalizeEmail(raw)
		if err != nil || got != want {
			t.Errorf("NormalizeEmail(%q) = %q, %v; want %q", raw, got, err, want)
		}
	}
	first, err := NormalizeEmail("A@B.com")
	if err != nil {
		t.Fatal(err)
	}
	second, err := NormalizeEmail("a@b.com")
	if err != nil || first != second {
		t.Fatalf("case variants normalized to %q and %q (%v)", first, second, err)
	}
}

func TestNormalizeEmailRejectsInvalidShapes(t *testing.T) {
	tooLong := strings.Repeat("a", 249) + "@b.com"
	for name, raw := range map[string]string{
		"empty":              "",
		"missing at":         "a.example.com",
		"two ats":            "a@b@example.com",
		"empty local":        "@example.com",
		"undotted domain":    "a@example",
		"empty domain label": "a@example..com",
		"whitespace":         "a b@example.com",
		"control":            "a\x00@example.com",
		"too long":           tooLong,
	} {
		t.Run(name, func(t *testing.T) {
			if got, err := NormalizeEmail(raw); err == nil {
				t.Fatalf("NormalizeEmail(%q) = %q, want an error", raw, got)
			}
		})
	}
}
