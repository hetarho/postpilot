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
			Voice:      VoiceRef{ID: "voice-1", Name: "기본", Deleted: true, SourceLanguage: LanguageKorean},
			TemplateID: "tmpl",
			Template: &TemplateBrief{
				Name: "하루 기록", Body: "<write>인트로를 씁니다</write>{{slot:1}}",
				Slots:     []TemplateSlot{{Kind: "place", Label: "가게"}},
				Rows:      []TemplatePhotoRow{{Count: 2, Filenames: []string{"IMG_1.jpg", "IMG_2.jpg"}}},
				Facts:     []TemplateFact{{Label: "가게 이름", Value: "을지로 노포"}},
				TitleArea: "<ask>가게 이름</ask> 다녀온 날",
			},
			Guidelines:      []string{"CCTV를 언급하지 않기"},
			UseMemory:       true,
			QualityRules:    []string{"제목에 같은 말을 되풀이하지 않는다"},
			FieldPhrases:    []string{"분위기 좋은 카페"},
			TemplateAnswers: []TemplateAnswer{{Label: "가게 이름", Text: "을지로 노포", Enabled: true}},
			Title:           "가제", Memo: "메모",
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
					{Type: BlockText, Content: "{{slot:1}}", Slot: &BlockSlot{Kind: "place", Label: "가게"}},
				},
			},
			ContentLanguage: &english, TargetLanguage: LanguageKorean, TargetLength: &target, TagCount: 7,
			Field: "cafe", QualityRuleIDs: []string{"composition"}, Published: true,
		},
		profile: Profile{
			Styleguide: "스타일", ActiveRules: "규칙", Excerpts: []string{"발췌"}, Rules: "사용자 규칙",
			EndingMaxConsecutive: 2, SourceLanguage: LanguageKorean, TargetLanguage: LanguageKorean, Portable: true,
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
			Slug: "post", UserID: "alice", Voice: VoiceRef{ID: "voice-1", Name: "기본"}, TargetLanguage: LanguageKorean,
			Images: []Image{{Filename: "IMG_1.jpg", Key: "key-1"}}, Title: "가제", Memo: "메모",
		},
	}
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
		"write_snapshot_full.golden": fullSnapshotFixture(),
		"write_snapshot_bare.golden": bareSnapshotFixture(),
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
		if golden == "write_snapshot_bare.golden" {
			reencoded = strings.Replace(want, `"TargetLength":null,`, `"TargetLength":null,"tag_count":4,`, 1)
		}
		if string(again) != reencoded {
			t.Errorf("%s does not survive a decode and re-encode:\n got %s\nwant %s", golden, again, reencoded)
		}
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
	// One of each, with every member set, so requireNoZero can prove each member is walked.
	fixture.post.Images = []Image{{Filename: "clip.mp4", Key: "key-2", Kind: AttachmentVideo, ContentType: "video/mp4", DurationMs: 4200}}
	fixture.post.Content.Blocks = []Block{{
		Type: BlockText, Content: "본문", Level: 2, File: "IMG_1.jpg", Alt: "간판", Caption: "골목 간판",
		Items: []string{"하나"}, Slot: &BlockSlot{Kind: "place", Label: "가게"},
	}}
	snapshot := fixture.snapshot()
	requireNoZero(t, "snapshot", reflect.ValueOf(snapshot))
	raw, err := encodeWriteSnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeWriteSnapshot(raw)
	if err != nil {
		t.Fatal(err)
	}
	want := snapshot
	// unfrozen: inputs to resolve, never part of the frozen input.
	want.Post.Field, want.Post.QualityRuleIDs, want.Post.Published = "", nil, false
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
	svc := NewService(posts, fakeProfiles{}, &fakeRules{}, newFakeModels(), fakeImages{}, &fakeJobs{}, 4, testReasoningPolicy, testBudget, testDeps())
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
