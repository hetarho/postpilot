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

// The template grammar refuses sections at the service gate, on create and on
// update alike (CLIP-4).
func TestTemplateSaveRefusesSections(t *testing.T) {
	s, _, _ := setup(t)
	ctx := context.Background()
	sections := `<clip version="1"><field id="place" label="장소"/><guide>말투</guide><scene id="exterior" scope="context"><guide>가장 이른 클립으로 시작</guide><text id="c" kind="ai" role="caption" basis="cut">보이는 것 하나</text></scene><text id="empty-hook" kind="fixed" role="hook"/><text id="empty-ending" kind="fixed" role="ending"/></clip>`
	var problem *composition.Problem
	if _, err := s.CreateTemplate(ctx, "alice", clip.Recipe{Name: "sections", CompositionBody: sections}); err == nil || !asProblem(err, &problem) || problem.Reason != "unsupported_section" || problem.ElementID != "exterior" {
		t.Fatalf("section template was saved: %v", err)
	}
	clean := `<clip version="1"><field id="place" label="장소"/><guide>말투</guide><text id="empty-hook" kind="fixed" role="hook"/><text id="empty-ending" kind="fixed" role="ending"/></clip>`
	saved, err := s.CreateTemplate(ctx, "alice", clip.Recipe{Name: "clean", CompositionBody: clean})
	if err != nil || saved.CompositionBody != clean {
		t.Fatal(saved, err)
	}
	if _, err := s.UpdateTemplate(ctx, "alice", saved.ID, clip.TemplatePatch{CompositionBody: &sections}); err == nil || !asProblem(err, &problem) || problem.Reason != "unsupported_section" {
		t.Fatalf("section body accepted on update: %v", err)
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
