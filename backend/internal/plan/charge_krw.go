package plan

import (
	"errors"
	"math"
	"math/big"
)

// RateSnapshot is the reference and the disclosed rounded rate frozen for one
// admitted AI job. E4 keeps the supplier's decimal precision without floats.
type RateSnapshot struct {
	Source          string
	PublicationDate string
	ReferenceE4     int64
	AppliedE4       int64
	Temporary       bool
}

func (r RateSnapshot) Valid() bool {
	return r.Source != "" && r.PublicationDate != "" && r.ReferenceE4 > 0 &&
		r.AppliedE4 >= r.ReferenceE4 && r.AppliedE4%100_000 == 0
}

// AppliedRateE4 rounds the reference up to a whole ten KRW per USD.
func AppliedRateE4(referenceE4 int64) (int64, error) {
	if referenceE4 <= 0 || referenceE4 > math.MaxInt64-99_999 {
		return 0, errors.New("invalid KRW/USD reference")
	}
	return ((referenceE4 + 99_999) / 100_000) * 100_000, nil
}

// ChargeAt converts the summed confirmed micro-USD cost to whole KRW credits,
// rounding once for the user-visible job. Zero confirmed cost costs zero.
func ChargeAt(costMicrousd int64, rate RateSnapshot) (int, error) {
	if costMicrousd < 0 || !rate.Valid() {
		return 0, errors.New("invalid cost or frozen rate")
	}
	if costMicrousd == 0 {
		return 0, nil
	}
	n := new(big.Int).Mul(big.NewInt(costMicrousd), big.NewInt(rate.AppliedE4))
	n.Add(n, big.NewInt(10_000_000_000-1)).Quo(n, big.NewInt(10_000_000_000))
	if !n.IsInt64() || n.Int64() > math.MaxInt32 {
		return 0, errors.New("KRW credit charge exceeds limit")
	}
	return int(n.Int64()), nil
}

// MilliAt is the same denomination at one-thousandth credit precision for
// estimate components. The final job charge still rounds only once.
func MilliAt(costMicrousd int64, rate RateSnapshot) (int, error) {
	if costMicrousd < 0 || !rate.Valid() {
		return 0, errors.New("invalid cost or frozen rate")
	}
	n := new(big.Int).Mul(big.NewInt(costMicrousd), big.NewInt(rate.AppliedE4))
	n.Quo(n, big.NewInt(10_000_000))
	if !n.IsInt64() || n.Int64() > math.MaxInt32 {
		return 0, errors.New("KRW milli-credit estimate exceeds limit")
	}
	return int(n.Int64()), nil
}
