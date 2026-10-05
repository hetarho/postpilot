package plan

// EstimatorRatesAt derives one combo's unit rates from what its two models charge, in KRW
// milli-credits at the rate a new job would freeze (QUOTA-59). Components truncate at milli
// precision; the actual job is rounded once at settlement.
//
// An observation call is shared by the batch it carries, so a photo or a clip carries one
// batch-share of that call's prompt — amortized rather than counted with a ceiling, because
// the client is only allowed to multiply; the figure is labelled an estimate and the refusal
// stays authoritative (QUOTA-36).
//
// False means a model published no usable price or the rate is not one a job could freeze,
// and a combo that cannot be priced is not published at all.
func EstimatorRatesAt(observe, write Pricer, rate RateSnapshot) (Rates, bool) {
	if !rate.Valid() {
		return Rates{}, false
	}
	price := func(p Pricer, in, out int64) (int, bool) { return estimatorMilliAt(p, rate, in, out) }
	share, ok := price(observe, estimatorObservePromptTokens/estimatorObserveBatch, 0)
	if !ok {
		return Rates{}, false
	}
	photo, ok := price(observe, estimatorTokensPerPhoto, estimatorObserveOutputPerItem)
	if !ok {
		return Rates{}, false
	}
	video, ok := price(observe, estimatorAssumedVideoSeconds*estimatorTokensPerVideoSec, estimatorObserveOutputPerItem)
	if !ok {
		return Rates{}, false
	}
	prompt, ok := price(write, estimatorWritePromptTokens, 0)
	if !ok {
		return Rates{}, false
	}
	chars, ok := price(write, 0, 10*estimatorOutputTokensPer100Chars)
	if !ok {
		return Rates{}, false
	}
	return Rates{PerPhoto: photo + share, PerVideo: video + share,
		Per1000Chars: chars, PerPostBase: prompt}, true
}

// ClipEstimatorRatesAt prices one combo's clip comparison the same way: KRW milli-credits at
// the given rate, false when a model has no usable price or the rate is not valid.
func ClipEstimatorRatesAt(observe, write Pricer, rate RateSnapshot) (ClipRates, bool) {
	if !rate.Valid() {
		return ClipRates{}, false
	}
	price := func(p Pricer, in, out int64) (int, bool) {
		cost, ok := p(estimatorTokenAllowance(in), estimatorTokenAllowance(out))
		if !ok {
			return 0, false
		}
		milli, err := MilliAt(cost, rate)
		return milli, err == nil
	}
	source, ok := price(observe, estimatorObservePromptTokens+EstimatorClipSourceSeconds*estimatorTokensPerVideoSec, 2_000)
	if !ok {
		return ClipRates{}, false
	}
	context, ok := price(write, 2*2_000, 0)
	if !ok {
		return ClipRates{}, false
	}
	prompts, ok := price(write, 10_000+6_000, 0)
	if !ok {
		return ClipRates{}, false
	}
	output, ok := price(write, 0, 80)
	if !ok {
		return ClipRates{}, false
	}
	return ClipRates{PerSource: source, PerOutputSecond: output,
		PerClipBase: context + prompts}, true
}

// estimatorMilliAt prices one assumed call, with the edit allowance applied to both token
// counts, in KRW milli-credits at the given rate.
func estimatorMilliAt(p Pricer, rate RateSnapshot, in, out int64) (int, bool) {
	cost, ok := p(estimatorTokenAllowance(in), estimatorTokenAllowance(out))
	if !ok {
		return 0, false
	}
	milli, err := MilliAt(cost, rate)
	return milli, err == nil
}
