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
// alike (both call FilterAttachments).
func TestFilterAttachmentsRepairsPhotoGroups(t *testing.T) {
	photos := []string{"a.jpg", "b.jpg", "c.jpg"}
	many := make([]string, 0, post.PhotoGroupMax+2)
	for i := 0; i < post.PhotoGroupMax+2; i++ {
		name := fmt.Sprintf("p%02d.jpg", i)
		many = append(many, name)
	}
	group := func(layout, caption string, files ...string) Block {
		return Block{Type: BlockGallery, Files: files, Layout: layout, Alt: "alt", Caption: caption}
	}
	for name, tc := range map[string]struct {
		photos []string
		in     Block
		want   []Block
	}{
		"kept as written": {photos, group("SLIDE", "c", "b.jpg", "a.jpg"), []Block{group("SLIDE", "c", "b.jpg", "a.jpg")}},
		"unattached and repeated names dropped": {photos, group("COLLAGE", "c", "a.jpg", "x.jpg", "a.jpg", " b.jpg "),
			[]Block{group("COLLAGE", "c", "a.jpg", "b.jpg")}},
		"one photo left stands alone": {photos, group("SLIDE", "c", "a.jpg", "video.mp4"),
			[]Block{{Type: BlockImage, File: "a.jpg", Alt: "alt", Caption: "c"}}},
		"no photo left is dropped":          {photos, group("COLLAGE", "c", "x.jpg"), nil},
		"unknown layout reads as a collage": {photos, group(" grid ", "c", "a.jpg", "b.jpg"), []Block{group("COLLAGE", "c", "a.jpg", "b.jpg")}},
		"lower-case layout is read":         {photos, group("slide", "c", "a.jpg", "b.jpg"), []Block{group("SLIDE", "c", "a.jpg", "b.jpg")}},
		"absent layout reads as a collage":  {photos, group("", "c", "a.jpg", "b.jpg"), []Block{group("COLLAGE", "c", "a.jpg", "b.jpg")}},
		"overflow continues without caption": {many, group("SLIDE", "c", many...),
			[]Block{group("SLIDE", "c", many[:post.PhotoGroupMax]...), group("SLIDE", "", many[post.PhotoGroupMax:]...)}},
		"a lone surplus photo stands alone": {many, group("COLLAGE", "c", many[:post.PhotoGroupMax+1]...),
			[]Block{group("COLLAGE", "c", many[:post.PhotoGroupMax]...), {Type: BlockImage, File: many[post.PhotoGroupMax], Alt: "alt"}}},
	} {
		t.Run(name, func(t *testing.T) {
			got := FilterAttachments(PostContent{Blocks: []Block{tc.in}}, tc.photos, []string{"video.mp4"})
			if len(got.Blocks) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got.Blocks, tc.want) {
				t.Fatalf("filtered = %+v\nwant %+v", got.Blocks, tc.want)
			}
		})
	}
}

// The static rules state the group bound the post context enforces, in both languages, and the
// write and revise prompts both carry the format (GEN-14, GEN-77).
func TestTheGroupRuleStatesThePostBound(t *testing.T) {
	if !strings.Contains(koreanGalleryRule, fmt.Sprintf("2~%d장", post.PhotoGroupMax)) {
		t.Fatalf("Korean rule does not state 2~%d: %s", post.PhotoGroupMax, koreanGalleryRule)
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
