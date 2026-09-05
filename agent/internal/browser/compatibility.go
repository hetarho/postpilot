package browser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/coder/websocket"
)

// CompatibilityEvidence is the read-only browser evidence consumed by the versioned Naver
// probe. It contains no cookie, profile path, CDP URL, selector or executable instruction.
type CompatibilityEvidence struct {
	BrowserProduct  string
	ProtocolVersion string
	TargetID        string
	TargetURL       string
	Domains         []string
	AXRoles         []string
	Editor          EditorSurface
}

// EditorSurface is a fixed projection produced by reviewed local code. The naver package
// validates this against its signed-release manifest rather than accepting remote locators.
type EditorSurface struct {
	EditorRoot       bool `json:"editor_root"`
	TitleEditor      bool `json:"title_editor"`
	BodyEditor       bool `json:"body_editor"`
	ImageControl     bool `json:"image_control"`
	SettingsLayer    bool `json:"settings_layer"`
	CategoryControl  bool `json:"category_control"`
	CategoryOptions  int  `json:"category_options"`
	VisibilityChoice bool `json:"visibility_choice"`
	TagsControl      bool `json:"tags_control"`
	FinalControl     bool `json:"final_control"`
	ReadbackSurface  bool `json:"readback_surface"`
}

const compatibilityObservationScript = `(() => {
  const layer = document.querySelector('div[class^="layer_popup__"][class*="is_show__"]');
  const categories = layer ? layer.querySelectorAll('input[data-testid^="categoryBtn_"], [data-category-id], [data-category-no], [role="option"], [role="menuitemradio"]') : [];
  const visibility = layer ? [...layer.querySelectorAll('input,button,[role="radio"]')].some((node) => /공개|이웃|비공개|visibility/i.test([node.getAttribute('aria-label'), node.textContent, node.name, node.id].join(' '))) : false;
  const tags = layer ? [...layer.querySelectorAll('input,textarea,[contenteditable="true"]')].some((node) => /태그|tag/i.test([node.getAttribute('aria-label'), node.placeholder, node.name, node.id].join(' '))) : false;
  return {
    editor_root: Boolean(document.querySelector('.blog_editor')),
    title_editor: Boolean(document.querySelector('.se-component.se-documentTitle .se-title-text')),
    body_editor: Boolean(document.querySelector('.se-body.__se-body')),
    image_control: Boolean(document.querySelector('button.se-image-toolbar-button')),
    settings_layer: Boolean(layer),
    category_control: Boolean(layer?.querySelector('button[aria-label="카테고리 목록 버튼"]')),
    category_options: categories.length,
    visibility_choice: visibility,
    tags_control: tags,
    final_control: layer?.querySelectorAll('button[class^="confirm_btn__"]').length === 1,
    readback_surface: location.hostname === 'blog.naver.com' && Boolean(document.body)
  };
})()`

// InspectCompatibility proves the fixed CDP capabilities and editor surface without typing,
// uploading, publishing, reading cookies/storage, or accepting a caller-supplied script.
func InspectCompatibility(ctx context.Context, cdpURL string) (CompatibilityEvidence, error) {
	target, err := discoverSinglePage(ctx, cdpURL)
	if err != nil {
		return CompatibilityEvidence{}, err
	}
	probeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(probeCtx, cdpURL, &websocket.DialOptions{HTTPClient: noProxyHTTPClient(10 * time.Second)})
	if err != nil {
		return CompatibilityEvidence{}, fmt.Errorf("connect dedicated browser: %w", err)
	}
	defer conn.CloseNow()
	conn.SetReadLimit(1 << 20)
	client := &cdpClient{conn: conn}
	var version struct {
		ProtocolVersion string `json:"protocolVersion"`
		Product         string `json:"product"`
	}
	if err := client.call(probeCtx, "Browser.getVersion", nil, &version); err != nil {
		return CompatibilityEvidence{}, err
	}
	var attached struct {
		SessionID string `json:"sessionId"`
	}
	if err := client.call(probeCtx, "Target.attachToTarget", map[string]any{"targetId": target.ID, "flatten": true}, &attached); err != nil || attached.SessionID == "" {
		if err == nil {
			err = errors.New("attach returned no session")
		}
		return CompatibilityEvidence{}, fmt.Errorf("attach selected browser page: %w", err)
	}
	client.sessionID = attached.SessionID
	var schema struct {
		Domains []struct {
			Name string `json:"name"`
		} `json:"domains"`
	}
	if err := client.call(probeCtx, "Schema.getDomains", nil, &schema); err != nil {
		return CompatibilityEvidence{}, err
	}
	domains := make([]string, 0, len(schema.Domains))
	for _, domain := range schema.Domains {
		domains = append(domains, domain.Name)
	}
	sort.Strings(domains)
	var document struct {
		Root json.RawMessage `json:"root"`
	}
	if err := client.call(probeCtx, "DOM.getDocument", map[string]any{"depth": 0}, &document); err != nil || len(document.Root) == 0 {
		if err == nil {
			err = errors.New("DOM.getDocument returned no root")
		}
		return CompatibilityEvidence{}, err
	}
	if err := client.call(probeCtx, "Accessibility.enable", nil, nil); err != nil {
		return CompatibilityEvidence{}, err
	}
	var tree struct {
		Nodes []struct {
			Role struct {
				Value any `json:"value"`
			} `json:"role"`
		} `json:"nodes"`
	}
	if err := client.call(probeCtx, "Accessibility.getFullAXTree", nil, &tree); err != nil {
		return CompatibilityEvidence{}, err
	}
	roles := make([]string, 0, len(tree.Nodes))
	for _, node := range tree.Nodes {
		if role, ok := node.Role.Value.(string); ok && role != "" {
			roles = append(roles, role)
		}
	}
	if len(roles) == 0 {
		return CompatibilityEvidence{}, errors.New("browser returned an empty accessibility tree")
	}
	var evaluated struct {
		Result struct {
			Value EditorSurface `json:"value"`
		} `json:"result"`
	}
	if err := client.call(probeCtx, "Runtime.evaluate", map[string]any{"expression": compatibilityObservationScript, "returnByValue": true, "awaitPromise": true}, &evaluated); err != nil {
		return CompatibilityEvidence{}, err
	}
	confirmed, err := discoverSinglePage(probeCtx, cdpURL)
	if err != nil || confirmed.ID != target.ID || confirmed.WebSocketDebuggerURL != target.WebSocketDebuggerURL || confirmed.URL != target.URL {
		if err == nil {
			err = errors.New("dedicated browser target changed during compatibility probe")
		}
		return CompatibilityEvidence{}, fmt.Errorf("recheck dedicated browser target: %w", err)
	}
	return CompatibilityEvidence{BrowserProduct: version.Product, ProtocolVersion: version.ProtocolVersion, TargetID: target.ID, TargetURL: target.URL, Domains: domains, AXRoles: roles, Editor: evaluated.Result.Value}, nil
}
