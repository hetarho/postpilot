package auth

import (
	"net/http"
	"testing"
)

func TestClientIPUsesThePeerUnlessTheHeaderIsTrusted(t *testing.T) {
	header := http.Header{"X-Forwarded-For": {"198.51.100.90, 192.0.2.20"}}
	if got := ClientIP(header, "203.0.113.4:4321", ""); got != "203.0.113.4" {
		t.Fatalf("untrusted header IP = %q, want direct peer", got)
	}
	if got := ClientIP(header, "203.0.113.4:4321", "X-Forwarded-For"); got != "192.0.2.20" {
		t.Fatalf("trusted header IP = %q, want rightmost edge entry", got)
	}
}

func TestClientIPHandlesIPv6AndEmptyForwardedEntries(t *testing.T) {
	if got := ClientIP(nil, "[2001:db8::7]:8443", ""); got != "2001:db8::7" {
		t.Fatalf("IPv6 peer = %q", got)
	}
	header := http.Header{"X-Forwarded-For": {"198.51.100.1, , 2001:db8::8, "}}
	if got := ClientIP(header, "[2001:db8::7]:8443", "X-Forwarded-For"); got != "2001:db8::8" {
		t.Fatalf("forwarded IPv6 = %q", got)
	}
}
