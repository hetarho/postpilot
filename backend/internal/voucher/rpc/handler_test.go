package rpc_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/auth"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/usage"
	usagestore "github.com/postpilot/backend/internal/usage/store"
	"github.com/postpilot/backend/internal/voucher"
	voucherrpc "github.com/postpilot/backend/internal/voucher/rpc"
	voucherstore "github.com/postpilot/backend/internal/voucher/store"
)

type fixedAnchors struct{}

func (fixedAnchors) AnchorFor(context.Context, string) (time.Time, error) {
	return time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), nil
}

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

func newHandler(t *testing.T) *voucherrpc.Handler {
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
	return voucherrpc.NewHandler(voucher.NewService(store, ledgerCredits{ledger}))
}

func as(user string) context.Context { return auth.WithUser(context.Background(), user) }

func issue(t *testing.T, handler *voucherrpc.Handler, request *postpilotv1.IssueVoucherRequest) *postpilotv1.Voucher {
	t.Helper()
	response, err := handler.IssueVoucher(as("root"), connect.NewRequest(request))
	if err != nil {
		t.Fatal(err)
	}
	return response.Msg.GetVoucher()
}

func detailOf(t *testing.T, err error) *postpilotv1.AppErrorDetail {
	t.Helper()
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) || len(connectErr.Details()) != 1 {
		t.Fatalf("error = %v, details unavailable", err)
	}
	value, valueErr := connectErr.Details()[0].Value()
	if valueErr != nil {
		t.Fatal(valueErr)
	}
	detail, ok := value.(*postpilotv1.AppErrorDetail)
	if !ok {
		t.Fatalf("detail = %T", value)
	}
	return detail
}

func TestGetVoucherAnswersThePublicViewOnly(t *testing.T) {
	handler := newHandler(t)
	issued := issue(t, handler, &postpilotv1.IssueVoucherRequest{
		Credits: 1150, ValidityDays: 30, Message: "파일럿 감사합니다",
		Sale: &postpilotv1.VoucherSale{AmountKrw: 14900, PayerName: "김민수"},
	})
	if issued.GetToken() == "" || issued.GetState() != postpilotv1.VoucherState_VOUCHER_STATE_REDEEMABLE {
		t.Fatalf("issued = %+v", issued)
	}

	response, err := handler.GetVoucher(context.Background(), connect.NewRequest(&postpilotv1.GetVoucherRequest{Token: issued.GetToken()}))
	if err != nil {
		t.Fatal(err)
	}
	got := response.Msg
	if got.GetCredits() != 1150 || got.GetValidityDays() != 30 || got.GetMessage() != "파일럿 감사합니다" ||
		got.GetState() != postpilotv1.VoucherState_VOUCHER_STATE_REDEEMABLE || got.GetLinkExpiresAt() != issued.GetLinkExpiresAt() {
		t.Fatalf("public view = %+v", got)
	}

	secret := "not-a-real-token-but-secret"
	_, err = handler.GetVoucher(context.Background(), connect.NewRequest(&postpilotv1.GetVoucherRequest{Token: secret}))
	if connect.CodeOf(err) != connect.CodeNotFound || detailOf(t, err).GetReason() != postpilotv1.FailureReason_VOUCHER_NOT_FOUND.String() {
		t.Fatalf("unknown token = %v", err)
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("the error names the token: %v", err)
	}
}

func TestRedeemVoucherNeedsASessionAndRedeemsOnce(t *testing.T) {
	handler := newHandler(t)
	issued := issue(t, handler, &postpilotv1.IssueVoucherRequest{Credits: 330, ValidityDays: 30})
	request := func() *connect.Request[postpilotv1.RedeemVoucherRequest] {
		return connect.NewRequest(&postpilotv1.RedeemVoucherRequest{Token: issued.GetToken()})
	}

	if _, err := handler.RedeemVoucher(context.Background(), request()); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("anonymous redeem = %v, want unauthenticated", err)
	}
	response, err := handler.RedeemVoucher(as("alice"), request())
	if err != nil {
		t.Fatal(err)
	}
	if response.Msg.GetCredits() != 330 || response.Msg.GetCreditsExpireAt() == "" {
		t.Fatalf("redeem = %+v", response.Msg)
	}
	_, err = handler.RedeemVoucher(as("bob"), request())
	if connect.CodeOf(err) != connect.CodeFailedPrecondition || detailOf(t, err).GetReason() != postpilotv1.FailureReason_VOUCHER_REDEEMED.String() {
		t.Fatalf("second redeem = %v", err)
	}
}

func TestIssueVoucherRefusesAnInvalidRequest(t *testing.T) {
	handler := newHandler(t)
	_, err := handler.IssueVoucher(as("root"), connect.NewRequest(&postpilotv1.IssueVoucherRequest{
		Credits: 330, ValidityDays: 30, Sale: &postpilotv1.VoucherSale{AmountKrw: 4900},
	}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument || detailOf(t, err).GetReason() != postpilotv1.FailureReason_VOUCHER_INVALID.String() {
		t.Fatalf("invalid issue = %v", err)
	}
}

// GIFT-14: the list carries a token only while the link can still be redeemed.
func TestListVouchersShowsTheTokenOnlyWhileRedeemable(t *testing.T) {
	handler := newHandler(t)
	open := issue(t, handler, &postpilotv1.IssueVoucherRequest{Credits: 330, ValidityDays: 30})
	spent := issue(t, handler, &postpilotv1.IssueVoucherRequest{
		Credits: 1150, ValidityDays: 30, Sale: &postpilotv1.VoucherSale{AmountKrw: 14900, PayerName: "김민수"},
	})
	revoked := issue(t, handler, &postpilotv1.IssueVoucherRequest{Credits: 50, ValidityDays: 7})
	if _, err := handler.RedeemVoucher(as("alice"), connect.NewRequest(&postpilotv1.RedeemVoucherRequest{Token: spent.GetToken()})); err != nil {
		t.Fatal(err)
	}
	revokeResponse, err := handler.RevokeVoucher(as("root"), connect.NewRequest(&postpilotv1.RevokeVoucherRequest{Id: revoked.GetId()}))
	if err != nil {
		t.Fatal(err)
	}
	if revokeResponse.Msg.GetVoucher().GetToken() != "" || revokeResponse.Msg.GetVoucher().GetState() != postpilotv1.VoucherState_VOUCHER_STATE_REVOKED {
		t.Fatalf("revoke response = %+v", revokeResponse.Msg.GetVoucher())
	}

	response, err := handler.ListVouchers(as("root"), connect.NewRequest(&postpilotv1.ListVouchersRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]*postpilotv1.Voucher{}
	for _, v := range response.Msg.GetVouchers() {
		byID[v.GetId()] = v
	}
	if got := byID[open.GetId()]; got.GetToken() != open.GetToken() || got.GetState() != postpilotv1.VoucherState_VOUCHER_STATE_REDEEMABLE {
		t.Errorf("open = %+v", got)
	}
	got := byID[spent.GetId()]
	if got.GetToken() != "" || got.GetState() != postpilotv1.VoucherState_VOUCHER_STATE_REDEEMED ||
		got.GetRedeemedBy() != "alice" || got.GetRedeemedAt() == "" || got.GetRemainingCredits() != 1150 ||
		got.GetCreditsExpireAt() == "" || got.GetSale().GetAmountKrw() != 14900 || got.GetSale().GetPayerName() != "김민수" {
		t.Errorf("spent = %+v", got)
	}
	if got := byID[revoked.GetId()]; got.GetToken() != "" || got.GetState() != postpilotv1.VoucherState_VOUCHER_STATE_REVOKED {
		t.Errorf("revoked = %+v", got)
	}
	if presets := response.Msg.GetPresets(); len(presets) != 3 || presets[1].GetPlan() != postpilotv1.Plan_PLAN_PRO || presets[1].GetValidityDays() != 30 {
		t.Errorf("presets = %+v", presets)
	}

	_, err = handler.RevokeVoucher(as("root"), connect.NewRequest(&postpilotv1.RevokeVoucherRequest{Id: "missing"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("revoke missing = %v", err)
	}
}
