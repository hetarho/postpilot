package voucher

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"
)

// tokenBytes is the gift link's entropy: the same 256 bits a session or an auth link
// carries, far past enumeration, which is why the public read needs no throttle.
const tokenBytes = 32

// Service issues, reads, redeems and revokes vouchers.
type Service struct {
	store   Store
	credits Credits

	// now, newToken and newID are seams for tests in this package, not configuration.
	now      func() time.Time
	newToken func() (string, error)
	newID    func() string
}

// NewService wires the context. credits is the ledger outside any transaction, for the
// operator's list; a write reaches the ledger through the store's transaction instead.
func NewService(store Store, credits Credits) *Service {
	if store == nil || credits == nil {
		panic("voucher: store and credits are required")
	}
	return &Service{store: store, credits: credits, now: time.Now, newToken: newToken, newID: newID}
}

// Issue stores a new voucher whose link expires LinkLifetime after now, and returns it with
// its token (GIFT-3, GIFT-4, GIFT-6, GIFT-7).
func (s *Service) Issue(ctx context.Context, issuer string, request Issue) (Voucher, error) {
	normalized, err := request.normalized()
	if err != nil {
		return Voucher{}, err
	}
	token, err := s.newToken()
	if err != nil {
		return Voucher{}, err
	}
	now := s.now().UTC()
	voucher := Voucher{
		ID: s.newID(), Token: token, Credits: normalized.Credits, ValidityDays: normalized.ValidityDays,
		Sale: normalized.Sale, Message: normalized.Message, IssuedBy: issuer,
		IssuedAt: now, LinkExpiresAt: now.Add(LinkLifetime),
	}
	if err := s.store.InsertVoucher(ctx, voucher); err != nil {
		return Voucher{}, err
	}
	return voucher, nil
}

// View is the public gift page's read (GIFT-8). An empty or unknown token is ErrNotFound.
func (s *Service) View(ctx context.Context, token string) (PublicView, error) {
	if token == "" {
		return PublicView{}, ErrNotFound
	}
	voucher, found, err := s.store.VoucherByToken(ctx, token)
	if err != nil {
		return PublicView{}, err
	}
	if !found {
		return PublicView{}, ErrNotFound
	}
	return PublicView{
		Credits: voucher.Credits, ValidityDays: voucher.ValidityDays, Message: voucher.Message,
		State: voucher.State(s.now()), LinkExpiresAt: voucher.LinkExpiresAt,
	}, nil
}

// Redeem opens the voucher's credits for the account, once (GIFT-9). The lot and the
// redemption row commit together; of two racing redemptions exactly one wins, and the
// other meets ErrRedeemed with its lot rolled back.
func (s *Service) Redeem(ctx context.Context, userID, token string) (Redemption, error) {
	if token == "" {
		return Redemption{}, ErrNotFound
	}
	var redemption Redemption
	err := s.store.InWriteTx(ctx, func(tx Store, credits Credits) error {
		voucher, found, err := tx.VoucherByToken(ctx, token)
		if err != nil {
			return err
		}
		if !found {
			return ErrNotFound
		}
		now := s.now().UTC()
		if refusal := voucher.State(now).refusal(); refusal != nil {
			return refusal
		}
		expires := now.Add(time.Duration(voucher.ValidityDays) * 24 * time.Hour)
		lotID, err := credits.OpenVoucherLot(ctx, userID, voucher.Credits, expires)
		if err != nil {
			return err
		}
		marked, err := tx.MarkRedeemed(ctx, voucher.ID, userID, lotID, now)
		if err != nil {
			return err
		}
		if !marked {
			return ErrRedeemed
		}
		redemption = Redemption{Credits: voucher.Credits, CreditsExpireAt: expires}
		return nil
	})
	if err != nil {
		return Redemption{}, err
	}
	return redemption, nil
}

// List is the operator's voucher list (GIFT-14), newest first.
func (s *Service) List(ctx context.Context) ([]Listed, error) {
	vouchers, err := s.store.Vouchers(ctx)
	if err != nil {
		return nil, err
	}
	return s.listed(ctx, vouchers)
}

// Revoke stops an unredeemed link, or voids a redeemed voucher's unspent credits while the
// spent ones stay spent (GIFT-10). Revoking twice changes nothing the second time.
func (s *Service) Revoke(ctx context.Context, id string) (Listed, error) {
	var revoked Voucher
	err := s.store.InWriteTx(ctx, func(tx Store, credits Credits) error {
		voucher, found, err := tx.VoucherByID(ctx, id)
		if err != nil {
			return err
		}
		if !found {
			return ErrNotFound
		}
		if voucher.RevokedAt != nil {
			revoked = voucher
			return nil
		}
		now := s.now().UTC()
		if _, err := tx.MarkRevoked(ctx, voucher.ID, now); err != nil {
			return err
		}
		if voucher.RedeemedAt != nil && voucher.LotID != "" {
			if err := credits.ExpireVoucherLot(ctx, voucher.LotID, now); err != nil {
				return err
			}
		}
		voucher.RevokedAt = &now
		revoked = voucher
		return nil
	})
	if err != nil {
		return Listed{}, err
	}
	listed, err := s.listed(ctx, []Voucher{revoked})
	if err != nil {
		return Listed{}, err
	}
	return listed[0], nil
}

// listed attaches each voucher's state and, for a redeemed one, where its lot stands.
func (s *Service) listed(ctx context.Context, vouchers []Voucher) ([]Listed, error) {
	now := s.now()
	var lotIDs []string
	for _, voucher := range vouchers {
		if voucher.LotID != "" {
			lotIDs = append(lotIDs, voucher.LotID)
		}
	}
	standings, err := s.credits.VoucherLotStandings(ctx, lotIDs, now)
	if err != nil {
		return nil, err
	}
	out := make([]Listed, 0, len(vouchers))
	for _, voucher := range vouchers {
		item := Listed{Voucher: voucher, State: voucher.State(now)}
		if standing, ok := standings[voucher.LotID]; ok && voucher.LotID != "" {
			item.CreditsExpireAt = standing.ExpiresAt
			if item.State != StateRevoked {
				item.RemainingCredits = standing.Remaining
			}
		}
		out = append(out, item)
	}
	return out, nil
}

func newToken() (string, error) {
	buffer := make([]byte, tokenBytes)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate voucher token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func newID() string {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		panic("voucher: cannot read random bytes for an id: " + err.Error())
	}
	return hex.EncodeToString(buffer)
}
