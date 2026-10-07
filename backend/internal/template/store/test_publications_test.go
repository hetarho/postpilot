package store_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/postpilot/backend/internal/template"
	"github.com/postpilot/backend/internal/template/store"
)

func publicationService(s *store.Store, max int) *template.Service {
	return template.NewService(s, template.NewLimits(template.Ceilings{
		NameMaxChars: 40, DescriptionMaxChars: 200, BodyMaxChars: 4000, TitleAreaMaxChars: 200,
		MaxPerAccount: max, PhotoRowMax: 3, AskLabelMaxChars: 40, AskMaxPerBody: 10,
	}, template.NumberBounds{TargetLengthMin: 100, TargetLengthMax: 10000, TagCountMin: 1, TagCountMax: 50}))
}
func TestTemplateWinnerChoiceValidationUsesOwnedNameLimitsBeforeAnyReceiptOrTarget(t *testing.T) {
	s, handle := newStore(t)
	pub := template.NewTestedSettings(publicationService(s, 5), s)
	ctx := context.Background()
	for _, in := range []template.TestedPublication{{Action: "save_setting", Name: ""}, {Action: "save_setting", Name: strings.Repeat("명", 41)}, {Action: "save_setting", Name: "Valid", MakeDefault: true}, {Action: "save_setting", Name: "Valid", Scope: "global"}} {
		if err := pub.ValidateTestPublicationChoices(ctx, in); err == nil {
			t.Fatalf("invalid initial choice accepted=%+v", in)
		}
	}
	if err := pub.ValidateTestPublicationChoices(ctx, template.TestedPublication{Action: "save_setting", Name: "Corrected copy"}); err != nil {
		t.Fatal(err)
	}
	var receipts int
	if err := handle.Reader.QueryRow("SELECT count(*) FROM template_test_publications").Scan(&receipts); err != nil || receipts != 0 {
		t.Fatalf("preflight wrote receipts=%d err=%v", receipts, err)
	}
}

func TestAuthoringNumbersAreExplicitOwnerEditsUnderTheSameVersionFence(t *testing.T) {
	ctx := context.Background()
	s, handle := newStore(t)
	service := publicationService(s, 5)
	length, tags := 1234, 6
	saved, err := service.Create(ctx, "alice", template.Authored{Name: "Saved", Body: body, Numbers: template.Numbers{TargetLength: &length, TagCount: &tags}})
	if err != nil {
		t.Fatal(err)
	}
	authoring := template.NewAuthoring(service, s)
	draft, numbers, version, err := authoring.SeedWithNumbers(ctx, "alice", saved.ID)
	if err != nil || *numbers.TargetLength != length || *numbers.TagCount != tags {
		t.Fatalf("seed=%+v %v", numbers, err)
	}
	draft.Body = "<write>AI changed structure</write>"
	in := template.AuthoringPublication{Key: template.AuthoringKey{Key: "ai", SessionID: "session", Revision: 1}, TargetID: saved.ID, TargetVersion: version, Draft: draft}
	updated, err := authoring.Publish(ctx, "alice", in)
	if err != nil || *updated.TargetLength != length || *updated.TagCount != tags {
		t.Fatalf("AI changed numbers=%+v err=%v", updated, err)
	}
	// A deliberate direct edit can replace or clear both numbers without bypassing CAS.
	newLength := 4321
	in.Key = template.AuthoringKey{Key: "direct", SessionID: "session", Revision: 2}
	in.TargetVersion = template.AuthoringVersion(updated)
	in.Numbers = &template.Numbers{TargetLength: &newLength}
	updated, err = authoring.Publish(ctx, "alice", in)
	if err != nil || *updated.TargetLength != newLength || updated.TagCount != nil {
		t.Fatalf("owner numbers=%+v err=%v", updated, err)
	}
	stale := in
	stale.Key = template.AuthoringKey{Key: "stale", SessionID: "session", Revision: 3}
	if _, err := authoring.Publish(ctx, "alice", stale); !errors.Is(err, template.ErrAuthoringConflict) {
		t.Fatalf("stale numbers accepted: %v", err)
	}
	badLength := 99
	bad := in
	bad.Key = template.AuthoringKey{Key: "bad", SessionID: "session", Revision: 3}
	bad.TargetVersion = template.AuthoringVersion(updated)
	bad.Numbers = &template.Numbers{TargetLength: &badLength}
	var outOfRange *template.NumberOutOfRangeError
	if _, err := authoring.Publish(ctx, "alice", bad); !errors.As(err, &outOfRange) {
		t.Fatalf("invalid numbers accepted: %v", err)
	}
	if _, err := handle.Writer.Exec(`CREATE TRIGGER fail_number_receipt BEFORE INSERT ON template_authoring_publications BEGIN SELECT RAISE(ABORT,'injected receipt failure'); END`); err != nil {
		t.Fatal(err)
	}
	bad.Numbers = &template.Numbers{TagCount: &tags}
	if _, err := authoring.Publish(ctx, "alice", bad); err == nil {
		t.Fatal("receipt failure accepted")
	}
	current, err := s.Get(ctx, "alice", saved.ID)
	if err != nil || *current.TargetLength != newLength || current.TagCount != nil || current.Body != updated.Body {
		t.Fatalf("partial number save=%+v err=%v", current, err)
	}
}

func TestTemplateWinnerCopiesCommitOnceAndReceiptSurvivesLaterEditsAndDeletion(t *testing.T) {
	ctx := context.Background()
	s, handle := newStore(t)
	service := publicationService(s, 2)
	pub := template.NewTestedSettings(service, s)
	length, tags := 2400, 7
	frozen, err := template.EncodeTestSnapshot(template.TestSnapshot{Draft: template.Draft{Name: "Frozen", Body: "<write>Tested body</write>", TitleArea: "<write>Tested title</write>"}, Numbers: template.Numbers{TargetLength: &length, TagCount: &tags}})
	if err != nil {
		t.Fatal(err)
	}
	in := template.TestedPublication{UserID: "alice", TestID: "test", WinnerID: "winner", Action: "save_setting", RequestKey: "save", Fingerprint: "frozen", Name: "Winner copy", FrozenContent: frozen}
	var wg sync.WaitGroup
	refs := make(chan template.TestedPublicationReceipt, 8)
	errs := make(chan error, 8)
	for range 8 {
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
	var receipt template.TestedPublicationReceipt
	for ref := range refs {
		if receipt.TargetID == "" {
			receipt = ref
		}
		if ref != receipt {
			t.Fatalf("different copies=%+v/%+v", ref, receipt)
		}
	}
	saved, err := s.Get(ctx, "alice", receipt.TargetID)
	if err != nil || saved.Name != in.Name || saved.Body != "<write>Tested body</write>" || *saved.TargetLength != length || *saved.TagCount != tags {
		t.Fatalf("frozen copy=%+v err=%v", saved, err)
	}
	name := "Later manual edit"
	if _, err := service.Update(ctx, "alice", saved.ID, template.Patch{Name: &name}); err != nil {
		t.Fatal(err)
	}
	if replay, err := pub.PublishTestWinner(ctx, in); err != nil || replay != receipt {
		t.Fatalf("later edit replay=%+v %v", replay, err)
	}
	current, _ := s.Get(ctx, "alice", saved.ID)
	if current.Name != name {
		t.Fatal("replay undid manual edit")
	}
	if _, err := service.Delete(ctx, "alice", saved.ID); err != nil {
		t.Fatal(err)
	}
	if replay, err := pub.PublishTestWinner(ctx, in); err != nil || replay != receipt {
		t.Fatalf("deleted copy replay=%+v %v", replay, err)
	}
	if retained, found, err := pub.ReadTestPublicationReceipt(ctx, "alice", "test", "winner", "save_setting"); err != nil || !found || retained != receipt {
		t.Fatalf("payload-free receipt=%+v found=%v err=%v", retained, found, err)
	}
	for _, user := range []string{"bob", "missing"} {
		if _, found, err := pub.ReadTestPublicationReceipt(ctx, user, "test", "winner", "save_setting"); err != nil || found {
			t.Fatalf("foreign receipt user=%s found=%v err=%v", user, found, err)
		}
	}
	if _, found, err := pub.ReadTestPublicationReceipt(ctx, "alice", "test", "winner", "use_setting"); err != nil || found {
		t.Fatalf("different action receipt found=%v err=%v", found, err)
	}
	rows, _ := s.List(ctx, "alice")
	if len(rows) != 0 {
		t.Fatal("replay resurrected deleted winner")
	}
	var receipts int
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM template_test_publications`).Scan(&receipts); err != nil || receipts != 1 {
		t.Fatalf("receipt count=%d %v", receipts, err)
	}
	conflict := in
	conflict.TestID = "other"
	if _, err := pub.PublishTestWinner(ctx, conflict); !errors.Is(err, template.ErrTestPublicationConflict) {
		t.Fatalf("reused request key=%v", err)
	}
}

func TestTemplateWinnerUseRequiresCurrentOwnedVersionAndNewCopiesKeepDomainRules(t *testing.T) {
	ctx := context.Background()
	s, handle := newStore(t)
	service := publicationService(s, 1)
	saved, err := service.Create(ctx, "alice", template.Authored{Name: "Original", Body: body})
	if err != nil {
		t.Fatal(err)
	}
	frozen, _ := template.EncodeTestSnapshot(template.TestSnapshot{TargetID: saved.ID, TargetVersion: template.AuthoringVersion(saved), Draft: template.Draft{Name: saved.Name, Body: saved.Body}})
	pub := template.NewTestedSettings(service, s)
	in := template.TestedPublication{UserID: "alice", TestID: "test", WinnerID: "winner", Action: "use_setting", RequestKey: "use", Fingerprint: "frozen", FrozenContent: frozen}
	if receipt, err := pub.PublishTestWinner(ctx, in); err != nil || receipt.TargetID != saved.ID {
		t.Fatalf("use existing=%+v %v", receipt, err)
	}
	laterName := "Changed"
	if _, err := service.Update(ctx, "alice", saved.ID, template.Patch{Name: &laterName}); err != nil {
		t.Fatal(err)
	}
	stale := in
	stale.TestID, stale.RequestKey = "stale-test", "stale-use"
	if _, err := pub.PublishTestWinner(ctx, stale); !errors.Is(err, template.ErrTestPublicationConflict) {
		t.Fatalf("changed source used=%v", err)
	}
	copy := stale
	copy.Action, copy.RequestKey, copy.Name = "save_setting", "copy", "New copy"
	if _, err := pub.PublishTestWinner(ctx, copy); !errors.Is(err, template.ErrTooMany) {
		t.Fatalf("cap bypassed=%v", err)
	}
	if _, err := service.Delete(ctx, "alice", saved.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.Exec(`CREATE TRIGGER fail_test_receipt BEFORE INSERT ON template_test_publications BEGIN SELECT RAISE(ABORT,'injected receipt failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := pub.PublishTestWinner(ctx, copy); err == nil {
		t.Fatal("failed receipt accepted")
	}
	rows, _ := s.List(ctx, "alice")
	if len(rows) != 0 {
		t.Fatal("copy survived receipt rollback")
	}
	if _, err := handle.Writer.Exec(`DROP TRIGGER fail_test_receipt`); err != nil {
		t.Fatal(err)
	}
	if _, err := pub.PublishTestWinner(ctx, copy); err != nil {
		t.Fatal(err)
	}
	duplicate := copy
	duplicate.TestID, duplicate.RequestKey = "duplicate", "duplicate"
	// Expand the cap to exercise uniqueness independently.
	pub = template.NewTestedSettings(publicationService(s, 3), s)
	if _, err := pub.PublishTestWinner(ctx, duplicate); !errors.Is(err, template.ErrDuplicateName) {
		t.Fatalf("duplicate name accepted=%v", err)
	}
	badNumber := 99
	duplicate.RequestKey, duplicate.Name = "invalid", "Invalid"
	duplicate.FrozenContent, _ = template.EncodeTestSnapshot(template.TestSnapshot{Draft: template.Draft{Name: "Invalid", Body: body}, Numbers: template.Numbers{TargetLength: &badNumber}})
	var numberError *template.NumberOutOfRangeError
	if _, err := pub.PublishTestWinner(ctx, duplicate); !errors.As(err, &numberError) {
		t.Fatalf("invalid frozen number accepted=%v", err)
	}
	duplicate.FrozenContent, _ = template.EncodeTestSnapshot(template.TestSnapshot{Draft: template.Draft{Name: "Invalid", Body: "<unknown/>"}})
	if _, err := pub.PublishTestWinner(ctx, duplicate); err == nil {
		t.Fatal("invalid frozen shape accepted")
	}
}
