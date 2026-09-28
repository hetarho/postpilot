package store_test

import (
	"errors"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

func TestProjectRegionPersistencePresenceAndConflicts(t *testing.T) {
	s, _, db := setup(t)
	p, err := s.CreateProject(t.Context(), "alice", clip.ProjectInput{Language: "ko", Title: "Regions", Ratio: "vertical"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Regions == nil || p.Regions.Intro.Enabled || p.Regions.Outro.Enabled {
		t.Fatal("new project regions not off", p.Regions)
	}
	preset := "cover"
	p, err = s.UpdateProject(t.Context(), "alice", p.ID, clip.ProjectPatch{IntroPreset: &preset})
	if err != nil {
		t.Fatal(err)
	}
	if !p.Regions.Intro.Enabled || len(p.Regions.Intro.Slots) != 3 {
		t.Fatal(p.Regions)
	}
	revision := p.Regions.Revision
	empty := ""
	off := false
	patch := clip.ProjectPatch{ExpectedRegionRevision: &revision, IntroRegion: &clip.RegionPatch{Enabled: &off, Slots: []clip.RegionSlotPatch{{ID: p.Regions.Intro.Slots[0].ID, Text: &empty}}}}
	p, err = s.UpdateProject(t.Context(), "alice", p.ID, patch)
	if err != nil {
		t.Fatal(err)
	}
	if p.Regions.Intro.Enabled || !p.Regions.Intro.Slots[0].OwnerFixed || p.Regions.Intro.Slots[0].Text != "" {
		t.Fatal("explicit blank/off lost", p.Regions)
	}
	if _, err = s.UpdateProject(t.Context(), "alice", p.ID, patch); !errors.Is(err, clip.ErrPlanConflict) {
		t.Fatal("stale slot write", err)
	}
	p, err = s.UpdateProject(t.Context(), "alice", p.ID, clip.ProjectPatch{})
	if err != nil || p.Regions.Intro.Enabled || !p.Regions.Intro.Slots[0].OwnerFixed {
		t.Fatal("omitted values changed", err)
	}
	if _, err = s.UpdateProject(t.Context(), "bob", p.ID, clip.ProjectPatch{IntroRegion: &clip.RegionPatch{Enabled: &off}}); !errors.Is(err, clip.ErrNotFound) {
		t.Fatal("foreign update", err)
	}
	// Missing metadata is projected on read, never written back and never inferred from a preset.
	if _, err = db.Writer.Exec(`UPDATE clip_projects SET regions_json=NULL WHERE id=?`, p.ID); err != nil {
		t.Fatal(err)
	}
	before := p.UpdatedAt
	p, err = s.GetProject(t.Context(), "alice", p.ID)
	if err != nil || p.Regions.Intro.Enabled || !p.UpdatedAt.Equal(before) {
		t.Fatal("legacy read changed content", p, err)
	}
	var missing bool
	if err = db.Reader.QueryRow(`SELECT regions_json IS NULL FROM clip_projects WHERE id=?`, p.ID).Scan(&missing); err != nil || !missing {
		t.Fatal("read backfilled state", err)
	}
}

func TestTemplateRegionSeedsPreserveOwnerText(t *testing.T) {
	s, _, _ := setup(t)
	template, err := s.CreateTemplate(t.Context(), "alice", clip.Recipe{Name: "Seed", CompositionBody: `<clip version="1"><text id="intro" kind="fixed" role="hook"><row>Template</row></text></clip>`})
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.CreateProject(t.Context(), "alice", clip.ProjectInput{Language: "ko", Title: "Regions", Ratio: "vertical", VideoTemplateID: template.ID})
	if err != nil {
		t.Fatal(err)
	}
	if !p.Regions.Intro.Enabled || p.Regions.Outro.Enabled || p.Regions.Intro.Slots[0].Text != "Template" {
		t.Fatal(p.Regions)
	}
	owner := "Owner"
	p, err = s.UpdateProject(t.Context(), "alice", p.ID, clip.ProjectPatch{IntroRegion: &clip.RegionPatch{Slots: []clip.RegionSlotPatch{{ID: p.Regions.Intro.Slots[0].ID, Text: &owner}}}})
	if err != nil {
		t.Fatal(err)
	}
	empty := ""
	p, err = s.UpdateProject(t.Context(), "alice", p.ID, clip.ProjectPatch{VideoTemplateID: &empty})
	if err != nil {
		t.Fatal(err)
	}
	p, err = s.UpdateProject(t.Context(), "alice", p.ID, clip.ProjectPatch{VideoTemplateID: &template.ID})
	if err != nil {
		t.Fatal(err)
	}
	if p.Regions.Intro.Slots[0].Text != owner || !p.Regions.Intro.Slots[0].OwnerFixed {
		t.Fatal("reseed overwrote owner", p.Regions)
	}
}

func TestRegionWritesRefuseBusyAndFinalizedProjects(t *testing.T) {
	h := generationSetup(t)
	h.start(t)
	on := true
	if _, err := h.projects.UpdateProject(t.Context(), "alice", h.project.ID, clip.ProjectPatch{IntroRegion: &clip.RegionPatch{Enabled: &on}}); !errors.Is(err, clip.ErrBusy) {
		t.Fatal("active project edited", err)
	}
	s, _, db := setup(t)
	p, err := s.CreateProject(t.Context(), "alice", clip.ProjectInput{Language: "ko", Title: "Final", Ratio: "vertical"})
	if err != nil {
		t.Fatal(err)
	}
	const at = "2026-09-28T00:00:00Z"
	_, err = db.Writer.Exec(`UPDATE clip_projects SET result_id='result',result_key='result-key',result_content_type='video/mp4',result_bytes=1,result_duration_ms=15000,result_created_at=?,edit_plan_revision=1,rendered_plan_revision=1,finalized_at=?,finalized_plan_revision=1,finalized_result_key='result-key',source_access_revoked_at=? WHERE id=?`, at, at, at, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.UpdateProject(t.Context(), "alice", p.ID, clip.ProjectPatch{IntroRegion: &clip.RegionPatch{Enabled: &on}}); !errors.Is(err, clip.ErrFinalized) {
		t.Fatal("finalized project edited", err)
	}
	read, err := s.GetProject(t.Context(), "alice", p.ID)
	if err != nil || read.Finalized == nil || read.Regions.Intro.Enabled {
		t.Fatal("finalized read failed or enabled a region", err)
	}
	if _, err = db.Writer.Exec(`UPDATE clip_projects SET regions_json=NULL WHERE id=?`, p.ID); err == nil {
		t.Fatal("SQL changed finalized region state")
	}
}
