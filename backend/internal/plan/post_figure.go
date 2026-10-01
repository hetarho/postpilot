package plan

import (
	"sort"
	"time"
)

// The recent-usage per-post figure's product rules (QUOTA-64). They live with every other
// number the product publishes, so two deploys cannot disagree about them (QUOTA-8).
const (
	// PostFigureWindow is how far back real posts are read.
	PostFigureWindow = 30 * 24 * time.Hour
	// PostFigureMinPosts and PostFigureMinAccounts are the sample floor below which the
	// catalog-based estimate stands in: a thinner sample would expose one account's usage and
	// swing with every post.
	PostFigureMinPosts    = 10
	PostFigureMinAccounts = 3
	// EstimatorDefaultPhotos and EstimatorDefaultChars are the item conditions the
	// catalog-based estimate prices one post at (QUOTA-40).
	EstimatorDefaultPhotos = 5
	EstimatorDefaultChars  = 1_000
)

// PostCreditsBasis says where a per-post figure came from.
type PostCreditsBasis string

const (
	PostCreditsRecentUsage PostCreditsBasis = "recent_usage"
	PostCreditsEstimate    PostCreditsBasis = "estimate"
)

// PostFigure is what one post costs one stage in credits, and where the number came from.
type PostFigure struct {
	Credits int
	Basis   PostCreditsBasis
}

// Plus is two stages' figures as one post's: an estimate on either side makes the sum one.
func (f PostFigure) Plus(other PostFigure) PostFigure {
	basis := PostCreditsRecentUsage
	if f.Basis != PostCreditsRecentUsage || other.Basis != PostCreditsRecentUsage {
		basis = PostCreditsEstimate
	}
	return PostFigure{Credits: f.Credits + other.Credits, Basis: basis}
}

// UpperMedian is the middle of the values, and of an even count the mean of the two middle
// values rounded up: an estimate a user plans a month by should not understate. Zero values
// give zero.
func UpperMedian(values []int) int {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]int(nil), values...)
	sort.Ints(sorted)
	middle := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[middle]
	}
	return (sorted[middle-1] + sorted[middle] + 1) / 2
}

// ObservePostCreditsAt is the catalog-based observe share of one post with `photos` photos:
// the same per-photo assumption the estimator cards use, rounded up to a whole credit.
func ObservePostCreditsAt(observe Pricer, rate RateSnapshot, photos int) (int, bool) {
	if !rate.Valid() || photos < 0 {
		return 0, false
	}
	share, ok := estimatorMilliAt(observe, rate, estimatorObservePromptTokens/estimatorObserveBatch, 0)
	if !ok {
		return 0, false
	}
	photo, ok := estimatorMilliAt(observe, rate, estimatorTokensPerPhoto, estimatorObserveOutputPerItem)
	if !ok {
		return 0, false
	}
	return wholeCredits(photos * (photo + share)), true
}

// WritePostCreditsAt is the catalog-based write share of one post of `chars` finished
// characters: the prompt base plus the per-1000-character output, rounded up.
func WritePostCreditsAt(write Pricer, rate RateSnapshot, chars int) (int, bool) {
	if !rate.Valid() || chars < 0 {
		return 0, false
	}
	base, ok := estimatorMilliAt(write, rate, estimatorWritePromptTokens, 0)
	if !ok {
		return 0, false
	}
	perThousand, ok := estimatorMilliAt(write, rate, 0, 10*estimatorOutputTokensPer100Chars)
	if !ok {
		return 0, false
	}
	return wholeCredits(base + chars*perThousand/1_000), true
}

// CallCreditsAt is the catalog-based price of ONE assumed call of `prompt` input and
// `completion` output tokens, with the same edit allowance the post estimate applies, rounded up
// to a whole credit (QUOTA-40). It is an estimate, never a quote: a job's hold and settlement
// stay authoritative.
func CallCreditsAt(p Pricer, rate RateSnapshot, prompt, completion int64) (int, bool) {
	if !rate.Valid() || prompt < 0 || completion < 0 {
		return 0, false
	}
	milli, ok := estimatorMilliAt(p, rate, prompt, completion)
	if !ok {
		return 0, false
	}
	return wholeCredits(milli), true
}

func wholeCredits(milli int) int {
	if milli <= 0 {
		return 0
	}
	return (milli + 999) / 1_000
}
