package rpc

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	"github.com/postpilot/backend/internal/auth"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
)

// POST-117–119: the switch, the guide and the ticks are presence-aware on the wire — a new post
// reads campaign on, an absent part keeps what the post holds — and a refused guide reaches the
// client with its stable reason and the limit it hit.
func TestSavePostDraftCampaignPresence(t *testing.T) {
	h := rpcService(t)
	ctx := auth.WithUser(context.Background(), "alice")

	created, err := h.SavePostDraft(ctx, draftRequest("", "체험단 방문", nil))
	if err != nil || !created.Msg.GetPost().GetCampaign() {
		t.Fatalf("create = %v, %v", created, err)
	}
	slug := created.Msg.GetPost().GetSlug()
	withGuide := draftRequest(slug, "체험단 방문", nil)
	withGuide.Msg.AdvertiserGuide = &postpilotv1.AdvertiserGuide{Text: "주차 정보 넣기", Keywords: []string{"답십리맛집"}, MinChars: proto.Int32(1500), Hashtags: []string{"#협찬"}}
	withGuide.Msg.ChecklistTicks = &postpilotv1.ChecklistTicks{Lines: []string{"주차 정보 넣기"}}
	saved, err := h.SavePostDraft(ctx, withGuide)
	if err != nil {
		t.Fatal(err)
	}
	guide := saved.Msg.GetPost().GetAdvertiserGuide()
	if guide.GetText() != "주차 정보 넣기" || guide.GetMinChars() != 1500 || guide.MinPhotos != nil || guide.GetHashtags()[0] != "협찬" || saved.Msg.GetPost().GetChecklistTicks()[0] != "주차 정보 넣기" {
		t.Fatalf("saved = %v", saved.Msg.GetPost())
	}
	off := draftRequest(slug, "체험단 방문", nil)
	off.Msg.Campaign = proto.Bool(false)
	switched, err := h.SavePostDraft(ctx, off)
	if err != nil || switched.Msg.GetPost().GetCampaign() || switched.Msg.GetPost().GetAdvertiserGuide().GetText() != "주차 정보 넣기" {
		t.Fatalf("switch off = %v, %v", switched, err)
	}
	kept, err := h.SavePostDraft(ctx, draftRequest(slug, "체험단 방문 2", nil))
	if err != nil || kept.Msg.GetPost().GetCampaign() || len(kept.Msg.GetPost().GetChecklistTicks()) != 1 {
		t.Fatalf("absent = %v, %v", kept, err)
	}

	tooLong := draftRequest(slug, "바뀐 제목", nil)
	tooLong.Msg.AdvertiserGuide = &postpilotv1.AdvertiserGuide{Text: strings.Repeat("가", 3001)}
	_, err = h.SavePostDraft(ctx, tooLong)
	detail := postAppErrorDetail(t, err)
	if connect.CodeOf(err) != connect.CodeInvalidArgument || detail.GetReason() != "ADVERTISER_GUIDE_TOO_LONG" || detail.GetParams()["max"] != "3000" || detail.GetParams()["field"] != "text" {
		t.Fatalf("an over-long guide = %v", err)
	}
	invalid := draftRequest(slug, "바뀐 제목", nil)
	invalid.Msg.AdvertiserGuide = &postpilotv1.AdvertiserGuide{MinPhotos: proto.Int32(0)}
	_, err = h.SavePostDraft(ctx, invalid)
	if connect.CodeOf(err) != connect.CodeInvalidArgument || postAppErrorDetail(t, err).GetReason() != "ADVERTISER_GUIDE_ITEM_INVALID" || postAppErrorDetail(t, err).GetParams()["field"] != "min_photos" {
		t.Fatalf("an invalid item = %v", err)
	}
	got, err := h.GetPost(ctx, connect.NewRequest(&postpilotv1.GetPostRequest{Slug: slug}))
	if err != nil || got.Msg.GetPost().GetTitle() != "체험단 방문 2" {
		t.Fatalf("a refused guide applied its title: %q, %v", got.Msg.GetPost().GetTitle(), err)
	}
}
