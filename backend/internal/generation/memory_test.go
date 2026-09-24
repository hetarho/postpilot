package generation

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/post"
)

func testMemories() []string {
	return []string{"매운 음식을 못 먹는다", "연남동에 자주 간다"}
}

// MEM-20: the section renders in the PER-POST half, between the memo and the attachments,
// and the stable half — the one the provider caches and every golden pins — is untouched.
func TestMemoriesRenderInThePerPostHalfOnly(t *testing.T) {
	baselineSystem, baselineUser := loadGolden(t, "write_prompt_no_template.golden")
	system, user := BuildWritePromptForLanguage(WritePromptInput{
		Language:     LanguageKorean,
		Profile:      goldenProfile(),
		Observations: goldenObservations(),
		Memo:         "MEMO 본문",
		Title:        "가제 TITLE",
		Photos:       []string{"IMG_1.jpg", "IMG_2.jpg"},
		TagCount:     post.TagCountRange.Default,
		Memories:     testMemories(),
	})

	if system != baselineSystem {
		t.Fatalf("the memories reached the stable prefix:\n%s", system)
	}
	if user == baselineUser {
		t.Fatal("the memories reached nothing at all")
	}
	// Between the memo and the attachment material, exactly as GEN-14 orders the per-post half.
	memoAt := strings.Index(user, "메모: MEMO 본문")
	sectionAt := strings.Index(user, "[기억]")
	attachmentsAt := strings.Index(user, "첨부 파일명")
	if memoAt < 0 || sectionAt < memoAt || attachmentsAt < sectionAt {
		t.Fatalf("section order = memo %d, 기억 %d, 첨부 %d:\n%s", memoAt, sectionAt, attachmentsAt, user)
	}
	for _, text := range testMemories() {
		if !strings.Contains(user, "- "+text) {
			t.Errorf("the per-post half is missing %q", text)
		}
	}
	// MEM-21 and GUIDE-28: the closing line, once, naming the material and the precedence.
	if strings.Count(user, memoryPrecedence) != 1 {
		t.Fatalf("the precedence line appears %d times", strings.Count(user, memoryPrecedence))
	}
	if !strings.Contains(memoryPrecedence, "지침") {
		t.Fatal("the closing line does not state that a guideline outranks a memory")
	}
}

// MEM-1's promise, asserted directly against T287's goldens: a post with the option off —
// which is every post that never turned it on — produces the prompt it produced before this
// domain existed, byte for byte, in both halves.
func TestAPostWithoutMemoriesIsByteIdenticalToTheBaseline(t *testing.T) {
	wantSystem, wantUser := loadGolden(t, "write_prompt_no_template.golden")
	for name, memories := range map[string][]string{"nil": nil, "empty": {}} {
		system, user := BuildWritePrompt(goldenProfile(), goldenObservations(), "MEMO 본문", "가제 TITLE",
			[]string{"IMG_1.jpg", "IMG_2.jpg"}, nil, nil, nil)
		if system != wantSystem || user != wantUser {
			t.Fatalf("%s memories drifted from the baseline", name)
		}
		withExplicit, userExplicit := BuildWritePromptForLanguage(WritePromptInput{
			Language:     LanguageKorean,
			Profile:      goldenProfile(),
			Observations: goldenObservations(),
			Memo:         "MEMO 본문",
			Title:        "가제 TITLE",
			Photos:       []string{"IMG_1.jpg", "IMG_2.jpg"},
			TagCount:     post.TagCountRange.Default,
			Memories:     memories,
		})
		if withExplicit != wantSystem || userExplicit != wantUser {
			t.Fatalf("%s memories drifted from the baseline through the language builder", name)
		}
		if strings.Contains(userExplicit, "기억") {
			t.Fatalf("%s memories mentioned the section: %s", name, userExplicit)
		}
	}
}

// The whole prompt with memories, pinned. The diff against write_prompt_no_template.golden
// is exactly the section, which is what makes the addition reviewable.
func TestWritePromptWithMemoriesMatchesItsGolden(t *testing.T) {
	wantSystem, wantUser := loadGolden(t, "write_prompt_memories.golden")
	system, user := BuildWritePromptForLanguage(WritePromptInput{
		Language:     LanguageKorean,
		Profile:      goldenProfile(),
		Observations: goldenObservations(),
		Memo:         "MEMO 본문",
		Title:        "가제 TITLE",
		Photos:       []string{"IMG_1.jpg", "IMG_2.jpg"},
		TagCount:     post.TagCountRange.Default,
		Memories:     testMemories(),
	})
	if system != wantSystem {
		t.Fatalf("system drifted:\n--- got ---\n%s\n--- want ---\n%s", system, wantSystem)
	}
	if user != wantUser {
		t.Fatalf("per-post material drifted:\n--- got ---\n%s\n--- want ---\n%s", user, wantUser)
	}
}

// MEM-22: the revise pass receives no memories in any case. It holds neither the memo nor
// the observations, so material it cannot check against would license rewriting sentences
// the request never touched.
func TestTheRevisePromptCarriesNoMemories(t *testing.T) {
	system, user := BuildRevisePrompt(goldenProfile(), goldenContent(), []string{"IMG_1.jpg"}, "INSTRUCTION 수정 요청", nil, nil, nil)
	for _, half := range []string{system, user} {
		if strings.Contains(half, "[기억]") || strings.Contains(half, memoryPrecedence) {
			t.Fatalf("the revise prompt carries the memory section:\n%s", half)
		}
		for _, text := range testMemories() {
			if strings.Contains(half, text) {
				t.Fatalf("the revise prompt carries a memory text: %s", text)
			}
		}
	}
}

// The payload is what the worker reads, so the freeze is only real if it survives the
// encode: a memory edited or deleted after the start cannot reach queued work.
func TestMemoriesAreFrozenIntoThePayload(t *testing.T) {
	raw, err := encodeGenerationPayload(generationOptions{TargetLanguage: LanguageKorean, writeMaterial: writeMaterial{Memories: testMemories()}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"memories":["매운 음식을 못 먹는다","연남동에 자주 간다"]`) {
		t.Fatalf("payload = %s", raw)
	}
	back, err := decodeGenerationPayload(raw)
	if err != nil || len(back.Memories) != 2 || back.Memories[1] != "연남동에 자주 간다" {
		t.Fatalf("round trip = %+v, %v", back.Memories, err)
	}

	// A payload written before memories existed — and one frozen for a post with the option
	// off — are the same absent member, and both decode as none.
	legacy, err := decodeGenerationPayload([]byte(`{"target_language":"ko","observe_files":null}`))
	if err != nil || legacy.Memories != nil {
		t.Fatalf("a payload without the member decoded %v (%v)", legacy.Memories, err)
	}
	off, err := encodeGenerationPayload(generationOptions{TargetLanguage: LanguageKorean})
	if err != nil {
		t.Fatal(err)
	}
	var shape map[string]json.RawMessage
	if err := json.Unmarshal(off, &shape); err != nil {
		t.Fatal(err)
	}
	if _, present := shape["memories"]; present {
		t.Fatalf("a post with the option off wrote the member: %s", off)
	}
}

// The opt-in is the only thing that decides whether this context asks the memory context
// anything: a post with it off never reaches the port, so a retrieval cannot cost it a
// query — and a post with it on hands over its own words and nothing else (MEM-7, MEM-18).
func TestOnlyAPostThatOptedInReachesTheMemoryPort(t *testing.T) {
	recorder := &recordingMemories{texts: testMemories()}
	service := NewService(&fakePosts{}, fakeProfiles{}, &fakeRules{}, newFakeModels(), fakeImages{}, &fakeJobs{}, 4, testReasoningPolicy, testBudget, testDeps())
	service.memories = recorder

	off := PostInput{UserID: "alice", Memo: "메모", Title: "가제"}
	texts, err := service.freezeMemories(context.Background(), off)
	if err != nil || texts != nil {
		t.Fatalf("a post with the option off retrieved %v (%v)", texts, err)
	}
	if recorder.calls != 0 {
		t.Fatalf("a post with the option off made %d calls", recorder.calls)
	}

	on := PostInput{
		UserID: "alice", UseMemory: true, Memo: "연남동에서 점심", Title: "가제",
		TemplateAnswers: []TemplateAnswer{
			{Label: "장소", Text: "연남동 식당", Enabled: true},
			{Label: "숨김", Text: "쓰지 않는 답", Enabled: false},
		},
		Observations: []Observation{{
			File: "a.jpg", Scene: "관찰자의 산문", Mood: "고요함",
			Objects: []string{"접시"}, VisibleText: "영업중",
		}},
	}
	texts, err = service.freezeMemories(context.Background(), on)
	if err != nil || len(texts) != 2 {
		t.Fatalf("freeze = %v (%v)", texts, err)
	}
	key := strings.Join(recorder.lastKey, "|")
	for _, want := range []string{"연남동에서 점심", "가제", "연남동 식당", "접시", "영업중"} {
		if !strings.Contains(key, want) {
			t.Errorf("the retrieval key is missing %q: %s", want, key)
		}
	}
	// A switched-off answer is not part of the post, and the observer's prose is not the
	// post's own words: neither may pull a memory in.
	for _, unwanted := range []string{"쓰지 않는 답", "관찰자의 산문", "고요함"} {
		if strings.Contains(key, unwanted) {
			t.Errorf("the retrieval key carries %q: %s", unwanted, key)
		}
	}
}

// memoryDrainService is a service whose post has 기억 사용 on, so only the payload can keep
// a memory out of a run: a live retrieval would find the recorder's texts.
func memoryDrainService(recorder *recordingMemories) (*Service, *fakeJobs, *fakeModels) {
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: liveVoice, UseMemory: true, Memo: "연남동에서 점심"}}
	jobs := &fakeJobs{id: "job"}
	models := newFakeModels()
	models.complete = func(llm.ModelRef, llm.Request) (llm.Response, error) { return okContent(), nil }
	svc := NewService(posts, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, jobs, 4, testReasoningPolicy, testBudget, testDeps())
	svc.memories = recorder
	return svc, jobs, models
}

// MEM-19 through the drain: the texts Start froze are the texts the write prompt carries, even
// when the memory context answers differently by the time the job runs, and Generate never asks
// it again. They land in the per-post half as one section; the stable half is the run's without
// memories, byte for byte (MEM-20).
func TestADurableGenerateCarriesItsFrozenMemories(t *testing.T) {
	ctx := context.Background()
	recorder := &recordingMemories{texts: testMemories()}
	svc, jobs, models := memoryDrainService(recorder)

	if _, err := svc.Start(ctx, StartRequest{UserID: "alice", PostSlug: "post", WriteModel: writeRef.String()}); err != nil {
		t.Fatal(err)
	}
	frozen := jobs.frozen(t, 0).Memories
	if len(frozen) != 2 {
		t.Fatalf("the start froze %v", frozen)
	}

	// Between the enqueue and the drain every memory is edited or deleted.
	recorder.texts = []string{"바뀐 기억"}

	job := GenerateJob{UserID: "alice", PostSlug: "post", VoiceID: liveVoice.ID, WriteModel: writeRef.String(), Payload: mustGeneratePayload(t, generationOptions{writeMaterial: writeMaterial{Memories: frozen}})}
	if err := svc.Generate(ctx, job, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	sent := models.calls[0].request
	user := sent.Messages[0].Parts[0].Text
	section := "[기억]\n- 매운 음식을 못 먹는다\n- 연남동에 자주 간다\n" + memoryPrecedence + "\n"
	if !strings.Contains(user, section) || strings.Count(user, "[기억]") != 1 || strings.Count(user, memoryPrecedence) != 1 {
		t.Fatalf("the per-post half does not carry the frozen section once:\n%s", user)
	}
	if strings.Contains(sent.System, "[기억]") || strings.Contains(sent.System+user, "바뀐 기억") {
		t.Fatalf("the drain read the live memories or reached the stable half:\n%s\n%s", sent.System, user)
	}
	if recorder.calls != 1 {
		t.Fatalf("memories were retrieved %d times; only the enqueue may", recorder.calls)
	}

	job.Payload = mustGeneratePayload(t, generationOptions{})
	if err := svc.Generate(ctx, job, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	without := models.calls[1].request
	if without.System != sent.System {
		t.Fatal("the memories changed the stable half")
	}
	if strings.Replace(user, section, "", 1) != without.Messages[0].Parts[0].Text {
		t.Fatal("the memories changed the per-post half beyond their section")
	}
}

// A job whose payload holds no memories — the option was off, or the payload predates the
// member — writes the prompt it wrote before: no section, no closing line and no memory text,
// and nothing is retrieved, even for a post that has the option on by now.
func TestADurableGenerateWithoutMemoriesIsUnchanged(t *testing.T) {
	legacy, err := decodeGenerationPayload([]byte(`{"target_language":"ko","observe_files":null}`))
	if err != nil {
		t.Fatal(err)
	}
	var first *llm.Request
	for name, memories := range map[string][]string{"nil": nil, "empty": {}, "legacy payload": legacy.Memories} {
		recorder := &recordingMemories{texts: testMemories()}
		svc, _, models := memoryDrainService(recorder)
		if err := svc.Generate(context.Background(), GenerateJob{
			UserID:     "alice",
			PostSlug:   "post",
			VoiceID:    liveVoice.ID,
			WriteModel: writeRef.String(),
			Payload:    mustGeneratePayload(t, generationOptions{writeMaterial: writeMaterial{Memories: memories}}),
		}, func(string, int, int) {}); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		sent := models.calls[0].request
		for _, half := range []string{sent.System, sent.Messages[0].Parts[0].Text} {
			if strings.Contains(half, "기억") {
				t.Fatalf("%s: the prompt carries memory bytes:\n%s", name, half)
			}
			for _, text := range testMemories() {
				if strings.Contains(half, text) {
					t.Fatalf("%s: the prompt carries %q", name, text)
				}
			}
		}
		if recorder.calls != 0 {
			t.Fatalf("%s: the drain retrieved memories %d times", name, recorder.calls)
		}
		if first == nil {
			first = &sent
		} else if sent.System != first.System || sent.Messages[0].Parts[0].Text != first.Messages[0].Parts[0].Text {
			t.Fatalf("%s: the prompt differs from the other memory-less jobs", name)
		}
	}
}

type recordingMemories struct {
	texts   []string
	calls   int
	lastKey []string
}

func (r *recordingMemories) ForPost(_ context.Context, _ string, keyParts []string) ([]string, error) {
	r.calls++
	r.lastKey = keyParts
	return r.texts, nil
}

// MEM-19, GEN-18, MODEL-30: a write comparison of a post with 기억 사용 on freezes the memories
// once, at snapshot time, and both candidates write with the same [기억] section. A memory edited
// after the snapshot reaches neither; a retrieval that finds nothing snapshots `null`, as every
// snapshot did before, and so differs from one that found memories.
func TestWriteSnapshotFreezesMemoriesForBothCandidates(t *testing.T) {
	ctx := context.Background()
	recorder := &recordingMemories{texts: testMemories()}
	svc, _, models := memoryDrainService(recorder)
	raw, err := svc.SnapshotWriteInput(ctx, "alice", "post", llm.ModelRef{}, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := svc.PrepareWriteInput(ctx, raw, func(string, int, int) {})
	if err != nil {
		t.Fatal(err)
	}
	recorder.texts = []string{"편집됨"}
	for _, ref := range []llm.ModelRef{writeRef, observeRef} {
		if _, _, err := svc.RunWriteCandidate(ctx, prepared, ref); err != nil {
			t.Fatal(err)
		}
	}
	left, right := models.calls[0].request.Messages[0].Parts[0].Text, models.calls[1].request.Messages[0].Parts[0].Text
	if left != right {
		t.Fatalf("the candidates wrote from different per-post halves:\n%s\n---\n%s", left, right)
	}
	for _, want := range append([]string{"[기억]"}, testMemories()...) {
		if !strings.Contains(left, want) {
			t.Errorf("the comparison's per-post half lacks %q:\n%s", want, left)
		}
	}
	if strings.Contains(left, "편집됨") || recorder.calls != 1 {
		t.Fatalf("the comparison read the memories again: %d calls\n%s", recorder.calls, left)
	}

	recorder.texts = nil
	none, err := svc.SnapshotWriteInput(ctx, "alice", "post", llm.ModelRef{}, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(none), `"Memories":null`) || string(none) == string(raw) {
		t.Fatalf("a retrieval that found nothing snapshotted %s", none)
	}
}
