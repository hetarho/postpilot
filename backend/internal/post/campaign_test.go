package post

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/campaign"
)

func boolPtr(v bool) *bool { return &v }

// POST-117: a new post starts as a 체험단 post — the insert sets the switch — while a create that
// says otherwise, or carries a guide, applies it to the post it just minted.
func TestNewPostsStartAsCampaignPosts(t *testing.T) {
	svc, store, _ := newTestService(t)
	ctx := context.Background()
	created := mustCreatePost(t, svc, alice, "체험단 방문")
	if !created.Campaign || !store.posts[created.Slug].Campaign {
		t.Fatalf("a new post read as off: %+v", created)
	}
	language := LanguageKorean
	off, err := svc.SaveDraft(ctx, alice, DraftSave{Title: "내돈내산", TargetLanguage: &language, Campaign: boolPtr(false),
		AdvertiserGuide: &campaign.Guide{Text: "  주차 정보 넣기  ", Keywords: []string{"답십리맛집"}}})
	if err != nil {
		t.Fatal(err)
	}
	if off.Campaign || off.AdvertiserGuide.Text != "주차 정보 넣기" || !reflect.DeepEqual(off.AdvertiserGuide.Keywords, []string{"답십리맛집"}) {
		t.Fatalf("the create's campaign parts did not apply: %+v", off)
	}
}

// POST-117–119: the switch, the guide and the ticks are option saves — status, content revision,
// baseline and finalization stay; the switch and the guide move the input revision, the ticks do
// not; an absent part keeps what the post holds; an off switch keeps the guide; a refused guide
// changes nothing.
func TestCampaignAndGuideAreOptionSaves(t *testing.T) {
	svc, store, _ := newTestService(t)
	ctx := context.Background()
	finalized := finalizedPost(t, svc, alice)
	before := store.posts[finalized.Slug]
	minChars := 1500
	guide := campaign.Guide{Text: "필수 키워드: 답십리맛집\n주차 정보 넣기", Keywords: []string{"답십리맛집"}, MinChars: &minChars, Hashtags: []string{"#협찬"}}
	saved, err := svc.SaveDraft(ctx, alice, DraftSave{Slug: finalized.Slug, Title: before.Title, Memo: before.Memo, AdvertiserGuide: &guide})
	if err != nil {
		t.Fatal(err)
	}
	after := store.posts[finalized.Slug]
	if after.Status != StatusFinalized || after.ContentRevision != before.ContentRevision || after.MachineBaselineRevision != before.MachineBaselineRevision ||
		after.FinalizedRevision != before.FinalizedRevision || after.InputRevision != before.InputRevision+1 {
		t.Fatalf("a guide save moved the lifecycle: before %+v\nafter %+v", before, after)
	}
	if !reflect.DeepEqual(saved.AdvertiserGuide.Hashtags, []string{"협찬"}) || *saved.AdvertiserGuide.MinChars != 1500 {
		t.Fatalf("guide = %+v", saved.AdvertiserGuide)
	}
	ticks := []string{"주차 정보 넣기"}
	if _, err := svc.SaveDraft(ctx, alice, DraftSave{Slug: finalized.Slug, Title: before.Title, Memo: before.Memo, ChecklistTicks: &ticks}); err != nil {
		t.Fatal(err)
	}
	if got := store.posts[finalized.Slug]; got.InputRevision != after.InputRevision || !reflect.DeepEqual(got.ChecklistTicks, ticks) {
		t.Fatalf("ticks = %+v", got)
	}
	if _, err := svc.SaveDraft(ctx, alice, DraftSave{Slug: finalized.Slug, Title: before.Title, Memo: before.Memo, Campaign: boolPtr(false)}); err != nil {
		t.Fatal(err)
	}
	got := store.posts[finalized.Slug]
	if got.Campaign || got.AdvertiserGuide.Text != guide.Text || got.Status != StatusFinalized {
		t.Fatalf("switching off lost the guide or moved the lifecycle: %+v", got)
	}
	// An absent part keeps what the post holds.
	if _, err := svc.SaveDraft(ctx, alice, DraftSave{Slug: finalized.Slug, Title: before.Title, Memo: before.Memo}); err != nil {
		t.Fatal(err)
	}
	if kept := store.posts[finalized.Slug]; kept.AdvertiserGuide.Text != guide.Text || !reflect.DeepEqual(kept.ChecklistTicks, ticks) {
		t.Fatalf("an absent part changed the post: %+v", kept)
	}
	snapshot := store.posts[finalized.Slug]
	var tooLong *campaign.TooLongError
	if _, err := svc.SaveDraft(ctx, alice, DraftSave{Slug: finalized.Slug, Title: "바뀐 제목", Memo: before.Memo, AdvertiserGuide: &campaign.Guide{Text: strings.Repeat("가", campaign.GuideMaxChars+1)}}); !errors.As(err, &tooLong) {
		t.Fatalf("an over-long guide = %v", err)
	}
	if !reflect.DeepEqual(store.posts[finalized.Slug], snapshot) {
		t.Fatal("a refused guide changed the post")
	}
	if _, err := svc.SaveDraft(ctx, alice, DraftSave{Slug: finalized.Slug, Title: before.Title, Memo: before.Memo, AdvertiserGuide: &campaign.Guide{Keywords: []string{" "}}}); !errors.Is(err, campaign.ErrItemInvalid) {
		t.Fatalf("a blank keyword = %v", err)
	}
}

// POST-74: a published post refuses every campaign part, and nothing changes.
func TestAPublishedPostRefusesCampaignChanges(t *testing.T) {
	svc, store, _ := newTestService(t)
	ctx := context.Background()
	finalized := finalizedPost(t, svc, alice)
	if _, err := svc.SavePublishedURL(ctx, alice, finalized.Slug, firstAddress); err != nil {
		t.Fatal(err)
	}
	before := store.posts[finalized.Slug]
	ticks := []string{"주차 정보 넣기"}
	for name, save := range map[string]DraftSave{
		"switch": {Campaign: boolPtr(false)},
		"guide":  {AdvertiserGuide: &campaign.Guide{Text: "가이드"}},
		"ticks":  {ChecklistTicks: &ticks},
	} {
		save.Slug, save.Title = finalized.Slug, before.Title
		if _, err := svc.SaveDraft(ctx, alice, save); !errors.Is(err, ErrPostPublished) {
			t.Errorf("%s on a published post = %v, want ErrPostPublished", name, err)
		}
	}
	if after := store.posts[finalized.Slug]; !reflect.DeepEqual(after, before) {
		t.Fatalf("the lock let a campaign part through: %+v", after)
	}
}
