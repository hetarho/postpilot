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

// The drafting model is taught the title area's rules the parser enforces: a body field
// repeated in the title is refused, so the guide must say so and the shape it teaches instead
// (the fact asked once in the body, its title place a write) must validate.
func TestAuthoringGuideTeachesTheTitleAreaRulesTheParserEnforces(t *testing.T) {
	if guide := AuthoringGuide(testLimits()); !strings.Contains(guide, AuthoringTitleAreaRule) || !strings.Contains(AuthoringTitleAreaRule, "<write>제품명</write>") {
		t.Fatalf("authoring guide lacks the title area rule: %q", guide)
	}
	authoring := NewAuthoring(NewService(newFakeStore(), testLimits()), unusedAuthoringStore{})
	field := `<ask label="제품명과 구매처" required="true">제품명과 구매처를 적어 주세요.</ask>`
	taught := Draft{Name: "언박싱", TitleArea: `[언박싱] <write>제품명</write> 솔직 후기`, Body: field + "\n<write>개봉한 모습</write>"}
	if _, err := authoring.Validate(taught); err != nil {
		t.Fatalf("the taught title shape is refused: %v", err)
	}
	repeated := taught
	repeated.TitleArea = `[언박싱] <ask label="제품명과 구매처"/> 솔직 후기`
	_, err := authoring.Validate(repeated)
	var parseErr *ParseError
	if !errors.As(err, &parseErr) || parseErr.Reason != ReasonDuplicateAskLabel {
		t.Fatalf("a body field repeated in the title = %v", err)
	}
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
