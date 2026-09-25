package quality

import (
	"fmt"
	"strings"
	"testing"
)

func some(x float64) *float64 { return &x }

// titled is n published posts, newest first, each returning the noun 감자탕, the first withNoun
// of which carry it in the title.
func titled(n, withNoun int) []Sample {
	var out []Sample
	for i := 0; i < n; i++ {
		title := fmt.Sprintf("%d번째 기록", i)
		if i < withNoun {
			title = "을지로 감자탕 " + title
		}
		out = append(out, sample(fmt.Sprintf("p%02d", i), title, []string{"감자탕"}, "오늘은 쉬었다."))
	}
	return out
}

// sharing is three published posts holding sharedRun, each with its own words around it, so the
// run they share is exactly sharedRun.
func sharing() []Sample {
	return []Sample{
		sample("a", "후기", nil, "첫째 "+sharedRun+" 왔다"),
		sample("b", "후기", nil, "둘째 "+sharedRun+" 싶다"),
		sample("c", "후기", nil, "셋째 "+sharedRun+" 좋았다"),
	}
}

func TestAccountBelowMinimumIsNotAbsent(t *testing.T) {
	// Nine titles all holding the noun would read 100%, but M1's minimum is ten.
	nine := Aggregate(titled(9, 9), nil)
	if nine.PublishedCount != 9 {
		t.Fatalf("published count = %d", nine.PublishedCount)
	}
	if got := nine.TitleSaturation; got.Verdict != VerdictBelowMinimum || got.Value != nil || got.Noun != "" {
		t.Fatalf("M1 under its minimum = %+v, want below minimum with nothing measured or named", got)
	}
	ten := Aggregate(titled(10, 10), nil).TitleSaturation
	if ten.Verdict != VerdictOverBand || ten.Noun != "감자탕" {
		t.Fatalf("M1 at its minimum = %+v", ten)
	}
	near(t, "M1 at its minimum", ten.Value, 1)

	// Ten posts with no nouns meet the minimum and still measure nothing: absent, not below it.
	var bare []Sample
	for _, s := range titled(10, 10) {
		s.Nouns = nil
		bare = append(bare, s)
	}
	if got := Aggregate(bare, nil).TitleSaturation; got.Verdict != VerdictAbsent || got.Value != nil || got.Noun != "" {
		t.Fatalf("M1 without nouns = %+v, want absent", got)
	}

	// Two posts sharing a run are under M2's minimum of three: no median, no run named.
	if got := Aggregate(sharing()[:2], nil).CrossPost; got.Verdict != VerdictBelowMinimum || got.Median != nil || got.Run != "" {
		t.Fatalf("M2 under its minimum = %+v", got)
	}
	three := Aggregate(sharing(), nil).CrossPost
	if three.Verdict != VerdictOverBand || three.Median == nil || three.Run != sharedRun {
		t.Fatalf("M2 at its minimum = %+v, %v", three, deref(three.Median))
	}
	// Three posts too short to hold a run meet the minimum and have no share to take a median of.
	short := []Sample{sample("a", "후기", nil, "짧은 글"), sample("b", "후기", nil, "짧은 글"), sample("c", "후기", nil, "짧은 글")}
	if got := Aggregate(short, nil).CrossPost; got.Verdict != VerdictAbsent || got.Median != nil || got.Run != "" {
		t.Fatalf("M2 with no computable share = %+v, want absent", got)
	}
}

// words is n space-separated 어절 of its own, so a post padded with it shares no run by accident.
func words(prefix string, n int) string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("%s%d", prefix, i)
	}
	return strings.Join(out, " ")
}

// M2's run is read only by the over-band rule text, so a within-band account names none even
// where two posts share one (review F8).
func TestAWithinBandAccountNamesNoRun(t *testing.T) {
	// A: a and b share sharedRun inside a hundred words of their own, and c shares nothing, so the
	// median share is about 7% — within the band.
	within := Aggregate([]Sample{
		sample("a", "후기", nil, words("가", 100)+" "+sharedRun),
		sample("b", "후기", nil, words("나", 100)+" "+sharedRun),
		sample("c", "후기", nil, words("다", 100)),
	}, nil)
	if got := within.CrossPost; got.Verdict != VerdictWithinBand || got.Run != "" {
		t.Fatalf("within band = %+v, %v; want within band with no run", got, deref(got.Median))
	}
	if named := within.Named(MetricCrossPostPhrases); named != "" {
		t.Fatalf("a within-band account names %q", named)
	}

	// B: the same run shared widely enough to go over band is named.
	over := Aggregate(sharing(), nil).CrossPost
	if over.Verdict != VerdictOverBand || over.Run != sharedRun {
		t.Fatalf("over band = %+v, want the shared run named", over)
	}
}

func TestBandBoundariesFollowTheirWording(t *testing.T) {
	for _, c := range []struct {
		name      string
		got, want Verdict
	}{
		{"M1 exactly 30%", banded(some(0.30), titleSaturationOver), VerdictWithinBand},
		// (0.2+0.4)/2 computes to 0.30000000000000004, which is still exactly 30% in QUAL-11's words.
		{"M1 a median of 20% and 40%", banded(Median([]*float64{some(0.2), some(0.4)}), titleSaturationOver), VerdictWithinBand},
		{"M1 over 30%", banded(some(0.3001), titleSaturationOver), VerdictOverBand},
		{"M2 exactly 10%", banded(some(0.10), crossPostOver), VerdictWithinBand},
		{"M2 a median of 5% and 15%", banded(Median([]*float64{some(0.05), some(0.15)}), crossPostOver), VerdictWithinBand},
		{"M2 over 10%", banded(some(0.1001), crossPostOver), VerdictOverBand},
		{"M3 exactly 8% repetition", repetitionVerdict(some(0.08), nil), VerdictWithinBand},
		{"M3 a median of 6% and 10% repetition", repetitionVerdict(Median([]*float64{some(0.06), some(0.10)}), nil), VerdictWithinBand},
		{"M3 over 8% repetition", repetitionVerdict(some(0.0801), nil), VerdictOverBand},
		{"M3 exactly 50% relevance", repetitionVerdict(nil, some(0.50)), VerdictWithinBand},
		{"M3 under 50% relevance", repetitionVerdict(nil, some(0.4999)), VerdictOverBand},
		{"M4 exactly 2 types", banded(some(2), compositionOver), VerdictOverBand},
		{"M4 a median of 2 and 3 types", banded(Median([]*float64{some(2), some(3)}), compositionOver), VerdictWithinBand},
		{"M4 3 types", banded(some(3), compositionOver), VerdictWithinBand},
	} {
		if c.got != c.want {
			t.Errorf("%s: %s, want %s", c.name, c.got, c.want)
		}
	}

	// The account applies the same band: 3 of 10 titles is within it, 4 of 10 over.
	for withNoun, want := range map[int]Verdict{3: VerdictWithinBand, 4: VerdictOverBand} {
		if got := Aggregate(titled(10, withNoun), nil).TitleSaturation.Verdict; got != want {
			t.Errorf("%d of 10 titles: %s, want %s", withNoun, got, want)
		}
	}
}

func TestInPostRepetitionIsOverBandOnEitherHalf(t *testing.T) {
	for _, c := range []struct {
		name             string
		share, relevance *float64
		want             Verdict
	}{
		{"repetition over, relevance within", some(0.5), some(1), VerdictOverBand},
		{"repetition within, relevance under", some(0.05), some(0.2), VerdictOverBand},
		{"both crossing", some(0.5), some(0), VerdictOverBand},
		{"both within", some(0.05), some(1), VerdictWithinBand},
		{"repetition over, relevance absent", some(0.5), nil, VerdictOverBand},
		{"repetition absent, relevance under", nil, some(0.2), VerdictOverBand},
		{"repetition within, relevance absent", some(0.05), nil, VerdictWithinBand},
		{"repetition absent, relevance within", nil, some(1), VerdictWithinBand},
		{"both absent", nil, nil, VerdictAbsent},
	} {
		self := Self{Repetition: Repetition{Share: c.share, TitleRelevance: c.relevance}}
		if got := JudgePost(Sample{Slug: "p"}, self, nil).Repetition.Verdict; got != c.want {
			t.Errorf("post, %s: %s, want %s", c.name, got, c.want)
		}
		// One published post meets M3's minimum, and the medians are its own halves.
		if got := Aggregate([]Sample{{Slug: "p"}}, []Self{self}).Repetition.Verdict; got != c.want {
			t.Errorf("account, %s: %s, want %s", c.name, got, c.want)
		}
	}
}

func TestCompositionNeedsThreePublishedPosts(t *testing.T) {
	published := []Sample{
		{Slug: "a", Language: LanguageKorean, Doc: Document{Blocks: []Block{
			{Type: BlockText, Content: "감자탕을 먹었다."}, {Type: BlockImage, File: "a.jpg"},
		}}},
		{Slug: "b", Language: LanguageKorean, Doc: Document{Blocks: []Block{
			{Type: BlockText, Content: "국물이 진했다!"}, {Type: BlockHeading, Content: "메뉴"},
		}}},
		{Slug: "c", Language: LanguageKorean, Doc: Document{Blocks: []Block{
			{Type: BlockText, Content: "뼈가 많았다. 또 가고 싶다."},
			{Type: BlockImage, File: "c1.jpg"}, {Type: BlockImage, File: "c2.jpg"},
			{Type: BlockList, Items: []string{"볶음밥"}},
		}}},
	}
	two := Aggregate(published[:2], nil).Composition
	if two != (AccountComposition{Verdict: VerdictBelowMinimum}) {
		t.Fatalf("M4 over two posts = %+v, want below minimum with nothing measured", two)
	}

	three := Aggregate(published, nil).Composition
	// Per post: 9, 10 and 19 runes; 1, 0 and 2 photos; 2, 2 and 3 types; 9, 8 and 7.5 per sentence.
	near(t, "characters", three.CharCount, 10)
	near(t, "photos", three.PhotoCount, 1)
	near(t, "types", three.DistinctBlockTypes, 2)
	near(t, "sentence length", three.AvgSentenceLength, 8)
	if three.Verdict != VerdictOverBand {
		t.Fatalf("a median of two types = %s, want over band", three.Verdict)
	}

	// A stored self-measurement is read as it is, and a missing one is measured on the spot.
	stored := Aggregate(published, []Self{{Composition: Composition{DistinctBlockTypes: 5}}}).Composition
	near(t, "types with a stored first post", stored.DistinctBlockTypes, 3)
	if stored.Verdict != VerdictWithinBand {
		t.Fatalf("a median of three types = %s, want within band", stored.Verdict)
	}
}

func TestPostCrossPostIsBelowMinimumWithFewerThanThreeOthers(t *testing.T) {
	published := sharing()
	// A published post is not its own other, so three published posts leave each only two.
	for _, post := range published {
		got := JudgePost(post, MeasureSelf(post), OthersOf(published, post.Slug)).CrossPost
		if got.Verdict != VerdictBelowMinimum || got.Share != nil || got.Others != 2 {
			t.Errorf("%s: %+v, want below minimum naming 2 others", post.Slug, got)
		}
	}

	draft := sample("draft", "초안", []string{"감자탕"}, "넷째 "+sharedRun+" 갔다")
	for n := 0; n < Minimum(MetricCrossPostPhrases); n++ {
		got := JudgePost(draft, MeasureSelf(draft), published[:n]).CrossPost
		if got.Verdict != VerdictBelowMinimum || got.Share != nil || got.Others != n {
			t.Errorf("%d others: %+v", n, got)
		}
	}
	got := JudgePost(draft, MeasureSelf(draft), OthersOf(published, draft.Slug)).CrossPost
	if got.Verdict != VerdictOverBand || got.Share == nil || got.Others != 3 {
		t.Fatalf("the draft against three = %+v", got)
	}

	// M3 and M4 have no minimum: with no other post at all they are judged against the bands.
	alone := JudgePost(draft, MeasureSelf(draft), nil)
	if alone.Repetition.Verdict != VerdictOverBand {
		t.Fatalf("M3 alone = %+v", alone.Repetition)
	}
	if alone.Composition.Verdict != VerdictOverBand || alone.Composition.Composition == nil || alone.Composition.Composition.DistinctBlockTypes != 1 {
		t.Fatalf("M4 alone = %+v", alone.Composition)
	}
}

func TestAnAbsentValueNeitherPassesNorWarns(t *testing.T) {
	// A post with no content: every row is absent and carries no value.
	empty := AbsentPost()
	for name, verdict := range map[string]Verdict{
		"M2": empty.CrossPost.Verdict, "M3": empty.Repetition.Verdict, "M4": empty.Composition.Verdict,
	} {
		if verdict != VerdictAbsent {
			t.Errorf("%s of a post with no content = %s", name, verdict)
		}
	}
	if empty.CrossPost.Share != nil || empty.Repetition.Repetition != (Repetition{}) || empty.Composition.Composition != nil {
		t.Fatalf("a post with no content carries a value: %+v", empty)
	}

	three := []Sample{{Slug: "a"}, {Slug: "b"}, {Slug: "c"}}
	// An absent share does not pull the median down to a passing zero.
	pulled := Aggregate(three, []Self{{Repetition: Repetition{Share: some(0.5)}}, {}, {}}).Repetition
	near(t, "repetition median", pulled.Share, 0.5)
	if pulled.Verdict != VerdictOverBand || pulled.TitleRelevance != nil {
		t.Fatalf("absent shares passed: %+v", pulled)
	}
	// An absent relevance is not a warning zero either.
	quiet := Aggregate(three, []Self{
		{Repetition: Repetition{Share: some(0.05)}}, {Repetition: Repetition{Share: some(0.05)}}, {},
	}).Repetition
	if quiet.Verdict != VerdictWithinBand || quiet.TitleRelevance != nil {
		t.Fatalf("an absent relevance warned: %+v", quiet)
	}
	// A photo-only post has no sentence, and its absent length does not enter the median.
	lengths := Aggregate(three, []Self{
		{Composition: Composition{DistinctBlockTypes: 1}},
		{Composition: Composition{DistinctBlockTypes: 1}},
		{Composition: Composition{DistinctBlockTypes: 3, AvgSentenceLength: some(9)}},
	}).Composition
	near(t, "sentence length median", lengths.AvgSentenceLength, 9)

	// Ten titles and no noun: M1 is absent, not a within-band zero.
	var bare []Sample
	for _, s := range titled(10, 10) {
		s.Nouns = []string{}
		bare = append(bare, s)
	}
	account := Aggregate(bare, nil)
	if got := account.Verdict(MetricTitleSaturation); got != VerdictAbsent {
		t.Fatalf("M1 with no noun = %s", got)
	}
	if got := account.Verdict(MetricInPostRepetition); got != VerdictAbsent {
		t.Fatalf("M3 with no noun = %s", got)
	}
}

func TestNothingPublishedNamesNothing(t *testing.T) {
	account := Aggregate(nil, nil)
	if account.PublishedCount != 0 {
		t.Fatalf("published count = %d", account.PublishedCount)
	}
	for _, m := range Metrics() {
		if got := account.Verdict(m); got != VerdictBelowMinimum {
			t.Errorf("%s with nothing published = %s, want below minimum", m, got)
		}
		if named := account.Named(m); named != "" {
			t.Errorf("%s names %q with nothing published", m, named)
		}
	}
	for _, lang := range []Language{LanguageKorean, LanguageEnglish} {
		for _, m := range []Metric{MetricTitleSaturation, MetricCrossPostPhrases} {
			if text, ok := RuleText(m, lang, account.Named(m)); ok || text != "" {
				t.Errorf("%s (%s) rendered %q with nothing published", m, lang, text)
			}
		}
	}

	// A noun is named only while it stands in the account's own titles (QUAL-15): returned for
	// every post but in no title, it measures a real zero and names nothing.
	unnamed := Aggregate(titled(10, 0), nil)
	near(t, "M1 with the noun in no title", unnamed.TitleSaturation.Value, 0)
	if unnamed.Named(MetricTitleSaturation) != "" || unnamed.Verdict(MetricTitleSaturation) != VerdictWithinBand {
		t.Fatalf("M1 with the noun in no title = %+v", unnamed.TitleSaturation)
	}
}
