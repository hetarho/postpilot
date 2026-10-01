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
// including a photo row and a data field, and none of the retired ones; each `<write>` names
// what stands at its place, never how to write it (TMPL-57).
const GuideExampleBody = "<write>방문한 이유와 첫인상</write>\n" +
	"<repeat each=\"photo\">\n" +
	"<slot kind=\"photo\" count=\"2\"/>\n" +
	"<write>이 사진들이 보여주는 장면</write>\n" +
	"</repeat>\n" +
	"<ask label=\"총평 별점\">총평</ask>\n" +
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
- <slot kind="photo"/>: 사진이 들어갈 자리입니다. <slot kind="photo" count="2"/>처럼 count를 줄 수 있습니다. count는 한 줄에 나란히 놓을 사진 수이고, 어떤 사진을 놓을지는 AI가 스토리라인을 따라 고릅니다.
- <repeat each="photo">…</repeat>: 안쪽 내용이 스토리라인의 사진 묶음마다 한 번씩 되풀이됩니다.
- <ask label="입력란 제목"/>: 글을 쓸 때 사용자가 직접 입력한 내용이 그 자리에 그대로 들어갑니다.
- <ask label="입력란 제목">총평</ask>: 사용자가 입력한 내용으로 AI가 그 자리의 주제를 씁니다.

[지켜야 할 규칙]
- write, repeat는 반드시 닫아야 합니다.
- slot은 <slot …/>처럼 스스로 닫고, kind는 photo만 쓸 수 있습니다.
- count는 1에서 {photoRowMax} 사이의 정수입니다. 없으면 1장입니다.
- repeat는 each="photo"만 받고, repeat 안에 repeat를 넣을 수 없습니다.
- write는 비워 둘 수 없습니다.
- write, ask 안에는 글만 씁니다. 그 안에 다른 태그를 넣으면 저장되지 않습니다.
- 각 태그에는 위에 나온 속성만 쓰고, 한 태그에 같은 속성을 두 번 쓰지 않습니다.
- ask는 label이 반드시 있어야 하고, label은 {askLabelMax}자까지이며, 한 본문 안에서 label이 겹치면 안 됩니다. repeat 안에는 넣을 수 없고, 한 본문에 최대 {askMax}개까지입니다.
- ask는 사용자가 매번 알려줘야 하는 것(별점, 방문일, 가격처럼 AI가 알 수 없는 사실)에만 쓰세요.
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
- <slot kind="photo"/>: photos go here. It can take a count, as in <slot kind="photo" count="2"/>: count is how many photos stand side by side in one row, and the AI chooses which photos stand there along the storyline.
- <repeat each="photo">…</repeat>: what is inside repeats once per photo group of the storyline.
- <ask label="field title"/>: what the author types on the write screen goes here exactly as typed.
- <ask label="field title">overall verdict</ask>: the AI writes the topic here from what the author typed.

[Rules that must hold]
- write and repeat must be closed.
- slot closes itself, as <slot …/>, and kind may only be photo.
- count is a whole number from 1 to {photoRowMax}. Without it, one photo.
- repeat takes only each="photo", and a repeat may not contain a repeat.
- write may never be empty.
- write and ask hold text only. A tag inside one of them is refused.
- Each tag takes only the attributes shown above, and no attribute twice.
- ask must carry a label of at most {askLabelMax} characters, no two may share one in the same body, none may sit inside a repeat, and one body holds at most {askMax}.
- Use ask only for what the author has to supply each time — a rating, a visit date, a price: facts the AI cannot know.
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

// FormatGuide is the guide under this process's own limits: the numbers it states are the
// ones a save is held to.
func (s *Service) FormatGuide(language Language) (string, error) {
	return FormatGuide(language, s.limits)
}
