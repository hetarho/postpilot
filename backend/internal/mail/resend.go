// Package mail contains transactional-mail delivery adapters. Domain policy and final
// message text stay in auth; this package knows only how to deliver auth.Mail values.
package mail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/postpilot/backend/internal/auth"
)

const resendEndpoint = "https://api.resend.com/emails"

// Resend sends plain-text transactional mail through Resend's HTTPS API.
type Resend struct {
	apiKey   string
	from     string
	client   *http.Client
	endpoint string
}

// NewResend builds the adapter. Config validation owns required-value checks at boot.
func NewResend(apiKey, from string, client *http.Client) *Resend {
	if client == nil {
		client = http.DefaultClient
	}
	return &Resend{apiKey: apiKey, from: from, client: client, endpoint: resendEndpoint}
}

func (r *Resend) Send(ctx context.Context, mail auth.Mail) error {
	body, err := json.Marshal(struct {
		From    string   `json:"from"`
		To      []string `json:"to"`
		Subject string   `json:"subject"`
		Text    string   `json:"text"`
	}{From: r.from, To: []string{mail.To}, Subject: mail.Subject, Text: mail.Text})
	if err != nil {
		return fmt.Errorf("encode Resend email: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build Resend request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "postpilot/0.0.1")

	response, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("send Resend email: %w", err)
	}
	defer response.Body.Close()

	responseBody, readErr := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	if readErr != nil {
		return fmt.Errorf("read Resend response: %w", readErr)
	}
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return nil
	}
	if response.StatusCode >= 400 && response.StatusCode < 500 && recipientValidationError(responseBody) {
		return auth.ErrRecipientRejected
	}
	return fmt.Errorf("Resend send email returned %s: %s", response.Status, strings.TrimSpace(string(responseBody)))
}

func recipientValidationError(body []byte) bool {
	var failure struct {
		Name    string `json:"name"`
		Message string `json:"message"`
		Field   string `json:"field"`
	}
	if json.Unmarshal(body, &failure) != nil || failure.Name != "validation_error" {
		return false
	}
	if strings.EqualFold(failure.Field, "to") {
		return true
	}
	message := strings.ToLower(failure.Message)
	return strings.Contains(message, "recipient") || strings.Contains(message, "`to`") ||
		strings.Contains(message, "'to'") || strings.Contains(message, `"to"`)
}

var _ auth.Mailer = (*Resend)(nil)
