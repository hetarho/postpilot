package generation

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// snapshotFixture is a write snapshot's pieces, shared by the pinned encodings and the round
// trip, so the goldens and the tests read one fixture.
type snapshotFixture struct {
	prepared, snapshotOnly bool
	language               Language
	observeModel           string
	observeFiles           *[]string
	post                   PostInput
	profile                Profile
	observations           []Observation
}

// fullSnapshotFixture sets every member a write snapshot carries today, down to every nested
// member. Memories and the post's own Observations stay nil, as SnapshotWriteInput leaves them;
// Field, QualityRuleIDs and Published are set to prove they stay out of the bytes.
func fullSnapshotFixture() snapshotFixture {
	files := []string{"IMG_1.jpg"}
	english := LanguageEnglish
	target := 1500
	return snapshotFixture{
		prepared: true, snapshotOnly: true, language: LanguageKorean, observeModel: "p/observer", observeFiles: &files,
		post: PostInput{
			Slug: "post", UserID: "alice",
			Voice:      VoiceRef{ID: "voice-1", Name: "기본", Deleted: true},
			TemplateID: "tmpl",
			Template: &TemplateBrief{
				Name: "하루 기록", Body: "<write>인트로를 씁니다</write>네이버 지도",
				Facts:     []TemplateFact{{Label: "가게 이름", Value: "을지로 노포"}},
				TitleArea: "<ask>가게 이름</ask> 다녀온 날",
			},
			Guidelines:        []string{"CCTV를 언급하지 않기"},
			DefaultGuidelines: []string{"메모의 이름으로 쓰세요"},
			UseMemory:         true,
			QualityRules:      []string{"제목에 같은 말을 되풀이하지 않는다"},
			TemplateAnswers:   []TemplateAnswer{{Label: "가게 이름", Text: "을지로 노포", Enabled: true}},
			Title:             "가제", Memo: "메모",
			Images: []Image{
				{Filename: "IMG_1.jpg", Key: "key-1", Kind: AttachmentPhoto, ContentType: "image/jpeg"},
				{Filename: "clip.mp4", Key: "key-2", Kind: AttachmentVideo, ContentType: "video/mp4", DurationMs: 4200},
			},
			Content: &PostContent{
				Title: "제목", Summary: "요약", Tags: []string{"카페", "을지로"},
				Blocks: []Block{
					{Type: BlockHeading, Content: "소제목", Level: 2},
					{Type: BlockText, Content: "본문"},
					{Type: BlockImage, File: "IMG_1.jpg", Alt: "간판", Caption: "골목 간판"},
					{Type: BlockList, Items: []string{"하나", "둘"}},
					{Type: BlockText, Content: "네이버 지도"},
				},
			},
			ContentLanguage: &english, TargetLanguage: LanguageKorean, TargetLength: &target, TagCount: 7,
			Field: "cafe", QualityRuleIDs: []string{"composition"}, Published: true,
		},
		profile: Profile{
			Styleguide: "스타일", Excerpts: []string{"발췌"},
			EndingMaxConsecutive: 2, TargetLanguage: LanguageKorean, Portable: true,
		},
		observations: []Observation{{
			File: "IMG_1.jpg", Scene: "골목", Mood: "차분함", VisibleText: "영업중", Objects: []string{"간판"},
			PeoplePresent: true, Model: "p/observer", Events: []string{"문이 열린다"}, Speech: "어서 오세요",
		}},
	}
}

// bareSnapshotFixture is the smallest snapshot: every omitempty member absent.
func bareSnapshotFixture() snapshotFixture {
	return snapshotFixture{
		language: LanguageKorean,
		post: PostInput{
			Slug: "post", UserID: "alice", Voice: VoiceRef{ID: "voice-1", Name: "기본", Made: true}, TargetLanguage: LanguageKorean,
			Images: []Image{{Filename: "IMG_1.jpg", Key: "key-1"}}, Title: "가제", Memo: "메모",
		},
	}
}

// noVoiceSnapshotFixture is the bare snapshot of a post with 말투 없음: it freezes no voice
// and no profile (MODEL-31, LANG-18).
func noVoiceSnapshotFixture() snapshotFixture {
	fixture := bareSnapshotFixture()
	fixture.post.Voice = VoiceRef{}
	fixture.profile = Profile{NoVoice: true}
	return fixture
}

func (f snapshotFixture) snapshot() writeSnapshot {
	return writeSnapshot{
		Prepared: f.prepared, TargetLanguage: f.language, ObserveModel: f.observeModel, ObserveFiles: f.observeFiles,
		Post: f.post, Profile: f.profile, Observations: f.observations, SnapshotOnly: f.snapshotOnly,
	}
}

// The wire bytes a write snapshot has always had, pinned against goldens captured before the
// snapshot got its own wire struct: every stored experiment's input hash rests on them.
func TestWriteSnapshotEncodingIsPinned(t *testing.T) {
	for golden, fixture := range map[string]snapshotFixture{
		"write_snapshot_full.golden":     fullSnapshotFixture(),
		"write_snapshot_bare.golden":     bareSnapshotFixture(),
		"write_snapshot_no_voice.golden": noVoiceSnapshotFixture(),
	} {
		want := readSnapshotGolden(t, golden)
		got, err := encodeWriteSnapshot(fixture.snapshot())
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != want {
			t.Errorf("%s moved:\n got %s\nwant %s", golden, got, want)
		}
		decoded, err := decodeWriteSnapshot([]byte(want))
		if err != nil {
			t.Fatal(err)
		}
		again, err := encodeWriteSnapshot(decoded)
		if err != nil {
			t.Fatal(err)
		}
		// The decoder's legacy normalization resolves a missing tag count to the default, exactly
		// as it did before this wire struct, so a bare snapshot's prepared bytes gain it.
		reencoded := want
		if golden != "write_snapshot_full.golden" {
			reencoded = strings.Replace(want, `"TargetLength":null,`, `"TargetLength":null,"tag_count":4,`, 1)
		}
		if string(again) != reencoded {
			t.Errorf("%s does not survive a decode and re-encode:\n got %s\nwant %s", golden, again, reencoded)
		}
	}
	// A 말투 없음 snapshot reads back as 말투 없음, with no voice to apply a winner to.
	noVoice, err := decodeWriteSnapshot([]byte(readSnapshotGolden(t, "write_snapshot_no_voice.golden")))
	if err != nil || !noVoice.Profile.NoVoice || noVoice.Post.Voice != (VoiceRef{}) {
		t.Fatalf("the 말투 없음 snapshot decoded as %+v err=%v", noVoice, err)
	}
	// Memories are the one member that now carries a value (MEM-19, GEN-18): the bytes are the
	// full golden with that member filled, and nothing else moved.
	fixture := fullSnapshotFixture()
	fixture.post.Memories = []string{"매운 음식을 못 먹는다"}
	got, err := encodeWriteSnapshot(fixture.snapshot())
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(readSnapshotGolden(t, "write_snapshot_full.golden"), `"Memories":null`, `"Memories":["매운 음식을 못 먹는다"]`, 1)
	if string(got) != want {
		t.Errorf("memories moved more than their own member:\n got %s\nwant %s", got, want)
	}
}

// Every member of a write snapshot survives its bytes, except the three the snapshot never
// freezes. A PostInput member added later fails here until it is mapped into the wire struct or
// named unfrozen — an explicit "does this re-hash every snapshot?" decision.
func TestEveryWriteSnapshotMemberRoundTrips(t *testing.T) {
	fixture := fullSnapshotFixture()
	fixture.post.Memories = []string{"매운 음식을 못 먹는다"}
	fixture.post.Observations = fixture.observations
	fixture.post.WriteNativeEffort = true
	fixture.post.Storyline = &Storyline{Paragraphs: []StorylineParagraph{{Text: "가게 앞", Files: []string{"IMG_1.jpg"}}}, MadeWith: []string{"IMG_1.jpg"}}
	fixture.post.FollowStoryline = []StorylineParagraph{{Text: "가게 앞", Files: []string{"IMG_1.jpg"}}}
	// The profile version rides the snapshot so an applied winner files under it (VOICE-29).
	fixture.profile.Version = 7
	// One of each, with every member set, so requireNoZero can prove each member is walked.
	fixture.post.Images = []Image{{Filename: "clip.mp4", Key: "key-2", Kind: AttachmentVideo, ContentType: "video/mp4", DurationMs: 4200}}
	fixture.post.Content.Blocks = []Block{{
		Type: BlockText, Content: "본문", Level: 2, File: "IMG_1.jpg", Alt: "간판", Caption: "골목 간판",
		Items: []string{"하나"},
	}}
	fixture.post.Voice.Made = true
	snapshot := fixture.snapshot()
	// NoVoice is frozen as the absence of the whole profile, which the 말투 없음 golden pins;
	// a snapshot with a profile cannot also set it.
	probe := snapshot
	probe.Profile.NoVoice = true
	requireNoZero(t, "snapshot", reflect.ValueOf(probe))
	raw, err := encodeWriteSnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeWriteSnapshot(raw)
	if err != nil {
		t.Fatal(err)
	}
	want := snapshot
	// unfrozen: inputs to resolve, never part of the frozen input. A comparison never reads the
	// storyline, stored or followed (GEN-72).
	want.Post.Field, want.Post.QualityRuleIDs, want.Post.Published = "", nil, false
	want.Post.Storyline, want.Post.FollowStoryline = nil, nil
	// Whether the voice is made is read afresh at every run's start, never frozen (GEN-23).
	want.Post.Voice.Made = false
	if !reflect.DeepEqual(decoded, want) {
		t.Fatalf("a snapshot member did not round-trip:\n got %+v\nwant %+v", decoded, want)
	}
}

// The observation experiment's narrow input keeps its bytes too.
func TestObserveSnapshotEncodingIsPinned(t *testing.T) {
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: liveVoice, Images: []Image{
		{Filename: "IMG_1.jpg", Key: "key-1", Kind: AttachmentPhoto, ContentType: "image/jpeg"},
		{Filename: "clip.mp4", Key: "key-2", Kind: AttachmentVideo, ContentType: "video/mp4", DurationMs: 4200},
	}}}
	svc := NewService(posts, fakeProfiles{}, newFakeModels(), fakeImages{}, &fakeJobs{}, 4, testReasoningPolicy, testBudget, testDeps())
	raw, err := svc.SnapshotObserveInput(context.Background(), "alice", "post")
	if err != nil {
		t.Fatal(err)
	}
	if want := readSnapshotGolden(t, "observe_snapshot.golden"); string(raw) != want {
		t.Fatalf("observe snapshot moved:\n got %s\nwant %s", raw, want)
	}
	post, err := decodeObserveSnapshot(raw)
	if err != nil || !reflect.DeepEqual(post.Images, posts.input.Images) {
		t.Fatalf("observe snapshot round trip = %+v, %v", post, err)
	}
}

func readSnapshotGolden(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSuffix(string(raw), "\n")
}

// A write snapshot frozen while writes froze 분야 phrases decodes exactly as the same snapshot
// without them: the retired member is ignored, never converted (GEN-18).
func TestALegacySnapshotWithFieldPhrasesDecodesAsWithout(t *testing.T) {
	current := readSnapshotGolden(t, "write_snapshot_full.golden")
	const rules = `"quality_rules":["제목에 같은 말을 되풀이하지 않는다"],`
	if !strings.Contains(current, rules) {
		t.Fatalf("the full golden lost its rules member: %s", current)
	}
	legacy := strings.Replace(current, rules, rules+`"field_phrases":["분위기 좋은 카페"],`, 1)
	want, err := decodeWriteSnapshot([]byte(current))
	if err != nil {
		t.Fatal(err)
	}
	got, err := decodeWriteSnapshot([]byte(legacy))
	if err != nil {
		t.Fatalf("a snapshot carrying field_phrases was refused: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("legacy snapshot decoded as\n %+v\nwant %+v", got, want)
	}
}
