package generation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/llm"
)

var deletedVoice = VoiceRef{ID: "voice-gone", Name: "옛 말투", Deleted: true}

func okContent() llm.Response {
	return llm.Response{Text: `{"title":"t","summary":"s","tags":["a","b","c"],"blocks":[{"type":"TEXT","content":"ok"}]}`}
}

// Plan 10 A7: every AI start refuses a post whose voice is deleted before any queue or
// provider work, and a save-as-rule never lands in a tombstone.
func TestStartsRefuseADeletedVoiceBeforeEnqueue(t *testing.T) {
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: deletedVoice, Content: revisionContent("body")}}
	jobs := &fakeJobs{id: "never"}
	rules := &fakeRules{}
	models := newFakeModels()
	svc := NewService(posts, fakeProfiles{}, rules, models, fakeImages{}, jobs, 4, testReasoningPolicy)

	if _, err := svc.Start(context.Background(), StartRequest{UserID: "alice", PostSlug: "post", WriteModel: writeRef.String()}); !errors.Is(err, ErrVoiceDeleted) {
		t.Fatalf("generation start = %v", err)
	}
	if _, err := svc.StartRevision(context.Background(), StartRevisionRequest{UserID: "alice", PostSlug: "post", Instruction: "더 짧게", SaveAsRule: true, WriteModel: writeRef.String()}); !errors.Is(err, ErrVoiceDeleted) {
		t.Fatalf("revision start = %v", err)
	}
	if _, err := svc.SnapshotWriteInput(context.Background(), "alice", "post", llm.ModelRef{}, nil); !errors.Is(err, ErrVoiceDeleted) {
		t.Fatalf("write experiment snapshot = %v", err)
	}
	if jobs.enqueues != 0 || len(rules.lines) != 0 || len(models.calls) != 0 {
		t.Fatalf("a deleted voice reached the queue, the rules, or a provider: jobs=%d rules=%v calls=%d", jobs.enqueues, rules.lines, len(models.calls))
	}
	posts.input.Voice = VoiceRef{}
	if _, err := svc.Start(context.Background(), StartRequest{UserID: "alice", PostSlug: "post", WriteModel: writeRef.String()}); !errors.Is(err, ErrVoiceRequired) {
		t.Fatalf("voiceless post start = %v", err)
	}
}

// The start freezes the post's voice into the job and the saved rule goes to that voice.
func TestStartsFreezeThePostVoiceIntoTheJob(t *testing.T) {
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: liveVoice, Content: revisionContent("body")}}
	jobs := &fakeJobs{id: "job"}
	rules := &fakeRules{}
	svc := NewService(posts, fakeProfiles{}, rules, newFakeModels(), fakeImages{}, jobs, 4, testReasoningPolicy)
	if _, err := svc.Start(context.Background(), StartRequest{UserID: "alice", PostSlug: "post", WriteModel: writeRef.String()}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.StartRevision(context.Background(), StartRevisionRequest{UserID: "alice", PostSlug: "post", Instruction: "존댓말로", SaveAsRule: true, WriteModel: writeRef.String()}); err != nil {
		t.Fatal(err)
	}
	if len(jobs.generations) != 1 || jobs.generations[0].VoiceID != liveVoice.ID || len(jobs.revisions) != 1 || jobs.revisions[0].VoiceID != liveVoice.ID {
		t.Fatalf("jobs did not freeze the voice: generations=%+v revisions=%+v", jobs.generations, jobs.revisions)
	}
	if len(rules.voices) != 1 || rules.voices[0] != liveVoice.ID {
		t.Fatalf("rule saved to voice %v, want %q", rules.voices, liveVoice.ID)
	}
}

// Plan 10 A9/A14: a queued job whose post has since been reassigned or whose voice has since
// been deleted fails before the provider call, and the prompt only ever reads the post's
// current voice.
func TestHandlersRecheckTheFrozenVoiceBeforeProviderCalls(t *testing.T) {
	models := newFakeModels()
	models.complete = func(llm.ModelRef, llm.Request) (llm.Response, error) { return okContent(), nil }
	profiles := &recordingProfiles{}
	for name, tc := range map[string]struct {
		voice VoiceRef
		job   string
		want  error
	}{
		"reassigned": {liveVoice, "voice-other", ErrVoiceMismatch},
		"deleted":    {deletedVoice, deletedVoice.ID, ErrVoiceDeleted},
		"legacy job": {liveVoice, "", nil},
	} {
		t.Run(name, func(t *testing.T) {
			posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: tc.voice, Content: revisionContent("body")}}
			svc := NewService(posts, profiles, &fakeRules{}, models, fakeImages{}, &fakeJobs{}, 4, testReasoningPolicy)
			calls := len(models.calls)
			err := svc.Generate(context.Background(), GenerateJob{UserID: "alice", PostSlug: "post", VoiceID: tc.job, WriteModel: writeRef.String()}, func(string, int, int) {})
			if !errors.Is(err, tc.want) {
				t.Fatalf("generate = %v, want %v", err, tc.want)
			}
			err = svc.Revise(context.Background(), RevisionJob{UserID: "alice", PostSlug: "post", VoiceID: tc.job, WriteModel: writeRef.String(), Payload: mustRevisionPayload(t, "더 짧게", false)}, func(string, int, int) {})
			if !errors.Is(err, tc.want) {
				t.Fatalf("revise = %v, want %v", err, tc.want)
			}
			if tc.want != nil && (len(models.calls) != calls || len(posts.contents) != 0) {
				t.Fatalf("refused job called a provider or wrote content: calls=%d contents=%d", len(models.calls)-calls, len(posts.contents))
			}
		})
	}
	for _, voiceID := range profiles.voices {
		if voiceID != liveVoice.ID {
			t.Fatalf("profile loaded for voice %q, want %q", voiceID, liveVoice.ID)
		}
	}
}

// A reassignment that lands during the revision's provider call is caught on the fresh
// attachment snapshot, so the output is never persisted under the wrong voice.
func TestRevisionDropsOutputWhenThePostMovesMidCall(t *testing.T) {
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: liveVoice, Content: revisionContent("before")}}
	models := newFakeModels()
	models.complete = func(llm.ModelRef, llm.Request) (llm.Response, error) {
		posts.input.Voice = VoiceRef{ID: "voice-other", Name: "리뷰"}
		return okContent(), nil
	}
	svc := NewService(posts, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, &fakeJobs{}, 4, testReasoningPolicy)
	err := svc.Revise(context.Background(), RevisionJob{UserID: "alice", PostSlug: "post", VoiceID: liveVoice.ID, WriteModel: writeRef.String(), Payload: mustRevisionPayload(t, "더 짧게", false)}, func(string, int, int) {})
	if !errors.Is(err, ErrVoiceMismatch) || len(posts.contents) != 0 {
		t.Fatalf("mid-call reassignment: err=%v contents=%d", err, len(posts.contents))
	}
}

// Plan 10 A10: two voices with contradictory profiles each receive only their own
// projection, through generation and through five repeated revisions.
func TestContradictoryVoicesReceiveOnlyTheirOwnProjection(t *testing.T) {
	casual := VoiceRef{ID: "voice-casual", Name: "일상"}
	formal := VoiceRef{ID: "voice-formal", Name: "격식"}
	profiles := voiceProfiles{
		casual.ID: {Styleguide: "CASUAL-STYLE ~해요", ActiveRules: "CASUAL-RULE", Excerpts: []string{"CASUAL-EXCERPT"}, Rules: "CASUAL-MANUAL"},
		formal.ID: {Styleguide: "FORMAL-STYLE ~습니다", ActiveRules: "FORMAL-RULE", Excerpts: []string{"FORMAL-EXCERPT"}, Rules: "FORMAL-MANUAL"},
	}
	models := newFakeModels()
	models.complete = func(llm.ModelRef, llm.Request) (llm.Response, error) { return okContent(), nil }
	for _, voice := range []VoiceRef{casual, formal} {
		other := formal
		if voice.ID == formal.ID {
			other = casual
		}
		posts := &fakePosts{input: PostInput{Slug: "post-" + voice.ID, UserID: "alice", Voice: voice, Content: revisionContent("body")}}
		svc := NewService(posts, profiles, &fakeRules{}, models, fakeImages{}, &fakeJobs{}, 4, testReasoningPolicy)
		if err := svc.Generate(context.Background(), GenerateJob{UserID: "alice", PostSlug: posts.input.Slug, VoiceID: voice.ID, WriteModel: writeRef.String()}, func(string, int, int) {}); err != nil {
			t.Fatal(err)
		}
		for pass := 1; pass <= 5; pass++ {
			if err := svc.Revise(context.Background(), RevisionJob{UserID: "alice", PostSlug: posts.input.Slug, VoiceID: voice.ID, WriteModel: writeRef.String(), Payload: mustRevisionPayload(t, fmt.Sprintf("pass %d", pass), false)}, func(string, int, int) {}); err != nil {
				t.Fatalf("pass %d: %v", pass, err)
			}
		}
		own, foreign := profiles[voice.ID], profiles[other.ID]
		for _, call := range models.calls {
			system := call.request.System
			if !strings.Contains(system, own.Styleguide) || !strings.Contains(system, own.ActiveRules) || !strings.Contains(system, own.Excerpts[0]) || !strings.Contains(system, own.Rules) {
				t.Fatalf("%s prompt lost its own projection: %s", voice.Name, system)
			}
			for _, leak := range []string{foreign.Styleguide, foreign.ActiveRules, foreign.Excerpts[0], foreign.Rules} {
				if strings.Contains(system, leak) {
					t.Fatalf("%s prompt contains %s's %q", voice.Name, other.Name, leak)
				}
			}
		}
		models.calls = nil
	}
}

// ApplyWriteWinner is a machine result landing in a voice: it refuses when the post has
// left the frozen snapshot's voice or the voice is gone.
func TestApplyWriteWinnerRequiresTheFrozenVoice(t *testing.T) {
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: liveVoice, Memo: "memo"}}
	models := newFakeModels()
	svc := NewService(posts, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, &fakeJobs{}, 4, testReasoningPolicy)
	raw, err := svc.SnapshotWriteInput(context.Background(), "alice", "post", llm.ModelRef{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := SnapshotVoice(raw); got != liveVoice.ID {
		t.Fatalf("SnapshotVoice = %q", got)
	}
	winner := PostContent{Title: "w", Blocks: []Block{{Type: BlockText, Content: "ok"}}}
	posts.input.Voice = VoiceRef{ID: "voice-other", Name: "리뷰"}
	if err := svc.ApplyWriteWinner(context.Background(), "alice", "post", winner, raw); !errors.Is(err, ErrVoiceMismatch) || len(posts.contents) != 0 {
		t.Fatalf("apply after reassignment: err=%v contents=%d", err, len(posts.contents))
	}
	posts.input.Voice = deletedVoice
	if err := svc.ApplyWriteWinner(context.Background(), "alice", "post", winner, raw); !errors.Is(err, ErrVoiceDeleted) {
		t.Fatalf("apply into deleted voice: %v", err)
	}
	posts.input.Voice = liveVoice
	if err := svc.ApplyWriteWinner(context.Background(), "alice", "post", winner, raw); err != nil || len(posts.contents) != 1 {
		t.Fatalf("apply in frozen voice: err=%v contents=%d", err, len(posts.contents))
	}
}

// voiceProfiles serves a different projection per voice id and fails on an unknown one, so a
// fallback to another voice would surface as an error rather than a silent borrow.
type voiceProfiles map[string]Profile

func (p voiceProfiles) ProfileForPrompt(_ context.Context, _, voiceID string, _ Language) (Profile, error) {
	profile, ok := p[voiceID]
	if !ok {
		return Profile{}, fmt.Errorf("no profile for voice %q", voiceID)
	}
	return profile, nil
}
