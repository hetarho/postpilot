package voucher

import (
	"context"
	"time"
)

// Store is the persistence this context needs, declared by its consumer (ARCH-6).
type Store interface {
	// InWriteTx runs fn in ONE write transaction with a store and a credit port bound to it.
	// A redemption needs it: the lot it opens and the row that marks the voucher redeemed
	// commit together or not at all.
	InWriteTx(ctx context.Context, fn func(Store, Credits) error) error
	InsertVoucher(ctx context.Context, voucher Voucher) error
	VoucherByToken(ctx context.Context, token string) (Voucher, bool, error)
	VoucherByID(ctx context.Context, id string) (Voucher, bool, error)
	// Vouchers lists every voucher, newest first.
	Vouchers(ctx context.Context) ([]Voucher, error)
	// MarkRedeemed records the redemption only while the voucher is neither redeemed nor
	// revoked, and reports whether it did — the guard two racing redemptions meet.
	MarkRedeemed(ctx context.Context, id, userID, lotID string, at time.Time) (bool, error)
	// MarkRevoked records the revocation only once, and reports whether it did.
	MarkRevoked(ctx context.Context, id string, at time.Time) (bool, error)
}

// Credits is the credit ledger as a voucher asks for it (QUOTA-58).
type Credits interface {
	OpenVoucherLot(ctx context.Context, userID string, credits int, expiresAt time.Time) (lotID string, err error)
	ExpireVoucherLot(ctx context.Context, lotID string, at time.Time) error
	// VoucherLotStandings reads, for each lot, what it still holds at `at` (zero once
	// expired) and when it expires. An id it omits is absent.
	VoucherLotStandings(ctx context.Context, lotIDs []string, at time.Time) (map[string]LotStanding, error)
}
