package rpc

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/auth"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/post"
)

func TestSavePostDraftAndGeneratedHistoryExposeActualInputRevisions(t *testing.T) {
	h, svc := rpcServiceWith(t)
	ctx := auth.WithUser(context.Background(), "alice")
	created, err := h.SavePostDraft(ctx, draftRequest("", "Working title", nil))
	if err != nil {
		t.Fatal(err)
	}
	p := created.Msg.GetPost()
	if p.GetInputRevision() != 1 {
		t.Fatalf("new input revision = %d", p.GetInputRevision())
	}
	unchanged, err := h.SavePostDraft(ctx, draftRequest(p.GetSlug(), "Working title", nil))
	if err != nil || unchanged.Msg.GetPost().GetInputRevision() != 1 {
		t.Fatalf("no-op draft = %+v, %v", unchanged, err)
	}
	request := draftRequest(p.GetSlug(), "Working title", nil)
	request.Msg.Memo = "Newest memo"
	changed, err := h.SavePostDraft(ctx, request)
	if err != nil || changed.Msg.GetPost().GetInputRevision() != 2 {
		t.Fatalf("material draft = %+v, %v", changed, err)
	}
	if err := svc.SetGeneratedContent(ctx, "alice", p.GetSlug(), post.PostContent{Blocks: []post.Block{{Type: post.BlockText, Content: "Canonical body"}}}, post.LanguageKorean, nil); err != nil {
		t.Fatal(err)
	}
	page, err := h.ListPosts(ctx, connect.NewRequest(&postpilotv1.ListPostsRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	rows := page.Msg.GetPosts()
	if len(rows) != 1 || !rows[0].GetContentReady() || !rows[0].GetExportReady() || rows[0].GetInputRevision() != 2 || rows[0].GetContentRevision() != 1 {
		t.Fatalf("summary = %+v", rows)
	}
}
