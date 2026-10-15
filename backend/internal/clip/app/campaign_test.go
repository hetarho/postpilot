package app

import (
	"encoding/json"
	"fmt"
	"github.com/postpilot/backend/internal/clip"
	"testing"
)

func TestAdvertiserGuideInvalidatesEveryWritingDigestWhileOn(t *testing.T) {
	guide := clip.AdvertiserGuide{Text: "광고주 안내", Keywords: []string{"키워드"}, Hashtags: []string{"체험"}}
	before := clip.Project{Campaign: true, AdvertiserGuide: guide}
	digests := map[string]func(clip.Project) string{
		"generation": func(p clip.Project) string {
			return clip.QuoteInputDigest(p, clip.VideoTemplate{}, clip.SourceBatch{}, clip.GenerationPricing{})
		},
		"revision": func(p clip.Project) string {
			return clip.RevisionInputDigest(p, "줄여줘", clip.RevisionBoth, "writer", clip.GenerationPricing{})
		},
		"storyline-revision": func(p clip.Project) string {
			return StorylineRevisionInputDigest(p, "줄여줘", "writer", clip.GenerationPricing{})
		},
		"recovery": func(p clip.Project) string {
			return planRecoveryDigest(clip.GenerationPayload{AdvertiserGuide: clip.EffectiveAdvertiserGuide(p)})
		},
	}
	for name, digest := range digests {
		t.Run(name, func(t *testing.T) {
			for _, change := range []func(*clip.Project){
				func(p *clip.Project) { p.AdvertiserGuide.Text = "다른 요청" },
				func(p *clip.Project) { p.AdvertiserGuide.Keywords = []string{"다른 키워드"} },
				func(p *clip.Project) { p.AdvertiserGuide.Hashtags = []string{"다른 태그"} },
				func(p *clip.Project) { p.Campaign = false },
			} {
				after := before
				change(&after)
				if digest(before) == digest(after) {
					t.Fatal("changed material reused approved writing")
				}
			}
			off := before
			off.Campaign = false
			if digest(off) != digest(clip.Project{}) {
				t.Fatal("inactive material changed the digest")
			}
			if digest(clip.Project{Campaign: true}) != digest(clip.Project{}) {
				t.Fatal("empty material changed the digest")
			}
		})
	}
}

func TestAcceptedGenerationReadsOnlyFrozenAdvertiserMaterial(t *testing.T) {
	project := clip.Project{Campaign: true, AdvertiserGuide: clip.AdvertiserGuide{Text: "凍結", Keywords: []string{"원래"}}}
	frozen := clip.GenerationPayload{Version: clip.GenerationPayloadVersion, Language: "ko", AdvertiserGuide: clip.EffectiveAdvertiserGuide(project)}
	project.AdvertiserGuide.Keywords[0] = "나중"
	raw, err := json.Marshal(frozen)
	if err != nil {
		t.Fatal(err)
	}
	var restored clip.GenerationPayload
	if err = json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	run := generationRun{p: restored}
	if got := run.planningInput().AdvertiserGuide; got == nil || got.Text != "凍結" || got.Keywords[0] != "원래" {
		t.Fatalf("snapshot changed: %+v", got)
	}
	for _, version := range []int{3, 8, 9, 10} {
		if !clip.SupportedGenerationPayload(version) {
			t.Fatal("legacy job refused", version)
		}
		var old clip.GenerationPayload
		if err = json.Unmarshal([]byte(fmt.Sprintf(`{"Version":%d,"Language":"ko"}`, version)), &old); err != nil {
			t.Fatal(err)
		}
		old.Version = version
		run.p = old
		if run.planningInput().AdvertiserGuide != nil {
			t.Fatal("legacy job gained advertiser material")
		}
	}
}
