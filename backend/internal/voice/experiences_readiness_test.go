package voice

import (
	"fmt"
	"strings"
	"testing"
)

func threadAnswer(experience string, kind ExperienceKind, part PromptPart, n int) Sample {
	return Sample{
		ID: fmt.Sprintf("%s-%s-%d", experience, part, n), Kind: SampleKindAnswer,
		PromptKey:    fmt.Sprintf("thread_%s_%s_%d", kind, part, n),
		ExperienceID: experience,
		Body:         "여기는 정말 마음에 들었어요. 다음에도 또 오고 싶어요.",
	}
}

// experienceAnswers is one experience's five replies: 1 opening, 3 description, 1 closing.
func experienceAnswers(experience string, kind ExperienceKind) []Sample {
	return []Sample{
		threadAnswer(experience, kind, PartOpening, 1),
		threadAnswer(experience, kind, PartDescription, 1),
		threadAnswer(experience, kind, PartDescription, 2),
		threadAnswer(experience, kind, PartDescription, 3),
		threadAnswer(experience, kind, PartClosing, 1),
	}
}

func TestReadinessCountsEachExperienceSeparately(t *testing.T) {
	if ExperienceReplies != 5 || 2*ExperienceReplies != InitialQuestionCount {
		t.Fatalf("two experiences must make the first ten answers: %d", ExperienceReplies)
	}
	one := ReadinessOf(experienceAnswers("e1", ExperienceFood))
	if one.AnsweredQuestions != 5 || len(one.MissingParts) != 0 || one.Ready() {
		t.Fatalf("one experience = %+v", one)
	}
	// The same five questions answered about a second 맛집 count again.
	both := append(experienceAnswers("e1", ExperienceFood), experienceAnswers("e2", ExperienceFood)...)
	got := ReadinessOf(both)
	if got.AnsweredQuestions != 10 || len(got.MissingParts) != 0 || got.Percent != 100 || !got.Ready() {
		t.Fatalf("two experiences = %+v", got)
	}
	// Answering one experience's question again is a replacement, one row: it is not counted twice.
	if again := ReadinessOf(experienceAnswers("e1", ExperienceFood)); again.AnsweredQuestions != 5 {
		t.Fatalf("repeat = %+v", again)
	}
	// A situation answer with the same prompt key as no thread stays its own question.
	mixed := append(experienceAnswers("e1", ExperienceFood), Sample{ID: "s", Kind: SampleKindAnswer, PromptKey: "opening_greeting", Body: "안녕하세요! 오늘도 반가워요."})
	if got := ReadinessOf(mixed); got.AnsweredQuestions != 6 {
		t.Fatalf("mixed = %+v", got)
	}
}

func TestMaterialsOfAThreadAnswerIsTheBodyAlone(t *testing.T) {
	sample := Sample{
		ID: "a", Kind: SampleKindAnswer, PromptKey: "thread_food_description_1", ExperienceID: "e1",
		Body: "국물이 진해서 좋았어요.", Label: "성수 칼국수집 비밀 이름",
	}
	got := materialsOf([]Sample{sample})
	if len(got) != 1 || got[0].Text != sample.Body || got[0].Part != PartDescription {
		t.Fatalf("materials = %+v", got)
	}
	if strings.Contains(got[0].Text, "성수") || got[0].ID == "e1" {
		t.Fatalf("the experience must not leak into the materials: %+v", got[0])
	}
}
