package auth

import (
	"strings"
	"testing"
)

func TestHashPasswordRoundTrip(t *testing.T) {
	const password = "correct horse battery staple"

	encoded, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	if !strings.HasPrefix(encoded, "$argon2id$v=19$m=65536,t=3,p=2$") {
		t.Errorf("hash does not carry the PHC parameters: %q", encoded)
	}
	if strings.Contains(encoded, password) {
		t.Fatalf("hash contains the plaintext: %q", encoded)
	}

	ok, err := VerifyPassword(password, encoded)
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	if !ok {
		t.Error("correct password did not verify")
	}
}

func TestHashPasswordIsSalted(t *testing.T) {
	a, err := HashPassword("same")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	b, err := HashPassword("same")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if a == b {
		t.Error("two hashes of the same password are identical — the salt is not random")
	}
}

func TestVerifyPasswordWrong(t *testing.T) {
	encoded, err := HashPassword("right")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	for _, password := range []string{"wrong", "", "Right", "right "} {
		ok, err := VerifyPassword(password, encoded)
		if err != nil {
			t.Fatalf("VerifyPassword(%q): %v", password, err)
		}
		if ok {
			t.Errorf("VerifyPassword(%q) accepted a wrong password", password)
		}
	}
}

func TestVerifyPasswordMalformed(t *testing.T) {
	// A row that will not parse must be an error, not a silent false — the two are
	// indistinguishable to a client but very different to an operator reading logs.
	cases := map[string]string{
		"empty":            "",
		"not phc":          "plaintext",
		"wrong algorithm":  "$argon2i$v=19$m=65536,t=3,p=2$c2FsdHNhbHRzYWx0c2E$aGFzaGhhc2hoYXNoaGFzaA",
		"bad params":       "$argon2id$v=19$m=abc,t=3,p=2$c2FsdHNhbHRzYWx0c2E$aGFzaGhhc2hoYXNoaGFzaA",
		"future version":   "$argon2id$v=99$m=65536,t=3,p=2$c2FsdHNhbHRzYWx0c2E$aGFzaGhhc2hoYXNoaGFzaA",
		"bad base64 salt":  "$argon2id$v=19$m=65536,t=3,p=2$!!!$aGFzaGhhc2hoYXNoaGFzaA",
		"missing segments": "$argon2id$v=19$m=65536,t=3,p=2",
		// argon2.IDKey panics on these rather than returning an error, so they must be
		// rejected during decode or a single bad row takes the whole process down.
		"zero rounds":  "$argon2id$v=19$m=65536,t=0,p=2$c2FsdHNhbHRzYWx0c2E$aGFzaGhhc2hoYXNoaGFzaA",
		"zero threads": "$argon2id$v=19$m=65536,t=3,p=0$c2FsdHNhbHRzYWx0c2E$aGFzaGhhc2hoYXNoaGFzaA",
		"zero memory":  "$argon2id$v=19$m=0,t=3,p=2$c2FsdHNhbHRzYWx0c2E$aGFzaGhhc2hoYXNoaGFzaA",
	}

	for name, encoded := range cases {
		t.Run(name, func(t *testing.T) {
			ok, err := VerifyPassword("anything", encoded)
			if err == nil {
				t.Fatal("expected an error for a malformed hash")
			}
			if ok {
				t.Error("a malformed hash must never verify")
			}
		})
	}
}

func TestDummyHashIsUsableAndStable(t *testing.T) {
	// The dummy path is only cost-equal to the real one if the dummy hash actually
	// parses and runs a full derivation — a malformed constant would short-circuit.
	encoded := dummyHash()

	ok, err := VerifyPassword("whatever the caller guessed", encoded)
	if err != nil {
		t.Fatalf("the dummy hash does not verify cleanly: %v", err)
	}
	if ok {
		t.Error("the dummy hash matched a guessed password")
	}

	if encoded != dummyHash() {
		t.Error("dummyHash is not memoized — every unknown-id login would pay a fresh derivation")
	}

	// It must carry the CURRENT cost parameters, or the unknown-id path becomes the
	// cheap one and login timing leaks which ids exist.
	if !strings.HasPrefix(encoded, "$argon2id$v=19$m=65536,t=3,p=2$") {
		t.Errorf("dummy hash does not use the current parameters: %q", encoded)
	}
}
