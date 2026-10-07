package usage

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
)

func TestTokenPlanQuoteMatchesSixteenExactCallsHoldAndMutatesNoLotsOrAdmissions(t *testing.T) {
	svc, store := newTestService(t, seoulNoon)
	info := pricedModels[cheapRef]
	info.Stages = []string{"observe", "write"}
	svc.models = fakeModels{cheapRef: info}
	store.lots = []Lot{{ID: "bonus", UserID: "alice", Kind: LotBonus, Granted: 500, Remaining: 500}}
	before := append([]Lot(nil), store.lots...)
	calls := []PlannedCall{{Ref: cheapRef, Stage: "write", Count: 8, PromptTokens: 30000, CompletionTokens: 1024}, {Ref: cheapRef, Stage: "write", Count: 8, PromptTokens: 40000, CompletionTokens: 10000}}
	quote, err := svc.QuoteTokenPlan(t.Context(), "alice", plan.Free, "bounded_test", calls)
	if err != nil || quote.Free || quote.Credits <= 0 {
		t.Fatal(quote, err)
	}
	if !reflect.DeepEqual(before, store.lots) || len(store.admissions) != 0 || len(store.holdDebits) != 0 {
		t.Fatal("quote changed accounting")
	}
	if err := svc.Hold(t.Context(), holdStart("alice", plan.Free, "job", calls...)); err != nil {
		t.Fatal(err)
	}
	if store.admissions[0].HoldCredits != quote.Credits {
		t.Fatal("quote and real full hold differ", quote, store.admissions)
	}
}
func TestTokenPlanQuoteNeverTreatsMissingOrDisabledPricesAsFree(t *testing.T) {
	svc, _ := newTestService(t, seoulNoon)
	info := pricedModels[cheapRef]
	info.Stages = []string{"write"}
	svc.models = fakeModels{cheapRef: info}
	call := PlannedCall{Ref: cheapRef, Stage: "write", Count: 16, PromptTokens: 30000, CompletionTokens: 8192}
	for _, change := range []func(*llm.ModelInfo){func(i *llm.ModelInfo) { i.InputUSDPerMillion = "" }, func(i *llm.ModelInfo) { i.OutputUSDPerMillion = "unknown" }, func(i *llm.ModelInfo) { i.InputUSDPerMillion = "1" + strings.Repeat("0", 50) }, func(i *llm.ModelInfo) { i.Disabled = true }, func(i *llm.ModelInfo) { i.Stages = []string{"observe"} }} {
		v := info
		change(&v)
		svc.models = fakeModels{cheapRef: v}
		if _, err := svc.QuoteTokenPlan(t.Context(), "alice", plan.Free, "bounded_test", []PlannedCall{call}); !errors.Is(err, ErrPricingUnavailable) {
			t.Fatal("unknown quote accepted", v, err)
		}
	}
	svc.models = fakeModels{}
	if _, err := svc.QuoteTokenPlan(t.Context(), "alice", plan.Free, "bounded_test", []PlannedCall{call}); !errors.Is(err, ErrPricingUnavailable) {
		t.Fatal("missing model quoted free", err)
	}
	if q, err := svc.QuoteTokenPlan(t.Context(), "alice", plan.Free, "bounded_test", nil); err != nil || !q.Free || q.Credits != 0 {
		t.Fatal("proven empty replay plan", q, err)
	}
}
