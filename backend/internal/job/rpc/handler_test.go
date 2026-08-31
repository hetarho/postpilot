package rpc_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/auth"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/job"
	jobrpc "github.com/postpilot/backend/internal/job/rpc"
	jobstore "github.com/postpilot/backend/internal/job/store"
	"github.com/postpilot/backend/internal/platform/db"
)

func newHandler(t *testing.T) (*jobrpc.Handler, *job.Queue) {
	t.Helper()
	handle, err := db.Open(filepath.Join(t.TempDir(), "rpc.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { handle.Close() })
	ctx := context.Background()
	if err := db.Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, id := range []string{"alice", "bob"} {
		if _, err := handle.Writer.Exec(
			"INSERT INTO users (id, password_hash, created_at) VALUES (?, 'hash', ?)", id, now); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := handle.Writer.Exec(
		"INSERT INTO voices (id, user_id, name, is_default, created_at, updated_at) VALUES ('voice-alice', 'alice', '기본 말투', 1, ?, ?)", now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.Exec(
		"INSERT INTO posts (slug, user_id, voice_id, created_at, updated_at) VALUES ('post-a', 'alice', 'voice-alice', ?, ?)", now, now); err != nil {
		t.Fatal(err)
	}
	queue := job.New(jobstore.New(handle.Writer, handle.Reader), time.Second)
	return jobrpc.NewHandler(queue), queue
}

func TestGetGenerationMapsOwnershipAndMissing(t *testing.T) {
	handler, queue := newHandler(t)
	slug := "post-a"
	id, err := queue.Enqueue(context.Background(), job.NewJob{
		Kind: job.KindGenerate, UserID: "alice", PostSlug: &slug, TargetLanguage: "en",
	})
	if err != nil {
		t.Fatal(err)
	}

	request := func(id, userID string) error {
		_, err := handler.GetGeneration(
			auth.WithUser(context.Background(), userID),
			connect.NewRequest(&postpilotv1.GetGenerationRequest{Id: id}),
		)
		return err
	}
	foreignErr := request(id, "bob")
	if code := connect.CodeOf(foreignErr); code != connect.CodePermissionDenied {
		t.Errorf("foreign code = %s", code)
	}
	if reason := jobErrorReason(t, foreignErr); reason != "JOB_FORBIDDEN" {
		t.Errorf("foreign reason = %q", reason)
	}
	missingErr := request("missing", "alice")
	if code := connect.CodeOf(missingErr); code != connect.CodeNotFound {
		t.Errorf("missing code = %s", code)
	}
	if reason := jobErrorReason(t, missingErr); reason != "JOB_NOT_FOUND" {
		t.Errorf("missing reason = %q", reason)
	}
	response, err := handler.GetGeneration(
		auth.WithUser(context.Background(), "alice"),
		connect.NewRequest(&postpilotv1.GetGenerationRequest{Id: id}),
	)
	if err != nil {
		t.Errorf("owner GetGeneration: %v", err)
	} else if got := response.Msg.GetJob().GetTargetLanguage(); got != postpilotv1.ContentLanguage_CONTENT_LANGUAGE_ENGLISH {
		t.Errorf("target language = %s, want English", got)
	}
}

func TestToProtoMapsOnlyCanonicalJobLanguages(t *testing.T) {
	for _, test := range []struct {
		name string
		tag  string
		want postpilotv1.ContentLanguage
	}{
		{name: "absent", want: postpilotv1.ContentLanguage_CONTENT_LANGUAGE_UNSPECIFIED},
		{name: "Korean", tag: "ko", want: postpilotv1.ContentLanguage_CONTENT_LANGUAGE_KOREAN},
		{name: "English", tag: "en", want: postpilotv1.ContentLanguage_CONTENT_LANGUAGE_ENGLISH},
		{name: "unknown", tag: "fr", want: postpilotv1.ContentLanguage_CONTENT_LANGUAGE_UNSPECIFIED},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := jobrpc.ToProto(&job.JobSummary{TargetLanguage: test.tag}).GetTargetLanguage()
			if got != test.want {
				t.Fatalf("language = %s, want %s", got, test.want)
			}
		})
	}
}

func TestToProtoProjectsStructuredFailureWithoutDeprecatedRawError(t *testing.T) {
	params := map[string]string{"safe": "value"}
	mapped := jobrpc.ToProto(&job.JobSummary{Failure: &job.Failure{
		Reason: "MODEL_RATE_LIMITED", Params: params, TechnicalDetail: "provider detail",
	}})
	if mapped.GetFailure().GetReason() != "MODEL_RATE_LIMITED" || mapped.GetFailure().GetParams()["safe"] != "value" ||
		mapped.GetFailure().GetTechnicalDetail() != "provider detail" || mapped.GetError() != "" {
		t.Fatalf("mapped failure = %+v", mapped)
	}
	params["safe"] = "mutated"
	if mapped.GetFailure().GetParams()["safe"] != "value" {
		t.Fatalf("proto params alias domain map: %#v", mapped.GetFailure().GetParams())
	}
}

func jobErrorReason(t *testing.T, err error) string {
	t.Helper()
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		t.Fatalf("error type = %T", err)
	}
	if len(connectErr.Details()) != 1 {
		t.Fatalf("details = %d, want 1", len(connectErr.Details()))
	}
	value, valueErr := connectErr.Details()[0].Value()
	if valueErr != nil {
		t.Fatalf("decode detail: %v", valueErr)
	}
	detail, ok := value.(*postpilotv1.AppErrorDetail)
	if !ok {
		t.Fatalf("detail type = %T", value)
	}
	if len(detail.GetParams()) != 0 {
		t.Fatalf("unexpected params = %#v", detail.GetParams())
	}
	return detail.GetReason()
}
