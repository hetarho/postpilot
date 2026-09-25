package store_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/usage"
	usagestore "github.com/postpilot/backend/internal/usage/store"
	"github.com/postpilot/backend/internal/voucher"
	voucherstore "github.com/postpilot/backend/internal/voucher/store"
)

type fixedAnchors struct{}

func (fixedAnchors) AnchorFor(context.Context, string) (time.Time, error) {
	return time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), nil
}

// ledgerCredits is the composition root's adapter, restated here so the test wires the store
// the way production does: the redemption's lot rides the voucher store's transaction.
type ledgerCredits struct{ ledger *usage.Service }

func (c ledgerCredits) OpenVoucherLot(ctx context.Context, userID string, credits int, expiresAt time.Time) (string, error) {
	return c.ledger.OpenVoucherLot(ctx, userID, credits, expiresAt)
}

func (c ledgerCredits) ExpireVoucherLot(ctx context.Context, lotID string, at time.Time) error {
	return c.ledger.ExpireVoucherLot(ctx, lotID, at)
}

func (c ledgerCredits) VoucherLotStandings(ctx context.Context, lotIDs []string, at time.Time) (map[string]voucher.LotStanding, error) {
	standings, err := c.ledger.VoucherLotStandings(ctx, lotIDs, at)
	if err != nil {
		return nil, err
	}
	out := map[string]voucher.LotStanding{}
	for id, standing := range standings {
		out[id] = voucher.LotStanding{Remaining: standing.Remaining, ExpiresAt: standing.ExpiresAt}
	}
	return out, nil
}

func newService(t *testing.T) (*voucher.Service, *voucherstore.Store, *db.DB) {
	t.Helper()
	handle, err := db.Open(filepath.Join(t.TempDir(), "voucher.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { handle.Close() })
	ctx := context.Background()
	if err := db.Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"root", "alice", "bob"} {
		if _, err := handle.Writer.ExecContext(ctx,
			"INSERT INTO users (id, password_hash, plan, created_at) VALUES (?,'hash','free',?)",
			id, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			t.Fatal(err)
		}
	}
	store := voucherstore.New(handle.Writer, handle.Reader)
	store.SetCreditsForTx(func(tx *sql.Tx) voucher.Credits {
		return ledgerCredits{usage.NewService(usagestore.NewTx(tx), nil, 0, fixedAnchors{})}
	})
	ledger := usage.NewService(usagestore.New(handle.Writer, handle.Reader), nil, 0, fixedAnchors{})
	return voucher.NewService(store, ledgerCredits{ledger}), store, handle
}

func voucherLots(t *testing.T, handle *db.DB) int {
	t.Helper()
	var n int
	if err := handle.Writer.QueryRow("SELECT count(*) FROM credit_lots WHERE kind = 'voucher'").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// GIFT-9 under real concurrency: every racer reads the voucher as redeemable, and exactly
// one lot and one redemption survive.
func TestConcurrentRedemptionsOpenExactlyOneLot(t *testing.T) {
	svc, _, handle := newService(t)
	ctx := context.Background()
	issued, err := svc.Issue(ctx, "root", voucher.Issue{Credits: 1150, ValidityDays: 30})
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	results := make(chan error, 8)
	for i := range 8 {
		user := "alice"
		if i%2 == 1 {
			user = "bob"
		}
		wg.Go(func() {
			_, err := svc.Redeem(ctx, user, issued.Token)
			results <- err
		})
	}
	wg.Wait()
	close(results)

	wins := 0
	for err := range results {
		switch {
		case err == nil:
			wins++
		case errors.Is(err, voucher.ErrRedeemed):
		default:
			t.Fatalf("redeem: %v", err)
		}
	}
	if wins != 1 {
		t.Fatalf("wins = %d, want exactly one", wins)
	}
	if n := voucherLots(t, handle); n != 1 {
		t.Fatalf("voucher lots = %d, want one", n)
	}
}

// A refused redemption leaves no lot behind: the lot insert and the redemption row are one
// transaction.
func TestRefusedRedemptionRollsTheLotBack(t *testing.T) {
	svc, _, handle := newService(t)
	ctx := context.Background()
	issued, err := svc.Issue(ctx, "root", voucher.Issue{Credits: 10, ValidityDays: 30})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Revoke(ctx, issued.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Redeem(ctx, "alice", issued.Token); !errors.Is(err, voucher.ErrRevoked) {
		t.Fatalf("redeem revoked = %v", err)
	}
	if n := voucherLots(t, handle); n != 0 {
		t.Fatalf("voucher lots = %d after a refusal, want none", n)
	}
}

// The store maps both a sold and a given voucher, and every instant, faithfully.
func TestStoreRoundTripsSoldAndGivenVouchers(t *testing.T) {
	svc, store, _ := newService(t)
	ctx := context.Background()
	sold, err := svc.Issue(ctx, "root", voucher.Issue{
		Credits: 330, ValidityDays: 30, Sale: &voucher.Sale{AmountKRW: 4900, Payer: "김민수"}, Message: "파일럿 감사합니다",
	})
	if err != nil {
		t.Fatal(err)
	}
	given, err := svc.Issue(ctx, "root", voucher.Issue{Credits: 50, ValidityDays: 7})
	if err != nil {
		t.Fatal(err)
	}

	got, found, err := store.VoucherByToken(ctx, sold.Token)
	if err != nil || !found {
		t.Fatalf("by token = %v %v", found, err)
	}
	if got.ID != sold.ID || got.Credits != 330 || got.ValidityDays != 30 || got.Sale == nil ||
		got.Sale.AmountKRW != 4900 || got.Sale.Payer != "김민수" || got.Message != "파일럿 감사합니다" ||
		got.IssuedBy != "root" || !got.LinkExpiresAt.Equal(sold.LinkExpiresAt) || got.RedeemedAt != nil || got.RevokedAt != nil {
		t.Fatalf("sold = %+v", got)
	}
	byID, found, err := store.VoucherByID(ctx, given.ID)
	if err != nil || !found || byID.Sale != nil {
		t.Fatalf("given = %+v %v %v", byID, found, err)
	}
	if _, found, err := store.VoucherByToken(ctx, "missing"); err != nil || found {
		t.Fatalf("missing token = %v %v", found, err)
	}

	all, err := store.Vouchers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("vouchers = %d", len(all))
	}
}

// Revoking a redeemed voucher expires its lot in the ledger, so the credits stop counting.
func TestRevokingARedeemedVoucherExpiresItsLot(t *testing.T) {
	svc, _, handle := newService(t)
	ctx := context.Background()
	issued, err := svc.Issue(ctx, "root", voucher.Issue{Credits: 1150, ValidityDays: 30})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Redeem(ctx, "alice", issued.Token); err != nil {
		t.Fatal(err)
	}
	listed, err := svc.Revoke(ctx, issued.ID)
	if err != nil {
		t.Fatal(err)
	}
	if listed.State != voucher.StateRevoked || listed.RemainingCredits != 0 {
		t.Fatalf("revoked = %+v", listed)
	}
	var expires string
	if err := handle.Writer.QueryRow("SELECT expires_at FROM credit_lots WHERE kind = 'voucher'").Scan(&expires); err != nil {
		t.Fatal(err)
	}
	stored, err := time.Parse(time.RFC3339Nano, expires)
	if err != nil {
		t.Fatal(err)
	}
	if stored.After(time.Now()) {
		t.Fatalf("lot still expires at %v, want no later than now", stored)
	}
}
