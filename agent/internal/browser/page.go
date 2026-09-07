package browser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/coder/websocket"
)

// Page is one CDP connection bound to the single dedicated browser page. Its target id and
// page WebSocket endpoint are pinned at bind time and reproved by Recheck, so evidence can
// never be assembled across two documents or two targets.
//
// Page deliberately carries no editor knowledge. It exposes only the narrow operations the
// versioned publisher needs — URL reads, one reviewed script evaluation, a full
// accessibility snapshot, a document probe and a capability list — and the reviewed
// SmartEditor locator set stays with the driver that owns it.
type Page struct {
	cdpURL   string
	conn     *websocket.Conn
	client   *cdpClient
	target   pageTarget
	product  string
	protocol string
}

// BindSinglePage requires exactly one dedicated page, records the browser version before
// attaching (Browser.getVersion is browser-scoped and must not carry a page session), and
// attaches a flat session to that one target.
func BindSinglePage(ctx context.Context, cdpURL string) (*Page, error) {
	target, err := discoverSinglePage(ctx, cdpURL)
	if err != nil {
		return nil, err
	}
	conn, _, err := websocket.Dial(ctx, cdpURL, &websocket.DialOptions{HTTPClient: noProxyHTTPClient(10 * time.Second)})
	if err != nil {
		return nil, fmt.Errorf("connect dedicated browser: %w", err)
	}
	conn.SetReadLimit(1 << 22)
	page := &Page{cdpURL: cdpURL, conn: conn, client: &cdpClient{conn: conn}, target: target}
	var version struct {
		ProtocolVersion string `json:"protocolVersion"`
		Product         string `json:"product"`
	}
	if err := page.client.call(ctx, "Browser.getVersion", nil, &version); err != nil {
		conn.CloseNow()
		return nil, err
	}
	page.product, page.protocol = version.Product, version.ProtocolVersion
	var attached struct {
		SessionID string `json:"sessionId"`
	}
	if err := page.client.call(ctx, "Target.attachToTarget", map[string]any{"targetId": target.ID, "flatten": true}, &attached); err != nil || attached.SessionID == "" {
		conn.CloseNow()
		if err == nil {
			err = errors.New("attach returned no session")
		}
		return nil, fmt.Errorf("attach selected browser page: %w", err)
	}
	page.client.sessionID = attached.SessionID
	return page, nil
}

func (p *Page) TargetID() string        { return p.target.ID }
func (p *Page) URL() string             { return p.target.URL }
func (p *Page) BrowserProduct() string  { return p.product }
func (p *Page) ProtocolVersion() string { return p.protocol }

// Recheck proves the bound target is still the sole dedicated page and returns its current
// URL. A different id or page WebSocket endpoint, or a second page, poisons the run.
func (p *Page) Recheck(ctx context.Context) (string, error) {
	current, err := discoverSinglePage(ctx, p.cdpURL)
	if err != nil {
		return "", err
	}
	if current.ID != p.target.ID || current.WebSocketDebuggerURL != p.target.WebSocketDebuggerURL {
		return "", errors.New("dedicated browser target changed")
	}
	return current.URL, nil
}

// Evaluate runs one reviewed script from this signed agent release and decodes its data
// result. Callers pass a package-level constant; no expression originates off this Mac.
func (p *Page) Evaluate(ctx context.Context, script string, out any) error {
	var evaluated struct {
		Result struct {
			Value json.RawMessage `json:"value"`
		} `json:"result"`
	}
	if err := p.client.call(ctx, "Runtime.evaluate", map[string]any{"expression": script, "returnByValue": true, "awaitPromise": true}, &evaluated); err != nil {
		return err
	}
	if len(evaluated.Result.Value) == 0 {
		return errors.New("dedicated browser page returned no observation")
	}
	return json.Unmarshal(evaluated.Result.Value, out)
}

// Domains lists the CDP domains the attached browser actually implements.
func (p *Page) Domains(ctx context.Context) ([]string, error) {
	var schema struct {
		Domains []struct {
			Name string `json:"name"`
		} `json:"domains"`
	}
	if err := p.client.call(ctx, "Schema.getDomains", nil, &schema); err != nil {
		return nil, err
	}
	domains := make([]string, 0, len(schema.Domains))
	for _, domain := range schema.Domains {
		domains = append(domains, domain.Name)
	}
	return domains, nil
}

// DocumentPresent proves the attached page has a document root to observe.
func (p *Page) DocumentPresent(ctx context.Context) error {
	var document struct {
		Root json.RawMessage `json:"root"`
	}
	if err := p.client.call(ctx, "DOM.getDocument", map[string]any{"depth": 0}, &document); err != nil {
		return err
	}
	if len(document.Root) == 0 {
		return errors.New("DOM.getDocument returned no root")
	}
	return nil
}

// AccessibilityRoles returns the roles of the current full accessibility tree. An empty
// tree is never acceptable evidence.
func (p *Page) AccessibilityRoles(ctx context.Context) ([]string, error) {
	if err := p.client.call(ctx, "Accessibility.enable", nil, nil); err != nil {
		return nil, err
	}
	var tree struct {
		Nodes []struct {
			Role struct {
				Value any `json:"value"`
			} `json:"role"`
		} `json:"nodes"`
	}
	if err := p.client.call(ctx, "Accessibility.getFullAXTree", nil, &tree); err != nil {
		return nil, err
	}
	roles := make([]string, 0, len(tree.Nodes))
	for _, node := range tree.Nodes {
		if role, ok := node.Role.Value.(string); ok && role != "" {
			roles = append(roles, role)
		}
	}
	if len(roles) == 0 {
		return nil, errors.New("browser returned an empty accessibility tree")
	}
	return roles, nil
}

// CallFunction invokes one reviewed function declaration from this signed agent release
// against the page's document, passing manifest data as structured CDP arguments.
//
// Data never becomes code here: the declaration is a package-level constant and every value
// crosses as a typed argument, so no manifest string is ever concatenated into an
// expression, and the page cannot receive an instruction it was not shipped with.
func (p *Page) CallFunction(ctx context.Context, declaration string, arguments []any, out any) error {
	var document struct {
		Result struct {
			ObjectID string `json:"objectId"`
		} `json:"result"`
	}
	if err := p.client.call(ctx, "Runtime.evaluate", map[string]any{"expression": "document"}, &document); err != nil {
		return err
	}
	if document.Result.ObjectID == "" {
		return errors.New("dedicated browser page exposed no document")
	}
	call := make([]map[string]any, 0, len(arguments))
	for _, argument := range arguments {
		call = append(call, map[string]any{"value": argument})
	}
	var evaluated struct {
		Result struct {
			Value json.RawMessage `json:"value"`
		} `json:"result"`
		ExceptionDetails json.RawMessage `json:"exceptionDetails"`
	}
	if err := p.client.call(ctx, "Runtime.callFunctionOn", map[string]any{
		"functionDeclaration": declaration,
		"objectId":            document.Result.ObjectID,
		"arguments":           call,
		"returnByValue":       true,
		"awaitPromise":        true,
	}, &evaluated); err != nil {
		return err
	}
	if len(evaluated.ExceptionDetails) > 0 {
		return errors.New("reviewed driver function failed on the dedicated page")
	}
	if out == nil {
		return nil
	}
	if len(evaluated.Result.Value) == 0 {
		return errors.New("reviewed driver function returned no value")
	}
	return json.Unmarshal(evaluated.Result.Value, out)
}

// ClickPoint presses and releases the primary button at one viewport point. The caller must
// have just resolved that point from a versioned semantic locator's own live geometry and
// must not retain it (PUBLISH-36).
func (p *Page) ClickPoint(ctx context.Context, x, y float64) error {
	for _, phase := range []string{"mousePressed", "mouseReleased"} {
		if err := p.client.call(ctx, "Input.dispatchMouseEvent", map[string]any{
			"type": phase, "x": x, "y": y, "button": "left", "clickCount": 1,
		}, nil); err != nil {
			return err
		}
	}
	return nil
}

// InsertText enters manifest text at the page's current caret. It is not a keystroke script:
// the text crosses as data and no key sequence is synthesised from it.
func (p *Page) InsertText(ctx context.Context, text string) error {
	return p.client.call(ctx, "Input.insertText", map[string]any{"text": text}, nil)
}

// PressEnter sends the one editing key SmartEditor needs to open the next body paragraph
// or commit the current tag. It is never used to submit: PUBLISH-21 fails closed on
// keyboard submission.
func (p *Page) PressEnter(ctx context.Context) error {
	for _, phase := range []string{"rawKeyDown", "keyUp"} {
		if err := p.client.call(ctx, "Input.dispatchKeyEvent", map[string]any{
			"type": phase, "windowsVirtualKeyCode": 13, "key": "Enter", "code": "Enter",
		}, nil); err != nil {
			return err
		}
	}
	return nil
}

// InterceptFileChooser keeps the browser from ever opening a native file dialog; the driver
// supplies files through SetFileInputFiles instead (PUBLISH-21).
func (p *Page) InterceptFileChooser(ctx context.Context) error {
	return p.client.call(ctx, "Page.setInterceptFileChooserDialog", map[string]any{"enabled": true}, nil)
}

// SetFileInputFiles hands exactly the enumerated current-job paths to the resolved file
// input. It is the only filesystem-touching operation the driver exposes.
func (p *Page) SetFileInputFiles(ctx context.Context, selector string, files []string) error {
	var document struct {
		Root struct {
			NodeID int `json:"nodeId"`
		} `json:"root"`
	}
	if err := p.client.call(ctx, "DOM.getDocument", map[string]any{"depth": 0}, &document); err != nil {
		return err
	}
	var found struct {
		NodeID int `json:"nodeId"`
	}
	if err := p.client.call(ctx, "DOM.querySelector", map[string]any{"nodeId": document.Root.NodeID, "selector": selector}, &found); err != nil {
		return err
	}
	if found.NodeID == 0 {
		return errors.New("dedicated page exposed no upload control")
	}
	return p.client.call(ctx, "DOM.setFileInputFiles", map[string]any{"files": files, "nodeId": found.NodeID}, nil)
}

// Navigate moves the bound target to one approved Naver URL built by reviewed local code.
func (p *Page) Navigate(ctx context.Context, destination string) error {
	var navigation struct {
		ErrorText string `json:"errorText"`
	}
	if err := p.client.call(ctx, "Page.enable", nil, nil); err != nil {
		return err
	}
	if err := p.client.call(ctx, "Page.navigate", map[string]any{"url": destination}, &navigation); err != nil {
		return err
	}
	if navigation.ErrorText != "" {
		return errors.New("browser refused Naver navigation")
	}
	return nil
}

func (p *Page) Close() error {
	if p == nil || p.conn == nil {
		return nil
	}
	return p.conn.CloseNow()
}
