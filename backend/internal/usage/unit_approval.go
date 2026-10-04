package usage

import "context"

type unitReservationKey struct{}

// WithUnitReservation carries a server-read approval through a consumer's
// ordinary queue admission. Payload fields and token-plan fallbacks cannot
// authorize unit work. Hold still checks the durable quote independently.
func WithUnitReservation(ctx context.Context, r *Reservation) context.Context {
	return context.WithValue(ctx, unitReservationKey{}, r)
}

func UnitReservationFromContext(ctx context.Context) (*Reservation, bool) {
	r, ok := ctx.Value(unitReservationKey{}).(*Reservation)
	return r, ok && r != nil && r.UnitQuoteID != "" && len(r.Units) > 0
}
