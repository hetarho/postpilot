package config

import "testing"

func TestValidateConnectionTrustAndLease(t *testing.T) {
	valid := Connection{ID: "one", APIURL: "https://api.example.com", AgentID: "agent", KeychainAccount: "key", BrowserBinary: "/browser", ProfileDir: "/profile", LeaseTTLSeconds: 45}
	if err := ValidateConnection(valid); err != nil {
		t.Fatalf("valid connection: %v", err)
	}
	plain := valid
	plain.APIURL = "http://api.example.com"
	if err := ValidateConnection(plain); err == nil {
		t.Fatal("public plain HTTP was accepted")
	}
	loopback := valid
	loopback.APIURL = "http://127.0.0.1:8080"
	if err := ValidateConnection(loopback); err != nil {
		t.Fatalf("loopback development URL: %v", err)
	}
	short := valid
	short.LeaseTTLSeconds = 20
	if err := ValidateConnection(short); err == nil {
		t.Fatal("unsafe heartbeat/lease ratio was accepted")
	}
}

func TestValidateLeaseTTLRejectsServerValueThatCannotSustainHeartbeat(t *testing.T) {
	if err := ValidateLeaseTTL(20); err == nil {
		t.Fatal("lease equal to two heartbeats was accepted")
	}
	if err := ValidateLeaseTTL(21); err != nil {
		t.Fatalf("safe lease was rejected: %v", err)
	}
}
