package app

import (
	"encoding/json"
	"errors"
	"github.com/postpilot/backend/internal/clip"
	"testing"
	"time"
)

func linkedUploadFixture(t *testing.T) (*GenerationService, *uploadMemory, *uploadJobs, *uploadWriter, *linkedPostReader) {
	t.Helper()
	s, store, jobs, writer, _, _ := uploadFixture(t)
	at := time.Date(2026, 10, 15, 1, 0, 0, 0, time.UTC)
	reader := &linkedPostReader{posts: map[string]clip.LinkedPost{"alice:post": {Slug: "post", CreatedAt: at, ContentReady: true}}, texts: map[string]*clip.LinkedPostText{"alice:post": {Slug: "post", Title: "내 방문 기록", Sections: []clip.LinkedPostSection{{Heading: "한 끼", Text: "맛은 좋고 대기는 길었다"}}}}}
	s.projects.WithLinkedPosts(reader)
	store.project.LinkedPostRef = &clip.LinkedPostIdentity{Slug: "post", CreatedAt: at}
	return s, store, jobs, writer, reader
}
func TestLinkedPostFrozenWithAttempt(t *testing.T) {
	s, store, jobs, writer, reader := linkedUploadFixture(t)
	id, _ := uploadStart(t, s, clip.ChannelNaver, false)
	before := reader.calls
	reader.texts["alice:post"].Sections[0].Text = "나중에 고친 글"
	runUpload(t, s, jobs, id)
	if reader.calls != before || writer.input.LinkedPost == nil || writer.input.LinkedPost.Sections[0].Text != "맛은 좋고 대기는 길었다" {
		t.Fatal("worker reread or changed frozen text", writer.input.LinkedPost)
	}
	id, _ = uploadStart(t, s, clip.ChannelInstagram, false)
	runUpload(t, s, jobs, id)
	if writer.input.LinkedPost.Sections[0].Text != "나중에 고친 글" {
		t.Fatal("next call missed the edit")
	}
	// A generation and both revision snapshots carry the same read-once document.
	frozen, err := s.projects.freezeLinkedPost(t.Context(), "alice", store.project)
	if err != nil {
		t.Fatal(err)
	}
	payload := clip.GenerationPayload{Version: clip.GenerationPayloadVersion, Language: "ko", LinkedPost: frozen.LinkedPostText}
	raw, _ := json.Marshal(payload)
	reader.texts["alice:post"].Sections[0].Text = "더 늦은 수정"
	var restored clip.GenerationPayload
	if err = json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	in := (&generationRun{p: restored}).planningInput()
	if in.LinkedPost.Sections[0].Text != "나중에 고친 글" {
		t.Fatal("generation lost frozen text")
	}
	for _, version := range []int{3, 4, 8, 11, 12} {
		raw, _ = json.Marshal(clip.GenerationPayload{Version: version, Language: "ko"})
		if err = json.Unmarshal(raw, &restored); err != nil || !clip.SupportedGenerationPayload(version) || restored.LinkedPost != nil {
			t.Fatal("older accepted job gained a live post", version, err)
		}
	}
}
func TestPostEditBetweenQuoteAndStartChangesQuote(t *testing.T) {
	s, store, jobs, writer, reader := linkedUploadFixture(t)
	q, err := s.QuoteUploadText(t.Context(), "alice", "project", clip.ChannelNaver, "p/w", false)
	if err != nil {
		t.Fatal(err)
	}
	old, err := s.projects.freezeLinkedPost(t.Context(), "alice", store.project)
	if err != nil {
		t.Fatal(err)
	}
	reader.texts["alice:post"].Sections[0].Text = "맛은 별로 대기는 짧았다"
	_, err = s.StartUploadText(t.Context(), "alice", "project", clip.ChannelNaver, "p/w", false, clip.QuoteApproval{QuoteID: q.ID, MaxCredits: &q.Pricing.MaxCredits, CancellationPolicyVersion: clip.CancellationPolicyVersion})
	if !errors.Is(err, clip.ErrQuoteChanged) || len(jobs.jobs) != 0 || jobs.holds != 0 || writer.calls != 0 {
		t.Fatal("changed post consumed approval or held credits", err)
	}
	current, err := s.projects.freezeLinkedPost(t.Context(), "alice", store.project)
	if err != nil {
		t.Fatal(err)
	}
	before := clip.GenerationPricing{LinkedPostDigest: old.LinkedPostText.Digest()}
	after := clip.GenerationPricing{LinkedPostDigest: current.LinkedPostText.Digest()}
	if clip.QuoteInputDigest(old, clip.VideoTemplate{}, clip.SourceBatch{}, before) == clip.QuoteInputDigest(current, clip.VideoTemplate{}, clip.SourceBatch{}, after) ||
		clip.RevisionInputDigest(old, "revise", clip.RevisionBoth, "p/w", before) == clip.RevisionInputDigest(current, "revise", clip.RevisionBoth, "p/w", after) ||
		StorylineRevisionInputDigest(old, "revise", "p/w", before) == StorylineRevisionInputDigest(current, "revise", "p/w", after) ||
		planRecoveryDigest(clip.GenerationPayload{LinkedPost: old.LinkedPostText}) == planRecoveryDigest(clip.GenerationPayload{LinkedPost: current.LinkedPostText}) {
		t.Fatal("post edit reused writing approval or recovered plan")
	}
}
func TestLinkedTextIdentityUsesOneReadAndNeverAReusedSlug(t *testing.T) {
	s, store, _, _, reader := linkedUploadFixture(t)
	before := reader.calls
	p, err := s.projects.freezeLinkedPost(t.Context(), "alice", store.project)
	if err != nil || p.LinkedPostText == nil || reader.calls != before+1 {
		t.Fatal("text used a metadata/text double read", err)
	}
	meta := reader.posts["alice:post"]
	meta.CreatedAt = meta.CreatedAt.Add(time.Second)
	reader.posts["alice:post"] = meta
	p, err = s.projects.freezeLinkedPost(t.Context(), "alice", store.project)
	if err != nil || p.LinkedPostText != nil {
		t.Fatal("replacement identity supplied writing text", err)
	}
	delete(reader.posts, "alice:post")
	p, err = s.projects.freezeLinkedPost(t.Context(), "alice", store.project)
	if err != nil || p.LinkedPostText != nil {
		t.Fatal("deleted post supplied writing text", err)
	}
}

func TestUploadTextSnapshotCompatibilityKeepsApprovedAccounting(t *testing.T) {
	s, _, jobs, _, _ := linkedUploadFixture(t)
	id, _ := uploadStart(t, s, clip.ChannelNaver, false)
	var frozen clip.UploadTextSnapshot
	if err := json.Unmarshal(jobs.jobs[id].Payload, &frozen); err != nil {
		t.Fatal(err)
	}
	if frozen.Version != clip.UploadTextSnapshotVersion || frozen.Input.LinkedPost == nil {
		t.Fatal("new call did not use the linked-text snapshot version")
	}
	for _, version := range []int{0, 1, clip.UploadTextSnapshotVersion, clip.UploadTextSnapshotVersion + 1} {
		frozen.Version = version
		if version == 1 {
			frozen.Input.LinkedPost = nil
		}
		raw, _ := json.Marshal(frozen)
		got, ok := chargedApproval(clip.JobKindUploadText, raw, "alice", "project")
		want := version >= 1 && version <= clip.UploadTextSnapshotVersion
		if ok != want || ok && got.QuoteID != frozen.Approval.QuoteID {
			t.Fatal("snapshot version lost approved accounting", version, ok)
		}
	}
}

func TestCaptureNeighborsFrozenWithLinkedPhotos(t *testing.T) {
	s, store, _, _, reader := linkedUploadFixture(t)
	at := time.Date(2026, 10, 15, 3, 0, 0, 0, time.UTC)
	reader.texts["alice:post"].Sections[0].Photos = []clip.LinkedPhoto{{File: "31134.jpg", CapturedAt: &at}}
	frozen, err := s.projects.freezeLinkedPost(t.Context(), "alice", store.project)
	if err != nil {
		t.Fatal(err)
	}
	frozen = withCaptureNeighbors(frozen, clip.SourceBatch{Sources: []clip.SourceLease{{ID: "video", SourceMetadata: clip.SourceMetadata{Filename: "31136.mp4"}}}})
	before := frozen.LinkedPostText.Digest()
	reader.texts["alice:post"].Sections[0].Photos[0].File = "edited.jpg"
	*reader.texts["alice:post"].Sections[0].Photos[0].CapturedAt = at.Add(time.Hour)
	if frozen.LinkedPostText.Sections[0].Photos[0].File != "31134.jpg" || !frozen.LinkedPostText.Sections[0].Photos[0].CapturedAt.Equal(time.Date(2026, 10, 15, 3, 0, 0, 0, time.UTC)) || len(frozen.LinkedPostText.CaptureNeighbors["video"]) != 1 {
		t.Fatal("frozen photo metadata changed", frozen.LinkedPostText)
	}
	if frozen.LinkedPostText.Digest() != before {
		t.Fatal("snapshot was aliased")
	}
	current, err := s.projects.freezeLinkedPost(t.Context(), "alice", store.project)
	if err != nil || current.LinkedPostText.Digest() == before {
		t.Fatal("photo edit failed to change quote material")
	}
	raw, _ := json.Marshal(clip.GenerationPayload{Version: clip.GenerationPayloadVersion, Language: "ko", LinkedPost: frozen.LinkedPostText})
	var restored clip.GenerationPayload
	if err = json.Unmarshal(raw, &restored); err != nil || restored.LinkedPost.CaptureNeighbors["video"][0] != 0 {
		t.Fatal("accepted snapshot lost hints", err)
	}
}
