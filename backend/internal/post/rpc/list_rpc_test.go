package rpc

import (
	"context"
	"fmt"
	"testing"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/auth"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
)

// POST-90 over the wire: the request's size, token, query and status reach the service, the
// answer carries the next token, and following it walks every post once.
func TestListPostsPagesOverTheWire(t *testing.T) {
	h := rpcService(t)
	ctx := auth.WithUser(context.Background(), "alice")
	for i := range 5 {
		if _, err := h.SavePostDraft(ctx, draftRequest("", fmt.Sprintf("제주 %d", i), nil)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := h.SavePostDraft(ctx, draftRequest("", "부산", nil)); err != nil {
		t.Fatal(err)
	}

	seen := map[string]bool{}
	request := &postpilotv1.ListPostsRequest{PageSize: 2, Query: "#제주"}
	for pages := 0; ; pages++ {
		if pages > 5 {
			t.Fatal("the walk never ended")
		}
		answer, err := h.ListPosts(ctx, connect.NewRequest(request))
		if err != nil {
			t.Fatal(err)
		}
		if len(answer.Msg.GetPosts()) > 2 {
			t.Fatalf("page of %d rows", len(answer.Msg.GetPosts()))
		}
		for _, summary := range answer.Msg.GetPosts() {
			if seen[summary.GetSlug()] {
				t.Fatalf("%s answered twice", summary.GetSlug())
			}
			seen[summary.GetSlug()] = true
		}
		if answer.Msg.GetNextPageToken() == "" {
			break
		}
		request.PageToken = answer.Msg.GetNextPageToken()
	}
	if len(seen) != 5 {
		t.Fatalf("walked %d posts matching 제주, want 5", len(seen))
	}

	// Every draft is a draft, so 검토 narrows the same answer to nothing.
	reviewed, err := h.ListPosts(ctx, connect.NewRequest(&postpilotv1.ListPostsRequest{Status: "review"}))
	if err != nil || len(reviewed.Msg.GetPosts()) != 0 {
		t.Fatalf("review = %d, %v", len(reviewed.Msg.GetPosts()), err)
	}
	// The request a client that predates paging sends is today's answer: all of it, no token.
	all, err := h.ListPosts(ctx, connect.NewRequest(&postpilotv1.ListPostsRequest{}))
	if err != nil || len(all.Msg.GetPosts()) != 6 || all.Msg.GetNextPageToken() != "" {
		t.Fatalf("unpaged = %d posts token %q, %v", len(all.Msg.GetPosts()), all.Msg.GetNextPageToken(), err)
	}
}

func TestListPostsRefusesABadRequest(t *testing.T) {
	h := rpcService(t)
	ctx := auth.WithUser(context.Background(), "alice")
	for name, request := range map[string]*postpilotv1.ListPostsRequest{
		"negative size":  {PageSize: -1},
		"unknown status": {Status: "archived"},
		"forged token":   {PageToken: "%%%"},
	} {
		_, err := h.ListPosts(ctx, connect.NewRequest(request))
		if connect.CodeOf(err) != connect.CodeInvalidArgument || postAppErrorDetail(t, err).GetReason() != "POST_LIST_REQUEST_INVALID" {
			t.Errorf("%s = %v", name, err)
		}
	}
}
