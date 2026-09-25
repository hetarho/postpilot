package voucher

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/plan"
)

// fakeStore keeps vouchers in memory. InWriteTx is a pass-through: these tests assert the
// rules, and the real store's transaction is exercised in store/store_test.go.
type fakeStore struct {
	vouchers []Voucher
	credits  *fakeCredits
}

func (f *fakeStore) InWriteTx(_ context.Context, fn func(Store, Credits) error) error {
	return fn(f, f.credits)
}

func (f *fakeStore) InsertVoucher(_ context.Context, v Voucher) error {
	f.vouchers = append(f.vouchers, v)
	return nil
}

func (f *fakeStore) find(match func(Voucher) bool) (*Voucher, bool) {
	for i := range f.vouchers {
		if match(f.vouchers[i]) {
			return &f.vouchers[i], true
		}
	}
	return nil, false
}

func (f *fakeStore) VoucherByToken(_ context.Context, token string) (Voucher, bool, error) {
	v, ok := f.find(func(v Voucher) bool { return v.Token == token })
	if !ok {
		return Voucher{}, false, nil
	}
	return *v, true, nil
}

func (f *fakeStore) VoucherByID(_ context.Context, id string) (Voucher, bool, error) {
	v, ok := f.find(func(v Voucher) bool { return v.ID == id })
	if !ok {
		return Voucher{}, false, nil
	}
	return *v, true, nil
}

func (f *fakeStore) Vouchers(context.Context) ([]Voucher, error) {
	out := make([]Voucher, len(f.vouchers))
	for i := range f.vouchers {
		out[len(f.vouchers)-1-i] = f.vouchers[i]
	}
	return out, nil
}

func (f *fakeStore) MarkRedeemed(_ context.Context, id, userID, lotID string, at time.Time) (bool, error) {
	v, ok := f.find(func(v Voucher) bool { return v.ID == id })
	if !ok || v.RedeemedAt != nil || v.RevokedAt != nil {
		return false, nil
	}
	v.RedeemedBy, v.LotID, v.RedeemedAt = userID, lotID, &at
	return true, nil
}

func (f *fakeStore) MarkRevoked(_ context.Context, id string, at time.Time) (bool, error) {
	v, ok := f.find(func(v Voucher) bool { return v.ID == id })
	if !ok || v.RevokedAt != nil {
		return false, nil
	}
	v.RevokedAt = &at
	return true, nil
}

type fakeLot struct {
	user      string
	remaining int
	expires   time.Time
}

type fakeCredits struct {
	lots map[string]*fakeLot
	seq  int
}

func (f *fakeCredits) OpenVoucherLot(_ context.Context, userID string, credits int, expiresAt time.Time) (string, error) {
	f.seq++
	id := fmt.Sprintf("voucher:%d", f.seq)
	f.lots[id] = &fakeLot{user: userID, remaining: credits, expires: expiresAt}
	return id, nil
}

func (f *fakeCredits) ExpireVoucherLot(_ context.Context, lotID string, at time.Time) error {
	lot, ok := f.lots[lotID]
	if !ok {
		return errors.New("no such lot")
	}
	if lot.expires.After(at) {
		lot.expires = at
	}
	return nil
}

func (f *fakeCredits) VoucherLotStandings(_ context.Context, lotIDs []string, at time.Time) (map[string]LotStanding, error) {
	out := map[string]LotStanding{}
	for _, id := range lotIDs {
		lot, ok := f.lots[id]
		if !ok {
			continue
		}
		standing := LotStanding{Remaining: lot.remaining, ExpiresAt: lot.expires}
		if !at.Before(lot.expires) {
			standing.Remaining = 0
		}
		out[id] = standing
	}
	return out, nil
}

var issuedAt = time.Date(2026, 9, 25, 3, 0, 0, 0, time.UTC)

func newTestService(t *testing.T) (*Service, *fakeStore, *time.Time) {
	t.Helper()
	credits := &fakeCredits{lots: map[string]*fakeLot{}}
	store := &fakeStore{credits: credits}
	svc := NewService(store, credits)
	now := issuedAt
	svc.now = func() time.Time { return now }
	seq := 0
	svc.newID = func() string { seq++; return fmt.Sprintf("v-%d", seq) }
	svc.newToken = func() (string, error) { return fmt.Sprintf("token-%d", seq+1), nil }
	return svc, store, &now
}

func TestIssueValidatesTheBounds(t *testing.T) {
	ctx := context.Background()
	valid := Issue{Credits: 1150, ValidityDays: 30}
	for name, request := range map[string]Issue{
		"zero credits":       {Credits: 0, ValidityDays: 30},
		"too many credits":   {Credits: MaxCredits + 1, ValidityDays: 30},
		"zero days":          {Credits: 10, ValidityDays: 0},
		"too many days":      {Credits: 10, ValidityDays: MaxDays + 1},
		"long message":       {Credits: 10, ValidityDays: 30, Message: strings.Repeat("가", MaxMessageRunes+1)},
		"sale without paid":  {Credits: 10, ValidityDays: 30, Sale: &Sale{AmountKRW: 0, Payer: "홍길동"}},
		"sale over the cap":  {Credits: 10, ValidityDays: 30, Sale: &Sale{AmountKRW: MaxSaleKRW + 1, Payer: "홍길동"}},
		"sale without payer": {Credits: 10, ValidityDays: 30, Sale: &Sale{AmountKRW: 14900, Payer: "   "}},
		"long payer":         {Credits: 10, ValidityDays: 30, Sale: &Sale{AmountKRW: 14900, Payer: strings.Repeat("가", MaxPayerRunes+1)}},
	} {
		svc, store, _ := newTestService(t)
		if _, err := svc.Issue(ctx, "root", request); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", name, err)
		}
		if len(store.vouchers) != 0 {
			t.Errorf("%s: stored %d vouchers", name, len(store.vouchers))
		}
	}

	svc, store, _ := newTestService(t)
	edge := Issue{
		Credits: MaxCredits, ValidityDays: MaxDays,
		Message: " " + strings.Repeat("가", MaxMessageRunes) + " ",
		Sale:    &Sale{AmountKRW: MaxSaleKRW, Payer: " " + strings.Repeat("가", MaxPayerRunes) + " "},
	}
	if _, err := svc.Issue(ctx, "root", edge); err != nil {
		t.Fatalf("edge issue: %v", err)
	}
	if _, err := svc.Issue(ctx, "root", valid); err != nil {
		t.Fatalf("plain issue: %v", err)
	}
	if got := store.vouchers[0]; got.Message != strings.Repeat("가", MaxMessageRunes) || got.Sale.Payer != strings.Repeat("가", MaxPayerRunes) {
		t.Fatalf("free text was not trimmed: %+v", got)
	}
}

func TestIssueStoresALinkThatExpiresInNinetyDays(t *testing.T) {
	svc, store, _ := newTestService(t)
	issued, err := svc.Issue(context.Background(), "root", Issue{
		Credits: 330, ValidityDays: 30, Sale: &Sale{AmountKRW: 4900, Payer: "김민수"}, Message: "파일럿 감사합니다",
	})
	if err != nil {
		t.Fatal(err)
	}
	if issued.Token == "" || issued.IssuedBy != "root" || !issued.LinkExpiresAt.Equal(issuedAt.Add(90*24*time.Hour)) {
		t.Fatalf("issued = %+v", issued)
	}
	if len(store.vouchers) != 1 || store.vouchers[0].Sale.AmountKRW != 4900 {
		t.Fatalf("stored = %+v", store.vouchers)
	}
}

func TestStatePrecedence(t *testing.T) {
	past := issuedAt.Add(-time.Hour)
	for _, tc := range []struct {
		name string
		v    Voucher
		want State
	}{
		{"fresh", Voucher{LinkExpiresAt: issuedAt.Add(time.Hour)}, StateRedeemable},
		{"link lapsed", Voucher{LinkExpiresAt: issuedAt}, StateExpired},
		{"redeemed after the link lapsed", Voucher{LinkExpiresAt: past, RedeemedAt: &past}, StateRedeemed},
		{"revoked after redemption", Voucher{LinkExpiresAt: past, RedeemedAt: &past, RevokedAt: &past}, StateRevoked},
	} {
		if got := tc.v.State(issuedAt); got != tc.want {
			t.Errorf("%s: state = %s, want %s", tc.name, got, tc.want)
		}
	}
}

func TestViewRevealsOnlyWhatTheGiftPageShows(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	issued, err := svc.Issue(ctx, "root", Issue{Credits: 1150, ValidityDays: 30, Sale: &Sale{AmountKRW: 14900, Payer: "김민수"}, Message: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	view, err := svc.View(ctx, issued.Token)
	if err != nil {
		t.Fatal(err)
	}
	want := PublicView{Credits: 1150, ValidityDays: 30, Message: "hi", State: StateRedeemable, LinkExpiresAt: issued.LinkExpiresAt}
	if view != want {
		t.Fatalf("view = %+v, want %+v", view, want)
	}
	for _, token := range []string{"", "no-such-token"} {
		if _, err := svc.View(ctx, token); !errors.Is(err, ErrNotFound) {
			t.Errorf("view %q = %v, want ErrNotFound", token, err)
		}
	}
}

func TestRedeemOpensOneLotExpiringTheValidityAfterRedemption(t *testing.T) {
	svc, store, now := newTestService(t)
	ctx := context.Background()
	issued, err := svc.Issue(ctx, "root", Issue{Credits: 1150, ValidityDays: 30})
	if err != nil {
		t.Fatal(err)
	}
	*now = issuedAt.Add(10 * 24 * time.Hour)

	redemption, err := svc.Redeem(ctx, "alice", issued.Token)
	if err != nil {
		t.Fatal(err)
	}
	wantExpiry := now.Add(30 * 24 * time.Hour)
	if redemption.Credits != 1150 || !redemption.CreditsExpireAt.Equal(wantExpiry) {
		t.Fatalf("redemption = %+v, want 1150 until %v", redemption, wantExpiry)
	}
	lot := store.credits.lots["voucher:1"]
	if lot == nil || lot.user != "alice" || lot.remaining != 1150 || !lot.expires.Equal(wantExpiry) {
		t.Fatalf("lot = %+v", lot)
	}
	if got := store.vouchers[0]; got.RedeemedBy != "alice" || got.LotID != "voucher:1" {
		t.Fatalf("voucher = %+v", got)
	}

	if _, err := svc.Redeem(ctx, "bob", issued.Token); !errors.Is(err, ErrRedeemed) {
		t.Fatalf("second redeem = %v, want ErrRedeemed", err)
	}
	if len(store.credits.lots) != 1 {
		t.Fatalf("lots = %d, want one", len(store.credits.lots))
	}
}

func TestRedeemRefusesEveryOtherState(t *testing.T) {
	ctx := context.Background()

	svc, _, now := newTestService(t)
	lapsed, _ := svc.Issue(ctx, "root", Issue{Credits: 10, ValidityDays: 30})
	*now = issuedAt.Add(LinkLifetime)
	if _, err := svc.Redeem(ctx, "alice", lapsed.Token); !errors.Is(err, ErrExpired) {
		t.Errorf("lapsed link = %v, want ErrExpired", err)
	}

	svc, store, _ := newTestService(t)
	revoked, _ := svc.Issue(ctx, "root", Issue{Credits: 10, ValidityDays: 30})
	if _, err := svc.Revoke(ctx, revoked.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Redeem(ctx, "alice", revoked.Token); !errors.Is(err, ErrRevoked) {
		t.Errorf("revoked link = %v, want ErrRevoked", err)
	}
	if _, err := svc.Redeem(ctx, "alice", "no-such-token"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown link = %v, want ErrNotFound", err)
	}
	if len(store.credits.lots) != 0 {
		t.Fatalf("a refused redemption opened %d lots", len(store.credits.lots))
	}
}

func TestRevokeInBothStatesAndTwice(t *testing.T) {
	ctx := context.Background()
	svc, store, now := newTestService(t)
	unredeemed, _ := svc.Issue(ctx, "root", Issue{Credits: 10, ValidityDays: 30})
	redeemed, _ := svc.Issue(ctx, "root", Issue{Credits: 1150, ValidityDays: 30})
	if _, err := svc.Redeem(ctx, "alice", redeemed.Token); err != nil {
		t.Fatal(err)
	}
	store.credits.lots["voucher:1"].remaining = 900

	listed, err := svc.Revoke(ctx, unredeemed.ID)
	if err != nil {
		t.Fatal(err)
	}
	if listed.State != StateRevoked || listed.RevokedAt == nil {
		t.Fatalf("unredeemed after revoke = %+v", listed)
	}

	*now = issuedAt.Add(time.Hour)
	listed, err = svc.Revoke(ctx, redeemed.ID)
	if err != nil {
		t.Fatal(err)
	}
	if listed.State != StateRevoked || listed.RemainingCredits != 0 || !listed.CreditsExpireAt.Equal(*now) {
		t.Fatalf("redeemed after revoke = %+v", listed)
	}
	if lot := store.credits.lots["voucher:1"]; !lot.expires.Equal(*now) {
		t.Fatalf("lot expiry = %v, want the revocation instant %v", lot.expires, *now)
	}

	firstRevokedAt := *store.vouchers[1].RevokedAt
	*now = issuedAt.Add(2 * time.Hour)
	again, err := svc.Revoke(ctx, redeemed.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !again.RevokedAt.Equal(firstRevokedAt) {
		t.Fatalf("a second revoke moved revoked_at to %v", again.RevokedAt)
	}
	if _, err := svc.Revoke(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("revoke missing = %v, want ErrNotFound", err)
	}
}

func TestListAttachesStateAndRemainingCredits(t *testing.T) {
	ctx := context.Background()
	svc, store, now := newTestService(t)
	first, _ := svc.Issue(ctx, "root", Issue{Credits: 330, ValidityDays: 30})
	second, _ := svc.Issue(ctx, "root", Issue{Credits: 1150, ValidityDays: 1})
	if _, err := svc.Redeem(ctx, "alice", second.Token); err != nil {
		t.Fatal(err)
	}
	store.credits.lots["voucher:1"].remaining = 700

	listed, err := svc.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 2 || listed[0].ID != second.ID || listed[1].ID != first.ID {
		t.Fatalf("listed order = %+v, want newest first", listed)
	}
	if listed[0].State != StateRedeemed || listed[0].RemainingCredits != 700 {
		t.Fatalf("redeemed = %+v", listed[0])
	}
	if listed[1].State != StateRedeemable || listed[1].RemainingCredits != 0 {
		t.Fatalf("unredeemed = %+v", listed[1])
	}

	*now = issuedAt.Add(48 * time.Hour)
	lapsed, err := svc.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if lapsed[0].RemainingCredits != 0 {
		t.Fatalf("lapsed credits still show %d", lapsed[0].RemainingCredits)
	}
}

func TestPresetsFollowThePaidRungs(t *testing.T) {
	presets := Presets()
	if len(presets) != 3 {
		t.Fatalf("presets = %+v", presets)
	}
	for i, rung := range []plan.Plan{plan.Basic, plan.Pro, plan.Max} {
		if presets[i].Plan != rung || presets[i].Credits != plan.MonthlyCredits(rung) || presets[i].Days != PresetDays {
			t.Errorf("preset %d = %+v", i, presets[i])
		}
	}
}
