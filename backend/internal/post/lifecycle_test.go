package post

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestFinalizedLifecycleKeepsIdenticalSavesAndDemotesChangedContent(t *testing.T) {
	svc, _, _ := newTestService(t)
	created := mustCreatePost(t, svc, alice, "Final")
	content := PostContent{Title: "완성", Blocks: []Block{{Type: BlockText, Content: "생성 문장"}}}
	if err := svc.SetGeneratedContent(context.Background(), alice, created.Slug, content, LanguageKorean, nil); err != nil {
		t.Fatal(err)
	}
	target := 900
	withOption, err := svc.SaveGenerationOptions(context.Background(), alice, created.Slug, optionsSet(t, svc, alice, created.Slug, func(s *GenerationOptionsSet) { s.TargetLength = &target }))
	if err != nil || withOption.ContentRevision != 1 || withOption.Status != StatusReview {
		t.Fatalf("option changed lifecycle: %+v err=%v", withOption, err)
	}
	finalized, err := svc.Finalize(context.Background(), alice, created.Slug, 1)
	if err != nil || finalized.Status != StatusFinalized || finalized.FinalizedRevision != 1 {
		t.Fatalf("finalize = %+v err=%v", finalized, err)
	}
	unchanged, err := svc.SaveContent(context.Background(), alice, created.Slug, content, 1)
	if err != nil || unchanged.Status != StatusFinalized || unchanged.ContentRevision != 1 {
		t.Fatalf("identical save = %+v err=%v", unchanged, err)
	}
	changed := PostContent{Title: "직접 수정", Blocks: []Block{{Type: BlockText, Content: "내 문장"}}}
	review, err := svc.SaveContent(context.Background(), alice, created.Slug, changed, 1)
	if err != nil || review.Status != StatusReview || review.ContentRevision != 2 || review.FinalizedRevision != 0 {
		t.Fatalf("changed save = %+v err=%v", review, err)
	}
	if _, err := svc.LearningSnapshot(context.Background(), alice, created.Slug); !errors.Is(err, ErrPostNotFinalized) {
		t.Fatalf("learning snapshot before re-finalize = %v", err)
	}
}

func TestFinalizeIsRevisionCheckedOwnedAndIdempotent(t *testing.T) {
	svc, _, _ := newTestService(t)
	created := mustCreatePost(t, svc, alice, "Final")
	if _, err := svc.Finalize(context.Background(), alice, created.Slug, 0); !errors.Is(err, ErrNoMachineBaseline) {
		t.Fatalf("finalize without baseline = %v", err)
	}
	content := PostContent{Blocks: []Block{{Type: BlockText, Content: "본문"}}}
	if err := svc.SetGeneratedContent(context.Background(), alice, created.Slug, content, LanguageKorean, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Finalize(context.Background(), bob, created.Slug, 1); !errors.Is(err, ErrForbidden) {
		t.Fatalf("foreign finalize = %v", err)
	}
	if _, err := svc.Finalize(context.Background(), alice, created.Slug, 0); !errors.Is(err, ErrStaleContentRevision) {
		t.Fatalf("stale finalize = %v", err)
	}
	first, err := svc.Finalize(context.Background(), alice, created.Slug, 1)
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.Finalize(context.Background(), alice, created.Slug, 1)
	if err != nil || second.FinalizedAt == nil || !second.FinalizedAt.Equal(*first.FinalizedAt) {
		t.Fatalf("idempotent finalize = %+v err=%v", second, err)
	}
}

func TestFinalizeAllowsCrossLanguageContentAndPreservesProvenance(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	voiceID := aliceVoice // Korean source voice.
	target := LanguageEnglish
	created, err := svc.SaveDraft(ctx, alice, DraftSave{Title: "English target", VoiceID: &voiceID, TargetLanguage: &target})
	if err != nil {
		t.Fatal(err)
	}
	content := PostContent{Title: "An English post", Blocks: []Block{{Type: BlockText, Content: "English content remains publishable under a Korean source voice."}}}
	if err := svc.SetGeneratedContent(ctx, alice, created.Slug, content, LanguageEnglish, nil); err != nil {
		t.Fatal(err)
	}
	finalized, err := svc.Finalize(ctx, alice, created.Slug, 1)
	if err != nil || finalized.Status != StatusFinalized || finalized.TargetLanguage != LanguageEnglish || finalized.ContentLanguage == nil || *finalized.ContentLanguage != LanguageEnglish || finalized.Voice.SourceLanguage != LanguageKorean {
		t.Fatalf("cross-language finalize = %#v, err=%v", finalized, err)
	}
	learning, err := svc.LearningSnapshot(ctx, alice, created.Slug)
	if err != nil || learning.ContentLanguage != LanguageEnglish || learning.VoiceSourceLanguage != LanguageKorean {
		t.Fatalf("learning language projection = %#v, err=%v", learning, err)
	}
}

// A8/A9: 확정 copies the confirmed content title into posts.title, and nothing else moves with it.
func TestFinalizeCopiesContentTitleIntoThePost(t *testing.T) {
	svc, store, _ := newTestService(t)
	ctx := context.Background()
	created := mustCreatePost(t, svc, alice, "가제")
	content := PostContent{Title: "  모델이 지은 제목  ", Blocks: []Block{{Type: BlockText, Content: "본문"}}}
	if err := svc.SetGeneratedContent(ctx, alice, created.Slug, content, LanguageKorean, nil); err != nil {
		t.Fatal(err)
	}
	// The list read falls back to content.title only while the working title is empty, so a
	// draft is still listed under its 가제 before the confirmation.
	before, err := svc.List(ctx, alice)
	if err != nil || len(before) != 1 || before[0].Title != "가제" {
		t.Fatalf("list before finalize = %+v err=%v", before, err)
	}

	finalized, err := svc.Finalize(ctx, alice, created.Slug, 1)
	if err != nil {
		t.Fatal(err)
	}
	if finalized.Title != "모델이 지은 제목" {
		t.Fatalf("finalized title = %q", finalized.Title)
	}
	if finalized.Slug != created.Slug {
		t.Fatalf("slug moved: %q -> %q", created.Slug, finalized.Slug)
	}
	if finalized.ContentRevision != 1 || finalized.MachineBaselineRevision != 1 {
		t.Fatalf("revision moved: %+v", finalized)
	}
	after, err := svc.List(ctx, alice)
	if err != nil || len(after) != 1 || after[0].Title != "모델이 지은 제목" {
		t.Fatalf("list after finalize = %+v err=%v", after, err)
	}
	// The copy is not a content save: it starts no job and calls no provider, which is what
	// keeping it inside the one guarded UPDATE buys.
	stored, err := store.GetPost(ctx, created.Slug)
	if err != nil || stored.Title != "모델이 지은 제목" || stored.Content == nil || stored.Content.Title != "  모델이 지은 제목  " {
		t.Fatalf("stored post = %+v err=%v", stored, err)
	}
}

func TestFinalizeLeavesTheWorkingTitleWhenTheContentHasNone(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	created := mustCreatePost(t, svc, alice, "가제")
	content := PostContent{Title: "   ", Blocks: []Block{{Type: BlockText, Content: "본문"}}}
	if err := svc.SetGeneratedContent(ctx, alice, created.Slug, content, LanguageKorean, nil); err != nil {
		t.Fatal(err)
	}
	finalized, err := svc.Finalize(ctx, alice, created.Slug, 1)
	if err != nil || finalized.Title != "가제" {
		t.Fatalf("untitled finalize = %+v err=%v", finalized, err)
	}
}

// The already-finalized early return happens BEFORE the store call, so a second confirmation of
// the same revision cannot re-copy over a title the user has edited since.
func TestSecondFinalizeOfTheSameRevisionDoesNotRewriteTheTitle(t *testing.T) {
	svc, store, _ := newTestService(t)
	ctx := context.Background()
	created := mustCreatePost(t, svc, alice, "가제")
	content := PostContent{Title: "모델 제목", Blocks: []Block{{Type: BlockText, Content: "본문"}}}
	if err := svc.SetGeneratedContent(ctx, alice, created.Slug, content, LanguageKorean, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Finalize(ctx, alice, created.Slug, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SaveDraft(ctx, alice, DraftSave{Slug: created.Slug, Title: "사람이 고친 제목"}); err != nil {
		t.Fatal(err)
	}
	again, err := svc.Finalize(ctx, alice, created.Slug, 1)
	if err != nil || again.Title != "사람이 고친 제목" {
		t.Fatalf("re-finalize = %+v err=%v", again, err)
	}
	stored, err := store.GetPost(ctx, created.Slug)
	if err != nil || stored.Title != "사람이 고친 제목" {
		t.Fatalf("stored title = %+v err=%v", stored, err)
	}
}

// publishedFixture is a published post carrying everything the lock has to leave alone: an
// answer, a confirmed photo and clip, and a pending photo and clip upload whose objects landed.
type publishedFixture struct {
	slug, image, video, photoUpload, videoUpload string
	revision                                     int64
}

func attachPhoto(t *testing.T, svc *Service, blobs *fakeBlobs, slug, filename string) Image {
	t.Helper()
	upload, _, _, err := svc.CreateUpload(context.Background(), alice, slug, filename, AttachmentPhoto)
	if err != nil {
		t.Fatalf("CreateUpload(%s): %v", filename, err)
	}
	blobs.put(upload.Key, 200_000, testNow)
	confirmed, err := svc.ConfirmUpload(context.Background(), alice, upload.ID, 1024, 768, 0)
	if err != nil {
		t.Fatalf("ConfirmUpload(%s): %v", filename, err)
	}
	return confirmed.Image
}

func newPublishedFixture(t *testing.T) (*Service, *fakeStore, *fakeBlobs, publishedFixture) {
	t.Helper()
	ctx := context.Background()
	svc, store, blobs := newTestService(t)
	svc.SetTemplateDirectory(testTemplates())
	svc.answerLabelMax, svc.answerValueMax = 40, 500
	created := mustCreatePost(t, svc, alice, "제주 3일")
	voiceID := aliceVoice
	if _, err := svc.SaveDraft(ctx, alice, DraftSave{Slug: created.Slug, Title: created.Title, VoiceID: &voiceID, Answers: []TemplateAnswer{{Label: "총평", Text: "맑았다", Enabled: true}}}); err != nil {
		t.Fatal(err)
	}
	image := attachPhoto(t, svc, blobs, created.Slug, "IMG_1.jpg")
	video := mustAttachVideo(t, svc, blobs, alice, created.Slug, "clip.mp4", 3000)
	photoUpload, _, _, err := svc.CreateUpload(ctx, alice, created.Slug, "IMG_2.jpg", AttachmentPhoto)
	if err != nil {
		t.Fatal(err)
	}
	blobs.put(photoUpload.Key, 200_000, testNow)
	videoUpload, _, contentType, err := svc.CreateUpload(ctx, alice, created.Slug, "clip2.mp4", AttachmentVideo)
	if err != nil {
		t.Fatal(err)
	}
	blobs.putTyped(videoUpload.Key, 1_000_000, contentType, testNow)

	content := PostContent{Title: "제주 3일 기록", Blocks: []Block{{Type: BlockText, Content: "협재 해변은 물빛이 맑았다."}}}
	if err := svc.SetGeneratedContent(ctx, alice, created.Slug, content, LanguageKorean, nil); err != nil {
		t.Fatal(err)
	}
	finalized, err := svc.Finalize(ctx, alice, created.Slug, 1)
	if err != nil {
		t.Fatal(err)
	}
	published, err := svc.SavePublishedURL(ctx, alice, created.Slug, firstAddress)
	if err != nil || published.Status != StatusPublished {
		t.Fatalf("publish = %s, %v", published.Status, err)
	}
	return svc, store, blobs, publishedFixture{
		slug: created.Slug, image: image.ID, video: video.ID,
		photoUpload: photoUpload.ID, videoUpload: videoUpload.ID, revision: finalized.ContentRevision,
	}
}

// lockedState is everything a refused write must leave exactly as it was.
type lockedState struct {
	post    Post
	answers map[string]TemplateAnswer
	images  map[string]Image
	videos  map[string]Video
	uploads map[string]Upload
	deleted []string
}

func lockedSnapshot(store *fakeStore, blobs *fakeBlobs, slug string) lockedState {
	store.mu.Lock()
	defer store.mu.Unlock()
	blobs.mu.Lock()
	defer blobs.mu.Unlock()
	state := lockedState{
		post: store.posts[slug], answers: map[string]TemplateAnswer{}, images: map[string]Image{},
		videos: map[string]Video{}, uploads: map[string]Upload{}, deleted: append([]string(nil), blobs.deleted...),
	}
	for label, answer := range store.answers[slug] {
		state.answers[label] = answer
	}
	for id, image := range store.images {
		state.images[id] = image
	}
	for id, video := range store.videos {
		state.videos[id] = video
	}
	for id, upload := range store.uploads {
		state.uploads[id] = upload
	}
	return state
}

var (
	lockedEdit     = PostContent{Title: "직접 수정", Blocks: []Block{{Type: BlockText, Content: "내 문장"}}}
	lockedEnglish  = LanguageEnglish
	lockedReview   = aliceReview
	lockedTemplate = "template-review"
)

// publishedLockGuarded holds every exported Service method the published lock refuses (POST-74),
// keyed by method name and then by case, each run against the published fixture.
var publishedLockGuarded = map[string]map[string]func(*Service, publishedFixture) error{
	"SaveDraft": {
		"title and memo": func(svc *Service, f publishedFixture) error {
			_, err := svc.SaveDraft(context.Background(), alice, DraftSave{Slug: f.slug, Title: "새 제목", Memo: "새 메모"})
			return err
		},
		"voice": func(svc *Service, f publishedFixture) error {
			_, err := svc.SaveDraft(context.Background(), alice, DraftSave{Slug: f.slug, Title: "제주 3일 기록", VoiceID: &lockedReview})
			return err
		},
		"template": func(svc *Service, f publishedFixture) error {
			_, err := svc.SaveDraft(context.Background(), alice, DraftSave{Slug: f.slug, Title: "제주 3일 기록", TemplateID: &lockedTemplate})
			return err
		},
		"answers": func(svc *Service, f publishedFixture) error {
			_, err := svc.SaveDraft(context.Background(), alice, DraftSave{Slug: f.slug, Title: "제주 3일 기록", Answers: []TemplateAnswer{{Label: "총평", Text: "흐렸다", Enabled: true}}})
			return err
		},
		"target language": func(svc *Service, f publishedFixture) error {
			_, err := svc.SaveDraft(context.Background(), alice, DraftSave{Slug: f.slug, Title: "제주 3일 기록", TargetLanguage: &lockedEnglish})
			return err
		},
		"field": func(svc *Service, f publishedFixture) error {
			_, err := svc.SaveDraft(context.Background(), alice, DraftSave{Slug: f.slug, Title: "제주 3일 기록", Field: fieldPtr("cafe")})
			return err
		},
	},
	"SetObservations": {
		"a new contact sheet": func(svc *Service, f publishedFixture) error {
			return svc.SetObservations(context.Background(), alice, f.slug, []Observation{{File: "IMG_1.jpg", Scene: "바다"}})
		},
	},
	"SetGeneratedContent": {
		"a machine write": func(svc *Service, f publishedFixture) error {
			return svc.SetGeneratedContent(context.Background(), alice, f.slug, lockedEdit, LanguageKorean, nil)
		},
	},
	"SaveContent": {
		"a changed save": func(svc *Service, f publishedFixture) error {
			_, err := svc.SaveContent(context.Background(), alice, f.slug, lockedEdit, f.revision)
			return err
		},
	},
	"SaveGenerationOptions": {
		"the post's set with the length changed": func(svc *Service, f publishedFixture) error {
			found, err := svc.Get(context.Background(), alice, f.slug)
			if err != nil {
				return err
			}
			set := found.GenerationOptions()
			length := 1200
			set.TargetLength = &length
			_, err = svc.SaveGenerationOptions(context.Background(), alice, f.slug, set)
			return err
		},
	},
	"Finalize": {
		"the current revision": func(svc *Service, f publishedFixture) error {
			_, err := svc.Finalize(context.Background(), alice, f.slug, f.revision)
			return err
		},
	},
	"CreateUpload": {
		"a photo": func(svc *Service, f publishedFixture) error {
			_, _, _, err := svc.CreateUpload(context.Background(), alice, f.slug, "IMG_3.jpg", AttachmentPhoto)
			return err
		},
	},
	"ConfirmUpload": {
		"a photo": func(svc *Service, f publishedFixture) error {
			_, err := svc.ConfirmUpload(context.Background(), alice, f.photoUpload, 1024, 768, 0)
			return err
		},
		"a video": func(svc *Service, f publishedFixture) error {
			_, err := svc.ConfirmUpload(context.Background(), alice, f.videoUpload, 1920, 1080, 3000)
			return err
		},
	},
	"DeleteImage": {
		"a confirmed photo": func(svc *Service, f publishedFixture) error {
			return svc.DeleteImage(context.Background(), alice, f.image)
		},
	},
	"DeleteVideo": {
		"a confirmed clip": func(svc *Service, f publishedFixture) error {
			return svc.DeleteVideo(context.Background(), alice, f.video)
		},
	},
}

// publishedLockExempt is every exported Service method the lock deliberately lets through, with
// why.
var publishedLockExempt = map[string]string{
	"SetTemplateDirectory": "wiring, not a post write",
	"Get":                  "read",
	"List":                 "read",
	"AttachedImages":       "read",
	"LearningSnapshot":     "read",
	"PublishedPosts":       "read",
	"PostStatus":           "read",
	"DeletePost":           "POST-74 lets a published post be deleted",
	"SavePublishedURL":     "POST-74: the address is the one thing a published post takes",
}

// POST-74, default-deny: an exported Service method nobody classified fails here, so a new write
// cannot reach a published post by being forgotten.
func TestEveryExportedServiceMethodIsClassifiedForThePublishedLock(t *testing.T) {
	methods := reflect.TypeOf(&Service{})
	for i := range methods.NumMethod() {
		name := methods.Method(i).Name
		cases, guarded := publishedLockGuarded[name]
		_, exempt := publishedLockExempt[name]
		switch {
		case !guarded && !exempt:
			t.Errorf("%s is an exported Service method the published lock does not classify: add it to publishedLockGuarded with a published-post case, or to publishedLockExempt with its reason (POST-74)", name)
		case guarded && exempt:
			t.Errorf("%s is both guarded and exempt", name)
		case guarded && len(cases) == 0:
			t.Errorf("%s is guarded but has no published-post case", name)
		}
	}
	for name := range publishedLockGuarded {
		if _, ok := methods.MethodByName(name); !ok {
			t.Errorf("publishedLockGuarded names %s, which is no longer an exported Service method", name)
		}
	}
	for name := range publishedLockExempt {
		if _, ok := methods.MethodByName(name); !ok {
			t.Errorf("publishedLockExempt names %s, which is no longer an exported Service method", name)
		}
	}
}

// POST-74: a published post refuses every write but its address's and its own deletion, and
// the refusal comes before anything changes — the row, its updated_at, its answers, its
// attachments, its pending uploads and the storage objects all stay as they were. The table is
// the one the default-deny test checks, so a guarded method has a case here.
func TestPublishedPostRefusesEveryWriteBeforeChangingAnything(t *testing.T) {
	for method, cases := range publishedLockGuarded {
		for name, operation := range cases {
			t.Run(method+" "+name, func(t *testing.T) {
				svc, store, blobs, fixture := newPublishedFixture(t)
				before := lockedSnapshot(store, blobs, fixture.slug)
				if err := operation(svc, fixture); !errors.Is(err, ErrPostPublished) {
					t.Fatalf("err = %v, want ErrPostPublished", err)
				}
				if after := lockedSnapshot(store, blobs, fixture.slug); !reflect.DeepEqual(after, before) {
					t.Fatalf("a refused write changed something:\nbefore %+v\nafter  %+v", before, after)
				}
			})
		}
	}
}

// POST-15: an identical save writes nothing, so it stays the no-op answer it is on every post
// rather than turning into a refusal on a published one.
func TestIdenticalContentSaveOnAPublishedPostIsStillANoOp(t *testing.T) {
	svc, store, blobs, fixture := newPublishedFixture(t)
	before := lockedSnapshot(store, blobs, fixture.slug)
	same := *before.post.Content
	got, err := svc.SaveContent(context.Background(), alice, fixture.slug, same, fixture.revision)
	if err != nil || got.Status != StatusPublished || got.ContentRevision != fixture.revision {
		t.Fatalf("identical save = %s rev %d, %v", got.Status, got.ContentRevision, err)
	}
	if after := lockedSnapshot(store, blobs, fixture.slug); !reflect.DeepEqual(after, before) {
		t.Fatal("an identical save wrote something")
	}
}

// The two writes the lock leaves open: the address is still replaced or cleared, and the post
// can still be deleted, except while a job is active (POST-29).
func TestDeletePostStillRemovesAPublishedPost(t *testing.T) {
	svc, store, blobs, fixture := newPublishedFixture(t)
	ctx := context.Background()
	replaced, err := svc.SavePublishedURL(ctx, alice, fixture.slug, secondAddress)
	if err != nil || replaced.Status != StatusPublished || replaced.PublishedURL != secondAddress {
		t.Fatalf("replacing the address = %+v, %v", replaced, err)
	}

	svc.jobs = fakeActiveJobs{fixture.slug: {ID: "job-1", Status: "running"}}
	if err := svc.DeletePost(ctx, alice, fixture.slug); !errors.Is(err, ErrPostBusy) {
		t.Fatalf("delete while a job is active = %v, want ErrPostBusy", err)
	}
	if _, ok := store.posts[fixture.slug]; !ok {
		t.Fatal("a refused delete removed the post")
	}

	svc.jobs = neutralJobs{}
	image, video := store.images[fixture.image], store.videos[fixture.video]
	if err := svc.DeletePost(ctx, alice, fixture.slug); err != nil {
		t.Fatalf("delete a published post: %v", err)
	}
	if _, ok := store.posts[fixture.slug]; ok {
		t.Fatal("the published post survived its delete")
	}
	if blobs.has(image.Key) || blobs.has(video.Key) {
		t.Fatal("the delete left the post's objects behind")
	}
}

// A write that passed the service's check and then lost the race to a publish is refused by
// the store's own guard, and the answer names the lock rather than a missing post.
func TestAWriteThatLosesTheRaceToAPublishIsRefusedAsLocked(t *testing.T) {
	publish := func(p Post) Post {
		p.Status = StatusPublished
		p.PublishedURL = firstAddress
		stamp := testNow
		p.PublishedAt = &stamp
		return p
	}
	race := func(store *fakeStore) {
		store.beforeGuardedWrite = func(slug string) {
			store.mu.Lock()
			defer store.mu.Unlock()
			store.beforeGuardedWrite = nil
			store.posts[slug] = publish(store.posts[slug])
		}
	}

	t.Run("a row update", func(t *testing.T) {
		svc, store, _ := newTestService(t)
		finalized := finalizedPost(t, svc, alice)
		before := store.posts[finalized.Slug]
		race(store)
		if _, err := svc.SaveDraft(context.Background(), alice, DraftSave{Slug: finalized.Slug, Title: "새 제목", Memo: "새 메모"}); !errors.Is(err, ErrPostPublished) {
			t.Fatalf("err = %v, want ErrPostPublished", err)
		}
		if got := store.posts[finalized.Slug]; !reflect.DeepEqual(got, publish(before)) {
			t.Fatalf("the losing write changed the row: %+v", got)
		}
	})

	t.Run("a guarded insert", func(t *testing.T) {
		svc, store, _ := newTestService(t)
		finalized := finalizedPost(t, svc, alice)
		before := store.posts[finalized.Slug]
		race(store)
		if _, _, _, err := svc.CreateUpload(context.Background(), alice, finalized.Slug, "IMG_1.jpg", AttachmentPhoto); !errors.Is(err, ErrPostPublished) {
			t.Fatalf("err = %v, want ErrPostPublished", err)
		}
		if len(store.uploads) != 0 {
			t.Fatalf("the losing insert wrote %d upload rows", len(store.uploads))
		}
		if got := store.posts[finalized.Slug]; !reflect.DeepEqual(got, publish(before)) {
			t.Fatalf("the losing insert changed the row: %+v", got)
		}
	})
}
