package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

func TestConnectVerifiesLoopbackEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/json/version" {
			http.NotFound(writer, request)
			return
		}
		fmt.Fprintf(writer, `{"webSocketDebuggerUrl":%q}`, "ws"+strings.TrimPrefix(serverURL(request), "http")+"/devtools/browser/one")
	}))
	defer server.Close()
	_, port, err := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	profile := t.TempDir()
	if err := os.WriteFile(filepath.Join(profile, devToolsActivePort), []byte(port+"\n/devtools/browser/one\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	session, err := Connect(profile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(session.CDPURL, "ws://127.0.0.1:"+port+"/devtools/browser/") {
		t.Fatalf("unexpected CDP URL %q", session.CDPURL)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestValidateCDPURLRejectsRemoteOrMismatchedEndpoint(t *testing.T) {
	for _, value := range []string{
		"ws://example.com:9222/devtools/browser/one",
		"ws://127.0.0.1:9333/devtools/browser/one",
		"https://127.0.0.1:9222/devtools/browser/one",
		"ws://127.0.0.1:9222/devtools/browser/two",
		"ws://127.0.0.1:9222/devtools/browser/one?redirected=true",
	} {
		if err := validateCDPURL(value, 9222, "/devtools/browser/one"); err == nil {
			t.Fatalf("expected %q to be rejected", value)
		}
	}
}

func TestConnectRejectsStalePortReusedByDifferentBrowser(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		fmt.Fprintf(writer, `{"webSocketDebuggerUrl":%q}`, "ws"+strings.TrimPrefix(serverURL(request), "http")+"/devtools/browser/current")
	}))
	defer server.Close()
	_, port, err := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	profile := t.TempDir()
	if err := os.WriteFile(filepath.Join(profile, devToolsActivePort), []byte(port+"\n/devtools/browser/stale\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Connect(profile); err == nil || !strings.Contains(err.Error(), "browser id") {
		t.Fatalf("stale profile endpoint error=%v", err)
	}
}

func TestOpenLoginNavigatesAnExistingDedicatedPage(t *testing.T) {
	var server *httptest.Server
	var navigated string
	var navigationMu sync.Mutex
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/json/version":
			fmt.Fprintf(writer, `{"webSocketDebuggerUrl":%q}`, "ws"+strings.TrimPrefix(server.URL, "http")+"/devtools/browser/one")
		case "/json/list":
			_ = json.NewEncoder(writer).Encode([]pageTarget{{
				ID: "page-1", Type: "page", URL: "about:blank",
				WebSocketDebuggerURL: "ws" + strings.TrimPrefix(server.URL, "http") + "/devtools/page/page-1",
			}})
		case "/devtools/browser/one":
			connection, err := websocket.Accept(writer, request, nil)
			if err != nil {
				return
			}
			defer connection.CloseNow()
			for {
				var call cdpRequest
				if err := wsjson.Read(request.Context(), connection, &call); err != nil {
					return
				}
				result := map[string]any{}
				switch call.Method {
				case "Target.attachToTarget":
					result = map[string]any{"sessionId": "session-page-1"}
				case "Page.navigate":
					params, _ := call.Params.(map[string]any)
					navigationMu.Lock()
					navigated, _ = params["url"].(string)
					navigationMu.Unlock()
				}
				response := map[string]any{"id": call.ID, "result": result}
				if call.Method != "Target.attachToTarget" {
					response["sessionId"] = "session-page-1"
				}
				if err := wsjson.Write(request.Context(), connection, response); err != nil {
					return
				}
			}
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, port, err := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	profile := t.TempDir()
	if err := os.WriteFile(filepath.Join(profile, devToolsActivePort), []byte(port+"\n/devtools/browser/one\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "browser")
	if err := os.WriteFile(binary, []byte("browser"), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := OpenLogin(binary, profile); err != nil {
		t.Fatal(err)
	}
	navigationMu.Lock()
	defer navigationMu.Unlock()
	if navigated != "https://nid.naver.com/nidlogin.login" {
		t.Fatalf("existing page navigated to %q", navigated)
	}
}

func TestOpenLoginCreatesAPageWhenTheDedicatedBrowserHasNoWindow(t *testing.T) {
	var server *httptest.Server
	var stateMu sync.Mutex
	created := false
	createdURL := ""
	attachedTarget := ""
	attachFlatten := false
	navigated := ""
	navigationSession := ""
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/json/version":
			fmt.Fprintf(writer, `{"webSocketDebuggerUrl":%q}`, "ws"+strings.TrimPrefix(server.URL, "http")+"/devtools/browser/one")
		case "/json/list":
			stateMu.Lock()
			defer stateMu.Unlock()
			if !created {
				_ = json.NewEncoder(writer).Encode([]pageTarget{})
				return
			}
			_ = json.NewEncoder(writer).Encode([]pageTarget{{
				ID: "page-1", Type: "page", URL: "about:blank",
				WebSocketDebuggerURL: "ws" + strings.TrimPrefix(server.URL, "http") + "/devtools/page/page-1",
			}})
		case "/devtools/browser/one":
			connection, err := websocket.Accept(writer, request, nil)
			if err != nil {
				return
			}
			defer connection.CloseNow()
			for {
				var call cdpRequest
				if err := wsjson.Read(request.Context(), connection, &call); err != nil {
					return
				}
				result := map[string]any{}
				switch call.Method {
				case "Target.createTarget":
					params, _ := call.Params.(map[string]any)
					stateMu.Lock()
					created = true
					createdURL, _ = params["url"].(string)
					stateMu.Unlock()
					result = map[string]any{"targetId": "page-1"}
				case "Target.attachToTarget":
					params, _ := call.Params.(map[string]any)
					stateMu.Lock()
					attachedTarget, _ = params["targetId"].(string)
					attachFlatten, _ = params["flatten"].(bool)
					stateMu.Unlock()
					result = map[string]any{"sessionId": "session-page-1"}
				case "Page.navigate":
					params, _ := call.Params.(map[string]any)
					stateMu.Lock()
					navigated, _ = params["url"].(string)
					navigationSession = call.SessionID
					stateMu.Unlock()
				}
				response := map[string]any{"id": call.ID, "result": result}
				if call.Method != "Target.createTarget" && call.Method != "Target.attachToTarget" {
					response["sessionId"] = "session-page-1"
				}
				if err := wsjson.Write(request.Context(), connection, response); err != nil {
					return
				}
			}
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, port, err := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	profile := t.TempDir()
	if err := os.WriteFile(filepath.Join(profile, devToolsActivePort), []byte(port+"\n/devtools/browser/one\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "browser")
	if err := os.WriteFile(binary, []byte("browser"), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := OpenLogin(binary, profile); err != nil {
		t.Fatal(err)
	}
	stateMu.Lock()
	defer stateMu.Unlock()
	if !created || createdURL != "about:blank" || attachedTarget != "page-1" || !attachFlatten ||
		navigated != "https://nid.naver.com/nidlogin.login" || navigationSession != "session-page-1" {
		t.Fatalf("created=%t createdURL=%q attachedTarget=%q flatten=%t navigated=%q session=%q",
			created, createdURL, attachedTarget, attachFlatten, navigated, navigationSession)
	}
}

func serverURL(request *http.Request) string {
	return "http://" + request.Host
}

func TestObserveNaverIdentityUsesTargetBoundCDPEvidence(t *testing.T) {
	var server *httptest.Server
	var stateMu sync.Mutex
	targetURL := "https://blog.naver.com/PostWriteForm.naver?blogId=alice"
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/json/list":
			stateMu.Lock()
			currentURL := targetURL
			stateMu.Unlock()
			_ = json.NewEncoder(writer).Encode([]pageTarget{{
				ID: "page-1", Type: "page", URL: currentURL,
				WebSocketDebuggerURL: "ws" + strings.TrimPrefix(server.URL, "http") + "/devtools/page/page-1",
			}})
		case "/devtools/browser/one":
			connection, err := websocket.Accept(writer, request, nil)
			if err != nil {
				return
			}
			defer connection.CloseNow()
			for {
				var call cdpRequest
				if err := wsjson.Read(request.Context(), connection, &call); err != nil {
					return
				}
				result := map[string]any{}
				switch call.Method {
				case "Target.attachToTarget":
					result = map[string]any{"sessionId": "session-page-1"}
				case "Page.navigate":
					t.Errorf("identity verifier must not drive Naver UI: %#v", call.Params)
				case "Runtime.evaluate":
					params, _ := call.Params.(map[string]any)
					if params["expression"] == identityPreparationScript {
						result = map[string]any{"result": map[string]any{"type": "object", "value": map[string]any{
							"editor_ready": true, "ready": true, "stage": "categories_visible",
						}}}
					} else {
						result = map[string]any{"result": map[string]any{
							"type": "object",
							"value": map[string]any{
								"href":         "https://blog.naver.com/PostWriteForm.naver?blogId=alice",
								"editor_ready": true,
								"blog_label":   "Alice Blog",
								"categories":   []map[string]string{{"id": "7", "name": "Travel"}},
							},
						}}
					}
				}
				response := map[string]any{"id": call.ID, "result": result}
				if call.Method != "Target.attachToTarget" {
					if call.SessionID != "session-page-1" {
						t.Errorf("page command was not bound to the attached target: %+v", call)
					}
					response["sessionId"] = "session-page-1"
				}
				if err := wsjson.Write(request.Context(), connection, response); err != nil {
					return
				}
			}
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	identity, err := ObserveNaverIdentity(
		ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/devtools/browser/one",
	)
	if err != nil {
		t.Fatal(err)
	}
	if identity.BlogID != "alice" || identity.BlogLabel != "Alice Blog" || len(identity.Categories) != 1 || identity.Categories[0].ID != "7" {
		t.Fatalf("identity = %+v", identity)
	}
}

func TestDiscoverSinglePageRejectsMultipleOrForeignTargets(t *testing.T) {
	for name, targets := range map[string][]pageTarget{
		"multiple": {
			{ID: "one", Type: "page", URL: "about:blank", WebSocketDebuggerURL: "unused"},
			{ID: "two", Type: "page", URL: "about:blank", WebSocketDebuggerURL: "unused"},
		},
		"foreign": {{
			ID: "one", Type: "page", URL: "https://example.com", WebSocketDebuggerURL: "unused",
		}},
	} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				_ = json.NewEncoder(writer).Encode(targets)
			}))
			defer server.Close()
			_, err := discoverSinglePage(
				context.Background(), "ws"+strings.TrimPrefix(server.URL, "http")+"/devtools/browser/one",
			)
			if err == nil {
				t.Fatal("unsafe target set was accepted")
			}
		})
	}
}

func TestValidateIdentityObservationRejectsErrorPagesOrConflictingCategories(t *testing.T) {
	for _, observation := range []identityObservation{
		{Href: "https://blog.naver.com/error.naver?blogId=alice", EditorReady: true, Categories: []NaverCategory{{ID: "7", Name: "Travel"}}},
		{Href: "https://blog.naver.com/PostWriteForm.naver?blogId=not/valid", EditorReady: true, Categories: []NaverCategory{{ID: "7", Name: "Travel"}}},
		{Href: "https://blog.naver.com/PostWriteForm.naver?blogId=alice", EditorReady: true, Categories: []NaverCategory{{ID: "7", Name: "Travel"}, {ID: "7", Name: "Other"}}},
		// Models an access-denied page that retained the requested-looking URL and
		// happened to contain generic inputs/category-shaped controls.
		{Href: "https://blog.naver.com/PostWriteForm.naver?blogId=alice", EditorReady: false, Categories: []NaverCategory{{ID: "7", Name: "Travel"}}},
	} {
		if _, err := validateIdentityObservation(observation); err == nil {
			t.Fatalf("unsafe observation accepted: %+v", observation)
		}
	}
}

func TestObserveNaverIdentityRejectsSecondTargetAppearingDuringVerification(t *testing.T) {
	var server *httptest.Server
	var stateMu sync.Mutex
	targetURL := "about:blank"
	verified := false
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/json/list":
			stateMu.Lock()
			currentURL, includeSecond := targetURL, verified
			stateMu.Unlock()
			targets := []pageTarget{{
				ID: "page-1", Type: "page", URL: currentURL,
				WebSocketDebuggerURL: "ws" + strings.TrimPrefix(server.URL, "http") + "/devtools/page/page-1",
			}}
			if includeSecond {
				targets = append(targets, pageTarget{ID: "page-2", Type: "page", URL: "about:blank"})
			}
			_ = json.NewEncoder(writer).Encode(targets)
		case "/devtools/browser/one":
			connection, err := websocket.Accept(writer, request, nil)
			if err != nil {
				return
			}
			defer connection.CloseNow()
			for {
				var call cdpRequest
				if err := wsjson.Read(request.Context(), connection, &call); err != nil {
					return
				}
				result := map[string]any{}
				if call.Method == "Target.attachToTarget" {
					result = map[string]any{"sessionId": "session-page-1"}
				}
				if call.Method == "Page.navigate" {
					stateMu.Lock()
					targetURL = "https://blog.naver.com/PostWriteForm.naver?blogId=alice"
					stateMu.Unlock()
				}
				if call.Method == "Runtime.evaluate" {
					params, _ := call.Params.(map[string]any)
					if params["expression"] == identityPreparationScript {
						result = map[string]any{"result": map[string]any{"type": "object", "value": map[string]any{
							"editor_ready": true, "ready": true, "stage": "categories_visible",
						}}}
					} else {
						result = map[string]any{"result": map[string]any{"type": "object", "value": map[string]any{
							"href": "https://blog.naver.com/PostWriteForm.naver?blogId=alice", "editor_ready": true,
							"categories": []map[string]string{{"id": "7", "name": "Travel"}},
						}}}
						stateMu.Lock()
						verified = true
						stateMu.Unlock()
					}
				}
				response := map[string]any{"id": call.ID, "result": result}
				if call.Method != "Target.attachToTarget" {
					response["sessionId"] = "session-page-1"
				}
				if err := wsjson.Write(request.Context(), connection, response); err != nil {
					return
				}
			}
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := ObserveNaverIdentity(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/devtools/browser/one")
	if err == nil || !strings.Contains(err.Error(), "recheck dedicated browser target") {
		t.Fatalf("target switch error=%v", err)
	}
}

func TestStartRefusesALockedProfileWithoutAVerifiedEndpoint(t *testing.T) {
	profile := t.TempDir()
	if err := os.Symlink("locked-by-chromium", filepath.Join(profile, "SingletonLock")); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "chromium")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\nexit 77\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := Start(binary, profile, "about:blank"); err == nil || !strings.Contains(err.Error(), "profile is locked") {
		t.Fatalf("profile lock error=%v", err)
	}
}

func TestInspectCompatibilitySuccessAndRefusalClasses(t *testing.T) {
	for _, test := range []struct {
		name          string
		missingDomain bool
		emptyAX       bool
		badEditor     bool
		targetSwitch  bool
		wantError     string
	}{
		{name: "success"},
		{name: "missing CDP capability", missingDomain: true, wantError: "Schema.getDomains"},
		{name: "empty accessibility tree", emptyAX: true, wantError: "empty accessibility tree"},
		{name: "editor mismatch evidence", badEditor: true},
		{name: "target switch", targetSwitch: true, wantError: "recheck dedicated browser target"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var server *httptest.Server
			var mu sync.Mutex
			finished := false
			server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				switch request.URL.Path {
				case "/json/list":
					mu.Lock()
					switched := finished && test.targetSwitch
					mu.Unlock()
					targets := []pageTarget{{ID: "page-1", Type: "page", URL: "https://blog.naver.com/PostWriteForm.naver?blogId=alice", WebSocketDebuggerURL: "ws" + strings.TrimPrefix(server.URL, "http") + "/devtools/page/page-1"}}
					if switched {
						targets = append(targets, pageTarget{ID: "page-2", Type: "page", URL: "about:blank"})
					}
					_ = json.NewEncoder(writer).Encode(targets)
				case "/devtools/browser/one":
					connection, err := websocket.Accept(writer, request, nil)
					if err != nil {
						return
					}
					defer connection.CloseNow()
					for {
						var call cdpRequest
						if err := wsjson.Read(request.Context(), connection, &call); err != nil {
							return
						}
						result := map[string]any{}
						response := map[string]any{"id": call.ID}
						switch call.Method {
						case "Browser.getVersion":
							result = map[string]any{"protocolVersion": "1.3", "product": "Chrome/152.0.0.0"}
						case "Target.attachToTarget":
							result = map[string]any{"sessionId": "session-page-1"}
						case "Schema.getDomains":
							if test.missingDomain {
								response["error"] = map[string]any{"code": -32601, "message": "Schema.getDomains unavailable"}
							} else {
								result = map[string]any{"domains": []map[string]string{{"name": "Accessibility"}, {"name": "DOM"}, {"name": "Page"}, {"name": "Runtime"}, {"name": "Schema"}}}
							}
						case "DOM.getDocument":
							result = map[string]any{"root": map[string]any{"nodeId": 1}}
						case "Accessibility.getFullAXTree":
							nodes := []map[string]any{{"role": map[string]any{"value": "RootWebArea"}}}
							if test.emptyAX {
								nodes = nil
							}
							result = map[string]any{"nodes": nodes}
						case "Runtime.evaluate":
							surface := map[string]any{"editor_root": !test.badEditor, "title_editor": true, "body_editor": true, "image_control": true, "settings_layer": true, "category_control": true, "category_options": 1, "visibility_choice": true, "tags_control": true, "final_control": true, "readback_surface": true}
							result = map[string]any{"result": map[string]any{"value": surface}}
							mu.Lock()
							finished = true
							mu.Unlock()
						}
						if _, hasError := response["error"]; !hasError {
							response["result"] = result
						}
						if call.Method != "Browser.getVersion" && call.Method != "Target.attachToTarget" {
							response["sessionId"] = "session-page-1"
						}
						if err := wsjson.Write(request.Context(), connection, response); err != nil {
							return
						}
					}
				default:
					http.NotFound(writer, request)
				}
			}))
			defer server.Close()
			evidence, err := InspectCompatibility(context.Background(), "ws"+strings.TrimPrefix(server.URL, "http")+"/devtools/browser/one")
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("error=%v, want containing %q", err, test.wantError)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if test.badEditor {
				if evidence.Editor.EditorRoot {
					t.Fatal("mismatched editor evidence was changed")
				}
				return
			}
			if evidence.BrowserProduct != "Chrome/152.0.0.0" || evidence.TargetID != "page-1" || len(evidence.AXRoles) != 1 {
				t.Fatalf("evidence=%+v", evidence)
			}
		})
	}
}
