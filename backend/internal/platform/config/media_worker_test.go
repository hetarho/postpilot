package config

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func workerSecret() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
func TestMediaCredentialsRefuseAmbiguityAndWeakSecrets(t *testing.T) {
	secret := workerSecret()
	good := `{"prod-vps-1":"` + secret + `"}`
	if _, err := ParseMediaWorkerCredentials(good); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{"", "null", "[]", "{}", good + "{}", `{"":"` + secret + `"}`, `{"bad id":"` + secret + `"}`, `{"w":"placeholder"}`, `{"w":"` + strings.Repeat("A", 43) + `"}`, `{"w":"` + secret + `","w":"` + workerSecret() + `"}`, `{"a":"` + secret + `","b":"` + secret + `"}`} {
		if _, err := ParseMediaWorkerCredentials(raw); err == nil || strings.Contains(err.Error(), secret) {
			t.Error("accepted ambiguous/weak configuration or leaked its secret")
		}
	}
	rotation, _ := json.Marshal(map[string]string{"prod-vps-1": secret, "prod-vps-2": workerSecret()})
	if got, err := ParseMediaWorkerCredentials(string(rotation)); err != nil || len(got) != 2 {
		t.Fatal("rotation failed", err)
	}
}
func TestMediaConfigKeepsLegacyListenerDisabled(t *testing.T) {
	t.Setenv("MEDIA_INTERNAL_ADDR", "")
	t.Setenv("MEDIA_WORKER_CREDENTIALS", "")
	c := &Config{}
	if err := loadMediaListener(c); err != nil || c.MediaInternalAddr != "" {
		t.Fatal(err)
	}
	t.Setenv("MEDIA_INTERNAL_ADDR", ":9000")
	if err := loadMediaListener(c); err == nil {
		t.Fatal("enabled unauthenticated listener")
	}
	t.Setenv("MEDIA_WORKER_CREDENTIALS", `{"prod-1":"`+workerSecret()+`"}`)
	if err := loadMediaListener(c); err != nil {
		t.Fatal(err)
	}
	for _, addr := range []string{"9000", ":0", ":65536", ":http"} {
		t.Setenv("MEDIA_INTERNAL_ADDR", addr)
		if err := loadMediaListener(c); err == nil {
			t.Fatal("invalid address accepted")
		}
	}
}
func TestMediaWorkerConfigNeedsOnlyItsOwnCredentials(t *testing.T) {
	t.Setenv("MEDIA_API_URL", "http://api:9000")
	t.Setenv("MEDIA_WORKER_ID", "prod-cpu")
	t.Setenv("MEDIA_WORKER_TOKEN", workerSecret())
	if _, err := LoadMediaWorker(); err != nil {
		t.Fatal(err)
	}
	for _, url := range []string{"", "http://user:secret@api:9000", "file:///tmp/api", "https://api/private", "https://api?token=secret", "https://api/#token"} {
		t.Setenv("MEDIA_API_URL", url)
		if _, err := LoadMediaWorker(); err == nil {
			t.Fatal("invalid origin accepted")
		}
	}
}

func TestMediaStorageEndpointIsIndependentAndDefaultsToAPIEndpoint(t *testing.T) {
	t.Setenv("R2_ENDPOINT", "http://api-storage:9000")
	t.Setenv("R2_PUBLIC_ENDPOINT", "http://browser-storage:9000")
	t.Setenv("MEDIA_STORAGE_ENDPOINT", "")
	c, err := Load()
	if err != nil || c.MediaStorageEndpoint != "http://api-storage:9000" {
		t.Fatal("worker endpoint default", err)
	}
	t.Setenv("MEDIA_STORAGE_ENDPOINT", "http://worker-storage:9000")
	c, err = Load()
	if err != nil || c.MediaStorageEndpoint != "http://worker-storage:9000" || c.R2PublicEndpoint != "http://browser-storage:9000" {
		t.Fatal("worker/browser endpoints coupled", err)
	}
	t.Setenv("MEDIA_STORAGE_ENDPOINT", "file:///tmp/objects")
	if _, err := Load(); err == nil {
		t.Fatal("invalid worker storage origin accepted")
	}
}
