package main

import (
	"reflect"
	"testing"

	"connectrpc.com/connect"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/post"
)

func sourcePublicationFixture(t *testing.T, h *writingIntegrationHarness) post.Post {
	t.Helper()
	language := post.LanguageEnglish
	created, err := h.app.post.SaveDraft(t.Context(), "alice", post.DraftSave{Title: "Owner source title", Memo: "An explicitly supplied source account of a quiet walk.", TargetLanguage: &language})
	if err != nil {
		t.Fatal(err)
	}
	length := 1600
	created, err = h.app.post.SaveGenerationOptions(t.Context(), "alice", created.Slug, post.GenerationOptionsSet{TargetLength: &length, TagCount: 3})
	if err != nil {
		t.Fatal(err)
	}
	return created
}
func sourcePublicationChampion(t *testing.T, h *writingIntegrationHarness, source post.Post, key string, override bool) *v1.WritingTest {
	t.Helper()
	plan := h.plan(2)
	plan.Context.SourcePostSlug = source.Slug
	plan.Context.ExpectedInputRevision = source.InputRevision
	plan.Context.ExpectedContentRevision = source.ContentRevision
	if override {
		plan.Context.TargetLength = 2200
	}
	quote, err := h.client.EstimateWritingTest(t.Context(), writingRPCRequest(h, &v1.EstimateWritingTestRequest{Plan: plan}))
	if err != nil {
		t.Fatal(err)
	}
	started, err := h.client.StartWritingTest(t.Context(), writingRPCRequest(h, &v1.StartWritingTestRequest{Plan: plan, QuoteKey: quote.Msg.QuoteKey, RequestKey: key}))
	if err != nil {
		t.Fatal(err)
	}
	current := h.settledTest(t, started.Msg.Test.Id, v1.WritingTestStatus_WRITING_TEST_STATUS_REVIEW)
	if len(current.Matches) != 1 {
		t.Fatal("two complete outputs did not produce binary match", current)
	}
	match := current.Matches[0]
	picked, err := h.client.DecideTestMatch(t.Context(), writingRPCRequest(h, &v1.DecideTestMatchRequest{TestId: current.Id, ExpectedRevision: current.Revision, RequestKey: key + "-vote", MatchId: match.Id, WinnerCandidateId: match.LeftCandidateId}))
	if err != nil {
		t.Fatal(err)
	}
	if picked.Msg.Test.Status != v1.WritingTestStatus_WRITING_TEST_STATUS_COMPLETED {
		t.Fatal("no champion", picked.Msg.Test)
	}
	return picked.Msg.Test
}

func TestWritingTestsProductionSourceApplicationWritesCompleteBaselineAndReceiptWithoutReplayMutation(t *testing.T) {
	h := newWritingIntegrationHarness(t)
	source := sourcePublicationFixture(t, h)
	champion := sourcePublicationChampion(t, h, source, "source-champion", false)
	before, err := h.app.post.Get(t.Context(), "alice", source.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if before.Content != nil || before.ContentRevision != source.ContentRevision || before.InputRevision != source.InputRevision {
		t.Fatal("champion reveal mutated source", before)
	}
	var winner *v1.WritingTestCandidate
	for _, c := range champion.Candidates {
		if c.Id == champion.WinnerCandidateId {
			winner = c
			break
		}
	}
	if winner == nil || winner.Output == nil || winner.Storyline == nil {
		t.Fatal("full champion output missing")
	}
	request := &v1.ApplyWritingTestOutputRequest{TestId: champion.Id, WinnerCandidateId: champion.WinnerCandidateId, ExpectedRevision: champion.Revision, RequestKey: "apply-source-champion", ExpectedInputRevision: source.InputRevision, ExpectedContentRevision: source.ContentRevision}
	applied, err := h.client.ApplyWritingTestOutput(t.Context(), writingRPCRequest(h, request))
	if err != nil {
		t.Fatal(err)
	}
	current, err := h.app.post.Get(t.Context(), "alice", source.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if current.Content == nil || current.Content.Title != winner.Output.Title || len(current.Content.Blocks) != len(winner.Output.Blocks) || current.Storyline == nil || len(current.Storyline.Paragraphs) != 1 || current.Storyline.Paragraphs[0].Text != winner.Storyline.Paragraphs[0].Text || current.ContentRevision != source.ContentRevision+1 || current.MachineBaselineRevision != current.ContentRevision || current.ContentLanguage == nil || *current.ContentLanguage != post.LanguageEnglish {
		t.Fatal("publication lost full content/storyline/baseline provenance", current)
	}
	var baseline, body string
	var receipts int
	if err := h.platform.db.Reader.QueryRow("SELECT machine_baseline,content FROM posts WHERE slug=?", source.Slug).Scan(&baseline, &body); err != nil || baseline != body {
		t.Fatal("machine baseline differs from winner", baseline, body, err)
	}
	if err := h.platform.db.Reader.QueryRow("SELECT COUNT(*) FROM post_test_publications WHERE user_id='alice' AND test_id=? AND winner_candidate_id=? AND post_slug=?", champion.Id, champion.WinnerCandidateId, source.Slug).Scan(&receipts); err != nil || receipts != 1 {
		t.Fatal("target write lacks atomic provenance receipt", receipts, err)
	}
	manual := post.PostContent{Title: "Later owner edit", Blocks: []post.Block{{Type: post.BlockText, Content: "This manual edit must survive receipt recovery."}}}
	edited, err := h.app.post.SaveContent(t.Context(), "alice", source.Slug, manual, current.ContentRevision)
	if err != nil {
		t.Fatal(err)
	}
	request.RequestKey = "lost-response-new-key"
	replayed, err := h.client.ApplyWritingTestOutput(t.Context(), writingRPCRequest(h, request))
	if err != nil || replayed.Msg.Publication.TargetId != applied.Msg.Publication.TargetId {
		t.Fatal("publication receipt replay", replayed, err)
	}
	after, err := h.app.post.Get(t.Context(), "alice", source.Slug)
	if err != nil || after.ContentRevision != edited.ContentRevision || !reflect.DeepEqual(after.Content, edited.Content) || after.MachineBaselineRevision != current.MachineBaselineRevision {
		t.Fatal("receipt replay reversed later owner edit", after, err)
	}
	h.provider.mu.Lock()
	calls := len(h.provider.requests)
	h.provider.mu.Unlock()
	var admissions int
	if err := h.platform.db.Reader.QueryRow("SELECT COUNT(*) FROM usage_admissions WHERE kind='writing_test'").Scan(&admissions); err != nil || admissions != 1 || calls != 2 {
		t.Fatal("decisions/publication/replay admitted provider work", admissions, calls, err)
	}
}

func TestWritingTestsProductionSourceOptionOverrideCannotApplyIncompatibleFrozenOutput(t *testing.T) {
	h := newWritingIntegrationHarness(t)
	source := sourcePublicationFixture(t, h)
	champion := sourcePublicationChampion(t, h, source, "different-common-options", true)
	before, err := h.app.post.Get(t.Context(), "alice", source.Slug)
	if err != nil {
		t.Fatal(err)
	}
	_, err = h.client.ApplyWritingTestOutput(t.Context(), writingRPCRequest(h, &v1.ApplyWritingTestOutputRequest{TestId: champion.Id, WinnerCandidateId: champion.WinnerCandidateId, ExpectedRevision: champion.Revision, RequestKey: "incompatible-apply", ExpectedInputRevision: source.InputRevision, ExpectedContentRevision: source.ContentRevision}))
	if connect.CodeOf(err) != connect.CodeAborted && connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatal("incompatible common options were accepted", err)
	}
	after, readErr := h.app.post.Get(t.Context(), "alice", source.Slug)
	if readErr != nil || after.InputRevision != before.InputRevision || after.ContentRevision != before.ContentRevision || !reflect.DeepEqual(after.Content, before.Content) || !reflect.DeepEqual(after.Storyline, before.Storyline) {
		t.Fatal("refused source application mutated source", after, readErr)
	}
	var receipts int
	if err := h.platform.db.Reader.QueryRow("SELECT COUNT(*) FROM post_test_publications WHERE test_id=?", champion.Id).Scan(&receipts); err != nil || receipts != 0 {
		t.Fatal("incompatible source created target receipt", receipts, err)
	}
}
