package rpc

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	"github.com/postpilot/backend/internal/auth"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/quality"
)

type memoryMeasurements map[string]quality.StoredMeasurement

func (m memoryMeasurements) Measurement(_ context.Context, userID, slug string) (quality.StoredMeasurement, bool, error) {
	row, ok := m[userID+"/"+slug]
	return row, ok, nil
}

func (m memoryMeasurements) SaveMeasurement(_ context.Context, row quality.StoredMeasurement) error {
	m[row.UserID+"/"+row.PostSlug] = row
	return nil
}

type noPhrases struct{}

func (noPhrases) PhraseList(context.Context, string) (quality.PhraseList, bool, error) {
	return quality.PhraseList{}, false, nil
}
func (noPhrases) ReplacePhraseList(context.Context, quality.PhraseList) error { return nil }

// alicesPosts is alice's posts by slug; published is the published window, newest first.
type alicesPosts struct {
	posts     map[string]quality.PostSnapshot
	published []quality.PublishedPost
}

func (p *alicesPosts) Post(_ context.Context, userID, slug string) (quality.PostSnapshot, error) {
	found, ok := p.posts[slug]
	if !ok || userID != "alice" {
		return quality.PostSnapshot{}, quality.ErrPostNotFound
	}
	return found, nil
}

func (p *alicesPosts) Published(_ context.Context, userID string, limit int) ([]quality.PublishedPost, error) {
	if userID != "alice" {
		return nil, nil
	}
	return p.published[:min(limit, len(p.published))], nil
}

// saturated is ten published posts whose titles all hold one noun, each TEXT-only with two
// nouns used once: M1, M3 (the top noun takes half the occurrences) and M4 are over their
// bands, and M2 is within its band, since no post shares an eight-word run with another.
func saturated() *alicesPosts {
	posts := &alicesPosts{posts: map[string]quality.PostSnapshot{}}
	korean := quality.LanguageKorean
	for i := 0; i < 10; i++ {
		slug := fmt.Sprintf("p%d", i)
		doc := quality.Document{Title: fmt.Sprintf("감자탕 맛집 %d", i), Blocks: []quality.Block{
			// Nine words, each window of eight holding this post's own number: a real zero share.
			{Type: quality.BlockText, Content: fmt.Sprintf("오늘은 %d번째 날이라 감자탕 대신 따뜻한 국수를 천천히 먹었다.", i)},
		}}
		posts.posts[slug] = quality.PostSnapshot{
			Slug: slug, Revision: 1, Content: &doc, ContentLanguage: &korean, TargetLanguage: quality.LanguageKorean, Nouns: []string{"감자탕", "국수"},
		}
		posts.published = append(posts.published, quality.PublishedPost{
			Slug: slug, Revision: 1, Content: doc, ContentLanguage: &korean, Nouns: []string{"감자탕", "국수"},
			PublishedAt: time.Date(2026, 9, 24, 12, i, 0, 0, time.UTC),
		})
	}
	// A post with no content yet.
	posts.posts["draft"] = quality.PostSnapshot{Slug: "draft", TargetLanguage: quality.LanguageKorean}
	return posts
}

func handler(posts *alicesPosts) *Handler {
	return NewHandler(quality.NewService(quality.Deps{
		Measurements: memoryMeasurements{}, Phrases: noPhrases{}, Posts: posts,
		Now: func() time.Time { return time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC) },
	}))
}

func signedIn() context.Context { return auth.WithUser(context.Background(), "alice") }

func reasonOf(t *testing.T, err error) string {
	t.Helper()
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		t.Fatalf("not a connect error: %v", err)
	}
	for _, d := range connectErr.Details() {
		if value, err := d.Value(); err == nil {
			if app, ok := value.(*postpilotv1.AppErrorDetail); ok {
				return app.GetReason()
			}
		}
	}
	t.Fatalf("no app error detail on %v", err)
	return ""
}

// The account comes from the session on both procedures, and the contract gives a caller
// nowhere to claim one.
func TestEveryProcedureRequiresASessionAndNoRequestCarriesAUserID(t *testing.T) {
	h := handler(saturated())
	anonymous := context.Background()
	if _, err := h.GetPostMeasurement(anonymous, connect.NewRequest(&postpilotv1.GetPostMeasurementRequest{Slug: "p0"})); connect.CodeOf(err) != connect.CodeUnauthenticated || reasonOf(t, err) != "AUTH_REQUIRED" {
		t.Fatalf("post measurement = %v", err)
	}
	if _, err := h.GetAccountQuality(anonymous, connect.NewRequest(&postpilotv1.GetAccountQualityRequest{Slug: "p0"})); connect.CodeOf(err) != connect.CodeUnauthenticated || reasonOf(t, err) != "AUTH_REQUIRED" {
		t.Fatalf("account quality = %v", err)
	}
	for _, message := range []proto.Message{&postpilotv1.GetPostMeasurementRequest{}, &postpilotv1.GetAccountQualityRequest{}} {
		fields := message.ProtoReflect().Descriptor().Fields()
		for i := 0; i < fields.Len(); i++ {
			switch name := string(fields.Get(i).Name()); name {
			case "user_id", "account_id", "owner_id":
				t.Fatalf("%s carries %s", message.ProtoReflect().Descriptor().FullName(), name)
			}
		}
	}
}

// ARCH-3: every generated metric but UNSPECIFIED is reached by exactly one domain metric.
func TestQualityMetricMappingWalksTheGeneratedEnum(t *testing.T) {
	if len(postpilotv1.QualityMetric_name) != len(quality.Metrics())+1 {
		t.Fatalf("%d wire metrics for %d domain metrics", len(postpilotv1.QualityMetric_name), len(quality.Metrics()))
	}
	reached := map[postpilotv1.QualityMetric]int{}
	for _, m := range quality.Metrics() {
		reached[toProtoMetric(m)]++
	}
	for value := range postpilotv1.QualityMetric_name {
		wire := postpilotv1.QualityMetric(value)
		want := 1
		if wire == postpilotv1.QualityMetric_QUALITY_METRIC_UNSPECIFIED {
			want = 0
		}
		if reached[wire] != want {
			t.Errorf("%s reached %d times, want %d", wire, reached[wire], want)
		}
	}
}

func TestQualityVerdictMappingWalksTheGeneratedEnum(t *testing.T) {
	if len(postpilotv1.QualityVerdict_name) != len(quality.Verdicts())+1 {
		t.Fatalf("%d wire verdicts for %d domain verdicts", len(postpilotv1.QualityVerdict_name), len(quality.Verdicts()))
	}
	reached := map[postpilotv1.QualityVerdict]int{}
	for _, v := range quality.Verdicts() {
		reached[toProtoVerdict(v)]++
	}
	for value := range postpilotv1.QualityVerdict_name {
		wire := postpilotv1.QualityVerdict(value)
		want := 1
		if wire == postpilotv1.QualityVerdict_QUALITY_VERDICT_UNSPECIFIED {
			want = 0
		}
		if reached[wire] != want {
			t.Errorf("%s reached %d times, want %d", wire, reached[wire], want)
		}
	}
}

func TestAnUnknownOrForeignSlugAnswersPostNotFound(t *testing.T) {
	h := handler(saturated())
	bob := auth.WithUser(context.Background(), "bob")
	for name, call := range map[string]func() error{
		"an empty slug": func() error {
			_, err := h.GetPostMeasurement(signedIn(), connect.NewRequest(&postpilotv1.GetPostMeasurementRequest{}))
			return err
		},
		"an unknown slug": func() error {
			_, err := h.GetAccountQuality(signedIn(), connect.NewRequest(&postpilotv1.GetAccountQualityRequest{Slug: "nobody"}))
			return err
		},
		"another account's post, measured": func() error {
			_, err := h.GetPostMeasurement(bob, connect.NewRequest(&postpilotv1.GetPostMeasurementRequest{Slug: "p0"}))
			return err
		},
		"another account's post, aggregated": func() error {
			_, err := h.GetAccountQuality(bob, connect.NewRequest(&postpilotv1.GetAccountQualityRequest{Slug: "p0"}))
			return err
		},
	} {
		if err := call(); connect.CodeOf(err) != connect.CodeNotFound || reasonOf(t, err) != "POST_NOT_FOUND" {
			t.Errorf("%s = %v", name, err)
		}
	}
}

// QUAL-40: a value that cannot be computed leaves its optional field unset, so it can never
// read as zero on the wire.
func TestAbsentValuesAreUnsetOnTheWire(t *testing.T) {
	h := handler(saturated())
	res, err := h.GetPostMeasurement(signedIn(), connect.NewRequest(&postpilotv1.GetPostMeasurementRequest{Slug: "draft"}))
	if err != nil {
		t.Fatal(err)
	}
	readings := res.Msg.GetReadings()
	wantMetrics := []postpilotv1.QualityMetric{
		postpilotv1.QualityMetric_QUALITY_METRIC_CROSS_POST_PHRASES,
		postpilotv1.QualityMetric_QUALITY_METRIC_IN_POST_REPETITION,
		postpilotv1.QualityMetric_QUALITY_METRIC_COMPOSITION,
	}
	if len(readings) != len(wantMetrics) {
		t.Fatalf("readings = %v", readings)
	}
	for i, reading := range readings {
		if reading.GetMetric() != wantMetrics[i] || reading.GetVerdict() != postpilotv1.QualityVerdict_QUALITY_VERDICT_ABSENT {
			t.Fatalf("reading %d = %s %s", i, reading.GetMetric(), reading.GetVerdict())
		}
	}
	if readings[0].GetCrossPostPhrases().Share != nil {
		t.Fatal("an absent M2 share was set")
	}
	if r := readings[1].GetInPostRepetition(); r.RepetitionShare != nil || r.TitleRelevance != nil {
		t.Fatalf("an absent M3 was set: %+v", r)
	}
	if c := readings[2].GetComposition(); c.CharCount != nil || c.PhotoCount != nil || c.DistinctBlockTypes != nil || c.AverageSentenceLength != nil {
		t.Fatalf("an absent M4 was set: %+v", c)
	}
	// The bands ride beside the values even when there is no value.
	if readings[1].GetInPostRepetition().GetRepetitionShareWarnAbove() != quality.RepetitionShareBand ||
		readings[1].GetInPostRepetition().GetTitleRelevanceWarnBelow() != quality.TitleRelevanceFloor {
		t.Fatal("M3 lost one of its two bands")
	}

	// Two published posts leave M1 under its minimum: the reading says so and carries no share.
	two := saturated()
	two.published = two.published[:2]
	account, err := handler(two).GetAccountQuality(signedIn(), connect.NewRequest(&postpilotv1.GetAccountQualityRequest{Slug: "p0"}))
	if err != nil {
		t.Fatal(err)
	}
	m1 := account.Msg.GetReadings()[0]
	if m1.GetVerdict() != postpilotv1.QualityVerdict_QUALITY_VERDICT_BELOW_MINIMUM || m1.GetTitleSaturation().Share != nil ||
		m1.GetMinimum() != 10 || m1.GetPublishedCount() != 2 {
		t.Fatalf("M1 under its minimum = %+v", m1)
	}
}

// mixed is ten published posts where three titles hold the noun: M1 is within its band and still
// names the noun, and a heading, a paragraph and a list keep M4 within its band. Both would
// render a rule text if asked, so their having none is what the band decides.
func mixed() *alicesPosts {
	posts := saturated()
	for i := range posts.published {
		doc := posts.published[i].Content
		if i >= 3 {
			doc.Title = fmt.Sprintf("점심 기록 %d", i)
		}
		doc.Blocks = append([]quality.Block{{Type: quality.BlockHeading, Content: "점심"}}, doc.Blocks...)
		doc.Blocks = append(doc.Blocks, quality.Block{Type: quality.BlockList, Items: []string{"국수", "김치"}})
		posts.published[i].Content = doc
	}
	return posts
}

func TestOnlyAnOverBandMetricCarriesARuleText(t *testing.T) {
	m1, _ := quality.RuleText(quality.MetricTitleSaturation, quality.LanguageKorean, "감자탕")
	for name, test := range map[string]struct {
		posts *alicesPosts
		over  map[postpilotv1.QualityMetric]bool
	}{
		"M1, M3 and M4 over": {saturated(), map[postpilotv1.QualityMetric]bool{
			postpilotv1.QualityMetric_QUALITY_METRIC_TITLE_SATURATION:   true,
			postpilotv1.QualityMetric_QUALITY_METRIC_IN_POST_REPETITION: true,
			postpilotv1.QualityMetric_QUALITY_METRIC_COMPOSITION:        true,
		}},
		"M3 over, M1 and M4 within yet renderable": {mixed(), map[postpilotv1.QualityMetric]bool{
			postpilotv1.QualityMetric_QUALITY_METRIC_IN_POST_REPETITION: true,
		}},
	} {
		t.Run(name, func(t *testing.T) {
			res, err := handler(test.posts).GetAccountQuality(signedIn(), connect.NewRequest(&postpilotv1.GetAccountQualityRequest{Slug: "p0"}))
			if err != nil {
				t.Fatal(err)
			}
			readings := res.Msg.GetReadings()
			if len(readings) != len(quality.Metrics()) {
				t.Fatalf("readings = %d", len(readings))
			}
			for i, reading := range readings {
				if reading.GetMetric() != toProtoMetric(quality.Metrics()[i]) {
					t.Fatalf("reading %d is %s, out of enum order", i, reading.GetMetric())
				}
				isOver := reading.GetVerdict() == postpilotv1.QualityVerdict_QUALITY_VERDICT_OVER_BAND
				if isOver != test.over[reading.GetMetric()] || isOver != (reading.GetRuleText() != "") {
					t.Fatalf("%s: verdict %s with rule text %q", reading.GetMetric(), reading.GetVerdict(), reading.GetRuleText())
				}
				if reading.GetPublishedCount() != 10 || reading.GetMinimum() != int32(quality.Minimum(quality.Metrics()[i])) {
					t.Fatalf("%s: minimum %d, published %d", reading.GetMetric(), reading.GetMinimum(), reading.GetPublishedCount())
				}
			}
			if test.over[postpilotv1.QualityMetric_QUALITY_METRIC_TITLE_SATURATION] && readings[0].GetRuleText() != m1 {
				t.Fatalf("M1 = %q, want %q", readings[0].GetRuleText(), m1)
			}
		})
	}
}
