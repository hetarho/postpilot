// Package googleauth exchanges Google authorization codes for verified identity claims.
package googleauth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/postpilot/backend/internal/auth"
)

const googleTokenEndpoint = "https://oauth2.googleapis.com/token"

type Identity struct {
	clientID, clientSecret string
	client                 *http.Client
	tokenEndpoint          string
	allowedOrigin          string
	now                    func() time.Time
}

func New(clientID, clientSecret string, client *http.Client) *Identity {
	if client == nil {
		client = http.DefaultClient
	}
	return &Identity{
		clientID: clientID, clientSecret: clientSecret, client: client,
		tokenEndpoint: googleTokenEndpoint, now: time.Now,
	}
}

// SetAllowedOrigin restricts client-supplied redirect URIs before credentials are sent.
func (i *Identity) SetAllowedOrigin(origin string) { i.allowedOrigin = strings.TrimRight(origin, "/") }

func (i *Identity) Exchange(ctx context.Context, code, verifier, redirectURI string) (auth.GoogleClaims, error) {
	if err := i.validateRedirectOrigin(redirectURI); err != nil {
		return auth.GoogleClaims{}, err
	}
	values := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"code_verifier": {verifier},
		"redirect_uri":  {redirectURI},
		"client_id":     {i.clientID},
		"client_secret": {i.clientSecret},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, i.tokenEndpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return auth.GoogleClaims{}, fmt.Errorf("build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := i.client.Do(req)
	if err != nil {
		return auth.GoogleClaims{}, fmt.Errorf("exchange authorization code: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4<<10))
		return auth.GoogleClaims{}, fmt.Errorf("token endpoint returned %s", res.Status)
	}
	var token struct {
		IDToken string `json:"id_token"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&token); err != nil {
		return auth.GoogleClaims{}, fmt.Errorf("decode token response: %w", err)
	}
	if token.IDToken == "" {
		return auth.GoogleClaims{}, errors.New("token response has no id_token")
	}
	return i.decodeClaims(token.IDToken)
}

func (i *Identity) validateRedirectOrigin(raw string) error {
	redirect, err := url.Parse(raw)
	if err != nil || redirect.Scheme == "" || redirect.Host == "" || redirect.User != nil {
		return errors.New("redirect_uri is not an absolute browser URL")
	}
	allowed, err := url.Parse(i.allowedOrigin)
	if err != nil || allowed.Scheme == "" || allowed.Host == "" || allowed.User != nil {
		return errors.New("google identity has no valid allowed origin")
	}
	if !strings.EqualFold(redirect.Scheme, allowed.Scheme) || !strings.EqualFold(redirect.Host, allowed.Host) {
		return errors.New("redirect_uri origin is not allowed")
	}
	return nil
}

func (i *Identity) decodeClaims(raw string) (auth.GoogleClaims, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return auth.GoogleClaims{}, errors.New("id_token is not a JWT")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return auth.GoogleClaims{}, fmt.Errorf("decode id_token payload: %w", err)
	}
	var claims struct {
		Issuer        string          `json:"iss"`
		Audience      json.RawMessage `json:"aud"`
		ExpiresAt     int64           `json:"exp"`
		Subject       string          `json:"sub"`
		Email         string          `json:"email"`
		EmailVerified json.RawMessage `json:"email_verified"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return auth.GoogleClaims{}, fmt.Errorf("decode id_token claims: %w", err)
	}
	if claims.Issuer != "accounts.google.com" && claims.Issuer != "https://accounts.google.com" {
		return auth.GoogleClaims{}, errors.New("id_token issuer is not Google")
	}
	if !singleAudience(claims.Audience, i.clientID) {
		return auth.GoogleClaims{}, errors.New("id_token audience does not match client")
	}
	if claims.ExpiresAt <= i.now().Unix() {
		return auth.GoogleClaims{}, errors.New("id_token is expired")
	}
	if claims.Subject == "" {
		return auth.GoogleClaims{}, errors.New("id_token has no subject")
	}
	return auth.GoogleClaims{
		Subject: claims.Subject, Email: claims.Email, EmailVerified: verified(claims.EmailVerified),
	}, nil
}

func singleAudience(raw json.RawMessage, want string) bool {
	var value string
	if json.Unmarshal(raw, &value) == nil {
		return value == want
	}
	var values []string
	return json.Unmarshal(raw, &values) == nil && len(values) == 1 && values[0] == want
}

func verified(raw json.RawMessage) bool {
	var value bool
	if json.Unmarshal(raw, &value) == nil {
		return value
	}
	var text string
	return json.Unmarshal(raw, &text) == nil && text == "true"
}
