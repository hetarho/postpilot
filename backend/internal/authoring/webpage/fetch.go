package webpage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"time"

	"golang.org/x/net/html/charset"
)

// Limits bound one read. Zero values take the defaults.
type Limits struct {
	// Timeout covers the whole read: lookups, every redirect and the body.
	Timeout time.Duration
	// MaxBytes is the largest answer read; a larger one is RefusalTooLarge.
	MaxBytes int64
	// MaxRedirects is how many redirects are followed, each re-checked.
	MaxRedirects int
	// MaxChars bounds the extracted structure, in Unicode characters.
	MaxChars int
}

// DefaultLimits: about ten seconds, 2 MB, three redirects and 12,000 characters.
var DefaultLimits = Limits{Timeout: 10 * time.Second, MaxBytes: 2 << 20, MaxRedirects: 3, MaxChars: 12000}

// Resolver looks a host up. net.DefaultResolver is one; tests pass their own.
type Resolver interface {
	LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error)
}

// Fetcher reads web pages under the guard. It uses no proxy from the environment: a proxy
// would make the connection to an address the guard never saw.
type Fetcher struct {
	limits   Limits
	resolver Resolver
	client   *http.Client
	// dialTo, when set, replaces how a checked address is dialed. Only tests set it, to
	// reach a local server under a public-looking address the guard admits, or to stand in
	// for an address that never answers.
	dialTo func(ctx context.Context, network string, target netip.AddrPort) (net.Conn, error)
}

// deadlineKey carries one Fetch's absolute deadline to the dials it starts. The transport
// detaches a dial from its request's cancellation (a finished dial may serve a later
// request) and keeps only the context's values, so without it a lookup and every address
// attempt would run on after the read itself gave up.
type deadlineKey struct{}

// NewFetcher builds a fetcher with these limits and the system resolver.
func NewFetcher(limits Limits) *Fetcher {
	return newFetcher(limits, net.DefaultResolver, nil)
}

func newFetcher(limits Limits, resolver Resolver, dialTo func(context.Context, string, netip.AddrPort) (net.Conn, error)) *Fetcher {
	if limits.Timeout <= 0 {
		limits.Timeout = DefaultLimits.Timeout
	}
	if limits.MaxBytes <= 0 {
		limits.MaxBytes = DefaultLimits.MaxBytes
	}
	if limits.MaxRedirects <= 0 {
		limits.MaxRedirects = DefaultLimits.MaxRedirects
	}
	if limits.MaxChars <= 0 {
		limits.MaxChars = DefaultLimits.MaxChars
	}
	f := &Fetcher{limits: limits, resolver: resolver, dialTo: dialTo}
	dialer := &net.Dialer{Timeout: limits.Timeout}
	transport := &http.Transport{
		Proxy:                  nil,
		DialContext:            f.dial(dialer),
		ForceAttemptHTTP2:      true,
		MaxIdleConns:           4,
		IdleConnTimeout:        30 * time.Second,
		TLSHandshakeTimeout:    limits.Timeout,
		ResponseHeaderTimeout:  limits.Timeout,
		MaxResponseHeaderBytes: 64 << 10,
	}
	f.client = &http.Client{
		Transport: transport,
		Timeout:   limits.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > limits.MaxRedirects {
				return refuse(RefusalUnreachable, errors.New("too many redirects"))
			}
			// The address a redirect leads to is held to the same rules as the first one;
			// every address it resolves to is checked again when it is dialed.
			if refusal := checkURL(req.URL); refusal != nil {
				return refusal
			}
			// A shared short link (naver.me) lands on a Naver post's desktop frame: read the
			// post's mobile page instead, as a typed address would be.
			if mobile := naverMobile(req.URL); mobile != req.URL {
				req.URL, req.Host = mobile, ""
			}
			return nil
		},
	}
	return f
}

// dial resolves the host itself and connects only to an address the guard admits. Every
// address the name resolves to must be admitted, so a name that answers with one public
// and one private address cannot be steered to the private one.
func (f *Fetcher) dial(dialer *net.Dialer) func(ctx context.Context, network, address string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		// The lookup and every address attempt end with the read that asked for them.
		deadline, ok := ctx.Value(deadlineKey{}).(time.Time)
		if !ok {
			deadline = time.Now().Add(f.limits.Timeout)
		}
		ctx, cancel := context.WithDeadline(ctx, deadline)
		defer cancel()
		host, portText, err := net.SplitHostPort(address)
		if err != nil {
			return nil, refuse(RefusalInvalid, err)
		}
		port, err := strconv.ParseUint(portText, 10, 16)
		if err != nil || (port != 80 && port != 443) {
			return nil, refuse(RefusalNotAllowed, nil)
		}
		var addrs []netip.Addr
		if literal, err := netip.ParseAddr(host); err == nil {
			addrs = []netip.Addr{literal}
		} else {
			addrs, err = f.resolver.LookupNetIP(ctx, "ip", host)
			if err != nil || len(addrs) == 0 {
				return nil, refuse(RefusalUnreachable, err)
			}
			// The system resolver reports every IPv4 answer in its IPv4-mapped form; the
			// IPv4 address is what is checked and dialed. (A typed mapped literal is
			// refused by Allowed.)
			for i := range addrs {
				addrs[i] = addrs[i].Unmap()
			}
		}
		for _, addr := range addrs {
			if !Allowed(addr) {
				return nil, refuse(RefusalNotAllowed, nil)
			}
		}
		var last error
		for _, addr := range addrs {
			if err := ctx.Err(); err != nil {
				last = err
				break
			}
			target := netip.AddrPortFrom(addr, uint16(port))
			var conn net.Conn
			if f.dialTo != nil {
				conn, err = f.dialTo(ctx, network, target)
			} else {
				conn, err = dialer.DialContext(ctx, network, target.String())
			}
			if err == nil {
				return conn, nil
			}
			last = err
		}
		return nil, refuse(RefusalUnreachable, last)
	}
}

// Page is what a reference reads from a web page: its title and its body's structure.
type Page struct {
	URL       string
	Title     string
	Structure string
}

// Fetch reads one page. Every failure is a *Refusal.
func (f *Fetcher) Fetch(ctx context.Context, raw string) (Page, error) {
	target, refusal := Parse(raw)
	if refusal != nil {
		return Page{}, refusal
	}
	ctx, cancel := context.WithTimeout(ctx, f.limits.Timeout)
	defer cancel()
	if deadline, ok := ctx.Deadline(); ok {
		ctx = context.WithValue(ctx, deadlineKey{}, deadline)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return Page{}, refuse(RefusalInvalid, err)
	}
	request.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9")
	request.Header.Set("Accept-Language", "ko,en;q=0.8")
	request.Header.Set("User-Agent", "Mozilla/5.0 (compatible; PostPilotReference/1.0)")
	response, err := f.client.Do(request)
	if err != nil {
		var refused *Refusal
		if errors.As(err, &refused) {
			return Page{}, refused
		}
		return Page{}, refuse(RefusalUnreachable, err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return Page{}, refuse(RefusalUnreachable, fmt.Errorf("status %d", response.StatusCode))
	}
	contentType := response.Header.Get("Content-Type")
	media, _, err := mime.ParseMediaType(contentType)
	if err != nil || (media != "text/html" && media != "application/xhtml+xml") {
		return Page{}, refuse(RefusalNotHTML, nil)
	}
	if response.ContentLength > f.limits.MaxBytes {
		return Page{}, refuse(RefusalTooLarge, nil)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, f.limits.MaxBytes+1))
	if err != nil {
		return Page{}, refuse(RefusalUnreachable, err)
	}
	if int64(len(body)) > f.limits.MaxBytes {
		return Page{}, refuse(RefusalTooLarge, nil)
	}
	return readPage(ctx, response.Request.URL, body, contentType, f.limits.MaxChars)
}

// readPage decodes the page to UTF-8 and extracts its structure.
func readPage(ctx context.Context, at *url.URL, body []byte, contentType string, maxChars int) (Page, error) {
	reader, err := charset.NewReader(bytes.NewReader(body), contentType)
	if err != nil {
		return Page{}, refuse(RefusalNotHTML, err)
	}
	title, structure, err := extract(ctx, reader, maxChars)
	if ctx.Err() != nil {
		return Page{}, refuse(RefusalUnreachable, ctx.Err())
	}
	if err != nil {
		return Page{}, refuse(RefusalNotHTML, err)
	}
	if title == "" && structure == "" {
		return Page{}, refuse(RefusalEmpty, nil)
	}
	return Page{URL: at.String(), Title: title, Structure: structure}, nil
}
