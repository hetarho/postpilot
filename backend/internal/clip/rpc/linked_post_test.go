package rpc

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/clip"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
)

type rpcLinkedPosts struct{}

func (rpcLinkedPosts) Get(context.Context, string, string) (clip.LinkedPost, error) {
	return clip.LinkedPost{Slug: "story", Title: "post", CreatedAt: time.Unix(100, 0), UpdatedAt: time.Unix(200, 0), ContentReady: true}, nil
}

func TestClipLinkedPostProjectionAndPresence(t *testing.T) {
	p := clip.Project{Ratio: "vertical", Language: "ko", LinkedPost: &clip.LinkedPost{Slug: "story", Title: "post", UpdatedAt: time.Unix(200, 0)}}
	wire := projectProto(p)
	if wire.LinkedPost == nil || wire.LinkedPost.Slug != "story" || wire.LinkedPost.Title != "post" || wire.LinkedPost.UpdatedAt != time.Unix(200, 0).UTC().Format(time.RFC3339Nano) {
		t.Fatal("link metadata projection lost", wire.LinkedPost)
	}
	p.LinkedPost = nil
	if projectProto(p).LinkedPost != nil {
		t.Fatal("absent link became an empty message")
	}
	store := &rpcStore{}
	h := NewHandler(testProjects(store).WithLinkedPosts(rpcLinkedPosts{}))
	ctx := auth.WithUser(t.Context(), "alice")
	slug, empty := "story", ""
	for _, value := range []*string{nil, &slug, &empty} {
		if _, err := h.UpdateClipProject(ctx, connect.NewRequest(&v1.UpdateClipProjectRequest{Id: "owned", LinkedPostSlug: value})); err != nil {
			t.Fatal(err)
		}
		if value == nil && store.patch.LinkedPostSlug != nil || value != nil && (store.patch.LinkedPostSlug == nil || *store.patch.LinkedPostSlug != *value) {
			t.Fatal("optional link presence was lost")
		}
	}
}
