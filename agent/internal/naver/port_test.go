package naver

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

const fakeWriterURL = "https://blog.naver.com/PostWriteForm.naver?blogId=alice"

func healthyObservation() map[string]any {
	return map[string]any{
		"href": fakeWriterURL, "auth": "ready", "editor_root": true, "settings_layer": true,
		"title": "제목입니다",
		"blocks": []map[string]any{
			{"kind": "text", "text": "본문 문단", "ordinal": 0, "caption": "", "uploaded": false, "source": ""},
			{"kind": "text", "text": "“인용”", "ordinal": 0, "caption": "", "uploaded": false, "source": ""},
			{"kind": "text", "text": "- 하나\n- 둘", "ordinal": 0, "caption": "", "uploaded": false, "source": ""},
			{"kind": "image", "text": "", "ordinal": 1, "caption": "캡션", "uploaded": true, "source": "a.jpg"},
		},
		"image_count": 1,
		"tags":        []string{"태그"},
		"category":    map[string]any{"id": "17", "name": "식당", "selected": true},
		"visibility":  map[string]any{"id": "public", "name": "전체공개", "selected": true},
		"locator_matches": map[string]int{
			"title": 1, "text": 1, "heading": 1, "quote": 1, "list": 1, "open_settings": 1,
			"upload_image": 1, "image_caption": 1, "tags": 1, "category": 1, "visibility": 4,
		},
		"final_controls": 1, "unversioned_publish_like": 0,
	}
}

func closedObservation() map[string]any {
	observation := healthyObservation()
	observation["settings_layer"] = false
	observation["tags"] = []string{}
	observation["category"] = map[string]any{"id": "", "name": "", "selected": false}
	observation["visibility"] = map[string]any{"id": "", "name": "", "selected": false}
	matches := maps.Clone(observation["locator_matches"].(map[string]int))
	matches["tags"], matches["category"], matches["visibility"] = 0, 0, 0
	observation["locator_matches"] = matches
	observation["final_controls"] = 0
	return observation
}

type fakeEditor struct {
	observation map[string]any
	targetsFor  func(call int) []map[string]any
	evaluateErr bool
	emptyAX     bool

	// The driver's reviewed functions, scripted. Nil hooks resolve the healthy shape.
	point    func(target string, ordinal int) driverPoint
	activate func(control, id string) driverActivation
	state    func() driverSettingsState
	setting  func(control, id, name string) driverSetting

	mu               sync.Mutex
	calls            int
	inputs           []string
	pendingControl   string
	pendingID        string
	pendingName      string
	pendingText      string
	categoryExpanded bool
}

func (editor *fakeEditor) record(entry string) {
	editor.mu.Lock()
	defer editor.mu.Unlock()
	editor.inputs = append(editor.inputs, entry)
}

func (editor *fakeEditor) recorded() []string {
	editor.mu.Lock()
	defer editor.mu.Unlock()
	return slices.Clone(editor.inputs)
}

func (editor *fakeEditor) settingsState() driverSettingsState {
	if editor.state != nil {
		return editor.state()
	}
	if open, _ := editor.observation["settings_layer"].(bool); !open {
		return driverSettingsState{}
	}
	matches, _ := editor.observation["locator_matches"].(map[string]int)
	return driverSettingsState{LayerMatches: 1, Tags: matches["tags"], Category: matches["category"], Visibility: matches["visibility"]}
}

func (editor *fakeEditor) settingState(control, id, name string) driverSetting {
	if editor.setting != nil {
		return editor.setting(control, id, name)
	}
	open, _ := editor.observation["settings_layer"].(bool)
	if !open {
		return driverSetting{}
	}
	base := driverSetting{Matches: 1, Actionable: true, X: 30, Y: 40, NameMatches: true}
	switch control {
	case "category_open":
		base.GroupMatches = 1
		base.Expanded = editor.categoryExpanded
	case "category":
		base.GroupMatches = 11
		base.Expanded = editor.categoryExpanded
		selected, _ := editor.observation["category"].(map[string]any)
		base.Checked = selected["selected"] == true && selected["id"] == id
	case "visibility":
		base.GroupMatches = 4
		base.Expanded = true
		selected, _ := editor.observation["visibility"].(map[string]any)
		base.Checked = selected["selected"] == true && selected["id"] == id
	}
	return base
}

func (editor *fakeEditor) remember(control, id, name string) {
	editor.mu.Lock()
	defer editor.mu.Unlock()
	editor.pendingControl, editor.pendingID, editor.pendingName = control, id, name
}

func (editor *fakeEditor) applyPendingClick() {
	editor.mu.Lock()
	defer editor.mu.Unlock()
	switch editor.pendingControl {
	case "settings_open":
		editor.observation["settings_layer"] = true
		matches := maps.Clone(editor.observation["locator_matches"].(map[string]int))
		matches["tags"], matches["category"], matches["visibility"] = 1, 1, 4
		editor.observation["locator_matches"] = matches
	case "category_open":
		editor.categoryExpanded = true
	case "category":
		editor.observation["category"] = map[string]any{"id": editor.pendingID, "name": editor.pendingName, "selected": true}
	case "visibility":
		editor.observation["visibility"] = map[string]any{"id": editor.pendingID, "name": visibilityName(editor.pendingID), "selected": true}
	}
}

func (editor *fakeEditor) rememberText(value string) {
	editor.mu.Lock()
	defer editor.mu.Unlock()
	if editor.pendingControl == "tag_input" {
		editor.pendingText = value
	}
}

func (editor *fakeEditor) commitTag() {
	editor.mu.Lock()
	defer editor.mu.Unlock()
	if editor.pendingControl != "tag_input" || editor.pendingText == "" {
		return
	}
	tags, _ := editor.observation["tags"].([]string)
	editor.observation["tags"] = append(tags, editor.pendingText)
	editor.pendingText = ""
}

func startFakeCDP(t *testing.T, editor *fakeEditor) string {
	t.Helper()
	var server *httptest.Server
	page := func(id, pageURL string) map[string]any {
		return map[string]any{"id": id, "type": "page", "url": pageURL,
			"webSocketDebuggerUrl": "ws" + strings.TrimPrefix(server.URL, "http") + "/devtools/page/" + id}
	}
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/json/list":
			editor.mu.Lock()
			editor.calls++
			call := editor.calls
			editor.mu.Unlock()
			targets := []map[string]any{page("page-1", fakeWriterURL)}
			if editor.targetsFor != nil {
				if custom := editor.targetsFor(call); custom != nil {
					targets = custom
					for index := range targets {
						if _, ok := targets[index]["webSocketDebuggerUrl"]; !ok {
							targets[index] = page(targets[index]["id"].(string), targets[index]["url"].(string))
						}
					}
				}
			}
			_ = json.NewEncoder(writer).Encode(targets)
		case "/devtools/browser/one":
			connection, err := websocket.Accept(writer, request, nil)
			if err != nil {
				return
			}
			defer connection.CloseNow()
			for {
				var call struct {
					ID     int            `json:"id"`
					Method string         `json:"method"`
					Params map[string]any `json:"params"`
				}
				if err := wsjson.Read(request.Context(), connection, &call); err != nil {
					return
				}
				response := map[string]any{"id": call.ID}
				result := map[string]any{}
				switch call.Method {
				case "Browser.getVersion":
					result = map[string]any{"protocolVersion": "1.3", "product": "Chrome/152.0.0.0"}
				case "Target.attachToTarget":
					result = map[string]any{"sessionId": "session-page-1"}
				case "Accessibility.getFullAXTree":
					nodes := []map[string]any{{"role": map[string]any{"value": "RootWebArea"}}}
					if editor.emptyAX {
						nodes = nil
					}
					result = map[string]any{"nodes": nodes}
				case "Runtime.evaluate":
					if editor.evaluateErr {
						response["error"] = map[string]any{"code": -32000, "message": "Execution context was destroyed"}
					} else if expression, _ := call.Params["expression"].(string); expression == "document" {
						// CallFunction resolves the document once before every reviewed call.
						result = map[string]any{"result": map[string]any{"objectId": "document-1"}}
					} else {
						result = map[string]any{"result": map[string]any{"value": editor.observation}}
					}
				case "Runtime.callFunctionOn":
					declaration, _ := call.Params["functionDeclaration"].(string)
					first, second, third := callArgs(call.Params)
					switch {
					case strings.Contains(declaration, "const caretEnd"):
						resolved := driverPoint{Matches: 1, X: 10, Y: 20}
						if editor.point != nil {
							ordinal, _ := second.(float64)
							target, _ := first.(string)
							resolved = editor.point(target, int(ordinal))
						}
						target := toString(first)
						editor.record("point:" + target)
						editor.remember(target, "", "")
						result = map[string]any{"result": map[string]any{"value": resolved}}
					case strings.Contains(declaration, "layer_matches: layers.length"):
						result = map[string]any{"result": map[string]any{"value": editor.settingsState()}}
					case strings.Contains(declaration, "name_matches"):
						control, id, name := toString(first), toString(second), toString(third)
						resolved := editor.settingState(control, id, name)
						editor.record("setting:" + control + ":" + id)
						editor.remember(control, id, name)
						result = map[string]any{"result": map[string]any{"value": resolved}}
					case strings.Contains(declaration, "'settings_open'"):
						resolved := driverActivation{Matches: 1, Activated: true}
						if editor.activate != nil {
							control, _ := first.(string)
							resolved = editor.activate(control, toString(second))
						}
						editor.record("activate:" + toString(first))
						result = map[string]any{"result": map[string]any{"value": resolved}}
					}
				case "Input.dispatchMouseEvent":
					if phase, _ := call.Params["type"].(string); phase == "mousePressed" {
						editor.record("click")
					} else if phase == "mouseReleased" {
						editor.applyPendingClick()
					}
				case "Input.insertText":
					text, _ := call.Params["text"].(string)
					editor.record("text:" + text)
					editor.rememberText(text)
				case "Input.dispatchKeyEvent":
					if phase, _ := call.Params["type"].(string); phase == "rawKeyDown" {
						editor.record("enter")
						editor.commitTag()
					}
				}
				if _, failed := response["error"]; !failed {
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
	t.Cleanup(server.Close)
	return "ws" + strings.TrimPrefix(server.URL, "http") + "/devtools/browser/one"
}

func observeFake(t *testing.T, editor *fakeEditor) (Snapshot, error) {
	t.Helper()
	if editor.observation == nil {
		editor.observation = healthyObservation()
	}
	port, err := NewCDPPort(context.Background(), startFakeCDP(t, editor))
	if err != nil {
		return Snapshot{}, err
	}
	defer port.Close()
	return port.Observe(context.Background())
}

func TestObserveProjectsTheLiveEditorTheWayTheNaverExportMapsIt(t *testing.T) {
	snapshot, err := observeFake(t, &fakeEditor{})
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := Manifest()
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.TargetID != "page-1" || snapshot.URL != fakeWriterURL || snapshot.AccountID != "alice" ||
		snapshot.SignatureID != manifest.SignatureID || snapshot.Auth != AuthReady || snapshot.Token == "" {
		t.Fatalf("binding=%+v", snapshot)
	}
	if snapshot.Title != "제목입니다" || snapshot.ImageCount != 1 ||
		snapshot.Category != (SelectedSetting{ID: "17", Name: "식당", Selected: true}) ||
		snapshot.Visibility != (SelectedSetting{ID: "public", Name: "전체공개", Selected: true}) || !snapshot.SettingsLayerOpen {
		t.Fatalf("settings=%+v", snapshot)
	}
	want := []SemanticBlock{
		{Kind: SemanticText, Text: "본문 문단"},
		{Kind: SemanticText, Text: "“인용”"},
		{Kind: SemanticText, Text: "- 하나\n- 둘"},
		{Kind: SemanticImage, Ordinal: 1, Caption: "캡션", Uploaded: true},
	}
	if !equalBlocks(snapshot.Body, want) {
		t.Fatalf("body=%+v, want %+v", snapshot.Body, want)
	}
	for kind, matches := range snapshot.LocatorMatches {
		if matches != expectedLocatorMatches(kind) {
			t.Fatalf("locator %s matched %d times", kind, matches)
		}
	}
}

// A body paragraph may repeat a setting label; the settings are read from their own
// controls, so the paragraph is neither truncated nor mistaken for a control.
func TestObserveKeepsABodyParagraphThatEqualsASettingLabel(t *testing.T) {
	observation := healthyObservation()
	observation["blocks"] = []map[string]any{
		{"kind": "text", "text": "전체공개", "ordinal": 0, "caption": "", "uploaded": false, "source": ""},
		{"kind": "text", "text": "식당", "ordinal": 0, "caption": "", "uploaded": false, "source": ""},
	}
	observation["image_count"] = 0
	snapshot, err := observeFake(t, &fakeEditor{observation: observation})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Body) != 2 || snapshot.Body[0].Text != "전체공개" || snapshot.Body[1].Text != "식당" {
		t.Fatalf("body=%+v", snapshot.Body)
	}
	if !snapshot.Visibility.Selected || snapshot.Visibility.Name != "전체공개" || snapshot.Category.ID != "17" {
		t.Fatalf("settings drifted: %+v %+v", snapshot.Visibility, snapshot.Category)
	}
}

func TestObserveReportsEveryAuthChallengeWithoutBypassing(t *testing.T) {
	for value, want := range map[string]AuthState{
		"ready": AuthReady, "login": AuthLogin, "captcha": AuthCaptcha,
		"two_factor": Auth2FA, "something-else": AuthLogin,
	} {
		observation := healthyObservation()
		observation["auth"] = value
		snapshot, err := observeFake(t, &fakeEditor{observation: observation})
		if err != nil {
			t.Fatalf("auth %s: %v", value, err)
		}
		if snapshot.Auth != want {
			t.Fatalf("auth %s => %s, want %s", value, snapshot.Auth, want)
		}
	}
}

func TestObserveReportsMissingRenamedAndDuplicateControls(t *testing.T) {
	for _, test := range []struct {
		name  string
		kind  MutationKind
		value int
	}{
		{name: "missing category", kind: MutationCategory, value: 0},
		{name: "renamed final surface", kind: MutationTags, value: 0},
		{name: "duplicate title", kind: MutationTitle, value: 2},
		{name: "duplicate visibility group", kind: MutationVisibility, value: 5},
	} {
		t.Run(test.name, func(t *testing.T) {
			observation := healthyObservation()
			matches := maps.Clone(observation["locator_matches"].(map[string]int))
			matches[string(test.kind)] = test.value
			observation["locator_matches"] = matches
			snapshot, err := observeFake(t, &fakeEditor{observation: observation})
			if err != nil {
				t.Fatal(err)
			}
			if snapshot.LocatorMatches[test.kind] != test.value {
				t.Fatalf("%s matched %d, want %d", test.kind, snapshot.LocatorMatches[test.kind], test.value)
			}
		})
	}
}

func TestObserveCountsUnversionedPublishLikeControls(t *testing.T) {
	observation := healthyObservation()
	observation["unversioned_publish_like"] = 2
	snapshot, err := observeFake(t, &fakeEditor{observation: observation})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.UnversionedPublishLikeControls != 2 {
		t.Fatalf("unversioned=%d", snapshot.UnversionedPublishLikeControls)
	}
}

func TestObservePoisonsTheRunWhenTheBoundTargetIsNotTheSoleNaverPage(t *testing.T) {
	for _, test := range []struct {
		name       string
		targetsFor func(call int) []map[string]any
		evaluate   bool
		emptyAX    bool
	}{
		{name: "second page appears mid-observation", targetsFor: func(call int) []map[string]any {
			if call < 3 {
				return nil
			}
			return []map[string]any{{"id": "page-1", "url": fakeWriterURL}, {"id": "page-2", "url": "https://blog.naver.com/alice"}}
		}},
		{name: "target id changes", targetsFor: func(call int) []map[string]any {
			if call < 3 {
				return nil
			}
			return []map[string]any{{"id": "page-9", "url": fakeWriterURL}}
		}},
		{name: "page leaves the writer", targetsFor: func(call int) []map[string]any {
			if call < 3 {
				return nil
			}
			return []map[string]any{{"id": "page-1", "url": "https://blog.naver.com/alice/1"}}
		}},
		{name: "native dialog blocks evaluation", evaluate: true},
		{name: "empty accessibility tree", emptyAX: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := observeFake(t, &fakeEditor{targetsFor: test.targetsFor, evaluateErr: test.evaluate, emptyAX: test.emptyAX})
			var poisoned PortError
			if err == nil || !asPortError(err, &poisoned) || poisoned.Kind != FailureEditorChanged {
				t.Fatalf("err=%v, want editor_changed PortError", err)
			}
		})
	}
}

func TestBindRefusesAPageOutsideTheNaverHostAllowlist(t *testing.T) {
	_, err := observeFake(t, &fakeEditor{targetsFor: func(int) []map[string]any {
		return []map[string]any{{"id": "page-1", "url": "https://example.com/"}}
	}})
	if err == nil || !strings.Contains(err.Error(), "outside approved Naver hosts") {
		t.Fatalf("err=%v", err)
	}
}

// Publisher.mutate fails closed when the token does not move, so the token must be derived
// from the observed content and never from a counter.
func TestSnapshotTokenTracksEveryObservedFieldAndNothingElse(t *testing.T) {
	first, err := observeFake(t, &fakeEditor{})
	if err != nil {
		t.Fatal(err)
	}
	again, err := observeFake(t, &fakeEditor{})
	if err != nil {
		t.Fatal(err)
	}
	if first.Token != again.Token {
		t.Fatal("an unchanged editor produced two different tokens")
	}
	for _, test := range []struct {
		name   string
		change func(map[string]any)
	}{
		{name: "title", change: func(o map[string]any) { o["title"] = "다른 제목" }},
		{name: "body", change: func(o map[string]any) {
			o["blocks"] = []map[string]any{{"kind": "text", "text": "바뀐 본문", "ordinal": 0, "caption": "", "uploaded": false, "source": ""}}
		}},
		{name: "image count", change: func(o map[string]any) { o["image_count"] = 3 }},
		{name: "tags", change: func(o map[string]any) { o["tags"] = []string{"다른태그"} }},
		{name: "category", change: func(o map[string]any) {
			o["category"] = map[string]any{"id": "19", "name": "일상", "selected": true}
		}},
		{name: "visibility", change: func(o map[string]any) {
			o["visibility"] = map[string]any{"id": "private", "name": "비공개", "selected": true}
		}},
		{name: "settings layer", change: func(o map[string]any) { o["settings_layer"] = false }},
		{name: "auth", change: func(o map[string]any) { o["auth"] = "captcha" }},
		{name: "locator matches", change: func(o map[string]any) {
			matches := maps.Clone(o["locator_matches"].(map[string]int))
			matches["tags"] = 0
			o["locator_matches"] = matches
		}},
		{name: "unversioned publish-like controls", change: func(o map[string]any) { o["unversioned_publish_like"] = 1 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			observation := healthyObservation()
			test.change(observation)
			changed, err := observeFake(t, &fakeEditor{observation: observation})
			if err != nil {
				t.Fatal(err)
			}
			if changed.Token == first.Token {
				t.Fatalf("changing %s left the token unchanged", test.name)
			}
		})
	}
}

func asPortError(err error, out *PortError) bool {
	portErr, ok := err.(PortError)
	if ok {
		*out = portErr
	}
	return ok
}

// callArgs unwraps the {value: …} argument envelope Runtime.callFunctionOn takes.
func callArgs(params map[string]any) (any, any, any) {
	list, _ := params["arguments"].([]any)
	value := func(index int) any {
		if index >= len(list) {
			return nil
		}
		entry, _ := list[index].(map[string]any)
		return entry["value"]
	}
	return value(0), value(1), value(2)
}

func toString(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprint(value)
}
