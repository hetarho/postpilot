package generation

import (
	"fmt"
	"strings"
)

const ObservePrompt = `사진마다 파일명을 정확히 대응해 관찰 사실만 반환하세요. 추측하거나 이야기를 만들지 마세요.
출력은 설명이나 마크다운 없이 {"observations":[{"file":"...","scene":"...","mood":"...","visible_text":"...","objects":[],"people_present":false}]} 형태의 JSON 객체 하나여야 합니다.`

// koreanGrounding / englishGrounding are the built-in grounding constraint (plan 16): the
// writer may state no concrete fact the memo and the photo observations do not carry. It
// ships as fixed prompt text because the invented-fact failure — "주인분에게 건네받았다" for
// an unmanned store — is one every account hits with zero setup, so it cannot wait for a
// user-authored 지침. It is deliberately disjoint from NaturalnessBaseline, which owns style
// and nothing else, and from the observe prompt, which never sees it ([I3]).
const koreanGrounding = "메모와 사진 관찰에 없는 구체적 사실은 쓰지 마세요. 사람과의 상호작용, 시설, 서비스, 대화, 가격처럼 확인되지 않은 내용을 지어내지 마세요."

const englishGrounding = "State no concrete fact that the memo and the photo observations do not carry. Do not invent interactions with people, facilities, services, conversations, or prices."

// The write pass holds the memo and the observations, so it can also be told to omit what it
// cannot confirm. The revise pass does not receive either one, so the same instruction there
// would license stripping real facts out of blocks the request never mentioned — directly
// against the byte-for-byte preservation rule beside it. Hence the split: the core prohibition
// is shared, and each pass carries the clause its own material can support.
const koreanGroundingWriteScope = "확인할 수 없는 것은 생략하거나 관찰된 범위 안에서만 쓰세요."

const englishGroundingWriteScope = "Omit whatever you cannot confirm, or keep it within what was observed."

const koreanGroundingReviseScope = "이 기준은 수정 요청으로 새로 쓰거나 손대는 문장에만 적용하고, 요청 밖의 기존 문장은 사실 확인 없이 그대로 두세요."

const englishGroundingReviseScope = "Apply this only to sentences the request makes you write or touch; leave every sentence outside the request exactly as it is, without re-checking its facts."

const WritePrompt = `첨부 사진 관찰과 메모를 바탕으로 자연스러운 한국어 블로그 글을 작성하세요.
` + koreanGrounding + " " + koreanGroundingWriteScope + `
반드시 하나의 문단마다 TEXT 블록 하나만 사용하세요.
IMAGE 블록은 제공된 정확한 파일명만 사용하고, 목록에 없는 이미지를 절대 만들어내지 마세요.
IMAGE 블록은 사진이 글의 흐름상 가장 자연스러운 위치에 오도록 배치하세요.
출력은 설명이나 마크다운 없이 {"title":"...","summary":"...","tags":[],"blocks":[]} 형태의 JSON 객체 하나여야 합니다.
각 block은 type, content, level, file, alt, caption, items 필드를 사용하며 type은 TEXT, HEADING, IMAGE, QUOTE, LIST 중 하나입니다.`

const englishWritePrompt = `Write a natural English blog post from the photo observations and memo.
` + englishGrounding + " " + englishGroundingWriteScope + `
Use exactly one TEXT block for each paragraph.
IMAGE blocks may use only the exact filenames provided. Never invent an image that is not in the list.
Place each IMAGE block where the photo fits most naturally in the flow of the post.
Return exactly one JSON object shaped as {"title":"...","summary":"...","tags":[],"blocks":[]} with no explanation or Markdown.
Each block uses the type, content, level, file, alt, caption, and items fields. type must be one of TEXT, HEADING, IMAGE, QUOTE, or LIST.`

const NaturalnessBaseline = `[한국어 자연 문체 기준선]
- 아래 기준은 새로 쓰거나 수정 요청으로 손대는 TEXT 본문에만 적용하세요. 제목·요약·HEADING·LIST에는 적용하지 말고, 수정에서는 요청 밖의 기존 문장을 그대로 두세요.
- 대조 수사는 글 전체에서 “A가 아니라 B”, “~것이 아니라” 꼴을 합쳐 한 번만 쓰세요.
- 문단을 “필요한·중요한·핵심은 …이다”, “결국 …로 이어진다”, “~하는 이유다”로 닫지 마세요. “중요한 것은 실행력이다”보다 “오늘 할 일을 바로 적고 실행하세요”처럼 사실과 동작을 직접 쓰세요.
- 구체적 시점 없는 “향후·앞으로” 전망이나 내용 없는 “과제도 남아 있다”로 문단을 닫지 마세요. “~해야 한다”로 끝나는 문단은 글 전체에서 하나만 허용합니다.
- 연결어미 -고/-며/-지만/-면서/-아서 바로 뒤에는 쉼표를 놓지 말고, 대부분 문장은 쉼표 없이 쓰세요.
- 한 문단 안에서 짧은 문장과 긴 복문, 단문과 복문을 섞어 길이와 구조에 변화를 주세요.
- 확대·강화·개선·확보·구축 같은 포괄적 동사를 되풀이하지 말고 구체적인 동작을 쓰세요. 잠식·청사진·신호탄 같은 지어낸 비유를 겹치거나 과장 형용사를 쌓지 말고, “~적 명사”가 이어지지 않게 하세요.
- 메모가 요구하지 않은 수사·경구를 덧붙이지 마세요.
말투 프로필, 활성 대조 규칙, 사용자 규칙이 이 기준선과 충돌하면 해당 프로필과 규칙을 우선하세요.`

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
// A nil brief writes nothing at all, preserving the fixed no-purpose prompt byte for byte.
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

// guidelinePrecedence states the split the user needs guaranteed. A guideline is typically a
// prohibition the user added precisely because the default output was wrong, so it must beat
// the purpose's general content instruction — while register stays with the voice profile,
// consistent with purposePrecedence. Fixed prompt text, so it lives in code (ARCHITECTURE §4).
const guidelinePrecedence = "지침은 이 글에서 지켜야 할 주의 사항과 피해야 할 내용·표현을 정합니다. 지침이 용도의 요구와 충돌하면 지침을 우선하고, 문체·종결어미·어휘는 위의 말투 프로필을 따르세요."

// writeGuidelinesSection appends the frozen guideline texts as ONE section at ONE position:
// after the purpose section when the post has one, otherwise directly after the complete
// voice profile, and always before the per-post material. Both halves of that matter — the
// voice prefix stays byte-identical across posts (PRD §5's caching note), and prohibitions
// sit closest to the task material they constrain.
//
// The heading and the precedence sentence stay Korean for every target language, exactly as
// writePurposeSection does: the section frames user-authored text, and its framing is not
// part of the output-language contract.
//
// An empty slice writes nothing at all, preserving the fixed no-guideline prompt byte for byte.
func writeGuidelinesSection(out *strings.Builder, guidelines []string) {
	if len(guidelines) == 0 {
		return
	}
	out.WriteString("\n\n[작문 지침]")
	for _, text := range guidelines {
		fmt.Fprintf(out, "\n- %s", text)
	}
	fmt.Fprintf(out, "\n%s", guidelinePrecedence)
}

// BuildWritePrompt preserves the legacy Korean call surface for prompt goldens and
// consumers that explicitly request the established Korean contract. Runtime work uses
// BuildWritePromptForLanguage with its frozen language.
func BuildWritePrompt(profile Profile, observations []Observation, memo, title string, filenames []string, targetLength *int, purpose *PurposeBrief, guidelines []string) (string, string) {
	return BuildWritePromptForLanguage(LanguageKorean, profile, observations, memo, title, filenames, targetLength, purpose, guidelines)
}

func BuildWritePromptForLanguage(language Language, profile Profile, observations []Observation, memo, title string, filenames []string, targetLength *int, purpose *PurposeBrief, guidelines []string) (string, string) {
	var stable strings.Builder
	switch language {
	case LanguageKorean:
		stable.WriteString(WritePrompt)
		fmt.Fprintf(&stable, "\ntitle, 한 줄 summary, %d–%d개의 tags, blocks를 반환하세요.", TagsMin, TagsMax)
		stable.WriteString("\n출력 언어는 한국어입니다. title, summary, tags, 모든 본문, IMAGE alt와 caption을 한국어로 작성하세요. 말투 프로필, 용도, 메모, 가제의 언어 지시가 충돌해도 이 출력 언어를 우선하세요.")
	case LanguageEnglish:
		stable.WriteString(englishWritePrompt)
		fmt.Fprintf(&stable, "\nReturn title, a one-line summary, %d–%d tags, and blocks.", TagsMin, TagsMax)
		stable.WriteString("\nThe output language is English. Write the title, summary, tags, all prose, and every IMAGE alt and caption in English. This requirement overrides conflicting language instructions in the voice profile, purpose, memo, or title hint.")
	default:
		// Callers validate before prompt construction. Keeping this branch explicit makes
		// direct prompt use fail closed instead of silently defaulting to Korean.
		stable.WriteString("Unsupported output language; do not generate content.")
	}
	writeProfileSection(&stable, language, profile, targetLength)
	writePurposeSection(&stable, purpose)
	writeGuidelinesSection(&stable, guidelines)

	photoMaterial := "첨부 사진이 없습니다. 이미지 없이 메모만으로 작성하세요."
	if len(filenames) > 0 {
		photoMaterial = "첨부 파일명(정확히 일치해야 함): " + strings.Join(filenames, ", ") +
			"\n사진 관찰: " + marshalPromptJSON(observationsForPrompt(observations))
	}
	perPost := fmt.Sprintf("[이번 글]\n가제: %s\n메모: %s\n%s", title, memo, photoMaterial)
	return stable.String(), perPost
}

func writeProfileSection(stable *strings.Builder, language Language, profile Profile, targetLength *int) {
	if language == LanguageKorean {
		stable.WriteString("\n\n")
		stable.WriteString(NaturalnessBaseline)
	}
	if profile.Portable {
		stable.WriteString("\n\n[휴대 가능한 말투 프로필 / Portable voice profile]\n")
		stable.WriteString(profile.Styleguide)
		stable.WriteString("\n이 섹션에는 언어를 넘어 유지 가능한 구조와 수치 축만 포함됩니다. 출력 언어 지시를 우선하고 제외된 원문 표현을 추측하거나 번역해 보충하지 마세요.")
		writeGenericLength(stable, language, targetLength)
		return
	}

	stable.WriteString("\n\n[스타일가이드]\n")
	stable.WriteString(profile.Styleguide)
	stable.WriteString("\n\n[활성 대조 규칙]\n")
	stable.WriteString(profile.ActiveRules)
	stable.WriteString("\n\n[글 예시 발췌]")
	for i, excerpt := range profile.Excerpts {
		fmt.Fprintf(stable, "\n%d. %s", i+1, excerpt)
	}
	stable.WriteString("\n예시의 고유 사실, 주제, 문구를 복사하지 말고 문체 특징만 참고하세요.")
	stable.WriteString("\n\n[사용자 규칙]\n")
	stable.WriteString(profile.Rules)
	if language != LanguageKorean {
		writeGenericLength(stable, language, targetLength)
		return
	}
	endingMax := profile.EndingMaxConsecutive
	if endingMax <= 0 {
		endingMax = 2
	}
	stable.WriteString("\n\n[종결어미 제약]\n")
	if targetLength != nil {
		fmt.Fprintf(stable, "목표 길이: 약 %d자. ", *targetLength)
	}
	fmt.Fprintf(stable, "프로필의 측정된 종결어미 분포를 따르고 같은 종결어미를 %d문장보다 많이 연속 사용하지 마세요.", endingMax)
}

func writeGenericLength(stable *strings.Builder, language Language, targetLength *int) {
	if targetLength == nil {
		return
	}
	if language == LanguageEnglish {
		fmt.Fprintf(stable, "\n\n[Length]\nTarget approximately %d Unicode characters.", *targetLength)
		return
	}
	fmt.Fprintf(stable, "\n\n[길이]\n목표 길이: 약 %d자.", *targetLength)
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
