package template

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// No persistence or provider operation is part of private candidate validation.
type unusedAuthoringStore struct{}

func (unusedAuthoringStore) CountAuthoringTargets(context.Context, string) (int, error) {
	panic("unexpected persistence")
}
func (unusedAuthoringStore) PublishAuthoring(context.Context, string, AuthoringPublication, string, time.Time, int) (Template, error) {
	panic("unexpected publication")
}

func TestUnpublishedAuthoringCandidateUsesTheRequiredAnswerGateBeforeWriting(t *testing.T) {
	authoring := NewAuthoring(NewService(newFakeStore(), testLimits()), unusedAuthoringStore{})
	draft := Draft{Name: "Private template", TitleArea: `<ask label="제목" required="true">제목</ask>`, Body: `<write>본문</write>`}
	for _, answers := range [][]Answer{nil, {{Label: "제목", Text: "Owner's title", Enabled: false}}, {{Label: "제목", Text: " ", Enabled: true}}} {
		_, err := authoring.RenderedForNewWrite(draft, false, answers)
		var missing *RequiredAnswerError
		if !errors.As(err, &missing) || missing.Label != "제목" {
			t.Fatalf("missing answers=%+v err=%v", answers, err)
		}
	}
	rendered, err := authoring.RenderedForNewWrite(draft, false, []Answer{{Label: "제목", Text: "Owner's title", Enabled: true}})
	if err != nil || rendered.Name != draft.Name || !strings.Contains(rendered.TitleArea, "Owner's title") || len(rendered.Facts) != 1 {
		t.Fatalf("valid private candidate=%+v err=%v", rendered, err)
	}
	draft.Body = `<unknown/>`
	if _, err := authoring.RenderedForNewWrite(draft, false, nil); err == nil {
		t.Fatal("invalid contender shape accepted")
	}
}
