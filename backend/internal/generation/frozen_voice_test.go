package generation

import (
	"context"
	"errors"
	"github.com/postpilot/backend/internal/llm"
	"strings"
	"testing"
)

type changingAcceptedProfile struct {
	current   Profile
	withdrawn bool
	reads     int
	proofs    [][]ProfileSource
}

func (p *changingAcceptedProfile) ProfileForPrompt(context.Context, string, string, Language) (Profile, error) {
	p.reads++
	return p.current, nil
}
func (p *changingAcceptedProfile) ValidateProfileSources(_ context.Context, user, voiceID string, sources []ProfileSource) error {
	if user != "alice" || voiceID != liveVoice.ID {
		return errors.New("foreign voice")
	}
	p.proofs = append(p.proofs, append([]ProfileSource(nil), sources...))
	if p.withdrawn {
		return errors.New("accepted source withdrawn")
	}
	if len(sources) != 1 || sources[0] != (ProfileSource{SampleID: "original", ContentRevision: 1}) {
		return errors.New("original source identity lost")
	}
	return nil
}
func acceptedProfileHarness() (*Service, *fakePosts, *fakeJobs, *fakeModels, *changingAcceptedProfile) {
	profiles := &changingAcceptedProfile{current: Profile{Text: "ACCEPTED OLD HABITS", Excerpts: []string{"ACCEPTED OLD EXCERPT"}, Sources: []ProfileSource{{SampleID: "original", ContentRevision: 1}}}}
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: liveVoice, TargetLanguage: LanguageKorean, Content: revisionContent("Original body")}}
	jobs := &fakeJobs{id: "job"}
	models := newFakeModels()
	models.complete = func(_ llm.ModelRef, req llm.Request) (llm.Response, error) {
		if !strings.Contains(req.System, "ACCEPTED OLD HABITS") || !strings.Contains(req.System, "ACCEPTED OLD EXCERPT") || strings.Contains(req.System, "NEW MATERIAL") {
			return llm.Response{}, errors.New("admitted profile replaced by live prose")
		}
		return llm.Response{Text: `{"title":"글","summary":"요약","tags":["하나","둘","셋"],"blocks":[{"type":"TEXT","content":"본문"}]}`}, nil
	}
	return NewService(posts, profiles, models, fakeImages{}, jobs, 4, testReasoningPolicy, testBudget, testDeps()), posts, jobs, models, profiles
}
func TestNewWritingAndRevisionFreezeAcceptedProfileUntilExplicitNewAdmission(t *testing.T) {
	for _, kind := range []string{"writing", "revision"} {
		t.Run(kind, func(t *testing.T) {
			svc, _, jobs, models, profiles := acceptedProfileHarness()
			ctx := context.Background()
			if kind == "writing" {
				if _, err := svc.Start(ctx, StartRequest{UserID: "alice", PostSlug: "post", WriteModel: writeRef.String()}); err != nil {
					t.Fatal(err)
				}
			} else if _, err := svc.StartRevision(ctx, StartRevisionRequest{UserID: "alice", PostSlug: "post", Instruction: "더 간결하게", WriteModel: writeRef.String()}); err != nil {
				t.Fatal(err)
			}
			if len(models.calls) != 0 {
				t.Fatal("admission called provider")
			}
			profiles.current = Profile{Text: "NEW MATERIAL", Excerpts: []string{"NEW MATERIAL"}, Sources: []ProfileSource{{SampleID: "original", ContentRevision: 9}}}
			var err error
			if kind == "writing" {
				err = svc.Generate(ctx, jobs.queued(0), func(string, int, int) {})
			} else {
				err = svc.Revise(ctx, RevisionJob{UserID: "alice", PostSlug: "post", VoiceID: liveVoice.ID, WriteModel: writeRef.String(), Payload: jobs.payloads[0]}, func(string, int, int) {})
			}
			if err != nil {
				t.Fatal(err)
			}
			if profiles.reads != 1 || len(models.calls) != 1 || len(profiles.proofs) == 0 {
				t.Fatalf("reads=%d calls=%d proofs=%v", profiles.reads, len(models.calls), profiles.proofs)
			}
		})
	}
}
func TestWithdrawnAcceptedMaterialRefusesQueuedWorkBeforeAnyProviderCall(t *testing.T) {
	svc, posts, jobs, models, profiles := acceptedProfileHarness()
	if _, err := svc.Start(context.Background(), StartRequest{UserID: "alice", PostSlug: "post", WriteModel: writeRef.String()}); err != nil {
		t.Fatal(err)
	}
	profiles.withdrawn = true
	err := svc.Generate(context.Background(), jobs.queued(0), func(string, int, int) {})
	if err == nil || len(models.calls) != 0 || posts.input.Content.Blocks[0].Content != "Original body" {
		t.Fatalf("err=%v calls=%d content=%v", err, len(models.calls), posts.input.Content)
	}
}
func TestWithdrawalDuringObservationStopsTheRemainingWriterWithoutReplacingItsSnapshot(t *testing.T) {
	svc, posts, jobs, models, profiles := acceptedProfileHarness()
	posts.input.Images = []Image{{Filename: "one.jpg", Key: "private", Kind: AttachmentPhoto, ContentType: "image/jpeg"}}
	models.complete = func(_ llm.ModelRef, req llm.Request) (llm.Response, error) {
		if req.Stage != llm.StageNameObserve {
			return llm.Response{}, errors.New("writer used withdrawn sources")
		}
		profiles.withdrawn = true
		return llm.Response{Text: `{"files":[{"file":"one.jpg","scene":"풍경","objects":["나무"]}]}`}, nil
	}
	if _, err := svc.Start(context.Background(), StartRequest{UserID: "alice", PostSlug: "post", ObserveModel: observeRef.String(), WriteModel: writeRef.String()}); err != nil {
		t.Fatal(err)
	}
	err := svc.Generate(context.Background(), jobs.queued(0), func(string, int, int) {})
	if err == nil || len(models.calls) != 1 || models.calls[0].request.Stage != llm.StageNameObserve {
		t.Fatalf("err=%v calls=%v", err, models.calls)
	}
}

func TestAcceptedSourceWithdrawalDuringWriterPreventsOrdinaryPublication(t *testing.T) {
	for _, kind := range []string{"write", "revise"} {
		t.Run(kind, func(t *testing.T) {
			svc, posts, jobs, models, profiles := acceptedProfileHarness()
			models.complete = func(_ llm.ModelRef, _ llm.Request) (llm.Response, error) {
				profiles.withdrawn = true
				return llm.Response{Text: `{"title":"replacement","summary":"summary","tags":["one","two","three","four"],"blocks":[{"type":"TEXT","content":"new content"}]}`}, nil
			}
			ctx := context.Background()
			if kind == "write" {
				if _, err := svc.Start(ctx, StartRequest{UserID: "alice", PostSlug: "post", WriteModel: writeRef.String()}); err != nil {
					t.Fatal(err)
				}
				if err := svc.Generate(ctx, jobs.queued(0), func(string, int, int) {}); err == nil {
					t.Fatal("withdrawn source published")
				}
			} else {
				if _, err := svc.StartRevision(ctx, StartRevisionRequest{UserID: "alice", PostSlug: "post", WriteModel: writeRef.String(), Instruction: "make it clear"}); err != nil {
					t.Fatal(err)
				}
				if err := svc.Revise(ctx, RevisionJob{UserID: "alice", PostSlug: "post", VoiceID: liveVoice.ID, WriteModel: writeRef.String(), Payload: jobs.payloads[0]}, func(string, int, int) {}); err == nil {
					t.Fatal("withdrawn source revised canonical content")
				}
			}
			if len(models.calls) != 1 || len(posts.contents) != 0 || posts.input.Content.Blocks[0].Content != "Original body" {
				t.Fatal("withdrawal replayed work or changed content")
			}
		})
	}
}
