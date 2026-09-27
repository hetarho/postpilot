package ai

import (
	"strings"

	"github.com/postpilot/backend/internal/clip"
)

// The [영상 지침] block's fixed words, Korean for every language like the post prompt's
// [작문 지침] (GUIDE-15, GUIDE-17). The texts themselves are in the project's language.
const (
	videoGuidelinesHeading      = "[영상 지침]"
	videoDefaultGuidelinesLabel = "기본 지침:"
	videoOwnerGuidelinesLabel   = "사용자 지침:"
	videoGuidelinePrecedence    = "영상 지침은 이 영상의 흐름과 자막을 어떻게 만들지 정합니다. 사용자 지침이 기본 지침과 충돌하면 사용자 지침을 따르고, 같은 묶음 안에서 충돌하면 먼저 적힌 지침을 따르세요."
)

// videoGuidelineBlock is the clip's frozen 영상 지침 as the flow, narration and revision system
// prompts end: the 기본 지침 group, then the owner's, each line `- text`, then the precedence
// sentence. Both groups empty adds no bytes at all, so a clip with none sends the prefix it sent
// before 영상 지침 existed. It comes LAST, after every fixed rule and any revision block, so the
// fixed prefix the provider caches stays the same for every account.
func videoGuidelineBlock(g clip.VideoGuidelines) string {
	if g.Empty() {
		return ""
	}
	var out strings.Builder
	out.WriteString("\n" + videoGuidelinesHeading)
	for _, group := range []struct {
		label string
		texts []string
	}{{videoDefaultGuidelinesLabel, g.Defaults}, {videoOwnerGuidelinesLabel, g.Owner}} {
		if len(group.texts) == 0 {
			continue
		}
		out.WriteString("\n" + group.label)
		for _, text := range group.texts {
			out.WriteString("\n- " + strings.ReplaceAll(text, "\n", "\n  "))
		}
	}
	out.WriteString("\n" + videoGuidelinePrecedence + "\n")
	return out.String()
}
