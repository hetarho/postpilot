package quality

// Verdict is one of a metric row's four states (QUAL-12).
type Verdict string

const (
	VerdictOverBand     Verdict = "over_band"
	VerdictWithinBand   Verdict = "within_band"
	VerdictBelowMinimum Verdict = "below_minimum"
	VerdictAbsent       Verdict = "absent"
)

// Verdicts lists the four in their canonical order.
func Verdicts() []Verdict {
	return []Verdict{VerdictOverBand, VerdictWithinBand, VerdictBelowMinimum, VerdictAbsent}
}

// bandTolerance keeps a band boundary where QUAL-11's words put it. A median averages two
// floats, and (0.2+0.4)/2 computes to 0.30000000000000004: without the tolerance a value that is
// exactly 30% would warn.
const bandTolerance = 1e-9

func above(value *float64, band float64) bool  { return value != nil && *value > band+bandTolerance }
func below(value *float64, floor float64) bool { return value != nil && *value < floor-bandTolerance }

// banded is the verdict of a single-valued metric: absent without a value (QUAL-40), otherwise
// over or within its band.
func banded(value *float64, over func(*float64) bool) Verdict {
	switch {
	case value == nil:
		return VerdictAbsent
	case over(value):
		return VerdictOverBand
	default:
		return VerdictWithinBand
	}
}

func titleSaturationOver(value *float64) bool { return above(value, TitleSaturationBand) }
func crossPostOver(value *float64) bool       { return above(value, CrossPostPhrasesBand) }

// compositionOver reads the distinct block types: two or fewer is over band (QUAL-11).
func compositionOver(value *float64) bool {
	return value != nil && *value <= DistinctBlockTypesBand+bandTolerance
}

// repetitionVerdict combines M3's two halves: over when either present half crosses, within when
// at least one is present and none crosses, absent when both are — an absent half neither passes
// nor warns (QUAL-40).
func repetitionVerdict(share, relevance *float64) Verdict {
	switch {
	case share == nil && relevance == nil:
		return VerdictAbsent
	case above(share, RepetitionShareBand) || below(relevance, TitleRelevanceFloor):
		return VerdictOverBand
	default:
		return VerdictWithinBand
	}
}

// Account is the aggregate over an account's published posts (QUAL-2): one reading per metric.
// There is deliberately no score that combines them (QUAL-27).
type Account struct {
	PublishedCount  int
	TitleSaturation AccountTitleSaturation
	CrossPost       AccountCrossPost
	Repetition      AccountRepetition
	Composition     AccountComposition
}

type AccountTitleSaturation struct {
	Verdict Verdict
	Value   *float64
	Noun    string
}

type AccountCrossPost struct {
	Verdict Verdict
	Median  *float64
	Run     string
}

type AccountRepetition struct {
	Verdict               Verdict
	Share, TitleRelevance *float64
}

type AccountComposition struct {
	Verdict                                                      Verdict
	CharCount, PhotoCount, DistinctBlockTypes, AvgSentenceLength *float64
}

// Verdict is the row state of one metric.
func (a Account) Verdict(m Metric) Verdict {
	switch m {
	case MetricTitleSaturation:
		return a.TitleSaturation.Verdict
	case MetricCrossPostPhrases:
		return a.CrossPost.Verdict
	case MetricInPostRepetition:
		return a.Repetition.Verdict
	case MetricComposition:
		return a.Composition.Verdict
	default:
		return VerdictAbsent
	}
}

// Named is the measured string a metric's rule text names (QUAL-14): M1's noun and M2's run;
// M3 and M4 name nothing (QUAL-43, QUAL-44).
func (a Account) Named(m Metric) string {
	switch m {
	case MetricTitleSaturation:
		return a.TitleSaturation.Noun
	case MetricCrossPostPhrases:
		return a.CrossPost.Run
	default:
		return ""
	}
}

// Aggregate reads the account from its published posts, newest first (QUAL-39), capped at
// TitleWindow. self[i] is the stored self-measurement of published[i] for the PostWindow most
// recent posts, and any entry missing there is measured on the spot. A metric under its minimum
// is BELOW_MINIMUM with nothing measured or named; one whose values cannot be computed is ABSENT;
// every other is judged against its band.
func Aggregate(published []Sample, self []Self) Account {
	if len(published) > TitleWindow {
		published = published[:TitleWindow]
	}
	account := Account{PublishedCount: len(published)}
	window := published[:min(PostWindow, len(published))]
	selves := make([]Self, len(window))
	for i := range window {
		if i < len(self) {
			selves[i] = self[i]
		} else {
			selves[i] = MeasureSelf(window[i])
		}
	}

	if met(account.PublishedCount, MetricTitleSaturation) {
		saturation := MeasureTitleSaturation(published)
		account.TitleSaturation = AccountTitleSaturation{
			Verdict: banded(saturation.Value, titleSaturationOver), Value: saturation.Value, Noun: saturation.Noun,
		}
	} else {
		account.TitleSaturation.Verdict = VerdictBelowMinimum
	}

	if met(account.PublishedCount, MetricCrossPostPhrases) {
		shares := make([]*float64, 0, len(window))
		for _, post := range window {
			if overlap, ok := MeasureCrossPost(post, OthersOf(published, post.Slug)); ok {
				share := overlap.Share
				shares = append(shares, &share)
			}
		}
		median := Median(shares)
		account.CrossPost = AccountCrossPost{Verdict: banded(median, crossPostOver), Median: median}
		if median != nil {
			account.CrossPost.Run = namedRun(published)
		}
	} else {
		account.CrossPost.Verdict = VerdictBelowMinimum
	}

	if met(account.PublishedCount, MetricInPostRepetition) {
		shares := make([]*float64, 0, len(selves))
		relevances := make([]*float64, 0, len(selves))
		for _, s := range selves {
			shares = append(shares, s.Repetition.Share)
			relevances = append(relevances, s.Repetition.TitleRelevance)
		}
		share, relevance := Median(shares), Median(relevances)
		account.Repetition = AccountRepetition{Verdict: repetitionVerdict(share, relevance), Share: share, TitleRelevance: relevance}
	} else {
		account.Repetition.Verdict = VerdictBelowMinimum
	}

	if met(account.PublishedCount, MetricComposition) {
		var chars, photos, types, sentences []*float64
		for _, s := range selves {
			c := s.Composition
			chars = append(chars, count(c.CharCount))
			photos = append(photos, count(c.PhotoCount))
			types = append(types, count(c.DistinctBlockTypes))
			sentences = append(sentences, c.AvgSentenceLength)
		}
		composition := AccountComposition{
			CharCount: Median(chars), PhotoCount: Median(photos), DistinctBlockTypes: Median(types), AvgSentenceLength: Median(sentences),
		}
		composition.Verdict = banded(composition.DistinctBlockTypes, compositionOver)
		account.Composition = composition
	} else {
		account.Composition.Verdict = VerdictBelowMinimum
	}
	return account
}

func met(published int, m Metric) bool { return published >= Minimum(m) }

func count(n int) *float64 {
	value := float64(n)
	return &value
}

// PostMeasurement is one post's own readings as ② shows them (QUAL-36): M2 against the account's
// other published posts, and M3 and M4 against the bands alone, since they have no minimum.
type PostMeasurement struct {
	CrossPost   PostCrossPost
	Repetition  PostRepetition
	Composition PostComposition
}

type PostCrossPost struct {
	Verdict Verdict
	Share   *float64
	// Others is how many published posts this one was compared with, the count ② names beside
	// the minimum.
	Others int
}

type PostRepetition struct {
	Verdict    Verdict
	Repetition Repetition
}

type PostComposition struct {
	Verdict     Verdict
	Composition *Composition
}

// JudgePost gives a post with content its three verdicts. M2 is BELOW_MINIMUM while fewer than
// the minimum of other published posts exist, and ABSENT when its share cannot be computed.
func JudgePost(post Sample, self Self, others []Sample) PostMeasurement {
	var m PostMeasurement
	m.CrossPost.Others = len(others)
	switch {
	case len(others) < Minimum(MetricCrossPostPhrases):
		m.CrossPost.Verdict = VerdictBelowMinimum
	default:
		if overlap, ok := MeasureCrossPost(post, others); ok {
			share := overlap.Share
			m.CrossPost.Share = &share
		}
		m.CrossPost.Verdict = banded(m.CrossPost.Share, crossPostOver)
	}
	m.Repetition = PostRepetition{
		Verdict:    repetitionVerdict(self.Repetition.Share, self.Repetition.TitleRelevance),
		Repetition: self.Repetition,
	}
	composition := self.Composition
	m.Composition = PostComposition{
		Verdict:     banded(count(composition.DistinctBlockTypes), compositionOver),
		Composition: &composition,
	}
	return m
}

// AbsentPost is a post with no content: nothing can be measured, so every row is absent.
func AbsentPost() PostMeasurement {
	return PostMeasurement{
		CrossPost:   PostCrossPost{Verdict: VerdictAbsent},
		Repetition:  PostRepetition{Verdict: VerdictAbsent},
		Composition: PostComposition{Verdict: VerdictAbsent},
	}
}
