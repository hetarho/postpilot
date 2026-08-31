package browser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

const identityPreparationScript = `(() => {
  const editorReady = Boolean(
    document.querySelector('.blog_editor') &&
    document.querySelector('.se-component.se-documentTitle .se-title-text') &&
    document.querySelector('.se-body.__se-body') &&
    document.querySelector('button.se-image-toolbar-button')
  );
  if (!editorReady) return {editor_ready: false, ready: false, stage: 'waiting_editor'};

  const visibleLayer = document.querySelector('div[class^="layer_popup__"][class*="is_show__"]');
  if (!visibleLayer) {
    const openers = [...document.querySelectorAll('button[class^="publish_btn__"]')];
    const finalControls = document.querySelectorAll('button[class^="confirm_btn__"]');
    if (openers.length !== 1 || finalControls.length !== 0) {
      return {editor_ready: true, ready: false, error: 'unrecognized publish-settings opener'};
    }
    openers[0].click();
    return {editor_ready: true, ready: false, stage: 'opening_publish_settings'};
  }

  const finalControls = visibleLayer.querySelectorAll('button[class^="confirm_btn__"]');
  const categoryButton = visibleLayer.querySelector('button[aria-label="카테고리 목록 버튼"]');
  if (finalControls.length !== 1 || !categoryButton) {
    return {editor_ready: true, ready: false, error: 'unrecognized publish-settings layer'};
  }
  if (categoryButton.getAttribute('aria-expanded') !== 'true') {
    categoryButton.click();
    return {editor_ready: true, ready: false, stage: 'opening_categories'};
  }
  return {editor_ready: true, ready: true, stage: 'categories_visible'};
})()`

const identityObservationScript = `(() => {
  const documents = [];
  const visit = (doc) => {
    if (!doc || documents.includes(doc)) return;
    documents.push(doc);
    for (const frame of doc.querySelectorAll('iframe')) {
      try { visit(frame.contentDocument); } catch (_) {}
    }
  };
  visit(document);
  const editorReady = Boolean(
    document.querySelector('.blog_editor') &&
    document.querySelector('.se-component.se-documentTitle .se-title-text') &&
    document.querySelector('.se-body.__se-body') &&
    document.querySelector('button.se-image-toolbar-button')
  );
  const categories = new Map();
  const add = (id, name) => {
    id = String(id || '').trim();
    name = String(name || '').replace(/\s+/g, ' ').trim();
    if (/^[A-Za-z0-9_-]{1,128}$/.test(id) && name && name.length <= 200) {
      categories.set(id, name);
    }
  };
  for (const doc of documents) {
    for (const input of doc.querySelectorAll('input[data-testid^="categoryBtn_"]')) {
      const testID = String(input.getAttribute('data-testid') || '');
      const id = testID.slice('categoryBtn_'.length);
      const textNode = [...doc.querySelectorAll('[data-testid^="categoryItemText_"]')]
        .find((node) => node.getAttribute('data-testid') === 'categoryItemText_' + id);
      const name = String(textNode?.textContent || '').replace(/^\s*하위\s*카테고리\s*/, '');
      add(id, name);
    }
    for (const node of doc.querySelectorAll('[data-category-id], [data-category-no]')) {
      if (!node.closest('[id*="category"], [class*="category"], [aria-label*="카테고리"]')) continue;
      add(node.getAttribute('data-category-id') || node.getAttribute('data-category-no'), node.textContent);
    }
    for (const select of doc.querySelectorAll('select')) {
      const context = [select.id, select.name, select.className, select.getAttribute('aria-label'),
        select.closest('[class*="category"], [id*="category"]')?.textContent].join(' ');
      if (!/(category|카테고리)/i.test(context)) continue;
      for (const option of select.options) add(option.value, option.textContent);
    }
    for (const option of doc.querySelectorAll('[role="option"], [role="menuitemradio"]')) {
      const parent = option.closest('[class*="category"], [id*="category"], [aria-label*="카테고리"]');
      if (!parent) continue;
      add(option.getAttribute('data-category-id') || option.getAttribute('data-category-no') || option.getAttribute('value'), option.textContent);
    }
  }
  const blogLabelNode = documents.map((doc) =>
    doc.querySelector('[data-testid="blog-name"], .blog_name, [class~="blog_name"]')
  ).find(Boolean);
  return {
    href: location.href,
    editor_ready: editorReady,
    blog_label: String(blogLabelNode?.textContent || '').replace(/\s+/g, ' ').trim(),
    categories: [...categories].map(([id, name]) => ({id, name}))
  };
})()`

var (
	naverBlogID     = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)
	naverCategoryID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
)

type NaverCategory struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type NaverIdentity struct {
	BlogID     string
	BlogLabel  string
	Categories []NaverCategory
}

type pageTarget struct {
	ID                   string `json:"id"`
	Type                 string `json:"type"`
	URL                  string `json:"url"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

type cdpRequest struct {
	ID        int    `json:"id"`
	Method    string `json:"method"`
	Params    any    `json:"params,omitempty"`
	SessionID string `json:"sessionId,omitempty"`
}

type cdpResponse struct {
	ID        int             `json:"id"`
	Result    json.RawMessage `json:"result"`
	SessionID string          `json:"sessionId,omitempty"`
	Error     *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type cdpClient struct {
	conn      *websocket.Conn
	next      int
	sessionID string
}

// ObserveNaverIdentity obtains account/category evidence directly from the one
// Naver editor page Hermes reached through visible UI. No model output, page
// prose, cookie, or CDP endpoint is accepted as authority or sent to Postpilot.
func ObserveNaverIdentity(ctx context.Context, cdpURL string) (NaverIdentity, error) {
	target, err := discoverSinglePage(ctx, cdpURL)
	if err != nil {
		return NaverIdentity{}, err
	}
	observationCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(observationCtx, cdpURL, &websocket.DialOptions{
		HTTPClient: noProxyHTTPClient(10 * time.Second),
	})
	if err != nil {
		return NaverIdentity{}, fmt.Errorf("connect dedicated browser: %w", err)
	}
	defer conn.CloseNow()
	conn.SetReadLimit(1 << 20)
	client := &cdpClient{conn: conn}
	var attached struct {
		SessionID string `json:"sessionId"`
	}
	if err := client.call(observationCtx, "Target.attachToTarget", map[string]any{
		"targetId": target.ID,
		"flatten":  true,
	}, &attached); err != nil {
		return NaverIdentity{}, fmt.Errorf("attach selected browser page: %w", err)
	}
	if attached.SessionID == "" {
		return NaverIdentity{}, errors.New("attach selected browser page returned no session")
	}
	client.sessionID = attached.SessionID

	if err := client.call(observationCtx, "Page.enable", nil, nil); err != nil {
		return NaverIdentity{}, err
	}

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		preparation, prepareErr := client.prepareIdentity(observationCtx)
		if prepareErr != nil {
			err = prepareErr
		} else if preparation.Error != "" {
			err = errors.New(preparation.Error)
		} else if preparation.Ready {
			observation, observeErr := client.observe(observationCtx)
			if observeErr == nil {
				identity, validateErr := validateIdentityObservation(observation)
				if validateErr == nil {
					confirmed, confirmErr := discoverSinglePage(observationCtx, cdpURL)
					if confirmErr != nil || confirmed.ID != target.ID ||
						confirmed.WebSocketDebuggerURL != target.WebSocketDebuggerURL || confirmed.URL != observation.Href {
						if confirmErr == nil {
							confirmErr = errors.New("dedicated browser target changed during identity verification")
						}
						return NaverIdentity{}, fmt.Errorf("recheck dedicated browser target: %w", confirmErr)
					}
					return identity, nil
				}
				err = validateErr
			} else {
				err = observeErr
			}
		} else {
			err = fmt.Errorf("Naver editor preparation is incomplete: %s", preparation.Stage)
		}
		select {
		case <-observationCtx.Done():
			if ctx.Err() != nil {
				return NaverIdentity{}, ctx.Err()
			}
			return NaverIdentity{}, fmt.Errorf("Naver editor identity was not observable: %w", err)
		case <-ticker.C:
		}
	}
}

func navigateSinglePage(ctx context.Context, cdpURL, destination string) error {
	targetURL, err := url.Parse(destination)
	if err != nil || targetURL.Scheme != "https" || !isNaverHost(targetURL.Hostname()) || targetURL.User != nil {
		return errors.New("browser navigation destination is outside approved Naver hosts")
	}
	pages, err := discoverDedicatedPages(ctx, cdpURL)
	if err != nil {
		return err
	}
	if len(pages) > 1 {
		return errors.New("identity verification requires exactly one dedicated browser page")
	}
	conn, _, err := websocket.Dial(ctx, cdpURL, &websocket.DialOptions{
		HTTPClient: noProxyHTTPClient(10 * time.Second),
	})
	if err != nil {
		return fmt.Errorf("connect dedicated browser: %w", err)
	}
	defer conn.CloseNow()
	conn.SetReadLimit(1 << 20)
	client := &cdpClient{conn: conn}
	var target pageTarget
	if len(pages) == 0 {
		var created struct {
			TargetID string `json:"targetId"`
		}
		if err := client.call(ctx, "Target.createTarget", map[string]any{"url": "about:blank"}, &created); err != nil {
			return fmt.Errorf("create dedicated browser page: %w", err)
		}
		if created.TargetID == "" {
			return errors.New("create dedicated browser page returned no target")
		}
		target, err = discoverSinglePage(ctx, cdpURL)
		if err != nil {
			return fmt.Errorf("verify created dedicated browser page: %w", err)
		}
		if target.ID != created.TargetID {
			return errors.New("dedicated browser target changed while creating a page")
		}
	} else {
		target = pages[0]
	}
	var attached struct {
		SessionID string `json:"sessionId"`
	}
	if err := client.call(ctx, "Target.attachToTarget", map[string]any{
		"targetId": target.ID,
		"flatten":  true,
	}, &attached); err != nil {
		return fmt.Errorf("attach selected browser page: %w", err)
	}
	if attached.SessionID == "" {
		return errors.New("attach selected browser page returned no session")
	}
	client.sessionID = attached.SessionID
	if err := client.call(ctx, "Page.enable", nil, nil); err != nil {
		return err
	}
	var navigation struct {
		ErrorText string `json:"errorText"`
	}
	if err := client.call(ctx, "Page.navigate", map[string]any{"url": destination}, &navigation); err != nil {
		return err
	}
	if navigation.ErrorText != "" {
		return fmt.Errorf("browser refused Naver navigation: %s", navigation.ErrorText)
	}
	return nil
}

func discoverSinglePage(ctx context.Context, cdpURL string) (pageTarget, error) {
	pages, err := discoverDedicatedPages(ctx, cdpURL)
	if err != nil {
		return pageTarget{}, err
	}
	if len(pages) != 1 {
		return pageTarget{}, errors.New("identity verification requires exactly one dedicated browser page")
	}
	return pages[0], nil
}

func discoverDedicatedPages(ctx context.Context, cdpURL string) ([]pageTarget, error) {
	parsed, err := url.Parse(cdpURL)
	if err != nil || (parsed.Scheme != "ws" && parsed.Scheme != "wss") || parsed.User != nil {
		return nil, errors.New("invalid browser CDP endpoint")
	}
	address := net.ParseIP(parsed.Hostname())
	if address == nil || !address.IsLoopback() || parsed.Port() == "" {
		return nil, errors.New("browser CDP endpoint is not loopback")
	}
	scheme := "http"
	if parsed.Scheme == "wss" {
		scheme = "https"
	}
	listURL := scheme + "://" + net.JoinHostPort(parsed.Hostname(), parsed.Port()) + "/json/list"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, listURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := noProxyHTTPClient(5 * time.Second).Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("browser target discovery returned %d", response.StatusCode)
	}
	var targets []pageTarget
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&targets); err != nil {
		return nil, err
	}
	pages := make([]pageTarget, 0, 1)
	for _, target := range targets {
		if target.Type != "page" {
			continue
		}
		pageURL, err := url.Parse(target.URL)
		if err != nil || (target.URL != "about:blank" && (pageURL.Scheme != "https" || !isNaverHost(pageURL.Hostname()))) {
			return nil, errors.New("dedicated browser page is outside approved Naver hosts")
		}
		pageWS, err := url.Parse(target.WebSocketDebuggerURL)
		if err != nil || pageWS.Scheme != parsed.Scheme || pageWS.Host != parsed.Host || pageWS.User != nil ||
			pageWS.RawQuery != "" || pageWS.Fragment != "" || pageWS.EscapedPath() != "/devtools/page/"+target.ID {
			return nil, errors.New("browser returned an unbound page WebSocket endpoint")
		}
		pages = append(pages, target)
	}
	return pages, nil
}

func noProxyHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout, Transport: &http.Transport{Proxy: nil}}
}

func isNaverHost(host string) bool {
	// Naver's successful sign-in flow lands on www.naver.com. Accept that
	// official landing page only long enough to navigate the same bound target
	// to the generic blog editor. validateIdentityObservation still requires
	// the final evidence to come from blog.naver.com/PostWriteForm.naver.
	return host == "blog.naver.com" || host == "m.blog.naver.com" || host == "nid.naver.com" || host == "www.naver.com"
}

func (c *cdpClient) call(ctx context.Context, method string, params any, out any) error {
	c.next++
	id := c.next
	if err := wsjson.Write(ctx, c.conn, cdpRequest{ID: id, Method: method, Params: params, SessionID: c.sessionID}); err != nil {
		return fmt.Errorf("send CDP %s: %w", method, err)
	}
	for {
		var response cdpResponse
		if err := wsjson.Read(ctx, c.conn, &response); err != nil {
			return fmt.Errorf("read CDP %s: %w", method, err)
		}
		if response.ID != id || (c.sessionID != "" && response.SessionID != c.sessionID) {
			continue
		}
		if response.Error != nil {
			return fmt.Errorf("CDP %s failed (%d): %s", method, response.Error.Code, response.Error.Message)
		}
		if out == nil || len(response.Result) == 0 {
			return nil
		}
		if err := json.Unmarshal(response.Result, out); err != nil {
			return fmt.Errorf("decode CDP %s: %w", method, err)
		}
		return nil
	}
}

type identityObservation struct {
	Href        string          `json:"href"`
	EditorReady bool            `json:"editor_ready"`
	BlogLabel   string          `json:"blog_label"`
	Categories  []NaverCategory `json:"categories"`
}

type identityPreparation struct {
	EditorReady bool   `json:"editor_ready"`
	Ready       bool   `json:"ready"`
	Stage       string `json:"stage"`
	Error       string `json:"error"`
}

func (c *cdpClient) prepareIdentity(ctx context.Context) (identityPreparation, error) {
	var evaluated struct {
		Result struct {
			Value identityPreparation `json:"value"`
		} `json:"result"`
	}
	err := c.call(ctx, "Runtime.evaluate", map[string]any{
		"expression":    identityPreparationScript,
		"returnByValue": true,
		"awaitPromise":  true,
	}, &evaluated)
	return evaluated.Result.Value, err
}

func (c *cdpClient) observe(ctx context.Context) (identityObservation, error) {
	var evaluated struct {
		Result struct {
			Value identityObservation `json:"value"`
		} `json:"result"`
	}
	err := c.call(ctx, "Runtime.evaluate", map[string]any{
		"expression":    identityObservationScript,
		"returnByValue": true,
		"awaitPromise":  true,
	}, &evaluated)
	return evaluated.Result.Value, err
}

func validateIdentityObservation(observation identityObservation) (NaverIdentity, error) {
	current, err := url.Parse(observation.Href)
	if err != nil || current.Scheme != "https" || current.Hostname() != "blog.naver.com" ||
		current.EscapedPath() != "/PostWriteForm.naver" {
		return NaverIdentity{}, errors.New("Naver editor did not stay on the approved blog host")
	}
	blogID := strings.TrimSpace(current.Query().Get("blogId"))
	if !naverBlogID.MatchString(blogID) || !observation.EditorReady {
		return NaverIdentity{}, errors.New("signed-in account did not expose the versioned Naver editor identity")
	}
	byID := make(map[string]string, len(observation.Categories))
	for _, category := range observation.Categories {
		category.ID = strings.TrimSpace(category.ID)
		category.Name = strings.TrimSpace(category.Name)
		if !naverCategoryID.MatchString(category.ID) || category.Name == "" || len(category.Name) > 200 {
			continue
		}
		if previous, exists := byID[category.ID]; exists && previous != category.Name {
			return NaverIdentity{}, errors.New("Naver editor exposed conflicting category metadata")
		}
		byID[category.ID] = category.Name
	}
	if len(byID) == 0 {
		return NaverIdentity{}, errors.New("Naver editor exposed no categories")
	}
	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	categories := make([]NaverCategory, 0, len(ids))
	for _, id := range ids {
		categories = append(categories, NaverCategory{ID: id, Name: byID[id]})
	}
	blogLabel := strings.TrimSpace(observation.BlogLabel)
	if blogLabel == "" || len([]rune(blogLabel)) > 200 {
		blogLabel = blogID
	}
	return NaverIdentity{BlogID: blogID, BlogLabel: blogLabel, Categories: categories}, nil
}
