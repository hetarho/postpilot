package store_test

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/clip/store"
	"github.com/postpilot/backend/internal/platform/config"
)

// legacyTemplate stores a body under the section grammar the template editor no
// longer accepts (CLIP-140): it enters through the store, past the service's
// ParseTemplate gate, the way a template saved before the change sits in the
// database. Projects made from it freeze that body and keep rendering it.
func legacyTemplate(t *testing.T, st *store.Store, user, name, body string) clip.VideoTemplate {
	t.Helper()
	now := time.Now()
	template := clip.VideoTemplate{ID: "legacy-" + strings.ReplaceAll(name, " ", "-") + "-" + strconv.FormatInt(now.UnixNano(), 36), UserID: user, Recipe: clip.Recipe{Name: name, CompositionBody: body}, CreatedAt: now, UpdatedAt: now}
	if err := st.InsertTemplate(context.Background(), template); err != nil {
		t.Fatal(err)
	}
	return template
}

// The template grammar refuses sections at the service gate, while a legacy body
// already in the store is read back converted and flagged — and only there: the
// stored body, which generation freezes, stays as it was (CLIP-4, CLIP-140).
func TestTemplateSaveRefusesSectionsWhileStoredLegacyBodiesReadConverted(t *testing.T) {
	s, st, _ := setup(t)
	ctx := context.Background()
	sections := `<clip version="1" intro="b" caption="bold" outro="e"><field id="place" label="장소"/><guide>말투</guide><scene id="exterior" scope="context"><guide>가장 이른 클립으로 시작</guide><text id="c" kind="ai" role="caption" basis="cut">보이는 것 하나</text></scene><text id="empty-hook" kind="fixed" role="hook" basis="output-start"/><text id="empty-ending" kind="fixed" role="ending" basis="output-end"/></clip>`
	var problem *composition.Problem
	if _, err := s.CreateTemplate(ctx, "alice", clip.Recipe{Name: "sections", CompositionBody: sections}); err == nil || !asProblem(err, &problem) || problem.Reason != "unsupported_section" || problem.ElementID != "exterior" {
		t.Fatalf("section template was saved: %v", err)
	}
	clean := `<clip version="1" intro="b" caption="bold" outro="e"><field id="place" label="장소"/><guide>말투</guide><text id="empty-hook" kind="fixed" role="hook" basis="output-start"/><text id="empty-ending" kind="fixed" role="ending" basis="output-end"/></clip>`
	saved, err := s.CreateTemplate(ctx, "alice", clip.Recipe{Name: "clean", CompositionBody: clean})
	if err != nil || saved.CompositionConverted {
		t.Fatal(saved, err)
	}
	if _, err := s.UpdateTemplate(ctx, "alice", saved.ID, clip.TemplatePatch{CompositionBody: &sections}); err == nil || !asProblem(err, &problem) || problem.Reason != "unsupported_section" {
		t.Fatalf("section body accepted on update: %v", err)
	}
	if projection := s.TemplateProjection(saved); projection.CompositionConverted || projection.CompositionBody != clean {
		t.Fatalf("conforming body was converted: %+v", projection)
	}

	legacy := legacyTemplate(t, st, "alice", "legacy", sections)
	stored, err := st.GetTemplate(ctx, "alice", legacy.ID)
	if err != nil || stored.CompositionBody != sections || stored.CompositionConverted {
		t.Fatalf("stored body changed: %+v %v", stored, err)
	}
	projection := s.TemplateProjection(stored)
	if !projection.CompositionConverted || strings.Contains(projection.CompositionBody, "<scene") || !strings.Contains(projection.CompositionBody, "가장 이른 클립으로 시작") || !strings.Contains(projection.CompositionBody, "c [ai]: 보이는 것 하나") {
		t.Fatalf("projection not converted: %+v", projection)
	}
	if _, problem := composition.ParseTemplate(projection.CompositionBody, config.ClipCompositionLimits()); problem != nil {
		t.Fatalf("converted projection refused: %+v", problem)
	}
	// A project frozen from the legacy template carries the stored sections, not
	// the projection; that is what keeps existing plans rendering as before.
	p, err := s.CreateProject(ctx, "alice", clip.ProjectInput{Language: "ko", Title: "frozen", VideoTemplateID: legacy.ID, Ratio: "vertical", TargetDurationMS: 15000})
	if err != nil || p.Composition == nil || p.Composition.Snapshot.Body != sections {
		t.Fatalf("frozen snapshot lost its sections: %+v %v", p.Composition, err)
	}
	again, err := st.GetTemplate(ctx, "alice", legacy.ID)
	if err != nil || again.CompositionBody != sections {
		t.Fatal("reading converted the stored body", err)
	}
}

func asProblem(err error, target **composition.Problem) bool {
	for e := err; e != nil; {
		if p, ok := e.(*composition.Problem); ok {
			*target = p
			return true
		}
		u, ok := e.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		e = u.Unwrap()
	}
	return false
}
