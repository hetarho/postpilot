package generation

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/post"
)

// The model writes a photo group as a GALLERY block with files and layout (GEN-77), and the parser
// carries both.
func TestParseContentReadsAPhotoGroup(t *testing.T) {
	content, err := ParseContent(`{"title":"t","summary":"s","tags":[],"blocks":[
		{"type":"GALLERY","content":"","level":0,"file":"","files":["a.jpg","b.jpg"],"layout":"SLIDE","alt":"묶음","caption":"넘겨 보기","items":[]},
		{"type":"TEXT","content":"문단","level":0,"file":"","files":[],"layout":"","alt":"","caption":"","items":[]}
	]}`, 0)
	if err != nil {
		t.Fatal(err)
	}
	group := content.Blocks[0]
	if group.Type != BlockGallery || !reflect.DeepEqual(group.Files, []string{"a.jpg", "b.jpg"}) || group.Layout != GallerySlide || group.Caption != "넘겨 보기" {
		t.Fatalf("group = %+v", group)
	}
}

// A group needs a name to be a group; a stray file on it is cleared rather than dropping its
// photos, and files/layout mean nothing on every other block (GEN-2, GEN-77).
func TestValidateBlocksHandlesPhotoGroups(t *testing.T) {
	got := ValidateBlocks([]Block{
		{Type: BlockGallery, Files: []string{"a.jpg", "b.jpg"}, Layout: "COLLAGE", File: "a.jpg", Caption: "c"},
		{Type: BlockGallery, Files: []string{" ", ""}},
		{Type: BlockGallery},
		{Type: BlockGallery, Files: []string{"a.jpg"}, Content: "문단"},
		{Type: BlockGallery, Files: []string{"a.jpg"}, Items: []string{"하나"}},
		{Type: BlockText, Content: "문단", Files: []string{"a.jpg"}, Layout: "SLIDE"},
		{Type: BlockImage, File: "c.jpg", Files: []string{}, Layout: ""},
	})
	want := []Block{
		{Type: BlockGallery, Files: []string{"a.jpg", "b.jpg"}, Layout: "COLLAGE", Caption: "c"},
		{Type: BlockText, Content: "문단"},
		{Type: BlockImage, File: "c.jpg"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("validated = %+v\nwant %+v", got, want)
	}
}

// GEN-78: the attachment filter repairs a group photo by photo, on the write and the revision
// alike (both call FilterAttachments): no placed photo is lost, no group mixes orientations or
// holds more than post.PhotoGroupMax, and every part carries a caption.
func TestFilterAttachmentsRepairsPhotoGroups(t *testing.T) {
	photos := []string{"a.jpg", "b.jpg", "c.jpg", "d.jpg", "e.jpg", "f.jpg", "g.jpg", "p.jpg", "q.jpg"}
	portrait := map[string]bool{"p.jpg": true, "q.jpg": true}
	group := func(layout, caption string, files ...string) Block {
		return Block{Type: BlockGallery, Files: files, Layout: layout, Alt: "alt", Caption: caption}
	}
	image := func(file, caption string) Block {
		return Block{Type: BlockImage, File: file, Alt: "alt", Caption: caption}
	}
	for name, tc := range map[string]struct {
		in   Block
		want []Block
	}{
		"kept as written": {group("SLIDE", "c", "b.jpg", "a.jpg"), []Block{group("SLIDE", "c", "b.jpg", "a.jpg")}},
		"unattached and repeated names dropped": {group("COLLAGE", "c", "a.jpg", "x.jpg", "a.jpg", " b.jpg "),
			[]Block{group("COLLAGE", "c", "a.jpg", "b.jpg")}},
		"one photo left stands alone":       {group("SLIDE", "c", "a.jpg", "video.mp4"), []Block{image("a.jpg", "c")}},
		"no photo left is dropped":          {group("COLLAGE", "c", "x.jpg"), nil},
		"unknown layout reads as a collage": {group(" grid ", "c", "a.jpg", "b.jpg"), []Block{group("COLLAGE", "c", "a.jpg", "b.jpg")}},
		"lower-case layout is read":         {group("slide", "c", "a.jpg", "b.jpg"), []Block{group("SLIDE", "c", "a.jpg", "b.jpg")}},
		"four split two and two, the later part captioned by the alt": {group("COLLAGE", "c", "a.jpg", "b.jpg", "c.jpg", "d.jpg"),
			[]Block{group("COLLAGE", "c", "a.jpg", "b.jpg"), group("COLLAGE", "alt", "c.jpg", "d.jpg")}},
		"seven split three, two and two": {group("SLIDE", "c", "a.jpg", "b.jpg", "c.jpg", "d.jpg", "e.jpg", "f.jpg", "g.jpg"),
			[]Block{group("SLIDE", "c", "a.jpg", "b.jpg", "c.jpg"), group("SLIDE", "alt", "d.jpg", "e.jpg"), group("SLIDE", "alt", "f.jpg", "g.jpg")}},
		"orientations split, the first photo's first": {group("COLLAGE", "c", "a.jpg", "p.jpg", "b.jpg", "q.jpg"),
			[]Block{group("COLLAGE", "c", "a.jpg", "b.jpg"), group("COLLAGE", "alt", "p.jpg", "q.jpg")}},
		"a lone photo of the other orientation stands alone": {group("COLLAGE", "c", "p.jpg", "a.jpg", "q.jpg"),
			[]Block{group("COLLAGE", "c", "p.jpg", "q.jpg"), image("a.jpg", "alt")}},
		"an empty caption takes the alt": {group("COLLAGE", " ", "a.jpg", "b.jpg"), []Block{group("COLLAGE", "alt", "a.jpg", "b.jpg")}},
		"without an alt a later part repeats the caption": {Block{Type: BlockGallery, Files: []string{"a.jpg", "b.jpg", "c.jpg", "d.jpg"}, Caption: "c"},
			[]Block{{Type: BlockGallery, Files: []string{"a.jpg", "b.jpg"}, Layout: "COLLAGE", Caption: "c"}, {Type: BlockGallery, Files: []string{"c.jpg", "d.jpg"}, Layout: "COLLAGE", Caption: "c"}}},
	} {
		t.Run(name, func(t *testing.T) {
			got := FilterAttachments(PostContent{Blocks: []Block{tc.in}}, photos, []string{"video.mp4"}, portrait)
			if len(got.Blocks) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got.Blocks, tc.want) {
				t.Fatalf("filtered = %+v\nwant %+v", got.Blocks, tc.want)
			}
		})
	}
}

// Orientation is decided once, from the dimensions on record turned by the photo's rotation —
// the owner's when they set one, else its observation's: taller than wide is portrait, a square
// photo or one with no dimensions landscape, and a video none (GEN-77, GEN-79, POST-107).
func TestPhotoPortraits(t *testing.T) {
	got := PhotoPortraits([]Image{
		{Filename: "tall.jpg", Width: 768, Height: 1024},
		{Filename: "wide.jpg", Width: 1024, Height: 768},
		{Filename: "square.jpg", Width: 800, Height: 800},
		{Filename: "unknown.jpg"},
		{Filename: "clip.mp4", Kind: AttachmentVideo, Width: 1080, Height: 1920},
		// A sideways shot the observation turns a quarter: stored wide, shown tall.
		{Filename: "turned.jpg", Width: 1024, Height: 768},
		// Half a turn changes nothing about the shape.
		{Filename: "upside.jpg", Width: 768, Height: 1024},
		// The owner's own turn stands over the observation's.
		{Filename: "owner.jpg", Width: 768, Height: 1024, Rotation: 270, RotationByOwner: true},
	}, []Observation{
		{File: "turned.jpg", Rotation: 90},
		{File: "upside.jpg", Rotation: 180},
		{File: "owner.jpg", Rotation: 0},
	})
	if !reflect.DeepEqual(got, map[string]bool{"tall.jpg": true, "turned.jpg": true, "upside.jpg": true}) {
		t.Fatalf("portraits = %v", got)
	}
}

// The observe answer's rotation is kept only as one of the four quarter turns (GEN-79).
func TestParseObservationsKeepsOnlyQuarterTurns(t *testing.T) {
	got, err := parseObservations(`{"observations":[
		{"file":"a.jpg","scene":"","mood":"","visible_text":"","objects":[],"people_present":false,"rotation":90},
		{"file":"b.jpg","scene":"","mood":"","visible_text":"","objects":[],"people_present":false,"rotation":270},
		{"file":"c.jpg","scene":"","mood":"","visible_text":"","objects":[],"people_present":false,"rotation":45},
		{"file":"d.jpg","scene":"","mood":"","visible_text":"","objects":[],"people_present":false,"rotation":-90},
		{"file":"e.jpg","scene":"","mood":"","visible_text":"","objects":[],"people_present":false}
	]}`)
	if err != nil {
		t.Fatal(err)
	}
	var turns []int
	for _, observation := range got {
		turns = append(turns, observation.Rotation)
	}
	if !reflect.DeepEqual(turns, []int{90, 270, 0, 0, 0}) {
		t.Fatalf("turns = %v", turns)
	}
	if !strings.Contains(ObservePrompt, `"rotation":0`) || !strings.Contains(ObservePrompt, "0, 90, 180, 270") {
		t.Fatal("the observe prompt does not ask for the rotation")
	}
}

// The write and revise prompts name every attached photo's orientation after the filenames, in
// post order, and a caller with no dimensions keeps its bytes (GEN-14, GEN-40).
func TestThePromptsNameEachPhotosOrientation(t *testing.T) {
	portraits := map[string]bool{"b.jpg": true}
	for _, language := range []Language{LanguageKorean, LanguageEnglish} {
		_, user := BuildWritePromptForLanguage(WritePromptInput{Language: language, Profile: goldenProfile(), Photos: []string{"a.jpg", "b.jpg"}, TagCount: 4, Portraits: portraits})
		if !strings.Contains(user, "첨부 파일명(정확히 일치해야 함): a.jpg, b.jpg\n사진 방향: a.jpg 가로, b.jpg 세로\n사진 관찰:") {
			t.Errorf("%s write user half lacks the orientation line:\n%s", language, user)
		}
		_, withVideo := BuildWritePromptForLanguage(WritePromptInput{Language: language, Profile: goldenProfile(), Photos: []string{"a.jpg", "b.jpg"}, Videos: []string{"c.mp4"}, TagCount: 4, Portraits: portraits})
		if !strings.Contains(withVideo, "첨부 사진 파일명(정확히 일치해야 함): a.jpg, b.jpg\n사진 방향: a.jpg 가로, b.jpg 세로") {
			t.Errorf("%s write with a video lacks the orientation line:\n%s", language, withVideo)
		}
		_, revise := buildRevisePrompt(language, goldenProfile(), goldenContent(), []string{"a.jpg", "b.jpg", "c.mp4"}, []string{"a.jpg", "b.jpg"}, portraits, "묶어줘", nil, 4, nil, FrozenGuidelines{})
		if !strings.Contains(revise, "[첨부 파일명]\na.jpg, b.jpg, c.mp4\n\n[사진 방향]\na.jpg 가로, b.jpg 세로\n\n[수정 요청]") {
			t.Errorf("%s revise user half lacks the orientation section:\n%s", language, revise)
		}
	}
	_, plain := BuildWritePromptForLanguage(WritePromptInput{Language: LanguageKorean, Profile: goldenProfile(), Photos: []string{"a.jpg"}, TagCount: 4})
	if strings.Contains(plain, "사진 방향") {
		t.Fatal("a caller with no dimensions grew an orientation line")
	}
}

// The static rules state the group bound the post context enforces, in both languages, and the
// write and revise prompts both carry the format (GEN-14, GEN-77).
func TestTheGroupRuleStatesThePostBound(t *testing.T) {
	if !strings.Contains(koreanGalleryRule, fmt.Sprintf("2~%d장", post.PhotoGroupMax)) || !strings.Contains(koreanGalleryRule, "사진 방향이 같은") || !strings.Contains(koreanGalleryRule, "비워 두지 말고") {
		t.Fatalf("Korean rule does not state 2~%d, one orientation and a caption: %s", post.PhotoGroupMax, koreanGalleryRule)
	}
	if !strings.Contains(englishGalleryRule, fmt.Sprintf("2 to %d", post.PhotoGroupMax)) {
		t.Fatalf("English rule does not state 2 to %d: %s", post.PhotoGroupMax, englishGalleryRule)
	}
	for _, language := range []Language{LanguageKorean, LanguageEnglish} {
		rule := map[Language]string{LanguageKorean: koreanGalleryRule, LanguageEnglish: englishGalleryRule}[language]
		write, _ := BuildWritePromptForLanguage(WritePromptInput{Language: language, Profile: goldenProfile(), Photos: []string{"IMG_1.jpg"}, TagCount: 4})
		revise, _ := BuildRevisePromptForLanguage(language, goldenProfile(), goldenContent(), []string{"IMG_1.jpg"}, "묶어줘", nil, 4, nil, FrozenGuidelines{})
		if strings.Count(write, rule) != 1 || strings.Count(revise, rule) != 1 {
			t.Errorf("%s: the group rule appears %d times in the write and %d in the revise prompt", language, strings.Count(write, rule), strings.Count(revise, rule))
		}
	}
}
