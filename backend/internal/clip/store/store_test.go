package store_test

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/auth"
	authstore "github.com/postpilot/backend/internal/auth/store"
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
	"github.com/postpilot/backend/internal/clip/store"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/config"
	"github.com/postpilot/backend/internal/platform/db"
)

func setup(t *testing.T) (*clip.Service, *store.Store, *db.DB) {
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
	return clip.NewService(s, config.ClipLimits()), s, d
}
func recipe() clip.Recipe {
	return clip.Recipe{Name: " 여행 ", Preset: "stay", InformationFields: []clip.InformationField{{Label: " 장소 ", Prompt: " 어디였나요? "}}, CopyStyles: []string{"clean", "memo"}, Accent: "teal"}
}
func create(t *testing.T, s *clip.Service) (clip.VideoTemplate, clip.Project) {
	t.Helper()
	ctx := context.Background()
	v, err := s.CreateTemplate(ctx, "alice", recipe())
	if err != nil {
		t.Fatal(err)
	}
	// A campaign type and two of the four reserved facts: without them the
	// approval gate refuses the quote and the start (CDS-1, CDS-5).
	p, err := s.CreateProject(ctx, "alice", clip.ProjectInput{Title: " 여행 기록 ", VideoTemplateID: v.ID, Ratio: "vertical", TargetDurationMS: 30000, Disclosure: "sponsored", Answers: []clip.Answer{{Label: "장소", Text: "서울"}, {Label: "이전 질문", Text: "보존"}, {Label: "상호", Text: "연남 스테이"}, {Label: "위치", Text: "서울 연남동"}}})
	if err != nil {
		t.Fatal(err)
	}
	return v, p
}
func TestOwnedLifecyclePresenceAndTemplateDetach(t *testing.T) {
	s, _, d := setup(t)
	ctx := context.Background()
	v, p := create(t, s)
	if len(v.ID) != 32 || len(p.ID) != 32 || v.Name != "여행" || v.InformationFields[0].Label != "장소" {
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
	if _, err := s.CreateProject(ctx, "bob", clip.ProjectInput{Title: "foreign", VideoTemplateID: v.ID, Ratio: "square", TargetDurationMS: 15000}); !errors.Is(err, clip.ErrNotFound) {
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
	updated, err := s.UpdateProject(ctx, "alice", p.ID, clip.ProjectPatch{TargetDurationMS: &ms, Answers: []clip.Answer{{Label: "장소", Text: ""}}})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Title != title || updated.Ratio != "vertical" || updated.TargetDurationMS != ms || len(updated.Answers) != 4 {
		t.Fatalf("partial update: %+v", updated)
	}
	name := "새 이름"
	empty := []clip.InformationField{}
	neutral := ""
	vt, err := s.UpdateTemplate(ctx, "alice", v.ID, clip.TemplatePatch{Name: &name, InformationFields: &empty, Accent: &neutral})
	if err != nil {
		t.Fatal(err)
	}
	if vt.Name != name || vt.ProjectCount != 1 || len(vt.InformationFields) != 0 || len(vt.CopyStyles) != 2 || vt.Accent != "" {
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
	var n int
	if err := d.Reader.QueryRow("SELECT count(*) FROM clip_project_answers").Scan(&n); err != nil || n != 0 {
		t.Fatalf("answer cascade: %d %v", n, err)
	}
}
func TestMalformedStoredRecipeFailsRead(t *testing.T) {
	for _, bad := range []string{"{", "null", "{}", `[{}]`, `[{"label":"x","prompt":"y","extra":1}]`, `[] []`} {
		t.Run(bad, func(t *testing.T) {
			s, _, d := setup(t)
			v, _ := create(t, s)
			if _, err := d.Writer.Exec("UPDATE video_templates SET information_fields=? WHERE id=?", bad, v.ID); err != nil {
				t.Fatal(err)
			}
			if _, err := s.ListTemplates(context.Background(), "alice"); err == nil {
				t.Fatal("malformed JSON became a usable recipe")
			}
		})
	}
	s, _, d := setup(t)
	v, _ := create(t, s)
	if _, err := d.Writer.Exec("UPDATE video_templates SET copy_styles='null' WHERE id=?", v.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ListTemplates(context.Background(), "alice"); err == nil {
		t.Fatal("null styles accepted")
	}
}
func TestSchemaOwnerConstraintsAndUserCascade(t *testing.T) {
	s, _, d := setup(t)
	v, p := create(t, s)
	if _, err := d.Writer.Exec("UPDATE clip_projects SET user_id='bob' WHERE id=?", p.ID); err == nil {
		t.Fatal("foreign template assignment accepted")
	}
	if _, err := d.Writer.Exec("UPDATE clip_project_answers SET user_id='bob' WHERE project_id=?", p.ID); err == nil {
		t.Fatal("foreign answer ownership accepted")
	}
	if _, err := d.Writer.Exec("DELETE FROM users WHERE id='alice'"); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"video_templates", "clip_projects", "clip_project_answers"} {
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
		func(r *clip.Recipe) { r.Name = " " }, func(r *clip.Recipe) { r.Name = strings.Repeat("한", 41) }, func(r *clip.Recipe) { r.CutGuidance = strings.Repeat("한", 4001) },
		func(r *clip.Recipe) { r.CopyStyles = nil }, func(r *clip.Recipe) { r.CopyStyles = []string{"unknown"} }, func(r *clip.Recipe) { r.CopyStyles = []string{"clean", "clean"} }, func(r *clip.Recipe) { r.Accent = "#123456" },
		func(r *clip.Recipe) {
			r.InformationFields = append(r.InformationFields, clip.InformationField{Label: "장소", Prompt: "duplicate"})
		}, func(r *clip.Recipe) { r.InformationFields[0].Label = strings.Repeat("한", 41) }, func(r *clip.Recipe) { r.InformationFields[0].Prompt = strings.Repeat("한", 201) }, func(r *clip.Recipe) { r.InformationFields = make([]clip.InformationField, 11) },
	} {
		r := recipe()
		mutate(&r)
		if _, err := s.CreateTemplate(ctx, "alice", r); !errors.Is(err, clip.ErrInvalid) {
			t.Fatalf("accepted %+v: %v", r, err)
		}
	}
	v, _ := create(t, s)
	for _, mutate := range []func(*clip.ProjectInput){func(p *clip.ProjectInput) { p.Title = " " }, func(p *clip.ProjectInput) { p.Title = strings.Repeat("한", 101) }, func(p *clip.ProjectInput) { p.Ratio = "9:16" }, func(p *clip.ProjectInput) { p.TargetDurationMS = 14999 }, func(p *clip.ProjectInput) { p.TargetDurationMS = 90001 }, func(p *clip.ProjectInput) {
		p.Answers = []clip.Answer{{Label: "장소", Text: strings.Repeat("한", 501)}}
	}} {
		p := clip.ProjectInput{Title: "valid", VideoTemplateID: v.ID, Ratio: "square", TargetDurationMS: 15000}
		mutate(&p)
		if _, err := s.CreateProject(ctx, "alice", p); !errors.Is(err, clip.ErrInvalid) {
			t.Fatalf("accepted project %+v: %v", p, err)
		}
	}
	for _, ratio := range []string{"vertical", "horizontal", "square"} {
		p, err := s.CreateProject(ctx, "alice", clip.ProjectInput{Title: "incomplete", VideoTemplateID: v.ID, Ratio: ratio, TargetDurationMS: 90000})
		if err != nil {
			t.Fatal(err)
		}
		if err := clip.RequiredAnswers(v, p); !errors.Is(err, clip.ErrInvalid) {
			t.Fatal("generation gate accepted blank answers")
		}
	}
}

// The three columns 0042 added round-trip through the store, and each patch
// carries presence semantics of its own.
func TestPresetDisclosureAndCTARoundTrip(t *testing.T) {
	s, _, _ := setup(t)
	ctx := context.Background()
	v, p := create(t, s)
	if v.Preset != "stay" || p.Disclosure != "sponsored" || p.CTA != "" {
		t.Fatalf("create lost a column: %q %q %q", v.Preset, p.Disclosure, p.CTA)
	}
	// Reading is enough to know the CTA falls back to the preset's.
	if design.DefaultCTA(v.Preset, p.CTA) != "blog" {
		t.Fatal("stay's default CTA")
	}
	beauty, ad, save := "beauty", "ad", "save"
	vt, err := s.UpdateTemplate(ctx, "alice", v.ID, clip.TemplatePatch{Preset: &beauty})
	if err != nil || vt.Preset != "beauty" || vt.Name != "여행" {
		t.Fatalf("preset patch: %+v %v", vt, err)
	}
	up, err := s.UpdateProject(ctx, "alice", p.ID, clip.ProjectPatch{Disclosure: &ad, CTA: &save})
	if err != nil || up.Disclosure != "ad" || up.CTA != "save" {
		t.Fatalf("disclosure patch: %+v %v", up, err)
	}
	// An absent field is not a change, and an unknown value is refused before
	// it can reach a column.
	again, err := s.UpdateProject(ctx, "alice", p.ID, clip.ProjectPatch{})
	if err != nil || again.Disclosure != "ad" || again.CTA != "save" {
		t.Fatalf("absent fields changed a column: %+v %v", again, err)
	}
	for _, patch := range []clip.ProjectPatch{{Disclosure: ptr("편집")}, {Disclosure: ptr("")}, {CTA: ptr("subscribe")}} {
		if _, err := s.UpdateProject(ctx, "alice", p.ID, patch); !errors.Is(err, clip.ErrInvalid) {
			t.Fatalf("invalid patch accepted: %+v %v", patch, err)
		}
	}
	for _, patch := range []clip.TemplatePatch{{Preset: ptr("")}, {Preset: ptr("bakery")}} {
		if _, err := s.UpdateTemplate(ctx, "alice", v.ID, patch); !errors.Is(err, clip.ErrInvalid) {
			t.Fatalf("invalid preset accepted: %+v %v", patch, err)
		}
	}
	// A template must name a preset at creation (CDS-50).
	uncategorised := recipe()
	uncategorised.Name, uncategorised.Preset = "무분류", ""
	if _, err := s.CreateTemplate(ctx, "alice", uncategorised); !errors.Is(err, clip.ErrInvalid) {
		t.Fatal(err)
	}
	// A project may be created without a campaign type; the gate is what
	// refuses one at approval.
	blank, err := s.CreateProject(ctx, "alice", clip.ProjectInput{Title: "미정", VideoTemplateID: v.ID, Ratio: "square", TargetDurationMS: 15000})
	if err != nil || blank.Disclosure != "" {
		t.Fatalf("%+v %v", blank, err)
	}
	if _, err := s.CreateProject(ctx, "alice", clip.ProjectInput{Title: "잘못된", VideoTemplateID: v.ID, Ratio: "square", TargetDurationMS: 15000, Disclosure: "편집"}); !errors.Is(err, clip.ErrInvalid) {
		t.Fatal(err)
	}
}

func ptr[T any](value T) *T { return &value }
