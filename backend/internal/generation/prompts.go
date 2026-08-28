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

func BuildWritePrompt(profile Profile, observations []Observation, memo, title string, filenames []string) (string, string) {
	var stable strings.Builder
	stable.WriteString(WritePrompt)
	fmt.Fprintf(&stable, "\ntitle, 한 줄 summary, %d–%d개의 tags, blocks를 반환하세요.", TagsMin, TagsMax)
	stable.WriteString("\n\n[스타일가이드]\n")
	stable.WriteString(profile.Styleguide)
	stable.WriteString("\n\n[글 예시 발췌]")
	for i, excerpt := range profile.Excerpts {
		fmt.Fprintf(&stable, "\n%d. %s", i+1, excerpt)
	}
	stable.WriteString("\n\n[사용자 규칙]\n")
	stable.WriteString(profile.Rules)

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
