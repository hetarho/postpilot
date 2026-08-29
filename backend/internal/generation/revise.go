package generation

import (
	"encoding/json"
	"fmt"
	"strings"
)

const RevisePrompt = `현재 블로그 글에 사용자의 수정 요청만 최소한으로 반영하세요.
요청과 무관한 문장은 글자 그대로 유지하고, 손대지 않은 블록을 다듬거나 다시 쓰지 마세요.
제목, 한 줄 요약, 태그는 사용자가 그것들을 고쳐 달라고 한 경우에만 바꾸세요.
IMAGE 블록은 첨부된 정확한 파일명만 사용할 수 있습니다. 순서 변경이나 요청에 따른 제거는 가능하지만 파일명을 바꾸거나 새 이미지를 만들지 마세요.
출력은 diff가 아니라 완전한 PostContent이며, 설명이나 마크다운 없이 {"title":"...","summary":"...","tags":[],"blocks":[]} 형태의 JSON 객체 하나여야 합니다.
각 block은 type, content, level, file, alt, caption, items 필드를 사용하며 type은 TEXT, HEADING, IMAGE, QUOTE, LIST 중 하나입니다.`

type revisionPayloadJSON struct {
	Instruction string `json:"instruction"`
	SaveAsRule  bool   `json:"save_as_rule"`
}

func encodeRevisionPayload(instruction string, saveAsRule bool) ([]byte, error) {
	return json.Marshal(revisionPayloadJSON{Instruction: instruction, SaveAsRule: saveAsRule})
}

func parseRevisionPayload(payload []byte) (revisionPayloadJSON, error) {
	var value revisionPayloadJSON
	if err := json.Unmarshal(payload, &value); err != nil {
		return revisionPayloadJSON{}, fmt.Errorf("invalid revision payload: %w", err)
	}
	value.Instruction = strings.TrimSpace(value.Instruction)
	if value.Instruction == "" {
		return revisionPayloadJSON{}, ErrRevisionInstructionRequired
	}
	return value, nil
}

func BuildRevisePrompt(profile Profile, content PostContent, filenames []string, instruction string, targetLength ...int) (string, string) {
	var stable strings.Builder
	stable.WriteString(RevisePrompt)
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
	length := 1200
	if len(targetLength) > 0 && targetLength[0] > 0 {
		length = targetLength[0]
	}
	endingMax := profile.EndingMaxConsecutive
	if endingMax <= 0 {
		endingMax = 2
	}
	fmt.Fprintf(&stable, "\n\n[길이·종결어미 제약]\n목표 길이: 약 %d자. 프로필의 측정된 종결어미 분포를 따르고 같은 종결어미를 %d문장보다 많이 연속 사용하지 마세요.", length, endingMax)

	files := "없음"
	if len(filenames) > 0 {
		files = strings.Join(filenames, ", ")
	}
	user := fmt.Sprintf(
		"[현재 PostContent]\n%s\n\n[첨부 파일명]\n%s\n\n[수정 요청]\n%s",
		marshalPromptJSON(contentForPrompt(content)), files, instruction,
	)
	return stable.String(), user
}

func contentForPrompt(content PostContent) contentJSON {
	wire := contentJSON{Title: content.Title, Summary: content.Summary, Tags: content.Tags}
	for _, block := range content.Blocks {
		wire.Blocks = append(wire.Blocks, blockJSON{
			Type: string(block.Type), Content: block.Content, Level: block.Level,
			File: block.File, Alt: block.Alt, Caption: block.Caption, Items: block.Items,
		})
	}
	return wire
}
