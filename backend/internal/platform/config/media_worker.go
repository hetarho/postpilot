package config

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const maxWorkerCredentials = 64

// Worker credentials are deployment secrets, independent of user sessions.
func loadMediaListener(c *Config) error {
	c.MediaInternalAddr = os.Getenv("MEDIA_INTERNAL_ADDR")
	if c.MediaInternalAddr == "" {
		return nil
	}
	_, port, err := net.SplitHostPort(c.MediaInternalAddr)
	if err != nil {
		return errors.New("MEDIA_INTERNAL_ADDR must be host:port")
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return errors.New("MEDIA_INTERNAL_ADDR has invalid port")
	}
	c.MediaWorkerCredentials, err = ParseMediaWorkerCredentials(os.Getenv("MEDIA_WORKER_CREDENTIALS"))
	return err
}

func ValidWorkerIdentity(id string) bool {
	if len(id) < 1 || len(id) > 128 {
		return false
	}
	for _, c := range id {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.') {
			return false
		}
	}
	return true
}

func validWorkerSecret(secret string) bool {
	decoded, err := base64.RawURLEncoding.DecodeString(secret)
	if err != nil || len(decoded) < 32 || len(decoded) > 96 {
		return false
	}
	unique := map[byte]bool{}
	for _, b := range decoded {
		unique[b] = true
	}
	return len(unique) >= 16
}

// Token-by-token decoding rejects duplicate keys, which json.Unmarshal into a
// map would silently overwrite. Errors never include identities or secrets.
func ParseMediaWorkerCredentials(raw string) (map[string]string, error) {
	invalid := errors.New("MEDIA_WORKER_CREDENTIALS requires unique worker ids and random base64url secrets of at least 32 bytes")
	if len(raw) > 16<<10 {
		return nil, invalid
	}
	d := json.NewDecoder(strings.NewReader(raw))
	opening, err := d.Token()
	if err != nil || opening != json.Delim('{') {
		return nil, invalid
	}
	out := map[string]string{}
	secrets := map[string]bool{}
	for d.More() {
		key, err := d.Token()
		if err != nil {
			return nil, invalid
		}
		id, ok := key.(string)
		var token string
		if !ok || d.Decode(&token) != nil || !ValidWorkerIdentity(id) || !validWorkerSecret(token) || out[id] != "" || secrets[token] || len(out) >= maxWorkerCredentials {
			return nil, invalid
		}
		out[id], secrets[token] = token, true
	}
	if end, err := d.Token(); err != nil || end != json.Delim('}') {
		return nil, invalid
	}
	if _, err := d.Token(); err != io.EOF || len(out) == 0 {
		return nil, invalid
	}
	return out, nil
}

type MediaWorker struct{ APIURL, ID, Token string }

// LoadMediaWorker requires no API/database/provider configuration.
func LoadMediaWorker() (MediaWorker, error) {
	c := MediaWorker{APIURL: os.Getenv("MEDIA_API_URL"), ID: os.Getenv("MEDIA_WORKER_ID"), Token: os.Getenv("MEDIA_WORKER_TOKEN")}
	u, err := url.Parse(c.APIURL)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return MediaWorker{}, errors.New("MEDIA_API_URL must be an HTTP(S) origin on the private network")
	}
	if !ValidWorkerIdentity(c.ID) || !validWorkerSecret(c.Token) {
		return MediaWorker{}, errors.New("MEDIA_WORKER_ID and MEDIA_WORKER_TOKEN must match a configured worker credential")
	}
	c.APIURL = strings.TrimSuffix(c.APIURL, "/")
	return c, nil
}

// WorkerConfig contains no database, provider, storage credentials or user auth.
type WorkerConfig struct {
	MediaWorker
	Accel                                                    string
	Concurrency                                              int
	DrainTimeout                                             time.Duration
	WorkRoot, FFmpegPath, FFprobePath, ResvgPath, OverlayDir string
	FontPaths                                                map[string]string
	WorkStaleAge, MediaTimeout                               time.Duration
	EncodeThreads, DecodeThreads                             int
}

func LoadWorkerConfig() (WorkerConfig, error) {
	c, err := LoadWorkerRuntime()
	if err != nil {
		return c, err
	}
	c.MediaWorker, err = LoadMediaWorker()
	return c, err
}

// LoadWorkerRuntime permits offline image inspection without deployment secrets.
func LoadWorkerRuntime() (WorkerConfig, error) {
	c := WorkerConfig{Accel: getenv("MEDIA_ACCEL", "cpu")}
	var err error
	if c.Accel != "cpu" && c.Accel != "auto" && c.Accel != "nvenc" {
		return c, errors.New("MEDIA_ACCEL must be cpu, auto or nvenc")
	}
	c.Concurrency, err = positiveInt("MEDIA_WORKER_CONCURRENCY", "1")
	if err != nil || c.Concurrency != 1 {
		return c, errors.New("MEDIA_WORKER_CONCURRENCY must be 1")
	}
	c.DrainTimeout, err = positiveDuration("MEDIA_DRAIN_TIMEOUT", "30s")
	if err != nil || c.DrainTimeout > 15*time.Minute {
		return c, errors.New("MEDIA_DRAIN_TIMEOUT must be positive and at most 15m")
	}
	var media Config // reuse only the pure media settings parser, never Load.
	if err = loadClipTools(&media); err != nil {
		return c, err
	}
	c.WorkRoot, c.FFmpegPath, c.FFprobePath, c.ResvgPath, c.OverlayDir = media.ClipWorkRoot, media.ClipFFmpegPath, media.ClipFFprobePath, media.ClipResvgPath, media.ClipOverlayDir
	c.FontPaths, c.WorkStaleAge, c.MediaTimeout = media.ClipFontPaths, media.ClipWorkStaleAge, media.ClipMediaTimeout
	c.EncodeThreads, c.DecodeThreads = media.ClipEncodeThreads, media.ClipDecodeThreads
	return c, nil
}
