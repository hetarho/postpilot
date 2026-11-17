package experiment

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"
)

func observationStart(photos ...string) TestStart {
	return TestStart{UserID: "owner", RequestKey: "request", Factor: FactorModel, ModelStage: StageObserve, Output: OutputObservation, Count: 2,
		Entrants: []TestEntrantRef{{SourceKind: "model", Model: ModelRef{ProviderID: "p", ModelID: "a"}}, {SourceKind: "model", Model: ModelRef{ProviderID: "p", ModelID: "b"}}},
		Input:    TestInput{TargetLanguage: "ko", PhotoIDs: photos}}
}

func TestObservationTestShapeComparesObserversOnPhotosAlone(t *testing.T) {
	if err := ValidateTestShape(observationStart("one", "two")); err != nil {
		t.Fatalf("a valid observation test: %v", err)
	}
	many := make([]string, TestPhotoMax+1)
	for i := range many {
		many[i] = fmt.Sprintf("photo-%d", i)
	}
	if err := ValidateTestShape(observationStart(many[:TestPhotoMax]...)); err != nil {
		t.Fatalf("the photo ceiling itself is accepted: %v", err)
	}
	cases := map[string]struct {
		edit func(*TestStart)
		want error
	}{
		"write stage":      {func(s *TestStart) { s.ModelStage = StageWrite }, ErrTestFactor},
		"setting factor":   {func(s *TestStart) { s.Factor, s.ModelStage = FactorVoice, "" }, ErrTestFactor},
		"unknown output":   {func(s *TestStart) { s.Output = "video" }, ErrTestFactor},
		"no photo":         {func(s *TestStart) { s.Input.PhotoIDs = nil }, ErrTestMaterialInvalid},
		"too many photos":  {func(s *TestStart) { s.Input.PhotoIDs = many }, ErrTestMaterialInvalid},
		"repeated photo":   {func(s *TestStart) { s.Input.PhotoIDs = []string{"one", "one"} }, ErrTestMaterialInvalid},
		"empty photo id":   {func(s *TestStart) { s.Input.PhotoIDs = []string{""} }, ErrTestMaterialInvalid},
		"source post":      {func(s *TestStart) { s.Input.SourcePostSlug = "post" }, ErrTestMaterialInvalid},
		"material text":    {func(s *TestStart) { s.Input.Material = "memo" }, ErrTestMaterialInvalid},
		"attachments":      {func(s *TestStart) { s.Input.AttachmentIDs = []string{"a"} }, ErrTestMaterialInvalid},
		"writer model":     {func(s *TestStart) { s.Input.WriteModel = ModelRef{ProviderID: "p", ModelID: "w"} }, ErrTestMaterialInvalid},
		"observer model":   {func(s *TestStart) { s.Input.ObserveModel = ModelRef{ProviderID: "p", ModelID: "o"} }, ErrTestMaterialInvalid},
		"voice":            {func(s *TestStart) { s.Input.VoiceID = "v" }, ErrTestMaterialInvalid},
		"template":         {func(s *TestStart) { s.Input.TemplateID = "t" }, ErrTestMaterialInvalid},
		"guideline slot":   {func(s *TestStart) { s.Input.GuidelineSlotID = "g" }, ErrTestMaterialInvalid},
		"template answers": {func(s *TestStart) { s.Input.TemplateAnswers = []TestAnswer{{Label: "l", Answer: "a"}} }, ErrTestMaterialInvalid},
		"quality rules":    {func(s *TestStart) { s.Input.QualityRules = []string{"composition"} }, ErrTestMaterialInvalid},
		"setting entrant": {func(s *TestStart) {
			s.Entrants[0] = TestEntrantRef{SourceKind: "setting", SettingKind: "voice", SettingID: "v", SettingRevision: "1"}
		}, ErrTestEntrant},
		"post with photo ids": {func(s *TestStart) { s.Output, s.ModelStage = OutputPost, StageWrite }, ErrTestMaterialInvalid},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			start := observationStart("one")
			start.Entrants = append([]TestEntrantRef(nil), start.Entrants...)
			tc.edit(&start)
			if err := ValidateTestShape(start); !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestUnspecifiedOutputIsAPostAndHashesAsBefore(t *testing.T) {
	post := observationStart()
	post.Output, post.ModelStage = "", StageWrite
	if post.OutputKind() != OutputPost || ValidateTestShape(post) != nil {
		t.Fatal("an unspecified output must read as a valid post test")
	}
	explicit := post
	explicit.Output = OutputPost
	if WritingTestStartFingerprint(post) != WritingTestStartFingerprint(explicit) {
		t.Fatal("a post test must hash the same whether its output is named or not")
	}
	built, err := BuildWritingTest(observationStart("one"), TestPlan{Snapshot: TestSnapshot{Common: []byte("c"), Hash: "h", PromptVersion: "v", Variants: []FrozenTestVariant{
		{Reference: observationStart().Entrants[0], Content: []byte("a"), SemanticKey: "a", Revision: "1"},
		{Reference: observationStart().Entrants[1], Content: []byte("b"), SemanticKey: "b", Revision: "1"},
	}}, Calls: []TestCall{{Ref: ModelRef{ProviderID: "p", ModelID: "a"}, Stage: StageObserve, Count: 1, PromptTokens: 1, CompletionTokens: 1}}}, time.Now())
	if err != nil || built.Output != OutputObservation {
		t.Fatalf("the built test keeps its output: %v %q", err, built.Output)
	}
}

func TestStartFingerprintCoversOutputAndPhotos(t *testing.T) {
	base := WritingTestStartFingerprint(observationStart("one", "two"))
	if base == WritingTestStartFingerprint(observationStart("one")) {
		t.Fatal("photo ids must change the fingerprint")
	}
	post := observationStart("one", "two")
	post.Output = OutputPost
	if base == WritingTestStartFingerprint(post) {
		t.Fatal("the output kind must change the fingerprint")
	}
}

func TestObservationOutputRoundTripsUnderItsOwnVersion(t *testing.T) {
	want := TestOutput{Observations: []TestOutputObservation{
		{File: "one.jpg", Scene: "국수", Mood: "따뜻함", VisibleText: "메뉴", Objects: []string{"그릇"}, PeoplePresent: true, Model: "p/a"},
		{File: "two.jpg", Scene: "입구"},
	}}
	raw, err := EncodeTestOutput(want)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(raw, []byte(`"version":2`)) || bytes.Contains(raw, []byte(`"content"`)) {
		t.Fatalf("observation output must be version 2 with no content: %s", raw)
	}
	got, err := DecodeTestOutput(raw)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip: %v\n got %#v\nwant %#v", err, got, want)
	}
	public, err := DecodeTestOutput(PublicTestOutput(raw))
	if err != nil || len(public.Observations) != 2 || public.Observations[0].Model != "" || public.Observations[0].Scene != "국수" {
		t.Fatalf("the public projection drops only the observer: %v %#v", err, public)
	}
	for name, bad := range map[string]TestOutput{
		"no observation":   {Observations: []TestOutputObservation{}},
		"repeated file":    {Observations: []TestOutputObservation{{File: "a"}, {File: "a"}}},
		"empty file":       {Observations: []TestOutputObservation{{}}},
		"carries a post":   {Observations: []TestOutputObservation{{File: "a"}}, ContentLanguage: "ko"},
		"carries a title":  {Observations: []TestOutputObservation{{File: "a"}}, Content: TestOutputContent{Title: "t"}},
		"carries a story":  {Observations: []TestOutputObservation{{File: "a"}}, Storyline: &TestOutputStoryline{}},
		"carries the noun": {Observations: []TestOutputObservation{{File: "a"}}, Nouns: []string{"n"}},
	} {
		if _, err := EncodeTestOutput(bad); !errors.Is(err, ErrTestOutputIncompatible) {
			t.Fatalf("%s: got %v", name, err)
		}
	}
	if _, err := DecodeTestOutput([]byte(`{"version":2,"observations":[{"file":"a"}],"content":{}}`)); !errors.Is(err, ErrTestOutputIncompatible) {
		t.Fatalf("an observation output with content must not decode: %v", err)
	}
}

func TestPostOutputBytesAreUnchangedByObservations(t *testing.T) {
	raw, err := EncodeTestOutput(TestOutput{ContentLanguage: "ko", Content: TestOutputContent{Title: "t", Blocks: []TestOutputBlock{{Type: "TEXT", Content: "c"}}}})
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"version":1,"content":{"title":"t","summary":"","tags":null,"blocks":[{"type":"TEXT","content":"c","level":0,"file":"","alt":"","caption":"","items":null,"files":null,"layout":""}]},"storyline":null,"nouns":null,"content_language":"ko"}`
	if string(raw) != want {
		t.Fatalf("version 1 bytes changed:\n got %s\nwant %s", raw, want)
	}
	if _, err := DecodeTestOutput([]byte(want)); err != nil {
		t.Fatal(err)
	}
}
