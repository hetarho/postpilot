package naversearch

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const (
	testID     = "client-id-7f3a"
	testSecret = "client-secret-91bc"
)

// testClient points a client at an httptest server, the way fxrate's tests swap the endpoint.
func testClient(t *testing.T, handler http.HandlerFunc) (*Client, *int32) {
	t.Helper()
	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	client := New(testID, testSecret, server.Client())
	client.baseURL = server.URL
	return client, &requests
}

func TestSearchBlogRequestContract(t *testing.T) {
	if baseURL != "https://openapi.naver.com" || blogPath != "/v1/search/blog.json" {
		t.Fatalf("the endpoint moved: %s%s", baseURL, blogPath)
	}
	for name, test := range map[string]struct {
		query    string
		start    int
		rawQuery string
	}{
		// url.Values sorts its keys and encodes a space as +, as the docs' Java sample does.
		"a field query with a space": {"패션 미용", 101, "display=100&query=%ED%8C%A8%EC%85%98+%EB%AF%B8%EC%9A%A9&sort=sim&start=101"},
		"the first page":             {"맛집", 1, "display=100&query=%EB%A7%9B%EC%A7%91&sort=sim&start=1"},
	} {
		t.Run(name, func(t *testing.T) {
			client, requests := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/v1/search/blog.json" || r.URL.RawQuery != test.rawQuery {
					t.Errorf("request = %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
				}
				if r.Header.Get("X-Naver-Client-Id") != testID || r.Header.Get("X-Naver-Client-Secret") != testSecret {
					t.Errorf("credential headers = %q, %q", r.Header.Get("X-Naver-Client-Id"), r.Header.Get("X-Naver-Client-Secret"))
				}
				if strings.Contains(r.URL.String(), testID) || strings.Contains(r.URL.String(), testSecret) {
					t.Errorf("a credential reached the URL: %s", r.URL)
				}
				_, _ = w.Write([]byte(`{"items":[]}`))
			})
			if _, err := client.SearchBlog(context.Background(), test.query, test.start); err != nil {
				t.Fatal(err)
			}
			if got := atomic.LoadInt32(requests); got != 1 {
				t.Fatalf("requests = %d, want exactly one", got)
			}
		})
	}
}

func TestSearchBlogReturnsPlainText(t *testing.T) {
	client, _ := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{
			"lastBuildDate": "Wed, 24 Sep 2026 10:39:37 +0900", "total": 2, "start": 1, "display": 2,
			"items": [
				{"title": "<b>성수</b> 카페 &quot;추천&quot;", "link": "https://blog.naver.com/a/1",
				 "description": "라떼 &amp; 디저트, 사장님&#39;s pick &lt;b&gt;굵게&lt;/b&gt;", "bloggername": "a", "postdate": "20260920"},
				{"title": "  <b>성수동</b> 브런치 ", "description": "", "bloggername": "b", "postdate": "20260921"}
			]
		}`))
	})
	items, err := client.SearchBlog(context.Background(), "성수 카페", 1)
	if err != nil {
		t.Fatal(err)
	}
	want := []Item{
		{Title: `성수 카페 "추천"`, Description: "라떼 & 디저트, 사장님's pick <b>굵게</b>"},
		{Title: "성수동 브런치", Description: ""},
	}
	if !reflect.DeepEqual(items, want) {
		t.Fatalf("items =\n%q\nwant\n%q", items, want)
	}
}

func TestSearchBlogEmptyItems(t *testing.T) {
	client, _ := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"total":0,"start":1,"display":0,"items":[]}`))
	})
	items, err := client.SearchBlog(context.Background(), "아무도 쓰지 않은 말", 1)
	if err != nil || items == nil || len(items) != 0 {
		t.Fatalf("items = %#v, err = %v; want an empty slice and no error", items, err)
	}
}

func TestSearchBlogStatusErrors(t *testing.T) {
	for name, test := range map[string]struct {
		status int
		body   string
		code   string
	}{
		"401":          {http.StatusUnauthorized, `{"errorMessage":"Authentication failed (인증에 실패하였습니다.)","errorCode":"024"}`, "024"},
		"429":          {http.StatusTooManyRequests, `{"errorMessage":"Rate limit exceeded","errorCode":"012"}`, "012"},
		"500":          {http.StatusInternalServerError, `{"errorMessage":"System Error (시스템 에러)","errorCode":"SE99"}`, "SE99"},
		"500 not JSON": {http.StatusInternalServerError, `<html>bad gateway</html>`, ""},
	} {
		t.Run(name, func(t *testing.T) {
			client, _ := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(test.body))
			})
			_, err := client.SearchBlog(context.Background(), "맛집", 1)
			var status *StatusError
			if !errors.As(err, &status) || status.Status != test.status || status.Code != test.code {
				t.Fatalf("error = %#v, want status %d and code %q", err, test.status, test.code)
			}
			if strings.Contains(err.Error(), testID) || strings.Contains(err.Error(), testSecret) {
				t.Fatalf("a credential reached the error: %v", err)
			}
		})
	}
}

func TestSearchBlogRefusesAnInvalidRequest(t *testing.T) {
	client, requests := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"items":[]}`))
	})
	for name, test := range map[string]struct {
		query string
		start int
	}{
		"a blank query":  {"  ", 1},
		"an empty query": {"", 1},
		"start 0":        {"맛집", 0},
		"start 1001":     {"맛집", 1001},
		"a negative one": {"맛집", -1},
	} {
		if _, err := client.SearchBlog(context.Background(), test.query, test.start); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
	if got := atomic.LoadInt32(requests); got != 0 {
		t.Fatalf("%d requests were sent for refused searches", got)
	}
	// The bounds themselves are legal.
	for _, start := range []int{1, 1000} {
		if _, err := client.SearchBlog(context.Background(), "맛집", start); err != nil {
			t.Errorf("start %d was refused: %v", start, err)
		}
	}
}

func TestSearchBlogTimesOut(t *testing.T) {
	client, _ := testClient(t, func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	})
	client.timeout = 50 * time.Millisecond
	began := time.Now()
	if _, err := client.SearchBlog(context.Background(), "맛집", 1); err == nil {
		t.Fatal("a request that never answers returned no error")
	}
	if elapsed := time.Since(began); elapsed > time.Second {
		t.Fatalf("the request took %s, beyond its own timeout", elapsed)
	}

	// The caller's cancellation still ends it, long before the client's own bound.
	client.timeout = time.Minute
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)
	began = time.Now()
	if _, err := client.SearchBlog(ctx, "맛집", 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("a cancelled request returned %v", err)
	}
	if elapsed := time.Since(began); elapsed > time.Second {
		t.Fatalf("the cancelled request took %s", elapsed)
	}
}
