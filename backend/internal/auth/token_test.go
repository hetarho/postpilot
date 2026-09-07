package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"testing"
	"time"
)

func TestNewLinkTokenStoresOnlyTheSHA256Hash(t *testing.T) {
	raw, hash, err := NewLinkToken()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(decoded) != 32 {
		t.Fatalf("raw token decodes to %d bytes, err=%v", len(decoded), err)
	}
	want := sha256.Sum256([]byte(raw))
	if hash != hex.EncodeToString(want[:]) {
		t.Fatalf("stored hash = %q, want SHA-256 hex", hash)
	}
	if raw == hash {
		t.Fatal("raw link token was returned as its stored value")
	}
	if VerifyLinkTTL != 24*time.Hour || ResetLinkTTL != time.Hour {
		t.Fatalf("link TTLs = %v / %v", VerifyLinkTTL, ResetLinkTTL)
	}
}
