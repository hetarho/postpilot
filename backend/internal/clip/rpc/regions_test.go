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
