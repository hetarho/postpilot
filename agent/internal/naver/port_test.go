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
			{"kind": "image", "text": "", "ordinal": 0, "caption": "캡션", "uploaded": true, "source": "a.jpg"},
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
	final     func(allowed []string) driverFinalControl
	point     func(target string, ordinal int) driverPoint
	activate  func(control, id string) driverActivation
	state     func() driverSettingsState
	setting   func(control, id, name string) driverSetting
	paragraph func(index int) driverParagraphState
	// deafList makes an activated list control leave the paragraph unconverted, and
	// deafEnter makes Enter inside a list add no item, so the port's post-conversion
	// assertions can be exercised.
	deafList  bool
	deafEnter bool
	// The upload surface. fileInputMissing models the hidden input never appearing after
	// the image button's click, which is a live failure mode: the input is created on
	// demand by that click.
	fileInputMissing bool
	uploaded         []string
	// The image surface after an upload: how many images the editor holds and how many have
	// finished processing. baseImages seeds the count so the fake starts consistent with its
	// observation, and the knobs model the three ways an upload fails to settle.
	baseImages       int
	imageAddsNone    bool
	imageAddsTwo     bool
	imageNeverSettle bool

	mu               sync.Mutex
	calls            int
	inputs           []string
	pendingControl   string
	pendingID        string
	pendingName      string
	pendingText      string
	categoryExpanded bool
	// The fake's model of the one addressed paragraph: a conversion switches its component
	// kind, which is exactly what the port re-observes after activating a control.
	convertedKind    string
	inList           bool
	listItems        int
	pendingEmptyItem bool
	bodyEndEmpty     bool
	// The post-publish surface. The page shows the writer until the fence's activation, then
	// the permalink Naver lands on, then whatever the driver navigates to — which is how
	// the readback's own URL predicate gets exercised.
	permalink   string
	navigatedTo string
	activations int
	finalReads  int
	postView    map[string]any
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

func (editor *fakeEditor) paragraphState(target string, index int) driverParagraphState {
	if editor.paragraph != nil {
		return editor.paragraph(index)
	}
	editor.mu.Lock()
	defer editor.mu.Unlock()
	kind := "se-text"
	if editor.convertedKind != "" {
		kind = editor.convertedKind
	}
	state := driverParagraphState{Matches: 1, Total: 1, Kind: kind, InList: editor.inList, ListItems: editor.listItems}
	// The append target is empty exactly when the last thing that happened there was the
	// Enter that committed the previous write.
	if target == "body_end" {
		state.Empty = editor.bodyEndEmpty
	}
	return state
}

// convert models what the live editor does when one of the paragraph controls is activated:
// 소제목 and 인용구 move the caret's paragraph into its own component, and 글머리 기호 turns it
// into the first item of a list in place (verified live 2026-09-10).
func (editor *fakeEditor) convert(control string) {
	editor.mu.Lock()
	defer editor.mu.Unlock()
	switch control {
	case "format_heading":
		editor.convertedKind = "se-sectionTitle"
	case "format_quote":
		editor.convertedKind = "se-quotation"
	case "list_bullet":
		if editor.deafList {
			return
		}
		editor.inList, editor.listItems = true, 1
	}
}

// growList models Enter inside a list: it appends another LI, and a SECOND Enter on that
// still-empty item drops it and leaves the list — which is how the last item's trailing word
// is committed without changing the list. The addressed paragraph stays in the list either
// way, so inList is not cleared.
func (editor *fakeEditor) growList() {
	editor.mu.Lock()
	defer editor.mu.Unlock()
	if !editor.inList {
		editor.bodyEndEmpty = true
		return
	}
	if editor.deafEnter {
		return
	}
	if editor.pendingEmptyItem {
		editor.listItems--
		editor.pendingEmptyItem = false
		return
	}
	editor.listItems++
	editor.pendingEmptyItem = true
}

// fillList models text typed into the item the last Enter opened.
func (editor *fakeEditor) fillList() {
	editor.mu.Lock()
	defer editor.mu.Unlock()
	editor.pendingEmptyItem = false
	editor.bodyEndEmpty = false
}

// evaluateFailsFrom makes every later observation fail, which is how a browser loss or a
// process death reaches the port: the page stops answering.
func (port *CDPPort) evaluateFailsFrom(editor *fakeEditor) {
	editor.mu.Lock()
	defer editor.mu.Unlock()
	editor.evaluateErr = true
}

func (editor *fakeEditor) finalControlState(allowed []string) driverFinalControl {
	if editor.final != nil {
		return editor.final(allowed)
	}
	open, _ := editor.observation["settings_layer"].(bool)
	if !open {
		return driverFinalControl{}
	}
	name := "발행"
	if len(allowed) > 0 {
		name = allowed[0]
	}
	return driverFinalControl{Matches: 1, LayerMatches: 1, Name: name, Versioned: 1, Actionable: true, X: 90, Y: 120}
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
			editor.mu.Lock()
			current := fakeWriterURL
			if editor.activations > 0 && editor.permalink != "" {
				current = editor.permalink
			}
			if editor.navigatedTo != "" {
				current = editor.navigatedTo
			}
			editor.mu.Unlock()
			targets := []map[string]any{page("page-1", current)}
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
					} else if expression, _ := call.Params["expression"].(string); strings.Contains(expression, "category_title") {
						result = map[string]any{"result": map[string]any{"value": editor.postView}}
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
					case strings.Contains(declaration, "seOnePublishBtn"):
						allowed := []string{}
						if list, ok := first.([]any); ok {
							for _, value := range list {
								allowed = append(allowed, toString(value))
							}
						}
						editor.record("final:" + strings.Join(allowed, "|"))
						editor.mu.Lock()
						editor.finalReads++
						editor.mu.Unlock()
						result = map[string]any{"result": map[string]any{"value": editor.finalControlState(allowed)}}
					case strings.Contains(declaration, "settled += 1"):
						editor.mu.Lock()
						added := len(editor.uploaded)
						editor.mu.Unlock()
						images := editor.baseImages
						switch {
						case editor.imageAddsNone:
						case editor.imageAddsTwo:
							images += added * 2
						default:
							images += added
						}
						settled := images
						if editor.imageNeverSettle && added > 0 {
							settled = images - 1
						}
						result = map[string]any{"result": map[string]any{"value": driverImageState{Images: images, Settled: settled}}}
					case strings.Contains(declaration, "list_items: list ?"):
						index, _ := second.(float64)
						result = map[string]any{"result": map[string]any{"value": editor.paragraphState(toString(first), int(index))}}
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
						if resolved.Matches == 1 && (resolved.Activated || resolved.Already) {
							editor.convert(toString(first))
						}
						result = map[string]any{"result": map[string]any{"value": resolved}}
					}
				case "Page.setInterceptFileChooserDialog":
					if enabled, _ := call.Params["enabled"].(bool); enabled {
						editor.record("intercept")
					}
				case "Page.enable":
				case "Page.navigate":
					destination, _ := call.Params["url"].(string)
					editor.mu.Lock()
					editor.navigatedTo = destination
					editor.mu.Unlock()
					editor.record("navigate:" + destination)
				case "DOM.getDocument":
					result = map[string]any{"root": map[string]any{"nodeId": 1}}
				case "DOM.querySelector":
					selector, _ := call.Params["selector"].(string)
					nodeID := 0
					// The hidden input exists only once the image button has been clicked,
					// which is what makes counting it as the upload locator impossible.
					if selector == "input[type=file]#hidden-file" && !editor.fileInputMissing && slices.Contains(editor.recorded(), "point:image_add") {
						nodeID = 7
					}
					editor.record("query:" + selector)
					result = map[string]any{"nodeId": nodeID}
				case "DOM.setFileInputFiles":
					files, _ := call.Params["files"].([]any)
					names := make([]string, 0, len(files))
					for _, file := range files {
						names = append(names, toString(file))
					}
					editor.mu.Lock()
					editor.uploaded = append(editor.uploaded, names...)
					editor.mu.Unlock()
					editor.record("upload:" + strings.Join(names, "|"))
				case "Input.dispatchMouseEvent":
					if phase, _ := call.Params["type"].(string); phase == "mousePressed" {
						editor.record("click")
					} else if phase == "mouseReleased" {
						editor.applyPendingClick()
						editor.mu.Lock()
						// The second resolve of the final control is ActivateFinal's, so the
						// click that follows it is the one activation.
						if editor.finalReads >= 2 && editor.permalink != "" {
							editor.activations++
						}
						editor.mu.Unlock()
					}
				case "Input.insertText":
					text, _ := call.Params["text"].(string)
					editor.record("text:" + text)
					editor.rememberText(text)
					editor.fillList()
				case "Input.dispatchKeyEvent":
					if phase, _ := call.Params["type"].(string); phase == "rawKeyDown" {
						editor.record("enter")
						editor.commitTag()
						editor.growList()
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
		{Kind: SemanticImage, Ordinal: 0, Caption: "캡션", Uploaded: true},
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
