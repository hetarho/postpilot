package clip_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

// CLIP-232: a posted URL is an https URL on its channel's platform host or a subdomain.
func TestPublishedURLHostsPerChannel(t *testing.T) {
	for _, c := range []struct {
		channel, url string
		ok           bool
	}{
		{clip.ChannelNaver, " https://naver.me/abc ", true},
		{clip.ChannelNaver, "https://m.clip.naver.com/v/1", true},
		{"", "https://naver.com/clip", true},
		{clip.ChannelNaver, "http://naver.me/abc", false},
		{clip.ChannelNaver, "https://naver.me.evil.com/abc", false},
		{clip.ChannelNaver, "https://notnaver.com/abc", false},
		{clip.ChannelNaver, "https://www.instagram.com/reel/a", false},
		{clip.ChannelInstagram, "https://www.instagram.com/reel/a", true},
		{clip.ChannelInstagram, "https://user@instagram.com/reel/a", false},
		{clip.ChannelInstagram, "https://naver.me/abc", false},
		{clip.ChannelInstagram, "", false},
		{clip.ChannelInstagram, "https://instagram.com/" + strings.Repeat("a", clip.PublishedURLMaxChars), false},
	} {
		got, err := clip.NormalizePublishedURL(c.channel, c.url)
		if c.ok != (err == nil) || (!c.ok && !errors.Is(err, clip.ErrPublishedURLInvalid)) {
			t.Errorf("%s %q: %v", c.channel, c.url, err)
		}
		if c.ok && got != strings.TrimSpace(c.url) {
			t.Errorf("%q was not trimmed: %q", c.url, got)
		}
	}
}

// CLIP-231: ticks are advisory metadata a finalized project still saves; anything else is not.
func TestTicksAreFinalizedWritableMetadata(t *testing.T) {
	ticks, channel, title := []string{"a"}, clip.ChannelInstagram, "t"
	if !(clip.ProjectPatch{ChecklistTicks: &ticks}).TicksOnly() || !(clip.ProjectPatch{ChecklistTicks: &ticks, Channel: &channel}).MetadataOnly() {
		t.Fatal("ticks or channel are not metadata")
	}
	if (clip.ProjectPatch{ChecklistTicks: &ticks, Channel: &channel}).TicksOnly() || (clip.ProjectPatch{ChecklistTicks: &ticks, Title: &title}).MetadataOnly() || (clip.ProjectPatch{}).MetadataOnly() {
		t.Fatal("a content patch passed as metadata")
	}
	if _, err := clip.NormalizeChecklistTicks([]string{strings.Repeat("가", 501)}); err == nil {
		t.Fatal("an over-long tick line was accepted")
	}
}
