package clip_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

// CLIP-232: a posted URL is an https URL on 네이버 클립's platform host or a subdomain.
func TestPublishedURLHosts(t *testing.T) {
	for _, c := range []struct {
		url string
		ok  bool
	}{
		{" https://naver.me/abc ", true},
		{"https://m.clip.naver.com/v/1", true},
		{"https://naver.com/clip", true},
		{"http://naver.me/abc", false},
		{"https://naver.me.evil.com/abc", false},
		{"https://notnaver.com/abc", false},
		{"https://user@naver.com/abc", false},
		{"https://www.instagram.com/reel/a", false},
		{"", false},
		{"https://naver.com/" + strings.Repeat("a", clip.PublishedURLMaxChars), false},
	} {
		got, err := clip.NormalizePublishedURL(c.url)
		if c.ok != (err == nil) || (!c.ok && !errors.Is(err, clip.ErrPublishedURLInvalid)) {
			t.Errorf("%q: %v", c.url, err)
		}
		if c.ok && got != strings.TrimSpace(c.url) {
			t.Errorf("%q was not trimmed: %q", c.url, got)
		}
	}
}

// CLIP-231: ticks are advisory metadata a finalized project still saves; anything else is not.
func TestTicksAreFinalizedWritableMetadata(t *testing.T) {
	ticks, title := []string{"a"}, "t"
	if !(clip.ProjectPatch{ChecklistTicks: &ticks}).TicksOnly() {
		t.Fatal("ticks are not metadata")
	}
	if (clip.ProjectPatch{ChecklistTicks: &ticks, Title: &title}).TicksOnly() || (clip.ProjectPatch{}).TicksOnly() {
		t.Fatal("a content patch passed as metadata")
	}
	if _, err := clip.NormalizeChecklistTicks([]string{strings.Repeat("가", 501)}); err == nil {
		t.Fatal("an over-long tick line was accepted")
	}
}
