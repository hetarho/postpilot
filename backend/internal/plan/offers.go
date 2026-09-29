package plan

// offerRules is the single source of commercial amounts. Credits and KRW are integers;
// a zero allowance means none, including for the free tier.
type offerRule struct {
	monthlyKRW    int
	annualKRW     int
	dailyCredits  int
	monthlyBonus  int
	modelCeiling  string
	serverExports int
}

var offerRules = map[Plan]offerRule{
	Free:  {modelCeiling: "none"},
	Light: {1900, 19000, 15, 290, "value", 2},
	Basic: {4900, 49000, 45, 510, "balanced", 6},
	Pro:   {9900, 99000, 85, 1070, "premium", 15},
	Max:   {29900, 299000, 235, 3170, "top", 60},
}

// CommercialOffer excludes the operator tier. The bool is false for an unknown plan.
func CommercialOffer(p Plan) (Offer, bool) {
	for _, offer := range Offers() {
		if offer.Plan == p {
			return offer, true
		}
	}
	return Offer{}, false
}

// Pack is a fixed-price, non-expiring credit purchase for an active paid subscriber.
type Pack struct {
	ID       string
	PriceKRW int
	Credits  int
}

func Packs() []Pack {
	return []Pack{
		{ID: "pack-1000", PriceKRW: 3000, Credits: 1000},
		{ID: "pack-3000", PriceKRW: 9000, Credits: 3000},
		{ID: "pack-10000", PriceKRW: 30000, Credits: 10000},
	}
}

func PackByID(id string) (Pack, bool) {
	for _, pack := range Packs() {
		if pack.ID == id {
			return pack, true
		}
	}
	return Pack{}, false
}
