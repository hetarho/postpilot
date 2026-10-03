package generation

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"testing"
)

// filledGenerationOptions sets every member of the frozen options, down to every brief and
// observation member, so a member added later fails requireNoZero until the fixture fills it.
func filledGenerationOptions() generationOptions {
	target := 1500
	files := []string{"IMG_1.jpg"}
	return generationOptions{
		TargetLanguage: LanguageEnglish,
		TargetLength:   &target,
		TagCount:       7,
		ObserveFiles:   &files,
		Observations: []Observation{{
			File: "IMG_1.jpg", Scene: "골목", Mood: "차분함", VisibleText: "영업중", Objects: []string{"간판"},
			PeoplePresent: true, Model: "p/observer", Events: []string{"문이 열린다"}, Speech: "어서 오세요", Rotation: 90,
		}},
		WriteNativeEffort: true,
		FollowStoryline:   []StorylineParagraph{{Text: "골목을 보여줍니다.", Files: []string{"IMG_1.jpg"}}},
		writeMaterial: writeMaterial{Template: &TemplateBrief{
			Name: "하루 기록", Body: "<write>인트로</write>네이버 지도",
			Facts:     []TemplateFact{{Label: "가게 이름", Value: "을지로 노포"}},
			TitleArea: "<ask>가게 이름</ask> 다녀온 날",
		}, Guidelines: []string{"CCTV를 언급하지 않기"}, DefaultGuidelines: []string{"메모의 이름으로 쓰세요"}, Memories: []string{"매운 음식을 못 먹는다"}, QualityRules: []string{"제목에 같은 말을 되풀이하지 않는다"}},
	}
}

// requireNoZero fails on any zero member reachable from v, naming its path: a zero string,
// bool or number, a nil pointer, or an empty slice. Slices are walked element by element and
// unexported fields are walked too.
func requireNoZero(t *testing.T, path string, v reflect.Value) {
	t.Helper()
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			t.Errorf("%s is nil", path)
			return
		}
		requireNoZero(t, path, v.Elem())
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			requireNoZero(t, path+"."+v.Type().Field(i).Name, v.Field(i))
		}
	case reflect.Slice:
		if v.Len() == 0 {
			t.Errorf("%s is empty", path)
			return
		}
		for i := 0; i < v.Len(); i++ {
			requireNoZero(t, path+"["+strconv.Itoa(i)+"]", v.Index(i))
		}
	default:
		if v.IsZero() {
			t.Errorf("%s is zero", path)
		}
	}
}

// Every frozen member survives the job row: what Start encodes is what Generate decodes.
func TestEveryGenerationOptionRoundTrips(t *testing.T) {
	fixture := filledGenerationOptions()
	requireNoZero(t, "options", reflect.ValueOf(fixture))
	raw, err := encodeGenerationPayload(fixture)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeGenerationPayload(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, fixture) {
		t.Fatalf("round trip lost a member:\n got %+v\nwant %+v", decoded, fixture)
	}

	legacy, err := decodeGenerationPayload([]byte(`{"target_language":"ko","observe_files":null}`))
	if err != nil {
		t.Fatal(err)
	}
	if legacy.WriteNativeEffort {
		t.Fatal("a payload queued before write_native_effort decoded with headroom")
	}
}

// Every frozen member reaches the run's post input under the same name; the observation
// decision stays with the observe stage and never overwrites the stored snapshot.
func TestEveryFrozenOptionReachesTheRun(t *testing.T) {
	fixture := filledGenerationOptions()
	got := reflect.ValueOf(fixture.onto(PostInput{}))
	options := reflect.ValueOf(fixture)
	// VisibleFields reaches the members writeMaterial promotes; the embedded struct itself is
	// not a post member, its five fields are.
	for _, field := range reflect.VisibleFields(options.Type()) {
		name := field.Name
		if field.Anonymous || name == "ObserveFiles" || name == "Observations" {
			continue
		}
		member := got.FieldByName(name)
		if !member.IsValid() {
			t.Errorf("PostInput has no %s: add it to PostInput and onto", name)
			continue
		}
		want := options.FieldByIndex(field.Index).Interface()
		if !reflect.DeepEqual(member.Interface(), want) {
			t.Errorf("%s = %v, want %v: add it to onto", name, member.Interface(), want)
		}
	}
	if fixture.onto(PostInput{}).Observations != nil {
		t.Fatal("the frozen observations overwrote the post's stored snapshot")
	}
}

// One Start whose every frozen member has something to freeze: a member Start forgets to set
// is zero in the payload, which is exactly how the native-effort flag went missing (review F1).
func TestStartFreezesEveryOption(t *testing.T) {
	observation := Observation{
		File: "IMG_1.jpg", Scene: "골목", Mood: "차분함", VisibleText: "영업중", Objects: []string{"간판"},
		PeoplePresent: true, Model: observeRef.String(), Events: []string{"문이 열린다"}, Speech: "어서 오세요", Rotation: 90,
	}
	posts := &fakePosts{input: PostInput{
		Slug: "post", UserID: "alice", Voice: liveVoice, TargetLanguage: LanguageKorean, TagCount: 7,
		TemplateID: "tmpl", UseMemory: true, QualityRuleIDs: []string{"composition"}, Field: "cafe",
		Images:       []Image{{Filename: "IMG_1.jpg", Key: "key-1"}, {Filename: "IMG_2.jpg", Key: "key-2"}},
		Observations: []Observation{observation},
		// Along a storyline holding both: IMG_1 is reused, IMG_2 is observed (GEN-70).
		Storyline: &Storyline{Paragraphs: []StorylineParagraph{{Text: "골목을 보여줍니다.", Files: []string{"IMG_1.jpg", "IMG_2.jpg"}}}},
	}}
	models := newFakeModels()
	info := models.infos[writeRef]
	info.ReasoningNativeEffort = true
	models.infos[writeRef] = info
	deps := testDeps()
	deps.Templates = &fakeTemplateBriefs{brief: *filledGenerationOptions().Template}
	deps.Guidelines = &fakeGuidelines{texts: testGuidelines(), defaults: productDefaults(LanguageKorean)}
	deps.Memories = &recordingMemories{texts: testMemories()}
	deps.QualityRules = &recordingRules{answer: []string{"제목에 같은 말을 되풀이하지 않는다"}}
	jobs := &fakeJobs{id: "job"}
	svc := NewService(posts, fakeProfiles{}, models, fakeImages{}, jobs, 4, testReasoningPolicy, testBudget, deps)

	target := 1500
	if _, err := svc.Start(context.Background(), StartRequest{
		UserID: "alice", PostSlug: "post", ObserveModel: observeRef.String(), WriteModel: writeRef.String(),
		TargetLength: &target, FromStoryline: true,
	}); err != nil {
		t.Fatal(err)
	}
	requireNoZero(t, "frozen", reflect.ValueOf(jobs.frozen(t, 0)))
}

// Decode owns the legacy language rules the handler used to repeat: no language is Korean, and a
// language this build does not know is refused before the run reads anything.
func TestAPayloadDecodesItsLanguageOrRefusesIt(t *testing.T) {
	legacy, err := decodeGenerationPayload(nil)
	if err != nil || legacy.TargetLanguage != LanguageKorean || legacy.TagCount != resolveTagCount(0) {
		t.Fatalf("an empty payload decoded as %+v, %v", legacy, err)
	}
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: liveVoice}}
	svc := NewService(posts, fakeProfiles{}, newFakeModels(), fakeImages{}, &fakeJobs{}, 4, testReasoningPolicy, testBudget, testDeps())
	err = svc.Generate(context.Background(), GenerateJob{UserID: "alice", PostSlug: "post", VoiceID: liveVoice.ID, WriteModel: writeRef.String(), Payload: []byte(`{"target_language":"xx"}`)}, func(string, int, int) {})
	if !errors.Is(err, ErrLanguageRequired) {
		t.Fatalf("an unknown language ran: %v", err)
	}
	if posts.reads != 0 {
		t.Fatal("the run read the post before refusing its payload")
	}
}

// A generate payload queued while writes froze 분야 phrases decodes exactly as the same payload
// without them: the retired member is ignored, never converted (GEN-5, GEN-30).
func TestALegacyPayloadWithFieldPhrasesDecodesAsWithout(t *testing.T) {
	const current = `{"target_language":"en","tag_count":5,"guidelines":["CCTV를 언급하지 않기"],` +
		`"quality_rules":["제목에 같은 말을 되풀이하지 않는다"],"observe_files":null}`
	const legacy = `{"target_language":"en","tag_count":5,"guidelines":["CCTV를 언급하지 않기"],` +
		`"quality_rules":["제목에 같은 말을 되풀이하지 않는다"],"field_phrases":["분위기 좋은 카페"],"observe_files":null}`
	want, err := decodeGenerationPayload([]byte(current))
	if err != nil {
		t.Fatal(err)
	}
	got, err := decodeGenerationPayload([]byte(legacy))
	if err != nil {
		t.Fatalf("a payload carrying field_phrases was refused: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("legacy payload decoded as\n %+v\nwant %+v", got, want)
	}
}
