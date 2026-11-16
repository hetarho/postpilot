package clip

import (
	"errors"
	"net/url"
	"strings"
	"unicode/utf8"
)

// Every clip is one kind of video (CLIP-209). The stored channel survives until
// the columns go (T868) and only finds a project's one stored upload-text row;
// no request or patch can set it.
const ChannelNaver = "naver_clip"

// An omitted stored channel is the Naver one every project began with.
func DefaultChannel(value string) string {
	if value == "" {
		return ChannelNaver
	}
	return value
}

// MetadataOnly admits directory metadata and advisory checklist ticks after
// finalization without opening any canonical setting (CLIP-209, CLIP-231). A
// future patch field also has to be absent.
func (p ProjectPatch) MetadataOnly() bool {
	if p.Channel == nil && p.ChecklistTicks == nil {
		return false
	}
	p.Channel, p.ChecklistTicks = nil, nil
	// Clearing an already absent link is internal storage maintenance.
	p.StaleLinkedPostIdentity = nil
	return p == (ProjectPatch{})
}

// TicksOnly is a patch of the advisory ticks alone, which no running job can conflict with.
func (p ProjectPatch) TicksOnly() bool {
	return p.Channel == nil && p.MetadataOnly()
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
