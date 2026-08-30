package setup

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/postpilot/agent/internal/config"
)

func TestConnectionIsPersistedUnarmedBeforeActivation(t *testing.T) {
	root := t.TempDir()
	paths := config.Paths{
		Root: root, ConfigFile: filepath.Join(root, "config.json"),
		Profiles: filepath.Join(root, "profiles"), Jobs: filepath.Join(root, "jobs"), Logs: filepath.Join(root, "logs"),
	}
	connection := config.Connection{ID: "pending", APIURL: "https://api.example.com", AgentID: "agent", KeychainAccount: "key", BrowserBinary: "/browser", ProfileDir: "/profile", HermesBinary: "/hermes", HermesProfile: "postpilot-pending", LeaseTTLSeconds: 45, Armed: true}

	cfg, index, err := persistPending(paths, config.File{}, connection)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := config.Load(paths)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Connections[index].Armed {
		t.Fatal("pending connection was selectable before server profile sync")
	}
	if err := activatePending(paths, cfg, index, connection); err != nil {
		t.Fatal(err)
	}
	stored, err = config.Load(paths)
	if err != nil {
		t.Fatal(err)
	}
	if !stored.Connections[index].Armed {
		t.Fatal("successfully synchronized connection remained unarmed")
	}
}

func TestRepairOpensTheExistingArmedConnectionProfileWithoutADeviceCode(t *testing.T) {
	root := t.TempDir()
	paths := config.Paths{
		Root: root, ConfigFile: filepath.Join(root, "config.json"),
		Profiles: filepath.Join(root, "profiles"), Jobs: filepath.Join(root, "jobs"), Logs: filepath.Join(root, "logs"),
	}
	connection := config.Connection{
		ID: "existing", Label: "침실 Mac", BrowserBinary: "/browser", BrowserLabel: "Chrome",
		ProfileDir: "/isolated/alice", Armed: true,
	}
	if err := config.Save(paths, config.File{Connections: []config.Connection{connection}}); err != nil {
		t.Fatal(err)
	}
	var openedBinary, openedProfile string
	server := Server{
		Paths: paths,
		nonce: "test-nonce",
		host:  "127.0.0.1:43210",
		OpenLogin: func(binary, profile string) error {
			openedBinary, openedProfile = binary, profile
			return nil
		},
	}
	form := url.Values{"action": {"repair"}, "connection_id": {"existing"}, "nonce": {server.nonce}}
	request := httptest.NewRequest(http.MethodPost, "/setup", strings.NewReader(form.Encode()))
	authorizeSetupRequest(request, server)
	response := httptest.NewRecorder()

	server.submit(response, request)

	if openedBinary != connection.BrowserBinary || openedProfile != connection.ProfileDir {
		t.Fatalf("opened %q %q", openedBinary, openedProfile)
	}
	if !strings.Contains(response.Body.String(), "같은 발행 작업을 안전하게 다시 시도") {
		t.Fatalf("repair guidance missing: %s", response.Body.String())
	}
}

func TestRepairRejectsUnknownOrUnarmedConnection(t *testing.T) {
	root := t.TempDir()
	paths := config.Paths{
		Root: root, ConfigFile: filepath.Join(root, "config.json"),
		Profiles: filepath.Join(root, "profiles"), Jobs: filepath.Join(root, "jobs"), Logs: filepath.Join(root, "logs"),
	}
	if err := config.Save(paths, config.File{Connections: []config.Connection{{ID: "pending", Armed: false}}}); err != nil {
		t.Fatal(err)
	}
	server := Server{Paths: paths, nonce: "test-nonce", host: "127.0.0.1:43210", OpenLogin: func(string, string) error {
		t.Fatal("unarmed connection was opened")
		return nil
	}}
	form := url.Values{"action": {"repair"}, "connection_id": {"pending"}, "nonce": {server.nonce}}
	request := httptest.NewRequest(http.MethodPost, "/setup", strings.NewReader(form.Encode()))
	authorizeSetupRequest(request, server)
	response := httptest.NewRecorder()

	server.submit(response, request)

	if !strings.Contains(response.Body.String(), "활성 연결을 찾지 못") {
		t.Fatalf("unexpected response: %s", response.Body.String())
	}
}

func TestSetupRejectsForgedHostOriginNonceAndCrossSiteRequests(t *testing.T) {
	for name, mutate := range map[string]func(*http.Request){
		"host":       func(request *http.Request) { request.Host = "attacker.example" },
		"origin":     func(request *http.Request) { request.Header.Set("Origin", "https://attacker.example") },
		"cross-site": func(request *http.Request) { request.Header.Set("Sec-Fetch-Site", "cross-site") },
	} {
		t.Run(name, func(t *testing.T) {
			opened := false
			server := Server{
				nonce: "test-nonce", host: "127.0.0.1:43210",
				OpenLogin: func(string, string) error { opened = true; return nil },
			}
			form := url.Values{"action": {"repair"}, "connection_id": {"anything"}, "nonce": {server.nonce}}
			request := httptest.NewRequest(http.MethodPost, "/setup", strings.NewReader(form.Encode()))
			authorizeSetupRequest(request, server)
			mutate(request)
			response := httptest.NewRecorder()
			server.submit(response, request)
			if response.Code != http.StatusForbidden || opened {
				t.Fatalf("code=%d opened=%v", response.Code, opened)
			}
		})
	}

	server := Server{nonce: "expected", host: "127.0.0.1:43210"}
	form := url.Values{"action": {"repair"}, "connection_id": {"anything"}, "nonce": {"wrong"}}
	request := httptest.NewRequest(http.MethodPost, "/setup", strings.NewReader(form.Encode()))
	authorizeSetupRequest(request, server)
	response := httptest.NewRecorder()
	server.submit(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("wrong nonce code=%d", response.Code)
	}
}

func TestSetupPageCarriesNonceAndSecurityHeaders(t *testing.T) {
	server := Server{nonce: "test-nonce", host: "127.0.0.1:43210"}
	request := httptest.NewRequest(http.MethodGet, "/?nonce=test-nonce", nil)
	request.Host = server.host
	request.Header.Set("Sec-Fetch-Site", "none")
	response := httptest.NewRecorder()
	server.index(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `name="nonce" value="test-nonce"`) {
		t.Fatalf("code=%d body=%s", response.Code, response.Body.String())
	}
	if response.Header().Get("Content-Security-Policy") == "" || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("missing security headers: %v", response.Header())
	}
	if response.Header().Get("Referrer-Policy") != "same-origin" {
		t.Fatalf("same-origin form contract is not preserved: %v", response.Header())
	}
	if !strings.Contains(response.Body.String(), `value="`+config.DefaultAPIURL+`"`) {
		t.Fatalf("production API default is missing: %s", response.Body.String())
	}
}

func TestSuccessfulSetupCompletionCanBeSignaledOnlyOnceWithoutBlocking(t *testing.T) {
	completed := make(chan struct{}, 1)
	server := Server{completed: completed}
	server.finish()
	server.finish()
	select {
	case <-completed:
	default:
		t.Fatal("setup completion was not signaled")
	}
}

func TestSetupRejectsOversizedForms(t *testing.T) {
	server := Server{nonce: "test-nonce", host: "127.0.0.1:43210"}
	body := "nonce=test-nonce&action=repair&connection_id=" + strings.Repeat("a", 70<<10)
	request := httptest.NewRequest(http.MethodPost, "/setup", strings.NewReader(body))
	authorizeSetupRequest(request, server)
	response := httptest.NewRecorder()
	server.submit(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("oversized form code=%d", response.Code)
	}
}

func authorizeSetupRequest(request *http.Request, server Server) {
	request.Host = server.host
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", "http://"+server.host)
	request.Header.Set("Sec-Fetch-Site", "same-origin")
}
