package naver

import (
	"context"
	"strings"
)

// Reviewed driver functions. Each is a package-level constant from this signed release and
// receives manifest data only as structured CDP arguments, so no string the server sent can
// become part of an instruction. None of them types, uploads or activates anything: they
// resolve a versioned control, report how many matched, and return its live geometry.

// driverPointFn returns the viewport point of the single element that backs one text target.
// The caller must treat the point as valid only for the action it is about to take.
const driverPointFn = `function (target, ordinal) {
  const point = (node) => {
    if (!node) return {matches: 0, x: 0, y: 0};
    const box = node.getBoundingClientRect();
    if (box.width <= 0 || box.height <= 0) return {matches: 0, x: 0, y: 0};
    return {matches: 1, x: Math.round(box.left + box.width / 2), y: Math.round(box.top + box.height / 2)};
  };
  if (target === 'title') {
    const nodes = document.querySelectorAll('.se-component.se-documentTitle .se-title-text');
    return nodes.length === 1 ? point(nodes[0]) : {matches: nodes.length, x: 0, y: 0};
  }
  if (target === 'body_end') {
    const body = document.querySelector('.se-body.__se-body');
    if (!body) return {matches: 0, x: 0, y: 0};
    const paragraphs = body.querySelectorAll('.se-component .se-module-text.__se-unit p.se-text-paragraph');
    if (paragraphs.length === 0) return {matches: 0, x: 0, y: 0};
    return point(paragraphs[paragraphs.length - 1]);
  }
  if (target === 'image_caption') {
    const images = document.querySelectorAll('.se-body.__se-body .se-component.se-image');
    const image = images[ordinal];
    if (!image) return {matches: 0, x: 0, y: 0};
    const captions = image.querySelectorAll('.se-module-text.se-caption');
    return captions.length === 1 ? point(captions[0]) : {matches: captions.length, x: 0, y: 0};
  }
  if (target === 'tag_input') {
    const layer = document.querySelector('div[class^="layer_popup__"][class*="is_show__"]');
    if (!layer) return {matches: 0, x: 0, y: 0};
    const inputs = layer.querySelectorAll('input[class^="tag_input__"]');
    return inputs.length === 1 ? point(inputs[0]) : {matches: inputs.length, x: 0, y: 0};
  }
  return {matches: 0, x: 0, y: 0};
}`

// driverActivateFn clicks the single element that backs one versioned control. It refuses
// unless exactly one element matches, and it never touches a publish-like control: the final
// activation lives behind the commit fence, not here.
const driverActivateFn = `function (control, id) {
  const layer = () => document.querySelector('div[class^="layer_popup__"][class*="is_show__"]');
  const one = (nodes) => (nodes.length === 1 ? nodes[0] : null);
  let node = null;
  let matches = 0;
  const pick = (list) => { matches = list.length; node = one(list); };
  switch (control) {
    case 'settings_open': {
      if (layer()) return {matches: 1, activated: false, already: true};
      pick([...document.querySelectorAll('button[class^="publish_btn__"]')]);
      break;
    }
    case 'category_open': {
      const shown = layer();
      if (!shown) return {matches: 0, activated: false};
      const buttons = [...shown.querySelectorAll('button[aria-label="카테고리 목록 버튼"]')];
      if (buttons.length === 1 && buttons[0].getAttribute('aria-expanded') === 'true') {
        return {matches: 1, activated: false, already: true};
      }
      pick(buttons);
      break;
    }
    case 'category': {
      const shown = layer();
      if (!shown) return {matches: 0, activated: false};
      const radios = [...shown.querySelectorAll('input[type=radio][data-testid="categoryBtn_' + String(id).replace(/[^A-Za-z0-9_-]/g, '') + '"]')];
      matches = radios.length;
      node = radios.length === 1 ? (radios[0].closest('li,div,span') || {}).querySelector('label[class^="radio_label__"]') : null;
      break;
    }
    case 'visibility': {
      const shown = layer();
      if (!shown) return {matches: 0, activated: false};
      const radios = [...shown.querySelectorAll('input[type=radio][data-testid="openType_' + String(id).replace(/[^0-9]/g, '') + '"]')];
      matches = radios.length;
      node = radios.length === 1 ? (radios[0].closest('li,div,span') || {}).querySelector('label[class^="radio_label__"]') : null;
      break;
    }
    case 'format_menu': pick([...document.querySelectorAll('button.se-text-format-toolbar-button')]); break;
    case 'format_text': pick([...document.querySelectorAll('button.se-toolbar-option-text-format-text-button')]); break;
    case 'format_heading': pick([...document.querySelectorAll('button.se-toolbar-option-text-format-sectionTitle-button')]); break;
    case 'format_quote': pick([...document.querySelectorAll('button.se-toolbar-option-text-format-quotation-button')]); break;
    case 'list_menu': pick([...document.querySelectorAll('button.se-list-bullet-toolbar-button')]); break;
    case 'list_bullet': pick([...document.querySelectorAll('button.se-toolbar-option-list-bullet-button')]); break;
    case 'image_add': pick([...document.querySelectorAll('button.se-image-toolbar-button')]); break;
    default: return {matches: 0, activated: false};
  }
  if (!node || matches !== 1) return {matches, activated: false};
  node.click();
  return {matches: 1, activated: true};
}`

type driverPoint struct {
	Matches int     `json:"matches"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
}

type driverActivation struct {
	Matches   int  `json:"matches"`
	Activated bool `json:"activated"`
	Already   bool `json:"already"`
}

// resolveAndClick places the caret by clicking the live geometry of the one element the
// versioned locator just resolved. The point is used immediately and never retained.
func (p *CDPPort) resolveAndClick(ctx context.Context, target string, ordinal int) error {
	var point driverPoint
	if err := p.page.CallFunction(ctx, driverPointFn, []any{target, ordinal}, &point); err != nil {
		return PortError{Kind: FailureEditorChanged}
	}
	if point.Matches != 1 {
		return PortError{Kind: FailureEditorChanged}
	}
	if err := p.page.ClickPoint(ctx, point.X, point.Y); err != nil {
		return PortError{Kind: FailureEditorChanged}
	}
	return nil
}

// activate clicks one versioned control, refusing unless exactly one element matched.
func (p *CDPPort) activate(ctx context.Context, control, id string) error {
	var activation driverActivation
	if err := p.page.CallFunction(ctx, driverActivateFn, []any{control, id}, &activation); err != nil {
		return PortError{Kind: FailureEditorChanged}
	}
	if activation.Matches != 1 || (!activation.Activated && !activation.Already) {
		return PortError{Kind: FailureEditorChanged}
	}
	return nil
}

// typeText enters manifest text verbatim. Text that resembles an instruction or a settings
// label is inert here: it crosses as an argument and is inserted at the caret.
func (p *CDPPort) typeText(ctx context.Context, text string) error {
	if strings.TrimSpace(text) == "" {
		return PortError{Kind: FailureSafe}
	}
	if err := p.page.InsertText(ctx, text); err != nil {
		return PortError{Kind: FailureEditorChanged}
	}
	return nil
}
