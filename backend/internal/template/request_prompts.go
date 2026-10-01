package template

import (
	_ "embed"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

//go:embed schemas/request_answer.schema.json
var requestAnswerSchema []byte

// RequestAnswerSchema is the response contract of a template request, attached only when the
// resolved model declares structured output; the rules state the same shape for every other
// model. Every field is a string, as every response schema here is.
func RequestAnswerSchema() []byte { return append([]byte(nil), requestAnswerSchema...) }

// The request rules are what a template request adds to the 형식 안내 (TMPL-59): the guide
// teaches the body's grammar to anyone, these say what THIS answer must be. They live beside the
// guide and change with it, and like it they are code (LANG-27). Every number arrives through a
// placeholder, from the process's own limits.
const requestRulesKorean = `[이번 요청에서 지켜야 할 것]
위 형식 안내는 템플릿 본문의 문법입니다. 이번에는 본문만이 아니라 템플릿 전체를 아래 JSON 객체 하나로 답해 주세요.
{"name": "...", "description": "...", "title_area": "...", "body": "...", "wishes": ["..."]}

- 답변은 이 JSON 객체 하나뿐입니다. 설명이나 코드 블록을 붙이지 마세요.
- name: 템플릿 이름입니다. 비워 둘 수 없고 {nameMax}자까지입니다.
- description: 이 템플릿이 어떤 글에 쓰이는지 한두 문장으로 적습니다. {descriptionMax}자까지이고 비워 둘 수 있습니다.
- title_area: 글 제목의 형식입니다. 그냥 쓴 문장, <write>, <ask>만 쓸 수 있고 사진 자리와 repeat는 쓸 수 없습니다. {titleAreaMax}자까지이고, 제목 형식이 필요 없으면 비워 둡니다. ask의 label은 본문의 label과 겹치면 안 됩니다.
- body: 위 형식 안내를 따르는 본문입니다.
- [현재 초안]에 이미 적힌 항목은 요청이 바꾸라고 한 것만 바꾸고 나머지는 그대로 둡니다. 비어 있는 항목은 채웁니다.
- [참고 글]의 문장은 고정 문구로 옮기지 않습니다. 글의 순서와 사진 위치만 템플릿의 자리로 바꾸고, 각 문장은 <write>가 다룰 주제로 바꿉니다. [사진] 줄은 사진이 놓였던 자리입니다. 요청이 어떤 문장을 그대로 두라고 할 때만 그 문장을 고정 문구로 씁니다.
- <write>와 내용이 있는 <ask>에는 그 자리에 무엇이 오는지만 적고, 어떻게 쓸지는 적지 않습니다.
- 말투, 길이, 강조, 빼고 싶은 내용처럼 글을 어떻게 쓸지에 대한 바람은 템플릿에 넣지 말고 wishes에 짧은 문장으로 옮깁니다. 그런 바람이 없으면 wishes는 빈 배열입니다.
- 목표 글자 수나 태그 수는 정하지 않습니다.`

const requestRulesEnglish = `[What this request must follow]
The format guide above is the grammar of a template body. This time, answer with the whole template as the one JSON object below, not the body alone.
{"name": "...", "description": "...", "title_area": "...", "body": "...", "wishes": ["..."]}

- The answer is that JSON object and nothing else. Add no explanation and no code fence.
- name: the template's name. It may not be empty and holds at most {nameMax} characters.
- description: one or two sentences on what kind of post this template is for. At most {descriptionMax} characters; it may be empty.
- title_area: the form of the post's title. It takes plain text, <write> and <ask> only — no photo position and no repeat. At most {titleAreaMax} characters; leave it empty when no title form is needed. An ask label here may not repeat one in the body.
- body: a body that follows the format guide above.
- In [Current draft], change only what the request asks to change and keep everything else as written. Fill the fields that are empty.
- Do not carry sentences of [Sample post] over as fixed text. Turn its order and photo positions into the template's places, and turn each sentence into the topic a <write> covers. A [사진] line marks where photos stood. Write a sentence as fixed text only when the request asks to keep it.
- A <write>, and an <ask> holding text, names what stands at its place, never how to write it.
- Wishes about how to write — tone, length, emphasis, what to leave out — never go into the template: move them to wishes as short sentences. With no such wish, wishes is an empty array.
- Never set a target length or a tag count.`

// requestSystem is the request's system prompt: the 형식 안내 the copy button gives, byte for
// byte, then the request rules.
func (s *Service) requestSystem(language Language) (string, error) {
	guide, err := FormatGuide(language, s.limits)
	if err != nil {
		return "", err
	}
	rules := requestRulesKorean
	if language == LanguageEnglish {
		rules = requestRulesEnglish
	}
	rules = strings.NewReplacer(
		"{nameMax}", strconv.Itoa(s.limits.NameMaxChars),
		"{descriptionMax}", strconv.Itoa(s.limits.DescriptionMaxChars),
		"{titleAreaMax}", strconv.Itoa(s.limits.TitleAreaMaxChars),
	).Replace(rules)
	return guide + "\n\n" + rules, nil
}

type requestWords struct {
	request, draft, sample, none, empty             string
	name, description, titleArea, body, sampleTitle string
}

var requestCopy = map[Language]requestWords{
	LanguageKorean: {
		request: "[요청]", draft: "[현재 초안]", sample: "[참고 글]", none: "(없음)", empty: "(비어 있음)",
		name: "이름", description: "설명", titleArea: "제목 형식", body: "본문", sampleTitle: "제목",
	},
	LanguageEnglish: {
		request: "[Request]", draft: "[Current draft]", sample: "[Sample post]", none: "(none)", empty: "(empty)",
		name: "Name", description: "Description", titleArea: "Title form", body: "Body", sampleTitle: "Title",
	},
}

// requestMessage is the user turn: the request, every field of the draft as the editor holds
// it, and the sample when one is attached. Nothing else reaches the model — no voice, guideline,
// memo, photo or other template (TMPL-59).
func requestMessage(input requestInput) string {
	words := requestCopy[input.Language]
	field := func(value string) string {
		if strings.TrimSpace(value) == "" {
			return words.empty
		}
		return value
	}
	var out strings.Builder
	out.WriteString(words.request + "\n")
	if input.Text == "" {
		out.WriteString(words.none)
	} else {
		out.WriteString(input.Text)
	}
	out.WriteString("\n\n" + words.draft + "\n")
	out.WriteString(words.name + ": " + field(input.Draft.Name) + "\n")
	out.WriteString(words.description + ": " + field(input.Draft.Description) + "\n")
	out.WriteString(words.titleArea + ":\n" + field(input.Draft.TitleArea) + "\n")
	out.WriteString(words.body + ":\n" + field(input.Draft.Body))
	if input.Sample != nil {
		out.WriteString("\n\n" + words.sample + "\n")
		out.WriteString(words.sampleTitle + ": " + field(input.Sample.Title) + "\n")
		out.WriteString(input.Sample.Text)
	}
	return out.String()
}

// correctionMessage tells the model what its last answer broke, in the terms the rule check
// reports it: the area, line and reason of a parse failure, or the field and its ceiling.
func correctionMessage(language Language, err error) string {
	korean := language != LanguageEnglish
	var problem string
	var parseErr *ParseError
	var tooLong *FieldTooLongError
	switch {
	case errors.As(err, &parseErr):
		if korean {
			problem = fmt.Sprintf("%s %d번째 줄을 읽을 수 없어요 (%s).", parseErr.Area, parseErr.Line, parseErr.Reason)
		} else {
			problem = fmt.Sprintf("Line %d of %s does not parse (%s).", parseErr.Line, parseErr.Area, parseErr.Reason)
		}
	case errors.As(err, &tooLong):
		if korean {
			problem = fmt.Sprintf("%s가 %d자로, %d자까지만 쓸 수 있어요.", tooLong.Field, tooLong.Chars, tooLong.Max)
		} else {
			problem = fmt.Sprintf("%s has %d characters; at most %d are allowed.", tooLong.Field, tooLong.Chars, tooLong.Max)
		}
	case errors.Is(err, ErrNameRequired):
		problem = map[bool]string{true: "name이 비어 있어요.", false: "name is empty."}[korean]
	case errors.Is(err, ErrBodyRequired):
		problem = map[bool]string{true: "body가 비어 있어요.", false: "body is empty."}[korean]
	default:
		problem = map[bool]string{true: "답변이 요청한 JSON 객체가 아니에요.", false: "The answer is not the requested JSON object."}[korean]
	}
	if korean {
		return problem + " 이 문제만 고쳐서 JSON 객체 전체를 다시 보내 주세요."
	}
	return problem + " Fix only this and send the whole JSON object again."
}
