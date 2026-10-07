package job

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestProtectedDetailsArePrivateOnPublicReadsAndLogsWithoutChangingInternalEvidence(t *testing.T) {
	store := &terminalStore{persisted: Job{ID: "opaque", UserID: "owner", Kind: "restricted", Status: StatusFailed, ObserveModel: "private/observer", WriteModel: "private/writer", Failure: &Failure{Reason: "MODEL_OUTPUT_INVALID", Params: map[string]string{"model": "private/writer"}, TechnicalDetail: "private provider body"}}}
	queue := New(store, time.Second, nil)
	queue.ProtectDetails("restricted")
	queue.ProtectDetails("restricted")
	public, err := queue.Get(context.Background(), "opaque", "owner")
	if err != nil || public.ObserveModel != "" || public.WriteModel != "" || public.Failure.TechnicalDetail != "" || len(public.Failure.Params) != 0 || public.Failure.Reason != "MODEL_OUTPUT_INVALID" {
		t.Fatalf("public job reveals private evidence: %v, %#v", err, public)
	}
	internal, err := queue.Result(context.Background(), "owner", "opaque")
	if err != nil || internal.Failure.TechnicalDetail != "private provider body" || internal.WriteModel != "private/writer" || store.persisted.Failure.Params["model"] != "private/writer" {
		t.Fatal("public projection changed private evidence", err)
	}
	if !queue.redacted("restricted") || queue.redacted("ordinary") {
		t.Fatal("log policy does not follow the registered work owner")
	}
	if _, err := queue.Get(context.Background(), "opaque", "foreign"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("foreign private work distinguishable: %v", err)
	}
	if _, err := queue.Result(context.Background(), "foreign", "opaque"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("foreign result distinguishable: %v", err)
	}
	store.readErr = ErrNotFound
	if _, err := queue.Get(context.Background(), "unknown", "foreign"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown private work differs: %v", err)
	}
}

func TestUnprotectedJobDetailsRetainTheirExistingPublicContract(t *testing.T) {
	store := &terminalStore{persisted: Job{ID: "ordinary", UserID: "owner", Kind: "ordinary", WriteModel: "public/writer", Failure: &Failure{Reason: "UNKNOWN_FAILURE", TechnicalDetail: "readable diagnostic"}}}
	queue := New(store, time.Second, nil)
	queue.ProtectDetails("restricted")
	public, err := queue.Get(context.Background(), "ordinary", "owner")
	if err != nil || public.WriteModel != "public/writer" || public.Failure.TechnicalDetail != "readable diagnostic" {
		t.Fatalf("unrelated work changed: %v", err)
	}
	if _, err := queue.Get(context.Background(), "ordinary", "foreign"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("unrelated ownership contract changed: %v", err)
	}
}
