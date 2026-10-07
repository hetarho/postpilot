package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/postpilot/backend/internal/authoring"
	"github.com/postpilot/backend/internal/llm"
	"reflect"
	"strings"
	"testing"
)

func TestRefineCompletesNewIncompleteOrInvalidSourceWithoutASelectedPreview(t *testing.T) {
	for _, body := range []string{"", "invalid-source"} {
		t.Run(body, func(t *testing.T) {
			h := fixture(t)
			ctx := context.Background()
			h.svc = authoring.NewService(h.store, h.models, h.jobs, realTargets(h), budget{}, estimates{})
			s := create(t, h, authoring.VideoTemplate)
			s, err := h.svc.PatchDraft(ctx, authoring.DraftMutation{UserID: "alice", SessionID: s.ID, ExpectedRevision: s.Revision, OperationKey: "incomplete", WorkingSource: authoring.Artifact{Name: "Unfinished", Body: body}})
			if err != nil || s.Selected != nil || s.WorkingSource == nil || s.DraftState == authoring.DraftValid {
				t.Fatal("fixture did not retain unfinished new work", err)
			}
			reloaded, err := h.svc.Get(ctx, "alice", s.ID)
			if err != nil || !reflect.DeepEqual(*reloaded.WorkingSource, *s.WorkingSource) {
				t.Fatal("unfinished source was not durable", err)
			}
			id, active, err := h.svc.Start(ctx, "alice", authoring.Start{SessionID: s.ID, ExpectedRevision: s.Revision, RequestID: "finish-source", Mode: authoring.Refine, Prompt: "Complete my current structure", WriteModel: llm.ModelRef{ProviderID: "p", ModelID: "m"}})
			if err != nil || active.Phase != "refining" || h.jobs.enqueues != 1 {
				t.Fatal("stored source was rejected at admission", err)
			}
			job, _ := h.jobs.Get(ctx, "alice", id)
			var input struct {
				Selected struct{ Name, Body string }
			}
			if err = json.Unmarshal(job.Payload, &input); err != nil || input.Selected.Body != body || input.Selected.Name != "Unfinished" {
				t.Fatal("admission did not freeze the raw current source", err)
			}
			completedBody := videoBody("Complete structure")
			output, err := json.Marshal(map[string]any{"artifact": map[string]string{"name": "Completed", "description": "", "body": completedBody, "title_area": ""}, "reply": "구성을 완성했어요."})
			if err != nil {
				t.Fatal(err)
			}
			h.models.text = string(output)
			if err = h.svc.Run(ctx, authoring.Run{ID: id, UserID: "alice", WriteModel: job.WriteModel, Payload: job.Payload}, func(string, int, int) {}); err != nil {
				t.Fatal("explicit refinement failed", err)
			}
			h.jobs.status(id, "done")
			completed, err := h.svc.Get(ctx, "alice", s.ID)
			if err != nil || completed.Selected == nil || completed.WorkingSource.Body != completedBody || completed.DraftState != authoring.DraftValid || len(completed.Turns) != 1 || completed.Turns[0].Status != "done" {
				t.Fatal("validated completion did not replace the unfinished work", err)
			}
			if h.models.calls != 1 || h.targets.creates != 0 || completed.Saved != nil {
				t.Fatal("refinement retried or published implicitly")
			}
			saved, err := h.svc.SaveWithKey(ctx, authoring.ResetMutation{UserID: "alice", SessionID: s.ID, ExpectedRevision: completed.Revision, OperationKey: "save-completed"}, false)
			if err != nil {
				t.Fatal(err)
			}
			reopened, err := h.svc.Get(ctx, "alice", saved.ID)
			if err != nil {
				t.Fatal(err)
			}
			continued := patch(t, h, reopened, "continue-completed", completedBody)
			if continued.Phase != "editing" || len(continued.Turns) != 1 || continued.Turns[0] != completed.Turns[0] || continued.Saved == nil || *continued.Saved != *saved.Saved {
				t.Fatal("save/reopen/continuation reset the conversation or confirmed receipt")
			}
			if h.models.calls != 1 || h.jobs.enqueues != 1 {
				t.Fatal("continuation performed model work")
			}
		})
	}
}

type strictTargets struct{ *targets }

func (s strictTargets) Validate(kind authoring.Kind, a authoring.Artifact) error {
	if a.Body == "invalid-source" {
		return authoring.ErrOutput
	}
	return s.targets.Validate(kind, a)
}
func patch(t *testing.T, h harness, s authoring.Session, key, body string) authoring.Session {
	t.Helper()
	source := *s.WorkingSource
	source.Body = body
	result, err := h.svc.PatchDraft(context.Background(), authoring.DraftMutation{UserID: "alice", SessionID: s.ID, ExpectedRevision: s.Revision, OperationKey: key, WorkingSource: source})
	if err != nil {
		t.Fatal(err)
	}
	return result
}
func TestWorkingSourceInvalidPersistenceOwnerCASAndReceipts(t *testing.T) {
	h := fixture(t)
	ctx := context.Background()
	h.svc = authoring.NewService(h.store, h.models, h.jobs, strictTargets{h.targets}, budget{}, estimates{})
	s, err := h.svc.Create(ctx, "alice", authoring.PostTemplate, "owned", "baseline")
	if err != nil {
		t.Fatal(err)
	}
	baseline := *s.SavedBaseline
	original := s
	invalid := patch(t, h, s, "patch-once", "invalid-source")
	if invalid.DraftState != authoring.DraftInvalid || !reflect.DeepEqual(*invalid.Selected, baseline) || !invalid.HasUnpublishedChanges || !invalid.SavedAvailable {
		t.Fatal("invalid source lost usable baseline or last-valid preview")
	}
	reloaded, err := secondService(t, h).Get(ctx, "alice", s.ID)
	if err != nil || reloaded.WorkingSource.Body != "invalid-source" || reloaded.DraftState != authoring.DraftInvalid {
		t.Fatal("invalid input did not survive reopen", err)
	}
	if _, err = h.svc.Save(ctx, "alice", s.ID, invalid.Revision, false); !errors.Is(err, authoring.ErrDraftInvalid) {
		t.Fatal("invalid source published", err)
	}
	newer := patch(t, h, invalid, "patch-two", "")
	retrySource := *original.WorkingSource
	retrySource.Body = "invalid-source"
	in := authoring.DraftMutation{UserID: "alice", SessionID: s.ID, ExpectedRevision: original.Revision, OperationKey: "patch-once", WorkingSource: retrySource}
	replay, err := h.svc.PatchDraft(ctx, in)
	if err != nil || replay.Revision != invalid.Revision || replay.WorkingSource.Body != "invalid-source" {
		t.Fatal("lost-response receipt changed", err)
	}
	current, _ := h.svc.Get(ctx, "alice", s.ID)
	if current.Revision != newer.Revision || current.DraftState != authoring.DraftIncomplete {
		t.Fatal("receipt replay rewound current work")
	}
	in.OperationKey = "stale"
	if _, err = h.svc.PatchDraft(ctx, in); !errors.Is(err, authoring.ErrStale) {
		t.Fatal("manual CAS bypass", err)
	}
	in.UserID = "bob"
	if _, err = h.svc.PatchDraft(ctx, in); !errors.Is(err, authoring.ErrNotFound) {
		t.Fatal("foreign source visible", err)
	}
	if h.models.calls != 0 || h.jobs.enqueues != 0 || h.targets.creates != 0 {
		t.Fatal("manual work spent/published")
	}
}
func TestFreshChatRetainsWorkAndNamedResetDiscardsOnlyExplicitly(t *testing.T) {
	h := fixture(t)
	ctx := context.Background()
	s, err := h.svc.Create(ctx, "alice", authoring.PostGuideline, "owned", "seed")
	if err != nil {
		t.Fatal(err)
	}
	s = patch(t, h, s, "edit", "edited working text")
	fresh, err := h.svc.ResetChat(ctx, authoring.ResetMutation{UserID: "alice", SessionID: s.ID, ExpectedRevision: s.Revision, OperationKey: "fresh"})
	if err != nil || fresh.WorkingSource.Body != s.WorkingSource.Body || !fresh.HasUnpublishedChanges {
		t.Fatal("fresh chat discarded work", err)
	}
	reset, err := h.svc.ResetBaseline(ctx, authoring.ResetMutation{UserID: "alice", SessionID: s.ID, ExpectedRevision: fresh.Revision, OperationKey: "reset"})
	if err != nil || reset.WorkingSource.Body != "old body" || reset.HasUnpublishedChanges {
		t.Fatal("baseline reset did not discard", err)
	}
	if h.models.calls != 0 || h.jobs.enqueues != 0 || h.targets.creates != 0 {
		t.Fatal("reset caused paid or canonical work")
	}
}
func TestLateAISettlesWithoutReplacingNewerManualWork(t *testing.T) {
	h := fixture(t)
	ctx := context.Background()
	s := selected(t, h, authoring.PostGuideline)
	id, active, err := h.svc.Start(ctx, "alice", authoring.Start{SessionID: s.ID, ExpectedRevision: s.Revision, RequestID: "refine", Mode: authoring.Refine, Prompt: "change", WriteModel: llm.ModelRef{ProviderID: "p", ModelID: "m"}})
	if err != nil {
		t.Fatal(err)
	}
	job, _ := h.jobs.Get(ctx, "alice", id)
	h.models.text = `{"artifact":{"name":"AI result","description":"","body":"late AI","title_area":""},"reply":"바꿨어요."}`
	h.models.after = func() { active = patch(t, h, active, "manual-during-call", "new manual text") }
	if err = h.svc.Run(ctx, authoring.Run{ID: id, UserID: "alice", WriteModel: job.WriteModel, Payload: job.Payload}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	h.jobs.status(id, "done")
	state, err := h.svc.Get(ctx, "alice", s.ID)
	if err != nil || state.WorkingSource.Body != "new manual text" || state.Selected.Body != "new manual text" || state.ActiveJobID != "" {
		t.Fatal("late AI overwrote direct work or blocked recovery", err)
	}
}
func TestCountedCandidatesStayPrivateAndFreezeByOwnerAndRevision(t *testing.T) {
	for _, count := range []int{2, 4, 8, 16} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			h := fixture(t)
			ctx := context.Background()
			s := create(t, h, authoring.PostGuideline)
			input := authoring.Start{SessionID: s.ID, ExpectedRevision: s.Revision, RequestID: "batch", Mode: authoring.Recommend, RequestedCandidateCount: count, WriteModel: llm.ModelRef{ProviderID: "p", ModelID: "m"}}
			id, _, err := h.svc.Start(ctx, "alice", input)
			if err != nil {
				t.Fatal(err)
			}
			j, _ := h.jobs.Get(ctx, "alice", id)
			var body strings.Builder
			body.WriteString(`{"candidates":[`)
			for i := 0; i < count; i++ {
				if i > 0 {
					body.WriteString(",")
				}
				fmt.Fprintf(&body, `{"name":"candidate%d","description":"","body":"direction%d","title_area":""}`, i, i)
			}
			body.WriteString("]}")
			h.models.text = body.String()
			if err = h.svc.Run(ctx, authoring.Run{ID: id, UserID: "alice", WriteModel: j.WriteModel, Payload: j.Payload}, func(string, int, int) {}); err != nil {
				t.Fatal(err)
			}
			h.jobs.status(id, "done")
			state, err := h.svc.Get(ctx, "alice", s.ID)
			if err != nil || len(state.Candidates) != count || state.Saved != nil || h.targets.creates != 0 || h.models.calls != 1 || h.jobs.enqueues != 1 {
				t.Fatal("batch count/calls/publication mismatch", err)
			}
			ref := authoring.OwnedCandidateRef{SessionID: s.ID, CandidateID: state.Candidates[0].ID, Revision: state.Candidates[0].Revision}
			frozen, err := h.svc.FreezeCandidate(ctx, "alice", ref)
			if err != nil || !frozen.Synthetic || frozen.Artifact.Body != state.Candidates[0].Body {
				t.Fatal("frozen candidate mismatch", err)
			}
			if _, err = h.svc.FreezeCandidate(ctx, "bob", ref); !errors.Is(err, authoring.ErrNotFound) {
				t.Fatal("foreign candidate exposed")
			}
			ref.Revision++
			if _, err = h.svc.FreezeCandidate(ctx, "alice", ref); !errors.Is(err, authoring.ErrNotFound) {
				t.Fatal("candidate revision ignored")
			}
			input.RequestedCandidateCount = 8
			if count != 8 {
				if _, _, err = h.svc.Start(ctx, "alice", input); !errors.Is(err, authoring.ErrStale) {
					t.Fatal("count missing from admission fingerprint")
				}
			}
		})
	}
}
func TestBatchSummariesSeparateOwnedTargetsNewCreationsAndUncertainSaves(t *testing.T) {
	h := fixture(t)
	ctx := context.Background()
	a, _ := h.svc.Create(ctx, "alice", authoring.PostGuideline, "owned", "one")
	h.svc.Create(ctx, "alice", authoring.PostGuideline, "owned", "two")
	for i := 0; i < 3; i++ {
		h.svc.Create(ctx, "alice", authoring.PostGuideline, "", fmt.Sprint(i))
	}
	rows, next, err := h.svc.ListSummaries(ctx, authoring.SummaryQuery{UserID: "alice", Kind: authoring.PostGuideline, PageSize: 2})
	if err != nil || len(rows) != 2 || next == "" {
		t.Fatal("summary pagination", err)
	}
	second, _, err := h.svc.ListSummaries(ctx, authoring.SummaryQuery{UserID: "alice", Kind: authoring.PostGuideline, PageSize: 2, PageToken: next})
	if err != nil || len(second) != 2 {
		t.Fatal("batched latest-target/new creation read", err)
	}
	for _, row := range append(rows, second...) {
		if row.SessionID == a.ID {
			t.Fatal("older target session shown")
		}
	}
	unsaved, _, err := h.svc.ListSummaries(ctx, authoring.SummaryQuery{UserID: "alice", Kind: authoring.PostGuideline, UnsavedOnly: true})
	if err != nil || len(unsaved) != 3 {
		t.Fatal("new creations mixed with saved targets", err)
	}
	foreign, _, err := h.svc.ListSummaries(ctx, authoring.SummaryQuery{UserID: "bob", Kind: authoring.PostGuideline})
	if err != nil || len(foreign) != 0 {
		t.Fatal("owner summaries leaked", err)
	}
	seeded, _ := h.svc.Latest(ctx, "alice", authoring.PostGuideline, "owned")
	h.targets.failAfterCommit = true
	in := authoring.ResetMutation{UserID: "alice", SessionID: seeded.ID, ExpectedRevision: seeded.Revision, OperationKey: "save-once"}
	if _, err = h.svc.SaveWithKey(ctx, in, false); err == nil {
		t.Fatal("uncertain save silently confirmed")
	}
	summaries, _, err := h.svc.ListSummaries(ctx, authoring.SummaryQuery{UserID: "alice", Kind: authoring.PostGuideline})
	if err != nil {
		t.Fatal(err)
	}
	pending := false
	for _, row := range summaries {
		if row.SessionID == seeded.ID {
			pending = row.PublicationPending
		}
	}
	if !pending {
		t.Fatal("uncertain save disappeared")
	}
	saved, err := h.svc.SaveWithKey(ctx, in, false)
	if err != nil || saved.Phase != "saved" || h.targets.creates != 1 {
		t.Fatal("save receipt duplicated publication", err)
	}
	saved = patch(t, h, saved, "later edit", "later text")
	replay, err := h.svc.SaveWithKey(ctx, in, false)
	if err != nil || replay.Phase != "saved" || h.targets.creates != 1 {
		t.Fatal("confirmed receipt not retained", err)
	}
	current, _ := h.svc.Get(ctx, "alice", saved.ID)
	if current.WorkingSource.Body != "later text" {
		t.Fatal("save retry reversed later edit")
	}
	id, _, err := h.svc.Start(ctx, "alice", authoring.Start{SessionID: unsaved[0].SessionID, ExpectedRevision: unsaved[0].Revision, RequestID: "summary-active", Mode: authoring.Recommend, RequestedCandidateCount: 2, WriteModel: llm.ModelRef{ProviderID: "p", ModelID: "m"}})
	if err != nil {
		t.Fatal(err)
	}
	summaries, _, err = h.svc.ListSummaries(ctx, authoring.SummaryQuery{UserID: "alice", Kind: authoring.PostGuideline})
	if err != nil {
		t.Fatal(err)
	}
	foundActive := false
	for _, row := range summaries {
		if row.SessionID == unsaved[0].SessionID {
			foundActive = row.ActiveJobID == id
		}
	}
	if !foundActive {
		t.Fatal("summary omitted the admitted job")
	}
	if err = h.db.Reader.Close(); err != nil {
		t.Fatal(err)
	}
	if rows, _, err := h.svc.ListSummaries(ctx, authoring.SummaryQuery{UserID: "alice", Kind: authoring.PostGuideline}); err == nil || rows != nil {
		t.Fatal("failed summary read appeared as no draft")
	}
}

type deletedTargets struct{ *targets }

func (d deletedTargets) Publish(context.Context, authoring.Publication) (authoring.SavedRef, error) {
	return authoring.SavedRef{}, authoring.ErrNotFound
}
func TestDeletedTargetIsAConflictAndNeverAnAbsentPrivateDraft(t *testing.T) {
	h := fixture(t)
	ctx := context.Background()
	h.svc = authoring.NewService(h.store, h.models, h.jobs, deletedTargets{h.targets}, budget{}, estimates{})
	s, err := h.svc.Create(ctx, "alice", authoring.PostGuideline, "owned", "target")
	if err != nil {
		t.Fatal(err)
	}
	_, err = h.svc.SaveWithKey(ctx, authoring.ResetMutation{UserID: "alice", SessionID: s.ID, ExpectedRevision: s.Revision, OperationKey: "save"}, false)
	if !errors.Is(err, authoring.ErrTargetConflict) {
		t.Fatal("deleted target hid the owned private session", err)
	}
	state, err := h.svc.Get(ctx, "alice", s.ID)
	if err != nil || state.WorkingSource.Body != "old body" || state.SavedAvailable {
		t.Fatal("deleted target lost draft or claimed availability", err)
	}
	rows, _, err := h.svc.ListSummaries(ctx, authoring.SummaryQuery{UserID: "alice", Kind: authoring.PostGuideline})
	if err != nil || len(rows) != 1 || !rows[0].TargetConflict || rows[0].SavedAvailable {
		t.Fatal("summary lost missing-target conflict", err)
	}
}

func TestUntitledGuidelineUsesReadableSummaryAndKeepsDomainValidManualWork(t *testing.T) {
	h := fixture(t)
	ctx := context.Background()
	s, err := h.svc.Create(ctx, "alice", authoring.PostGuideline, "owned", "untitled")
	if err != nil {
		t.Fatal(err)
	}
	source := *s.WorkingSource
	source.Name = ""
	source.Body = "과장 없이 경험을 써 주세요."
	s, err = h.svc.PatchDraft(ctx, authoring.DraftMutation{UserID: "alice", SessionID: s.ID, ExpectedRevision: s.Revision, OperationKey: "untitled-edit", WorkingSource: source})
	if err != nil || s.DraftState != authoring.DraftValid || s.Selected.Name != "" {
		t.Fatal("domain-valid untitled guideline was made invalid", err)
	}
	rows, _, err := h.svc.ListSummaries(ctx, authoring.SummaryQuery{UserID: "alice", Kind: authoring.PostGuideline})
	if err != nil || len(rows) != 1 || rows[0].DisplayName == "" {
		t.Fatal("untitled item lost its readable identity", err)
	}
	saved, err := h.svc.SaveWithKey(ctx, authoring.ResetMutation{UserID: "alice", SessionID: s.ID, ExpectedRevision: s.Revision, OperationKey: "save-untitled"}, false)
	if err != nil || saved.Saved == nil {
		t.Fatal("publication did not respect optional domain title", err)
	}
}

type versionedTargets struct {
	*targets
	published *authoring.Artifact
	drift     bool
}

func (v *versionedTargets) Publish(ctx context.Context, p authoring.Publication) (authoring.SavedRef, error) {
	a := p.Artifact
	v.published = &a
	return v.targets.Publish(ctx, p)
}
func (v *versionedTargets) Seed(ctx context.Context, user string, kind authoring.Kind, id string) (authoring.Seed, error) {
	seed, err := v.targets.Seed(ctx, user, kind, id)
	if strings.HasPrefix(id, "saved-") {
		seed.TargetVersion = "published-version"
		seed.Artifact = v.published
		if v.drift && seed.Artifact != nil {
			copy := *seed.Artifact
			copy.Body = "later owner edit"
			seed.Artifact = &copy
		}
	}
	return seed, err
}
func TestCandidateKeepsAdmittedTargetVersionAfterConfirmedPublication(t *testing.T) {
	h := fixture(t)
	ctx := context.Background()
	h.svc = authoring.NewService(h.store, h.models, h.jobs, &versionedTargets{targets: h.targets}, budget{}, estimates{})
	s, err := h.svc.Create(ctx, "alice", authoring.PostGuideline, "owned", "captured")
	if err != nil {
		t.Fatal(err)
	}
	s = recommend(t, h, s)
	candidate := s.Candidates[0]
	ref := authoring.OwnedCandidateRef{SessionID: s.ID, CandidateID: candidate.ID, Revision: candidate.Revision}
	s, err = h.svc.SelectWithKey(ctx, authoring.ResetMutation{UserID: "alice", SessionID: s.ID, ExpectedRevision: s.Revision, OperationKey: "choose"}, candidate.ID)
	if err != nil {
		t.Fatal(err)
	}
	s, err = h.svc.SaveWithKey(ctx, authoring.ResetMutation{UserID: "alice", SessionID: s.ID, ExpectedRevision: s.Revision, OperationKey: "publish"}, false)
	if err != nil || s.TargetVersion != "published-version" {
		t.Fatal("confirmed target version not refreshed", err)
	}
	frozen, err := h.svc.FreezeCandidate(ctx, "alice", ref)
	if err != nil || frozen.TargetID != "owned" || frozen.TargetVersion != "version-one" {
		t.Fatal("later publication rebound a prepared candidate", err)
	}
}

func TestReopenedUncertainPublicationConfirmsWithANewKeyAndReturnsItsReceiptAfterLaterEdits(t *testing.T) {
	h := fixture(t)
	ctx := context.Background()
	s := selected(t, h, authoring.PostGuideline)
	h.targets.failAfterCommit = true
	_, err := h.svc.SaveWithKey(ctx, authoring.ResetMutation{UserID: "alice", SessionID: s.ID, ExpectedRevision: s.Revision, OperationKey: "first-save"}, false)
	if err == nil {
		t.Fatal("fixture did not interrupt publication")
	}
	reopened, err := secondService(t, h).Get(ctx, "alice", s.ID)
	if err != nil || reopened.Phase != "saving" {
		t.Fatal("pending publication not recovered", err)
	}
	recovery := authoring.ResetMutation{UserID: "alice", SessionID: s.ID, ExpectedRevision: reopened.Revision, OperationKey: "reopened-confirmation"}
	saved, err := h.svc.SaveWithKey(ctx, recovery, true)
	if err != nil || saved.Phase != "saved" || h.targets.creates != 1 {
		t.Fatal("reopened confirmation failed or republished", err)
	}
	if saved.Publication.MakeDefault {
		t.Fatal("confirmation replaced the admitted default choice")
	}
	later := patch(t, h, saved, "later-working-edit", "later work")
	receipt, err := h.svc.SaveWithKey(ctx, recovery, true)
	if err != nil || receipt.Revision != saved.Revision || receipt.Phase != "saved" || h.targets.creates != 1 {
		t.Fatal("recovery receipt did not survive later edits", err)
	}
	current, _ := h.svc.Get(ctx, "alice", later.ID)
	if current.WorkingSource.Body != "later work" {
		t.Fatal("confirmation replay undid a later edit")
	}
}

func TestPublicationConfirmationDoesNotRebaseOverALaterSavedEdit(t *testing.T) {
	h := fixture(t)
	ctx := context.Background()
	h.svc = authoring.NewService(h.store, h.models, h.jobs, &versionedTargets{targets: h.targets, drift: true}, budget{}, estimates{})
	s, err := h.svc.Create(ctx, "alice", authoring.PostGuideline, "owned", "racing-publication")
	if err != nil {
		t.Fatal(err)
	}
	s = patch(t, h, s, "new-body", "my explicit publication")
	saved, err := h.svc.SaveWithKey(ctx, authoring.ResetMutation{UserID: "alice", SessionID: s.ID, ExpectedRevision: s.Revision, OperationKey: "save"}, false)
	if err != nil || saved.Phase != "saved" {
		t.Fatal("confirmed publication was lost", err)
	}
	if saved.TargetVersion != "version-one" {
		t.Fatal("later saved edit authorized replacing its newer content")
	}
}

func TestChangedBatchCountRetainsPriorVisibleBatchDuringCancellationAndInvalidOutput(t *testing.T) {
	for _, cancel := range []bool{true, false} {
		t.Run(fmt.Sprint(cancel), func(t *testing.T) {
			h := fixture(t)
			ctx := context.Background()
			s := recommend(t, h, create(t, h, authoring.PostGuideline))
			first := s.Candidates[0]
			id, active, err := h.svc.Start(ctx, "alice", authoring.Start{SessionID: s.ID, ExpectedRevision: s.Revision, RequestID: "sixteen", Mode: authoring.Recommend, RequestedCandidateCount: 16, WriteModel: llm.ModelRef{ProviderID: "p", ModelID: "m"}})
			if err != nil || active.RequestedCandidateCount != 8 || len(active.Candidates) != 8 {
				t.Fatal("pending request relabeled previous eight as sixteen", err)
			}
			if cancel {
				_, err = h.svc.Cancel(ctx, "alice", s.ID, id)
				if err != nil {
					t.Fatal(err)
				}
			} else {
				job, _ := h.jobs.Get(ctx, "alice", id)
				h.models.text = batch(authoring.PostGuideline)
				if err = h.svc.Run(ctx, authoring.Run{ID: id, UserID: "alice", WriteModel: job.WriteModel, Payload: job.Payload}, func(string, int, int) {}); err == nil {
					t.Fatal("silently shrank sixteen to eight")
				}
				h.jobs.status(id, "failed")
			}
			current, err := h.svc.Get(ctx, "alice", s.ID)
			if err != nil || current.RequestedCandidateCount != 8 || len(current.Candidates) != 8 || !reflect.DeepEqual(current.Candidates[0], first) {
				t.Fatal("failed/cancelled preparation lost the prior batch", err)
			}
		})
	}
}
