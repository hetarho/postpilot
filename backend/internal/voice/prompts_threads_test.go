package voice

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// VOICE-78: six kinds, each with a thread of at least 2 opening, 5 description and 2 closing
// photo-free questions keyed thread_<kind>_<part>_<n>, each naming the experience through one
// token that is never followed by a particle.
func TestEveryKindHasAThread(t *testing.T) {
	kinds := ExperienceKinds()
	want := []ExperienceKindInfo{
		{ExperienceFood, "맛집", "그곳"}, {ExperienceCafe, "카페", "그곳"}, {ExperienceBeauty, "뷰티", "그곳"},
		{ExperienceStay, "숙소", "그곳"}, {ExperienceLeisure, "체험·여가", "그곳"}, {ExperienceProduct, "제품", "그 제품"},
	}
	if fmt.Sprint(kinds) != fmt.Sprint(want) {
		t.Fatalf("kinds = %v", kinds)
	}
	key := regexp.MustCompile(`^thread_([a-z]+)_(opening|description|closing)_([1-9][0-9]*)$`)
	counts := map[ExperienceKind]map[PromptPart]int{}
	for _, prompt := range Prompts() {
		match := key.FindStringSubmatch(prompt.Key)
		if prompt.Kind == "" {
			if match != nil || strings.Contains(prompt.Text, ExperienceToken) {
				t.Fatalf("%s: a situation looks like a thread question", prompt.Key)
			}
			continue
		}
		if match == nil || ExperienceKind(match[1]) != prompt.Kind || PromptPart(match[2]) != prompt.Part {
			t.Fatalf("%s: key does not name kind %s and part %s", prompt.Key, prompt.Kind, prompt.Part)
		}
		if counts[prompt.Kind] == nil {
			counts[prompt.Kind] = map[PromptPart]int{}
		}
		counts[prompt.Kind][prompt.Part]++
		if match[3] != fmt.Sprint(counts[prompt.Kind][prompt.Part]) {
			t.Fatalf("%s: out of order", prompt.Key)
		}
		if prompt.Photo || prompt.Starter {
			t.Fatalf("%s: a thread question is neither photo nor starter", prompt.Key)
		}
		if strings.Count(prompt.Text, ExperienceToken) != 1 {
			t.Fatalf("%s: %q holds the token %d times", prompt.Key, prompt.Text, strings.Count(prompt.Text, ExperienceToken))
		}
		after := prompt.Text[strings.Index(prompt.Text, ExperienceToken)+len(ExperienceToken):]
		if after != "" && !strings.ContainsAny(after[:1], ",?.") {
			t.Fatalf("%s: %q puts %q after the token", prompt.Key, prompt.Text, after[:1])
		}
	}
	for _, info := range kinds {
		got := counts[info.Kind]
		if got[PartOpening] < 2 || got[PartDescription] < 5 || got[PartClosing] < 2 {
			t.Fatalf("%s thread = %v", info.Kind, got)
		}
	}
}

// VOICE-60: every key that existed before the threads still resolves with its text, and the
// review-moment families come before the other everyday families.
func TestCatalogKeepsKeysAndListsReviewMomentsFirst(t *testing.T) {
	keys := []string{"starter_day_open", "starter_discovery_open", "starter_snack", "starter_rain", "starter_recommend", "starter_inconvenience", "starter_surprise", "starter_choice", "starter_day_close", "starter_friend_close"}
	for _, prompt := range legacyPrompts {
		keys = append(keys, prompt.Key)
	}
	families := []string{"food", "drink", "commute", "shopping", "gift", "home", "weekend", "weather", "plans", "friends", "surprises", "waiting", "choices", "comfort", "everyday_app", "stories", "photos", "small_tasks", "neighborhood", "feelings", "memories", "recommendations", "story_openings", "story_closings"}
	for _, family := range families {
		for n := 1; n <= 8; n++ {
			keys = append(keys, fmt.Sprintf("everyday_%s_%02d", family, n))
		}
	}
	for _, key := range keys {
		prompt, ok := PromptByKey(key)
		if !ok || prompt.Kind != "" || prompt.Text == "" {
			t.Fatalf("pre-existing %s resolves to %+v ok=%v", key, prompt, ok)
		}
	}
	review := map[string]bool{}
	for _, family := range reviewMomentFamilies {
		review["everyday_"+family] = true
	}
	lastReview, firstOther := -1, -1
	for i, prompt := range Prompts() {
		family := promptFamily(prompt.Key)
		if !strings.HasPrefix(family, "everyday_") {
			continue
		}
		if review[family] {
			lastReview = i
		} else if firstOther < 0 {
			firstOther = i
		}
	}
	if lastReview < 0 || firstOther < 0 || lastReview > firstOther {
		t.Fatalf("last review moment at %d, first other everyday scene at %d", lastReview, firstOther)
	}
	if first := Prompts()[InitialQuestionCount]; promptFamily(first.Key) != "everyday_food" {
		t.Fatalf("first situation after the starters = %s", first.Key)
	}
}
