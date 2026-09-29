package plan

// EstimatorRatesAt prices the same visible post assumptions as EstimatorRates
// in KRW milli-credits at the published job-rate denomination. Components
// truncate at milli precision; the actual job is rounded once at settlement.
func EstimatorRatesAt(observe, write Pricer, rate RateSnapshot) (Rates, bool) {
	if !rate.Valid() {
		return Rates{}, false
	}
	price := func(p Pricer, in, out int64) (int, bool) {
		cost, ok := p(estimatorTokenAllowance(in), estimatorTokenAllowance(out))
		if !ok {
			return 0, false
		}
		milli, err := MilliAt(cost, rate)
		return milli, err == nil
	}
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
