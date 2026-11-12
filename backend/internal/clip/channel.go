package clip

import (
	"errors"
	"net/url"
	"strings"
	"unicode/utf8"
)

// A clip channel chooses its directory and upload text, never its composition.
const (
	ChannelNaver     = "naver_clip"
	ChannelInstagram = "instagram_clip"
)

func ValidChannel(value string) bool { return value == ChannelNaver || value == ChannelInstagram }

// An omitted channel is the earlier clients' and projects' Naver channel.
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

// A posted URL is an https URL on its channel's platform host or a subdomain of
// one (CLIP-232, decided in T759); nothing is fetched to check it.
const PublishedURLMaxChars = 2048

var PublishedURLHosts = map[string][]string{
	ChannelNaver:     {"naver.com", "naver.me"},
	ChannelInstagram: {"instagram.com"},
}

var ErrPublishedURLInvalid = errors.New("clip published url is invalid")

// NormalizePublishedURL trims the owner's URL and refuses one that is not an https URL on
// the channel's platform host.
func NormalizePublishedURL(channel, raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" || !utf8.ValidString(value) || utf8.RuneCountInString(value) > PublishedURLMaxChars {
		return "", ErrPublishedURLInvalid
	}
	u, err := url.Parse(value)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Opaque != "" {
		return "", ErrPublishedURLInvalid
	}
	host := strings.ToLower(u.Hostname())
	for _, allowed := range PublishedURLHosts[DefaultChannel(channel)] {
		if host == allowed || strings.HasSuffix(host, "."+allowed) {
			return value, nil
		}
	}
	return "", ErrPublishedURLInvalid
}
