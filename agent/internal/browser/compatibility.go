package browser

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"
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
  const visibilityOptions = /^(전체공개|이웃공개|서로이웃공개|공개|비공개)$/;
  const visibility = layer ? [...layer.querySelectorAll('label,input,button,[role="radio"]')].some((node) => {
    const own = [...node.childNodes].filter((n) => n.nodeType === 3).map((n) => n.textContent).join('').replace(/\s+/g, ' ').trim();
    return visibilityOptions.test(own) || /공개|이웃|비공개|visibility/i.test([node.getAttribute('aria-label'), node.name, node.id].join(' '));
  }) : false;
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
	probeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	page, err := BindSinglePage(probeCtx, cdpURL)
	if err != nil {
		return CompatibilityEvidence{}, err
	}
	defer page.Close()
	domains, err := page.Domains(probeCtx)
	if err != nil {
		return CompatibilityEvidence{}, err
	}
	sort.Strings(domains)
	if err := page.DocumentPresent(probeCtx); err != nil {
		return CompatibilityEvidence{}, err
	}
	roles, err := page.AccessibilityRoles(probeCtx)
	if err != nil {
		return CompatibilityEvidence{}, err
	}
	var surface EditorSurface
	if err := page.Evaluate(probeCtx, compatibilityObservationScript, &surface); err != nil {
		return CompatibilityEvidence{}, err
	}
	current, err := page.Recheck(probeCtx)
	if err != nil || current != page.URL() {
		if err == nil {
			err = errors.New("dedicated browser target changed during compatibility probe")
		}
		return CompatibilityEvidence{}, fmt.Errorf("recheck dedicated browser target: %w", err)
	}
	return CompatibilityEvidence{
		BrowserProduct: page.BrowserProduct(), ProtocolVersion: page.ProtocolVersion(),
		TargetID: page.TargetID(), TargetURL: page.URL(),
		Domains: domains, AXRoles: roles, Editor: surface,
	}, nil
}
