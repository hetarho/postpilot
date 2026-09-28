package rpc

import (
	"github.com/postpilot/backend/internal/clip"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"testing"
)

func TestRegionWirePresence(t *testing.T) {
	if regionEdit(nil) != nil {
		t.Fatal("absent region became a patch")
	}
	off, blank := false, ""
	p := regionEdit(&v1.ClipRegionEdit{Enabled: &off, Slots: []*v1.ClipRegionSlotEdit{{Id: "project-intro-1", Text: &blank}}})
	if p.Enabled == nil || *p.Enabled || p.Slots[0].Text == nil || *p.Slots[0].Text != "" || p.Slots[0].Instruction != nil {
		t.Fatal("wire lost presence", p)
	}
	value := regionsProto(clip.Project{Regions: &clip.ProjectRegions{Revision: 3, Intro: clip.ProjectRegion{Slots: []clip.RegionSlot{{ID: "project-intro-1", OwnerFixed: true}}}}})
	if value.Revision != 3 || value.Intro.Enabled || !value.Intro.Slots[0].OwnerFixed || value.Intro.Slots[0].Text != "" {
		t.Fatal(value)
	}
}

// A slot the chosen preset cannot draw is noticed by its own id, before the
// project has any plan (CLIP-147).
func TestProjectNoticesNameAnUnusedSlot(t *testing.T) {
	p := clip.Project{Ratio: "vertical", IntroPreset: "a", Regions: &clip.ProjectRegions{Intro: clip.ProjectRegion{Enabled: true, Slots: []clip.RegionSlot{{ID: "project-intro-1"}, {ID: "project-intro-2"}, {ID: "project-intro-3", Text: "남은 말"}}}}}
	notices := projectProto(p).Notices
	if len(notices) != 1 || notices[0].ElementId != "project-intro-3" || notices[0].Code != clip.NoticeRegionLineSurplus || notices[0].Action != "removal" {
		t.Fatal(notices)
	}
	p.Regions.Intro.Enabled = false
	if notices := projectProto(p).Notices; len(notices) != 0 {
		t.Fatal("a region that is off noticed its draft", notices)
	}
}
