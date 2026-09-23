package post

import (
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"
)

// naverBlogOrigin is where every accepted address is stored, whichever of the two hosts and
// schemes it was pasted with (POST-77).
const naverBlogOrigin = "https://blog.naver.com"

// ParseNaverBlogURL accepts a Naver Blog post address and returns it normalized, or refuses
// it with ErrPublishedURLInvalid (POST-77). Accepted: http or https, host blog.naver.com or
// its mobile share host m.blog.naver.com in any case, no userinfo, no port, and a path that
// names something below the blog's home — a bare `/` is the home, not a post. Stored:
// https://blog.naver.com plus the path, plus the query when there is one; the fragment is
// dropped and the path's case is kept.
//
// Every refusal is the one wire reason with no params, because the browser's pre-check states
// the rule itself. The shared fixture testdata/published_url/cases.json is the contract with
// that pre-check.
func ParseNaverBlogURL(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if utf8.RuneCountInString(value) > PublishedURLMaxChars {
		return "", fmt.Errorf("%w: longer than %d characters", ErrPublishedURLInvalid, PublishedURLMaxChars)
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("%w: not a URL", ErrPublishedURLInvalid)
	}
	switch {
	case parsed.Scheme != "http" && parsed.Scheme != "https":
		return "", fmt.Errorf("%w: scheme is not http or https", ErrPublishedURLInvalid)
	case parsed.Opaque != "" || parsed.Host == "":
		return "", fmt.Errorf("%w: no host", ErrPublishedURLInvalid)
	case parsed.User != nil:
		return "", fmt.Errorf("%w: carries userinfo", ErrPublishedURLInvalid)
	case strings.Contains(parsed.Host, ":"):
		return "", fmt.Errorf("%w: carries a port", ErrPublishedURLInvalid)
	}
	if host := strings.ToLower(parsed.Hostname()); host != "blog.naver.com" && host != "m.blog.naver.com" {
		return "", fmt.Errorf("%w: host is not blog.naver.com or m.blog.naver.com", ErrPublishedURLInvalid)
	}
	path := parsed.EscapedPath()
	if strings.Trim(path, "/") == "" {
		return "", fmt.Errorf("%w: names no post", ErrPublishedURLInvalid)
	}
	stored := naverBlogOrigin + path
	if parsed.RawQuery != "" {
		stored += "?" + parsed.RawQuery
	}
	return stored, nil
}
