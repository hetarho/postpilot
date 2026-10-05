package generation

import (
	"context"
	"errors"
	"testing"
)

// startRefusal is one link of the start chain, in the order every start checks them.
type startRefusal int

const (
	refusePublished startRefusal = iota
	refuseLanguage
	refuseVoice
	refusePendingComparison
	refuseWriteModel
	refuseObserveModel
	refuseNothing
)

func (r startRefusal) String() string {
	return [...]string{"published", "language", "deleted voice", "pending comparison", "missing write model", "missing observe model", "nothing"}[r]
}

// errPendingComparison stands for the ExperimentPendingError naming the comparison.
var errPendingComparison = errors.New("the pending comparison is named")

// preconditionFixture fails the link `from` and every link after it, so a start answers with
// the first one it checks: the table below pins each start's reasons and their order together.
func preconditionFixture(from startRefusal) (*fakePosts, Deps, string, string) {
	korean := LanguageKorean
	post := PostInput{
		Slug: "post", UserID: "alice", Voice: liveVoice, Title: "가제", Memo: "메모",
		Images:          []Image{{Filename: "IMG_1.jpg", Key: "k1"}},
		Content:         revisionContent("body"),
		Storyline:       &Storyline{Paragraphs: []StorylineParagraph{{Text: "가게를 보여줍니다.", Files: []string{"IMG_1.jpg"}}}},
		TargetLanguage:  LanguageKorean,
		ContentLanguage: &korean,
	}
	if from <= refusePublished {
		post.Published = true
	}
	if from <= refuseLanguage {
		post.TargetLanguage, post.ContentLanguage = "", nil
	}
	if from <= refuseVoice {
		post.Voice = deletedVoice
	}
	deps := testDeps()
	if from <= refusePendingComparison {
		deps.Experiments = fakePendingExperiments{id: "experiment-pending"}
	}
	write, observe := writeRef.String(), observeRef.String()
	if from <= refuseWriteModel {
		write = ""
	}
	if from <= refuseObserveModel {
		observe = ""
	}
	return &fakePosts{input: post, preserveMissingLanguages: true}, deps, write, observe
}

// GEN-23, GEN-25, GEN-38, GEN-68, GEN-69: every start runs one precondition chain and answers
// with the first link that fails. A revision checks its content language, the comparisons
// resolve their own write candidates and refuse a pending comparison themselves, a lab
// comparison reads a published post, and the starts that observe nothing take no observe model.
func TestEveryStartAnswersItsFirstFailingPrecondition(t *testing.T) {
	type start func(ctx context.Context, svc *Service, write, observe string) error
	starts := []struct {
		name  string
		start start
		want  map[startRefusal]error
	}{
		{"generation", func(ctx context.Context, svc *Service, write, observe string) error {
			_, err := svc.Start(ctx, StartRequest{UserID: "alice", PostSlug: "post", WriteModel: write, ObserveModel: observe})
			return err
		}, map[startRefusal]error{
			refusePublished: ErrPostPublished, refuseLanguage: ErrLanguageRequired, refuseVoice: ErrVoiceDeleted,
			refusePendingComparison: errPendingComparison, refuseWriteModel: ErrWriteModelRequired, refuseObserveModel: ErrObserveModelRequired,
		}},
		{"generation along the storyline", func(ctx context.Context, svc *Service, write, observe string) error {
			_, err := svc.Start(ctx, StartRequest{UserID: "alice", PostSlug: "post", WriteModel: write, ObserveModel: observe, FromStoryline: true})
			return err
		}, map[startRefusal]error{
			refusePublished: ErrPostPublished, refuseLanguage: ErrLanguageRequired, refuseVoice: ErrVoiceDeleted,
			refusePendingComparison: errPendingComparison, refuseWriteModel: ErrWriteModelRequired, refuseObserveModel: ErrObserveModelRequired,
		}},
		{"storyline", func(ctx context.Context, svc *Service, write, observe string) error {
			_, err := svc.StartStoryline(ctx, StartStorylineRequest{UserID: "alice", PostSlug: "post", WriteModel: write, ObserveModel: observe})
			return err
		}, map[startRefusal]error{
			refusePublished: ErrPostPublished, refuseLanguage: ErrLanguageRequired, refuseVoice: ErrVoiceDeleted,
			refusePendingComparison: errPendingComparison, refuseWriteModel: ErrWriteModelRequired, refuseObserveModel: ErrObserveModelRequired,
		}},
		{"storyline request", func(ctx context.Context, svc *Service, write, _ string) error {
			_, err := svc.StartStorylineRevision(ctx, StartStorylineRevisionRequest{UserID: "alice", PostSlug: "post", Request: "더 짧게", WriteModel: write})
			return err
		}, map[startRefusal]error{
			refusePublished: ErrPostPublished, refuseLanguage: ErrLanguageRequired, refuseVoice: ErrVoiceDeleted,
			refusePendingComparison: errPendingComparison, refuseWriteModel: ErrWriteModelRequired, refuseObserveModel: nil,
		}},
		{"revision", func(ctx context.Context, svc *Service, write, _ string) error {
			_, err := svc.StartRevision(ctx, StartRevisionRequest{UserID: "alice", PostSlug: "post", Instruction: "더 짧게", WriteModel: write})
			return err
		}, map[startRefusal]error{
			refusePublished: ErrPostPublished, refuseLanguage: ErrContentLanguageRequired, refuseVoice: ErrVoiceDeleted,
			refusePendingComparison: errPendingComparison, refuseWriteModel: ErrWriteModelRequired, refuseObserveModel: nil,
		}},
		{"editor comparison snapshot", func(ctx context.Context, svc *Service, _, observe string) error {
			ref, _ := parseModelRef(observe)
			_, err := svc.SnapshotWriteInput(ctx, "alice", "post", ref, nil, nil, false)
			return err
		}, map[startRefusal]error{
			refusePublished: ErrPostPublished, refuseLanguage: ErrLanguageRequired, refuseVoice: ErrVoiceDeleted,
			refusePendingComparison: ErrObserveModelRequired, refuseWriteModel: ErrObserveModelRequired, refuseObserveModel: ErrObserveModelRequired,
		}},
		{"lab comparison snapshot", func(ctx context.Context, svc *Service, _, observe string) error {
			ref, _ := parseModelRef(observe)
			_, err := svc.SnapshotWriteInput(ctx, "alice", "post", ref, nil, nil, true)
			return err
		}, map[startRefusal]error{
			refusePublished: ErrLanguageRequired, refuseLanguage: ErrLanguageRequired, refuseVoice: ErrVoiceDeleted,
			refusePendingComparison: ErrObserveModelRequired, refuseWriteModel: ErrObserveModelRequired, refuseObserveModel: ErrObserveModelRequired,
		}},
	}
	for _, test := range starts {
		for from := refusePublished; from <= refuseNothing; from++ {
			want := test.want[from]
			posts, deps, write, observe := preconditionFixture(from)
			models, jobs := newFakeModels(), &fakeJobs{id: "job"}
			svc := NewService(posts, fakeProfiles{}, models, fakeImages{}, jobs, 4, testReasoningPolicy, testBudget, deps)
			err := test.start(context.Background(), svc, write, observe)
			var pending *ExperimentPendingError
			switch {
			case want == errPendingComparison:
				if !errors.As(err, &pending) || pending.ExperimentID != "experiment-pending" {
					t.Errorf("%s from %s: err = %v, want the pending comparison named", test.name, from, err)
				}
			case !errors.Is(err, want) || (want == nil && err != nil):
				t.Errorf("%s from %s: err = %v, want %v", test.name, from, err, want)
			}
			if len(models.calls) != 0 {
				t.Errorf("%s from %s: a start called a provider %d times", test.name, from, len(models.calls))
			}
			if want != nil && jobs.enqueues != 0 {
				t.Errorf("%s from %s: a refused start queued %d jobs", test.name, from, jobs.enqueues)
			}
		}
	}
}

// The comparison snapshot still freezes its material before the voice check and loads the
// voice's projection right after it: a missing required template answer is what a post in a
// deleted voice hears from a comparison, as it always has.
func TestAComparisonSnapshotFreezesItsMaterialBeforeTheVoice(t *testing.T) {
	posts, deps, _, _ := preconditionFixture(refuseVoice)
	posts.input.TemplateID = "template"
	deps.Templates = refusingBriefs{}
	svc := NewService(posts, fakeProfiles{}, newFakeModels(), fakeImages{}, &fakeJobs{}, 4, testReasoningPolicy, testBudget, deps)
	_, err := svc.SnapshotWriteInput(context.Background(), "alice", "post", observeRef, nil, nil, false)
	var missing *RequiredTemplateAnswerError
	if !errors.As(err, &missing) || missing.Label != "가게 이름" {
		t.Fatalf("err = %v, want the required template answer", err)
	}
	// The ordinary start checks the voice first.
	_, err = svc.Start(context.Background(), StartRequest{UserID: "alice", PostSlug: "post", WriteModel: writeRef.String(), ObserveModel: observeRef.String()})
	if !errors.Is(err, ErrVoiceDeleted) {
		t.Fatalf("start err = %v, want the deleted voice", err)
	}
}

// refusingBriefs is a template whose required answer the post has not filled.
type refusingBriefs struct{ neutralBriefs }

func (refusingBriefs) RenderedForNewWrite(context.Context, string, string, bool, []TemplateAnswer) (TemplateBrief, bool, error) {
	return TemplateBrief{}, false, &RequiredTemplateAnswerError{Label: "가게 이름"}
}
