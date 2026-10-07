package template

import (
	"errors"
	"strconv"
	"strings"
)

// Language is the UI locale the 형식 안내 is written in. It is the reader's language, not a
// post's target: the guide frames an outside AI's task for the person who copies it (TMPL-41).
type Language string

const (
	LanguageKorean  Language = "ko"
	LanguageEnglish Language = "en"
)

// ErrUnsupportedLanguage is a guide asked for in a language the product does not speak (LANG-1).
var ErrUnsupportedLanguage = errors.New("template: unsupported guide language")

// GuideExampleBody is the worked example the guide carries, and the one place it is written
// down. It is a BODY, not prose: the guide test parses it with the real parser, so an example
// that drifted from the grammar fails the build rather than teaching an outside AI to write
// something this app refuses (TMPL-41). It uses every construct a person authors today,
// including a suggested photo group and a data field, and none of the retired ones; each `<write>` names
// what stands at its place, never how to write it (TMPL-57).
const GuideExampleBody = "<write>방문한 이유와 첫인상</write>\n" +
	"<repeat each=\"photo\">\n" +
	"<slot kind=\"photo\" count=\"2\"/>\n" +
	"<write>이 사진들이 보여주는 장면</write>\n" +
	"</repeat>\n" +
	"<ask label=\"총평 별점\" required=\"true\">직접 매긴 별점과 그 이유는 무엇인가요?</ask>\n" +
	"오늘도 좋은 하루 보내세요"

// The guide's prose, one per UI language. Tags, attributes and the example are identical in
// both by construction: only the surrounding sentences differ, and every number and the example
// arrive through the placeholders FormatGuide fills. It is prompt text and code (LANG-27), read
// by the copy button and taught to the 글 작성 모델 by a template request alike, so the two can
// never teach different grammars.
const guideKorean = `아래 형식으로 블로그 글 템플릿의 본문을 하나 작성해 주세요.

[템플릿이 하는 일]
템플릿은 글의 뼈대입니다. 글의 순서와 어디에 무엇이 들어갈지를 정하고, 문체나 어휘는 정하지 않습니다.

[쓸 수 있는 표기 다섯 가지]
- 그냥 쓴 문장: 글에 그대로 나옵니다.
- <write>메뉴 소개</write>: AI가 그 자리에 적힌 주제로 글을 씁니다. 말투·길이·강조 같은 쓰는 방식은 여기에 적지 말고 지침으로 정하세요. 태그 안의 글은 글에 나오지 않습니다.
- <slot kind="photo"/>: 사진이 들어갈 자리입니다. <slot kind="photo" count="2"/>처럼 count를 줄 수 있습니다. count는 이 자리에 콜라주나 슬라이드 하나로 함께 보여 주기를 제안하는 사진 수이고, 실제로 몇 장을 어떻게 묶을지와 어떤 사진을 놓을지는 AI가 스토리라인을 따라 정합니다.
- <repeat each="photo">…</repeat>: 안쪽 내용이 스토리라인의 사진 묶음마다 한 번씩 되풀이됩니다.
- <ask label="입력란 제목"/>: 글을 쓸 때 사용자가 직접 입력한 내용이 그 자리에 그대로 들어갑니다.
- <ask label="총평 별점">직접 매긴 별점과 그 이유는 무엇인가요?</ask>: 작성자가 질문에 답하면 AI가 그 답을 근거로 이 자리의 총평을 씁니다.
- <ask label="총평 별점" required="true">직접 매긴 별점과 그 이유는 무엇인가요?</ask>: 해당 입력란을 채워야 새 글을 쓸 수 있습니다. required가 없으면 선택 입력입니다.

[지켜야 할 규칙]
- write, repeat는 반드시 닫아야 합니다.
- slot은 <slot …/>처럼 스스로 닫고, kind는 photo만 쓸 수 있습니다.
- count는 1에서 {photoRowMax} 사이의 정수입니다. 없으면 1장입니다.
- repeat는 each="photo"만 받고, repeat 안에 repeat를 넣을 수 없습니다.
- write는 비워 둘 수 없습니다.
- write, ask 안에는 글만 씁니다. 그 안에 다른 태그를 넣으면 저장되지 않습니다.
- 각 태그에는 위에 나온 속성만 쓰고, 한 태그에 같은 속성을 두 번 쓰지 않습니다.
- ask는 label이 반드시 있어야 하고, label은 {askLabelMax}자까지이며, 한 본문 안에서 label이 겹치면 안 됩니다. repeat 안에는 넣을 수 없고, 한 본문에 최대 {askMax}개까지입니다.
- ask의 required는 true만 쓸 수 있습니다. 비워 두거나 끌 수 없는 입력란에만 붙이세요.
- write에는 그 자리에 들어갈 주제만 적습니다. 내용 있는 ask에는 작성자의 실제 경험·확인한 사실·불확실한 점을 묻는 구체적인 질문을 적어도 됩니다. 어느 쪽에도 모델의 말투·길이·서식·생략·반복을 지시하지 말고, 그런 작문 규칙은 지침에 적으세요.
- ask는 사용자가 직접 겪거나 확인해야 아는 내용(별점, 방문일, 가격 등)을 AI가 지어내지 않도록 물을 때 쓰세요.
- 위 다섯 가지 말고 다른 태그를 쓰면 저장되지 않습니다.
- 문장 안에 <로 시작하는 글자를 그대로 쓰려면 &lt;로 적어 주세요.
- 본문 전체는 {bodyMax}자를 넘을 수 없습니다.

[예시]
{example}

[답변 방식]
설명이나 코드 블록 없이 본문만 보내 주세요. 글은 제가 쓰는 언어로 써 주세요.`

const guideEnglish = `Write the body of one blog post template in the format below.

[What a template does]
A template is the skeleton of a post. It decides the order and what goes where; it never decides tone or word choice.

[The five things you can write]
- Plain text: appears in the post exactly as written.
- <write>the menu</write>: the AI writes here about the topic named. How to write it — tone, length, emphasis — belongs to guidelines, not here. The text inside the tag never appears in the post.
- <slot kind="photo"/>: photos go here. It can take a count, as in <slot kind="photo" count="2"/>: count is how many photos you suggest showing together there as one collage or slide; the AI decides the actual group and which photos stand there along the storyline.
- <repeat each="photo">…</repeat>: what is inside repeats once per photo group of the storyline.
- <ask label="field title"/>: what the author types on the write screen goes here exactly as typed.
- <ask label="overall rating">What rating did you give, and why?</ask>: the author answers the question, and the AI writes this section from that answer.
- <ask label="overall rating" required="true">What rating did you give, and why?</ask>: the author must fill this field before starting a new post. Without required, the field is optional.

[Rules that must hold]
- write and repeat must be closed.
- slot closes itself, as <slot …/>, and kind may only be photo.
- count is a whole number from 1 to {photoRowMax}. Without it, one photo.
- repeat takes only each="photo", and a repeat may not contain a repeat.
- write may never be empty.
- write and ask hold text only. A tag inside one of them is refused.
- Each tag takes only the attributes shown above, and no attribute twice.
- ask must carry a label of at most {askLabelMax} characters, no two may share one in the same body, none may sit inside a repeat, and one body holds at most {askMax}.
- An ask's required attribute may only have the value true. Use it only for a field the author must fill.
- A write names only what belongs at its position. A text-bearing ask may ask the author a concrete question about firsthand experience, verified facts, or uncertainty. Neither may command the model's tone, length, formatting, omissions, or repetition; those writing rules belong in guidelines.
- Use ask for what only the author experienced or checked, such as a rating, visit date, or price, so the AI does not invent it.
- Any tag other than those five is refused.
- To write a literal < in a sentence, write &lt; instead.
- The whole body may not exceed {bodyMax} characters.

[Example]
{example}

[How to answer]
Send the body only — no explanation and no code fence. Write it in the language I am writing in.`

// FormatGuide is the self-contained instruction a user hands to any outside AI so it writes a
// body in this app's grammar (TMPL-41), with the configured ceilings stated as the numbers they
// are.
func FormatGuide(language Language, limits Limits) (string, error) {
	var prose string
	switch language {
	case LanguageKorean:
		prose = guideKorean
	case LanguageEnglish:
		prose = guideEnglish
	default:
		return "", ErrUnsupportedLanguage
	}
	return strings.NewReplacer(
		"{photoRowMax}", strconv.Itoa(limits.PhotoRowMax),
		"{askLabelMax}", strconv.Itoa(limits.AskLabelMaxChars),
		"{askMax}", strconv.Itoa(limits.AskMaxPerBody),
		"{bodyMax}", strconv.Itoa(limits.BodyMaxChars),
		"{example}", GuideExampleBody,
	).Replace(prose), nil
}

// GrammarGuide shares the public grammar, excluding the external task and
// body-only response instruction. Its consumer owns the response contract.
func GrammarGuide(language Language, limits Limits) (string, error) {
	guide, err := FormatGuide(language, limits)
	if err != nil {
		return "", err
	}
	_, guide, _ = strings.Cut(guide, "\n\n")
	for _, heading := range []string{"\n\n[답변 방식]", "\n\n[How to answer]"} {
		guide, _, _ = strings.Cut(guide, heading)
	}
	return guide, nil
}

// FormatGuide is the guide under this process's own limits: the numbers it states are the
// ones a save is held to.
func (s *Service) FormatGuide(language Language) (string, error) {
	return FormatGuide(language, s.limits)
}
