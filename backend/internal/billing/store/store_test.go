package store

import (
	"reflect"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/billing/store/sqlc"
)

func TestGeneratedBillingLedgerAPIHasNoUpdateOrDelete(t *testing.T) {
	queries := reflect.TypeOf((*sqlc.Queries)(nil))
	for index := 0; index < queries.NumMethod(); index++ {
		name := queries.Method(index).Name
		if strings.Contains(name, "BillingEvent") && (strings.HasPrefix(name, "Update") || strings.HasPrefix(name, "Delete")) {
			t.Fatalf("append-only billing event API exposes %s", name)
		}
	}
}
