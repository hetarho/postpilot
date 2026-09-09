package naver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/postpilot/agent/internal/browser"
)

// editorObservationScript is reviewed local release code. It reads only the versioned
// SmartEditor surface and returns inert data: no string it returns is ever evaluated,
// used as a selector, or turned into an action. It reads no cookie, storage or credential.
//
// Every hook below was verified against the live signed-in editor (Chrome 152,
// SmartEditor ONE, 2026-09-06). Fuzzy attribute matching is deliberately absent: Naver's
// CSS-module hashes contain arbitrary letters, and a case-insensitive "otp" probe matched
// the class "icon_arrow__OTpnA" on the writer itself.
const editorObservationScript = `(() => {
  const norm = (value) => String(value == null ? '' : value).replace(/\s+/g, ' ').trim();
  // Placeholder chrome lives outside __se-node, so the title of an empty editor reads as
  // its placeholder unless only the real text nodes are collected.
  const nodeText = (root) => root ? [...root.querySelectorAll('span.__se-node')].map((n) => n.textContent).join('') : '';
  const shown = (node) => {
    if (!node) return false;
    const box = node.getBoundingClientRect();
    return box.width > 0 && box.height > 0 && getComputedStyle(node).visibility !== 'hidden';
  };
  const visible = (selector) => [...document.querySelectorAll(selector)].some(shown);
  const count = (selector) => document.querySelectorAll(selector).length;

  const editorRoot = Boolean(document.querySelector('.blog_editor'));
  const body = document.querySelector('.se-body.__se-body');
  const settingsLayers = [...document.querySelectorAll('div[class^="layer_popup__"][class*="is_show__"]')];
  const layer = settingsLayers.length === 1 ? settingsLayers[0] : null;

  // Naver preloads a hidden captcha iframe on the writer, so presence is not a challenge;
  // only a control the user can actually see counts.
  const onLoginHost = location.hostname === 'nid.naver.com';
  let auth = 'ready';
  if (visible('iframe[id^="ncaptcha"], #captcha, #captchaimg')) auth = 'captcha';
  else if (onLoginHost && visible('input[name="otp"], input[id="otp"]')) auth = 'two_factor';
  else if (onLoginHost || visible('input[type=password]')) auth = 'login';

  // The body projection matches the Naver export exactly: a section title and a quotation
  // are plain text, a list is one "- item" block, and an image holds its ordinal.
  const blocks = [];
  let imageOrdinal = 0;
  if (body) {
    for (const component of body.querySelectorAll('.se-component')) {
      if (component.parentElement && component.parentElement.closest('.se-component')) continue;
      const kinds = component.classList;
      if (kinds.contains('se-documentTitle')) continue;
      if (kinds.contains('se-image')) {
        imageOrdinal += 1;
        const caption = component.querySelector('.se-module-text.se-caption');
        const resource = component.querySelector('img.se-image-resource');
        blocks.push({
          kind: 'image', text: '', ordinal: imageOrdinal,
          caption: caption && !caption.classList.contains('se-is-empty') ? nodeText(caption) : '',
          uploaded: Boolean(resource && /^https:\/\//.test(resource.src)),
          source: resource ? norm(resource.alt) : ''
        });
      } else if (kinds.contains('se-sectionTitle')) {
        blocks.push({kind: 'text', text: nodeText(component), ordinal: 0, caption: '', uploaded: false, source: ''});
      } else if (kinds.contains('se-quotation')) {
        const quoted = component.querySelector('.se-module-text.se-quote') || component;
        blocks.push({kind: 'text', text: '“' + nodeText(quoted) + '”', ordinal: 0, caption: '', uploaded: false, source: ''});
      } else if (kinds.contains('se-text')) {
        for (const unit of component.querySelectorAll('.se-module-text.__se-unit')) {
          for (const child of unit.children) {
            if (child.tagName === 'UL' || child.tagName === 'OL') {
              const items = [...child.querySelectorAll('li.se-text-list-item')].map((li) => nodeText(li));
              blocks.push({kind: 'text', text: items.map((item) => '- ' + item).join('\n'), ordinal: 0, caption: '', uploaded: false, source: ''});
            } else if (child.tagName === 'P') {
              // No manifest block is ever empty, so an empty paragraph is editor chrome.
              const text = nodeText(child);
              if (text !== '') blocks.push({kind: 'text', text, ordinal: 0, caption: '', uploaded: false, source: ''});
            }
          }
        }
      }
    }
  }

  const titleModule = document.querySelector('.se-component.se-documentTitle .se-title-text');
  const title = titleModule && !titleModule.classList.contains('se-is-empty') ? nodeText(titleModule) : '';

  const categoryRadios = layer ? [...layer.querySelectorAll('input[type=radio][data-testid^="categoryBtn_"]')] : [];
  let category = {id: '', name: '', selected: false};
  for (const radio of categoryRadios) {
    if (!radio.checked) continue;
    const id = String(radio.getAttribute('data-testid')).slice('categoryBtn_'.length);
    const label = layer.querySelector('[data-testid="categoryItemText_' + id + '"]');
    category = {id, name: norm(label && label.textContent).replace(/^하위\s*카테고리\s*/, ''), selected: true};
  }

  const visibilityByTestID = {openType_2: 'public', openType_1: 'neighbor', openType_3: 'both_neighbor', openType_0: 'private'};
  const visibilityRadios = layer ? [...layer.querySelectorAll('input[type=radio][data-testid^="openType_"]')] : [];
  let visibility = {id: '', name: '', selected: false};
  for (const radio of visibilityRadios) {
    if (!radio.checked) continue;
    const holder = radio.closest('li,div,span');
    const label = holder ? holder.querySelector('label[class^="radio_label__"]') : null;
    visibility = {id: visibilityByTestID[String(radio.getAttribute('data-testid'))] || '', name: norm(label && label.textContent), selected: true};
  }
  const tags = layer ? [...layer.querySelectorAll('span[class^="tag__"]')].map((n) => norm(n.textContent).replace(/^#/, '')).filter(Boolean) : [];

  const images = [...document.querySelectorAll('.se-component.se-image')];
  const captionsWellFormed = images.length > 0 &&
    images.every((image) => image.querySelectorAll('.se-module-text.se-caption').length === 1);

  // The opener and the final control are both versioned; anything else carrying a
  // final-control accessible name is an unreviewed duplicate and must fail closed.
  const versioned = new Set([...document.querySelectorAll('button[class^="confirm_btn__"], button[class^="publish_btn__"]')]);
  const finalNames = ['발행'];
  const unversionedPublishLike = [...document.querySelectorAll('button')]
    .filter((button) => !versioned.has(button) && finalNames.includes(norm(button.getAttribute('aria-label') || button.textContent))).length;

  return {
    href: location.href,
    auth,
    editor_root: editorRoot,
    settings_layer: settingsLayers.length === 1,
    title,
    blocks,
    image_count: imageOrdinal,
    tags,
    category,
    visibility,
    locator_matches: {
      title: count('.se-component.se-documentTitle .se-title-text'),
      text: count('.se-body.__se-body'),
      heading: count('.se-body.__se-body'),
      quote: count('.se-body.__se-body'),
      list: count('.se-body.__se-body'),
      upload_image: count('input[type=file]#hidden-file'),
      image_caption: captionsWellFormed ? 1 : 0,
      // The opener stays countable once the layer is already open, so re-observing after
      // open_settings does not report a vanished control.
      open_settings: count('button[class^="publish_btn__"]'),
      tags: layer ? layer.querySelectorAll('input[class^="tag_input__"]').length : 0,
      category: layer ? layer.querySelectorAll('button[class^="selectbox_button__"]').length : 0,
      visibility: visibilityRadios.length
    },
    final_controls: layer ? layer.querySelectorAll('button[class^="confirm_btn__"]').length : 0,
    unversioned_publish_like: unversionedPublishLike
  };
})()`

type observedBlock struct {
	Kind     string `json:"kind"`
	Text     string `json:"text"`
	Ordinal  int    `json:"ordinal"`
	Caption  string `json:"caption"`
	Uploaded bool   `json:"uploaded"`
	Source   string `json:"source"`
}

type observedSetting struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Selected bool   `json:"selected"`
}

type editorObservation struct {
	Href                   string          `json:"href"`
	Auth                   string          `json:"auth"`
	EditorRoot             bool            `json:"editor_root"`
	SettingsLayer          bool            `json:"settings_layer"`
	Title                  string          `json:"title"`
	Blocks                 []observedBlock `json:"blocks"`
	ImageCount             int             `json:"image_count"`
	Tags                   []string        `json:"tags"`
	Category               observedSetting `json:"category"`
	Visibility             observedSetting `json:"visibility"`
	LocatorMatches         map[string]int  `json:"locator_matches"`
	FinalControls          int             `json:"final_controls"`
	UnversionedPublishLike int             `json:"unversioned_publish_like"`
}

var writerBlogID = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

// CDPPort is the deterministic observation half of Port. It owns the reviewed SmartEditor
// locator set and binds every observation to one dedicated CDP page.
type CDPPort struct {
	page     *browser.Page
	manifest CompatibilityManifest
	// settingsOpen latches when the publish settings layer opens. It never clears: the
	// layer occludes the editor and no body write may follow it (PUBLISH-37).
	settingsOpen bool
}

// NewCDPPort binds the sole dedicated page. It never navigates: the caller has already
// opened the account's own writer through Naver's own redirect.
func NewCDPPort(ctx context.Context, cdpURL string) (*CDPPort, error) {
	manifest, err := Manifest()
	if err != nil {
		return nil, err
	}
	page, err := browser.BindSinglePage(ctx, cdpURL)
	if err != nil {
		return nil, err
	}
	return &CDPPort{page: page, manifest: manifest}, nil
}

func (p *CDPPort) Close() error { return p.page.Close() }

// Observe returns one full snapshot fenced by a recheck on both sides, so a target switch,
// a second page or a navigation during the read poisons the run instead of producing
// evidence assembled from two documents.
func (p *CDPPort) Observe(ctx context.Context) (Snapshot, error) {
	before, err := p.page.Recheck(ctx)
	if err != nil {
		return Snapshot{}, PortError{Kind: FailureEditorChanged}
	}
	var observation editorObservation
	if err := p.page.Evaluate(ctx, editorObservationScript, &observation); err != nil {
		// A native dialog suspends script execution on the page, so a blocked evaluation
		// against a still-present target is itself the evidence that one is open.
		return Snapshot{}, PortError{Kind: FailureEditorChanged}
	}
	if _, err := p.page.AccessibilityRoles(ctx); err != nil {
		return Snapshot{}, PortError{Kind: FailureEditorChanged}
	}
	after, err := p.page.Recheck(ctx)
	if err != nil || after != before || observation.Href != after {
		return Snapshot{}, PortError{Kind: FailureEditorChanged}
	}
	account, err := writerAccount(after)
	if err != nil {
		return Snapshot{}, PortError{Kind: FailureEditorChanged}
	}
	snapshot := Snapshot{
		TargetID:                       p.page.TargetID(),
		URL:                            after,
		AccountID:                      account,
		SignatureID:                    p.manifest.SignatureID,
		Auth:                           authState(observation.Auth),
		Title:                          observation.Title,
		Body:                           semanticBlocks(observation.Blocks),
		ImageCount:                     observation.ImageCount,
		Tags:                           append([]string{}, observation.Tags...),
		Category:                       SelectedSetting(observation.Category),
		Visibility:                     SelectedSetting(observation.Visibility),
		SettingsLayerOpen:              observation.SettingsLayer,
		LocatorMatches:                 locatorMatches(observation.LocatorMatches),
		UnversionedPublishLikeControls: observation.UnversionedPublishLike,
		NativeDialogOpen:               false,
	}
	if !observation.EditorRoot && snapshot.Auth == AuthReady {
		return Snapshot{}, PortError{Kind: FailureEditorChanged}
	}
	snapshot.Token = snapshotToken(snapshot)
	return snapshot, nil
}

// writerAccount reads the blog identity from the writer URL Naver itself resolved. It is
// never taken from user input or page prose.
// Apply executes one typed command. It is the only place the driver writes, and it writes
// nothing the caller described: every step resolves a versioned locator of this signed
// release, requires exactly one match, and uses the geometry it just read once.
//
// PUBLISH-37: once the settings layer is open it covers the editor, so a body write
// resolved from a body element's own geometry would land on the layer. The latch below
// refuses that in the port rather than trusting the plan.
func (p *CDPPort) Apply(ctx context.Context, mutation Mutation) error {
	if p.settingsOpen && isBodyMutation(mutation.Kind) {
		return PortError{Kind: FailureEditorChanged}
	}
	if !p.settingsOpen && slices.Contains([]MutationKind{MutationTags, MutationCategory, MutationVisibility}, mutation.Kind) {
		return PortError{Kind: FailureEditorChanged}
	}
	switch mutation.Kind {
	case MutationTitle:
		if err := p.resolveAndClick(ctx, "title", 0); err != nil {
			return err
		}
		return p.typeText(ctx, mutation.Text)
	case MutationText:
		return p.appendParagraph(ctx, mutation.Text)
	case MutationHeading:
		return p.applyHeading(ctx, mutation.Text)
	case MutationQuote:
		return p.applyQuote(ctx, mutation.Ordinal)
	case MutationList:
		return p.applyList(ctx, mutation.Ordinal, mutation.Items)
	case MutationOpenSettings:
		if p.settingsOpen {
			return PortError{Kind: FailureEditorChanged}
		}
		if err := p.resolveAndClick(ctx, "settings_open", 0); err != nil {
			return err
		}
		// Once the opener was clicked, an ambiguous observation must never permit another
		// body write: the layer may be covering the editor even if its shape changed.
		p.settingsOpen = true
		state, err := p.settingsState(ctx)
		if err != nil || state.LayerMatches != 1 || state.Tags != 1 || state.Category != 1 || state.Visibility != 4 {
			return PortError{Kind: FailureEditorChanged}
		}
		return nil
	case MutationTags:
		return p.applyTags(ctx, mutation.Values)
	case MutationCategory:
		return p.applyCategory(ctx, mutation.ID, mutation.Name)
	case MutationVisibility:
		return p.applyVisibility(ctx, mutation.ID)
	default:
		// Every remaining kind lands with its own task. Refusing here keeps a half-built
		// driver from writing to a live editor it cannot finish (PUBLISH-19).
		return PortError{Kind: FailureSafe}
	}
}

// applyHeading writes the paragraph and converts it in the SAME step. The conversion alone
// is invisible to the projection — Naver exports a section title as plain text, so the
// observed snapshot would not change — and Publisher.mutate requires every mutation to move
// the snapshot token. Writing first and converting the paragraph just written keeps the new
// text block as the step's one observable change.
func (p *CDPPort) applyHeading(ctx context.Context, text string) error {
	if err := p.appendParagraph(ctx, text); err != nil {
		return err
	}
	return p.convertParagraph(ctx, -1, "format_heading", "se-sectionTitle")
}

// applyQuote converts a paragraph the write pass already entered, and enters no text of its
// own. 인용구 is born with an empty `se-cite` (출처) module whose paragraph is the new
// component's LAST one, so text typed after the conversion would land in the citation
// instead of the quote. Verified live 2026-09-10.
func (p *CDPPort) applyQuote(ctx context.Context, index int) error {
	return p.convertParagraph(ctx, index, "format_quote", "se-quotation")
}

// applyList converts the paragraph the write pass entered as the list's first item, then
// appends the remaining items with Enter: Enter from a list item adds another LI to the SAME
// UL, which is the only way one manifest LIST block with N items is built. The list control
// exists only while the caret sits in a plain text paragraph — with the caret inside a
// quotation it resolved 0 live — so the kind assertion below is a precondition, not a
// defence. Verified live 2026-09-10.
func (p *CDPPort) applyList(ctx context.Context, index int, items []string) error {
	if len(items) == 0 {
		return PortError{Kind: FailureSafe}
	}
	for _, item := range items {
		if strings.TrimSpace(item) == "" {
			return PortError{Kind: FailureSafe}
		}
	}
	if err := p.requirePlainParagraph(ctx, index); err != nil {
		return err
	}
	if err := p.resolveAndClick(ctx, "body_paragraph", index); err != nil {
		return err
	}
	if err := p.activate(ctx, "list_menu", ""); err != nil {
		return err
	}
	if err := p.activate(ctx, "list_bullet", ""); err != nil {
		return err
	}
	converted, err := p.paragraphState(ctx, index)
	if err != nil {
		return err
	}
	if converted.Matches != 1 || !converted.InList || converted.ListItems != 1 {
		return PortError{Kind: FailureEditorChanged}
	}
	for _, item := range items[1:] {
		if err := p.page.PressEnter(ctx); err != nil {
			return PortError{Kind: FailureEditorChanged}
		}
		if err := p.typeText(ctx, item); err != nil {
			return err
		}
	}
	final, err := p.paragraphState(ctx, index)
	if err != nil {
		return err
	}
	if final.Matches != 1 || !final.InList || final.ListItems != len(items) {
		return PortError{Kind: FailureEditorChanged}
	}
	return nil
}

// convertParagraph puts the caret in one addressed paragraph and converts it through the
// paragraph-format toolbar. The option controls do not exist until the menu is open — they
// resolve 0 while it is closed — so the option is counted after the menu opens, the same
// shape the settings layer uses. Nothing is typed on this path, so a refused option leaves
// the document exactly as the write pass left it.
func (p *CDPPort) convertParagraph(ctx context.Context, index int, option, want string) error {
	if err := p.requirePlainParagraph(ctx, index); err != nil {
		return err
	}
	if err := p.resolveAndClick(ctx, "body_paragraph", index); err != nil {
		return err
	}
	if err := p.activate(ctx, "format_menu", ""); err != nil {
		return err
	}
	if err := p.activate(ctx, option, ""); err != nil {
		return err
	}
	after, err := p.paragraphState(ctx, index)
	if err != nil {
		return err
	}
	if after.Matches != 1 || after.Kind != want {
		return PortError{Kind: FailureEditorChanged}
	}
	return nil
}

// requirePlainParagraph refuses every conversion whose target is not still an unconverted
// body paragraph: a section title, a quotation, a citation module or an existing list item
// carries a different property toolbar, and converting the wrong paragraph is exactly the
// silent corruption PUBLISH-19 forbids guessing about.
func (p *CDPPort) requirePlainParagraph(ctx context.Context, index int) error {
	state, err := p.paragraphState(ctx, index)
	if err != nil {
		return err
	}
	if state.Matches != 1 || state.Kind != "se-text" || state.InList || state.InCite {
		return PortError{Kind: FailureEditorChanged}
	}
	return nil
}

func (p *CDPPort) applyTags(ctx context.Context, values []string) error {
	normalized, ok := uniqueTags(values)
	if !ok || len(normalized) == 0 || !slices.Equal(normalized, values) {
		return PortError{Kind: FailureSafe}
	}
	for _, value := range values {
		if err := p.requireSettingsState(ctx, MutationTags); err != nil {
			return err
		}
		// The reviewed target is tag_input__, never the adjacent fake_input__ decoy.
		if err := p.resolveAndClick(ctx, "tag_input", 0); err != nil {
			return err
		}
		if err := p.typeText(ctx, value); err != nil {
			return err
		}
		if err := p.page.PressEnter(ctx); err != nil {
			return PortError{Kind: FailureEditorChanged}
		}
	}
	return nil
}

func (p *CDPPort) applyCategory(ctx context.Context, id, name string) error {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(name) == "" {
		return PortError{Kind: FailureSafe}
	}
	if err := p.requireSettingsState(ctx, MutationCategory); err != nil {
		return err
	}
	opener, err := p.setting(ctx, "category_open", "", "")
	if err != nil || opener.Matches != 1 || !opener.Actionable {
		return PortError{Kind: FailureEditorChanged}
	}
	if !opener.Expanded {
		if err := p.clickSetting(ctx, opener); err != nil {
			return err
		}
		opener, err = p.setting(ctx, "category_open", "", "")
		if err != nil || opener.Matches != 1 || !opener.Expanded {
			return PortError{Kind: FailureEditorChanged}
		}
	}
	choice, err := p.setting(ctx, "category", id, name)
	if err != nil || choice.Matches != 1 || choice.GroupMatches == 0 || !choice.Expanded || !choice.NameMatches || !choice.Actionable {
		return PortError{Kind: FailureEditorChanged}
	}
	if !choice.Checked {
		if err := p.clickSetting(ctx, choice); err != nil {
			return err
		}
	}
	verified, err := p.setting(ctx, "category", id, name)
	if err != nil || verified.Matches != 1 || !verified.Expanded || !verified.NameMatches || !verified.Checked {
		return PortError{Kind: FailureEditorChanged}
	}
	return nil
}

func (p *CDPPort) applyVisibility(ctx context.Context, id string) error {
	if visibilityName(id) == "" {
		return PortError{Kind: FailureSafe}
	}
	if err := p.requireSettingsState(ctx, MutationVisibility); err != nil {
		return err
	}
	choice, err := p.setting(ctx, "visibility", id, "")
	if err != nil || choice.Matches != 1 || choice.GroupMatches != 4 || !choice.Actionable {
		return PortError{Kind: FailureEditorChanged}
	}
	if !choice.Checked {
		if err := p.clickSetting(ctx, choice); err != nil {
			return err
		}
	}
	verified, err := p.setting(ctx, "visibility", id, "")
	if err != nil || verified.Matches != 1 || verified.GroupMatches != 4 || !verified.Checked {
		return PortError{Kind: FailureEditorChanged}
	}
	return nil
}

// appendParagraph opens the next paragraph at the end of the body and types into it.
//
// Enter appends a paragraph INSIDE the current text component rather than opening a new
// component, which is why the projection counts paragraphs and not components. Verified
// live 2026-09-07.
func (p *CDPPort) appendParagraph(ctx context.Context, text string) error {
	if err := p.resolveAndClick(ctx, "body_end", 0); err != nil {
		return err
	}
	if err := p.page.PressEnter(ctx); err != nil {
		return PortError{Kind: FailureEditorChanged}
	}
	return p.typeText(ctx, text)
}

func writerAccount(current string) (string, error) {
	parsed, err := url.Parse(current)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() != "blog.naver.com" || parsed.EscapedPath() != "/PostWriteForm.naver" {
		return "", errors.New("dedicated page is not the versioned Naver writer")
	}
	blogID := strings.TrimSpace(parsed.Query().Get("blogId"))
	if !writerBlogID.MatchString(blogID) {
		return "", errors.New("Naver writer exposed no blog identity")
	}
	return blogID, nil
}

func authState(value string) AuthState {
	switch AuthState(value) {
	case AuthReady, AuthLogin, AuthCaptcha, Auth2FA:
		return AuthState(value)
	default:
		return AuthLogin
	}
}

func semanticBlocks(observed []observedBlock) []SemanticBlock {
	blocks := make([]SemanticBlock, 0, len(observed))
	for _, block := range observed {
		if block.Kind == "image" {
			blocks = append(blocks, SemanticBlock{Kind: SemanticImage, Ordinal: block.Ordinal, Caption: block.Caption, Uploaded: block.Uploaded})
			continue
		}
		blocks = append(blocks, SemanticBlock{Kind: SemanticText, Text: block.Text})
	}
	return blocks
}

func locatorMatches(observed map[string]int) map[MutationKind]int {
	matches := make(map[MutationKind]int, len(observed))
	for _, kind := range reviewedMutationKinds {
		matches[kind] = observed[string(kind)]
	}
	return matches
}

// snapshotToken digests the whole observed projection. It must stay content-derived:
// Publisher.mutate fails closed when the token is unchanged across a mutation, so a
// counter would silently stop catching a mutation that changed nothing.
func snapshotToken(snapshot Snapshot) string {
	matches := make(map[string]int, len(snapshot.LocatorMatches))
	for kind, value := range snapshot.LocatorMatches {
		matches[string(kind)] = value
	}
	payload := struct {
		TargetID       string          `json:"target_id"`
		URL            string          `json:"url"`
		AccountID      string          `json:"account_id"`
		SignatureID    string          `json:"signature_id"`
		Auth           AuthState       `json:"auth"`
		Title          string          `json:"title"`
		Body           []SemanticBlock `json:"body"`
		ImageCount     int             `json:"image_count"`
		Tags           []string        `json:"tags"`
		Category       SelectedSetting `json:"category"`
		Visibility     SelectedSetting `json:"visibility"`
		SettingsLayer  bool            `json:"settings_layer_open"`
		LocatorMatches map[string]int  `json:"locator_matches"`
		Unversioned    int             `json:"unversioned_publish_like"`
		NativeDialog   bool            `json:"native_dialog_open"`
	}{
		snapshot.TargetID, snapshot.URL, snapshot.AccountID, snapshot.SignatureID, snapshot.Auth,
		snapshot.Title, snapshot.Body, snapshot.ImageCount, snapshot.Tags, snapshot.Category,
		snapshot.Visibility, snapshot.SettingsLayerOpen, matches, snapshot.UnversionedPublishLikeControls, snapshot.NativeDialogOpen,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}
