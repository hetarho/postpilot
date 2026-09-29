package billing

import (
	"context"
	"errors"
	"time"

	"github.com/postpilot/backend/internal/plan"
)

var (
	ErrRefundNotFound         = errors.New("refund request or payment not found")
	ErrRefundConflict         = errors.New("refund request is already open or resolved")
	ErrRefundAmount           = errors.New("refund amount is outside the remaining payment")
	ErrRefundActiveUse        = errors.New("funded benefit has an active reservation")
	ErrRefundDependentPayment = errors.New("a later upgrade payment must be reviewed first")
	ErrRefundProviderPending  = errors.New("refund provider outcome is unresolved")
	ErrInvalidRefundRequest   = errors.New("refund request is invalid")
)

type RefundPayment struct {
	OrderID, UserID, Kind, PaymentKey  string
	PackLotID, CoverageID              string
	Tier, PriorTier                    plan.Plan
	Term                               Term
	KRW                                int
	ChargedAt, EffectiveAt, FundingEnd time.Time
}

type RefundEvidence struct {
	PaidModelJobs          int `json:"paid_model_jobs"`
	CreditsUsed            int `json:"credits_used"`
	CreditsReserved        int `json:"credits_reserved"`
	ServerExportsUsed      int `json:"server_exports_used"`
	ServerExportsReserved  int `json:"server_exports_reserved"`
	FundedCreditsRemaining int `json:"funded_credits_remaining"`
	FundedExportsRemaining int `json:"funded_exports_remaining"`
}

func (e RefundEvidence) Unused() bool {
	return e.PaidModelJobs == 0 && e.CreditsUsed == 0 && e.CreditsReserved == 0 &&
		e.ServerExportsUsed == 0 && e.ServerExportsReserved == 0
}

func (e RefundEvidence) Active() bool {
	return e.CreditsReserved > 0 || e.ServerExportsReserved > 0
}

type RefundRequest struct {
	ID, UserID, OrderID, Reason, Status                    string
	RequestedAt                                            time.Time
	ReviewedBy                                             string
	ReviewedAt                                             *time.Time
	ReviewedAmountKRW                                      int
	PriorRefundedKRW                                       int
	ProviderBalanceBeforeKRW                               int
	ConfirmedAmountKRW                                     int
	ConfirmedAt                                            *time.Time
	ProviderStatus, ProviderTransactionKey, IdempotencyKey string
	DispositionJSON                                        string
	Payment                                                RefundPayment
	Evidence                                               RefundEvidence
}

type RefundDecision struct {
	ID, RequestID, ReviewerID, Outcome string
	AmountKRW                          int
	EvidenceJSON, DispositionJSON      string
	CreatedAt                          time.Time
}

// RefundBenefits is the consumer's view of payment-funded credit and export
// rights. Its implementation joins the same billing writer transaction.
type RefundBenefits interface {
	Inspect(ctx context.Context, payment RefundPayment, at time.Time) (RefundEvidence, error)
	Guard(ctx context.Context, request RefundRequest, payment RefundPayment, at time.Time) error
	Release(ctx context.Context, request RefundRequest, payment RefundPayment) error
	Confirm(ctx context.Context, request RefundRequest, payment RefundPayment, at time.Time) error
}

type RefundStore interface {
	RefundPayment(ctx context.Context, userID, orderID string) (RefundPayment, bool, error)
	RefundRequest(ctx context.Context, id string) (RefundRequest, bool, error)
	OpenRefundForOrder(ctx context.Context, orderID string) (bool, error)
	Refunds(ctx context.Context, userID string) ([]RefundRequest, error)
	ProcessingRefundIDs(ctx context.Context, since time.Time, limit int) ([]string, error)
	ReviewedEvidence(ctx context.Context, requestID string) (RefundEvidence, bool, error)
	InsertRefundRequest(ctx context.Context, request RefundRequest) error
	RecordRefundDecision(ctx context.Context, request RefundRequest, decision RefundDecision) error
	RecordRefundProviderAttempt(ctx context.Context, requestID, transactionKey string) error
	RecordRefundOutcome(ctx context.Context, request RefundRequest, payment Payment, at time.Time) error
	RejectRefund(ctx context.Context, requestID string, at time.Time) error
	FailRefund(ctx context.Context, requestID, providerStatus string, at time.Time) error
	ConfirmedRefundTotal(ctx context.Context, orderID string) (int, error)
	HasUnresolvedDependentUpgrade(ctx context.Context, payment RefundPayment) (bool, error)
	RefundBenefits() RefundBenefits
}
