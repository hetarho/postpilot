package post

import "testing"

// never is the "no slug is taken" collision probe.
func never(string) bool { return false }

// taken makes exactly the listed slugs collide.
func taken(slugs ...string) func(string) bool {
	set := make(map[string]bool, len(slugs))
	for _, s := range slugs {
		set[s] = true
	}
	return func(candidate string) bool { return set[candidate] }
}

func TestMintSlug(t *testing.T) {
	cases := map[string]struct {
		title string
		want  string
	}{
		"plain":                {title: "Jeju Day 3", want: "20260301-jeju-day-3"},
		"korean is kept":       {title: "성산 3일차", want: "20260301-성산-3일차"},
		"collapses whitespace": {title: "  제주   3일차  ", want: "20260301-제주-3일차"},
		"existing separators":  {title: "jeju-day_3", want: "20260301-jeju-day-3"},
		"empty title":          {title: "", want: "20260301-untitled"},
		"only whitespace":      {title: "   ", want: "20260301-untitled"},
		// A title made only of stripped characters must not mint a bare date, which
		// would collide with every other such post that day.
		"only unsafe characters": {title: "///???", want: "20260301-untitled"},
		// These would break a URL path segment or an object key.
		"strips path characters": {title: "a/b\\c?d#e%f:g*h\"i<j>k|l", want: "20260301-abcdefghijkl"},
		"strips dots":            {title: "v1.2.3", want: "20260301-v123"},
		"lowercases":             {title: "HELLO World", want: "20260301-hello-world"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := MintSlug("20260301", tc.title, never); got != tc.want {
				t.Errorf("MintSlug(%q) = %q, want %q", tc.title, got, tc.want)
			}
		})
	}
}

// TestMintSlugCollision is plan 02 AC10: same title, same day, serial suffix.
func TestMintSlugCollision(t *testing.T) {
	first := MintSlug("20260301", "성산", never)
	if first != "20260301-성산" {
		t.Fatalf("first = %q", first)
	}

	second := MintSlug("20260301", "성산", taken("20260301-성산"))
	if second != "20260301-성산-2" {
		t.Errorf("second = %q, want 20260301-성산-2", second)
	}

	third := MintSlug("20260301", "성산", taken("20260301-성산", "20260301-성산-2"))
	if third != "20260301-성산-3" {
		t.Errorf("third = %q, want 20260301-성산-3", third)
	}
}

func TestMintSlugTruncatesLongTitles(t *testing.T) {
	// The slug goes into an object key, so a pasted paragraph must not become one.
	long := ""
	for range 200 {
		long += "가"
	}

	got := MintSlug("20260301", long, never)
	body := []rune(got[len("20260301-"):])
	if len(body) != maxSlugBodyRunes {
		t.Errorf("body length = %d runes, want %d", len(body), maxSlugBodyRunes)
	}
}

func TestMintSlugNeverEndsWithASeparator(t *testing.T) {
	// Truncation can land mid-separator, and a trailing hyphen reads as a mistake.
	for _, title := range []string{"trailing-", "trailing ", "a-", "-"} {
		got := MintSlug("20260301", title, never)
		if got[len(got)-1] == '-' {
			t.Errorf("MintSlug(%q) = %q, ends with a separator", title, got)
		}
	}
}

func TestObjectKey(t *testing.T) {
	// PRD §5 fixes this shape; the sweep and the store both depend on it.
	if got := ObjectKey("20260301-jeju", "abc123"); got != "posts/20260301-jeju/abc123.jpg" {
		t.Errorf("ObjectKey = %q", got)
	}
}
