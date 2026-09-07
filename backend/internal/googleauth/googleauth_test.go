package googleauth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestExchangePostsCodeAndPKCEAndReturnsClaims(t *testing.T) {
	now := time.Date(2026, 9, 8, 9, 0, 0, 0, time.UTC)
	var got map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Errorf("request = %s %q", r.Method, r.Header.Get("Content-Type"))
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		got = map[string]string{}
		for _, key := range []string{"grant_type", "code", "code_verifier", "redirect_uri", "client_id", "client_secret"} {
			got[key] = r.Form.Get(key)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"id_token": jwt(map[string]any{
			"iss": "https://accounts.google.com", "aud": "client-id", "exp": now.Add(time.Minute).Unix(),
			"sub": "subject-1", "email": "person@example.com", "email_verified": true,
		})})
	}))
	defer server.Close()

	identity := New("client-id", "client-secret", server.Client())
	identity.tokenEndpoint = server.URL
	identity.SetAllowedOrigin("https://postpilot.example.com")
	identity.now = func() time.Time { return now }
	claims, err := identity.Exchange(context.Background(), "code-1", "verifier-1", "https://postpilot.example.com/login/google/callback")
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	want := map[string]string{
		"grant_type": "authorization_code", "code": "code-1", "code_verifier": "verifier-1",
		"redirect_uri": "https://postpilot.example.com/login/google/callback",
		"client_id":    "client-id", "client_secret": "client-secret",
	}
	for key, value := range want {
		if got[key] != value {
			t.Errorf("form[%s] = %q, want %q", key, got[key], value)
		}
	}
	if claims.Subject != "subject-1" || claims.Email != "person@example.com" || !claims.EmailVerified {
		t.Fatalf("claims = %+v", claims)
	}
}

func TestExchangeRefusesInvalidTokenResponses(t *testing.T) {
	now := time.Date(2026, 9, 8, 9, 0, 0, 0, time.UTC)
	tests := map[string]struct {
		status int
		body   any
	}{
		"non-2xx":        {status: http.StatusBadRequest, body: map[string]string{"error": "invalid_grant"}},
		"missing token":  {status: http.StatusOK, body: map[string]string{"access_token": "secret"}},
		"wrong audience": {status: http.StatusOK, body: map[string]string{"id_token": jwt(validClaims(now.Add(time.Minute), "other-client"))}},
		"expired":        {status: http.StatusOK, body: map[string]string{"id_token": jwt(validClaims(now.Add(-2*time.Minute), "client-id"))}},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.status)
				_ = json.NewEncoder(w).Encode(test.body)
			}))
			defer server.Close()
			identity := New("client-id", "client-secret", server.Client())
			identity.tokenEndpoint = server.URL
			identity.SetAllowedOrigin("https://postpilot.example.com")
			identity.now = func() time.Time { return now }
			if _, err := identity.Exchange(context.Background(), "code", "verifier", "https://postpilot.example.com/login/google/callback"); err == nil {
				t.Fatal("Exchange returned nil error")
			}
		})
	}
}

func TestExchangeRefusesForeignRedirectBeforeGoogle(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	defer server.Close()
	identity := New("client-id", "client-secret", server.Client())
	identity.tokenEndpoint = server.URL
	identity.SetAllowedOrigin("https://postpilot.example.com")
	if _, err := identity.Exchange(context.Background(), "code", "verifier", "https://evil.example/login/google/callback"); err == nil {
		t.Fatal("Exchange accepted a foreign redirect origin")
	}
	if called {
		t.Fatal("token endpoint was called before redirect origin refusal")
	}
}

func validClaims(exp time.Time, audience string) map[string]any {
	return map[string]any{
		"iss": "accounts.google.com", "aud": audience, "exp": exp.Unix(), "sub": "subject",
		"email": "person@example.com", "email_verified": "true",
	}
}

func jwt(claims map[string]any) string {
	header, _ := json.Marshal(map[string]string{"alg": "RS256"})
	payload, _ := json.Marshal(claims)
	return base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload) + ".signature"
}
