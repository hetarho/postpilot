package store_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/postpilot/backend/internal/guideline"
	"github.com/postpilot/backend/internal/guideline/store"
)

type publicationFields struct{}

func (publicationFields) Known(id string) bool { return id == "restaurant" }

type publicationTemplates struct{}

func (publicationTemplates) Templates(_ context.Context, user string) ([]guideline.TemplateRef, error) {
	return []guideline.TemplateRef{{ID: user + "-p1", Name: "First"}, {ID: user + "-p2", Name: "Second"}}, nil
}

func publicationService(s *store.Store, max int) *guideline.Service {
	service := guideline.NewService(s, publicationFields{}, guideline.Limits{TextMaxChars: 300, TitleMaxChars: 40, MaxPerAccount: max}, 50)
	service.SetTemplateDirectory(publicationTemplates{})
	return service
}

func TestAuthoringGuidelineScopeRequiresExplicitNewChoiceAndSharesTheVersionFence(t *testing.T) {
	ctx := context.Background()
	s, handle := newStore(t)
	service := publicationService(s, 5)
	authoring := guideline.NewAuthoring(service, s)
	in := guideline.AuthoringPublication{Key: guideline.AuthoringKey{Key: "new", SessionID: "session", Revision: 1}, Kind: guideline.KindPost, Draft: guideline.AuthoringDraft{Name: "Rule", Body: "Use the owner's supplied facts"}}
	if _, err := authoring.Publish(ctx, "alice", in); !errors.Is(err, guideline.ErrScopeShape) {
		t.Fatalf("implicit new global scope accepted=%v", err)
	}
	in.Scope = &guideline.ScopePatch{Scope: guideline.ScopeTemplates, TemplateIDs: []string{"alice-p1", "alice-p1"}}
	saved, err := authoring.Publish(ctx, "alice", in)
	if err != nil || len(saved.TemplateIDs) != 1 || saved.TemplateIDs[0] != "alice-p1" {
		t.Fatalf("explicit scope=%+v %v", saved, err)
	}
	draft, scope, version, err := authoring.SeedWithScope(ctx, "alice", guideline.KindPost, saved.ID)
	if err != nil || scope.Scope != guideline.ScopeTemplates || len(scope.TemplateIDs) != 1 {
		t.Fatalf("scope seed=%+v %v", scope, err)
	}
	in.Key = guideline.AuthoringKey{Key: "ai", SessionID: "session", Revision: 2}
	in.TargetID, in.TargetVersion, in.Scope = saved.ID, version, nil
	draft.Body = "AI refinement of the saved rule"
	in.Draft = draft
	updated, err := authoring.Publish(ctx, "alice", in)
	if err != nil || updated.Scope != saved.Scope || len(updated.TemplateIDs) != 1 {
		t.Fatalf("AI changed scope=%+v %v", updated, err)
	}
	in.Key = guideline.AuthoringKey{Key: "direct", SessionID: "session", Revision: 3}
	in.TargetVersion = guideline.AuthoringVersion(updated)
	in.Scope = &guideline.ScopePatch{Scope: guideline.ScopeFields, Fields: []string{"restaurant", "restaurant"}}
	updated, err = authoring.Publish(ctx, "alice", in)
	if err != nil || updated.Scope != guideline.ScopeFields || len(updated.Fields) != 1 || len(updated.TemplateIDs) != 0 {
		t.Fatalf("explicit direct scope=%+v %v", updated, err)
	}
	stale := in
	stale.Key = guideline.AuthoringKey{Key: "stale", SessionID: "session", Revision: 4}
	stale.Scope = &guideline.ScopePatch{Scope: guideline.ScopeGlobal}
	if _, err := authoring.Publish(ctx, "alice", stale); !errors.Is(err, guideline.ErrAuthoringConflict) {
		t.Fatalf("stale rescope accepted=%v", err)
	}
	stale.TargetVersion = guideline.AuthoringVersion(updated)
	stale.Scope = &guideline.ScopePatch{Scope: guideline.ScopeTemplates, TemplateIDs: []string{"bob-p1"}}
	if _, err := authoring.Publish(ctx, "alice", stale); !errors.Is(err, guideline.ErrTemplateNotFound) {
		t.Fatalf("foreign scope accepted=%v", err)
	}
	stale.Scope = &guideline.ScopePatch{Scope: guideline.ScopeGlobal}
	stale.Draft.Body = "This text and scope must roll back together"
	if _, err := handle.Writer.Exec(`CREATE TRIGGER fail_scope_receipt BEFORE INSERT ON guideline_authoring_publications BEGIN SELECT RAISE(ABORT,'injected receipt failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := authoring.Publish(ctx, "alice", stale); err == nil {
		t.Fatal("failed receipt accepted")
	}
	current, err := s.Get(ctx, "alice", saved.ID)
	if err != nil || current.Scope != guideline.ScopeFields || len(current.Fields) != 1 || current.Text != updated.Text {
		t.Fatalf("partial text/scope save=%+v %v", current, err)
	}
}

func TestGuidelineWinnerCopiesAreScopedAndIdempotentAfterLaterEditOrDeletion(t *testing.T) {
	ctx := context.Background()
	s, handle := newStore(t)
	service := publicationService(s, 2)
	pub := guideline.NewTestedSettings(service, s)
	frozen, _ := guideline.EncodeTestSnapshot(guideline.TestSnapshot{Kind: guideline.KindPost, Draft: guideline.AuthoringDraft{Name: "Synthetic", Body: "The tested direction"}})
	in := guideline.TestedPublication{UserID: "alice", TestID: "test", WinnerID: "winner", Action: "save_setting", RequestKey: "save", Fingerprint: "frozen", Name: "Winner", FrozenContent: frozen}
	if _, err := pub.PublishTestWinner(ctx, in); !errors.Is(err, guideline.ErrScopeShape) {
		t.Fatalf("implicit scope accepted=%v", err)
	}
	in.Scope, in.ScopeIDs = "templates", []string{"bob-p1"}
	if _, err := pub.PublishTestWinner(ctx, in); !errors.Is(err, guideline.ErrTemplateNotFound) {
		t.Fatalf("foreign scope accepted=%v", err)
	}
	in.ScopeIDs = []string{"alice-p2", "alice-p1", "alice-p2"}
	var wg sync.WaitGroup
	refs := make(chan guideline.TestedPublicationReceipt, 6)
	errs := make(chan error, 6)
	for range 6 {
		wg.Go(func() { receipt, err := pub.PublishTestWinner(ctx, in); refs <- receipt; errs <- err })
	}
	wg.Wait()
	close(refs)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var receipt guideline.TestedPublicationReceipt
	for ref := range refs {
		if receipt.TargetID == "" {
			receipt = ref
		}
		if ref != receipt {
			t.Fatalf("different guideline copies=%+v/%+v", receipt, ref)
		}
	}
	saved, err := s.Get(ctx, "alice", receipt.TargetID)
	if err != nil || saved.Title != in.Name || saved.Text != "The tested direction" || saved.Scope != guideline.ScopeTemplates || len(saved.TemplateIDs) != 2 {
		t.Fatalf("published copy=%+v %v", saved, err)
	}
	global := guideline.ScopePatch{Scope: guideline.ScopeGlobal}
	changed := "Later manual rule"
	if _, err := service.Update(ctx, "alice", saved.ID, guideline.Patch{Text: &changed, Scope: &global}); err != nil {
		t.Fatal(err)
	}
	if replay, err := pub.PublishTestWinner(ctx, in); err != nil || replay != receipt {
		t.Fatalf("replay=%+v %v", replay, err)
	}
	current, _ := s.Get(ctx, "alice", saved.ID)
	if current.Text != changed || current.Scope != guideline.ScopeGlobal {
		t.Fatal("replay undid manual edit")
	}
	if err := service.Delete(ctx, "alice", saved.ID); err != nil {
		t.Fatal(err)
	}
	if replay, err := pub.PublishTestWinner(ctx, in); err != nil || replay != receipt {
		t.Fatalf("deleted winner receipt=%+v %v", replay, err)
	}
	rows, _ := s.List(ctx, "alice", guideline.KindPost)
	if len(rows) != 0 || count(t, handle, `SELECT count(*) FROM guideline_test_publications`) != 1 {
		t.Fatalf("duplicate or resurrected copy=%+v", rows)
	}
}

func TestGuidelineWinnerUseChecksNormalizedScopeAndCurrentVersionAndCopiesKeepCapsAndDedupe(t *testing.T) {
	ctx := context.Background()
	s, handle := newStore(t)
	service := publicationService(s, 1)
	saved, err := service.Create(ctx, "alice", guideline.KindPost, "Existing", "Original tested rule", guideline.ScopePatch{Scope: guideline.ScopeTemplates, TemplateIDs: []string{"alice-p1", "alice-p2"}}, "")
	if err != nil {
		t.Fatal(err)
	}
	frozen, _ := guideline.EncodeTestSnapshot(guideline.TestSnapshot{TargetID: saved.ID, TargetVersion: guideline.AuthoringVersion(saved), Kind: saved.Kind, Draft: guideline.AuthoringDraft{Name: saved.Title, Body: saved.Text}, Scope: guideline.ScopePatch{Scope: saved.Scope, TemplateIDs: saved.TemplateIDs}})
	pub := guideline.NewTestedSettings(service, s)
	in := guideline.TestedPublication{UserID: "alice", TestID: "test", WinnerID: "winner", Action: "use_setting", RequestKey: "use", Fingerprint: "frozen", Scope: "templates", ScopeIDs: []string{"alice-p2", "alice-p1", "alice-p2"}, FrozenContent: frozen}
	if receipt, err := pub.PublishTestWinner(ctx, in); err != nil || receipt.TargetID != saved.ID {
		t.Fatalf("normalized scope use=%+v %v", receipt, err)
	}
	changed := "Changed after the test"
	if _, err := service.Update(ctx, "alice", saved.ID, guideline.Patch{Text: &changed}); err != nil {
		t.Fatal(err)
	}
	stale := in
	stale.TestID, stale.RequestKey = "stale", "stale"
	if _, err := pub.PublishTestWinner(ctx, stale); !errors.Is(err, guideline.ErrTestPublicationConflict) {
		t.Fatalf("changed source used=%v", err)
	}
	copy := stale
	copy.Action, copy.RequestKey, copy.Name, copy.Scope, copy.ScopeIDs = "save_setting", "copy", "New copy", "global", nil
	var capError *guideline.AccountCapError
	if _, err := pub.PublishTestWinner(ctx, copy); !errors.As(err, &capError) {
		t.Fatalf("cap bypassed=%v", err)
	}
	if err := service.Delete(ctx, "alice", saved.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.Exec(`CREATE TRIGGER fail_test_receipt BEFORE INSERT ON guideline_test_publications BEGIN SELECT RAISE(ABORT,'injected receipt failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := pub.PublishTestWinner(ctx, copy); err == nil {
		t.Fatal("failed receipt accepted")
	}
	if count(t, handle, `SELECT count(*) FROM guidelines WHERE user_id='alice'`) != 0 {
		t.Fatal("copy survived receipt rollback")
	}
	if _, err := handle.Writer.Exec(`DROP TRIGGER fail_test_receipt`); err != nil {
		t.Fatal(err)
	}
	if _, err := pub.PublishTestWinner(ctx, copy); err != nil {
		t.Fatal(err)
	}
	pub = guideline.NewTestedSettings(publicationService(s, 3), s)
	duplicate := copy
	duplicate.TestID, duplicate.RequestKey = "duplicate", "duplicate"
	if _, err := pub.PublishTestWinner(ctx, duplicate); !errors.Is(err, guideline.ErrDuplicateText) {
		t.Fatalf("exact duplicate accepted=%v", err)
	}
	duplicate.FrozenContent, _ = guideline.EncodeTestSnapshot(guideline.TestSnapshot{Kind: guideline.KindClip, Draft: guideline.AuthoringDraft{Body: "Video-only direction"}})
	duplicate.Scope, duplicate.ScopeIDs = "fields", []string{"restaurant"}
	if _, err := pub.PublishTestWinner(ctx, duplicate); !errors.Is(err, guideline.ErrScopeShape) {
		t.Fatalf("video fields scope accepted=%v", err)
	}
}
