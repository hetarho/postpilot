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
  // caretEnd aims at the paragraph's trailing edge rather than its centre. A box centre
  // lands at the text's end only while the text stops short of it; on a paragraph that
  // fills or wraps its line the centre lands mid-text and the next insertion would split
  // it. Verified live 2026-09-07.
  const caretEnd = (node) => {
    if (!node) return {matches: 0, x: 0, y: 0};
    const box = node.getBoundingClientRect();
    if (box.width <= 0 || box.height <= 0) return {matches: 0, x: 0, y: 0};
    return {matches: 1, x: Math.round(box.left + box.width * 0.98), y: Math.round(box.bottom - box.height * 0.25)};
  };
  if (target === 'title') {
    const nodes = document.querySelectorAll('.se-component.se-documentTitle .se-title-text');
    return nodes.length === 1 ? point(nodes[0]) : {matches: nodes.length, x: 0, y: 0};
  }
  if (target === 'body_end') {
    const body = document.querySelector('.se-body.__se-body');
    if (!body) return {matches: 0, x: 0, y: 0};
    // The document title is a .se-component INSIDE .se-body, and an image's caption is a
    // paragraph too, so an unqualified descendant query can put the caret in the title or
    // under a photo. Only a text component's own paragraphs are body text.
    const paragraphs = [...body.querySelectorAll('.se-component.se-text .se-module-text.__se-unit p.se-text-paragraph')];
    if (paragraphs.length === 0) return {matches: 0, x: 0, y: 0};
    return caretEnd(paragraphs[paragraphs.length - 1]);
  }
  if (target === 'image_caption') {
    const images = document.querySelectorAll('.se-body.__se-body .se-component.se-image');
    const image = images[ordinal];
    if (!image) return {matches: 0, x: 0, y: 0};
    const captions = image.querySelectorAll('.se-module-text.se-caption');
    return captions.length === 1 ? point(captions[0]) : {matches: captions.length, x: 0, y: 0};
  }
  if (target === 'settings_open') {
    const layers = document.querySelectorAll('div[class^="layer_popup__"][class*="is_show__"]');
    if (layers.length !== 0) return {matches: 0, x: 0, y: 0};
    const buttons = document.querySelectorAll('button[class^="publish_btn__"]');
    return buttons.length === 1 ? point(buttons[0]) : {matches: buttons.length, x: 0, y: 0};
  }
  if (target === 'tag_input') {
    const layers = document.querySelectorAll('div[class^="layer_popup__"][class*="is_show__"]');
    if (layers.length !== 1) return {matches: layers.length, x: 0, y: 0};
    const layer = layers[0];
    const inputs = layer.querySelectorAll('input[class^="tag_input__"]');
    return inputs.length === 1 ? point(inputs[0]) : {matches: inputs.length, x: 0, y: 0};
  }
  return {matches: 0, x: 0, y: 0};
}`

// driverSettingsStateFn observes only the three fixed setting surfaces within exactly one
// shown layer. It lets every settings action re-prove that the layer did not close or
// duplicate between full Publisher observations.
const driverSettingsStateFn = `function () {
  const layers = document.querySelectorAll('div[class^="layer_popup__"][class*="is_show__"]');
  if (layers.length !== 1) {
    return {layer_matches: layers.length, tags: 0, category: 0, visibility: 0};
  }
  const layer = layers[0];
  return {
    layer_matches: 1,
    tags: layer.querySelectorAll('input[class^="tag_input__"]').length,
    category: layer.querySelectorAll('button[class^="selectbox_button__"]').length,
    visibility: layer.querySelectorAll('input[type=radio][data-testid^="openType_"]').length
  };
}`

// driverSettingFn resolves one category or visibility control inside exactly one shown
// layer. IDs and names arrive as data arguments. The function never activates a node; it
// only returns current state and live geometry for one immediate native pointer event.
const driverSettingFn = `function (control, id, name) {
  const empty = (matches = 0) => ({matches, group_matches: 0, actionable: false, x: 0, y: 0, expanded: false, checked: false, name_matches: false});
  const point = (node, value) => {
    if (!node) return value;
    const box = node.getBoundingClientRect();
    if (box.width <= 0 || box.height <= 0) return value;
    return {...value, actionable: true, x: Math.round(box.left + box.width / 2), y: Math.round(box.top + box.height / 2)};
  };
  const norm = (value) => String(value == null ? '' : value).replace(/\s+/g, ' ').trim();
  const layers = document.querySelectorAll('div[class^="layer_popup__"][class*="is_show__"]');
  if (layers.length !== 1) return empty(layers.length);
  const layer = layers[0];

  if (control === 'category_open') {
    const buttons = [...layer.querySelectorAll('button[class^="selectbox_button__"]')];
    const value = {matches: buttons.length, group_matches: buttons.length, actionable: false, x: 0, y: 0,
      expanded: buttons.length === 1 && buttons[0].getAttribute('aria-expanded') === 'true', checked: false, name_matches: true};
    return buttons.length === 1 ? point(buttons[0], value) : value;
  }

  if (control === 'category') {
    const raw = String(id);
    if (!/^[A-Za-z0-9_-]+$/.test(raw)) return empty();
    const opener = layer.querySelectorAll('button[class^="selectbox_button__"]');
    const expanded = opener.length === 1 && opener[0].getAttribute('aria-expanded') === 'true';
    const group = [...layer.querySelectorAll('input[type=radio][data-testid^="categoryBtn_"]')];
    const testID = 'categoryBtn_' + raw;
    const radios = group.filter((radio) => radio.getAttribute('data-testid') === testID);
    const texts = [...layer.querySelectorAll('[data-testid^="categoryItemText_"]')]
      .filter((node) => node.getAttribute('data-testid') === 'categoryItemText_' + raw);
    const nameMatches = texts.length === 1 && norm(texts[0].textContent).replace(/^하위\s*카테고리\s*/, '') === norm(name);
    const value = {matches: radios.length, group_matches: group.length, actionable: false, x: 0, y: 0,
      expanded, checked: radios.length === 1 && radios[0].checked, name_matches: nameMatches};
    return radios.length === 1 && texts.length === 1 ? point(texts[0], value) : value;
  }

  if (control === 'visibility') {
    const testIDs = {public: 'openType_2', neighbor: 'openType_1', both_neighbor: 'openType_3', private: 'openType_0'};
    const testID = testIDs[String(id)];
    if (!testID) return empty();
    const group = [...layer.querySelectorAll('input[type=radio][data-testid^="openType_"]')];
    const complete = Object.values(testIDs).every((wanted) => group.filter((radio) => radio.getAttribute('data-testid') === wanted).length === 1);
    const radios = group.filter((radio) => radio.getAttribute('data-testid') === testID);
    if (!complete || radios.length !== 1) return {...empty(radios.length), group_matches: group.length};
    const radio = radios[0];
    const holder = radio.closest('li') || radio.parentElement;
    const labels = holder ? [...holder.querySelectorAll('label[class^="radio_label__"]')] : [];
    const value = {matches: 1, group_matches: group.length, actionable: false, x: 0, y: 0,
      expanded: true, checked: radio.checked, name_matches: true};
    return labels.length === 1 ? point(labels[0], value) : value;
  }
  return empty();
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

type driverSettingsState struct {
	LayerMatches int `json:"layer_matches"`
	Tags         int `json:"tags"`
	Category     int `json:"category"`
	Visibility   int `json:"visibility"`
}

type driverSetting struct {
	Matches      int     `json:"matches"`
	GroupMatches int     `json:"group_matches"`
	Actionable   bool    `json:"actionable"`
	X            float64 `json:"x"`
	Y            float64 `json:"y"`
	Expanded     bool    `json:"expanded"`
	Checked      bool    `json:"checked"`
	NameMatches  bool    `json:"name_matches"`
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

func (p *CDPPort) settingsState(ctx context.Context) (driverSettingsState, error) {
	var state driverSettingsState
	if err := p.page.CallFunction(ctx, driverSettingsStateFn, nil, &state); err != nil {
		return driverSettingsState{}, PortError{Kind: FailureEditorChanged}
	}
	return state, nil
}

func (p *CDPPort) requireSettingsState(ctx context.Context, kind MutationKind) error {
	state, err := p.settingsState(ctx)
	if err != nil || state.LayerMatches != 1 {
		return PortError{Kind: FailureEditorChanged}
	}
	matches := map[MutationKind]int{
		MutationTags: state.Tags, MutationCategory: state.Category, MutationVisibility: state.Visibility,
	}
	if matches[kind] != expectedLocatorMatches(kind) {
		return PortError{Kind: FailureEditorChanged}
	}
	return nil
}

func (p *CDPPort) setting(ctx context.Context, control, id, name string) (driverSetting, error) {
	var setting driverSetting
	if err := p.page.CallFunction(ctx, driverSettingFn, []any{control, id, name}, &setting); err != nil {
		return driverSetting{}, PortError{Kind: FailureEditorChanged}
	}
	return setting, nil
}

func (p *CDPPort) clickSetting(ctx context.Context, setting driverSetting) error {
	if setting.Matches != 1 || !setting.Actionable {
		return PortError{Kind: FailureEditorChanged}
	}
	if err := p.page.ClickPoint(ctx, setting.X, setting.Y); err != nil {
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
