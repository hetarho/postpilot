package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/postpilot/backend/internal/authoring"
	"github.com/postpilot/backend/internal/llm"
)

func currentCandidateRef(s authoring.Session) authoring.OwnedCandidateRef {
	return authoring.OwnedCandidateRef{SessionID: s.ID, CandidateID: s.WorkingSource.ID, Revision: s.WorkingSource.Revision}
}

func TestCurrentDirectCandidateFreezesExactOwnedRevisionWithoutModelOrPublicationWork(t *testing.T) {
	h := fixture(t)
	ctx := context.Background()
	s, err := h.svc.Create(ctx, "alice", authoring.PostTemplate, "owned", "capture-current")
	if err != nil {
		t.Fatal(err)
	}
	source := *s.WorkingSource
	length, tags := "1800", "7"
	source.Body = "direct composition"
	source.TargetLength, source.TagCount = &length, &tags
	s, err = h.svc.PatchDraft(ctx, authoring.DraftMutation{UserID: "alice", SessionID: s.ID, ExpectedRevision: s.Revision, OperationKey: "direct", WorkingSource: source})
	if err != nil {
		t.Fatal(err)
	}
	before := s
	ref := currentCandidateRef(s)
	seeds := h.targets.seeds
	frozen, err := h.svc.FreezeCandidate(ctx, "alice", ref)
	if err != nil || frozen.Artifact.Body != source.Body || frozen.Artifact.Revision != ref.Revision || frozen.TargetID != "owned" || frozen.TargetVersion != "version-one" {
		t.Fatalf("frozen=%+v err=%v", frozen, err)
	}
	if _, err = h.svc.FreezeCandidate(ctx, "bob", ref); !errors.Is(err, authoring.ErrNotFound) {
		t.Fatalf("foreign: %v", err)
	}
	for _, missing := range []authoring.OwnedCandidateRef{{SessionID: s.ID, CandidateID: ref.CandidateID, Revision: 0}, {SessionID: s.ID, CandidateID: ref.CandidateID, Revision: ref.Revision + 1}, {SessionID: s.ID, CandidateID: "unknown", Revision: ref.Revision}} {
		if _, err = h.svc.FreezeCandidate(ctx, "alice", missing); !errors.Is(err, authoring.ErrNotFound) {
			t.Fatalf("missing=%+v err=%v", missing, err)
		}
	}
	unchanged, err := h.svc.Get(ctx, "alice", s.ID)
	if err != nil || !reflect.DeepEqual(unchanged, before) {
		t.Fatal("freeze mutated current work", err)
	}
	later := patch(t, h, s, "later", "later composition")
	if frozen.Artifact.Body != source.Body || *frozen.Artifact.TargetLength != "1800" || later.WorkingSource.Body == frozen.Artifact.Body {
		t.Fatal("later work replaced frozen bytes")
	}
	if _, err = h.svc.FreezeCandidate(ctx, "alice", ref); !errors.Is(err, authoring.ErrNotFound) {
		t.Fatalf("stale working revision survived: %v", err)
	}
	if h.models.calls != 0 || h.jobs.enqueues != 0 || h.targets.creates != 0 || h.targets.seeds != seeds {
		t.Fatal("freezing issued, published, or rebound work")
	}
}

func TestCurrentGuidelineCandidateAllowsItsDomainOptionalTitle(t *testing.T) {
	h := fixture(t)
	ctx := context.Background()
	s, err := h.svc.Create(ctx, "alice", authoring.PostGuideline, "", "untitled")
	if err != nil {
		t.Fatal(err)
	}
	source := *s.WorkingSource
	source.Body, source.Name = "실제 메모의 사실만 사용하기", ""
	s, err = h.svc.PatchDraft(ctx, authoring.DraftMutation{UserID: "alice", SessionID: s.ID, ExpectedRevision: s.Revision, OperationKey: "untitled-source", WorkingSource: source})
	if err != nil {
		t.Fatal(err)
	}
	frozen, err := h.svc.FreezeCandidate(ctx, "alice", currentCandidateRef(s))
	if err != nil || frozen.Artifact.Name != "" || frozen.Artifact.Body != source.Body {
		t.Fatalf("optional title refused: %+v %v", frozen, err)
	}
}

func TestInvalidCurrentCandidateCannotSubstituteItsLastValidPreviewOrAnOldWorkingRevision(t *testing.T) {
	h := fixture(t)
	ctx := context.Background()
	h.svc = authoring.NewService(h.store, h.models, h.jobs, strictTargets{h.targets}, budget{}, estimates{})
	s, err := h.svc.Create(ctx, "alice", authoring.PostGuideline, "owned", "captured")
	if err != nil {
		t.Fatal(err)
	}
	s = recommend(t, h, s)
	historical := authoring.OwnedCandidateRef{SessionID: s.ID, CandidateID: s.Candidates[0].ID, Revision: s.Candidates[0].Revision}
	historicalBody := s.Candidates[0].Body
	s, err = h.svc.SelectWithKey(ctx, authoring.ResetMutation{UserID: "alice", SessionID: s.ID, ExpectedRevision: s.Revision, OperationKey: "choose"}, historical.CandidateID)
	if err != nil {
		t.Fatal(err)
	}
	s = patch(t, h, s, "edit-valid", "private valid revision")
	validRef := currentCandidateRef(s)
	frozen, err := h.svc.FreezeCandidate(ctx, "alice", validRef)
	if err != nil {
		t.Fatal(err)
	}
	invalid := patch(t, h, s, "invalid", "invalid-source")
	if _, err = h.svc.FreezeCandidate(ctx, "alice", currentCandidateRef(invalid)); !errors.Is(err, authoring.ErrDraftInvalid) {
		t.Fatalf("invalid source: %v", err)
	}
	if _, err = h.svc.FreezeCandidate(ctx, "alice", validRef); !errors.Is(err, authoring.ErrNotFound) {
		t.Fatalf("preview impersonated current work: %v", err)
	}
	original, err := h.svc.FreezeCandidate(ctx, "alice", historical)
	if err != nil || original.Artifact.Body != historicalBody || original.TargetID != "owned" || original.TargetVersion != "version-one" {
		t.Fatalf("historical recommendation lost: %+v %v", original, err)
	}
	if frozen.Artifact.Body != "private valid revision" {
		t.Fatal("invalid later source altered prior snapshot")
	}
}

func TestRefinedCurrentCandidateRemainsFrozenAfterLaterDirectEditing(t *testing.T) {
	h := fixture(t)
	ctx := context.Background()
	s, err := h.svc.Create(ctx, "alice", authoring.PostGuideline, "owned", "refine-current")
	if err != nil {
		t.Fatal(err)
	}
	id, _, err := h.svc.Start(ctx, "alice", authoring.Start{SessionID: s.ID, ExpectedRevision: s.Revision, RequestID: "refine", Mode: authoring.Refine, Prompt: "다듬어 주세요", WriteModel: llm.ModelRef{ProviderID: "p", ModelID: "m"}})
	if err != nil {
		t.Fatal(err)
	}
	j, err := h.jobs.Get(ctx, "alice", id)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]any{"artifact": map[string]string{"name": "Refined", "description": "Direction", "body": "refined complete direction", "title_area": ""}, "reply": "지침을 다듬었어요."})
	if err != nil {
		t.Fatal(err)
	}
	h.models.text = string(payload)
	if err = h.svc.Run(ctx, authoring.Run{ID: id, UserID: "alice", WriteModel: j.WriteModel, Payload: j.Payload}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	h.jobs.status(id, "done")
	s, err = h.svc.Get(ctx, "alice", s.ID)
	if err != nil {
		t.Fatal(err)
	}
	calls, enqueues := h.models.calls, h.jobs.enqueues
	frozen, err := h.svc.FreezeCandidate(ctx, "alice", currentCandidateRef(s))
	if err != nil || frozen.Artifact.Body != "refined complete direction" || frozen.TargetVersion != "version-one" {
		t.Fatalf("refined=%+v err=%v", frozen, err)
	}
	later := patch(t, h, s, "direct-after-refine", "later direct direction")
	if frozen.Artifact.Body == later.WorkingSource.Body || frozen.Artifact.Body != "refined complete direction" {
		t.Fatal("later direct edit rewrote the frozen refinement")
	}
	if h.models.calls != calls || h.jobs.enqueues != enqueues || h.targets.creates != 0 {
		t.Fatal("freeze replayed refinement or published")
	}
}

func TestCurrentCandidateUsesConfirmedEffectiveTargetWhileHistoricalRecommendationKeepsItsAdmission(t *testing.T) {
	h := fixture(t)
	ctx := context.Background()
	targets := &versionedTargets{targets: h.targets}
	h.svc = authoring.NewService(h.store, h.models, h.jobs, targets, budget{}, estimates{})
	s := selected(t, h, authoring.PostGuideline)
	historical := authoring.OwnedCandidateRef{SessionID: s.ID, CandidateID: s.Candidates[0].ID, Revision: s.Candidates[0].Revision}
	targets.published = s.Selected
	s, err := h.svc.SaveWithKey(ctx, authoring.ResetMutation{UserID: "alice", SessionID: s.ID, ExpectedRevision: s.Revision, OperationKey: "publish-before-edit"}, false)
	if err != nil {
		t.Fatal(err)
	}
	s = patch(t, h, s, "after-publication", "edit the newly saved target")
	frozen, err := h.svc.FreezeCandidate(ctx, "alice", currentCandidateRef(s))
	if err != nil || frozen.TargetID != s.Saved.ID || frozen.TargetVersion != "published-version" {
		t.Fatalf("effective target=%+v err=%v", frozen, err)
	}
	old, err := h.svc.FreezeCandidate(ctx, "alice", historical)
	if err != nil || old.TargetID != "" || old.TargetVersion != "" {
		t.Fatalf("historical context rebound=%+v err=%v", old, err)
	}
	if h.targets.creates != 1 {
		t.Fatal("freezing published again")
	}
}

type selectedOnlyStore struct{ authoring.Store }

func (s selectedOnlyStore) Get(ctx context.Context, user, id string) (authoring.Session, error) {
	state, err := s.Store.Get(ctx, user, id)
	state.WorkingSource = nil
	return state, err
}

func TestCurrentSelectedOnlyCompatibilityUsesTheExactValidatedRevision(t *testing.T) {
	h := fixture(t)
	s := create(t, h, authoring.PostGuideline)
	s = patch(t, h, s, "legacy-selected-only", "selected-only direction")
	svc := authoring.NewService(selectedOnlyStore{h.store}, h.models, h.jobs, h.targets, budget{}, estimates{})
	frozen, err := svc.FreezeCandidate(context.Background(), "alice", currentCandidateRef(s))
	if err != nil || frozen.Artifact.Body != s.Selected.Body {
		t.Fatalf("selected compatibility=%+v err=%v", frozen, err)
	}
}
