package store_test

import (
	"context"
	"errors"
	"github.com/postpilot/backend/internal/authoring"
	"github.com/postpilot/backend/internal/llm"
	"reflect"
	"strings"
	"testing"
)

func metadataString(s string) *string { return &s }
func TestDirectMetadataIsDurableAndAIKeepsOwnerFieldsWithoutSendingThem(t *testing.T) {
	h := fixture(t)
	ctx := context.Background()
	s := create(t, h, authoring.PostTemplate)
	a := *s.WorkingSource
	a.Name = "Owner template"
	a.Body = "Owner text"
	a.TargetLength = metadataString("1200")
	a.TagCount = metadataString("7")
	a.BuilderState = "PRIVATE_EDITOR_ROWS"
	s, e := h.svc.PatchDraft(ctx, authoring.DraftMutation{UserID: "alice", SessionID: s.ID, ExpectedRevision: s.Revision, OperationKey: "metadata", WorkingSource: a})
	if e != nil {
		t.Fatal(e)
	}
	reopened, e := secondService(t, h).Get(ctx, "alice", s.ID)
	if e != nil || !reflect.DeepEqual(reopened.WorkingSource, s.WorkingSource) {
		t.Fatal("metadata lost on reopen", e)
	}
	id, _, e := h.svc.Start(ctx, "alice", authoring.Start{SessionID: s.ID, ExpectedRevision: s.Revision, RequestID: "refine-owner", Mode: authoring.Refine, Prompt: "Refine the wording", WriteModel: llm.ModelRef{ProviderID: "p", ModelID: "m"}})
	if e != nil {
		t.Fatal(e)
	}
	job, _ := h.jobs.Get(ctx, "alice", id)
	h.models.text = `{"artifact":{"name":"Owner template","description":"","body":"Refined wording","title_area":""},"reply":"내용을 다듬었어요."}`
	if e = h.svc.Run(ctx, authoring.Run{ID: id, UserID: "alice", WriteModel: job.WriteModel, Payload: job.Payload}, func(string, int, int) {}); e != nil {
		t.Fatal(e)
	}
	h.jobs.status(id, "done")
	done, e := h.svc.Get(ctx, "alice", s.ID)
	if e != nil {
		t.Fatal(e)
	}
	if done.WorkingSource.TargetLength == nil || *done.WorkingSource.TargetLength != "1200" || done.WorkingSource.TagCount == nil || *done.WorkingSource.TagCount != "7" || done.WorkingSource.BuilderState != "PRIVATE_EDITOR_ROWS" {
		t.Fatal("AI changed owner metadata or lost private editor continuity")
	}
	request := h.models.requests[0]
	for _, message := range request.Messages {
		for _, part := range message.Parts {
			if strings.Contains(part.Text, "PRIVATE_EDITOR_ROWS") || strings.Contains(part.Text, "target_length") || strings.Contains(part.Text, "tag_count") {
				t.Fatal("private direct fields entered model prompt")
			}
		}
	}
	if h.models.calls != 1 || h.targets.creates != 0 {
		t.Fatal("hidden model work or publication")
	}
}
func TestExplicitReferenceCreationIsBoundedOwnerScopedAndNeverReadFromOtherSettings(t *testing.T) {
	h := fixture(t)
	ctx := context.Background()
	reference := "Selected title\n[photo position]\nSelected structure"
	s, e := h.svc.CreateWithReference(ctx, "alice", authoring.PostTemplate, "", "reference", reference)
	if e != nil || s.ReferencePost != reference {
		t.Fatal(e)
	}
	if _, e = h.svc.Get(ctx, "bob", s.ID); !errors.Is(e, authoring.ErrNotFound) {
		t.Fatal("reference owner boundary", e)
	}
	if _, e = h.svc.CreateWithReference(ctx, "alice", authoring.PostTemplate, "", "reference", "different"); !errors.Is(e, authoring.ErrStale) {
		t.Fatal("reference replay mismatch accepted", e)
	}
	if _, e = h.svc.CreateWithReference(ctx, "alice", authoring.PostTemplate, "", "too-long", strings.Repeat("가", 12001)); !errors.Is(e, authoring.ErrInvalid) {
		t.Fatal("oversize reference accepted", e)
	}
	if _, e = h.svc.CreateWithReference(ctx, "alice", authoring.PostGuideline, "", "wrong-kind", reference); !errors.Is(e, authoring.ErrInvalid) {
		t.Fatal("reference crossed kind", e)
	}
	if _, e = h.svc.CreateWithReference(ctx, "alice", authoring.PostTemplate, "owned", "existing", reference); !errors.Is(e, authoring.ErrInvalid) {
		t.Fatal("reference silently entered existing target", e)
	}
	if h.models.calls != 0 || h.jobs.enqueues != 0 || h.targets.creates != 0 {
		t.Fatal("reference creation performed paid/canonical work")
	}
}
