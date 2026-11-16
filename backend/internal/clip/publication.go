package clip

import (
	"errors"
	"net/url"
	"strings"
	"unicode/utf8"
)

// TicksOnly is a patch of the advisory checklist ticks alone, the one write a
// finalized project still admits and no running job can conflict with
// (CLIP-231). A future patch field also has to be absent.
func (p ProjectPatch) TicksOnly() bool {
	if p.ChecklistTicks == nil {
		return false
	}
	p.ChecklistTicks = nil
	// Clearing an already absent link is internal storage maintenance.
	p.StaleLinkedPostIdentity = nil
	return p == (ProjectPatch{})
}

// A posted URL is an https URL on 네이버 클립's platform host or a subdomain of
// one (CLIP-232, decided in T759); nothing is fetched to check it.
const PublishedURLMaxChars = 2048

var PublishedURLHosts = []string{"naver.com", "naver.me"}

var ErrPublishedURLInvalid = errors.New("clip published url is invalid")

// NormalizePublishedURL trims the owner's URL and refuses one that is not an https URL on
// the platform host.
func NormalizePublishedURL(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" || !utf8.ValidString(value) || utf8.RuneCountInString(value) > PublishedURLMaxChars {
		return "", ErrPublishedURLInvalid
	}
	u, err := url.Parse(value)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Opaque != "" {
		return "", ErrPublishedURLInvalid
	}
	host := strings.ToLower(u.Hostname())
	for _, allowed := range PublishedURLHosts {
		if host == allowed || strings.HasSuffix(host, "."+allowed) {
			return value, nil
		}
	}
	return "", ErrPublishedURLInvalid
}
