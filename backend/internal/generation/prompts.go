package generation

import (
	"fmt"
	"strings"
)

const ObservePrompt = `사진마다 파일명을 정확히 대응해 관찰 사실만 반환하세요. 추측하거나 이야기를 만들지 마세요.
출력은 설명이나 마크다운 없이 {"observations":[{"file":"...","scene":"...","mood":"...","visible_text":"...","objects":[],"people_present":false}]} 형태의 JSON 객체 하나여야 합니다.`

const WritePrompt = `첨부 사진 관찰과 메모를 바탕으로 자연스러운 한국어 블로그 글을 작성하세요.
반드시 하나의 문단마다 TEXT 블록 하나만 사용하세요.
IMAGE 블록은 제공된 정확한 파일명만 사용하고, 목록에 없는 이미지를 절대 만들어내지 마세요.
IMAGE 블록은 사진이 글의 흐름상 가장 자연스러운 위치에 오도록 배치하세요.
출력은 설명이나 마크다운 없이 {"title":"...","summary":"...","tags":[],"blocks":[]} 형태의 JSON 객체 하나여야 합니다.
각 block은 type, content, level, file, alt, caption, items 필드를 사용하며 type은 TEXT, HEADING, IMAGE, QUOTE, LIST 중 하나입니다.`

// purposePrecedence keeps the brief from quietly overriding the voice. A brief like
// "정보성 리뷰, 담담하게" reads as a style instruction to the model unless the split is stated:
// the purpose owns genre, the voice owns register. It is prompt text, so it lives in code
// beside the section it belongs to rather than in configuration (ARCHITECTURE §4).
const purposePrecedence = "용도는 글의 내용·구성·포함할 정보를 정하고, 문체·종결어미·어휘는 위의 말투 프로필을 따릅니다. 지침이 문체와 충돌하면 말투 프로필을 우선하세요."

// writePurposeSection appends the frozen brief AFTER the complete voice profile and before
// the per-post material. That position is load-bearing twice over: the profile prefix stays
// byte-identical across posts of different purposes (PRD §5's caching note), and the brief
// stays in the stable half, so every revision of one post re-injects the identical block.
//
// A nil brief writes nothing at all — the prompt for a post without a purpose must be
// byte-identical to the one this code produced before purposes existed.
func writePurposeSection(out *strings.Builder, purpose *PurposeBrief) {
	if purpose == nil {
		return
	}
	fmt.Fprintf(out, "\n\n[글의 용도: %s]", purpose.Name)
	if purpose.Description != "" {
		fmt.Fprintf(out, "\n이 글의 용도: %s", purpose.Description)
	}
	fmt.Fprintf(out, "\n작성 지침:\n%s", purpose.Instructions)
	fmt.Fprintf(out, "\n%s", purposePrecedence)
}

func BuildWritePrompt(profile Profile, observations []Observation, memo, title string, filenames []string, targetLength *int, purpose *PurposeBrief) (string, string) {
	var stable strings.Builder
	stable.WriteString(WritePrompt)
	fmt.Fprintf(&stable, "\ntitle, 한 줄 summary, %d–%d개의 tags, blocks를 반환하세요.", TagsMin, TagsMax)
	stable.WriteString("\n\n[스타일가이드]\n")
	stable.WriteString(profile.Styleguide)
	stable.WriteString("\n\n[활성 대조 규칙]\n")
	stable.WriteString(profile.ActiveRules)
	stable.WriteString("\n\n[글 예시 발췌]")
	for i, excerpt := range profile.Excerpts {
		fmt.Fprintf(&stable, "\n%d. %s", i+1, excerpt)
	}
	stable.WriteString("\n예시의 고유 사실, 주제, 문구를 복사하지 말고 문체 특징만 참고하세요.")
	stable.WriteString("\n\n[사용자 규칙]\n")
	stable.WriteString(profile.Rules)
	endingMax := profile.EndingMaxConsecutive
	if endingMax <= 0 {
		endingMax = 2
	}
	stable.WriteString("\n\n[종결어미 제약]\n")
	if targetLength != nil {
		fmt.Fprintf(&stable, "목표 길이: 약 %d자. ", *targetLength)
	}
	fmt.Fprintf(&stable, "프로필의 측정된 종결어미 분포를 따르고 같은 종결어미를 %d문장보다 많이 연속 사용하지 마세요.", endingMax)
	writePurposeSection(&stable, purpose)

	photoMaterial := "첨부 사진이 없습니다. 이미지 없이 메모만으로 작성하세요."
	if len(filenames) > 0 {
		photoMaterial = "첨부 파일명(정확히 일치해야 함): " + strings.Join(filenames, ", ") +
			"\n사진 관찰: " + marshalPromptJSON(observationsForPrompt(observations))
	}
	perPost := fmt.Sprintf("[이번 글]\n가제: %s\n메모: %s\n%s", title, memo, photoMaterial)
	return stable.String(), perPost
}

func observationsForPrompt(observations []Observation) []observationJSON {
	wire := make([]observationJSON, 0, len(observations))
	for _, observation := range observations {
		wire = append(wire, observationJSON{
			File: observation.File, Scene: observation.Scene, Mood: observation.Mood,
			VisibleText: observation.VisibleText, Objects: observation.Objects,
			PeoplePresent: observation.PeoplePresent,
		})
	}
	return wire
}
