package quality

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Deps are the service's collaborators (ARCH-40). Every one is required.
type Deps struct {
	Measurements Measurements
	Phrases      PhraseLists
	Posts        PostSource
	Now          func() time.Time
}

// Service answers what code can count about a post and an account (QUAL-1, QUAL-2). It calls
// no provider and costs no credit (QUAL-16): every value is arithmetic over stored text.
type Service struct {
	measurements Measurements
	phrases      PhraseLists
	posts        PostSource
	now          func() time.Time
}

func NewService(deps Deps) *Service {
	if deps.Measurements == nil {
		panic("quality: measurements collaborator is required")
	}
	if deps.Phrases == nil {
		panic("quality: phrases collaborator is required")
	}
	if deps.Posts == nil {
		panic("quality: posts collaborator is required")
	}
	// Tested directly rather than through an `any`: a nil func stored in one does not compare
	// equal to nil.
	if deps.Now == nil {
		panic("quality: now collaborator is required")
	}
	return &Service{measurements: deps.Measurements, phrases: deps.Phrases, posts: deps.Posts, now: deps.Now}
}

// PostReading is one post's own readings at the content revision they describe (QUAL-3).
type PostReading struct {
	Revision    int64
	Measurement PostMeasurement
}

// AccountReading is the aggregate and, for each metric over its band, the one rule text it
// offers, already rendered in the post's target language.
type AccountReading struct {
	Account   Account
	RuleTexts map[Metric]string
}

// PostMeasurement is ②'s reading of one post (QUAL-36). M3 and M4 are the revision's own and
// are stored against it; M2 depends on the published window and is measured at read. A post
// with no content has nothing to measure, so it reads and writes no row.
func (s *Service) PostMeasurement(ctx context.Context, userID, slug string) (PostReading, error) {
	snapshot, err := s.ownedPost(ctx, userID, slug)
	if err != nil {
		return PostReading{}, err
	}
	if snapshot.Content == nil {
		return PostReading{Revision: snapshot.Revision, Measurement: AbsentPost()}, nil
	}
	sample := Sample{Slug: snapshot.Slug, Doc: *snapshot.Content, Language: LanguageOf(snapshot.ContentLanguage), Nouns: snapshot.Nouns}
	self, err := s.self(ctx, userID, sample, snapshot.Revision)
	if err != nil {
		return PostReading{}, err
	}
	// One more than the window, which is enough for twenty others when the post is itself
	// published.
	published, err := s.posts.Published(ctx, userID, PostWindow+1)
	if err != nil {
		return PostReading{}, fmt.Errorf("read published posts: %w", err)
	}
	others := OthersOf(publishedSamples(published), snapshot.Slug)
	return PostReading{Revision: snapshot.Revision, Measurement: JudgePost(sample, self, others)}, nil
}

// AccountQuality is the writing brief's reading of the account (POST-81): the aggregate over its
// published posts, derived at read (QUAL-2), with the rule text of each metric over its band in
// the target language of the post the brief belongs to.
func (s *Service) AccountQuality(ctx context.Context, userID, slug string) (AccountReading, error) {
	snapshot, err := s.ownedPost(ctx, userID, slug)
	if err != nil {
		return AccountReading{}, err
	}
	account, err := s.aggregate(ctx, userID)
	if err != nil {
		return AccountReading{}, err
	}
	texts := map[Metric]string{}
	for _, m := range Metrics() {
		if account.Verdict(m) != VerdictOverBand {
			continue
		}
		if text, ok := RuleText(m, snapshot.TargetLanguage, account.Named(m)); ok {
			texts[m] = text
		}
	}
	return AccountReading{Account: account, RuleTexts: texts}, nil
}

// RulesFor renders the rule texts of the ticked metrics that are over their band now, in M1–M4
// order, for the enqueue to freeze. An unknown or repeated id is dropped, and with nothing left
// ticked it reads nothing at all: ticking nothing adds no bytes and costs no query.
func (s *Service) RulesFor(ctx context.Context, userID, slug string, ticked []string, lang Language) ([]string, error) {
	wanted := map[Metric]bool{}
	for _, id := range ticked {
		if m, ok := ParseMetric(id); ok {
			wanted[m] = true
		}
	}
	if len(wanted) == 0 {
		return nil, nil
	}
	// The slug is what proves the caller owns the post the rules are frozen into.
	if _, err := s.ownedPost(ctx, userID, slug); err != nil {
		return nil, err
	}
	account, err := s.aggregate(ctx, userID)
	if err != nil {
		return nil, err
	}
	var texts []string
	for _, m := range Metrics() {
		if !wanted[m] || account.Verdict(m) != VerdictOverBand {
			continue
		}
		if text, ok := RuleText(m, lang, account.Named(m)); ok {
			texts = append(texts, text)
		}
	}
	return texts, nil
}

// PhrasesFor is one field's stored phrases in rank order (QUAL-41), empty for a blank field, a
// field the batch has not written yet, or an empty row.
func (s *Service) PhrasesFor(ctx context.Context, field string) ([]string, error) {
	field = strings.TrimSpace(field)
	if field == "" {
		return nil, nil
	}
	list, found, err := s.phrases.PhraseList(ctx, field)
	if err != nil {
		return nil, fmt.Errorf("read phrase list: %w", err)
	}
	if !found || len(list.Phrases) == 0 {
		return nil, nil
	}
	return append([]string(nil), list.Phrases...), nil
}

func (s *Service) ownedPost(ctx context.Context, userID, slug string) (PostSnapshot, error) {
	if strings.TrimSpace(slug) == "" {
		return PostSnapshot{}, ErrPostNotFound
	}
	return s.posts.Post(ctx, userID, slug)
}

// aggregate reads the account over at most TitleWindow published posts, the PostWindow most
// recent of them through their stored self-measurements.
func (s *Service) aggregate(ctx context.Context, userID string) (Account, error) {
	published, err := s.posts.Published(ctx, userID, TitleWindow)
	if err != nil {
		return Account{}, fmt.Errorf("read published posts: %w", err)
	}
	samples := publishedSamples(published)
	selves := make([]Self, 0, min(PostWindow, len(samples)))
	for i := range min(PostWindow, len(samples)) {
		self, err := s.self(ctx, userID, samples[i], published[i].Revision)
		if err != nil {
			return Account{}, err
		}
		selves = append(selves, self)
	}
	return Aggregate(samples, selves), nil
}

// self is one revision's self-measurement: the stored row when it describes this revision under
// this MeasureVersion, otherwise measured now and stored. Two reads racing compute the same
// value, and a row a late write left at an older revision is measured again on the next read,
// because the row is keyed by revision and version.
func (s *Service) self(ctx context.Context, userID string, sample Sample, revision int64) (Self, error) {
	stored, found, err := s.measurements.Measurement(ctx, userID, sample.Slug)
	if err != nil {
		return Self{}, fmt.Errorf("read measurement: %w", err)
	}
	if found && stored.Revision == revision && stored.MeasureVersion == MeasureVersion {
		return stored.Self, nil
	}
	self := MeasureSelf(sample)
	if err := s.measurements.SaveMeasurement(ctx, StoredMeasurement{
		PostSlug: sample.Slug, UserID: userID, Revision: revision, MeasureVersion: MeasureVersion,
		Self: self, ComputedAt: s.now().UTC(),
	}); err != nil {
		return Self{}, fmt.Errorf("save measurement: %w", err)
	}
	return self, nil
}

func publishedSamples(published []PublishedPost) []Sample {
	samples := make([]Sample, len(published))
	for i, post := range published {
		samples[i] = Sample{Slug: post.Slug, Doc: post.Content, Language: LanguageOf(post.ContentLanguage), Nouns: post.Nouns}
	}
	return samples
}
