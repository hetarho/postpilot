package config

import "testing"

// TestValidateOrigin guards the one config mistake that cannot be caught later: the
// credentialed CORS layer must never be handed an origin the browser will refuse.
func TestValidateOrigin(t *testing.T) {
	valid := []string{
		"http://localhost:2564",
		"https://postpilot.haeram.me",
		"https://postpilot.example.com:8443",
	}
	for _, origin := range valid {
		if err := validateOrigin(origin); err != nil {
			t.Errorf("validateOrigin(%q) = %v, want nil", origin, err)
		}
	}

	invalid := map[string]string{
		"empty":    "",
		"wildcard": "*",
		// rs/cors reads an embedded * as a pattern and reflects any matching origin,
		// so a sibling project on the same registered domain could make authenticated
		// calls with the user's session.
		"wildcard subdomain": "https://*.example.com",
		"wildcard scheme":    "*://postpilot.example.com",
		"wildcard suffix":    "https://postpilot.example.*",
		"no scheme":          "postpilot.example.com",
		"scheme only":        "https://",
		"with path":          "https://postpilot.example.com/app",
		"with query":         "https://postpilot.example.com?x=1",
		"with fragment":      "https://postpilot.example.com#top",
	}
	for name, origin := range invalid {
		t.Run(name, func(t *testing.T) {
			if err := validateOrigin(origin); err == nil {
				t.Errorf("validateOrigin(%q) = nil, want an error", origin)
			}
		})
	}
}

func TestLoadRejectsWildcardOrigin(t *testing.T) {
	for _, origin := range []string{"*", "https://*.example.com"} {
		t.Run(origin, func(t *testing.T) {
			t.Setenv("CORS_ORIGIN", origin)
			if _, err := Load(); err == nil {
				t.Fatalf("Load accepted CORS_ORIGIN=%q", origin)
			}
		})
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("CORS_ORIGIN", "")
	t.Setenv("DB_PATH", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DBPath != "data/postpilot.db" {
		t.Errorf("DBPath = %q, want data/postpilot.db", cfg.DBPath)
	}
	// 30 days, on both ends: this value becomes sessions.expires_at AND the cookie's
	// Max-Age, so a change here silently changes the login lifetime (PRD F-1).
	if got := cfg.SessionTTL.Hours(); got != 720 {
		t.Errorf("SessionTTL = %v hours, want 720", got)
	}
}
