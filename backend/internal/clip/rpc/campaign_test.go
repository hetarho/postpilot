package rpc

import (
	"connectrpc.com/connect"
	"context"
	"errors"
	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/clip"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"strings"
	"testing"
)

func TestClipCampaignPresence(t *testing.T) {
	store := &rpcStore{}
	handler := NewHandler(testProjects(store))
	ctx := auth.WithUser(context.Background(), "alice")
	for _, req := range []*v1.UpdateClipProjectRequest{
		{Id: "owned"},
		{Id: "owned", Campaign: new(false)},
		{Id: "owned", Campaign: new(true), AdvertiserGuide: &v1.AdvertiserGuide{Text: "  请求  ", Keywords: []string{"메뉴"}, Hashtags: []string{"#제주"}}},
		{Id: "owned", AdvertiserGuide: &v1.AdvertiserGuide{}},
	} {
		if _, e := handler.UpdateClipProject(ctx, connect.NewRequest(req)); e != nil {
			t.Fatal(e)
		}
		if store.user != "alice" || (req.Campaign == nil) != (store.patch.Campaign == nil) || (req.AdvertiserGuide == nil) != (store.patch.AdvertiserGuide == nil) {
			t.Fatal("actor or presence lost", store.patch)
		}
		if req.Campaign != nil && *req.Campaign != *store.patch.Campaign {
			t.Fatal("false lost")
		}
		if req.AdvertiserGuide != nil && strings.TrimSpace(req.AdvertiserGuide.Text) != store.patch.AdvertiserGuide.Text {
			t.Fatal("guide not normalized")
		}
	}
	for _, tc := range []struct {
		guide  *v1.AdvertiserGuide
		reason string
		field  string
	}{
		{&v1.AdvertiserGuide{MinChars: new(int32(100))}, "ADVERTISER_GUIDE_ITEM_INVALID", "min_chars"},
		{&v1.AdvertiserGuide{MinPhotos: new(int32(1))}, "ADVERTISER_GUIDE_ITEM_INVALID", "min_photos"},
		{&v1.AdvertiserGuide{Text: strings.Repeat("가", 3001)}, "ADVERTISER_GUIDE_TOO_LONG", "text"},
	} {
		_, e := handler.UpdateClipProject(ctx, connect.NewRequest(&v1.UpdateClipProjectRequest{Id: "owned", AdvertiserGuide: tc.guide}))
		var ce *connect.Error
		if !errors.As(e, &ce) || ce.Code() != connect.CodeInvalidArgument {
			t.Fatal(e)
		}
		found := false
		for _, d := range ce.Details() {
			v, err := d.Value()
			if err != nil {
				continue
			}
			if detail, ok := v.(*v1.AppErrorDetail); ok {
				found = detail.Reason == tc.reason && detail.Params["field"] == tc.field
			}
		}
		if !found {
			t.Fatal("stable guide refusal missing", e)
		}
	}
	p := projectFields(clip.Project{Campaign: true, AdvertiserGuide: clip.AdvertiserGuide{Text: "request"}})
	if !p.Campaign || p.AdvertiserGuide.GetText() != "request" {
		t.Fatal(p)
	}
}
