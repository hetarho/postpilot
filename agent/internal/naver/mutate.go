package naver

import (
	"context"
	"strings"
	"time"
)

// Reviewed driver functions. Each is a package-level constant from this signed release and
// receives manifest data only as structured CDP arguments, so no string the server sent can
// become part of an instruction. None of them types, uploads or activates anything: they
// resolve a versioned control, report how many matched, and return its live geometry.

// bodyParagraphsJS is the document's body text in paragraph order: a text component's own
// paragraphs plus the paragraphs of the components a conversion produced. The document TITLE
// is a .se-component inside .se-body and an image's caption is a paragraph too, so both are
// excluded by component kind rather than by a descendant query.
//
// Verified live 2026-09-10: 문단 서식 변경 moves the caret's paragraph into its OWN
// se-sectionTitle or se-quotation component, so a set scoped to `.se-component.se-text`
// alone stops at the last unconverted paragraph — an append after a heading would then land
// before the heading instead of after it.
const bodyParagraphsJS = `  const bodyParagraphs = () => {
    const body = document.querySelector('.se-body.__se-body');
    if (!body) return [];
    const collected = [];
    for (const component of body.querySelectorAll('.se-component')) {
      if (component.parentElement && component.parentElement.closest('.se-component')) continue;
      const kinds = component.classList;
      if (!kinds.contains('se-text') && !kinds.contains('se-sectionTitle') && !kinds.contains('se-quotation')) continue;
      for (const paragraph of component.querySelectorAll('p.se-text-paragraph')) {
        if (paragraph.closest('.se-module-text.se-caption')) continue;
        collected.push(paragraph);
      }
    }
    return collected;
  };
  const paragraphText = (node) => {
    const nodes = [...node.querySelectorAll('span.__se-node')];
    return (nodes.length ? nodes.map((n) => n.textContent).join('') : node.textContent).trim();
  };
  const writtenParagraphs = () => bodyParagraphs().filter((node) => paragraphText(node) !== '');
`

// driverPointFn returns the viewport point of the single element that backs one text target.
// The caller must treat the point as valid only for the action it is about to take.
const driverPointFn = `function (target, ordinal) {
  // A point is only usable if it is actually in the viewport and actually hits the element
  // the locator resolved. Both were assumptions until 260910, when an image upload opened
  // Naver's photo-library sidebar — aside.se-sidebar, 300 px wide, OVERLAYING the editor's
  // right edge — and every caret click resolved cleanly, landed on the sidebar and left the
  // caret outside the document. PUB-19 says scroll and viewport cannot be product
  // authority, so the element is brought into view first, and PUB-36's "the live geometry
  // of the single element its locator just resolved" is then verified by hit test rather
  // than assumed.
  const reveal = (node) => {
    const box = node.getBoundingClientRect();
    if (box.top >= 0 && box.bottom <= window.innerHeight) return;
    node.scrollIntoView({block: 'center', inline: 'nearest'});
  };
  const hitsComponent = (node, x, y) => {
    const at = document.elementFromPoint(x, y);
    if (!at) return false;
    if (at === node || node.contains(at)) return true;
    const owner = node.closest('.se-component');
    return Boolean(owner && at.closest('.se-component') === owner);
  };
  const hitsControl = (node, x, y) => {
    const at = document.elementFromPoint(x, y);
    return Boolean(at && (at === node || node.contains(at) || at.closest('button') === node));
  };
  const at = (node, place, verify) => {
    if (!node) return {matches: 0, x: 0, y: 0};
    reveal(node);
    const box = node.getBoundingClientRect();
    if (box.width <= 0 || box.height <= 0) return {matches: 0, x: 0, y: 0};
    const candidate = place(box);
    if (!verify(node, candidate.x, candidate.y)) return {matches: 0, x: 0, y: 0};
    return {matches: 1, x: candidate.x, y: candidate.y};
  };
  const centre = (box) => ({x: Math.round(box.left + box.width / 2), y: Math.round(box.top + box.height / 2)});
  // caretEnd aims at the paragraph's trailing edge rather than its centre. A box centre
  // lands at the text's end only while the text stops short of it; on a paragraph that
  // fills or wraps its line the centre lands mid-text and the next insertion would split
  // it. Verified live 2026-09-07.
  const trailing = (box) => ({x: Math.round(box.left + box.width * 0.98), y: Math.round(box.bottom - box.height * 0.25)});
  const point = (node) => at(node, centre, hitsControl);
  const caretEnd = (node) => at(node, trailing, hitsComponent);
  if (target === 'title') {
    const nodes = document.querySelectorAll('.se-component.se-documentTitle .se-title-text');
    return nodes.length === 1 ? point(nodes[0]) : {matches: nodes.length, x: 0, y: 0};
  }
` + bodyParagraphsJS + `  if (target === 'body_end') {
    const paragraphs = bodyParagraphs();
    return paragraphs.length === 0 ? {matches: 0, x: 0, y: 0} : caretEnd(paragraphs[paragraphs.length - 1]);
  }
  // body_paragraph addresses one paragraph by its position among the NON-EMPTY ones, which
  // is the same order the projection reports and therefore the manifest's own block order.
  // Empty paragraphs are editor chrome — the slot SmartEditor leaves after an image, the
  // quotation's empty 출처 — and counting them would shift every later index. A negative
  // ordinal counts from the end, so -1 is the paragraph the last write produced.
  if (target === 'body_paragraph') {
    const paragraphs = writtenParagraphs();
    return caretEnd(paragraphs[ordinal < 0 ? paragraphs.length + ordinal : ordinal]);
  }
  if (target === 'image_add') {
    const buttons = document.querySelectorAll('button.se-image-toolbar-button');
    return buttons.length === 1 ? point(buttons[0]) : {matches: buttons.length, x: 0, y: 0};
  }
  // An empty caption module has NO BOX until its image is selected — it measured 0 x 0 on
  // a freshly uploaded photo and 320 x 24 the moment the image was clicked (verified live
  // 2026-09-10) — so a caption is always preceded by selecting its image through this
  // target. Selecting changes no document content; it only reveals the caption field.
  if (target === 'image_select') {
    const images = document.querySelectorAll('.se-body.__se-body .se-component.se-image');
    const image = images[ordinal];
    if (!image) return {matches: 0, x: 0, y: 0};
    const modules = image.querySelectorAll('.se-module-image');
    return modules.length === 1 ? at(modules[0], centre, hitsComponent) : {matches: modules.length, x: 0, y: 0};
  }
  if (target === 'image_caption') {
    const images = document.querySelectorAll('.se-body.__se-body .se-component.se-image');
    const image = images[ordinal];
    if (!image) return {matches: 0, x: 0, y: 0};
    const captions = image.querySelectorAll('.se-module-text.se-caption');
    return captions.length === 1 ? at(captions[0], centre, hitsComponent) : {matches: captions.length, x: 0, y: 0};
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

// driverParagraphStateFn observes one addressed paragraph without touching it: which kind of
// component now holds it, whether it sits in a list item or in a quotation's 출처 module, and
// how many items its list has. Every conversion asserts this before resolving a control and
// again after activating it, because a paragraph conversion is not always visible to the
// projection (Naver exports a section title as plain text).
const driverParagraphStateFn = `function (target, index) {
` + bodyParagraphsJS + `  const paragraphs = target === 'body_paragraph' ? writtenParagraphs() : bodyParagraphs();
  const paragraph = paragraphs[index < 0 ? paragraphs.length + index : index];
  if (!paragraph) return {matches: 0, total: paragraphs.length, kind: '', in_list: false, in_cite: false, list_items: 0, empty: false};
  const component = paragraph.closest('.se-component');
  const kind = ['se-text', 'se-sectionTitle', 'se-quotation'].find((name) => component.classList.contains(name)) || '';
  const item = paragraph.closest('li.se-text-list-item');
  const list = item ? item.closest('ul.se-text-list, ol.se-text-list') : null;
  return {
    matches: 1,
    total: paragraphs.length,
    kind,
    in_list: Boolean(item),
    in_cite: Boolean(paragraph.closest('.se-module-text.se-cite')),
    list_items: list ? list.querySelectorAll('li.se-text-list-item').length : 0,
    empty: paragraphText(paragraph) === ''
  };
}`

// driverImageStateFn counts the editor's images and how many of them Naver has finished
// processing — a settled image is one whose resource resolved to an https URL. Nothing about
// an upload is evidence until this settles: at the moment an image appears but is not yet
// settled, SmartEditor is still splitting the caret's component, and the projection catches
// a paragraph whose tail has been detached but not yet reattached. That is exactly how a
// clean-editor run read "LIVE T1 첫 " for a paragraph that had just read back whole
// (observed live 2026-09-10).
const driverImageStateFn = `function () {
  const images = [...document.querySelectorAll('.se-body.__se-body .se-component.se-image')];
  let settled = 0;
  for (const image of images) {
    const resource = image.querySelector('img.se-image-resource');
    if (resource && /^https:\/\//.test(resource.src)) settled += 1;
  }
  return {images: images.length, settled};
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
    // Naver opens its photo-library sidebar when an image is uploaded, and that sidebar
    // overlays the editor's right edge — which is where every caret point is taken. Closing
    // it is a versioned step of the upload, and a sidebar that is already gone is a no-op
    // rather than a refusal. Verified live 2026-09-10.
    case 'close_library': {
      const sidebars = [...document.querySelectorAll('aside.se-sidebar')].filter((node) => {
        const box = node.getBoundingClientRect();
        return box.width > 0 && box.height > 0 && getComputedStyle(node).visibility !== 'hidden';
      });
      if (sidebars.length === 0) return {matches: 1, activated: false, already: true};
      if (sidebars.length !== 1) return {matches: sidebars.length, activated: false};
      pick([...sidebars[0].querySelectorAll('button.se-sidebar-close-button')]);
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

type driverParagraphState struct {
	Matches   int    `json:"matches"`
	Total     int    `json:"total"`
	Kind      string `json:"kind"`
	InList    bool   `json:"in_list"`
	InCite    bool   `json:"in_cite"`
	ListItems int    `json:"list_items"`
	Empty     bool   `json:"empty"`
}

type driverImageState struct {
	Images  int `json:"images"`
	Settled int `json:"settled"`
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

func (p *CDPPort) paragraphState(ctx context.Context, target string, index int) (driverParagraphState, error) {
	var state driverParagraphState
	if err := p.page.CallFunction(ctx, driverParagraphStateFn, []any{target, index}, &state); err != nil {
		return driverParagraphState{}, PortError{Kind: FailureEditorChanged}
	}
	return state, nil
}

func (p *CDPPort) imageState(ctx context.Context) (driverImageState, error) {
	var state driverImageState
	if err := p.page.CallFunction(ctx, driverImageStateFn, nil, &state); err != nil {
		return driverImageState{}, PortError{Kind: FailureEditorChanged}
	}
	return state, nil
}

// awaitImage waits for exactly one more settled image and for nothing else. An upload that
// adds none, adds two, or never finishes processing all fail closed — a resolved path or an
// accepted upload call is never evidence on its own (PUB-21).
func (p *CDPPort) awaitImage(ctx context.Context, want int) error {
	deadline := time.Now().Add(p.settle)
	for {
		state, err := p.imageState(ctx)
		if err != nil {
			return err
		}
		if state.Images > want {
			return PortError{Kind: FailureEditorChanged}
		}
		if state.Images == want && state.Settled == want {
			return nil
		}
		if !time.Now().Before(deadline) {
			return PortError{Kind: FailureEditorChanged}
		}
		select {
		case <-ctx.Done():
			return PortError{Kind: FailureEditorChanged}
		case <-time.After(uploadPollInterval):
		}
	}
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

// enterText writes manifest text and COMMITS it. Input.insertText leaves everything after
// the last space uncommitted in SmartEditor: the DOM shows the whole string, but the first
// structural change — an image insertion or a paragraph conversion rebuilding that paragraph
// — discards the pending word. Measured live on 2026-09-10: without the trailing key 4 of 5
// writes lost their last word, with it 0 of 5. Only a key commits; a caret-only key (End)
// does not, and waiting up to 8s does not reliably either.
//
// Enter is safe in every module this is used for: in the title and in a caption it changes
// nothing observable, and in the body it opens the next paragraph, which the following write
// then types into instead of adding one of its own.
func (p *CDPPort) enterText(ctx context.Context, text string) error {
	if err := p.typeText(ctx, text); err != nil {
		return err
	}
	if err := p.page.PressEnter(ctx); err != nil {
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
