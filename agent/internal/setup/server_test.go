package setup

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/agent/internal/browser"
	"github.com/postpilot/agent/internal/config"
)

func TestConnectionIsPersistedUnarmedBeforeActivation(t *testing.T) {
	root := t.TempDir()
	paths := config.Paths{
		Root: root, ConfigFile: filepath.Join(root, "config.json"),
		Profiles: filepath.Join(root, "profiles"), Jobs: filepath.Join(root, "jobs"), Logs: filepath.Join(root, "logs"),
	}
	id := "00112233445566778899aabbccddeeff"
	draft := config.ConnectionDraft{ID: id, Label: "내 Mac", APIURL: "https://api.example.com", BrowserBinary: "/browser", BrowserLabel: "Chrome", ProfileDir: filepath.Join(paths.Profiles, id), WorkDir: filepath.Join(paths.Jobs, id), CreatedAt: time.Unix(1, 0)}
	connection := config.Connection{ID: id, DraftID: id, APIURL: draft.APIURL, AgentID: "agent", KeychainAccount: "key", BrowserBinary: draft.BrowserBinary, BrowserLabel: draft.BrowserLabel, ProfileDir: draft.ProfileDir, WorkDir: draft.WorkDir, LeaseTTLSeconds: 45, Armed: true}

	cfg, index, err := persistPending(paths, config.File{Drafts: []config.ConnectionDraft{draft}}, connection)
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
	if err := activatePending(paths, cfg, index, 0, connection); err != nil {
		t.Fatal(err)
	}
	stored, err = config.Load(paths)
	if err != nil {
		t.Fatal(err)
	}
	if !stored.Connections[index].Armed {
		t.Fatal("successfully synchronized connection remained unarmed")
	}
	if len(stored.Drafts) != 0 || stored.Connections[index].DraftID != "" || stored.Connections[index].ProfileDir != draft.ProfileDir || stored.Connections[index].WorkDir != draft.WorkDir {
		t.Fatalf("activation did not bind and preserve the draft paths: %+v", stored)
	}
}

func TestDraftSurvivesCodeReplacementAndSetupRestartWithTheSameProfile(t *testing.T) {
	root := t.TempDir()
	paths := config.Paths{Root: root, ConfigFile: filepath.Join(root, "config.json"), Profiles: filepath.Join(root, "profiles"), Jobs: filepath.Join(root, "jobs"), Logs: filepath.Join(root, "logs")}
	id := "00112233445566778899aabbccddeeff"
	server := Server{
		Paths: paths, nonce: "test-nonce", host: "127.0.0.1:43210",
		NewDraftID: func() (string, error) { return id, nil },
		Now:        func() time.Time { return time.Unix(123, 0) },
		SupportedBrowser: func(binary string) (browser.Installation, bool) {
			return browser.Installation{Label: "Test Chromium", Binary: binary}, true
		},
	}
	postSetup(t, server, url.Values{"action": {"new"}, "api_url": {"https://api.example.com"}, "label": {"내 Mac"}, "browser_binary": {"/test/chromium"}})

	stored, err := config.Load(paths)
	if err != nil || len(stored.Drafts) != 1 {
		t.Fatalf("stored draft=%+v err=%v", stored, err)
	}
	wantProfile := stored.Drafts[0].ProfileDir
	// A local identity marker stands in for Chromium's persisted Naver cookie jar. Setup
	// never reads it; the fake browser does, exactly where a relaunched browser would.
	if err := os.WriteFile(filepath.Join(wantProfile, "signed-in-blog"), []byte("alice-blog"), 0o600); err != nil {
		t.Fatal(err)
	}
	var opened []string
	var identities []string
	// A fresh Server value represents a stopped and restarted setup companion. Device codes
	// are deliberately different and neither participates in path selection.
	restarted := Server{Paths: paths, nonce: server.nonce, host: server.host,
		SupportedBrowser: server.SupportedBrowser,
		OpenLogin: func(_, profile string) error {
			opened = append(opened, profile)
			identity, err := os.ReadFile(filepath.Join(profile, "signed-in-blog"))
			if err != nil {
				return err
			}
			identities = append(identities, string(identity))
			return nil
		},
	}
	indexRequest := httptest.NewRequest(http.MethodGet, "/?nonce="+restarted.nonce, nil)
	indexRequest.Host = restarted.host
	indexResponse := httptest.NewRecorder()
	restarted.index(indexResponse, indexRequest)
	if !strings.Contains(indexResponse.Body.String(), `name="draft_id" value="`+id+`"`) {
		t.Fatalf("restarted setup did not present the draft as resumable: %s", indexResponse.Body.String())
	}
	for _, code := range []string{"OLD-CODE", "NEW-CODE"} {
		postSetup(t, restarted, url.Values{"action": {"login"}, "draft_id": {id}, "device_code": {code}})
	}
	if len(opened) != 2 || opened[0] != wantProfile || opened[1] != wantProfile {
		t.Fatalf("opened profiles = %v, want the same %q", opened, wantProfile)
	}
	if len(identities) != 2 || identities[0] != "alice-blog" || identities[1] != "alice-blog" {
		t.Fatalf("reopened browser identities = %v", identities)
	}
	stored, err = config.Load(paths)
	if err != nil || len(stored.Drafts) != 1 {
		t.Fatalf("resume created another draft: %+v err=%v", stored, err)
	}
}

func TestExplicitNewConnectionsCreateIsolatedProfiles(t *testing.T) {
	root := t.TempDir()
	paths := config.Paths{Root: root, ConfigFile: filepath.Join(root, "config.json"), Profiles: filepath.Join(root, "profiles"), Jobs: filepath.Join(root, "jobs"), Logs: filepath.Join(root, "logs")}
	ids := []string{"00112233445566778899aabbccddeeff", "ffeeddccbbaa99887766554433221100"}
	server := Server{Paths: paths, nonce: "test-nonce", host: "127.0.0.1:43210", Now: time.Now,
		NewDraftID: func() (string, error) { id := ids[0]; ids = ids[1:]; return id, nil },
		SupportedBrowser: func(binary string) (browser.Installation, bool) {
			return browser.Installation{Label: "Test Chromium", Binary: binary}, true
		},
	}
	for _, label := range []string{"첫 계정", "둘째 계정"} {
		postSetup(t, server, url.Values{"action": {"new"}, "api_url": {"https://api.example.com"}, "label": {label}, "browser_binary": {"/test/chromium"}})
	}
	stored, err := config.Load(paths)
	if err != nil || len(stored.Drafts) != 2 || stored.Drafts[0].ProfileDir == stored.Drafts[1].ProfileDir || stored.Drafts[0].WorkDir == stored.Drafts[1].WorkDir {
		t.Fatalf("draft isolation failed: %+v err=%v", stored, err)
	}
}

func TestDraftLookupFailsClosedForMissingMalformedAndDuplicateReferences(t *testing.T) {
	id := "00112233445566778899aabbccddeeff"
	draft := config.ConnectionDraft{ID: id}
	for name, cfgAndID := range map[string]struct {
		cfg config.File
		id  string
	}{
		"absent":    {cfg: config.File{}, id: id},
		"malformed": {cfg: config.File{Drafts: []config.ConnectionDraft{draft}}, id: "../profile"},
		"duplicate": {cfg: config.File{Drafts: []config.ConnectionDraft{draft, draft}}, id: id},
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := exactDraft(cfgAndID.cfg, cfgAndID.id); err == nil {
				t.Fatal("unsafe draft reference was accepted")
			}
		})
	}
}

func TestSetupPageReportsAnInvalidLocalConfig(t *testing.T) {
	root := t.TempDir()
	paths := config.Paths{Root: root, ConfigFile: filepath.Join(root, "config.json"), Profiles: filepath.Join(root, "profiles"), Jobs: filepath.Join(root, "jobs"), Logs: filepath.Join(root, "logs")}
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.ConfigFile, []byte(`{"drafts":[{"id":"../escape"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	server := Server{Paths: paths, nonce: "test-nonce", host: "127.0.0.1:43210"}
	request := httptest.NewRequest(http.MethodGet, "/?nonce="+server.nonce, nil)
	request.Host = server.host
	response := httptest.NewRecorder()
	server.index(response, request)
	if !strings.Contains(response.Body.String(), "구성이 올바르지 않아 안전하게 열 수 없") {
		t.Fatalf("invalid config was not actionable: %s", response.Body.String())
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

func TestPairingFailsClosedWithoutMutatingTheDraftProfileWhenPublisherProbeIsMissing(t *testing.T) {
	root := t.TempDir()
	id := "00112233445566778899aabbccddeeff"
	paths := config.Paths{Root: root, ConfigFile: filepath.Join(root, "config.json"), Profiles: filepath.Join(root, "profiles"), Jobs: filepath.Join(root, "jobs"), Logs: filepath.Join(root, "logs")}
	draft := config.ConnectionDraft{ID: id, Label: "내 Mac", APIURL: "https://api.example.com", BrowserBinary: "/test/chromium", BrowserLabel: "Test Chromium", ProfileDir: filepath.Join(paths.Profiles, id), WorkDir: filepath.Join(paths.Jobs, id), CreatedAt: time.Now()}
	if err := config.Save(paths, config.File{Drafts: []config.ConnectionDraft{draft}}); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(draft.ProfileDir, 0o700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(draft.ProfileDir, "cookie-marker")
	if err := os.WriteFile(marker, []byte("same-session"), 0o600); err != nil {
		t.Fatal(err)
	}
	server := Server{
		Paths: paths,
		nonce: "test-nonce",
		host:  "127.0.0.1:43210",
	}
	form := url.Values{
		"action":             {"pair"},
		"draft_id":           {id},
		"device_code":        {"ABCD-EFGH"},
		"identity_confirmed": {"yes"},
		"nonce":              {server.nonce},
	}
	request := httptest.NewRequest(http.MethodPost, "/setup", strings.NewReader(form.Encode()))
	authorizeSetupRequest(request, server)
	response := httptest.NewRecorder()

	server.submit(response, request)

	if !strings.Contains(response.Body.String(), "아직 구현되지 않아 연결을 활성화할 수 없") {
		t.Fatalf("unexpected response: %s", response.Body.String())
	}
	if data, err := os.ReadFile(marker); err != nil || string(data) != "same-session" {
		t.Fatalf("fail-closed pairing changed the draft profile: %q %v", data, err)
	}
}

func authorizeSetupRequest(request *http.Request, server Server) {
	request.Host = server.host
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", "http://"+server.host)
	request.Header.Set("Sec-Fetch-Site", "same-origin")
}

func postSetup(t *testing.T, server Server, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	form.Set("nonce", server.nonce)
	request := httptest.NewRequest(http.MethodPost, "/setup", strings.NewReader(form.Encode()))
	authorizeSetupRequest(request, server)
	response := httptest.NewRecorder()
	server.submit(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("setup returned %d: %s", response.Code, response.Body.String())
	}
	return response
}
