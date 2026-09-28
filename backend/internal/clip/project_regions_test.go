package clip

import (
	"reflect"
	"testing"

	"github.com/postpilot/backend/internal/clip/composition"
)

func regionFixture(t *testing.T) Project {
	t.Helper()
	c := NoTemplateComposition()
	c.Snapshot.Body = `<clip version="1"><field id="name" label="Name"/><text id="intro" kind="fixed" role="hook">Hello <value field="name"/></text><text id="end" kind="ai" role="ending">A farewell</text></clip>`
	c.Inputs.Values["name"] = "Ada"
	return Project{Composition: &c, IntroPreset: "cover", OutroPreset: "credits"}
}

func TestProjectRegionSeedingEditsAndCapacity(t *testing.T) {
	p := regionFixture(t)
	r, err := SeedProjectRegions(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Intro.Enabled || !r.Outro.Enabled || r.Intro.Slots[0].Text != "Hello Ada" || r.Outro.Slots[0].Instruction != "A farewell" {
		t.Fatalf("seed: %+v", r)
	}
	empty, instruction := "", "Owner instruction"
	if err := ApplyRegionPatch(&r.Intro, &RegionPatch{Slots: []RegionSlotPatch{{ID: r.Intro.Slots[0].ID, Text: &empty}}}, DefaultLimits()); err != nil {
		t.Fatal(err)
	}
	if err := ApplyRegionPatch(&r.Outro, &RegionPatch{Slots: []RegionSlotPatch{{ID: r.Outro.Slots[0].ID, Instruction: &instruction}}}, DefaultLimits()); err != nil {
		t.Fatal(err)
	}
	p.Composition.Inputs.Values["name"] = "Grace"
	RefreshRegionBindings(&r, p.Composition.Inputs)
	seeded, err := SeedProjectRegions(p, &r)
	if err != nil {
		t.Fatal(err)
	}
	if !seeded.Intro.Slots[0].OwnerFixed || seeded.Intro.Slots[0].Text != "" || len(seeded.Intro.Slots[0].Binding) != 0 || seeded.Outro.Slots[0].Instruction != instruction || seeded.Outro.Slots[0].OwnerFixed {
		t.Fatalf("ownership lost: %+v", seeded)
	}
	r.Intro.Slots[2].Text = "retained"
	p.IntroPreset = "a"
	EnsureRegionSlots(&r, p)
	if len(r.Intro.Slots) != 3 || r.Intro.Slots[2].Text != "retained" {
		t.Fatal("surplus lost", r)
	}
	p.IntroPreset = "cover"
	EnsureRegionSlots(&r, p)
	if r.Intro.Slots[2].Text != "retained" {
		t.Fatal("restore lost", r)
	}
}

func TestRegionCompatibilityDoesNotEnableAStoredPreset(t *testing.T) {
	p := regionFixture(t)
	p.EditPlan = ""
	p.Composition = nil
	r := EffectiveProjectRegions(p)
	if r.Intro.Enabled || r.Outro.Enabled {
		t.Fatal("presets enabled absent regions", r)
	}
	r.Intro.Enabled = true
	r.Intro.Slots[0].Text = "saved"
	p.Regions = &r
	copy := EffectiveProjectRegions(p)
	copy.Intro.Slots[0].Text = "changed"
	if reflect.DeepEqual(copy, r) || r.Intro.Slots[0].Text != "saved" {
		t.Fatal("draft aliased stored slots")
	}
}

func TestRegionBindingRefreshAndPatchValidation(t *testing.T) {
	p := regionFixture(t)
	r, err := SeedProjectRegions(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	RefreshRegionBindings(&r, CompositionInputs{Values: map[string]string{"name": "Grace"}})
	if r.Intro.Slots[0].Text != "Hello Grace" {
		t.Fatal(r)
	}
	if err := ApplyRegionPatch(&r.Intro, &RegionPatch{Slots: []RegionSlotPatch{{ID: "unknown"}}}, DefaultLimits()); err == nil {
		t.Fatal("unknown slot admitted")
	}
	r.Intro.Slots[0].Binding = []composition.Part{{Field: "name"}}
	f := false
	if err := ApplyRegionPatch(&r.Intro, &RegionPatch{Enabled: &f}, DefaultLimits()); err != nil || r.Intro.Enabled || r.Intro.Slots[0].Text != "Hello Grace" {
		t.Fatal("off deleted content", r, err)
	}
}
