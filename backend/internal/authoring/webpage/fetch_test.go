package webpage

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/text/encoding/korean"
)

// fakeResolver answers lookups from a table, offline, and records every name it was asked.
type fakeResolver struct {
	mu      sync.Mutex
	answers map[string][]string
	lookups []string
}

func (r *fakeResolver) LookupNetIP(_ context.Context, _ string, host string) ([]netip.Addr, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lookups = append(r.lookups, host)
	raws, ok := r.answers[host]
	if !ok {
		return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
	}
	addrs := make([]netip.Addr, 0, len(raws))
	for _, raw := range raws {
		addrs = append(addrs, netip.MustParseAddr(raw))
	}
	return addrs, nil
}

func (r *fakeResolver) asked() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.lookups...)
}

// upstream is a local web server standing in for the internet: names resolve through the
// fake resolver to public-looking addresses the guard admits, and every admitted address is
// dialed at a local listener — port 80 at a plain server and port 443 at a TLS one, both
// serving the same handler, which tells sites apart by Host.
type upstream struct {
	resolver *fakeResolver
	mu       sync.Mutex
	dialed   []netip.AddrPort
	hits     []string
}

func (u *upstream) record(ap netip.AddrPort) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.dialed = append(u.dialed, ap)
}

func (u *upstream) hit(r *http.Request) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.hits = append(u.hits, r.Host+r.URL.RequestURI())
}

func (u *upstream) served() []string {
	u.mu.Lock()
	defer u.mu.Unlock()
	return append([]string(nil), u.hits...)
}

func (u *upstream) dials() []netip.AddrPort {
	u.mu.Lock()
	defer u.mu.Unlock()
	return append([]netip.AddrPort(nil), u.dialed...)
}

const publicA = "93.184.216.34"
const publicB = "93.184.216.35"

func newUpstream(t *testing.T, limits Limits, answers map[string][]string, handler http.HandlerFunc) (*Fetcher, *upstream) {
	t.Helper()
	up := &upstream{resolver: &fakeResolver{answers: answers}}
	serve := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		up.hit(r)
		handler(w, r)
	})
	plain := httptest.NewServer(serve)
	secure := httptest.NewTLSServer(serve)
	t.Cleanup(plain.Close)
	t.Cleanup(secure.Close)
	f := newFetcher(limits, up.resolver, func(ctx context.Context, network string, ap netip.AddrPort) (net.Conn, error) {
		up.record(ap)
		local := plain.Listener.Addr().String()
		if ap.Port() == 443 {
			local = secure.Listener.Addr().String()
		}
		return (&net.Dialer{}).DialContext(ctx, network, local)
	})
	// The local TLS server's certificate names none of the test hosts. Only this test
	// transport skips verification; the guard under test is the dial, not the certificate.
	f.client.Transport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // test-only upstream
	return f, up
}

func page(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	io.WriteString(w, body)
}

const simplePage = `<html><head><title>Simple</title></head><body><article><h2>Section</h2><p>Body text.</p></article></body></html>`

func refusalCode(t *testing.T, err error) string {
	t.Helper()
	var refusal *Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("err = %v (%T), want a *Refusal", err, err)
	}
	return refusal.Code
}

func TestFetchReadsAPublicPageThroughTheGuard(t *testing.T) {
	var accept, agent string
	f, up := newUpstream(t, Limits{}, map[string][]string{"www.example.com": {publicA}}, func(w http.ResponseWriter, r *http.Request) {
		accept, agent = r.Header.Get("Accept"), r.Header.Get("User-Agent")
		page(w, simplePage)
	})
	got, err := f.Fetch(context.Background(), "www.example.com/post")
	if err != nil {
		t.Fatal(err)
	}
	if got.URL != "https://www.example.com/post" || got.Title != "Simple" || got.Structure != "## Section\nBody text." {
		t.Fatalf("page = %+v", got)
	}
	if asked := up.resolver.asked(); len(asked) != 1 || asked[0] != "www.example.com" {
		t.Fatalf("lookups = %v", asked)
	}
	if dials := up.dials(); len(dials) != 1 || dials[0] != netip.MustParseAddrPort(publicA+":443") {
		t.Fatalf("dialed = %v", dials)
	}
	if !strings.Contains(accept, "text/html") || !strings.Contains(agent, "PostPilotReference") {
		t.Fatalf("Accept = %q, User-Agent = %q", accept, agent)
	}
}

func TestFetchRefusesBeforeAnyLookupOrDial(t *testing.T) {
	f, up := newUpstream(t, Limits{}, map[string][]string{"example.com": {publicA}}, func(w http.ResponseWriter, r *http.Request) {
		page(w, simplePage)
	})
	cases := map[string]string{
		"http://127.0.0.1/":              RefusalNotAllowed,
		"http://169.254.169.254/latest/": RefusalNotAllowed,
		"http://example.com:8080/":       RefusalNotAllowed,
		"http://user:pw@example.com/":    RefusalNotAllowed,
		"http://localhost/":              RefusalNotAllowed,
		"ftp://example.com/":             RefusalInvalid,
	}
	for raw, want := range cases {
		_, err := f.Fetch(context.Background(), raw)
		if code := refusalCode(t, err); code != want {
			t.Errorf("Fetch(%q) = %s, want %s", raw, code, want)
		}
	}
	if len(up.resolver.asked()) != 0 || len(up.dials()) != 0 || len(up.served()) != 0 {
		t.Fatalf("refused addresses reached the network: lookups %v, dials %v, hits %v", up.resolver.asked(), up.dials(), up.served())
	}
}

// Every address a name resolves to must be public: one private answer among public ones
// refuses the name, so the connection cannot be steered to it.
func TestFetchRefusesANameWithAnyNonPublicAnswer(t *testing.T) {
	answers := map[string][]string{
		"mixed-private.example":   {publicA, "10.0.0.7"},
		"private-first.example":   {"192.168.0.10", publicA},
		"loopback.example":        {"127.0.0.1"},
		"v6-loopback.example":     {publicA, "::1"},
		"mapped.example":          {"::ffff:192.168.1.1"},
		"metadata.example":        {"169.254.169.254"},
		"v6-metadata.example":     {"2606:4700:4700::1111", "fd00:ec2::254"},
		"azure.example":           {"168.63.129.16"},
		"cgnat.example":           {publicA, "100.64.1.1"},
		"nat64.example":           {"64:ff9b::a00:1"},
		"sixtofour.example":       {"2002:a00:1::1"},
		"linklocal.example":       {"fe80::1"},
		"unspecified.example":     {"0.0.0.0"},
		"127.1":                   {"127.0.0.1"}, // legacy dotted forms a system resolver expands
		"2130706433":              {"127.0.0.1"},
		"rebinding-after.example": {publicB, publicA, "172.20.0.1"},
	}
	f, up := newUpstream(t, Limits{}, answers, func(w http.ResponseWriter, r *http.Request) {
		page(w, simplePage)
	})
	for host := range answers {
		_, err := f.Fetch(context.Background(), "http://"+host+"/")
		if code := refusalCode(t, err); code != RefusalNotAllowed {
			t.Errorf("Fetch(%s) = %s, want %s", host, code, RefusalNotAllowed)
		}
	}
	if len(up.dials()) != 0 || len(up.served()) != 0 {
		t.Fatalf("a refused name was dialed: dials %v, hits %v", up.dials(), up.served())
	}
}

func TestFetchNameThatDoesNotResolveIsUnreachable(t *testing.T) {
	f, up := newUpstream(t, Limits{}, map[string][]string{"empty.example": {}}, func(w http.ResponseWriter, r *http.Request) {
		page(w, simplePage)
	})
	for _, raw := range []string{"https://missing.example/", "https://empty.example/"} {
		_, err := f.Fetch(context.Background(), raw)
		if code := refusalCode(t, err); code != RefusalUnreachable {
			t.Errorf("Fetch(%s) = %s, want %s", raw, code, RefusalUnreachable)
		}
	}
	if len(up.dials()) != 0 {
		t.Fatalf("dialed %v", up.dials())
	}
}

// A redirect is held to the same rules as the typed address: its URL before it is
// followed, and every address its name resolves to when it is dialed.
func TestFetchRefusesARedirectToWhatTheGuardRefuses(t *testing.T) {
	cases := map[string]string{
		"http://10.0.0.5/admin":                    RefusalNotAllowed,
		"http://169.254.169.254/latest/meta-data/": RefusalNotAllowed,
		"http://[::1]/":                            RefusalNotAllowed,
		"http://[::ffff:127.0.0.1]/":               RefusalNotAllowed,
		"http://[fd00:ec2::254]/":                  RefusalNotAllowed,
		"http://internal.example/":                 RefusalNotAllowed, // resolves to a private address
		"http://half-internal.example/":            RefusalNotAllowed, // one public, one private answer
		"http://public.example:8080/":              RefusalNotAllowed,
		"http://user:pw@public.example/":           RefusalNotAllowed,
		"http://localhost/":                        RefusalNotAllowed,
		"http://metadata.google.internal/":         RefusalNotAllowed,
		"file:///etc/passwd":                       RefusalInvalid,
		"gopher://public.example/":                 RefusalInvalid,
	}
	for location, want := range cases {
		t.Run(location, func(t *testing.T) {
			answers := map[string][]string{
				"start.example":         {publicA},
				"public.example":        {publicB},
				"internal.example":      {"10.9.8.7"},
				"half-internal.example": {publicB, "192.168.5.5"},
			}
			f, up := newUpstream(t, Limits{}, answers, func(w http.ResponseWriter, r *http.Request) {
				if r.Host == "start.example" {
					http.Redirect(w, r, location, http.StatusFound)
					return
				}
				page(w, simplePage)
			})
			_, err := f.Fetch(context.Background(), "http://start.example/")
			if code := refusalCode(t, err); code != want {
				t.Fatalf("redirect to %s = %s, want %s", location, code, want)
			}
			if hits := up.served(); len(hits) != 1 || hits[0] != "start.example/" {
				t.Fatalf("hits = %v, want only the first page", hits)
			}
			for _, ap := range up.dials() {
				if ap.Addr() != netip.MustParseAddr(publicA) {
					t.Fatalf("dialed %v after a refused redirect", ap)
				}
			}
		})
	}
}

func TestFetchFollowsThreeRedirectsAndRefusesTheFourth(t *testing.T) {
	answers := map[string][]string{"a.example": {publicA}, "b.example": {publicB}, "c.example": {publicA}, "d.example": {publicB}, "e.example": {publicA}}
	hops := map[string]string{"a.example": "b.example", "b.example": "c.example", "c.example": "d.example", "d.example": "e.example"}
	handler := func(w http.ResponseWriter, r *http.Request) {
		// /stop/<host> serves the page once that host is reached.
		if r.URL.Path == "/stop/"+r.Host {
			page(w, simplePage)
			return
		}
		http.Redirect(w, r, "http://"+hops[r.Host]+r.URL.Path, http.StatusMovedPermanently)
	}

	f, up := newUpstream(t, Limits{}, answers, handler)
	got, err := f.Fetch(context.Background(), "http://a.example/stop/d.example")
	if err != nil {
		t.Fatalf("three redirects: %v", err)
	}
	if got.URL != "http://d.example/stop/d.example" {
		t.Fatalf("URL = %q", got.URL)
	}
	// Each hop's name was looked up again on its own.
	if asked := strings.Join(up.resolver.asked(), ","); asked != "a.example,b.example,c.example,d.example" {
		t.Fatalf("lookups = %s", asked)
	}

	f, up = newUpstream(t, Limits{}, answers, handler)
	_, err = f.Fetch(context.Background(), "http://a.example/stop/e.example")
	if code := refusalCode(t, err); code != RefusalUnreachable {
		t.Fatalf("four redirects = %s, want %s", code, RefusalUnreachable)
	}
	for _, hit := range up.served() {
		if strings.HasPrefix(hit, "e.example") {
			t.Fatalf("the fourth redirect was followed: %v", up.served())
		}
	}
}

func TestFetchCapsTheAnswerSize(t *testing.T) {
	const max = 4096
	fill := func(n int) string {
		head := "<html><body><p>"
		tail := "</p></body></html>"
		return head + strings.Repeat("a", n-len(head)-len(tail)) + tail
	}
	answers := map[string][]string{"big.example": {publicA}}
	cases := []struct {
		name    string
		handler http.HandlerFunc
		want    string
	}{
		{"content-length at the cap", func(w http.ResponseWriter, r *http.Request) {
			body := fill(max)
			w.Header().Set("Content-Length", strconv.Itoa(len(body)))
			page(w, body)
		}, ""},
		{"content-length over the cap", func(w http.ResponseWriter, r *http.Request) {
			body := fill(max + 1)
			w.Header().Set("Content-Length", strconv.Itoa(len(body)))
			page(w, body)
		}, RefusalTooLarge},
		{"chunked at the cap", func(w http.ResponseWriter, r *http.Request) {
			chunked(w, fill(max))
		}, ""},
		{"chunked over the cap", func(w http.ResponseWriter, r *http.Request) {
			chunked(w, fill(max+1))
		}, RefusalTooLarge},
		{"gzip that expands over the cap", func(w http.ResponseWriter, r *http.Request) {
			if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
				page(w, simplePage)
				return
			}
			var b bytes.Buffer
			z := gzip.NewWriter(&b)
			io.WriteString(z, fill(64*max))
			z.Close()
			if b.Len() >= max {
				panic("compressed fixture is not smaller than the cap")
			}
			w.Header().Set("Content-Type", "text/html")
			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Set("Content-Length", strconv.Itoa(b.Len()))
			w.Write(b.Bytes())
		}, RefusalTooLarge},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, _ := newUpstream(t, Limits{MaxBytes: max}, answers, tc.handler)
			_, err := f.Fetch(context.Background(), "http://big.example/")
			if tc.want == "" {
				if err != nil {
					t.Fatalf("err = %v, want the page", err)
				}
				return
			}
			if code := refusalCode(t, err); code != tc.want {
				t.Fatalf("code = %s, want %s", code, tc.want)
			}
		})
	}
}

// chunked writes body in flushed pieces, so it travels without a Content-Length.
func chunked(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "text/html")
	flusher := w.(http.Flusher)
	for len(body) > 0 {
		n := min(1000, len(body))
		io.WriteString(w, body[:n])
		flusher.Flush()
		body = body[n:]
	}
}

func TestDefaultLimitsAreTheDocumentedOnes(t *testing.T) {
	if DefaultLimits != (Limits{Timeout: 10 * time.Second, MaxBytes: 2 << 20, MaxRedirects: 3, MaxChars: 12000}) {
		t.Fatalf("DefaultLimits = %+v", DefaultLimits)
	}
	f := NewFetcher(Limits{})
	if f.limits != DefaultLimits {
		t.Fatalf("zero limits = %+v, want the defaults", f.limits)
	}
	transport := f.client.Transport.(*http.Transport)
	if transport.Proxy != nil {
		t.Fatal("the fetcher's transport takes a proxy")
	}
	if f.client.Timeout != 10*time.Second {
		t.Fatalf("client timeout = %v", f.client.Timeout)
	}
}

// The real 2 MB cap, with the default limits, on a body that announces no length.
func TestFetchRefusesAChunkedBodyOverTwoMegabytes(t *testing.T) {
	answers := map[string][]string{"big.example": {publicA}}
	f, _ := newUpstream(t, Limits{}, answers, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		piece := strings.Repeat("<p>x</p>", 4096)
		flusher := w.(http.Flusher)
		for sent := 0; sent <= 2<<20; sent += len(piece) {
			if _, err := io.WriteString(w, piece); err != nil {
				return
			}
			flusher.Flush()
		}
	})
	_, err := f.Fetch(context.Background(), "http://big.example/")
	if code := refusalCode(t, err); code != RefusalTooLarge {
		t.Fatalf("code = %s, want %s", code, RefusalTooLarge)
	}

	f, _ = newUpstream(t, Limits{}, answers, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("Content-Length", strconv.Itoa(2<<20+1))
		w.WriteHeader(http.StatusOK)
	})
	_, err = f.Fetch(context.Background(), "http://big.example/")
	if code := refusalCode(t, err); code != RefusalTooLarge {
		t.Fatalf("announced length code = %s, want %s", code, RefusalTooLarge)
	}
}

func TestFetchReadsOnlyWebPages(t *testing.T) {
	answers := map[string][]string{"site.example": {publicA}}
	cases := map[string]string{
		"application/json":                     RefusalNotHTML,
		"image/png":                            RefusalNotHTML,
		"text/plain; charset=utf-8":            RefusalNotHTML,
		"application/pdf":                      RefusalNotHTML,
		"":                                     RefusalNotHTML,
		"text/html":                            "",
		"TEXT/HTML; charset=UTF-8":             "",
		"application/xhtml+xml; charset=utf-8": "",
	}
	for contentType, want := range cases {
		t.Run(contentType, func(t *testing.T) {
			f, _ := newUpstream(t, Limits{}, answers, func(w http.ResponseWriter, r *http.Request) {
				if contentType == "" {
					w.Header()["Content-Type"] = nil // no sniffing either
				} else {
					w.Header().Set("Content-Type", contentType)
				}
				io.WriteString(w, simplePage)
			})
			got, err := f.Fetch(context.Background(), "http://site.example/")
			if want == "" {
				if err != nil || got.Title != "Simple" {
					t.Fatalf("page = %+v, err = %v", got, err)
				}
				return
			}
			if code := refusalCode(t, err); code != want {
				t.Fatalf("code = %s, want %s", code, want)
			}
		})
	}
}

func TestFetchErrorStatusIsUnreachable(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusForbidden, http.StatusInternalServerError, http.StatusFound} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			f, _ := newUpstream(t, Limits{}, map[string][]string{"site.example": {publicA}}, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/html")
				w.WriteHeader(status) // a 302 without a Location is not followed
				io.WriteString(w, simplePage)
			})
			_, err := f.Fetch(context.Background(), "http://site.example/")
			if code := refusalCode(t, err); code != RefusalUnreachable {
				t.Fatalf("code = %s, want %s", code, RefusalUnreachable)
			}
		})
	}
}

func TestFetchTimeoutCoversHeadersAndBody(t *testing.T) {
	answers := map[string][]string{"slow.example": {publicA}}
	cases := map[string]http.HandlerFunc{
		"no answer": func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-r.Context().Done():
			case <-time.After(5 * time.Second):
			}
		},
		"body stalls": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			io.WriteString(w, "<html><body><p>first")
			w.(http.Flusher).Flush()
			select {
			case <-r.Context().Done():
			case <-time.After(5 * time.Second):
			}
		},
	}
	for name, handler := range cases {
		t.Run(name, func(t *testing.T) {
			f, _ := newUpstream(t, Limits{Timeout: 200 * time.Millisecond}, answers, handler)
			started := time.Now()
			_, err := f.Fetch(context.Background(), "http://slow.example/")
			if code := refusalCode(t, err); code != RefusalUnreachable {
				t.Fatalf("code = %s, want %s", code, RefusalUnreachable)
			}
			if elapsed := time.Since(started); elapsed > 3*time.Second {
				t.Fatalf("took %v, want the 200ms timeout", elapsed)
			}
		})
	}
}

// A proxy from the environment would connect to an address the guard never saw.
func TestFetchIgnoresAProxyInTheEnvironment(t *testing.T) {
	proxied := false
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxied = true
		page(w, simplePage)
	}))
	defer proxy.Close()
	t.Setenv("HTTP_PROXY", proxy.URL)
	t.Setenv("HTTPS_PROXY", proxy.URL)
	t.Setenv("http_proxy", proxy.URL)
	t.Setenv("https_proxy", proxy.URL)
	t.Setenv("NO_PROXY", "")
	f, up := newUpstream(t, Limits{}, map[string][]string{"site.example": {publicA}}, func(w http.ResponseWriter, r *http.Request) {
		page(w, simplePage)
	})
	if _, err := f.Fetch(context.Background(), "http://site.example/"); err != nil {
		t.Fatal(err)
	}
	if proxied || len(up.served()) != 1 {
		t.Fatalf("proxied = %v, direct hits = %v", proxied, up.served())
	}
}

// The dial itself refuses a port other than 80/443 and a refused literal, whatever let the
// address through to it.
func TestDialHoldsPortAndAddressOnItsOwn(t *testing.T) {
	f := newFetcher(Limits{}, &fakeResolver{answers: map[string][]string{"site.example": {publicA}}}, func(context.Context, string, netip.AddrPort) (net.Conn, error) {
		t.Error("dialed")
		return nil, errors.New("dialed")
	})
	dial := f.dial(&net.Dialer{Timeout: time.Second})
	for address, want := range map[string]string{
		"site.example:22":       RefusalNotAllowed,
		"site.example:8080":     RefusalNotAllowed,
		"127.0.0.1:80":          RefusalNotAllowed,
		"[::ffff:10.0.0.1]:443": RefusalNotAllowed,
		"169.254.169.254:80":    RefusalNotAllowed,
		"no-port":               RefusalInvalid,
	} {
		_, err := dial(context.Background(), "tcp", address)
		if code := refusalCode(t, err); code != want {
			t.Errorf("dial(%s) = %s, want %s", address, code, want)
		}
	}
}

// A shared naver.me link redirects to the desktop post, whose HTML is only a frame: the
// fetcher follows it to the post's mobile page instead.
func TestFetchFollowsANaverShortLinkToTheMobilePost(t *testing.T) {
	locations := map[string]string{
		"path form":     "https://blog.naver.com/foo/223000000001?from=naverme",
		"postview form": "http://blog.naver.com/PostView.naver?blogId=foo&logNo=223000000001",
	}
	for name, location := range locations {
		t.Run(name, func(t *testing.T) {
			answers := map[string][]string{"naver.me": {publicA}, "blog.naver.com": {publicB}, "m.blog.naver.com": {publicB}}
			f, up := newUpstream(t, Limits{}, answers, func(w http.ResponseWriter, r *http.Request) {
				switch r.Host {
				case "naver.me":
					http.Redirect(w, r, location, http.StatusFound)
				case "m.blog.naver.com":
					page(w, `<html><head><meta property="og:title" content="모바일 글"></head><body><div class="se-main-container"><div class="se-component se-sectionTitle"><p>소제목</p></div></div></body></html>`)
				default:
					page(w, `<html><body><iframe src="/PostView.naver"></iframe></body></html>`)
				}
			})
			got, err := f.Fetch(context.Background(), "https://naver.me/xAbCdEf")
			if err != nil {
				t.Fatal(err)
			}
			if got.URL != "https://m.blog.naver.com/foo/223000000001" || got.Title != "모바일 글" || got.Structure != "## 소제목" {
				t.Fatalf("page = %+v", got)
			}
			for _, hit := range up.served() {
				if strings.HasPrefix(hit, "blog.naver.com") {
					t.Fatalf("the desktop frame was read: %v", up.served())
				}
			}
		})
	}
}

func TestFetchDecodesTheDeclaredCharset(t *testing.T) {
	encode := func(s string) []byte {
		b, err := korean.EUCKR.NewEncoder().Bytes([]byte(s))
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	cases := map[string]struct {
		contentType string
		body        []byte
	}{
		"header charset":  {"text/html; charset=euc-kr", encode(`<html><head><title>한글 제목</title></head><body><p>본문 문단</p></body></html>`)},
		"meta charset":    {"text/html", encode(`<html><head><meta charset="euc-kr"><title>한글 제목</title></head><body><p>본문 문단</p></body></html>`)},
		"meta http-equiv": {"text/html", encode(`<html><head><meta http-equiv="Content-Type" content="text/html; charset=EUC-KR"><title>한글 제목</title></head><body><p>본문 문단</p></body></html>`)},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f, _ := newUpstream(t, Limits{}, map[string][]string{"old.example": {publicA}}, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tc.contentType)
				w.Write(tc.body)
			})
			got, err := f.Fetch(context.Background(), "http://old.example/")
			if err != nil {
				t.Fatal(err)
			}
			if got.Title != "한글 제목" || got.Structure != "본문 문단" {
				t.Fatalf("page = %+v", got)
			}
		})
	}
}

func TestFetchPageWithNothingToReadIsEmpty(t *testing.T) {
	f, _ := newUpstream(t, Limits{}, map[string][]string{"bare.example": {publicA}}, func(w http.ResponseWriter, r *http.Request) {
		page(w, `<html><body><nav><a href="/">Home</a></nav><script>var a = 1</script><footer>© 2026</footer></body></html>`)
	})
	_, err := f.Fetch(context.Background(), "http://bare.example/")
	if code := refusalCode(t, err); code != RefusalEmpty {
		t.Fatalf("code = %s, want %s", code, RefusalEmpty)
	}
}

func TestFetchBoundsTheStructureToMaxChars(t *testing.T) {
	var body strings.Builder
	body.WriteString("<html><head><title>Long</title></head><body>")
	for i := 0; i < 1000; i++ {
		fmt.Fprintf(&body, "<p>문단 %03d 의 내용입니다.</p>", i)
	}
	body.WriteString("</body></html>")
	f, _ := newUpstream(t, Limits{}, map[string][]string{"long.example": {publicA}}, func(w http.ResponseWriter, r *http.Request) {
		page(w, body.String())
	})
	got, err := f.Fetch(context.Background(), "http://long.example/")
	if err != nil {
		t.Fatal(err)
	}
	if n := len([]rune(got.Structure)); n > 12000 || n < 11000 {
		t.Fatalf("structure is %d characters, want just under the default 12,000", n)
	}
}

// The system resolver reports an A record as an IPv4-mapped address. That answer is the
// IPv4 address it carries: checked and dialed as such, not refused as a mapped literal.
func TestFetchAdmitsAResolverIPv4AnswerInItsMappedForm(t *testing.T) {
	f, up := newUpstream(t, Limits{}, map[string][]string{"v4.example": {"::ffff:" + publicA}, "v4-private.example": {"::ffff:10.1.1.1"}}, func(w http.ResponseWriter, r *http.Request) {
		page(w, simplePage)
	})
	if _, err := f.Fetch(context.Background(), "http://v4.example/"); err != nil {
		t.Fatalf("a mapped public answer was refused: %v", err)
	}
	if dials := up.dials(); len(dials) != 1 || dials[0] != netip.MustParseAddrPort(publicA+":80") {
		t.Fatalf("dialed = %v, want the unmapped IPv4 address", dials)
	}
	_, err := f.Fetch(context.Background(), "http://v4-private.example/")
	if code := refusalCode(t, err); code != RefusalNotAllowed {
		t.Fatalf("mapped private answer = %s, want %s", code, RefusalNotAllowed)
	}
}

// blockingResolver stands in for a lookup that never answers on its own.
type blockingResolver struct{ done chan time.Time }

func (r *blockingResolver) LookupNetIP(ctx context.Context, _ string, _ string) ([]netip.Addr, error) {
	defer func() { r.done <- time.Now() }()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(5 * time.Second):
		return nil, errors.New("lookup outlived its read")
	}
}

// The transport detaches a dial from its request, so the read's deadline must reach the
// lookup and the address attempts on its own: a name with several blackholed addresses
// must not keep dialing after Fetch has given up.
func TestFetchDeadlineEndsTheDialItStarted(t *testing.T) {
	const timeout = 200 * time.Millisecond
	t.Run("blackholed addresses", func(t *testing.T) {
		resolver := &fakeResolver{answers: map[string][]string{"blackhole.example": {publicA, publicB, "93.184.216.36", "93.184.216.37", "93.184.216.38"}}}
		var mu sync.Mutex
		attempts := 0
		exited := make(chan time.Time, 8)
		f := newFetcher(Limits{Timeout: timeout}, resolver, func(ctx context.Context, _ string, _ netip.AddrPort) (net.Conn, error) {
			mu.Lock()
			attempts++
			mu.Unlock()
			defer func() { exited <- time.Now() }()
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(5 * time.Second):
				return nil, errors.New("dial outlived its read")
			}
		})
		started := time.Now()
		_, err := f.Fetch(context.Background(), "http://blackhole.example/")
		returned := time.Since(started)
		if code := refusalCode(t, err); code != RefusalUnreachable {
			t.Fatalf("code = %s, want %s", code, RefusalUnreachable)
		}
		if returned > 2*time.Second {
			t.Fatalf("Fetch returned after %v", returned)
		}
		select {
		case at := <-exited:
			if at.Sub(started) > timeout+time.Second {
				t.Fatalf("dial attempt ran %v past a %v read", at.Sub(started), timeout)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("the dial goroutine is still running after Fetch returned")
		}
		// Once the deadline has passed no further address is tried.
		time.Sleep(50 * time.Millisecond)
		mu.Lock()
		defer mu.Unlock()
		if attempts != 1 {
			t.Fatalf("attempts = %d, want the loop to stop at the deadline", attempts)
		}
	})
	t.Run("lookup that never answers", func(t *testing.T) {
		resolver := &blockingResolver{done: make(chan time.Time, 1)}
		f := newFetcher(Limits{Timeout: timeout}, resolver, nil)
		started := time.Now()
		_, err := f.Fetch(context.Background(), "http://silent.example/")
		if code := refusalCode(t, err); code != RefusalUnreachable {
			t.Fatalf("code = %s, want %s", code, RefusalUnreachable)
		}
		select {
		case at := <-resolver.done:
			if at.Sub(started) > timeout+time.Second {
				t.Fatalf("lookup ran %v past a %v read", at.Sub(started), timeout)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("the lookup is still running after Fetch returned")
		}
	})
}
