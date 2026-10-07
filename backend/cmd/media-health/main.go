// media-health is the small, container-local Docker readiness client. The live
// worker owns capability, artifact, profile and authenticated API validation.
package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var configKeys = []string{
	"MEDIA_API_URL", "MEDIA_WORKER_ID", "MEDIA_WORKER_ROLE", "MEDIA_ACCEL",
	"MEDIA_WORKER_CONCURRENCY", "MEDIA_DRAIN_TIMEOUT", "CLIP_WORK_ROOT",
	"CLIP_FFMPEG_PATH", "CLIP_FFPROBE_PATH", "CLIP_RESVG_PATH", "CLIP_OVERLAY_DIR",
	"CLIP_FONT_PATH", "CLIP_FONT_PAPERLOGY_PATH", "CLIP_FONT_JUA_PATH",
	"CLIP_FONT_NANUM_MYEONGJO_PATH", "CLIP_FONT_NANUM_MYEONGJO_BOLD_PATH",
	"CLIP_WORK_STALE_AGE", "CLIP_MEDIA_TIMEOUT", "CLIP_ENCODE_THREADS", "CLIP_DECODE_THREADS",
}

type healthStatus struct {
	Ready                      bool
	Profile                    string
	Waiting, Active, OwnActive int64
}

func socketPath(base, identity string) (string, error) {
	if base == "" {
		base = "/tmp/postpilot-clip-work"
	}
	worker := sha256.Sum256([]byte(identity))
	root, err := filepath.Abs(filepath.Join(base, "worker-"+hex.EncodeToString(worker[:])))
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(root))
	return filepath.Join("/tmp", "postpilot-health-"+hex.EncodeToString(digest[:]), "h.sock"), nil
}

func configuration(getenv func(string) string) string {
	values := make(map[string]string, len(configKeys))
	for _, key := range configKeys {
		values[key] = getenv(key)
	}
	raw, _ := json.Marshal(values)
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func check(ctx context.Context, path, identity, token, stamp string) (healthStatus, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if strings.ContainsAny(identity+token+stamp, "\r\n") {
		return healthStatus{}, errors.New("invalid worker readiness credentials")
	}
	connection, err := (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, "unix", path)
	if err != nil {
		return healthStatus{}, errors.New("worker readiness unavailable")
	}
	defer connection.Close()
	deadline := time.Now().Add(10 * time.Second)
	if limit, ok := ctx.Deadline(); ok && limit.Before(deadline) {
		deadline = limit
	}
	if err = connection.SetDeadline(deadline); err != nil {
		return healthStatus{}, err
	}
	stop := context.AfterFunc(ctx, func() { connection.SetDeadline(time.Now()) })
	defer stop()
	// A fixed local HTTP exchange needs no TLS, DNS, redirects or HTTP client
	// initialization in a second process alongside the executing renderer.
	_, err = fmt.Fprintf(connection, "GET /health HTTP/1.1\r\nHost: worker\r\nConnection: close\r\nX-Media-Worker-ID: %s\r\nX-Media-Worker-Config: %s\r\nAuthorization: Bearer %s\r\n\r\n", identity, stamp, token)
	if err != nil {
		return healthStatus{}, errors.New("worker readiness unavailable")
	}
	reader := bufio.NewReaderSize(connection, 4096)
	line := func() (string, error) {
		raw, err := reader.ReadSlice('\n')
		if err != nil {
			return "", errors.New("invalid worker readiness response")
		}
		return strings.TrimSuffix(strings.TrimSuffix(string(raw), "\n"), "\r"), nil
	}
	first, err := line()
	fields := strings.Fields(first)
	if err != nil || len(fields) < 2 || (fields[0] != "HTTP/1.1" && fields[0] != "HTTP/1.0") || fields[1] != "200" {
		return healthStatus{}, errors.New("worker is not ready")
	}
	length, headerBytes := -1, len(first)
	for {
		header, err := line()
		if err != nil {
			return healthStatus{}, err
		}
		headerBytes += len(header) + 2
		if headerBytes > 8192 {
			return healthStatus{}, errors.New("invalid worker readiness response")
		}
		if header == "" {
			break
		}
		name, value, found := strings.Cut(header, ":")
		if !found || name == "" || strings.IndexFunc(name, func(r rune) bool {
			return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("!#$%&'*+-.^_`|~", r))
		}) >= 0 {
			return healthStatus{}, errors.New("invalid worker readiness response")
		}
		switch strings.ToLower(name) {
		case "transfer-encoding":
			return healthStatus{}, errors.New("invalid worker readiness response")
		case "content-length":
			if length >= 0 {
				return healthStatus{}, errors.New("invalid worker readiness response")
			}
			value = strings.TrimSpace(value)
			if value == "" || strings.IndexFunc(value, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
				return healthStatus{}, errors.New("invalid worker readiness response")
			}
			length, err = strconv.Atoi(value)
			if err != nil || length < 0 || length > 4096 {
				return healthStatus{}, errors.New("invalid worker readiness response")
			}
		}
	}
	if length < 0 {
		return healthStatus{}, errors.New("invalid worker readiness response")
	}
	raw := make([]byte, length)
	if _, err = io.ReadFull(reader, raw); err != nil {
		return healthStatus{}, errors.New("invalid worker readiness response")
	}
	var status healthStatus
	if err = json.Unmarshal(raw, &status); err != nil || !status.Ready || status.Profile == "" ||
		status.Waiting < 0 || status.Active < 0 || status.OwnActive < 0 || status.OwnActive > status.Active {
		return healthStatus{}, errors.New("invalid worker readiness response")
	}
	return status, nil
}

func run(getenv func(string) string) error {
	identity, token := getenv("MEDIA_WORKER_ID"), getenv("MEDIA_WORKER_TOKEN")
	if identity == "" || token == "" || strings.ContainsAny(identity+token, "\r\n") {
		return errors.New("worker readiness credentials required")
	}
	path, err := socketPath(getenv("CLIP_WORK_ROOT"), identity)
	if err != nil {
		return errors.New("invalid worker readiness root")
	}
	status, err := check(context.Background(), path, identity, token, configuration(getenv))
	if err != nil {
		return err
	}
	raw, err := json.Marshal(status)
	if err != nil {
		return err
	}
	fmt.Println(string(raw))
	return nil
}

func main() {
	if err := run(os.Getenv); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
