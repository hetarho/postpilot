package clip

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
