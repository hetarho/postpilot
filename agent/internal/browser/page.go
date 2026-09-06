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

func (p *Page) Close() error {
	if p == nil || p.conn == nil {
		return nil
	}
	return p.conn.CloseNow()
}
