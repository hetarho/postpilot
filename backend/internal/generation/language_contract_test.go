package generation

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/llm"
)

type languageRecordingProfiles struct {
	profile Profile
	targets []Language
}

func (p *languageRecordingProfiles) ProfileForPrompt(_ context.Context, _, _ string, target Language) (Profile, error) {
	p.targets = append(p.targets, target)
	profile := p.profile
	profile.TargetLanguage = target
	if profile.SourceLanguage == "" {
		profile.SourceLanguage = target
	}
	return profile, nil
}

func languagePointer(value Language) *Language { return &value }

func TestGenerationPayloadRequiresAndFreezesCanonicalTargetLanguage(t *testing.T) {
	for _, invalid := range []Language{"", "fr"} {
		if _, err := EncodeGenerationPayload(GenerationOptions{TargetLanguage: invalid}); !strings.Contains(err.Error(), ErrLanguageRequired.Error()) {
			t.Fatalf("EncodeGenerationPayload(%q) error = %v", invalid, err)
		}
	}
	raw, err := EncodeGenerationPayload(GenerationOptions{TargetLanguage: LanguageEnglish})
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeGenerationPayload(raw)
	if err != nil || decoded.TargetLanguage != LanguageEnglish {
		t.Fatalf("decoded = %+v, err = %v", decoded, err)
	}
	for _, legacy := range [][]byte{nil, []byte(`{}`)} {
		decoded, err := DecodeGenerationPayload(legacy)
		if err != nil || decoded.TargetLanguage != LanguageKorean {
			t.Fatalf("legacy payload %q decoded = %+v, err = %v", legacy, decoded, err)
		}
	}
}

func TestOrdinaryGenerationUsesFrozenTargetAndWritesMatchingProvenance(t *testing.T) {
	posts := &fakePosts{input: PostInput{
		Slug: "post", UserID: "alice", Voice: VoiceRef{ID: "voice", SourceLanguage: LanguageKorean},
		Title: "title", Memo: "memo", TargetLanguage: LanguageEnglish,
	}}
	jobs := &fakeJobs{id: "job"}
	profiles := &languageRecordingProfiles{profile: Profile{Styleguide: "PORTABLE", SourceLanguage: LanguageKorean, Portable: true}}
	models := newFakeModels()
	models.complete = func(_ llm.ModelRef, request llm.Request) (llm.Response, error) {
		if !strings.Contains(request.System, "The output language is English") {
			t.Fatalf("English target was not used:\n%s", request.System)
		}
		if strings.Contains(request.System, NaturalnessBaseline) || strings.Contains(request.System, "[한국어 자연 문체 기준선]") {
			t.Fatalf("Korean baseline leaked into English generation:\n%s", request.System)
		}
		return llm.Response{Text: `{"title":"English title","summary":"Summary","tags":["one","two","three"],"blocks":[{"type":"TEXT","content":"Body"}]}`}, nil
	}
	svc := NewService(posts, profiles, &fakeRules{}, models, fakeImages{}, jobs, 4, testReasoningPolicy)
	if _, err := svc.Start(context.Background(), StartRequest{UserID: "alice", PostSlug: "post", WriteModel: writeRef.String()}); err != nil {
		t.Fatal(err)
	}
	if len(jobs.generations) != 1 || jobs.generations[0].TargetLanguage != LanguageEnglish {
		t.Fatalf("enqueued generation = %+v", jobs.generations)
	}

	// A target selection after enqueue is deliberately newer than this frozen run.
	posts.input.TargetLanguage = LanguageKorean
	request := jobs.generations[0]
	err := svc.Generate(context.Background(), GenerateJob{
		UserID: request.UserID, PostSlug: request.PostSlug, VoiceID: request.VoiceID,
		WriteModel: request.WriteModel, TargetLanguage: request.TargetLanguage,
	}, func(string, int, int) {})
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles.targets) != 1 || profiles.targets[0] != LanguageEnglish {
		t.Fatalf("profile targets = %v", profiles.targets)
	}
	if len(posts.contentLanguages) != 1 || posts.contentLanguages[0] != LanguageEnglish {
		t.Fatalf("content languages = %v", posts.contentLanguages)
	}
	if posts.input.TargetLanguage != LanguageKorean || posts.input.ContentLanguage == nil || *posts.input.ContentLanguage != LanguageEnglish {
		t.Fatalf("post target/provenance = %q / %v", posts.input.TargetLanguage, posts.input.ContentLanguage)
	}
}

func TestObservationInputsAreByteIdenticalAcrossTargets(t *testing.T) {
	base := PostInput{
		Slug: "post", UserID: "alice", Voice: liveVoice,
		Images: []Image{{Filename: "IMG.jpg", Key: "key"}},
	}
	base.TargetLanguage = LanguageKorean
	koRaw, err := NewService(&fakePosts{input: base}, fakeProfiles{}, &fakeRules{}, newFakeModels(), fakeImages{}, &fakeJobs{}, 4, testReasoningPolicy).
		SnapshotObserveInput(context.Background(), "alice", "post")
	if err != nil {
		t.Fatal(err)
	}
	base.TargetLanguage = LanguageEnglish
	enRaw, err := NewService(&fakePosts{input: base}, fakeProfiles{}, &fakeRules{}, newFakeModels(), fakeImages{}, &fakeJobs{}, 4, testReasoningPolicy).
		SnapshotObserveInput(context.Background(), "alice", "post")
	if err != nil {
		t.Fatal(err)
	}
	if string(koRaw) != string(enRaw) {
		t.Fatalf("observation snapshots differ by target:\nko=%s\nen=%s", koRaw, enRaw)
	}
	for _, forbidden := range []string{"target_language", "content_language", "source_language", "profile"} {
		if strings.Contains(string(koRaw), forbidden) {
			t.Fatalf("observation snapshot contains %q: %s", forbidden, koRaw)
		}
	}

	models := newFakeModels()
	models.complete = func(_ llm.ModelRef, _ llm.Request) (llm.Response, error) {
		return llm.Response{Text: `{"observations":[{"file":"IMG.jpg","scene":"same","mood":"","visible_text":"","objects":[],"people_present":false}]}`}, nil
	}
	svc := NewService(&fakePosts{}, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, &fakeJobs{}, 4, testReasoningPolicy)
	for _, target := range []Language{LanguageKorean, LanguageEnglish} {
		post := base
		post.TargetLanguage = target
		if _, _, err := svc.observeCandidate(context.Background(), post, post.Images, nil, observeRef, func(string, int, int) {}, false); err != nil {
			t.Fatal(err)
		}
	}
	if len(models.calls) != 2 || !reflect.DeepEqual(models.calls[0].request, models.calls[1].request) {
		t.Fatalf("observation requests differ by target:\nko=%+v\nen=%+v", models.calls[0].request, models.calls[1].request)
	}
}

func TestLanguageAwarePromptsKeepKoreanBaselineAndDefendPortableProjection(t *testing.T) {
	leaky := Profile{
		Styleguide: "PORTABLE-STRUCTURE", ActiveRules: "DO-NOT-LEAK-ACTIVE",
		Excerpts: []string{"DO-NOT-LEAK-EXCERPT"}, Rules: "DO-NOT-LEAK-RULE",
		EndingMaxConsecutive: 7, SourceLanguage: LanguageKorean, TargetLanguage: LanguageEnglish, Portable: true,
	}
	english, _ := BuildWritePromptForLanguage(LanguageEnglish, leaky, nil, "memo", "title", nil, nil, nil, nil)
	for _, required := range []string{"The output language is English", "title, summary, tags", "IMAGE alt and caption", "PORTABLE-STRUCTURE", "Portable voice profile"} {
		if !strings.Contains(english, required) {
			t.Errorf("English prompt missing %q", required)
		}
	}
	for _, forbidden := range []string{NaturalnessBaseline, "[한국어 자연 문체 기준선]", "DO-NOT-LEAK-ACTIVE", "DO-NOT-LEAK-EXCERPT", "DO-NOT-LEAK-RULE", "종결어미 제약"} {
		if strings.Contains(english, forbidden) {
			t.Errorf("English portable prompt leaked %q", forbidden)
		}
	}
	korean, _ := BuildWritePromptForLanguage(LanguageKorean, Profile{}, nil, "", "", nil, nil, nil, nil)
	if strings.Count(korean, NaturalnessBaseline) != 1 {
		t.Fatalf("Korean baseline count = %d", strings.Count(korean, NaturalnessBaseline))
	}
}

func TestWriteExperimentFreezesTargetForCandidatesAndWinner(t *testing.T) {
	posts := &fakePosts{input: PostInput{
		Slug: "post", UserID: "alice", Voice: VoiceRef{ID: "voice", SourceLanguage: LanguageKorean},
		TargetLanguage: LanguageEnglish,
	}}
	models := newFakeModels()
	models.infos[writeRefB] = llm.ModelInfo{Ref: writeRefB, StructuredOutput: true}
	models.complete = func(ref llm.ModelRef, _ llm.Request) (llm.Response, error) {
		return llm.Response{Text: `{"title":"` + ref.ModelID + `","summary":"s","tags":["a","b","c"],"blocks":[{"type":"TEXT","content":"body"}]}`}, nil
	}
	profiles := &languageRecordingProfiles{profile: Profile{Styleguide: "PORTABLE", SourceLanguage: LanguageKorean, Portable: true}}
	svc := NewService(posts, profiles, &fakeRules{}, models, fakeImages{}, &fakeJobs{}, 4, testReasoningPolicy)
	raw, err := svc.SnapshotWriteInput(context.Background(), "alice", "post", llm.ModelRef{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := SnapshotTargetLanguage(raw); got != LanguageEnglish {
		t.Fatalf("snapshot target = %q", got)
	}
	posts.input.TargetLanguage = LanguageKorean
	profiles.profile.Styleguide = "MUTATED"
	prepared, err := svc.PrepareWriteInput(context.Background(), raw, func(string, int, int) {})
	if err != nil {
		t.Fatal(err)
	}
	left, _, err := svc.RunWriteCandidate(context.Background(), prepared, writeRef)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.RunWriteCandidate(context.Background(), prepared, writeRefB); err != nil {
		t.Fatal(err)
	}
	if len(models.calls) != 2 || !reflect.DeepEqual(models.calls[0].request, models.calls[1].request) {
		t.Fatalf("A/B candidate requests differ: %+v", models.calls)
	}
	if strings.Contains(models.calls[0].request.System, "MUTATED") || !strings.Contains(models.calls[0].request.System, "PORTABLE") {
		t.Fatalf("profile was not frozen:\n%s", models.calls[0].request.System)
	}
	if err := svc.ApplyWriteWinner(context.Background(), "alice", "post", left, prepared); err != nil {
		t.Fatal(err)
	}
	if len(posts.contentLanguages) != 1 || posts.contentLanguages[0] != LanguageEnglish || posts.input.TargetLanguage != LanguageKorean {
		t.Fatalf("winner target/provenance = %q / %v", posts.input.TargetLanguage, posts.contentLanguages)
	}
}

func TestRevisionFreezesContentLanguageAcrossTargetChangeAndFivePasses(t *testing.T) {
	posts := &fakePosts{input: PostInput{
		Slug: "post", UserID: "alice", Voice: VoiceRef{ID: "voice", SourceLanguage: LanguageKorean},
		TargetLanguage: LanguageEnglish, ContentLanguage: languagePointer(LanguageEnglish), Content: revisionContent("pass-0"),
	}}
	jobs := &fakeJobs{id: "revision-job"}
	profiles := &languageRecordingProfiles{profile: Profile{Styleguide: "PORTABLE", SourceLanguage: LanguageKorean, Portable: true}}
	models := newFakeModels()
	models.complete = func(_ llm.ModelRef, request llm.Request) (llm.Response, error) {
		if !strings.Contains(request.System, "Preserve English") || !strings.Contains(request.System, "Translation is outside revision semantics") {
			t.Fatalf("revision language contract missing:\n%s", request.System)
		}
		if strings.Contains(request.System, NaturalnessBaseline) {
			t.Fatalf("Korean baseline leaked into English revision:\n%s", request.System)
		}
		return llm.Response{Text: `{"title":"title","summary":"summary","tags":["a","b","c"],"blocks":[{"type":"TEXT","content":"next"}]}`}, nil
	}
	svc := NewService(posts, profiles, &fakeRules{}, models, fakeImages{}, jobs, 4, testReasoningPolicy)
	_, err := svc.StartRevision(context.Background(), StartRevisionRequest{
		UserID: "alice", PostSlug: "post", Instruction: "Translate to Korean and shorten it", WriteModel: writeRef.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := parseRevisionPayload(jobs.payloads[0])
	if err != nil || payload.ContentLanguage != LanguageEnglish || jobs.revisions[0].ContentLanguage != LanguageEnglish {
		t.Fatalf("frozen revision payload/request = %+v / %+v, err = %v", payload, jobs.revisions, err)
	}

	posts.input.TargetLanguage = LanguageKorean
	for pass := 0; pass < 5; pass++ {
		if err := svc.Revise(context.Background(), RevisionJob{
			UserID: "alice", PostSlug: "post", VoiceID: "voice", WriteModel: writeRef.String(), Payload: jobs.payloads[0],
		}, func(string, int, int) {}); err != nil {
			t.Fatalf("pass %d: %v", pass+1, err)
		}
	}
	if len(profiles.targets) != 5 || len(posts.contentLanguages) != 5 {
		t.Fatalf("profile targets/content writes = %v / %v", profiles.targets, posts.contentLanguages)
	}
	for pass := range posts.contentLanguages {
		if profiles.targets[pass] != LanguageEnglish || posts.contentLanguages[pass] != LanguageEnglish {
			t.Fatalf("pass %d target/provenance = %q/%q", pass+1, profiles.targets[pass], posts.contentLanguages[pass])
		}
	}
	if posts.input.TargetLanguage != LanguageKorean || posts.input.ContentLanguage == nil || *posts.input.ContentLanguage != LanguageEnglish {
		t.Fatalf("final target/provenance = %q / %v", posts.input.TargetLanguage, posts.input.ContentLanguage)
	}
}

func TestRevisionRejectsMissingProvenanceBeforeMutationOrProvider(t *testing.T) {
	posts := &fakePosts{input: PostInput{
		Slug: "post", UserID: "alice", Voice: liveVoice, TargetLanguage: LanguageKorean, Content: revisionContent("body"),
	}, preserveMissingLanguages: true}
	jobs := &fakeJobs{id: "must-not-enqueue"}
	rules := &fakeRules{}
	models := newFakeModels()
	svc := NewService(posts, fakeProfiles{}, rules, models, fakeImages{}, jobs, 4, testReasoningPolicy)
	_, err := svc.StartRevision(context.Background(), StartRevisionRequest{
		UserID: "alice", PostSlug: "post", Instruction: "shorten", SaveAsRule: true, WriteModel: writeRef.String(),
	})
	if err != ErrContentLanguageRequired {
		t.Fatalf("error = %v", err)
	}
	if jobs.enqueues != 0 || len(rules.lines) != 0 || len(models.calls) != 0 {
		t.Fatalf("rejected revision mutated work: jobs=%d rules=%v calls=%d", jobs.enqueues, rules.lines, len(models.calls))
	}
}

func TestRevisionSaveAsRuleRejectsCrossLanguageContentBeforeRuleQueueOrProvider(t *testing.T) {
	posts := &fakePosts{input: PostInput{
		Slug: "post", UserID: "alice", Voice: VoiceRef{ID: "voice", SourceLanguage: LanguageKorean},
		TargetLanguage: LanguageEnglish, ContentLanguage: languagePointer(LanguageEnglish), Content: revisionContent("English content"),
	}}
	jobs := &fakeJobs{id: "must-not-enqueue"}
	rules := &fakeRules{}
	models := newFakeModels()
	svc := NewService(posts, fakeProfiles{}, rules, models, fakeImages{}, jobs, 4, testReasoningPolicy)
	_, err := svc.StartRevision(context.Background(), StartRevisionRequest{
		UserID: "alice", PostSlug: "post", Instruction: "make this my rule", SaveAsRule: true, WriteModel: writeRef.String(),
	})
	if !errors.Is(err, ErrVoiceContentLanguageMismatch) {
		t.Fatalf("error = %v, want ErrVoiceContentLanguageMismatch", err)
	}
	if jobs.enqueues != 0 || len(rules.lines) != 0 || len(models.calls) != 0 {
		t.Fatalf("mismatched rule revision mutated work: jobs=%d rules=%v calls=%d", jobs.enqueues, rules.lines, len(models.calls))
	}
}
