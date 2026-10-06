package rpc

import (
	"context"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/auth"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/provider"
	providerstore "github.com/postpilot/backend/internal/provider/store"
)

type initializedCatalog struct{ model llm.ModelInfo }

func (c initializedCatalog) Models() []llm.ModelInfo { return []llm.ModelInfo{c.model} }
func (c initializedCatalog) Lookup(ref llm.ModelRef) (llm.ModelInfo, bool) {
	return c.model, c.model.Ref == ref
}

type initializedCredits struct{}

func (initializedCredits) Tier(context.Context, string) (plan.Plan, error) { return plan.Free, nil }
func (initializedCredits) ForCalls([]provider.PlannedCall) int {
	panic("initializer must not price calls")
}
func (initializedCredits) Balance(context.Context, string) (int, bool, error) {
	panic("initializer must not spend credits")
}

func TestInitializeDefaultsUsesOnlyTheAuthenticatedOwner(t *testing.T) {
	request := connect.NewRequest(&postpilotv1.InitializeDefaultSelectionsRequest{})
	if request.Msg.ProtoReflect().Descriptor().Fields().Len() != 0 {
		t.Fatal("initializer request must not accept a user or model claim")
	}
	if _, err := NewHandler(nil).InitializeDefaultSelections(context.Background(), request); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("initializer without actor: %v", err)
	}
	handle, err := db.Open(filepath.Join(t.TempDir(), "owner.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = handle.Close() })
	ctx := context.Background()
	if err := db.Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	for _, userID := range []string{"alice", "bob"} {
		if _, err := handle.Writer.Exec(`INSERT INTO users(id,password_hash,created_at) VALUES(?, 'hash', '2026-10-07T00:00:00Z')`, userID); err != nil {
			t.Fatal(err)
		}
	}
	store := providerstore.New(handle.Writer, handle.Reader)
	model := llm.ModelInfo{Ref: llm.ModelRef{ProviderID: "p", ModelID: "ready"}, Vision: true,
		Stages: []string{"observe", "write", "analyze"}, Levels: map[string]string{"observe": "free", "write": "free", "analyze": "free"},
		InputUSDPerMillion: "0", OutputUSDPerMillion: "0"}
	handler := NewHandler(provider.NewService(store, initializedCatalog{model}, initializedCredits{}).WithModelGrades())
	acting := auth.WithActor(ctx, auth.Actor{UserID: "alice", Plan: plan.Free})
	response, err := handler.InitializeDefaultSelections(acting, request)
	if err != nil || len(response.Msg.GetSelections()) != 3 {
		t.Fatalf("initialized owner: %+v %v", response, err)
	}
	bob, err := store.ListSelectionSlots(ctx, "bob")
	if err != nil || len(bob) != 0 {
		t.Fatalf("other owner's selections changed: %+v %v", bob, err)
	}
}
