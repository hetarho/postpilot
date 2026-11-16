package db

import (
	"github.com/pressly/goose/v3"
	"io/fs"
	"testing"
)

// CLIP-209: a project keeps the upload text of the channel it was on; the other
// channel's text and unconsumed quote go, and the stored columns are dropped.
func TestMigration0168FoldsUploadTextsAndDropsTheClipChannel(t *testing.T) {
	h := openTemp(t)
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, h.Writer, sub, goose.WithLogger(goose.NopLogger()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = provider.UpTo(t.Context(), 167); err != nil {
		t.Fatal(err)
	}
	const stamp = "2026-10-01T00:00:00Z"
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := h.Writer.Exec(query, args...); err != nil {
			t.Fatal(query, err)
		}
	}
	exec(`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash',?)`, stamp)
	for _, p := range []struct{ id, channel string }{{"insta", "instagram_clip"}, {"moved", "instagram_clip"}, {"naver", "naver_clip"}} {
		exec(`INSERT INTO clip_projects(id,user_id,title,ratio,target_duration_ms,created_at,updated_at,channel) VALUES(?,'alice','keep','vertical',15000,?,?,?)`, p.id, stamp, stamp, p.channel)
	}
	exec(`UPDATE clip_projects SET result_id='result',result_key='result',result_content_type='video/mp4',result_bytes=1,result_duration_ms=15000,result_created_at=?,edit_plan_revision=1,rendered_plan_revision=1,finalized_at=?,finalized_plan_revision=1,finalized_result_key='result',source_access_revoked_at=?,published_url='https://www.instagram.com/reel/kept',published_at=? WHERE id='insta'`, stamp, stamp, stamp, stamp)
	text := func(project, channel, body string) {
		exec(`INSERT INTO clip_upload_texts(project_id,user_id,channel,body,tags,edited,updated_at,revision,last_job_id) VALUES(?,'alice',?,?,'["tag"]',1,?,3,'job')`, project, channel, body, stamp)
	}
	quote := func(id, project, channel string, consumed any) {
		exec(`INSERT INTO clip_upload_text_quotes(id,user_id,project_id,channel,expected_revision,input_digest,pricing_json,max_credits,expires_at,consumed_job_id) VALUES(?,'alice',?,?,3,'digest','{}',1,?,?)`, id, project, channel, stamp, consumed)
	}
	text("insta", "instagram_clip", "insta own")
	quote("insta-pending", "insta", "instagram_clip", nil)
	text("moved", "naver_clip", "moved other")
	text("moved", "instagram_clip", "moved own")
	quote("moved-other-pending", "moved", "naver_clip", nil)
	quote("moved-own-pending", "moved", "instagram_clip", nil)
	quote("moved-other-consumed", "moved", "naver_clip", "consumed-job")
	text("naver", "naver_clip", "naver own")

	if _, err = provider.UpTo(t.Context(), 168); err != nil {
		t.Fatal(err)
	}
	for project, want := range map[string]string{"insta": "insta own", "moved": "moved own", "naver": "naver own"} {
		var body, tags string
		var edited, revision int
		if err = h.Reader.QueryRow(`SELECT body,tags,edited,revision FROM clip_upload_texts WHERE project_id=?`, project).Scan(&body, &tags, &edited, &revision); err != nil {
			t.Fatal(project, err)
		}
		if body != want || tags != `["tag"]` || edited != 1 || revision != 3 {
			t.Fatal(project, body, tags, edited, revision)
		}
	}
	var texts int
	if err = h.Reader.QueryRow(`SELECT COUNT(*) FROM clip_upload_texts`).Scan(&texts); err != nil || texts != 3 {
		t.Fatal("other-channel text survived", texts, err)
	}
	rows, err := h.Reader.Query(`SELECT id FROM clip_upload_text_quotes ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	var quotes []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		quotes = append(quotes, id)
	}
	rows.Close()
	if len(quotes) != 3 || quotes[0] != "insta-pending" || quotes[1] != "moved-other-consumed" || quotes[2] != "moved-own-pending" {
		t.Fatal("quotes", quotes)
	}
	var url, title string
	if err = h.Reader.QueryRow(`SELECT published_url,title FROM clip_projects WHERE id='insta'`).Scan(&url, &title); err != nil || url != "https://www.instagram.com/reel/kept" || title != "keep" {
		t.Fatal("posted URL changed", url, title, err)
	}
	if _, err = h.Reader.Exec(`SELECT channel FROM clip_projects`); err == nil {
		t.Fatal("clip_projects.channel survived")
	}
	// One text per project; one unconsumed quote per project; the job-owner trigger stands.
	if _, err = h.Writer.Exec(`INSERT INTO clip_upload_texts(project_id,user_id,body,updated_at) VALUES('naver','alice','second',?)`, stamp); err == nil {
		t.Fatal("a second text was accepted")
	}
	if _, err = h.Writer.Exec(`INSERT INTO clip_upload_text_quotes(id,user_id,project_id,expected_revision,input_digest,pricing_json,max_credits,expires_at) VALUES('second','alice','moved',3,'d','{}',1,?)`, stamp); err == nil {
		t.Fatal("a second unconsumed quote was accepted")
	}
	if _, err = h.Writer.Exec(`UPDATE clip_upload_text_quotes SET consumed_job_id='missing' WHERE id='insta-pending'`); err == nil {
		t.Fatal("job-owner trigger missing")
	}

	if _, err = provider.DownTo(t.Context(), 167); err != nil {
		t.Fatal(err)
	}
	var channel string
	if err = h.Reader.QueryRow(`SELECT channel FROM clip_projects WHERE id='moved'`).Scan(&channel); err != nil || channel != "naver_clip" {
		t.Fatal(channel, err)
	}
	if err = h.Reader.QueryRow(`SELECT channel FROM clip_upload_texts WHERE project_id='moved'`).Scan(&channel); err != nil || channel != "naver_clip" {
		t.Fatal(channel, err)
	}
	if _, err = h.Writer.Exec(`UPDATE clip_upload_text_quotes SET consumed_job_id='missing' WHERE id='insta-pending'`); err == nil {
		t.Fatal("job-owner trigger missing after down")
	}
}
