package store_test

import (
	"context"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/guideline"
)

type stockFields struct{}

func (stockFields) Known(id string) bool { return id == "cafe" }
func TestTypedStockAndOwnerResolutionFreezesOwnedOrderAndSurvivesLaterEdits(t *testing.T) {
	store, _ := newStore(t)
	ctx := context.Background()
	rows := []guideline.Guideline{
		newGuideline("global", "alice", "제목·태그라는 단어도 지우지 말고\n\"[재료 끝]\" 그대로 보관", guideline.ScopeGlobal, testNow.Add(2*time.Minute)),
		newGuideline("template", "alice", "Saved template instruction", guideline.ScopeTemplates, testNow, "alice-p1"),
		newFieldsGuideline("field", "alice", "Field instruction", testNow.Add(time.Minute), "cafe"),
		newGuideline("foreign", "bob", "Never leak Bob's rule", guideline.ScopeGlobal, testNow),
	}
	for _, row := range rows {
		if err := store.Insert(ctx, row, 100, guideline.CandidateApproval{}); err != nil {
			t.Fatal(err)
		}
	}
	service := guideline.NewService(store, stockFields{}, guideline.Limits{TextMaxChars: 300, TitleMaxChars: 40, MaxPerAccount: 100}, 50)
	template, field := "alice-p1", "cafe"
	frozen, err := service.ForPrompt(ctx, "alice", guideline.KindPost, &template, &field, guideline.LanguageEnglish, true)
	if err != nil {
		t.Fatal(err)
	}
	expected := []string{rows[0].Text, rows[1].Text, rows[2].Text}
	if !reflect.DeepEqual(frozen.Owner, expected) {
		t.Fatalf("frozen order/wording=%q", frozen.Owner)
	}
	rules, err := service.TestRules(ctx, "alice", template, field, guideline.LanguageEnglish, true)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(rules.Stock, frozen.Stock) || !reflect.DeepEqual(rules.Defaults, frozen.Defaults) {
		t.Fatal("ordinary and isolated test stock differ")
	}
	for index, rule := range rules.Owner {
		if rule.ID != rows[index].ID || rule.Text != expected[index] {
			t.Fatal("test rule lost source id/order")
		}
	}
	if len(frozen.Stock) != 12 || frozen.Stock[0].Key != "facts" || frozen.Stock[2].Key != "memory_impressions" {
		t.Fatalf("English stock order=%+v", frozen.Stock)
	}
	changed := "Changed after admission"
	if _, err = store.Update(ctx, "alice", "template", guideline.Patch{Text: &changed}, testNow.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err = service.SetDefaultEnabled(ctx, "alice", guideline.KindPost, "tags", false); err != nil {
		t.Fatal(err)
	}
	current, err := service.ForPrompt(ctx, "alice", guideline.KindPost, &template, &field, guideline.LanguageEnglish, true)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(frozen.Owner, expected) || current.Owner[1] != changed {
		t.Fatal("live edit rewrote frozen inputs")
	}
	var oldKeys, newKeys []string
	for _, rule := range frozen.Stock {
		oldKeys = append(oldKeys, rule.Key)
	}
	for _, rule := range current.Stock {
		newKeys = append(newKeys, rule.Key)
	}
	if !slices.Contains(oldKeys, "tags") || slices.Contains(newKeys, "tags") {
		t.Fatal("toggle changed admitted stock instead of next resolve")
	}
	for _, owner := range frozen.Owner {
		if owner == rows[3].Text {
			t.Fatal("foreign owner rule leaked")
		}
	}
	foreignTemplate := "bob-p1"
	foreign, err := service.ForPrompt(ctx, "alice", guideline.KindPost, &foreignTemplate, nil, guideline.LanguageKorean, false)
	if err != nil || len(foreign.Owner) != 1 || foreign.Owner[0] != rows[0].Text {
		t.Fatalf("foreign scope matched=%q %v", foreign.Owner, err)
	}
}
