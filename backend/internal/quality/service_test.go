package quality

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

var serviceNow = time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)

// fakeMeasurements records every read and write, so "no write" and "reads nothing" are counts.
type fakeMeasurements struct {
	rows          map[string]StoredMeasurement
	reads, writes int
}

func (f *fakeMeasurements) Measurement(_ context.Context, userID, slug string) (StoredMeasurement, bool, error) {
	f.reads++
	row, ok := f.rows[userID+"/"+slug]
	return row, ok, nil
}

func (f *fakeMeasurements) SaveMeasurement(_ context.Context, m StoredMeasurement) error {
	f.writes++
	f.rows[m.UserID+"/"+m.PostSlug] = m
	return nil
}

type fakePhrases struct {
	lists map[string]PhraseList
	reads int
}

func (f *fakePhrases) PhraseList(_ context.Context, field string) (PhraseList, bool, error) {
	f.reads++
	list, ok := f.lists[field]
	return list, ok, nil
}

func (f *fakePhrases) ReplacePhraseList(_ context.Context, list PhraseList) error {
	f.lists[list.Field] = list
	return nil
}

// fakePosts is the post context: every post belongs to its owner, and published is the whole
// published window newest first, across accounts.
type fakePosts struct {
	posts                     map[string]PostSnapshot
	owners                    map[string]string
	published                 []PublishedPost
	postReads, publishedReads int
	limits                    []int
}

func (f *fakePosts) Post(_ context.Context, userID, slug string) (PostSnapshot, error) {
	f.postReads++
	found, ok := f.posts[slug]
	if !ok || f.owners[slug] != userID {
		return PostSnapshot{}, ErrPostNotFound
	}
	return found, nil
}

func (f *fakePosts) Published(_ context.Context, userID string, limit int) ([]PublishedPost, error) {
	f.publishedReads++
	f.limits = append(f.limits, limit)
	var out []PublishedPost
	for _, p := range f.published {
		if f.owners[p.Slug] == userID && len(out) < limit {
			out = append(out, p)
		}
	}
	return out, nil
}

// add registers one of alice's posts; a published one joins the front of the window, so the
// last one added is the newest.
func (f *fakePosts) add(slug, title string, revision int64, published bool, nouns []string, paragraphs ...string) PostSnapshot {
	doc := Document{Title: title, Blocks: text(paragraphs...)}
	korean := LanguageKorean
	snapshot := PostSnapshot{Slug: slug, Revision: revision, Content: &doc, ContentLanguage: &korean, TargetLanguage: LanguageKorean, Nouns: nouns}
	f.posts[slug], f.owners[slug] = snapshot, "alice"
	if published {
		f.published = append([]PublishedPost{{
			Slug: slug, Revision: revision, Content: doc, ContentLanguage: &korean, Nouns: nouns,
			PublishedAt: serviceNow.Add(time.Duration(len(f.published)) * time.Minute),
		}}, f.published...)
	}
	return snapshot
}

func newQualityService(t *testing.T) (*Service, *fakeMeasurements, *fakePhrases, *fakePosts) {
	t.Helper()
	measurements := &fakeMeasurements{rows: map[string]StoredMeasurement{}}
	phrases := &fakePhrases{lists: map[string]PhraseList{}}
	posts := &fakePosts{posts: map[string]PostSnapshot{}, owners: map[string]string{}}
	svc := NewService(Deps{Measurements: measurements, Phrases: phrases, Posts: posts, Now: func() time.Time { return serviceNow }})
	return svc, measurements, phrases, posts
}

func sampleOf(p PostSnapshot) Sample {
	return Sample{Slug: p.Slug, Doc: *p.Content, Language: LanguageOf(p.ContentLanguage), Nouns: p.Nouns}
}

func TestAStaleRevisionIsRecomputedAndStored(t *testing.T) {
	svc, measurements, _, posts := newQualityService(t)
	post := posts.add("p", "을지로 감자탕 후기", 2, false, []string{"감자탕"}, "감자탕을 먹었다. 국물이 진했다.")
	measurements.rows["alice/p"] = StoredMeasurement{
		PostSlug: "p", UserID: "alice", Revision: 1, MeasureVersion: MeasureVersion,
		Self: Self{Composition: Composition{CharCount: 999}},
	}
	reading, err := svc.PostMeasurement(context.Background(), "alice", "p")
	if err != nil {
		t.Fatal(err)
	}
	want := MeasureSelf(sampleOf(post))
	stored := measurements.rows["alice/p"]
	if measurements.writes != 1 || stored.Revision != 2 || stored.MeasureVersion != MeasureVersion || !reflect.DeepEqual(stored.Self, want) {
		t.Fatalf("stored = %+v after %d writes, want revision 2 of %+v", stored, measurements.writes, want)
	}
	if !stored.ComputedAt.Equal(serviceNow) || reading.Revision != 2 {
		t.Fatalf("computed at %v, reading revision %d", stored.ComputedAt, reading.Revision)
	}
	if got := reading.Measurement.Composition.Composition; got == nil || got.CharCount != want.Composition.CharCount {
		t.Fatalf("the stale row was answered: %+v", got)
	}
}

func TestAMeasureVersionChangeIsRecomputed(t *testing.T) {
	svc, measurements, _, posts := newQualityService(t)
	posts.add("p", "후기", 3, false, nil, "오늘은 쉬었다.")
	measurements.rows["alice/p"] = StoredMeasurement{
		PostSlug: "p", UserID: "alice", Revision: 3, MeasureVersion: MeasureVersion - 1,
		Self: Self{Composition: Composition{CharCount: 999}},
	}
	if _, err := svc.PostMeasurement(context.Background(), "alice", "p"); err != nil {
		t.Fatal(err)
	}
	if stored := measurements.rows["alice/p"]; measurements.writes != 1 || stored.MeasureVersion != MeasureVersion || stored.Self.Composition.CharCount == 999 {
		t.Fatalf("an old version was kept: %+v after %d writes", stored, measurements.writes)
	}
}

func TestAFreshRowIsReadWithoutAWrite(t *testing.T) {
	svc, measurements, _, posts := newQualityService(t)
	posts.add("p", "후기", 3, false, nil, "오늘은 쉬었다.")
	// Distinctive values no measurement of this post would produce: the answer must come from
	// the row, not from measuring again.
	measurements.rows["alice/p"] = StoredMeasurement{
		PostSlug: "p", UserID: "alice", Revision: 3, MeasureVersion: MeasureVersion,
		Self: Self{Composition: Composition{CharCount: 999, DistinctBlockTypes: 4}},
	}
	reading, err := svc.PostMeasurement(context.Background(), "alice", "p")
	if err != nil {
		t.Fatal(err)
	}
	if measurements.writes != 0 || reading.Measurement.Composition.Composition.CharCount != 999 {
		t.Fatalf("a fresh row was recomputed: %d writes, %+v", measurements.writes, reading.Measurement.Composition)
	}
	if reading.Measurement.Composition.Verdict != VerdictWithinBand {
		t.Fatalf("the stored four types = %s", reading.Measurement.Composition.Verdict)
	}
}

func TestAPostWithoutContentAnswersEveryValueAbsent(t *testing.T) {
	svc, measurements, _, posts := newQualityService(t)
	posts.posts["draft"], posts.owners["draft"] = PostSnapshot{Slug: "draft", Revision: 0, TargetLanguage: LanguageKorean}, "alice"
	reading, err := svc.PostMeasurement(context.Background(), "alice", "draft")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(reading.Measurement, AbsentPost()) || reading.Revision != 0 {
		t.Fatalf("reading = %+v", reading)
	}
	if measurements.reads != 0 || measurements.writes != 0 || posts.publishedReads != 0 {
		t.Fatalf("a post with no content touched the store: reads=%d writes=%d published=%d", measurements.reads, measurements.writes, posts.publishedReads)
	}
}

func TestPostM2IsBelowMinimumWithFewerThanThreeOthers(t *testing.T) {
	svc, _, _, posts := newQualityService(t)
	for i, word := range []string{"첫째", "둘째"} {
		posts.add(fmt.Sprintf("p%d", i), "후기", 1, true, nil, word+" "+sharedRun+" 왔다")
	}
	// A published post is not its own other: three published posts leave it two.
	posts.add("self", "후기", 1, true, nil, "셋째 "+sharedRun+" 좋았다")
	reading, err := svc.PostMeasurement(context.Background(), "alice", "self")
	if err != nil {
		t.Fatal(err)
	}
	if got := reading.Measurement.CrossPost; got.Verdict != VerdictBelowMinimum || got.Others != 2 || got.Share != nil {
		t.Fatalf("M2 with two others = %+v", got)
	}
	if posts.limits[len(posts.limits)-1] != PostWindow+1 {
		t.Fatalf("the window read asked for %d posts, want %d", posts.limits[len(posts.limits)-1], PostWindow+1)
	}

	posts.add("p2", "후기", 1, true, nil, "넷째 "+sharedRun+" 싶다")
	reading, err = svc.PostMeasurement(context.Background(), "alice", "self")
	if err != nil {
		t.Fatal(err)
	}
	if got := reading.Measurement.CrossPost; got.Verdict != VerdictOverBand || got.Others != 3 || got.Share == nil {
		t.Fatalf("M2 with three others = %+v", got)
	}
}

func TestTheAggregateReadsTheNewestPublishedPosts(t *testing.T) {
	svc, measurements, _, posts := newQualityService(t)
	for i := 0; i < 25; i++ {
		posts.add(fmt.Sprintf("p%02d", i), fmt.Sprintf("%d번째 기록", i), 1, true, []string{"기록"}, fmt.Sprintf("오늘은 %d번째 날이다.", i))
	}
	if _, err := svc.AccountQuality(context.Background(), "alice", "p00"); err != nil {
		t.Fatal(err)
	}
	if posts.limits[0] != TitleWindow {
		t.Fatalf("the aggregate asked for %d posts, want %d", posts.limits[0], TitleWindow)
	}
	// The PostWindow newest posts are measured and stored; the five oldest are not.
	if measurements.writes != PostWindow {
		t.Fatalf("stored %d rows, want %d", measurements.writes, PostWindow)
	}
	for i := 0; i < 5; i++ {
		if _, ok := measurements.rows[fmt.Sprintf("alice/p%02d", i)]; ok {
			t.Fatalf("p%02d is outside the window and was measured", i)
		}
	}
	if _, ok := measurements.rows["alice/p24"]; !ok {
		t.Fatal("the newest post was not measured")
	}
	// A second read finds every row fresh.
	if _, err := svc.AccountQuality(context.Background(), "alice", "p00"); err != nil || measurements.writes != PostWindow {
		t.Fatalf("a second read wrote again: %d writes, %v", measurements.writes, err)
	}
}

func TestAccountStatesBelowMinimumAndAbsent(t *testing.T) {
	svc, _, _, posts := newQualityService(t)
	posts.add("a", "감자탕 맛집", 1, true, []string{"감자탕"}, "감자탕을 먹었다.")
	posts.add("b", "감자탕 후기", 1, true, []string{"감자탕"}, "감자탕을 또 먹었다.")
	reading, err := svc.AccountQuality(context.Background(), "alice", "a")
	if err != nil {
		t.Fatal(err)
	}
	a := reading.Account
	for _, m := range []Metric{MetricTitleSaturation, MetricCrossPostPhrases, MetricComposition} {
		if a.Verdict(m) != VerdictBelowMinimum {
			t.Errorf("%s over two posts = %s, want below minimum", m, a.Verdict(m))
		}
	}
	if a.PublishedCount != 2 || a.Verdict(MetricInPostRepetition) == VerdictBelowMinimum {
		t.Fatalf("count %d, M3 %s", a.PublishedCount, a.Verdict(MetricInPostRepetition))
	}

	// Ten posts with no nouns meet M1's minimum and measure nothing: absent.
	svc, _, _, posts = newQualityService(t)
	for i := 0; i < 10; i++ {
		posts.add(fmt.Sprintf("p%d", i), "감자탕 맛집", 1, true, nil, "오늘은 쉬었다.")
	}
	reading, err = svc.AccountQuality(context.Background(), "alice", "p0")
	if err != nil || reading.Account.Verdict(MetricTitleSaturation) != VerdictAbsent {
		t.Fatalf("M1 without nouns = %s, %v", reading.Account.Verdict(MetricTitleSaturation), err)
	}
}

// Ten titles holding the same noun put M1 over its band, and TEXT-only posts put M4 over its;
// each over-band metric carries its rule text in the target language of the post the brief
// belongs to, and a metric within its band carries none.
func saturatedAccount(posts *fakePosts) {
	for i := 0; i < 10; i++ {
		posts.add(fmt.Sprintf("p%d", i), fmt.Sprintf("감자탕 맛집 %d", i), 1, true, []string{"감자탕"}, fmt.Sprintf("오늘은 %d번째 날이다.", i))
	}
}

func TestAnOverBandMetricCarriesItsRuleTextInThePostsTargetLanguage(t *testing.T) {
	svc, _, _, posts := newQualityService(t)
	saturatedAccount(posts)
	english := posts.posts["p0"]
	english.TargetLanguage = LanguageEnglish
	posts.posts["english"], posts.owners["english"] = english, "alice"

	for slug, language := range map[string]Language{"p0": LanguageKorean, "english": LanguageEnglish} {
		reading, err := svc.AccountQuality(context.Background(), "alice", slug)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range Metrics() {
			text, carried := reading.RuleTexts[m]
			if over := reading.Account.Verdict(m) == VerdictOverBand; over != carried {
				t.Fatalf("%s (%s): over band %v, rule text carried %v", m, slug, over, carried)
			}
			if carried {
				want, _ := RuleText(m, language, reading.Account.Named(m))
				if text != want {
					t.Fatalf("%s (%s) = %q, want %q", m, slug, text, want)
				}
			}
		}
		if !strings.Contains(reading.RuleTexts[MetricTitleSaturation], "감자탕") || reading.RuleTexts[MetricComposition] == "" {
			t.Fatalf("%s texts = %v", slug, reading.RuleTexts)
		}
	}
}

func TestRulesForKeepsOnlyTicksOverBandNow(t *testing.T) {
	svc, _, _, posts := newQualityService(t)
	saturatedAccount(posts)
	ticked := []string{string(MetricComposition), string(MetricCrossPostPhrases), "score", string(MetricTitleSaturation), string(MetricComposition)}
	texts, err := svc.RulesFor(context.Background(), "alice", "p0", ticked, LanguageEnglish)
	if err != nil {
		t.Fatal(err)
	}
	m1, _ := RuleText(MetricTitleSaturation, LanguageEnglish, "감자탕")
	m4, _ := RuleText(MetricComposition, LanguageEnglish, "")
	// M2 is within its band, so its tick adds nothing; the order is M1-M4, not the ticks'.
	if want := []string{m1, m4}; !reflect.DeepEqual(texts, want) {
		t.Fatalf("texts = %q, want %q", texts, want)
	}
}

func TestRulesForWithNothingTickedReadsNothing(t *testing.T) {
	svc, measurements, _, posts := newQualityService(t)
	saturatedAccount(posts)
	for name, ticked := range map[string][]string{"nil": nil, "empty": {}, "only unknown": {"score", ""}} {
		texts, err := svc.RulesFor(context.Background(), "alice", "p0", ticked, LanguageKorean)
		if err != nil || texts != nil {
			t.Fatalf("%s: %q, %v", name, texts, err)
		}
	}
	if posts.postReads != 0 || posts.publishedReads != 0 || measurements.reads != 0 {
		t.Fatalf("nothing ticked still read: posts=%d published=%d measurements=%d", posts.postReads, posts.publishedReads, measurements.reads)
	}
}

func TestPhrasesForReturnsTheStoredRankOrderOrNothing(t *testing.T) {
	svc, _, phrases, _ := newQualityService(t)
	phrases.lists["restaurant"] = PhraseList{Field: "restaurant", Phrases: []string{"웨이팅 없는", "주차 가능", "혼밥"}}
	phrases.lists["cafe"] = PhraseList{Field: "cafe", Phrases: []string{}}

	got, err := svc.PhrasesFor(context.Background(), "restaurant")
	if err != nil || !reflect.DeepEqual(got, []string{"웨이팅 없는", "주차 가능", "혼밥"}) {
		t.Fatalf("phrases = %q, %v", got, err)
	}
	got[0] = "변경"
	if phrases.lists["restaurant"].Phrases[0] != "웨이팅 없는" {
		t.Fatal("the answer shares the stored slice")
	}
	for name, field := range map[string]string{"an empty row": "cafe", "no row": "pets"} {
		if got, err := svc.PhrasesFor(context.Background(), field); err != nil || len(got) != 0 {
			t.Fatalf("%s = %q, %v", name, got, err)
		}
	}
	reads := phrases.reads
	if got, err := svc.PhrasesFor(context.Background(), "  "); err != nil || got != nil || phrases.reads != reads {
		t.Fatalf("a blank field = %q, %v after %d reads", got, err, phrases.reads-reads)
	}
}

func TestAnUnknownOrForeignSlugIsPostNotFound(t *testing.T) {
	svc, _, _, posts := newQualityService(t)
	saturatedAccount(posts)
	posts.posts["bobs"], posts.owners["bobs"] = posts.posts["p0"], "bob"
	for _, slug := range []string{"", "  ", "nobody", "bobs"} {
		if _, err := svc.PostMeasurement(context.Background(), "alice", slug); !errors.Is(err, ErrPostNotFound) {
			t.Errorf("post measurement %q = %v", slug, err)
		}
		if _, err := svc.AccountQuality(context.Background(), "alice", slug); !errors.Is(err, ErrPostNotFound) {
			t.Errorf("account quality %q = %v", slug, err)
		}
		if _, err := svc.RulesFor(context.Background(), "alice", slug, []string{string(MetricTitleSaturation)}, LanguageKorean); !errors.Is(err, ErrPostNotFound) {
			t.Errorf("rules for %q = %v", slug, err)
		}
	}
}

func TestNewServicePanicsOnAMissingCollaborator(t *testing.T) {
	full := func() Deps {
		return Deps{
			Measurements: &fakeMeasurements{}, Phrases: &fakePhrases{}, Posts: &fakePosts{},
			Now: func() time.Time { return serviceNow },
		}
	}
	for name, strip := range map[string]func(*Deps){
		"measurements": func(d *Deps) { d.Measurements = nil },
		"phrases":      func(d *Deps) { d.Phrases = nil },
		"posts":        func(d *Deps) { d.Posts = nil },
		"now":          func(d *Deps) { d.Now = nil },
	} {
		t.Run(name, func(t *testing.T) {
			deps := full()
			strip(&deps)
			defer func() {
				if got := recover(); got != "quality: "+name+" collaborator is required" {
					t.Fatalf("panic = %v", got)
				}
			}()
			NewService(deps)
		})
	}
}
