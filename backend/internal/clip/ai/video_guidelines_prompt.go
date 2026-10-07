package ai

import (
	"slices"
	"strings"

	"github.com/postpilot/backend/internal/clip"
)

// videoRuleResponsibility is a stage/output declaration, never a classifier of
// arbitrary owner text. The owning guideline supplies each stock rule's text
// and applicability; old untyped defaults retain their original behavior.
func videoRuleResponsibility(mode string) (string, []string) {
	switch mode {
	case "flow", "flow-measured-speech":
		return "clip-write", []string{"plan", "placements"}
	case "flow-follow-storyline", "flow-follow-storyline-measured-speech":
		return "clip-write", []string{"placements"}
	case "flow-revision":
		return "clip-revise", []string{"placements"}
	case "narration":
		return "clip-write", []string{"captions"}
	case "narration-revision":
		return "clip-revise", []string{"captions"}
	case "storyline", "storyline-revision":
		return "clip-storyline", []string{"plan", "placements"}
	case "spoken-script":
		return "clip-write", []string{"narration", "plan", "placements"}
	case "spoken-script-follow-storyline":
		return "clip-write", []string{"narration"}
	case "spoken-script-revision":
		return "clip-revise", []string{"narration"}
	default:
		return "", nil
	}
}

func videoStockApplies(rule clip.VideoStockRule, mode string) bool {
	stage, outputs := videoRuleResponsibility(mode)
	for _, applicability := range rule.Applicability {
		if applicability.Stage != stage || stage == "" {
			continue
		}
		for _, output := range outputs {
			if slices.Contains(applicability.Outputs, output) {
				return true
			}
		}
	}
	return false
}

func videoGuidelinesFor(g clip.VideoGuidelines, mode string) clip.VideoGuidelines {
	if mode == "observe" {
		return clip.VideoGuidelines{}
	}
	selected := clip.VideoGuidelines{Owner: g.Owner}
	if g.Stock == nil {
		selected.Defaults = g.Defaults
		return selected
	}
	for _, rule := range g.Stock {
		if videoStockApplies(rule, mode) {
			selected.Defaults = append(selected.Defaults, rule.Text)
		}
	}
	return selected
}

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
