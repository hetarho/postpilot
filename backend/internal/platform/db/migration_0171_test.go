package db

import "testing"

// POST r57 (T912): 171 moves each stored post hashtag its guide text does not hold into the text
// as a #tag token on a new last line, keeps every other text byte-for-byte, and drops the list.
func TestMigration0171FoldsPostHashtagsIntoGuideText(t *testing.T) {
	h := openTemp(t)
	ctx := t.Context()
	if err := migrate(ctx, h.Writer, migrationsBefore(t, "0171_")); err != nil {
		t.Fatal(err)
	}
	const at = "2026-11-19T00:00:00Z"
	if _, err := h.Writer.Exec(`INSERT INTO users(id,password_hash,created_at) VALUES('alice','h','` + at + `')`); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		slug, text, hashtags, want string
	}{
		{"none", "주차 정보 넣기", `[]`, "주차 정보 넣기"},
		{"held", "주차 정보 넣기\n#협찬 #답십리", `["협찬","답십리"]`, "주차 정보 넣기\n#협찬 #답십리"},
		{"some", "주차 정보 넣기 #협찬", `["협찬","답십리맛집","분식"]`, "주차 정보 넣기 #협찬\n#답십리맛집 #분식"},
		{"empty-text", "", `["협찬"]`, "#협찬"},
		{"untouched-trailing", "끝에 줄바꿈\n", `[]`, "끝에 줄바꿈\n"},
	}
	for _, c := range cases {
		if _, err := h.Writer.Exec(`INSERT INTO posts(slug,user_id,created_at,updated_at,advertiser_guide,advertiser_hashtags) VALUES(?,?,?,?,?,?)`,
			c.slug, "alice", at, at, c.text, c.hashtags); err != nil {
			t.Fatal(err)
		}
	}
	if err := Migrate(ctx, h.Writer); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		var got, updated string
		if err := h.Reader.QueryRow(`SELECT advertiser_guide,updated_at FROM posts WHERE slug=?`, c.slug).Scan(&got, &updated); err != nil {
			t.Fatal(err)
		}
		if got != c.want || updated != at {
			t.Fatalf("%s: guide %q (want %q), updated_at %q", c.slug, got, c.want, updated)
		}
	}
	var column int
	if err := h.Reader.QueryRow(`SELECT count(*) FROM pragma_table_info('posts') WHERE name='advertiser_hashtags'`).Scan(&column); err != nil || column != 0 {
		t.Fatalf("the posts hashtag column survived: %d %v", column, err)
	}
	if err := h.Reader.QueryRow(`SELECT count(*) FROM pragma_table_info('clip_projects') WHERE name='advertiser_hashtags'`).Scan(&column); err != nil || column != 1 {
		t.Fatalf("the clip hashtag column is gone: %d %v", column, err)
	}
}
