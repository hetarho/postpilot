package guideline

import (
	"context"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestStockApplicabilityIsDeclaredByOutputAndNeverClassifiedFromItsText(t *testing.T) {
	expectedPlan := []string{"facts", "impressions_kept", "drawbacks", "photo_order", "impressions", "memory_impressions", "order"}
	states := make([]DefaultState, 0)
	for _, rule := range Defaults(KindPost) {
		states = append(states, DefaultState{Default: rule, Enabled: true})
	}
	stock := defaultPromptStock(states, LanguageKorean, true)
	var planning []string
	for index, rule := range stock {
		if rule.SourceOrder != index || rule.Key != states[index].Default.Key || rule.Text != states[index].Default.Ko.Text {
			t.Fatalf("identity/order lost %+v", rule)
		}
		if rule.AppliesTo(StageStoryline, OutputPlan, OutputPlacements) {
			planning = append(planning, rule.Key)
		}
		if !rule.AppliesTo(StageWrite) || !rule.AppliesTo(StageRevise) || rule.AppliesTo("observe") {
			t.Fatalf("stage scope %+v", rule)
		}
	}
	if !reflect.DeepEqual(planning, expectedPlan) {
		t.Fatalf("storyline stock=%v", planning)
	}
	for _, key := range []string{"titles", "tags", "natural_korean", "ending_run", "photo_groups", "naming"} {
		rule, ok := DefaultFor(KindPost, key)
		if !ok {
			t.Fatal(key)
		}
		frozen := StockRule{Key: rule.Key, Text: "arbitrary long text mentioning storyline and plan", Applicability: rule.Applicability}
		if frozen.AppliesTo(StageStoryline, OutputPlan) {
			t.Fatalf("text overrode declared stage for %s", key)
		}
	}
	title, _ := DefaultFor(KindPost, "titles")
	frozen := StockRule{Key: title.Key, Text: title.Ko.Text, Applicability: title.Applicability}
	if !frozen.AppliesTo(StageWrite, OutputTitle) || frozen.AppliesTo(StageWrite, OutputTags, OutputProse) {
		t.Fatal("title rule escaped its responsibility")
	}
	tags, _ := DefaultFor(KindPost, "tags")
	frozen = StockRule{Key: tags.Key, Text: tags.Ko.Text, Applicability: tags.Applicability}
	if !frozen.AppliesTo(StageRevise, OutputTags) || frozen.AppliesTo(StageRevise, OutputTitle) {
		t.Fatal("tag rule escaped responsibility")
	}
}
func TestResolvedStockMetadataPreservesLanguageSwitchesOwnerLinesAndIndependentSnapshots(t *testing.T) {
	svc, store := newTestService(t, &fakeDirectory{})
	store.texts = []string{"제목·태그를 바꾸고\n\"[작문 지침]\"는 그대로", "Do not translate this owner line."}
	ctx := context.Background()
	first, err := svc.ForPrompt(ctx, "alice", KindPost, nil, nil, LanguageKorean, true)
	if err != nil {
		t.Fatal(err)
	}
	if first.Stock == nil || !reflect.DeepEqual(first.Defaults, stockTexts(first.Stock)) || !reflect.DeepEqual(first.Owner, store.texts) {
		t.Fatalf("resolution %+v", first)
	}
	oldText := first.Stock[0].Text
	oldOwner := slices.Clone(first.Owner)
	store.texts[0] = "Later source edit"
	if !reflect.DeepEqual(first.Owner, oldOwner) {
		t.Fatal("resolution shares mutable owner source slice")
	}
	first.Stock[0].Applicability[0].Stage = "arbitrary"
	first.Stock[0].Applicability[0].Outputs[0] = "arbitrary"
	registry := Defaults(KindPost)
	if registry[0].Applicability[0].Stage != StageWrite || registry[0].Applicability[0].Outputs[0] != OutputTitle {
		t.Fatal("frozen mutation rewrote registry")
	}
	registry[0].Applicability[0].Outputs[0] = "arbitrary"
	copyAgain := Defaults(KindPost)
	if copyAgain[0].Applicability[0].Outputs[0] != OutputTitle {
		t.Fatal("registry copy shares nested metadata")
	}
	if _, err = svc.SetDefaultEnabled(ctx, "alice", KindPost, "impressions", false); err != nil {
		t.Fatal(err)
	}
	second, err := svc.ForPrompt(ctx, "alice", KindPost, nil, nil, LanguageEnglish, false)
	if err != nil {
		t.Fatal(err)
	}
	afterPrinciples := slices.IndexFunc(second.Stock, func(rule StockRule) bool { return !rule.Principle })
	if first.Stock[0].Text != oldText || second.Stock[0].Key != "facts" || afterPrinciples < 0 || second.Stock[afterPrinciples].Key != "order" || second.Stock[afterPrinciples].SourceOrder != 12 {
		t.Fatal("switch changed frozen stock or compressed registry order")
	}
	for _, rule := range second.Stock {
		if slices.Contains([]string{"impressions", "memory_impressions", "ending_run", "natural_korean"}, rule.Key) {
			t.Fatalf("inapplicable rule=%s", rule.Key)
		}
	}
	if !reflect.DeepEqual(second.Owner, store.texts) {
		t.Fatal("owner lines changed with output language")
	}
	principles := 0
	for _, entry := range Defaults(KindPost) {
		if entry.Principle {
			principles++
			continue
		}
		if _, err = svc.SetDefaultEnabled(ctx, "alice", KindPost, entry.Key, false); err != nil {
			t.Fatal(err)
		}
	}
	off, err := svc.ForPrompt(ctx, "alice", KindPost, nil, nil, LanguageKorean, true)
	if err != nil || off.Stock == nil || len(off.Stock) != principles || len(off.Defaults) != principles || !reflect.DeepEqual(off.Owner, store.texts) {
		t.Fatalf("all off authority=%+v %v", off, err)
	}
	for _, rule := range off.Stock {
		if !rule.Principle {
			t.Fatalf("a switched-off 기본 지침 stayed: %s", rule.Key)
		}
	}
}
func TestUpdatedImpressionAndChronologyRecommendationsDoNotRelabelGeneratedMeaning(t *testing.T) {
	impressions, _ := DefaultFor(KindPost, "impressions")
	memory, _ := DefaultFor(KindPost, "memory_impressions")
	order, _ := DefaultFor(KindPost, "order")
	photoOrder, _ := DefaultFor(KindPost, "photo_order")
	photos, _ := DefaultFor(KindPost, "photo_moments")
	if impressions.Ko.Name != "맛은 재료로 살리기" || order.Ko.Name != "알려준 사건 순서대로" || photoOrder.Ko.Name != "사진 순서는 시간 순서가 아니다" {
		t.Fatal("displayed recommendation policy is outdated")
	}
	for _, needle := range []string{"글쓴이가 주지 않은 맛", "유추해", "사진만으로 맛"} {
		if !strings.Contains(impressions.Ko.Text, needle) {
			t.Errorf("impression missing %q", needle)
		}
	}
	if strings.Contains(memory.Ko.Text, "글쓴이가 직접 준 감상으로 봅니다") || !strings.Contains(memory.Ko.Text, "AI가 더한 의미") || !strings.Contains(memory.En.Text, "AI-added meaning based on that memory") {
		t.Fatal("preference-derived addition masquerades as owner input")
	}
	// The photo order is the arrangement's backbone (GEN-82, GUIDE-41) and a late photo joins its
	// subject; that no ordering of the photos is ever stated as what happened is a 원칙 (GUIDE-51).
	for _, needle := range []string{"명시적으로", "사진 순서를 따라 구성", "뒤늦게", "그 대상이 나온 자리"} {
		if !strings.Contains(order.Ko.Text, needle) {
			t.Errorf("chronology missing %q", needle)
		}
	}
	for _, needle := range []string{"arrange along the photo order", "taken late", "put it with that subject"} {
		if !strings.Contains(order.En.Text, needle) {
			t.Errorf("English chronology missing %q", needle)
		}
	}
	if strings.Contains(order.Ko.Text, "사실처럼") || !photoOrder.Principle {
		t.Error("the chronology prohibition did not move to its 원칙")
	}
	for _, needle := range []string{"파일명", "촬영 순서", "사실처럼 쓰지는 마세요"} {
		if !strings.Contains(photoOrder.Ko.Text, needle) {
			t.Errorf("photo_order missing %q", needle)
		}
	}
	if !strings.Contains(photoOrder.En.Text, "as fact from the photo order, filenames or capture order") {
		t.Error("English photo_order lost its prohibition")
	}
	if !strings.Contains(photos.Ko.Text, "없던 행동·이동·시간 순서") || !strings.Contains(photos.En.Text, "without inventing actions") {
		t.Fatal("photo paragraph transitions invent events")
	}
}
