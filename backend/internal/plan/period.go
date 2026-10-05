package plan

import (
	"errors"
	"math"
	"math/big"
	"time"
)

// MonthBoundary derives every boundary from the original subscription instant. This
// restores a day such as the 31st after a shorter month and preserves nanoseconds.
func MonthBoundary(anchor time.Time, index int) time.Time {
	a := anchor.In(seoul)
	first := time.Date(a.Year(), a.Month()+time.Month(index), 1, 0, 0, 0, 0, seoul)
	lastDay := time.Date(first.Year(), first.Month()+1, 0, 0, 0, 0, 0, seoul).Day()
	return time.Date(first.Year(), first.Month(), min(a.Day(), lastDay),
		a.Hour(), a.Minute(), a.Second(), a.Nanosecond(), seoul)
}

// BenefitWindow returns the half-open anchored calendar-month window containing at.
func BenefitWindow(anchor, at time.Time) (start, end time.Time) {
	a, n := anchor.In(seoul), at.In(seoul)
	index := (n.Year()-a.Year())*12 + int(n.Month()-a.Month())
	start = MonthBoundary(anchor, index)
	if at.Before(start) {
		index--
		start = MonthBoundary(anchor, index)
	}
	return start, MonthBoundary(anchor, index+1)
}

// DailyWindow uses consecutive 24-hour intervals from the paid start instant.
func DailyWindow(anchor, at time.Time) (start, end time.Time) {
	index := int64(at.Sub(anchor) / (24 * time.Hour))
	if at.Before(anchor) && at.Sub(anchor)%(24*time.Hour) != 0 {
		index--
	}
	start = anchor.Add(time.Duration(index) * 24 * time.Hour).In(seoul)
	return start, start.Add(24 * time.Hour)
}

// CoverageEnd separates paid coverage from monthly benefit renewal: a paid term that starts
// at start ends one, or for annual twelve, anchored calendar months after the calendar month
// start falls in, counted from the anchor so a clamped month returns to the original day.
// Annual means twelve calendar months and never a fixed 365-day duration.
func CoverageEnd(anchor, start time.Time, annual bool) time.Time {
	months := 1
	if annual {
		months = 12
	}
	a, s := anchor.In(seoul), start.In(seoul)
	index := (s.Year()-a.Year())*12 + int(s.Month()-a.Month())
	return MonthBoundary(anchor, index+months)
}

var ErrInvalidProration = errors.New("invalid proration interval or amount")

// ProrateFloor and ProrateCeil use elapsed instants and checked integer arithmetic.
func ProrateFloor(amount int64, start, end, at time.Time) (int64, error) {
	return prorate(amount, start, end, at, false)
}

func ProrateCeil(amount int64, start, end, at time.Time) (int64, error) {
	return prorate(amount, start, end, at, true)
}

func prorate(amount int64, start, end, at time.Time, roundUp bool) (int64, error) {
	if amount < 0 || !end.After(start) || at.Before(start) || at.After(end) {
		return 0, ErrInvalidProration
	}
	remaining, total := end.Sub(at), end.Sub(start)
	if remaining < 0 || total <= 0 {
		return 0, ErrInvalidProration
	}
	n := new(big.Int).Mul(big.NewInt(amount), big.NewInt(int64(remaining)))
	q, r := new(big.Int).QuoRem(n, big.NewInt(int64(total)), new(big.Int))
	if roundUp && r.Sign() > 0 {
		q.Add(q, big.NewInt(1))
	}
	if !q.IsInt64() || q.Int64() > math.MaxInt64 {
		return 0, ErrInvalidProration
	}
	return q.Int64(), nil
}

type UpgradeAmounts struct {
	ChargeKRW     int64
	BonusCredits  int64
	ServerExports int64
}

// QuoteUpgrade compares the current tier with the requested higher tier once. Callers
// supply the frozen payment instant and the existing paid and benefit windows.
func QuoteUpgrade(current, next Plan, annual bool, paidStart, paidEnd, benefitStart, benefitEnd, at time.Time) (UpgradeAmounts, error) {
	oldOffer, oldOK := CommercialOffer(current)
	newOffer, newOK := CommercialOffer(next)
	if !oldOK || !newOK || current == Free || next.Rank() <= current.Rank() {
		return UpgradeAmounts{}, ErrInvalidProration
	}
	priceDifference := newOffer.MonthlyKRW - oldOffer.MonthlyKRW
	if annual {
		priceDifference = newOffer.AnnualKRW - oldOffer.AnnualKRW
	}
	charge, err := ProrateCeil(int64(priceDifference), paidStart, paidEnd, at)
	if err != nil {
		return UpgradeAmounts{}, err
	}
	bonus, err := ProrateCeil(int64(newOffer.MonthlyBonus-oldOffer.MonthlyBonus), benefitStart, benefitEnd, at)
	if err != nil {
		return UpgradeAmounts{}, err
	}
	exports, err := ProrateFloor(int64(newOffer.ServerExports-oldOffer.ServerExports), benefitStart, benefitEnd, at)
	if err != nil {
		return UpgradeAmounts{}, err
	}
	return UpgradeAmounts{charge, bonus, exports}, nil
}
