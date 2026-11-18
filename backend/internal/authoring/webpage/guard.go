// Package webpage reads a public web page as a template's form reference: the address is
// held to http(s), every address it resolves to — and every address a redirect leads to —
// must be a public unicast one, the answer must be a bounded HTML page, and what comes back
// is the page's title and its body structure, never the page itself.
package webpage

import (
	"net/netip"
	"net/url"
	"regexp"
	"strings"
)

// Refusal codes. Each is a display-safe word the screen turns into its own sentence.
const (
	// RefusalInvalid: not an http(s) address at all.
	RefusalInvalid = "invalid"
	// RefusalNotAllowed: an address the guard refuses — a private, loopback, link-local,
	// shared (CGNAT) or cloud-metadata address, a port other than 80/443, or credentials
	// in the address.
	RefusalNotAllowed = "not_allowed"
	// RefusalUnreachable: the page did not answer, answered with an error, or redirected
	// more than MaxRedirects times.
	RefusalUnreachable = "unreachable"
	// RefusalNotHTML: the answer is not a web page.
	RefusalNotHTML = "not_html"
	// RefusalTooLarge: the page is larger than MaxBytes.
	RefusalTooLarge = "too_large"
	// RefusalEmpty: the page holds no title, heading, paragraph or photo to read.
	RefusalEmpty = "empty"
)

// Refusal is a page that cannot be read as a reference, with the code saying why.
type Refusal struct {
	Code  string
	cause error
}

func (r *Refusal) Error() string {
	if r.cause != nil {
		return "web page reference refused: " + r.Code + ": " + r.cause.Error()
	}
	return "web page reference refused: " + r.Code
}
func (r *Refusal) Unwrap() error { return r.cause }

func refuse(code string, cause error) *Refusal { return &Refusal{Code: code, cause: cause} }

// blocked are the ranges no reference may reach besides what netip already names
// (loopback, private, link-local, multicast, unspecified).
var blocked = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),          // "this network"
	netip.MustParsePrefix("100.64.0.0/10"),      // shared address space (CGNAT), incl. 100.100.100.200 metadata
	netip.MustParsePrefix("192.0.0.0/24"),       // IETF protocol assignments
	netip.MustParsePrefix("192.0.2.0/24"),       // documentation
	netip.MustParsePrefix("198.18.0.0/15"),      // benchmarking
	netip.MustParsePrefix("198.51.100.0/24"),    // documentation
	netip.MustParsePrefix("203.0.113.0/24"),     // documentation
	netip.MustParsePrefix("240.0.0.0/4"),        // reserved, incl. broadcast
	netip.MustParsePrefix("168.63.129.16/32"),   // Azure platform endpoint (public, metadata-like)
	netip.MustParsePrefix("64:ff9b::/96"),       // NAT64: may embed a private IPv4 address
	netip.MustParsePrefix("64:ff9b:1::/48"),     // local-use NAT64
	netip.MustParsePrefix("2001::/32"),          // Teredo: embeds an IPv4 address
	netip.MustParsePrefix("2001:db8::/32"),      // documentation
	netip.MustParsePrefix("2002::/16"),          // 6to4: embeds an IPv4 address
	netip.MustParsePrefix("100::/64"),           // discard-only
	netip.MustParsePrefix("fec0::/10"),          // deprecated site-local
	netip.MustParsePrefix("fd00:ec2::254/128"),  // AWS metadata over IPv6 (also inside fc00::/7)
	netip.MustParsePrefix("169.254.169.254/32"), // cloud metadata (also link-local)
}

// Allowed reports whether a page may be fetched from addr: a public unicast address only.
// An IPv4-mapped IPv6 address is refused as written, whatever IPv4 address it carries; a
// resolver's answers are unmapped before they are checked (see Fetcher.dial).
func Allowed(addr netip.Addr) bool {
	if !addr.IsValid() || addr.Is4In6() {
		return false
	}
	if addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() ||
		addr.IsInterfaceLocalMulticast() || addr.IsMulticast() || addr.IsUnspecified() || !addr.IsGlobalUnicast() {
		return false
	}
	for _, prefix := range blocked {
		if prefix.Contains(addr) {
			return false
		}
	}
	return true
}

// checkURL holds an address to what may be fetched before any lookup: http or https, a
// host, no credentials, and the default port of its scheme.
func checkURL(u *url.URL) *Refusal {
	if u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.Opaque != "" {
		return refuse(RefusalInvalid, nil)
	}
	if u.User != nil {
		return refuse(RefusalNotAllowed, nil)
	}
	if port := u.Port(); port != "" && port != "80" && port != "443" {
		return refuse(RefusalNotAllowed, nil)
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return refuse(RefusalNotAllowed, nil)
	}
	if addr, err := netip.ParseAddr(host); err == nil && !Allowed(addr) {
		return refuse(RefusalNotAllowed, nil)
	}
	return nil
}

var naverPath = regexp.MustCompile(`^/([A-Za-z0-9_-]+)/([0-9]+)/?$`)
var naverLogNo = regexp.MustCompile(`^[0-9]+$`)
var naverBlogID = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// Parse reads an address the owner typed into the URL that is fetched: a missing scheme
// is https, and a Naver blog post address — blog.naver.com/<id>/<logNo>, its PostView
// form, or m.blog.naver.com — becomes its mobile page, whose HTML holds the post body
// rather than the frame the desktop page loads it into.
func Parse(raw string) (*url.URL, *Refusal) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > 2048 {
		return nil, refuse(RefusalInvalid, nil)
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, refuse(RefusalInvalid, err)
	}
	u.Scheme = strings.ToLower(u.Scheme)
	if refusal := checkURL(u); refusal != nil {
		return nil, refusal
	}
	u.Fragment = ""
	return naverMobile(u), nil
}

func naverMobile(u *url.URL) *url.URL {
	host := strings.ToLower(u.Hostname())
	if host != "blog.naver.com" && host != "www.blog.naver.com" && host != "m.blog.naver.com" {
		return u
	}
	query := u.Query()
	blogID, logNo := "", ""
	if match := naverPath.FindStringSubmatch(u.Path); match != nil {
		blogID, logNo = match[1], match[2]
	} else if strings.EqualFold(u.Path, "/PostView.naver") || strings.EqualFold(u.Path, "/PostView.nhn") || strings.EqualFold(u.Path, "/PostView.nhn/") {
		blogID, logNo = query.Get("blogId"), query.Get("logNo")
	} else if id := strings.Trim(u.Path, "/"); naverBlogID.MatchString(id) && query.Get("logNo") != "" {
		blogID, logNo = id, query.Get("logNo")
	}
	if !naverBlogID.MatchString(blogID) || !naverLogNo.MatchString(logNo) {
		if host == "m.blog.naver.com" {
			return &url.URL{Scheme: "https", Host: "m.blog.naver.com", Path: u.Path, RawQuery: u.RawQuery}
		}
		return u
	}
	return &url.URL{Scheme: "https", Host: "m.blog.naver.com", Path: "/" + blogID + "/" + logNo}
}
