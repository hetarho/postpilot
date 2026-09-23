// Package naversearch adapts one page of the 네이버 검색 API's blog search to plain titles and
// descriptions (QUAL-17, QUAL-18). It is the only way the product reads Naver blog text: the
// API is the sanctioned route, and blog.naver.com itself is never fetched (QUAL-33).
//
// It schedules, retries and logs nothing. The daily phrase batch that calls it owns when a
// page is read and what a failure means.
package naversearch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	baseURL  = "https://openapi.naver.com"
	blogPath = "/v1/search/blog.json"
	// pageSize is the API's own maximum, and the batch reads three such pages per 분야.
	pageSize = 100
	// maxStart is the API's own bound on `start`.
	maxStart          = 1000
	requestTimeout    = 10 * time.Second
	maxBodyBytes      = int64(1 << 20)
	maxErrorBodyBytes = int64(4 << 10)
)

// Client reads the blog search with one application's credentials.
type Client struct {
	clientID, clientSecret string
	http                   *http.Client
	baseURL                string
	timeout                time.Duration
}

// New returns a client for one registered application. A nil http client means
// http.DefaultClient.
func New(clientID, clientSecret string, client *http.Client) *Client {
	if client == nil {
		client = http.DefaultClient
	}
	return &Client{clientID: clientID, clientSecret: clientSecret, http: client, baseURL: baseURL, timeout: requestTimeout}
}

// Item is one search result as plain text: the post's title and the API's summary passage.
// Nothing else the API answers is kept, because the corpus is these two alone (QUAL-18).
type Item struct {
	Title, Description string
}

// StatusError is a non-200 answer: its HTTP status and Naver's errorCode, empty when the body
// was not the documented error JSON. A 403 means the application does not have 검색 enabled;
// a 429 is the daily or per-second quota.
type StatusError struct {
	Status int
	Code   string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("naver blog search: status %d, errorCode %q", e.Status, e.Code)
}

// SearchBlog reads one page of up to 100 results for query, ranked by similarity, starting at
// the 1-based position start. A blank query or a start outside 1…1000 is refused before any
// request is sent. Each request is bounded by the client's own timeout even when ctx has
// none, and ctx still cancels it.
func (c *Client) SearchBlog(ctx context.Context, query string, start int) ([]Item, error) {
	if strings.TrimSpace(query) == "" {
		return nil, errors.New("naver blog search: the query is blank")
	}
	if start < 1 || start > maxStart {
		return nil, fmt.Errorf("naver blog search: start %d is outside 1…%d", start, maxStart)
	}
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	parameters := url.Values{
		"query":   {query},
		"display": {strconv.Itoa(pageSize)},
		"start":   {strconv.Itoa(start)},
		"sort":    {"sim"},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+blogPath+"?"+parameters.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("naver blog search: build the request: %w", err)
	}
	// The credentials travel only in these headers and never in the URL, so a transport
	// error, which quotes the URL, cannot carry them.
	request.Header.Set("X-Naver-Client-Id", c.clientID)
	request.Header.Set("X-Naver-Client-Secret", c.clientSecret)

	response, err := c.http.Do(request)
	if err != nil {
		return nil, fmt.Errorf("naver blog search: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		var failure struct {
			ErrorCode string `json:"errorCode"`
		}
		_ = json.NewDecoder(io.LimitReader(response.Body, maxErrorBodyBytes)).Decode(&failure)
		return nil, &StatusError{Status: response.StatusCode, Code: failure.ErrorCode}
	}

	var page struct {
		Items []struct {
			Title       string `json:"title"`
			Description string `json:"description"`
		} `json:"items"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, maxBodyBytes)).Decode(&page); err != nil {
		return nil, fmt.Errorf("naver blog search: decode the answer: %w", err)
	}
	items := make([]Item, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, Item{Title: plainText(item.Title), Description: plainText(item.Description)})
	}
	return items, nil
}

// plainText removes the API's match highlighting and then decodes entities. The order matters:
// stripping first means an author's own escaped &lt;b&gt; survives as a literal <b> rather than
// being taken for highlighting.
func plainText(value string) string {
	value = strings.ReplaceAll(value, "<b>", "")
	value = strings.ReplaceAll(value, "</b>", "")
	return strings.TrimSpace(html.UnescapeString(value))
}
