package webpage

import (
	"errors"
	"net/netip"
	"strings"
	"testing"
)

func TestAllowedAdmitsOnlyPublicUnicast(t *testing.T) {
	refused := map[string]string{
		"127.0.0.1":              "loopback",
		"127.255.255.254":        "loopback range",
		"::1":                    "IPv6 loopback",
		"10.0.0.1":               "private 10/8",
		"172.16.0.1":             "private 172.16/12",
		"172.31.255.255":         "private 172.16/12 top",
		"192.168.1.1":            "private 192.168/16",
		"fc00::1":                "unique local",
		"fd12:3456::1":           "unique local fd",
		"169.254.0.1":            "link-local",
		"fe80::1":                "IPv6 link-local",
		"fe80::1%eth0":           "IPv6 link-local with zone",
		"100.64.0.1":             "CGNAT bottom",
		"100.127.255.255":        "CGNAT top",
		"100.100.100.200":        "Alibaba metadata inside CGNAT",
		"169.254.169.254":        "cloud metadata",
		"fd00:ec2::254":          "AWS IPv6 metadata",
		"168.63.129.16":          "Azure platform endpoint",
		"64:ff9b::a00:1":         "NAT64 of 10.0.0.1",
		"64:ff9b::a9fe:a9fe":     "NAT64 of 169.254.169.254",
		"64:ff9b::808:808":       "NAT64 of a public address is still NAT64",
		"64:ff9b:1::1":           "local-use NAT64",
		"2002:a00:1::1":          "6to4 of 10.0.0.1",
		"2002:7f00:1::1":         "6to4 of 127.0.0.1",
		"2001::1":                "Teredo",
		"::ffff:127.0.0.1":       "IPv4-mapped loopback",
		"::ffff:10.0.0.1":        "IPv4-mapped private",
		"::ffff:169.254.169.254": "IPv4-mapped metadata",
		"::ffff:100.64.0.1":      "IPv4-mapped CGNAT",
		"::ffff:168.63.129.16":   "IPv4-mapped Azure endpoint",
		"::ffff:8.8.8.8":         "IPv4-mapped, even of a public address",
		"::ffff:93.184.216.34":   "IPv4-mapped, even of a public address",
		"0.0.0.0":                "unspecified",
		"::":                     "IPv6 unspecified",
		"0.1.2.3":                "this network",
		"224.0.0.1":              "multicast",
		"239.255.255.250":        "multicast SSDP",
		"ff02::1":                "IPv6 link-local multicast",
		"ff05::2":                "IPv6 site multicast",
		"255.255.255.255":        "broadcast",
		"240.0.0.1":              "reserved",
		"192.0.0.1":              "IETF assignments",
		"192.0.2.1":              "documentation",
		"198.18.0.1":             "benchmarking",
		"198.51.100.1":           "documentation 2",
		"203.0.113.1":            "documentation 3",
		"2001:db8::1":            "IPv6 documentation",
		"100::1":                 "discard-only",
		"fec0::1":                "site-local",
	}
	for raw, why := range refused {
		addr, err := netip.ParseAddr(raw)
		if err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
		if Allowed(addr) {
			t.Errorf("Allowed(%s) = true, want refused (%s)", raw, why)
		}
	}
	if Allowed(netip.Addr{}) {
		t.Error("Allowed(zero Addr) = true")
	}

	admitted := []string{
		"93.184.216.34",
		"8.8.8.8",
		"1.1.1.1",
		"100.63.255.255", // just below CGNAT
		"100.128.0.0",    // just above CGNAT
		"172.32.0.1",     // just above 172.16/12
		"168.63.129.17",  // the Azure endpoint's neighbour
		"169.255.0.1",    // just above link-local
		"2606:4700:4700::1111",
		"2001:4860:4860::8888", // outside Teredo's 2001::/32
	}
	for _, raw := range admitted {
		if !Allowed(netip.MustParseAddr(raw)) {
			t.Errorf("Allowed(%s) = false, want admitted", raw)
		}
	}
}

func TestParseRefusesWhatTheGuardDoesNotFetch(t *testing.T) {
	cases := []struct {
		raw  string
		code string
	}{
		{"", RefusalInvalid},
		{"   ", RefusalInvalid},
		{"https://example.com/" + strings.Repeat("a", 2048), RefusalInvalid},
		{"ftp://example.com/file", RefusalInvalid},
		{"file:///etc/passwd", RefusalInvalid},
		{"gopher://example.com/", RefusalInvalid},
		{"javascript://example.com/%0aalert(1)", RefusalInvalid},
		{"http://", RefusalInvalid},
		{"http:///path", RefusalInvalid},
		{"https://exa mple.com/", RefusalInvalid},
		{"http://user:pass@example.com/", RefusalNotAllowed},
		{"https://user@example.com/", RefusalNotAllowed},
		{"http://example.com:8080/", RefusalNotAllowed},
		{"https://example.com:8443/", RefusalNotAllowed},
		{"http://example.com:22/", RefusalNotAllowed},
		{"http://example.com:0/", RefusalNotAllowed},
		{"localhost", RefusalNotAllowed},
		{"localhost:80", RefusalNotAllowed},
		{"http://localhost/", RefusalNotAllowed},
		{"http://LOCALHOST/", RefusalNotAllowed},
		{"http://localhost./", RefusalNotAllowed},
		{"http://api.localhost/", RefusalNotAllowed},
		{"http://printer.local/", RefusalNotAllowed},
		{"http://metadata.google.internal/computeMetadata/v1/", RefusalNotAllowed},
		{"http://127.0.0.1/", RefusalNotAllowed},
		{"http://[::1]/", RefusalNotAllowed},
		{"http://10.0.0.1/", RefusalNotAllowed},
		{"http://169.254.169.254/latest/meta-data/", RefusalNotAllowed},
		{"http://[fd00:ec2::254]/", RefusalNotAllowed},
		{"http://168.63.129.16/", RefusalNotAllowed},
		{"http://100.100.100.200/", RefusalNotAllowed},
		{"http://[::ffff:127.0.0.1]/", RefusalNotAllowed},
		{"http://[64:ff9b::a00:1]/", RefusalNotAllowed},
		{"http://[::ffff:8.8.8.8]/", RefusalNotAllowed},
		{"http://[::ffff:93.184.216.34]/", RefusalNotAllowed},
		{"http://0.0.0.0/", RefusalNotAllowed},
	}
	for _, tc := range cases {
		u, refusal := Parse(tc.raw)
		if refusal == nil {
			t.Errorf("Parse(%.60q) = %v, want refusal %s", tc.raw, u, tc.code)
			continue
		}
		if refusal.Code != tc.code {
			t.Errorf("Parse(%.60q) refusal = %s, want %s", tc.raw, refusal.Code, tc.code)
		}
		var asRefusal *Refusal
		if !errors.As(error(refusal), &asRefusal) || !strings.Contains(refusal.Error(), tc.code) {
			t.Errorf("Parse(%.60q) error %q does not name its code", tc.raw, refusal.Error())
		}
	}
}

func TestParseNormalizesAnAcceptedAddress(t *testing.T) {
	cases := map[string]string{
		"example.com":                     "https://example.com",
		"  example.com/a?b=1  ":           "https://example.com/a?b=1",
		"http://example.com:80/a":         "http://example.com:80/a",
		"https://example.com:443/a#frag":  "https://example.com:443/a",
		"HTTPS://Example.com/A":           "https://Example.com/A",
		"http://93.184.216.34/page":       "http://93.184.216.34/page",
		"https://[2606:4700:4700::1111]/": "https://[2606:4700:4700::1111]/",
	}
	for raw, want := range cases {
		u, refusal := Parse(raw)
		if refusal != nil {
			t.Errorf("Parse(%q) refused: %v", raw, refusal)
			continue
		}
		if u.String() != want {
			t.Errorf("Parse(%q) = %q, want %q", raw, u.String(), want)
		}
	}
}

func TestParseRewritesNaverBlogPostsToTheirMobilePage(t *testing.T) {
	cases := map[string]string{
		"https://blog.naver.com/foo_bar-1/223456789012":                                     "https://m.blog.naver.com/foo_bar-1/223456789012",
		"blog.naver.com/foo/223456789012/":                                                  "https://m.blog.naver.com/foo/223456789012",
		"http://blog.naver.com/foo/223456789012?from=search#comment":                        "https://m.blog.naver.com/foo/223456789012",
		"https://www.blog.naver.com/foo/1":                                                  "https://m.blog.naver.com/foo/1",
		"https://BLOG.NAVER.COM/foo/1":                                                      "https://m.blog.naver.com/foo/1",
		"https://blog.naver.com/PostView.naver?blogId=foo&logNo=223456789012&redirect=Dlog": "https://m.blog.naver.com/foo/223456789012",
		"https://blog.naver.com/PostView.nhn?blogId=foo&logNo=1":                            "https://m.blog.naver.com/foo/1",
		"https://blog.naver.com/postview.naver?blogId=foo&logNo=1":                          "https://m.blog.naver.com/foo/1",
		"https://blog.naver.com/foo?Redirect=Log&logNo=223456789012":                        "https://m.blog.naver.com/foo/223456789012",
		"https://m.blog.naver.com/foo/1":                                                    "https://m.blog.naver.com/foo/1",
		"https://m.blog.naver.com/PostView.naver?blogId=foo&logNo=1":                        "https://m.blog.naver.com/foo/1",
		"http://m.blog.naver.com/foo":                                                       "https://m.blog.naver.com/foo",
		// Not a post: left as typed.
		"https://blog.naver.com/foo": "https://blog.naver.com/foo",
		// A blog id or log number outside their alphabets is never spliced into a path.
		"https://blog.naver.com/PostView.naver?blogId=../admin&logNo=1": "https://blog.naver.com/PostView.naver?blogId=../admin&logNo=1",
		"https://blog.naver.com/PostView.naver?blogId=foo&logNo=1abc":   "https://blog.naver.com/PostView.naver?blogId=foo&logNo=1abc",
		// Other hosts, including look-alikes, are not Naver blogs.
		"https://cafe.naver.com/foo/1":               "https://cafe.naver.com/foo/1",
		"https://blog.naver.com.evil.example/foo/1":  "https://blog.naver.com.evil.example/foo/1",
		"https://notblog.naver.com/foo/223456789012": "https://notblog.naver.com/foo/223456789012",
	}
	for raw, want := range cases {
		u, refusal := Parse(raw)
		if refusal != nil {
			t.Errorf("Parse(%q) refused: %v", raw, refusal)
			continue
		}
		if u.String() != want {
			t.Errorf("Parse(%q) = %q, want %q", raw, u.String(), want)
		}
	}
}

func TestRefusalUnwrapsItsCause(t *testing.T) {
	cause := errors.New("dial tcp: boom")
	err := error(refuse(RefusalUnreachable, cause))
	if !errors.Is(err, cause) {
		t.Fatal("refusal does not unwrap to its cause")
	}
	if got := err.Error(); !strings.Contains(got, RefusalUnreachable) || !strings.Contains(got, "boom") {
		t.Fatalf("Error() = %q", got)
	}
	if got := refuse(RefusalEmpty, nil).Error(); got != "web page reference refused: empty" {
		t.Fatalf("Error() = %q", got)
	}
}
