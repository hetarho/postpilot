// Package fxrate adapts the Korea Eximbank daily reference-rate API to billing.Rates.
package fxrate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const eximbankEndpoint = "https://www.koreaexim.go.kr/site/program/financial/exchangeJSON"

type Eximbank struct {
	authKey  string
	client   *http.Client
	endpoint string
}

func NewEximbank(authKey string, client *http.Client) *Eximbank {
	if client == nil {
		client = http.DefaultClient
	}
	return &Eximbank{authKey: authKey, client: client, endpoint: eximbankEndpoint}
}

func (e *Eximbank) KRWPerUSD(ctx context.Context, date time.Time) (int64, bool, error) {
	endpoint, err := url.Parse(e.endpoint)
	if err != nil {
		return 0, false, fmt.Errorf("parse Eximbank endpoint: %w", err)
	}
	query := endpoint.Query()
	query.Set("authkey", e.authKey)
	query.Set("searchdate", date.Format("20060102"))
	query.Set("data", "AP01")
	endpoint.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return 0, false, err
	}
	response, err := e.client.Do(req)
	if err != nil {
		return 0, false, fmt.Errorf("request Eximbank rate: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4<<10))
		return 0, false, fmt.Errorf("Eximbank rate status %d", response.StatusCode)
	}
	var rows []struct {
		Result   int    `json:"result"`
		Currency string `json:"cur_unit"`
		BaseRate string `json:"deal_bas_r"`
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1<<20))
	if err := decoder.Decode(&rows); err != nil {
		return 0, false, fmt.Errorf("decode Eximbank rate: %w", err)
	}
	if len(rows) == 0 {
		return 0, false, nil
	}
	for _, row := range rows {
		if row.Result != 1 {
			return 0, false, fmt.Errorf("Eximbank result %d", row.Result)
		}
		if row.Currency == "USD" {
			rate, err := decimalE4(row.BaseRate)
			if err != nil {
				return 0, false, fmt.Errorf("parse USD base rate: %w", err)
			}
			return rate, true, nil
		}
	}
	return 0, false, errors.New("Eximbank response has no USD row")
}

func decimalE4(raw string) (int64, error) {
	value := strings.ReplaceAll(strings.TrimSpace(raw), ",", "")
	parts := strings.Split(value, ".")
	if len(parts) > 2 || len(parts) == 0 || parts[0] == "" {
		return 0, fmt.Errorf("invalid decimal %q", raw)
	}
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || whole <= 0 {
		return 0, fmt.Errorf("invalid decimal %q", raw)
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	if len(fraction) > 4 {
		return 0, fmt.Errorf("too many decimal places in %q", raw)
	}
	for len(fraction) < 4 {
		fraction += "0"
	}
	frac := int64(0)
	if fraction != "" {
		frac, err = strconv.ParseInt(fraction, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid decimal %q", raw)
		}
	}
	return whole*10_000 + frac, nil
}
