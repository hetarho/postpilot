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
	sections := `<clip version="1"><field id="place" label="장소"/><guide>말투</guide><scene id="exterior" scope="context"><guide>가장 이른 클립으로 시작</guide><text id="c" kind="ai" role="caption" basis="cut">보이는 것 하나</text></scene><text id="empty-hook" kind="fixed" role="hook"/><text id="empty-ending" kind="fixed" role="ending"/></clip>`
	var problem *composition.Problem
	if _, err := s.CreateTemplate(ctx, "alice", clip.Recipe{Name: "sections", CompositionBody: sections}); err == nil || !asProblem(err, &problem) || problem.Reason != "unsupported_section" || problem.ElementID != "exterior" {
		t.Fatalf("section template was saved: %v", err)
	}
	clean := `<clip version="1"><field id="place" label="장소"/><guide>말투</guide><text id="empty-hook" kind="fixed" role="hook"/><text id="empty-ending" kind="fixed" role="ending"/></clip>`
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

// A template saved under r23's design-first grammar opens as an outline: its
// stored design attributes and authored intervals are ignored rather than
// refused, the next save writes it without them, and the projects already
// frozen from it are untouched (CLIP-114, CLIP-140, CLIP-144).
func TestDesignFirstTemplateReadsAsAnOutlineAndSavesWithoutItsDesign(t *testing.T) {
	s, st, _ := setup(t)
	ctx := context.Background()
	body := `<clip version="1" intro="a" caption="bold" outro="b" accent="teal" pace="rapid">` +
		`<field id="place" label="상호" required="true">가게 이름</field>` +
		`<text id="badge" kind="fixed" role="badge" position="header" basis="whole">직접 작성</text>` +
		`<text id="intro" kind="fixed" role="hook" basis="output-start" start="0" end="2.5"><row><value field="place"/></row><row>다녀왔어요</row></text>` +
		`<text id="outro" kind="fixed" role="ending" basis="output-end" start="-3" end="0"><row>또 갈래요</row></text></clip>`
	legacy := legacyTemplate(t, st, "alice", "design first", body)
	stored, err := st.GetTemplate(ctx, "alice", legacy.ID)
	if err != nil || stored.CompositionBody != body {
		t.Fatal("the stored body changed on read", err)
	}
	projection := s.TemplateProjection(stored)
	if !projection.CompositionConverted {
		t.Fatal("the design-first body was not carried onto the current grammar")
	}
	for _, gone := range []string{`intro="a"`, `caption="bold"`, `outro="b"`, `basis="`, `start="`, `end="`} {
		if strings.Contains(projection.CompositionBody, gone) {
			t.Fatalf("the projection kept %s:\n%s", gone, projection.CompositionBody)
		}
	}
	doc, problem := composition.ParseTemplate(projection.CompositionBody, config.ClipCompositionLimits())
	if problem != nil {
		t.Fatalf("the projection is not savable: %+v", problem)
	}
	ids := []string{}
	for _, entry := range doc.Outline {
		ids = append(ids, doc.Elements[entry.Index].ID)
	}
	if strings.Join(ids, " ") != "badge intro outro" {
		t.Fatal("the entries lost their declared order", ids)
	}
	if doc.Accent != "teal" || doc.Pace != "rapid" {
		t.Fatal("the root lost what it still carries", doc.Accent, doc.Pace)
	}
	if rows := doc.Elements[1].Rows; len(rows) != 2 || rows[1].Parts[0].Literal != "다녀왔어요" {
		t.Fatal("a region line was dropped by the read", rows)
	}
	// The owner's next save keeps it exactly as the projection wrote it.
	saved, err := s.UpdateTemplate(ctx, "alice", legacy.ID, clip.TemplatePatch{CompositionBody: &projection.CompositionBody})
	if err != nil || saved.CompositionBody != projection.CompositionBody {
		t.Fatal("the converted body was refused on save", err)
	}
	// A project made from it takes none of the design the body used to name.
	p, err := s.CreateProject(ctx, "alice", clip.ProjectInput{Language: "ko", Title: "frozen", VideoTemplateID: legacy.ID, Ratio: "vertical", TargetDurationMS: 15000})
	if err != nil {
		t.Fatal(err)
	}
	if p.IntroPreset != "" || p.OutroPreset != "" || len(p.CaptionStyles) != 0 || p.CaptionPace != "" || p.Accent != "" {
		t.Fatal("the template seeded the project's design", p.IntroPreset, p.OutroPreset, p.CaptionStyles, p.CaptionPace, p.Accent)
	}
	if p.Composition == nil || p.Composition.Snapshot.Body != saved.CompositionBody {
		t.Fatal("the frozen snapshot is not the body the template now holds", p.Composition)
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
