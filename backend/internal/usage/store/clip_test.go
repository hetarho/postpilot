package store_test

import (
	"context"
	"testing"

	"github.com/postpilot/backend/internal/plan"
)

func TestClipDuplicateReservationCannotDebitAgain(t *testing.T) {
	svc, _ := newServiceWithDB(t)
	ctx := context.Background()
	request := holdFor("clip-once")
	request.Kind = "generate_clip"
	if err := svc.Hold(ctx, request); err != nil {
		t.Fatal(err)
	}
	before, err := svc.BalanceFor(ctx, "alice", plan.Free)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Hold(ctx, request); err == nil {
		t.Fatal("duplicate hold must fail closed")
	}
	after, err := svc.BalanceFor(ctx, "alice", plan.Free)
	if err != nil || before.Credits != after.Credits {
		t.Fatal(before, after, err)
	}
}
