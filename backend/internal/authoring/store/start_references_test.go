package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/postpilot/backend/internal/authoring"
	"github.com/postpilot/backend/internal/llm"
)

// references reads references from a table; a missing entry is the refusal it names.
type references struct {
	docs  map[string]authoring.ReferenceDocument
	reads []authoring.Reference
}

func (r *references) Read(_ context.Context, owner string, ref authoring.Reference) (authoring.ReferenceDocument, error) {
	r.reads = append(r.reads, ref)
	if owner != "alice" {
		return authoring.ReferenceDocument{}, &authoring.ReferenceRefusal{Code: authoring.ReferenceRefusalMissing}
	}
	key := ref.PostSlug + ref.TemplateID + ref.URL
	doc, ok := r.docs[key]
	if !ok {
		return authoring.ReferenceDocument{}, &authoring.ReferenceRefusal{Code: key}
	}
	return doc, nil
}

func structureLines(prefix string, n int) string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = fmt.Sprintf("## %s 소제목 %03d", prefix, i)
	}
	return strings.Join(lines, "\n")
}

func startWith(h harness, s authoring.Session, request string, refs ...authoring.Reference) (string, error) {
	id, _, err := h.svc.Start(context.Background(), "alice", authoring.Start{SessionID: s.ID, ExpectedRevision: s.Revision, RequestID: request, Mode: authoring.Refine, Prompt: "이 형식으로 템플릿을 만들어 줘", WriteModel: llm.ModelRef{ProviderID: "p", ModelID: "writer"}, References: refs})
	return id, err
}

func refusalOf(t *testing.T, err error) string {
	t.Helper()
	var refusal *authoring.ReferenceRefusal
	if !errors.As(err, &refusal) {
		t.Fatalf("err = %v, want a reference refusal", err)
	}
	if refusal.Reason() != authoring.FailureReasonReferenceUnavailable || refusal.Params()["refusal"] != refusal.Code || len(refusal.Params()) != 1 {
		t.Fatalf("refusal reason %q params %v", refusal.Reason(), refusal.Params())
	}
	return refusal.Code
}

func TestStartFreezesFormReferencesIntoTheJobAlone(t *testing.T) {
	h := fixture(t)
	refs := &references{docs: map[string]authoring.ReferenceDocument{
		"my-post":                  {Kind: "post", Title: "내 글", Structure: structureLines("글", 400)},
		"tmpl-1":                   {Kind: "template", Title: "내 템플릿", Structure: "## 방문 정보\n[본문]"},
		"https://example.com/page": {Kind: "url", Title: "웹 페이지 " + strings.Repeat("제", 300), Structure: "## 웹 소제목\n[사진]"},
	}}
	h.svc.WithReferences(refs)
	s := create(t, h, authoring.PostTemplate)
	id, err := startWith(h, s, "with-references", authoring.Reference{PostSlug: "my-post"}, authoring.Reference{TemplateID: "tmpl-1"}, authoring.Reference{URL: "https://example.com/page"})
	if err != nil {
		t.Fatal(err)
	}
	job, err := h.jobs.Get(context.Background(), "alice", id)
	if err != nil {
		t.Fatal(err)
	}
	var frozen struct {
		References []struct{ Kind, Title, Structure string } `json:"form_references"`
	}
	if err := json.Unmarshal(job.Payload, &frozen); err != nil {
		t.Fatal(err)
	}
	if len(frozen.References) != 3 || frozen.References[0].Kind != "post" || frozen.References[1].Kind != "template" || frozen.References[2].Kind != "url" {
		t.Fatalf("frozen references = %+v", frozen.References)
	}
	share := authoring.MaxReferenceChars / 3
	for _, ref := range frozen.References {
		if n := utf8.RuneCountInString(ref.Structure); n > share {
			t.Fatalf("%s structure is %d characters, over its %d share", ref.Kind, n, share)
		}
		if utf8.RuneCountInString(ref.Title) > 200 {
			t.Fatalf("%s title is not bounded", ref.Kind)
		}
	}
	post := frozen.References[0].Structure
	if !strings.HasPrefix(structureLines("글", 400), post) || strings.HasSuffix(post, "\n") || !strings.HasSuffix(post, strings.Split(post, "\n")[len(strings.Split(post, "\n"))-1]) {
		t.Fatalf("post structure is not cut at a line boundary")
	}
	for _, line := range strings.Split(post, "\n") {
		if !strings.HasPrefix(line, "## 글 소제목 ") || utf8.RuneCountInString(line) != utf8.RuneCountInString("## 글 소제목 000") {
			t.Fatalf("a line was cut inside: %q", line)
		}
	}
	// The references belong to the request: nothing of them is stored on the session.
	state, err := h.svc.Get(context.Background(), "alice", s.ID)
	if err != nil {
		t.Fatal(err)
	}
	stored, _ := json.Marshal(state)
	for _, leaked := range []string{"내 글", "내 템플릿", "방문 정보", "웹 소제목", "example.com", "my-post", "tmpl-1"} {
		if strings.Contains(string(stored), leaked) {
			t.Fatalf("session stores reference material %q", leaked)
		}
	}
	if state.ReferencePost != "" {
		t.Fatalf("reference_post = %q", state.ReferencePost)
	}
}

func TestStartRefusesReferencesItCannotUse(t *testing.T) {
	h := fixture(t)
	refs := &references{docs: map[string]authoring.ReferenceDocument{"ok": {Kind: "post", Title: "t", Structure: "[본문]"}}}
	ok := authoring.Reference{PostSlug: "ok"}

	s := create(t, h, authoring.PostTemplate)
	if _, err := startWith(h, s, "no-reader", ok); !errors.Is(err, authoring.ErrFeatureUnavailable) {
		t.Fatalf("without a reader err = %v, want ErrFeatureUnavailable", err)
	}
	h.svc.WithReferences(refs)
	enqueued := h.jobs.enqueues
	if _, err := startWith(h, s, "too-many", ok, ok, ok, ok); refusalOf(t, err) != authoring.ReferenceRefusalTooMany {
		t.Fatalf("four references = %v", err)
	}
	if _, err := startWith(h, s, "ambiguous", authoring.Reference{PostSlug: "ok", URL: "https://example.com/"}); refusalOf(t, err) != authoring.ReferenceRefusalInvalid {
		t.Fatalf("a reference naming two sources = %v", err)
	}
	if _, err := startWith(h, s, "blocked", ok, authoring.Reference{URL: authoring.ReferenceRefusalNotAllowed}); refusalOf(t, err) != authoring.ReferenceRefusalNotAllowed {
		t.Fatalf("a refused URL = %v", err)
	}
	guideline, err := h.svc.Create(context.Background(), "alice", authoring.PostGuideline, "", "guideline-key")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := startWith(h, guideline, "guideline", ok); !errors.Is(err, authoring.ErrInvalid) {
		t.Fatalf("references on a guideline err = %v, want ErrInvalid", err)
	}
	if h.jobs.enqueues != enqueued {
		t.Fatal("a refused request was admitted")
	}
	// The request goes through once the refused reference is taken off.
	if _, err := startWith(h, s, "retry", ok); err != nil {
		t.Fatal(err)
	}
}

func TestCheckReferenceReadsWithoutStartingWork(t *testing.T) {
	h := fixture(t)
	if _, err := h.svc.CheckReference(context.Background(), "alice", authoring.Reference{URL: "https://example.com/"}); !errors.Is(err, authoring.ErrFeatureUnavailable) {
		t.Fatalf("without a reader err = %v", err)
	}
	refs := &references{docs: map[string]authoring.ReferenceDocument{"tmpl": {Kind: "template", Title: "Mine", Structure: "[본문]"}}}
	h.svc.WithReferences(refs)
	doc, err := h.svc.CheckReference(context.Background(), "alice", authoring.Reference{TemplateID: "tmpl"})
	if err != nil || doc.Title != "Mine" {
		t.Fatalf("doc = %+v err = %v", doc, err)
	}
	if _, err := h.svc.CheckReference(context.Background(), "bob", authoring.Reference{TemplateID: "tmpl"}); refusalOf(t, err) != authoring.ReferenceRefusalMissing {
		t.Fatalf("another account's template = %v", err)
	}
	if _, err := h.svc.CheckReference(context.Background(), "alice", authoring.Reference{}); refusalOf(t, err) != authoring.ReferenceRefusalInvalid {
		t.Fatalf("an empty reference = %v", err)
	}
	if _, err := h.svc.CheckReference(context.Background(), "", authoring.Reference{TemplateID: "tmpl"}); !errors.Is(err, authoring.ErrNotFound) {
		t.Fatalf("no owner err = %v", err)
	}
	if h.jobs.enqueues != 0 || h.models.calls != 0 {
		t.Fatal("a check started work")
	}
}
