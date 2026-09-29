package store_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	clipapp "github.com/postpilot/backend/internal/clip/app"
	"github.com/postpilot/backend/internal/clip/composition"

	"github.com/postpilot/backend/internal/auth"
	authstore "github.com/postpilot/backend/internal/auth/store"
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/store"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/db"
)

func setup(t *testing.T) (*clipapp.Service, *store.Store, *db.DB) {
	t.Helper()
	projects, st, d, _ := setupWith(t, fakeSources())
	return projects, st, d
}

// setupWith builds the project service over a migrated database with the source side
// reading objects: the constructor needs both (ARCH-40), so the objects a test wants to
// observe are chosen before the service exists.
func setupWith(t *testing.T, objects clip.ObjectStore) (*clipapp.Service, *store.Store, *db.DB, *clipapp.SourceService) {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "clip.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	if err := db.Migrate(context.Background(), d.Writer); err != nil {
		t.Fatal(err)
	}
	for _, u := range []string{"alice", "bob"} {
		if err := authstore.New(d.Writer, d.Reader).CreateUser(context.Background(), auth.User{ID: u, PasswordHash: "hash", Plan: plan.Free, CreatedAt: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	s := store.New(d.Writer, d.Reader)
	sources := clipapp.NewSourceService(s, objects, clip.DefaultSourceLimits(clip.Environment{SourceBatchTTL: 6 * time.Hour, PutTTL: 10 * time.Minute}))
	return clipapp.NewService(s, clip.DefaultLimits(), sources, nullFinalizer{}), s, d, sources
}

// recipeBody is an outline with one required value, which is all a template
// carries besides its name (CLIP-4).
const recipeBody = `<clip version="1"><field id="place" label="장소" required="true">어디였나요?</field></clip>`

func recipe() clip.Recipe {
	return clip.Recipe{Name: " 여행 ", CompositionBody: recipeBody}
}
func create(t *testing.T, s *clipapp.Service) (clip.VideoTemplate, clip.Project) {
	t.Helper()
	ctx := context.Background()
	v, err := s.CreateTemplate(ctx, "alice", recipe())
	if err != nil {
		t.Fatal(err)
	}
	// A campaign type and the template's one required value: without either
	// the approval gate refuses the quote and the start (CDS-5, CLIP-102).
	p, err := s.CreateProject(ctx, "alice", clip.ProjectInput{Language: "ko", Title: " 여행 기록 ", VideoTemplateID: v.ID, Ratio: "vertical", TargetDurationMS: 30000, Disclosure: "sponsored", CompositionInputs: &clip.CompositionInputs{Values: map[string]string{"place": "서울"}, Items: map[string][]composition.Item{}}})
	if err != nil {
		t.Fatal(err)
	}
	return v, p
}
func TestOwnedLifecyclePresenceAndTemplateDetach(t *testing.T) {
	s, _, d := setup(t)
	ctx := context.Background()
	v, p := create(t, s)
	if len(v.ID) != 32 || len(p.ID) != 32 || v.Name != "여행" || v.CompositionBody != recipeBody || p.Composition == nil || p.Composition.Inputs.Values["place"] != "서울" {
		t.Fatalf("normalized creation: %+v %+v", v, p)
	}
	for _, id := range []string{p.ID, "missing"} {
		if _, err := s.GetProject(ctx, "bob", id); !errors.Is(err, clip.ErrNotFound) {
			t.Fatal(err)
		}
		if err := s.DeleteProject(ctx, "bob", id); !errors.Is(err, clip.ErrNotFound) {
			t.Fatal(err)
		}
	}
	for _, id := range []string{v.ID, "missing"} {
		if _, err := s.UpdateTemplate(ctx, "bob", id, clip.TemplatePatch{}); !errors.Is(err, clip.ErrNotFound) {
			t.Fatal(err)
		}
		if _, err := s.DeleteTemplate(ctx, "bob", id); !errors.Is(err, clip.ErrNotFound) {
			t.Fatal(err)
		}
	}
	if _, err := s.CreateProject(ctx, "bob", clip.ProjectInput{Language: "ko", Title: "foreign", VideoTemplateID: v.ID, Ratio: "square", TargetDurationMS: 15000}); !errors.Is(err, clip.ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := s.CreateTemplate(ctx, "alice", recipe()); !errors.Is(err, clip.ErrDuplicateName) {
		t.Fatal(err)
	}
	if _, err := s.CreateTemplate(ctx, "bob", recipe()); err != nil {
		t.Fatal(err)
	}
	title := "changed"
	ms := 45000
	if _, err := s.UpdateProject(ctx, "alice", p.ID, clip.ProjectPatch{Title: &title}); err != nil {
		t.Fatal(err)
	}
	updated, err := s.UpdateProject(ctx, "alice", p.ID, clip.ProjectPatch{TargetDurationMS: &ms, CompositionInputs: &clip.CompositionInputs{Values: map[string]string{"place": ""}, Items: map[string][]composition.Item{}}})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Title != title || updated.Ratio != "vertical" || updated.TargetDurationMS != ms || updated.Composition == nil || updated.Composition.Inputs.Values["place"] != "" {
		t.Fatalf("partial update: %+v", updated)
	}
	name := "새 이름"
	body := `<clip version="1"/>`
	vt, err := s.UpdateTemplate(ctx, "alice", v.ID, clip.TemplatePatch{Name: &name, CompositionBody: &body})
	if err != nil {
		t.Fatal(err)
	}
	if vt.Name != name || vt.ProjectCount != 1 || vt.CompositionBody != body {
		t.Fatalf("patch: %+v", vt)
	}
	if _, err := d.Writer.Exec("UPDATE clip_projects SET analysis_json='analysis', edit_plan_json='plan', result_key='result', result_content_type='video/mp4', result_bytes=123, result_duration_ms=30000, result_created_at=?, edit_plan_revision=2, rendered_plan_revision=1 WHERE id=?", time.Now().UTC().Format(time.RFC3339Nano), p.ID); err != nil {
		t.Fatal(err)
	}
	before, err := s.GetProject(ctx, "alice", p.ID)
	if err != nil {
		t.Fatal(err)
	}
	count, err := s.DeleteTemplate(ctx, "alice", v.ID)
	if err != nil || count != 1 {
		t.Fatalf("detach %d %v", count, err)
	}
	after, err := s.GetProject(ctx, "alice", p.ID)
	if err != nil {
		t.Fatal(err)
	}
	before.VideoTemplateID = ""
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("detach changed retained state: before=%+v after=%+v", before, after)
	}
	if err := s.DeleteProject(ctx, "alice", p.ID); err != nil {
		t.Fatal(err)
	}
}

// A template row is its name and a readable outline body; a row whose body is
// missing or unreadable is corrupt and fails the read (CLIP-4).
func TestMalformedStoredTemplateFailsRead(t *testing.T) {
	for _, bad := range []any{nil, "<clip", `<clip version="2"/>`} {
		t.Run(fmt.Sprint(bad), func(t *testing.T) {
			s, _, d := setup(t)
			v, _ := create(t, s)
			if _, err := d.Writer.Exec("UPDATE video_templates SET composition_body=? WHERE id=?", bad, v.ID); err != nil {
				t.Fatal(err)
			}
			if _, err := s.ListTemplates(context.Background(), "alice"); err == nil {
				t.Fatal("a malformed body became a usable template")
			}
		})
	}
}
func TestSchemaOwnerConstraintsAndUserCascade(t *testing.T) {
	s, _, d := setup(t)
	v, p := create(t, s)
	if _, err := d.Writer.Exec("UPDATE clip_projects SET user_id='bob' WHERE id=?", p.ID); err == nil {
		t.Fatal("foreign template assignment accepted")
	}
	if _, err := d.Writer.Exec("DELETE FROM users WHERE id='alice'"); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"video_templates", "clip_projects"} {
		var n int
		if err := d.Reader.QueryRow("SELECT count(*) FROM " + table).Scan(&n); err != nil || n != 0 {
			t.Fatalf("%s cascade: %d %v", table, n, err)
		}
	}
	if _, err := s.UpdateTemplate(context.Background(), "alice", v.ID, clip.TemplatePatch{}); !errors.Is(err, clip.ErrNotFound) {
		t.Fatal(err)
	}
}
func TestValidationAndIncompleteAnswers(t *testing.T) {
	s, _, _ := setup(t)
	ctx := context.Background()
	for _, mutate := range []func(*clip.Recipe){
		func(r *clip.Recipe) { r.Name = " " }, func(r *clip.Recipe) { r.Name = strings.Repeat("한", 41) }, func(r *clip.Recipe) { r.CompositionBody = "" },
	} {
		r := recipe()
		mutate(&r)
		if _, err := s.CreateTemplate(ctx, "alice", r); !errors.Is(err, clip.ErrInvalid) {
			t.Fatalf("accepted %+v: %v", r, err)
		}
	}
	var problem *composition.Problem
	for _, body := range []string{`<clip version="1"><field id="place" label="장소"/><field id="place" label="중복"/></clip>`, `<clip version="1"><field id="place" label="` + strings.Repeat("한", 41) + `"/></clip>`} {
		r := recipe()
		r.CompositionBody = body
		if _, err := s.CreateTemplate(ctx, "alice", r); !errors.As(err, &problem) {
			t.Fatalf("accepted an unreadable outline %q: %v", body, err)
		}
	}
	v, _ := create(t, s)
	for _, mutate := range []func(*clip.ProjectInput){func(p *clip.ProjectInput) { p.Title = " " }, func(p *clip.ProjectInput) { p.Title = strings.Repeat("한", 101) }, func(p *clip.ProjectInput) { p.Ratio = "9:16" }, func(p *clip.ProjectInput) { p.TargetDurationMS = 14999 }, func(p *clip.ProjectInput) { p.TargetDurationMS = 60001 }} {
		p := clip.ProjectInput{Language: "ko", Title: "valid", VideoTemplateID: v.ID, Ratio: "square", TargetDurationMS: 15000}
		mutate(&p)
		if _, err := s.CreateProject(ctx, "alice", p); !errors.Is(err, clip.ErrInvalid) {
			t.Fatalf("accepted project %+v: %v", p, err)
		}
	}
	long := clip.ProjectInput{Language: "ko", Title: "valid", VideoTemplateID: v.ID, Ratio: "square", TargetDurationMS: 15000, CompositionInputs: &clip.CompositionInputs{Values: map[string]string{"place": strings.Repeat("한", 501)}, Items: map[string][]composition.Item{}}}
	if _, err := s.CreateProject(ctx, "alice", long); !errors.As(err, &problem) || problem.Reason != "answer_limit" {
		t.Fatalf("accepted a value over its limit: %v", err)
	}
	for _, ratio := range []string{"vertical", "horizontal", "square"} {
		p, err := s.CreateProject(ctx, "alice", clip.ProjectInput{Language: "ko", Title: "incomplete", VideoTemplateID: v.ID, Ratio: ratio, TargetDurationMS: 60000})
		if err != nil {
			t.Fatal(err)
		}
		if err := clip.RequiredAnswers(v, p, clip.DefaultCompositionLimits()); !errors.As(err, &problem) || problem.Reason != "required_binding" {
			t.Fatal("generation gate accepted a blank required value", err)
		}
	}
}

// The disclosure round-trips through the store with presence semantics of its
// own: absent is not a change, and an unknown value is refused before it can
// reach a column.
func TestDisclosureRoundTrip(t *testing.T) {
	s, _, _ := setup(t)
	ctx := context.Background()
	v, p := create(t, s)
	if p.Disclosure != "sponsored" {
		t.Fatalf("create lost the disclosure: %q", p.Disclosure)
	}
	ad := "ad"
	up, err := s.UpdateProject(ctx, "alice", p.ID, clip.ProjectPatch{Disclosure: &ad})
	if err != nil || up.Disclosure != "ad" {
		t.Fatalf("disclosure patch: %+v %v", up, err)
	}
	again, err := s.UpdateProject(ctx, "alice", p.ID, clip.ProjectPatch{})
	if err != nil || again.Disclosure != "ad" {
		t.Fatalf("an absent field changed a column: %+v %v", again, err)
	}
	if _, err := s.UpdateProject(ctx, "alice", p.ID, clip.ProjectPatch{Disclosure: ptr("편집")}); !errors.Is(err, clip.ErrInvalid) {
		t.Fatal("an unknown disclosure was accepted", err)
	}
	// A project may be created without a campaign type; the gate is what
	// refuses one at approval.
	blank, err := s.CreateProject(ctx, "alice", clip.ProjectInput{Language: "ko", Title: "미정", VideoTemplateID: v.ID, Ratio: "square", TargetDurationMS: 15000})
	if err != nil || blank.Disclosure != "" {
		t.Fatalf("%+v %v", blank, err)
	}
	// The same is true of the target duration: the creation screen settles the
	// ratio alone, and the length is chosen in ① beside the sources (CLIP-130).
	unset, err := s.CreateProject(ctx, "alice", clip.ProjectInput{Language: "ko", Title: "길이 미정", VideoTemplateID: v.ID, Ratio: "square"})
	if err != nil || unset.TargetDurationMS != 0 {
		t.Fatalf("%+v %v", unset, err)
	}
	written, err := s.UpdateProject(ctx, "alice", unset.ID, clip.ProjectPatch{TargetDurationMS: ptr(45000)})
	if err != nil || written.TargetDurationMS != 45000 {
		t.Fatalf("%+v %v", written, err)
	}
	if _, err := s.UpdateProject(ctx, "alice", unset.ID, clip.ProjectPatch{TargetDurationMS: ptr(14999)}); !errors.Is(err, clip.ErrInvalid) {
		t.Fatal("a written duration escaped its bounds", err)
	}
	if _, err := s.CreateProject(ctx, "alice", clip.ProjectInput{Language: "ko", Title: "잘못된", VideoTemplateID: v.ID, Ratio: "square", TargetDurationMS: 15000, Disclosure: "편집"}); !errors.Is(err, clip.ErrInvalid) {
		t.Fatal(err)
	}
}

func ptr[T any](value T) *T { return &value }
