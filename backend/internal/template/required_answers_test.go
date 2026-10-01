package template

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestNewWriteRequiresEnabledNonblankAnswersInTemplateOrder(t *testing.T) {
	svc, _ := newService(t)
	created, err := svc.Create(context.Background(), "alice", Authored{
		Name:      "방문기",
		TitleArea: `<ask label="방문 장소" required="true"/> 방문기`,
		Body: `<ask label="직접 겪은 일" required="true">경험을 근거로 쓰세요</ask>` + "\n" +
			`<ask label="최종 평가" required="true"/>`,
	})
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name    string
		answers []Answer
		missing string
	}{
		{"absent", nil, "방문 장소"},
		{"disabled", []Answer{{Label: "방문 장소", Text: "성수", Enabled: false}}, "방문 장소"},
		{"blank by shared rune set", []Answer{{Label: "방문 장소", Text: "\ufeff", Enabled: true}}, "방문 장소"},
		{"body follows title", []Answer{{Label: "방문 장소", Text: "성수", Enabled: true}}, "직접 겪은 일"},
		{"body follows source order", []Answer{
			{Label: "방문 장소", Text: "성수", Enabled: true},
			{Label: "직접 겪은 일", Text: "웨이팅이 길었다", Enabled: true},
		}, "최종 평가"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, ok, err := svc.RenderedForNewWrite(context.Background(), "alice", created.ID, false, tc.answers)
			var missing *RequiredAnswerError
			if ok || !errors.As(err, &missing) || missing.Label != tc.missing || !errors.Is(err, ErrRequiredAnswerMissing) {
				t.Fatalf("render = ok:%v err:%v, want missing %q", ok, err, tc.missing)
			}
		})
	}

	answers := []Answer{
		{Label: "방문 장소", Text: "성수", Enabled: true},
		{Label: "직접 겪은 일", Text: "웨이팅이 길었다", Enabled: true},
		{Label: "최종 평가", Text: "4.5점", Enabled: true},
	}
	rendered, ok, err := svc.RenderedForNewWrite(context.Background(), "alice", created.ID, false, answers)
	if err != nil || !ok || !strings.Contains(rendered.Body, "웨이팅이 길었다") || !strings.Contains(rendered.TitleArea, "성수") {
		t.Fatalf("complete render = %+v, ok:%v err:%v", rendered, ok, err)
	}
	// Existing content may still be revised after its template gains mandatory fields.
	if _, ok, err := svc.RenderedFor(context.Background(), "alice", created.ID, false, nil); err != nil || !ok {
		t.Fatalf("revision render = ok:%v err:%v", ok, err)
	}
	if _, ok, err := svc.RenderedForNewWrite(context.Background(), "alice", "removed", false, nil); err != nil || ok {
		t.Fatalf("deleted template = ok:%v err:%v", ok, err)
	}
}

func TestNewWriteKeepsOptionalAnswersOptional(t *testing.T) {
	svc, _ := newService(t)
	created, err := svc.Create(context.Background(), "alice", Authored{
		Name: "선택형",
		Body: `<ask label="방문일"/>` + "\n" + `<ask label="총평">한 줄 총평을 쓰세요</ask>`,
	})
	if err != nil {
		t.Fatal(err)
	}
	rendered, ok, err := svc.RenderedForNewWrite(context.Background(), "alice", created.ID, false, nil)
	if err != nil || !ok {
		t.Fatalf("optional render = ok:%v err:%v", ok, err)
	}
	if strings.Contains(rendered.Body, "방문일") || strings.Contains(rendered.Body, "총평") {
		t.Fatalf("unanswered optional fields remained: %q", rendered.Body)
	}
}
