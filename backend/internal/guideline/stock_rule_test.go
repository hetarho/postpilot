package guideline

import (
	"context"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestStockApplicabilityIsDeclaredByOutputAndNeverClassifiedFromItsText(t *testing.T) {
	expectedPlan := []string{"facts", "impressions", "memory_impressions", "order"}
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
	if _, err = svc.SetDefaultEnabled(ctx, "alice", KindPost, "facts", false); err != nil {
		t.Fatal(err)
	}
	second, err := svc.ForPrompt(ctx, "alice", KindPost, nil, nil, LanguageEnglish, false)
	if err != nil {
		t.Fatal(err)
	}
	if first.Stock[0].Text != oldText || second.Stock[0].Key != "impressions" || second.Stock[0].SourceOrder != 1 {
		t.Fatal("switch changed frozen stock or compressed registry order")
	}
	for _, rule := range second.Stock {
		if slices.Contains([]string{"facts", "memory_impressions", "ending_run", "natural_korean"}, rule.Key) {
			t.Fatalf("inapplicable rule=%s", rule.Key)
		}
	}
	if !reflect.DeepEqual(second.Owner, store.texts) {
		t.Fatal("owner lines changed with output language")
	}
	for _, entry := range Defaults(KindPost) {
		if _, err = svc.SetDefaultEnabled(ctx, "alice", KindPost, entry.Key, false); err != nil {
			t.Fatal(err)
		}
	}
	off, err := svc.ForPrompt(ctx, "alice", KindPost, nil, nil, LanguageKorean, true)
	if err != nil || off.Stock == nil || len(off.Stock) != 0 || len(off.Defaults) != 0 || !reflect.DeepEqual(off.Owner, store.texts) {
		t.Fatalf("all off authority=%+v %v", off, err)
	}
}
func TestUpdatedImpressionAndChronologyRecommendationsDoNotRelabelGeneratedMeaning(t *testing.T) {
	impressions, _ := DefaultFor(KindPost, "impressions")
	memory, _ := DefaultFor(KindPost, "memory_impressions")
	order, _ := DefaultFor(KindPost, "order")
	photos, _ := DefaultFor(KindPost, "photo_moments")
	if impressions.Ko.Name != "내 감상을 지키고 AI 제안 구분" || order.Ko.Name != "알려준 사건 순서대로" {
		t.Fatal("displayed recommendation policy is outdated")
	}
	for _, needle := range []string{"AI가 더한 의미", "직접 준", "사진만으로 맛"} {
		if !strings.Contains(impressions.Ko.Text, needle) {
			t.Errorf("impression missing %q", needle)
		}
	}
	if strings.Contains(memory.Ko.Text, "글쓴이가 직접 준 감상으로 봅니다") || !strings.Contains(memory.Ko.Text, "AI가 더한 의미") || !strings.Contains(memory.En.Text, "not an impression the author directly supplied for this visit") {
		t.Fatal("preference-derived addition masquerades as owner input")
	}
	for _, needle := range []string{"명시적으로", "사진 업로드 순서", "저장된 배열 순서", "파일명", "촬영 순서"} {
		if !strings.Contains(order.Ko.Text, needle) {
			t.Errorf("chronology missing %q", needle)
		}
	}
	if !strings.Contains(photos.Ko.Text, "없던 행동·이동·시간 순서") || !strings.Contains(photos.En.Text, "without inventing actions") {
		t.Fatal("photo paragraph transitions invent events")
	}
}
