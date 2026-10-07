package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/post"
)

func originPublication(content post.PostContent, category post.OriginCategory, quote string) post.GeneratedOriginResult {
	index := 0
	identity := post.ContentOriginIdentity(content, 0)
	resolved := post.ResolveOriginCandidates(content, identity, nil, []post.OriginCandidate{{Field: post.OriginFieldLocator{Kind: post.OriginFieldBlockContent, BlockIndex: &index}, Quote: quote, Category: category}}).Review
	return post.GeneratedOriginResult{Content: content, Language: post.LanguageEnglish, Origins: &resolved}
}

func TestOriginAttachmentTraceReadAndWriteShareWriterTransaction(t *testing.T) {
	s, handle := newStoreWithHandle(t)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	seedPost(t, s, "p", "alice", testNow)
	content := post.PostContent{Blocks: []post.Block{{Type: post.BlockText, Content: "Known AI meaning"}}}
	if _, err := s.PublishGeneratedResult(ctx, "alice", "p", originPublication(content, post.OriginAIAdded, "Known AI meaning"), testNow); err != nil {
		t.Fatal(err)
	}
	// Hold the single writer while a trace cleanup queues. Its reader must see
	// this newly committed sidecar, rather than copying the old reader-pool row.
	tx, err := handle.Writer.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	waits := handle.Writer.Stats().WaitCount
	finished := make(chan error, 1)
	go func() {
		_, err := s.UpdateAttachmentTraces(ctx, "p", "alice", "deleted.jpg", "old-photo", testNow.Add(time.Minute))
		finished <- err
	}()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for handle.Writer.Stats().WaitCount == waits {
		select {
		case err := <-finished:
			t.Fatalf("trace write did not wait for writer: %v", err)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-tick.C:
		}
	}
	if _, err := tx.ExecContext(ctx, "UPDATE posts SET content_origins=NULL WHERE slug='p'"); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
	after, err := s.GetPost(ctx, "p")
	if err != nil || after.ContentOrigins != nil || after.ContentRevision != 1 || after.MachineBaselineRevision != 1 {
		t.Fatalf("cleanup restored superseded sidecar: %+v, %v", after, err)
	}
}

func TestOriginQueuedAttachmentCleanupPreservesNewestPlanAndObservations(t *testing.T) {
	s, handle := newStoreWithHandle(t)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	seedPost(t, s, "p", "alice", testNow)
	old := post.Storyline{Paragraphs: []post.StorylineParagraph{{Text: "Old AI plan", Files: []string{"deleted.jpg"}}}, MadeWith: []string{"deleted.jpg"}}
	if _, err := s.UpdateStoryline(ctx, "p", "alice", &old, testNow); err != nil {
		t.Fatal(err)
	}
	newer := post.Storyline{EditedByHand: true, Paragraphs: []post.StorylineParagraph{{Text: "New owner plan"}}}
	identity := post.PlanOriginIdentity(newer.Paragraphs)
	newOrigins, err := json.Marshal(map[string]any{"version": post.OriginVersion, "result": map[string]any{"content_revision": 0, "content_hash": identity.ContentHash}, "sources": []map[string]any{{"id": "new-edit", "kind": post.OriginSourceOwnerEdit, "text": "New owner plan", "available": true}}, "spans": []map[string]any{{"paragraph_index": 0, "start": 0, "end": len("New owner plan"), "quote": "New owner plan", "category": post.OriginOwnerInput, "source_refs": []string{"new-edit"}, "review_state": post.OriginConfirmed}}})
	if err != nil {
		t.Fatal(err)
	}
	tx, err := handle.Writer.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	waits := handle.Writer.Stats().WaitCount
	finished := make(chan error, 1)
	go func() {
		_, err := s.UpdateAttachmentTraces(ctx, "p", "alice", "deleted.jpg", "old-photo", testNow)
		finished <- err
	}()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for handle.Writer.Stats().WaitCount == waits {
		select {
		case err := <-finished:
			t.Fatalf("cleanup did not wait for writer: %v", err)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-tick.C:
		}
	}
	const newPlan = `{"paragraphs":[{"text":"New owner plan","files":[]}],"edited_by_hand":true,"made_with":[]}`
	const newObservations = `[{"file":"deleted.jpg","scene":"Old deleted scene"},{"file":"other.jpg","scene":"New current scene"}]`
	if _, err := tx.ExecContext(ctx, "UPDATE posts SET storyline=?,storyline_origins=?,observations=?,input_revision=input_revision+1 WHERE slug='p'", newPlan, string(newOrigins), newObservations); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
	after, err := s.GetPost(ctx, "p")
	if err != nil || after.Storyline == nil || !after.Storyline.EditedByHand || !reflect.DeepEqual(after.Storyline.Paragraphs, newer.Paragraphs) || after.Storyline.Origins == nil || len(after.Storyline.Origins.Spans) != 1 || after.Storyline.Origins.Spans[0].ReviewState != post.OriginConfirmed || len(after.Observations) != 1 || after.Observations[0].File != "other.jpg" || after.Observations[0].Scene != "New current scene" || after.Observations[0].Origins != nil {
		t.Fatalf("queued cleanup restored old plan/observations: %+v %v", after, err)
	}
}

func TestOriginAttachmentCleanupRebindsKnownPlanMeaningAfterFilenameRemoval(t *testing.T) {
	s := newStore(t)
	ctx := t.Context()
	seedPost(t, s, "p", "alice", testNow)
	image := post.Image{ID: "old-photo", PostSlug: "p", Filename: "a.jpg", Key: "private-key", Width: 100, Height: 100, Bytes: 1, CreatedAt: testNow}
	if err := s.CreateImage(ctx, image); err != nil {
		t.Fatal(err)
	}
	paragraphs := []post.StorylineParagraph{{Text: "Known AI meaning", Files: []string{"a.jpg"}}}
	plan := post.Storyline{Paragraphs: paragraphs, MadeWith: []string{"a.jpg"}, Origins: &post.PlanOriginReview{Version: post.OriginVersion, Result: post.PlanOriginIdentity(paragraphs), Sources: []post.OriginSource{{ID: "ai", Kind: post.OriginSourceAIProposal, Text: "Frozen proposal", Available: true}}, Spans: []post.PlanOriginSpan{{ParagraphIndex: 0, Start: 0, End: len("Known AI meaning"), Quote: "Known AI meaning", Category: post.OriginAIAdded, SourceRefs: []string{"ai"}, ReviewState: post.OriginConfirmed}}}}
	expected := post.StorylineFingerprint(nil)
	if err := s.PublishStorylineResult(ctx, "alice", "p", post.StorylineOriginResult{Storyline: plan, ExpectedPlanFingerprint: &expected}, testNow); err != nil {
		t.Fatal(err)
	}
	if deleted, err := s.DeleteImage(ctx, "old-photo"); err != nil || !deleted {
		t.Fatal(deleted, err)
	}
	if _, err := s.UpdateAttachmentTraces(ctx, "p", "alice", "a.jpg", "old-photo", testNow); err != nil {
		t.Fatal(err)
	}
	after, err := s.GetPost(ctx, "p")
	if err != nil || after.Storyline == nil || len(after.Storyline.Paragraphs[0].Files) != 0 || len(after.Storyline.MadeWith) != 0 || after.Storyline.Origins == nil || after.Storyline.Origins.Result != post.PlanOriginIdentity(after.Storyline.Paragraphs) || len(after.Storyline.Origins.Spans) != 1 || !reflect.DeepEqual(after.Storyline.Origins.Spans[0], plan.Origins.Spans[0]) || !reflect.DeepEqual(after.Storyline.Origins.Sources, plan.Origins.Sources) {
		t.Fatalf("filename cleanup erased/promoted known plan meaning: %+v %v", after, err)
	}
}

func TestOriginDelayedCleanupKeepsOnlyReplacementIncarnationObservation(t *testing.T) {
	for _, mode := range []string{"old evidence", "new evidence", "unknown legacy", "mismatched result"} {
		t.Run(mode, func(t *testing.T) {
			s := newStore(t)
			ctx := t.Context()
			seedPost(t, s, "p", "alice", testNow)
			image := post.Image{ID: "old-photo", PostSlug: "p", Filename: "a.jpg", Key: "old-key", Width: 100, Height: 100, Bytes: 1, CreatedAt: testNow}
			if err := s.CreateImage(ctx, image); err != nil {
				t.Fatal(err)
			}
			if deleted, err := s.DeleteImage(ctx, "old-photo"); err != nil || !deleted {
				t.Fatal(deleted, err)
			}
			image.ID, image.Key = "new-photo", "new-key"
			if err := s.CreateImage(ctx, image); err != nil {
				t.Fatal(err)
			}
			observation := post.Observation{File: "a.jpg", Scene: "Current scene", Objects: []string{}, Events: []string{}}
			if mode != "unknown legacy" {
				mediaID := "new-photo"
				if mode == "old evidence" {
					mediaID = "old-photo"
				}
				observation.Origins = &post.ObservationOriginReview{Version: post.OriginVersion, Result: post.ObservationOriginIdentity(observation), Sources: []post.OriginSource{{ID: "source", Kind: post.OriginSourceVisualObservation, Text: "Frozen current scene", AttachmentFilename: "a.jpg", AttachmentID: mediaID, Available: true}}, Spans: []post.ObservationOriginSpan{{Field: "scene", Start: 0, End: len(observation.Scene), Quote: observation.Scene, Category: post.OriginPhotoInterpretation, SourceRefs: []string{"source"}, ReviewState: post.OriginUnconfirmed}}}
				if mode == "mismatched result" {
					observation.Origins.Result.ContentHash = "other observation"
				}
			}
			if _, err := s.UpdateObservations(ctx, "p", "alice", []post.Observation{observation, {File: "unaffected.jpg", Scene: "Unconfirmed legacy scene"}}, testNow); err != nil {
				t.Fatal(err)
			}
			manual := post.Storyline{EditedByHand: true, Paragraphs: []post.StorylineParagraph{{Text: "New plan for replacement", Files: []string{"a.jpg"}}}, MadeWith: []string{"a.jpg"}}
			if _, err := s.UpdateStoryline(ctx, "p", "alice", &manual, testNow); err != nil {
				t.Fatal(err)
			}
			before, _ := s.GetPost(ctx, "p")
			if _, err := s.UpdateAttachmentTraces(ctx, "p", "alice", "a.jpg", "old-photo", testNow); err != nil {
				t.Fatal(err)
			}
			after, err := s.GetPost(ctx, "p")
			want := 1
			if mode == "new evidence" {
				want = 2
			}
			if err != nil || len(after.Observations) != want || !reflect.DeepEqual(after.Storyline, before.Storyline) {
				t.Fatalf("old cleanup removed replacement plan or kept old eyesight: %+v %v", after, err)
			}
			legacy := after.Observations[len(after.Observations)-1]
			if legacy.File != "unaffected.jpg" || legacy.Origins != nil {
				t.Fatal("unaffected legacy observation was backfilled/removed", legacy)
			}
			if mode == "new evidence" && (after.Observations[0].Origins == nil || after.Observations[0].Origins.Result != post.ObservationOriginIdentity(after.Observations[0]) || len(after.Observations[0].Origins.Spans) != 1) {
				t.Fatal("new incarnation observation lost matching stored evidence", after.Observations[0])
			}
		})
	}
}

func TestOriginPublicationPreservesOwnerEditedPlanThroughProseRevision(t *testing.T) {
	for _, annotations := range []*post.WriteAnnotations{nil, {Nouns: []string{"new noun"}}} {
		s := newStore(t)
		ctx := t.Context()
		seedPost(t, s, "p", "alice", testNow)
		paragraphs := []post.StorylineParagraph{{Text: "Known AI proposal"}}
		plan := post.Storyline{Paragraphs: paragraphs, Origins: &post.PlanOriginReview{Version: post.OriginVersion, Result: post.PlanOriginIdentity(paragraphs), Spans: []post.PlanOriginSpan{{ParagraphIndex: 0, Start: 0, End: len("Known AI proposal"), Quote: "Known AI proposal", Category: post.OriginAIAdded, ReviewState: post.OriginConfirmed}}}}
		expected := post.StorylineFingerprint(nil)
		if err := s.PublishStorylineResult(ctx, "alice", "p", post.StorylineOriginResult{Storyline: plan, ExpectedPlanFingerprint: &expected}, testNow); err != nil {
			t.Fatal(err)
		}
		manual := plan
		manual.EditedByHand = true
		manual.Paragraphs = append(append([]post.StorylineParagraph(nil), plan.Paragraphs...), post.StorylineParagraph{Text: "Explicit owner fact"})
		if changed, err := s.UpdateStoryline(ctx, "p", "alice", &manual, testNow); err != nil || !changed {
			t.Fatal(changed, err)
		}
		before, _ := s.GetPost(ctx, "p")
		content := post.PostContent{Blocks: []post.Block{{Type: post.BlockText, Content: "Valid revised prose"}}}
		in := originPublication(content, post.OriginAIAdded, "Valid revised prose")
		in.Annotations = annotations
		if _, err := s.PublishGeneratedResult(ctx, "alice", "p", in, testNow); err != nil {
			t.Fatal("owner-edited retained plan refused valid prose", err)
		}
		after, _ := s.GetPost(ctx, "p")
		if after.Storyline == nil || !reflect.DeepEqual(after.Storyline, before.Storyline) || after.InputRevision != before.InputRevision || after.ContentOrigins == nil {
			t.Fatalf("revision changed retained plan/evidence: before %+v, after %+v", before, after)
		}
	}
}

func TestOriginPublicationMalformedMetadataDoesNotRejectCanonicalOrBaseline(t *testing.T) {
	for _, defect := range []string{"version", "result", "unknown source", "source category", "offset"} {
		t.Run(defect, func(t *testing.T) {
			s, handle := newStoreWithHandle(t)
			ctx := t.Context()
			seedPost(t, s, "p", "alice", testNow)
			content := post.PostContent{Blocks: []post.Block{{Type: post.BlockText, Content: "Valid canonical"}}}
			in := originPublication(content, post.OriginAIAdded, "Valid canonical")
			switch defect {
			case "version":
				in.Origins.Version++
			case "result":
				in.Origins.Result.ContentHash = "another result"
			case "unknown source":
				in.Origins.Spans[0].SourceRefs = []string{"missing"}
			case "source category":
				in.Origins.Sources = []post.OriginSource{{ID: "ai", Kind: post.OriginSourceAIProposal, Text: "Frozen AI proposal", Available: true}}
				in.Origins.Spans[0].Category = post.OriginOwnerInput
				in.Origins.Spans[0].SourceRefs = []string{"ai"}
			case "offset":
				in.Origins.Spans[0].End++
			}
			if _, err := s.PublishGeneratedResult(ctx, "alice", "p", in, testNow); err != nil {
				t.Fatal("metadata refused valid content", err)
			}
			after, err := s.GetPost(ctx, "p")
			if err != nil || after.Content == nil || !reflect.DeepEqual(*after.Content, content) || after.Status != post.StatusReview || after.ContentRevision != after.MachineBaselineRevision || (after.ContentOrigins != nil && len(after.ContentOrigins.Spans) != 0) {
				t.Fatalf("metadata changed canonical/lifecycle: %+v, %v", after, err)
			}
			var canonical, baseline string
			if err := handle.Reader.QueryRow("SELECT content,machine_baseline FROM posts WHERE slug='p'").Scan(&canonical, &baseline); err != nil || canonical != baseline || strings.Contains(canonical, "origins") {
				t.Fatal("baseline or canonical polluted", canonical, baseline, err)
			}
			if _, err := handle.Writer.Exec("UPDATE posts SET content_origins='[broken', storyline_origins='{}',observations='[{\"file\":\"a.jpg\",\"scene\":\"Usable observation\",\"origins\":\"broken\"}]' WHERE slug='p'"); err != nil {
				t.Fatal(err)
			}
			read, err := s.GetPost(ctx, "p")
			if err != nil || read.ContentOrigins != nil || !reflect.DeepEqual(*read.Content, content) || len(read.Observations) != 1 || read.Observations[0].Scene != "Usable observation" || read.Observations[0].Origins != nil {
				t.Fatalf("corrupt sidecar broke usable canonical: %+v %v", read, err)
			}
		})
	}
}

func TestOriginPublicationAndManualSaveShareCurrentRevisionFence(t *testing.T) {
	s := newStore(t)
	ctx := t.Context()
	seedPost(t, s, "p", "alice", testNow)
	base := post.PostContent{Blocks: []post.Block{{Type: post.BlockText, Content: "Prior AI phrase"}}}
	if _, err := s.PublishGeneratedResult(ctx, "alice", "p", originPublication(base, post.OriginAIAdded, "Prior AI phrase"), testNow); err != nil {
		t.Fatal(err)
	}
	manual := post.PostContent{Blocks: []post.Block{{Type: post.BlockText, Content: "Prior AI phrase. Explicit new input"}}}
	machine := post.PostContent{Blocks: []post.Block{{Type: post.BlockText, Content: "New machine result"}}}
	in := originPublication(machine, post.OriginAIAdded, "New machine result")
	in.ExpectedContentRevision = 1
	start := make(chan struct{})
	var group sync.WaitGroup
	var machineErr, manualErr error
	var saved bool
	group.Add(2)
	go func() {
		defer group.Done()
		<-start
		_, machineErr = s.PublishGeneratedResult(ctx, "alice", "p", in, testNow)
	}()
	go func() {
		defer group.Done()
		<-start
		saved, manualErr = s.SaveContent(ctx, "p", "alice", manual, 1, testNow)
	}()
	close(start)
	group.Wait()
	if manualErr != nil || (machineErr != nil && !errors.Is(machineErr, post.ErrStaleContentRevision)) || saved == (machineErr == nil) {
		t.Fatalf("revision fence admitted both/neither: manual %v %v, machine %v", saved, manualErr, machineErr)
	}
	after, err := s.GetPost(ctx, "p")
	if err != nil || after.ContentRevision != 2 || after.ContentOrigins == nil || after.ContentOrigins.Result != post.ContentOriginIdentity(*after.Content, 2) {
		t.Fatalf("race left mismatched sidecar: %+v %v", after, err)
	}
	if saved {
		if !reflect.DeepEqual(*after.Content, manual) || after.MachineBaselineRevision != 1 {
			t.Fatal("manual winner lost canonical/baseline", after)
		}
	} else if !reflect.DeepEqual(*after.Content, machine) || after.MachineBaselineRevision != 2 {
		t.Fatal("machine winner lost canonical/baseline", after)
	}
}

func TestOriginManualStoreAlignmentKeepsPunctuationReorderAndNewInputConservative(t *testing.T) {
	for _, scenario := range []string{"punctuation", "unique reorder", "new block", "duplicate removal", "replacement"} {
		t.Run(scenario, func(t *testing.T) {
			s := newStore(t)
			ctx := t.Context()
			seedPost(t, s, "p", "alice", testNow)
			before := post.PostContent{Blocks: []post.Block{{Type: post.BlockText, Content: "AI phrase."}, {Type: post.BlockText, Content: "Another meaning"}}}
			if scenario == "duplicate removal" {
				before.Blocks[1].Content = before.Blocks[0].Content
			}
			in := originPublication(before, post.OriginAIAdded, "AI phrase")
			if _, err := s.PublishGeneratedResult(ctx, "alice", "p", in, testNow); err != nil {
				t.Fatal(err)
			}
			afterContent := post.PostContent{Blocks: append([]post.Block(nil), before.Blocks...)}
			switch scenario {
			case "punctuation":
				afterContent.Blocks[0].Content = "(AI phrase)!"
			case "unique reorder":
				afterContent.Blocks[0], afterContent.Blocks[1] = afterContent.Blocks[1], afterContent.Blocks[0]
			case "new block":
				afterContent.Blocks = append(afterContent.Blocks, post.Block{Type: post.BlockText, Content: "Explicit new fact"})
			case "duplicate removal":
				afterContent.Blocks = afterContent.Blocks[:1]
			case "replacement":
				afterContent.Blocks[0].Content = "Another way to phrase the meaning"
			}
			if changed, err := s.SaveContent(ctx, "p", "alice", afterContent, 1, testNow); err != nil || !changed {
				t.Fatal(changed, err)
			}
			after, err := s.GetPost(ctx, "p")
			if err != nil || !reflect.DeepEqual(*after.Content, afterContent) || after.ContentRevision != 2 || after.MachineBaselineRevision != 1 {
				t.Fatalf("manual canonical/baseline: %+v, %v", after, err)
			}
			var spans []post.OriginSpan
			if after.ContentOrigins != nil {
				spans = after.ContentOrigins.Spans
				if after.ContentOrigins.Result != post.ContentOriginIdentity(*after.Content, after.ContentRevision) {
					t.Fatal("manual review belongs to another result")
				}
			}
			if scenario == "duplicate removal" || scenario == "replacement" {
				if len(spans) != 0 {
					t.Fatalf("ambiguous edit guessed an origin: %+v", spans)
				}
				return
			}
			ai, owner := 0, 0
			for _, span := range spans {
				switch span.Category {
				case post.OriginAIAdded:
					ai++
					if span.Quote != "AI phrase" || (scenario == "unique reorder" && *span.Field.BlockIndex != 1) {
						t.Fatal("reorder matched old index", span)
					}
				case post.OriginOwnerInput:
					owner++
					if scenario != "new block" || span.Quote != "Explicit new fact" || len(span.SourceRefs) != 1 || !strings.HasPrefix(span.SourceRefs[0], "manual-2.") {
						t.Fatal("save promoted old AI meaning", span)
					}
				}
			}
			if ai != 1 || (scenario == "new block" && owner != 1) || (scenario != "new block" && owner != 0) {
				t.Fatalf("known/new origins: %+v", spans)
			}
		})
	}
}

func TestOriginVideoDeletionWithdrawsContentAndPlanEvidenceWithoutRetargeting(t *testing.T) {
	s, handle := newStoreWithHandle(t)
	ctx := t.Context()
	seedPost(t, s, "p", "alice", testNow)
	video := post.Video{ID: "old-video", PostSlug: "p", Filename: "a.mp4", Key: "old-private-key", ContentType: "video/mp4", Width: 100, Height: 100, Bytes: 1, DurationMs: 1000, CreatedAt: testNow}
	if err := s.CreateVideo(ctx, video); err != nil {
		t.Fatal(err)
	}
	source := post.OriginSource{ID: "video-evidence", Kind: post.OriginSourceVisualObservation, Text: "Frozen blue wall observation", AttachmentFilename: "a.mp4", AttachmentID: "old-video", Available: true}
	content := post.PostContent{Blocks: []post.Block{{Type: post.BlockText, Content: "Blue wall"}}}
	index := 0
	review := post.ResolveOriginCandidates(content, post.ContentOriginIdentity(content, 0), []post.OriginSource{source}, []post.OriginCandidate{{Field: post.OriginFieldLocator{Kind: post.OriginFieldBlockContent, BlockIndex: &index}, Quote: "Blue wall", Category: post.OriginPhotoInterpretation, SourceRefs: []string{source.ID}}}).Review
	paragraphs := []post.StorylineParagraph{{Text: "Blue wall", Files: []string{"a.mp4"}}}
	plan := post.Storyline{Paragraphs: paragraphs, MadeWith: []string{"a.mp4"}, Origins: &post.PlanOriginReview{Version: post.OriginVersion, Result: post.PlanOriginIdentity(paragraphs), Sources: []post.OriginSource{source}, Spans: []post.PlanOriginSpan{{ParagraphIndex: 0, Start: 0, End: len("Blue wall"), Quote: "Blue wall", Category: post.OriginPhotoInterpretation, SourceRefs: []string{source.ID}, ReviewState: post.OriginUnconfirmed}}}}
	in := post.GeneratedOriginResult{Content: content, Language: post.LanguageEnglish, Origins: &review, Annotations: &post.WriteAnnotations{Storyline: &plan}}
	if _, err := s.PublishGeneratedResult(ctx, "alice", "p", in, testNow); err != nil {
		t.Fatal(err)
	}
	first, _ := s.GetPost(ctx, "p")
	if first.ContentOrigins == nil || first.Storyline.Origins == nil || len(first.Storyline.Origins.Spans) != 1 {
		t.Fatal("video/plan origin not stored", first)
	}
	if deleted, err := s.DeleteVideo(ctx, "old-video"); err != nil || !deleted {
		t.Fatal(deleted, err)
	}
	if _, err := s.UpdateAttachmentTraces(ctx, "p", "alice", "a.mp4", "old-video", testNow); err != nil {
		t.Fatal(err)
	}
	video.ID, video.Key = "new-video", "new-private-key"
	if err := s.CreateVideo(ctx, video); err != nil {
		t.Fatal(err)
	}
	in.ExpectedContentRevision = 1
	if _, err := s.PublishGeneratedResult(ctx, "alice", "p", in, testNow); err != nil {
		t.Fatal("unavailable sidecar refused usable canonical", err)
	}
	after, err := s.GetPost(ctx, "p")
	if err != nil || after.ContentOrigins == nil || after.Storyline == nil || after.Storyline.Origins == nil {
		t.Fatal(after, err)
	}
	for _, sources := range [][]post.OriginSource{after.ContentOrigins.Sources, after.Storyline.Origins.Sources} {
		if len(sources) != 1 || sources[0].Available || sources[0].AttachmentID != "old-video" || sources[0].Text != source.Text {
			t.Fatal("deleted source silently retargeted", sources)
		}
	}
	if len(after.ContentOrigins.Spans) != 0 || len(after.Storyline.Origins.Spans) != 0 {
		t.Fatal("withdrawn video remained phrase evidence", after)
	}
	var rawContent, rawPlan string
	if err := handle.Reader.QueryRow("SELECT content_origins,storyline_origins FROM posts WHERE slug='p'").Scan(&rawContent, &rawPlan); err != nil || !strings.Contains(rawContent, `"available":false`) || !strings.Contains(rawPlan, `"available":false`) {
		t.Fatal("withdrawal was not persisted", rawContent, rawPlan, err)
	}
}

func TestOriginAccountDeletionRemovesAllSidecarsAndReceiptsWithoutResurrection(t *testing.T) {
	s, handle := newStoreWithHandle(t)
	ctx := t.Context()
	seedPost(t, s, "p", "alice", testNow)
	seedPost(t, s, "other", "bob", testNow)
	in := frozenPublication(t, s)
	index := 0
	review := post.ResolveOriginCandidates(in.Content, post.ContentOriginIdentity(in.Content, 0), []post.OriginSource{{ID: "memo", Kind: post.OriginSourceMemo, Text: "PRIVATE CONTENT EVIDENCE", Available: true}}, []post.OriginCandidate{{Field: post.OriginFieldLocator{Kind: post.OriginFieldBlockContent, BlockIndex: &index}, Quote: "Winner's complete output", Category: post.OriginOwnerInput, SourceRefs: []string{"memo"}}}).Review
	in.Origins = &review
	in.Storyline.Origins = &post.PlanOriginReview{Version: post.OriginVersion, Result: post.PlanOriginIdentity(in.Storyline.Paragraphs), Sources: []post.OriginSource{{ID: "plan", Kind: post.OriginSourceAIProposal, Text: "PRIVATE PLAN EVIDENCE", Available: true}}, Spans: []post.PlanOriginSpan{{ParagraphIndex: 0, Start: 0, End: len("Winner's plan"), Quote: "Winner's plan", Category: post.OriginAIAdded, SourceRefs: []string{"plan"}, ReviewState: post.OriginUnconfirmed}}}
	publications := publicationStore(handle)
	if _, err := publications.ApplyTestResult(ctx, in, testNow); err != nil {
		t.Fatal(err)
	}
	observation := post.Observation{File: "a.jpg", Scene: "Visible blue wall", Origins: &post.ObservationOriginReview{Version: post.OriginVersion, Result: post.OriginResultIdentity{ContentHash: "frozen-observation"}, Sources: []post.OriginSource{{ID: "observation", Kind: post.OriginSourceVisualObservation, Text: "PRIVATE OBSERVATION EVIDENCE", AttachmentFilename: "a.jpg", AttachmentID: "old-photo", Available: true}}, Spans: []post.ObservationOriginSpan{{Field: "scene", Start: 0, End: len("Visible blue wall"), Quote: "Visible blue wall", Category: post.OriginPhotoInterpretation, SourceRefs: []string{"observation"}, ReviewState: post.OriginUnconfirmed}}}}
	if _, err := s.UpdateObservations(ctx, "p", "alice", []post.Observation{observation}, testNow); err != nil {
		t.Fatal(err)
	}
	var rawContent, rawPlan, rawObservation string
	if err := handle.Reader.QueryRow("SELECT content_origins,storyline_origins,observations FROM posts WHERE slug='p'").Scan(&rawContent, &rawPlan, &rawObservation); err != nil || !strings.Contains(rawContent, "PRIVATE CONTENT EVIDENCE") || !strings.Contains(rawPlan, "PRIVATE PLAN EVIDENCE") || !strings.Contains(rawObservation, "PRIVATE OBSERVATION EVIDENCE") {
		t.Fatal("fixture missing private sidecar evidence", err)
	}
	if _, err := handle.Writer.Exec("DELETE FROM users WHERE id='alice'"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetPost(ctx, "p"); !errors.Is(err, post.ErrNotFound) {
		t.Fatal("owner deletion retained private post", err)
	}
	if _, err := s.PublishGeneratedResult(ctx, "alice", "p", originPublication(in.Content, post.OriginAIAdded, "Winner's complete output"), testNow); !errors.Is(err, post.ErrNotFound) {
		t.Fatal("late content callback restored owner data", err)
	}
	expected := post.StorylineFingerprint(nil)
	if err := s.PublishStorylineResult(ctx, "alice", "p", post.StorylineOriginResult{Storyline: *in.Storyline, ExpectedPlanFingerprint: &expected}, testNow); !errors.Is(err, post.ErrNotFound) {
		t.Fatal("late plan callback restored owner data", err)
	}
	if _, err := publications.ApplyTestResult(ctx, in, testNow); !errors.Is(err, post.ErrNotFound) {
		t.Fatal("late test callback restored owner data", err)
	}
	if _, found, err := publications.ReadTestPublicationReceipt(ctx, "alice", in.TestID, in.WinnerID); err != nil || found {
		t.Fatal("owner receipt retained", found, err)
	}
	if _, err := s.GetPost(ctx, "other"); err != nil {
		t.Fatal("owner deletion changed another owner's post", err)
	}
}

func TestCurrentOriginStorageAtomicCASManualAlignmentAndLostResponse(t *testing.T) {
	s, handle := newStoreWithHandle(t)
	ctx := context.Background()
	seedPost(t, s, "p", "alice", testNow)
	content := post.PostContent{Title: "Title", Tags: []string{}, Blocks: []post.Block{{Type: post.BlockText, Content: "Known AI meaning 🍰."}}}
	in := originPublication(content, post.OriginAIAdded, "Known AI meaning 🍰.")
	first, err := s.PublishGeneratedResult(ctx, "alice", "p", in, testNow)
	if err != nil || first.ContentRevision != 1 {
		t.Fatal(first, err)
	}
	read, err := s.GetPost(ctx, "p")
	if err != nil || read.ContentOrigins == nil || len(read.ContentOrigins.Spans) != 1 || read.ContentOrigins.Result != post.ContentOriginIdentity(*read.Content, read.ContentRevision) {
		t.Fatal("origin result mismatched stored canonical", read, err)
	}
	var canonical, baseline string
	if err := handle.Reader.QueryRow("SELECT content,machine_baseline FROM posts WHERE slug='p'").Scan(&canonical, &baseline); err != nil || canonical != baseline {
		t.Fatal("metadata changed baseline", canonical, baseline, err)
	}
	var wire map[string]json.RawMessage
	if json.Unmarshal([]byte(canonical), &wire) != nil || wire["origins"] != nil {
		t.Fatal("presentation metadata entered canonical", canonical)
	}
	replay, err := s.PublishGeneratedResult(ctx, "alice", "p", in, testNow.Add(time.Minute))
	if err != nil || replay != first {
		t.Fatal("lost response changed result", replay, err)
	}
	manual := *read.Content
	manual.Blocks = append([]post.Block(nil), manual.Blocks...)
	manual.Blocks[0].Content = "A new prefix. " + manual.Blocks[0].Content
	if changed, err := s.SaveContent(ctx, "p", "alice", manual, 1, testNow.Add(time.Hour)); err != nil || !changed {
		t.Fatal("manual CAS", changed, err)
	}
	after, _ := s.GetPost(ctx, "p")
	if after.ContentRevision != 2 || after.MachineBaselineRevision != 1 || after.ContentOrigins == nil {
		t.Fatal("manual baseline/revision/sidecar", after)
	}
	retained := false
	for _, span := range after.ContentOrigins.Spans {
		if span.Quote == "Known AI meaning 🍰." && span.Category == post.OriginAIAdded {
			retained = true
		}
	}
	if !retained {
		t.Fatal("manual text promoted/dropped unchanged AI meaning", after.ContentOrigins)
	}
	if _, err := s.PublishGeneratedResult(ctx, "alice", "p", in, testNow.Add(2*time.Hour)); !errors.Is(err, post.ErrStaleContentRevision) {
		t.Fatal("late callback rewrote manual edit", err)
	}
	stable, _ := s.GetPost(ctx, "p")
	if !reflect.DeepEqual(*stable.Content, manual) {
		t.Fatal("late callback changed canonical")
	}
	if review, err := s.ReadOriginReview(ctx, "bob", "p", after.ContentOrigins.Result); !errors.Is(err, post.ErrForbidden) || review != nil {
		t.Fatal("foreign review", review, err)
	}
	if changed, err := s.DeletePost(ctx, "p", "alice"); err != nil || !changed {
		t.Fatal(changed, err)
	}
	if _, err := s.PublishGeneratedResult(ctx, "alice", "p", in, testNow); !errors.Is(err, post.ErrNotFound) {
		t.Fatal("deleted callback restored data", err)
	}
}

func TestOriginPlanLostResponseCannotUndoManualPlanAndOwnRotationIsAllowed(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	seedPost(t, s, "p", "alice", testNow)
	image := post.Image{ID: "photo", PostSlug: "p", Filename: "a.jpg", Key: "key", Width: 100, Height: 100, Bytes: 1, CreatedAt: testNow}
	if err := s.CreateImage(ctx, image); err != nil {
		t.Fatal(err)
	}
	before, _ := s.GetPost(ctx, "p")
	expected := post.StorylineFingerprint(before.Storyline)
	plan := post.Storyline{Paragraphs: []post.StorylineParagraph{{Text: "Initial plan", Files: []string{"a.jpg"}}}, MadeWith: []string{"a.jpg"}}
	in := post.StorylineOriginResult{Storyline: plan, ExpectedContentRevision: before.ContentRevision, ExpectedPlanFingerprint: &expected}
	if changed, err := s.UpdateObservations(ctx, "p", "alice", []post.Observation{{File: "a.jpg", Scene: "Photo", Rotation: 90}}, testNow); err != nil || !changed {
		t.Fatal(changed, err)
	}
	if err := s.PublishStorylineResult(ctx, "alice", "p", in, testNow); err != nil {
		t.Fatal("own rotation invalidated plan target", err)
	}
	if err := s.PublishStorylineResult(ctx, "alice", "p", in, testNow); err != nil {
		t.Fatal("exact committed plan replay", err)
	}
	manual := plan
	manual.EditedByHand = true
	manual.Paragraphs = []post.StorylineParagraph{{Text: "Later manual plan", Files: []string{"a.jpg"}}}
	if changed, err := s.UpdateStoryline(ctx, "p", "alice", &manual, testNow); err != nil || !changed {
		t.Fatal(changed, err)
	}
	if err := s.PublishStorylineResult(ctx, "alice", "p", in, testNow); !errors.Is(err, post.ErrStaleContentRevision) {
		t.Fatal("old plan undid manual edit", err)
	}
	after, _ := s.GetPost(ctx, "p")
	if after.Storyline == nil || after.Storyline.Paragraphs[0].Text != "Later manual plan" {
		t.Fatal("plan callback changed manual plan", after)
	}
}

func TestOriginVisualEvidenceBindsActualIncarnationAndCannotRestoreReusedFilename(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	seedPost(t, s, "p", "alice", testNow)
	image := post.Image{ID: "old-photo", PostSlug: "p", Filename: "a.jpg", Key: "old-key", Width: 100, Height: 100, Bytes: 1, CreatedAt: testNow}
	if err := s.CreateImage(ctx, image); err != nil {
		t.Fatal(err)
	}
	content := post.PostContent{Blocks: []post.Block{{Type: post.BlockText, Content: "Blue wall"}}}
	index := 0
	sources := []post.OriginSource{{ID: "visual", Kind: post.OriginSourceVisualObservation, Text: "Blue wall", AttachmentFilename: "a.jpg", AttachmentID: "old-photo", Available: true}}
	review := post.ResolveOriginCandidates(content, post.ContentOriginIdentity(content, 0), sources, []post.OriginCandidate{{Field: post.OriginFieldLocator{Kind: post.OriginFieldBlockContent, BlockIndex: &index}, Quote: "Blue wall", Category: post.OriginPhotoInterpretation, SourceRefs: []string{"visual"}}}).Review
	in := post.GeneratedOriginResult{Content: content, Language: post.LanguageEnglish, Origins: &review}
	if _, err := s.PublishGeneratedResult(ctx, "alice", "p", in, testNow); err != nil {
		t.Fatal(err)
	}
	first, _ := s.GetPost(ctx, "p")
	if first.ContentOrigins == nil || len(first.ContentOrigins.Spans) != 1 {
		t.Fatal("valid current visual capture lost", first)
	}
	if changed, err := s.DeleteImage(ctx, "old-photo"); err != nil || !changed {
		t.Fatal(changed, err)
	}
	image.ID, image.Key = "new-photo", "new-key"
	if err := s.CreateImage(ctx, image); err != nil {
		t.Fatal(err)
	}
	in.ExpectedContentRevision = 1
	if _, err := s.PublishGeneratedResult(ctx, "alice", "p", in, testNow); err != nil {
		t.Fatal("valid canonical result rejected with unavailable evidence", err)
	}
	after, _ := s.GetPost(ctx, "p")
	if after.ContentOrigins == nil || len(after.ContentOrigins.Spans) != 0 || len(after.ContentOrigins.Sources) != 1 || after.ContentOrigins.Sources[0].Available {
		t.Fatal("old visual evidence retargeted reused filename", after.ContentOrigins)
	}
}
