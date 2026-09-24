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

func TestStandaloneWorkerResourcesRequireNoAPISettings(t *testing.T) {
	t.Setenv("MEDIA_API_URL", "http://api:9000")
	t.Setenv("MEDIA_WORKER_ID", "cpu")
	t.Setenv("MEDIA_WORKER_TOKEN", workerSecret())
	t.Setenv("MEDIA_ACCEL", "")
	t.Setenv("MEDIA_WORKER_CONCURRENCY", "")
	t.Setenv("MEDIA_DRAIN_TIMEOUT", "")
	t.Setenv("CLIP_ENCODE_THREADS", "")
	t.Setenv("CLIP_DECODE_THREADS", "")
	t.Setenv("CLIP_MEDIA_STAGE_TIMEOUT", "invalid-api-only-setting")
	c, err := LoadWorkerConfig()
	if err != nil {
		t.Fatal(err)
	}
	if c.Accel != "cpu" || c.Concurrency != 1 || c.EncodeThreads != 1 || c.DecodeThreads != 2 {
		t.Fatal("CPU defaults", c.Accel, c.Concurrency, c.EncodeThreads, c.DecodeThreads)
	}
	for _, name := range []string{"MEDIA_ACCEL", "MEDIA_WORKER_CONCURRENCY", "MEDIA_DRAIN_TIMEOUT"} {
		t.Run(name, func(t *testing.T) {
			t.Setenv(name, "invalid")
			if _, err := LoadWorkerConfig(); err == nil {
				t.Fatal("invalid resource accepted")
			}
		})
	}
	t.Setenv("MEDIA_WORKER_CONCURRENCY", "2")
	if _, err := LoadWorkerConfig(); err == nil {
		t.Fatal("untested parallelism enabled")
	}
}

func TestAPIMediaSettingsStillLoadStageBudgets(t *testing.T) {
	t.Setenv("CLIP_MEDIA_STAGE_TIMEOUT", "47m")
	t.Setenv("CLIP_MEDIA_MAX_ATTEMPTS", "4")
	var c Config
	if err := loadClipMedia(&c); err != nil {
		t.Fatal(err)
	}
	if c.ClipMediaStageTimeout.String() != "47m0s" || c.ClipMediaMaxAttempts != 4 {
		t.Fatal("API stage budgets were dropped")
	}
}
