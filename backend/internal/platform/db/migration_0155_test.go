package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/post"
	poststore "github.com/postpilot/backend/internal/post/store"
	"github.com/pressly/goose/v3"
)

// This is the byte shape written by the publication DTO before origin sidecars.
// In particular, a populated old Storyline has no Origins member at all.
const legacyOriginPublication = `{"UserID":"alice","TestID":"legacy","WinnerID":"champion","RequestKey":"","PostSlug":"p","AssignmentsHash":"legacy-assignment","InputRevision":0,"ContentRevision":0,"Content":{"Title":"Winner","Summary":"","Tags":null,"Blocks":[{"Type":"TEXT","Content":"Frozen winner","Level":0,"File":"","Alt":"","Caption":"","Items":null,"Files":null,"Layout":""}]},"Baseline":{"Title":"Winner","Summary":"","Tags":null,"Blocks":[{"Type":"TEXT","Content":"Frozen winner","Level":0,"File":"","Alt":"","Caption":"","Items":null,"Files":null,"Layout":""}]},"ContentLanguage":"ko","Storyline":null,"Nouns":["winner"]}`

type replayOnlyOriginGuard struct{}

func (replayOnlyOriginGuard) HasOrdinaryWrite(context.Context, *sql.Tx, string, string) (bool, error) {
	return false, errors.New("a historical receipt must return before live publication guards")
}

func TestMigration0155KeepsLegacyCanonicalAndReceiptsThroughLostResponse(t *testing.T) {
	for _, populated := range []bool{false, true} {
		t.Run(map[bool]string{false: "nil-plan", true: "populated-plan"}[populated], func(t *testing.T) {
			handle := openTemp(t)
			ctx := t.Context()
			sub, err := fs.Sub(migrationsFS, "migrations")
			if err != nil {
				t.Fatal(err)
			}
			provider, err := goose.NewProvider(goose.DialectSQLite3, handle.Writer, sub, goose.WithLogger(goose.NopLogger()))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := provider.UpTo(ctx, 154); err != nil {
				t.Fatal(err)
			}
			const stamp = "2026-10-07T00:00:00Z"
			const canonical = `{"title":"Winner","blocks":[{"type":"TEXT","content":"Frozen winner"}]}`
			var storedPlan any
			fixture := legacyOriginPublication
			if populated {
				fixture = strings.Replace(fixture, `"Storyline":null`, `"Storyline":{"Paragraphs":[{"Text":"Frozen plan","Files":null}],"EditedByHand":false,"MadeWith":null}`, 1)
				storedPlan = `{"paragraphs":[{"text":"Frozen plan","files":[]}],"edited_by_hand":false,"made_with":[]}`
			}
			mustExec(t, handle.Writer, "INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash',?)", stamp)
			mustExec(t, handle.Writer, "INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES('voice','alice','Legacy',1,?,?)", stamp, stamp)
			mustExec(t, handle.Writer, `INSERT INTO posts(slug,user_id,voice_id,status,content,machine_baseline,content_revision,machine_baseline_revision,content_language,storyline,created_at,updated_at) VALUES('p','alice','voice','review',?,?,1,1,'ko',?,?,?)`, canonical, canonical, storedPlan, stamp, stamp)
			receipt := post.TestOutputReceipt{UserID: "alice", TestID: "legacy", WinnerID: "champion", Action: "apply_output", RequestKey: "original-request", TargetID: "p", ResultingRevision: 1}
			encoded, err := json.Marshal(receipt)
			if err != nil {
				t.Fatal(err)
			}
			digest := sha256.Sum256([]byte(fixture))
			mustExec(t, handle.Writer, `INSERT INTO post_test_publications(user_id,test_id,winner_candidate_id,action,request_key,fingerprint,post_slug,resulting_content_revision,receipt,created_at) VALUES('alice','legacy','champion','apply_output','original-request',?,'p',1,?,?)`, hex.EncodeToString(digest[:]), string(encoded), stamp)
			if err := Migrate(ctx, handle.Writer); err != nil {
				t.Fatal(err)
			}
			var content, baseline string
			var contentOrigins, planOrigins sql.NullString
			if err := handle.Reader.QueryRow("SELECT content,machine_baseline,content_origins,storyline_origins FROM posts WHERE slug='p'").Scan(&content, &baseline, &contentOrigins, &planOrigins); err != nil || content != canonical || baseline != canonical || contentOrigins.Valid || planOrigins.Valid {
				t.Fatalf("upgrade inferred origins or changed canonical: %q %q %+v %+v %v", content, baseline, contentOrigins, planOrigins, err)
			}
			posts := poststore.New(handle.Writer, handle.Reader)
			before, err := posts.GetPost(ctx, "p")
			if err != nil || before.ContentOrigins != nil || (before.Storyline != nil && before.Storyline.Origins != nil) {
				t.Fatalf("legacy read: %+v %v", before, err)
			}
			manual := post.PostContent{Title: "Later owner edit", Blocks: []post.Block{{Type: post.BlockText, Content: "Keep this later edit"}}}
			if changed, err := posts.SaveContent(ctx, "p", "alice", manual, 1, time.Now()); err != nil || !changed {
				t.Fatal(changed, err)
			}
			var in post.TestOutputPublication
			if err := json.Unmarshal([]byte(fixture), &in); err != nil {
				t.Fatal(err)
			}
			in.RequestKey = "lost-request-key"
			publications := poststore.NewTestResultStore(handle.Writer, handle.Reader, replayOnlyOriginGuard{})
			if replay, err := publications.ApplyTestResult(ctx, in, time.Now()); err != nil || replay != receipt {
				t.Fatalf("old receipt replay = %+v, %v", replay, err)
			}
			after, err := posts.GetPost(ctx, "p")
			if err != nil || after.ContentRevision != 2 || after.MachineBaselineRevision != 1 || !reflect.DeepEqual(*after.Content, manual) {
				t.Fatalf("old receipt replaced manual edit: %+v, %v", after, err)
			}
			if changed, err := posts.DeletePost(ctx, "p", "alice"); err != nil || !changed {
				t.Fatal(changed, err)
			}
			if replay, err := publications.ApplyTestResult(ctx, in, time.Now()); err != nil || replay != receipt {
				t.Fatalf("deleted target replay = %+v, %v", replay, err)
			}
			mustExec(t, handle.Writer, "DELETE FROM users WHERE id='alice'")
			if _, err := publications.ApplyTestResult(ctx, in, time.Now()); !errors.Is(err, post.ErrNotFound) {
				t.Fatalf("deleted owner restored publication: %v", err)
			}
			if _, found, err := publications.ReadTestPublicationReceipt(ctx, "alice", "legacy", "champion"); err != nil || found {
				t.Fatalf("deleted owner retained receipt: %v %v", found, err)
			}
		})
	}
}
