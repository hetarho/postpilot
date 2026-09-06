package naver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
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
  const layer = document.querySelector('div[class^="layer_popup__"][class*="is_show__"]');

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
  const categoryIDs = categoryRadios.map((radio) => String(radio.getAttribute('data-testid')).slice('categoryBtn_'.length));
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
  const visibilityComplete = Object.keys(visibilityByTestID)
    .every((testID) => visibilityRadios.filter((radio) => radio.getAttribute('data-testid') === testID).length === 1);

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
    settings_layer: Boolean(layer),
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
      image_placeholder: count('button.se-image-toolbar-button'),
      upload_image: count('input[type=file]#hidden-file'),
      image_caption: captionsWellFormed ? 1 : 0,
      tags: layer ? layer.querySelectorAll('input[class^="tag_input__"]').length : 0,
      category: categoryIDs.length > 0 && new Set(categoryIDs).size === categoryIDs.length ? 1 : categoryIDs.length,
      visibility: visibilityComplete ? 1 : visibilityRadios.length
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
	for _, kind := range []MutationKind{MutationTitle, MutationText, MutationHeading, MutationQuote, MutationList, MutationImagePlaceholder, MutationUploadImage, MutationImageCaption, MutationTags, MutationCategory, MutationVisibility} {
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
		LocatorMatches map[string]int  `json:"locator_matches"`
		Unversioned    int             `json:"unversioned_publish_like"`
		NativeDialog   bool            `json:"native_dialog_open"`
	}{
		snapshot.TargetID, snapshot.URL, snapshot.AccountID, snapshot.SignatureID, snapshot.Auth,
		snapshot.Title, snapshot.Body, snapshot.ImageCount, snapshot.Tags, snapshot.Category,
		snapshot.Visibility, matches, snapshot.UnversionedPublishLikeControls, snapshot.NativeDialogOpen,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}
