// Package tosspay adapts the Toss Payments Core API to billing.Provider.
package tosspay

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/postpilot/backend/internal/billing"
)

const apiEndpoint = "https://api.tosspayments.com"

// RequestTimeout bounds one Toss Payments call. A billing pass makes its calls one account
// after another, so a hung connection must not hold every later renewal behind it.
const RequestTimeout = 30 * time.Second

type Client struct {
	secretKey string
	client    *http.Client
	endpoint  string
}

var _ billing.Provider = (*Client)(nil)

func New(secretKey string, client *http.Client) *Client {
	if client == nil {
		client = http.DefaultClient
	}
	return &Client{secretKey: secretKey, client: client, endpoint: apiEndpoint}
}

func (c *Client) IssueBillingKey(ctx context.Context, authKey, customerKey string) (billing.BillingKey, error) {
	var response struct {
		BillingKey  string `json:"billingKey"`
		CustomerKey string `json:"customerKey"`
		CardCompany string `json:"cardCompany"`
		Card        struct {
			IssuerCode string `json:"issuerCode"`
			Number     string `json:"number"`
		} `json:"card"`
	}
	err := c.do(ctx, http.MethodPost, "/v1/billing/authorizations/issue", map[string]string{
		"authKey": authKey, "customerKey": customerKey,
	}, &response)
	if err != nil {
		return billing.BillingKey{}, err
	}
	issuer := response.Card.IssuerCode
	if issuer == "" {
		issuer = response.CardCompany
	}
	last := lastFour(response.Card.Number)
	label := strings.TrimSpace(strings.TrimSpace(issuer) + " " + last)
	return billing.BillingKey{Value: response.BillingKey, CustomerKey: response.CustomerKey, CardLabel: label}, nil
}

// Charge approves a billing-key payment. The order id doubles as the Idempotency-Key, so a
// retry of the same order inside Toss's 15-day key window answers with the first response
// instead of capturing again.
func (c *Client) Charge(ctx context.Context, request billing.ChargeRequest) (billing.Payment, error) {
	var response paymentResponse
	path := "/v1/billing/" + url.PathEscape(request.BillingKey)
	_, err := c.doStatusWithKey(ctx, http.MethodPost, path, map[string]any{
		"customerKey": request.CustomerKey, "amount": request.KRW,
		"orderId": request.OrderID, "orderName": request.Name,
	}, &response, request.OrderID)
	return response.domain(), err
}

func (c *Client) PaymentByOrder(ctx context.Context, orderID string) (billing.Payment, bool, error) {
	var response paymentResponse
	status, err := c.doStatus(ctx, http.MethodGet, "/v1/payments/orders/"+url.PathEscape(orderID), nil, &response)
	if status == http.StatusNotFound {
		return billing.Payment{}, false, nil
	}
	if err != nil {
		return billing.Payment{}, false, err
	}
	return response.domain(), true, nil
}

func (c *Client) CancelPayment(ctx context.Context, paymentKey string, amountKRW int, reason, idempotencyKey string) (billing.Payment, error) {
	var response paymentResponse
	_, err := c.doStatusWithKey(ctx, http.MethodPost, "/v1/payments/"+url.PathEscape(paymentKey)+"/cancel",
		map[string]any{"cancelReason": reason, "cancelAmount": amountKRW}, &response, idempotencyKey)
	return response.domain(), err
}

func (c *Client) ParseNotification(raw []byte) (billing.Notification, error) {
	var envelope struct {
		EventType  string `json:"eventType"`
		PaymentKey string `json:"paymentKey"`
		OrderID    string `json:"orderId"`
		Status     string `json:"status"`
		Data       struct {
			PaymentKey string `json:"paymentKey"`
			OrderID    string `json:"orderId"`
			Status     string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return billing.Notification{}, fmt.Errorf("decode Toss notification: %w", err)
	}
	if envelope.PaymentKey == "" {
		envelope.PaymentKey = envelope.Data.PaymentKey
	}
	if envelope.OrderID == "" {
		envelope.OrderID = envelope.Data.OrderID
	}
	if envelope.Status == "" {
		envelope.Status = envelope.Data.Status
	}
	if envelope.EventType == "" || envelope.OrderID == "" {
		return billing.Notification{}, fmt.Errorf("Toss notification is missing eventType or orderId")
	}
	return billing.Notification{EventType: envelope.EventType, PaymentKey: envelope.PaymentKey, OrderID: envelope.OrderID, Status: envelope.Status, Raw: raw}, nil
}

type paymentResponse struct {
	PaymentKey    string `json:"paymentKey"`
	OrderID       string `json:"orderId"`
	Status        string `json:"status"`
	TotalAmount   int    `json:"totalAmount"`
	BalanceAmount int    `json:"balanceAmount"`
	Currency      string `json:"currency"`
	Cancels       []struct {
		TransactionKey string `json:"transactionKey"`
		CancelAmount   int    `json:"cancelAmount"`
		CancelStatus   string `json:"cancelStatus"`
	} `json:"cancels"`
}

func (p paymentResponse) domain() billing.Payment {
	result := billing.Payment{PaymentKey: p.PaymentKey, OrderID: p.OrderID, Status: p.Status,
		AmountKRW: p.TotalAmount, BalanceKRW: p.BalanceAmount, Currency: p.Currency}
	for _, cancel := range p.Cancels {
		result.Cancels = append(result.Cancels, billing.PaymentCancel{
			TransactionKey: cancel.TransactionKey, AmountKRW: cancel.CancelAmount, Status: cancel.CancelStatus})
	}
	return result
}

func (c *Client) do(ctx context.Context, method, path string, body any, target any) error {
	_, err := c.doStatus(ctx, method, path, body, target)
	return err
}

func (c *Client) doStatus(ctx context.Context, method, path string, body any, target any) (int, error) {
	return c.doStatusWithKey(ctx, method, path, body, target, "")
}

func (c *Client) doStatusWithKey(ctx context.Context, method, path string, body any, target any, idempotencyKey string) (int, error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return 0, err
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.endpoint+path, reader)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(c.secretKey+":")))
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	response, err := c.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("request Toss Payments: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var providerErr billing.ProviderError
		if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&providerErr); err != nil {
			return response.StatusCode, fmt.Errorf("Toss Payments status %d", response.StatusCode)
		}
		providerErr.HTTPStatus = response.StatusCode
		return response.StatusCode, &providerErr
	}
	if target == nil {
		_, _ = io.Copy(io.Discard, response.Body)
		return response.StatusCode, nil
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(target); err != nil {
		return response.StatusCode, fmt.Errorf("decode Toss Payments response: %w", err)
	}
	return response.StatusCode, nil
}

func lastFour(number string) string {
	digits := strings.Map(func(r rune) rune {
		if unicode.IsDigit(r) {
			return r
		}
		return -1
	}, number)
	if len(digits) <= 4 {
		return digits
	}
	return digits[len(digits)-4:]
}
