package quality

import (
	"fmt"
	"strings"
)

// The one rule text each metric offers (QUAL-13), in Korean and English. M1 and M2 name the
// measured string itself (QUAL-14), inserted between 「」 or quotes so no Korean particle ever hangs
// on it; M3 and M4 name nothing (QUAL-43, QUAL-44). Every text closes by yielding to natural
// writing (QUAL-45), states no exposure gain, and licenses no fact the source lacks (GEN-16). The
// writing brief quotes them verbatim in its toggletip.
const (
	titleSaturationRuleKorean  = "제목에 「%s」 단어를 넣지 마세요. 최근 발행한 글 제목에 이미 자주 쓰인 단어입니다. 억지로 비슷한 말로 바꾸지 말고, 제목을 자연스럽게 다시 짜세요."
	titleSaturationRuleEnglish = "Do not put the word \"%s\" in the title; it already appears in many of the account's recent published titles. Rather than forcing a near-synonym in its place, rework the title so it reads naturally."

	crossPostRuleKorean  = "「%s」 문장을 그대로 쓰지 마세요. 최근 발행한 여러 글에 똑같이 들어간 문장입니다. 같은 뜻이 필요하면 이 글의 내용에 맞게 자연스럽게 새로 쓰세요."
	crossPostRuleEnglish = "Do not reuse the passage \"%s\" word for word; the same passage appears in several recent published posts. If this post needs that point, write it fresh in words that fit this post."

	repetitionRuleKorean  = "본문에서 한 명사를 계속 되풀이하지 마세요. 반복되는 곳은 읽기에 자연스러운 범위에서 다른 표현으로 바꾸거나 줄이되, 어색한 동의어를 억지로 넣지는 마세요. 제목에 쓴 대상은 본문에서도 다루세요."
	repetitionRuleEnglish = "Do not keep repeating one noun through the body; where it repeats, vary or drop it only as far as it still reads naturally, never forcing an awkward synonym. Cover in the body what the title names."

	compositionRuleKorean  = "본문을 문단과 사진만으로 구성하지 말고, 소제목(HEADING)·목록(LIST)·인용(QUOTE) 중 내용에 맞는 것을 섞어 서로 다른 블록 종류를 세 가지 이상 쓰세요. 내용에 맞지 않는 블록을 억지로 넣거나, 블록을 채우려고 자료에 없는 내용을 만들지는 마세요."
	compositionRuleEnglish = "Do not build the body from paragraphs and photos alone: mix in whichever of HEADING, LIST and QUOTE the material fits, so the post uses at least three distinct block types. Never force a block the content does not fit, and never invent anything the source lacks to fill one."
)

// RuleText renders a metric's rule text in the post's target language. M1 and M2 insert named
// verbatim, and with nothing named they render nothing, so no caller can offer a ban with nothing
// in it and an account with nothing published bans nothing (QUAL-15).
func RuleText(m Metric, lang Language, named string) (string, bool) {
	english := lang == LanguageEnglish
	pick := func(korean, englishText string) string {
		if english {
			return englishText
		}
		return korean
	}
	switch m {
	case MetricTitleSaturation:
		if strings.TrimSpace(named) == "" {
			return "", false
		}
		return fmt.Sprintf(pick(titleSaturationRuleKorean, titleSaturationRuleEnglish), named), true
	case MetricCrossPostPhrases:
		if strings.TrimSpace(named) == "" {
			return "", false
		}
		return fmt.Sprintf(pick(crossPostRuleKorean, crossPostRuleEnglish), named), true
	case MetricInPostRepetition:
		return pick(repetitionRuleKorean, repetitionRuleEnglish), true
	case MetricComposition:
		return pick(compositionRuleKorean, compositionRuleEnglish), true
	default:
		return "", false
	}
}
