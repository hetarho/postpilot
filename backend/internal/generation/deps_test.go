package generation

import (
	"context"
	"errors"
	"time"
)

// testDeps is the neutral set of cross-context collaborators: each answers the way the
// context behaved before the collaborator existed (no pending experiment, no brief, no
// 지침, nothing recorded, no video link), so a test that cares about one replaces it.
func testDeps() Deps {
	return Deps{
		Experiments: neutralExperiments{},
		Templates:   neutralBriefs{},
		Guidelines:  neutralGuidelines{},
		Memories:    neutralMemories{},
		Candidates:  neutralCandidates{},
		Samples:     neutralSamples{},
		Videos:      neutralLinker{},
		VideoURLTTL: time.Minute,
	}
}

type neutralExperiments struct{}

func (neutralExperiments) PendingForPost(context.Context, string, string) (string, error) {
	return "", nil
}

type neutralBriefs struct{}

func (neutralBriefs) RenderedFor(context.Context, string, string, []string, []TemplateAnswer) (TemplateBrief, bool, error) {
	return TemplateBrief{}, false, nil
}

type neutralGuidelines struct{}

func (neutralGuidelines) ForPrompt(context.Context, string, *string) ([]string, error) {
	return nil, nil
}

// neutralMemories answers the way the context behaved before memories existed: none, for
// every post. A post with the option off never reaches it in the first place.
type neutralMemories struct{}

func (neutralMemories) ForPost(context.Context, string, []string) ([]string, error) {
	return nil, nil
}

type neutralCandidates struct{}

func (neutralCandidates) Record(context.Context, string, string, string) error { return nil }

type neutralSamples struct{}

func (neutralSamples) RecordVersionSample(context.Context, string, string, PostContent) error {
	return nil
}

// neutralLinker refuses every link, which is what a run with a video met before a
// linker was wired: there is no other way to deliver a clip.
type neutralLinker struct{}

func (neutralLinker) PresignGet(context.Context, string, time.Duration) (string, error) {
	return "", errors.New("no video linker in this test")
}
