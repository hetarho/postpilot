package clip

import (
	"github.com/postpilot/backend/internal/campaign"
	"slices"
)

// AdvertiserGuide is advertiser material, separate from the owner's experience.
type AdvertiserGuide = campaign.Guide

func NormalizeAdvertiserGuide(guide AdvertiserGuide) (AdvertiserGuide, error) {
	return campaign.Normalize(guide, campaign.ClipOptions)
}

// NormalizeChecklistTicks bounds the owner's ticked checklist lines as a post's are
// (CLIP-231 →POST-119): trimmed, deduplicated, blank lines dropped.
func NormalizeChecklistTicks(lines []string) ([]string, error) {
	return campaign.NormalizeTicks(lines)
}

// EffectiveAdvertiserGuide freezes only material the writer is actually given.
// Off and empty stay absent, preserving existing prompts and semantic digests.
func EffectiveAdvertiserGuide(p Project) *AdvertiserGuide {
	if !p.Campaign || p.AdvertiserGuide.Empty() {
		return nil
	}
	guide := p.AdvertiserGuide
	guide.Keywords = slices.Clone(guide.Keywords)
	guide.Hashtags = slices.Clone(guide.Hashtags)
	return &guide
}
