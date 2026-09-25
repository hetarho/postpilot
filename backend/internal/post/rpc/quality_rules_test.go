package rpc

import (
	"context"
	"google.golang.org/protobuf/proto"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/auth"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/post"
	poststore "github.com/postpilot/backend/internal/post/store"
)

// ARCH-3: every generated metric but UNSPECIFIED is one of post's four ids and back, and the four
// are the canonical list in enum order.
func TestQualityMetricWalksTheGeneratedEnum(t *testing.T) {
	seen := map[string]bool{}
	var ids []string
	for value := range postpilotv1.QualityMetric_name {
		metric := postpilotv1.QualityMetric(value)
		id, ok := qualityRuleFromProto(metric)
		if metric == postpilotv1.QualityMetric_QUALITY_METRIC_UNSPECIFIED {
			if ok || id != "" {
				t.Fatalf("UNSPECIFIED mapped to %q", id)
			}
			continue
		}
		if !ok || seen[id] {
			t.Fatalf("%s mapped to %q (ok=%v, seen=%v)", metric, id, ok, seen[id])
		}
		seen[id] = true
		ids = append(ids, id)
		if back, ok := qualityRuleToProto(id); !ok || back != metric {
			t.Fatalf("%q came back as %s", id, back)
		}
	}
	if _, ok := qualityRuleFromProto(postpilotv1.QualityMetric(99)); ok {
		t.Fatal("an out-of-range metric was mapped")
	}
	if _, ok := qualityRuleToProto("score"); ok {
		t.Fatal("an unknown id was mapped")
	}
	canonical, err := post.NormalizeQualityRules(ids)
	if err != nil || len(canonical) != len(ids) || len(canonical) != 4 {
		t.Fatalf("canonical = %q, %v", canonical, err)
	}
	for i, id := range canonical {
		if metric, _ := qualityRuleToProto(id); int32(metric) != int32(i+1) {
			t.Fatalf("canonical[%d] = %q is %s, out of enum order", i, id, metric)
		}
	}
}

// The post service's collaborators, each answering "nothing here".
type (
	noBlobs       struct{}
	noJobs        struct{}
	noExperiments struct{}
	noPurge       struct{}
	noDetach      struct{}
	knownFields   struct{}
	aliceVoice    struct{}
)

func (noBlobs) PresignPut(context.Context, string, string, time.Duration) (string, error) {
	return "", nil
}
func (noBlobs) PresignGet(context.Context, string, time.Duration) (string, error) { return "", nil }
func (noBlobs) Head(context.Context, string) (post.ObjectHead, error) {
	return post.ObjectHead{}, post.ErrObjectNotFound
}
func (noBlobs) Delete(context.Context, string) error                          { return nil }
func (noBlobs) List(context.Context, string) ([]post.Object, error)           { return nil, nil }
func (noJobs) ActiveForPost(context.Context, string) (*post.ActiveJob, error) { return nil, nil }
func (noExperiments) PendingForPost(context.Context, string, string) (string, error) {
	return "", nil
}
func (noPurge) PurgePost(context.Context, string, string) error   { return nil }
func (noDetach) DetachPost(context.Context, string, string) error { return nil }
func (knownFields) Known(id string) bool                          { return id == "restaurant" || id == "cafe" }
func (aliceVoice) Voices(_ context.Context, userID string) ([]post.VoiceRef, error) {
	if userID != "alice" {
		return nil, nil
	}
	return []post.VoiceRef{{ID: "voice-alice", Name: "기본", SourceLanguage: post.LanguageKorean}}, nil
}

// rpcService is a real post service over SQLite: the handler holds the concrete service, so a
// presence rule on the wire is provable only through one.
func rpcService(t *testing.T) *Handler {
	t.Helper()
	h, _ := rpcServiceWith(t)
	return h
}

// rpcServiceWith also hands back the service, for a test that seeds what no RPC writes — a
// generation's replacement candidates, say.
func rpcServiceWith(t *testing.T) (*Handler, *post.Service) {
	t.Helper()
	handle, err := db.Open(filepath.Join(t.TempDir(), "post.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { handle.Close() })
	if err := db.Migrate(context.Background(), handle.Writer); err != nil {
		t.Fatal(err)
	}
	stamp := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
	for _, statement := range []string{
		`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash','` + stamp + `')`,
		`INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES('voice-alice','alice','기본',1,'` + stamp + `','` + stamp + `')`,
	} {
		if _, err := handle.Writer.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	svc := post.NewService(poststore.New(handle.Writer, handle.Reader), noBlobs{}, post.Limits{}, post.Deps{
		Jobs: noJobs{}, Voices: aliceVoice{}, Experiments: noExperiments{}, ContentPurger: noPurge{},
		CandidateLinks: noDetach{}, MemoryLinks: noDetach{}, Fields: knownFields{},
	})
	return NewHandler(svc), svc
}

func draftRequest(slug, title string, field *postpilotv1.BlogField) *connect.Request[postpilotv1.SavePostDraftRequest] {
	voice, language := "voice-alice", postpilotv1.ContentLanguage_CONTENT_LANGUAGE_KOREAN
	request := &postpilotv1.SavePostDraftRequest{Slug: slug, Title: title, Field: field}
	if slug == "" {
		request.VoiceId, request.TargetLanguage = &voice, &language
	}
	return connect.NewRequest(request)
}

func fieldOf(value postpilotv1.BlogField) *postpilotv1.BlogField { return &value }

func TestSavePostDraftFieldPresence(t *testing.T) {
	h := rpcService(t)
	ctx := auth.WithUser(context.Background(), "alice")

	created, err := h.SavePostDraft(ctx, draftRequest("", "제주", fieldOf(postpilotv1.BlogField_BLOG_FIELD_RESTAURANT)))
	if err != nil || created.Msg.GetPost().GetField() != postpilotv1.BlogField_BLOG_FIELD_RESTAURANT {
		t.Fatalf("create = %v, %v", created, err)
	}
	slug := created.Msg.GetPost().GetSlug()
	kept, err := h.SavePostDraft(ctx, draftRequest(slug, "제주 2", nil))
	if err != nil || kept.Msg.GetPost().GetField() != postpilotv1.BlogField_BLOG_FIELD_RESTAURANT {
		t.Fatalf("absent = %v, %v", kept.Msg.GetPost().GetField(), err)
	}
	cleared, err := h.SavePostDraft(ctx, draftRequest(slug, "제주 2", fieldOf(postpilotv1.BlogField_BLOG_FIELD_UNSPECIFIED)))
	if err != nil || cleared.Msg.GetPost().GetField() != postpilotv1.BlogField_BLOG_FIELD_UNSPECIFIED {
		t.Fatalf("UNSPECIFIED = %v, %v", cleared.Msg.GetPost().GetField(), err)
	}

	// An out-of-range number and a value the product does not list are both not found, and the
	// update applies nothing else in the request.
	for name, field := range map[string]postpilotv1.BlogField{
		"out of range":    postpilotv1.BlogField(99),
		"not in the list": postpilotv1.BlogField_BLOG_FIELD_PETS,
	} {
		_, err := h.SavePostDraft(ctx, draftRequest(slug, "바뀐 제목", fieldOf(field)))
		if connect.CodeOf(err) != connect.CodeNotFound || postAppErrorDetail(t, err).GetReason() != "POST_FIELD_NOT_FOUND" {
			t.Fatalf("%s = %v", name, err)
		}
	}
	got, err := h.GetPost(ctx, connect.NewRequest(&postpilotv1.GetPostRequest{Slug: slug}))
	if err != nil || got.Msg.GetPost().GetTitle() != "제주 2" {
		t.Fatalf("a refused update applied its title: %q, %v", got.Msg.GetPost().GetTitle(), err)
	}
	// Nor is a post minted by a refused create.
	if _, err := h.SavePostDraft(ctx, draftRequest("", "새 글", fieldOf(postpilotv1.BlogField(99)))); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("a create with an out-of-range 분야 = %v", err)
	}
	list, err := h.ListPosts(ctx, connect.NewRequest(&postpilotv1.ListPostsRequest{}))
	if err != nil || len(list.Msg.GetPosts()) != 1 {
		t.Fatalf("posts after a refused create = %d, %v", len(list.Msg.GetPosts()), err)
	}
}

// POST-89: the brief's run options are one whole set. Every member but target_length is
// required, a request missing one or carrying a bad value is refused with nothing written, and
// an absent length is natural length.
func TestSavePostGenerationOptionsIsAWholeSet(t *testing.T) {
	h := rpcService(t)
	ctx := auth.WithUser(context.Background(), "alice")
	created, err := h.SavePostDraft(ctx, draftRequest("", "제주", nil))
	if err != nil {
		t.Fatal(err)
	}
	slug := created.Msg.GetPost().GetSlug()
	i32 := func(n int32) *int32 { return &n }
	yes, no := true, false
	cafe, none := postpilotv1.BlogField_BLOG_FIELD_CAFE, postpilotv1.BlogField_BLOG_FIELD_UNSPECIFIED
	ticks := func(metrics ...postpilotv1.QualityMetric) *postpilotv1.QualityRuleTicks {
		return &postpilotv1.QualityRuleTicks{Metrics: metrics}
	}
	save := func(req *postpilotv1.SavePostGenerationOptionsRequest) error {
		req.Slug = slug
		_, err := h.SavePostGenerationOptions(ctx, connect.NewRequest(req))
		return err
	}
	read := func() *postpilotv1.Post {
		got, err := h.GetPost(ctx, connect.NewRequest(&postpilotv1.GetPostRequest{Slug: slug}))
		if err != nil {
			t.Fatal(err)
		}
		return got.Msg.GetPost()
	}

	if err := save(&postpilotv1.SavePostGenerationOptionsRequest{
		TargetLength: i32(1500), TagCount: i32(7), UseMemory: &yes, Field: &cafe,
		QualityRules: ticks(postpilotv1.QualityMetric_QUALITY_METRIC_COMPOSITION, postpilotv1.QualityMetric_QUALITY_METRIC_TITLE_SATURATION, postpilotv1.QualityMetric_QUALITY_METRIC_COMPOSITION),
	}); err != nil {
		t.Fatal(err)
	}
	full := read()
	if want := []postpilotv1.QualityMetric{postpilotv1.QualityMetric_QUALITY_METRIC_TITLE_SATURATION, postpilotv1.QualityMetric_QUALITY_METRIC_COMPOSITION}; full.GetTargetLength() != 1500 || full.GetTagCount() != 7 || !full.GetUseMemory() ||
		!reflect.DeepEqual(full.GetQualityRules(), want) || full.GetField() != cafe {
		t.Fatalf("saved = %+v", full)
	}

	refused := func(name string, req *postpilotv1.SavePostGenerationOptionsRequest, code connect.Code, reason string) {
		t.Helper()
		before := read()
		err := save(req)
		if connect.CodeOf(err) != code || postAppErrorDetail(t, err).GetReason() != reason {
			t.Errorf("%s = %v, want %v %s", name, err, code, reason)
		}
		if after := read(); !proto.Equal(after, before) {
			t.Errorf("%s: a refused save changed the post: %+v", name, after)
		}
	}
	changed := func() *postpilotv1.SavePostGenerationOptionsRequest {
		return &postpilotv1.SavePostGenerationOptionsRequest{TargetLength: i32(900), TagCount: i32(3), UseMemory: &no, QualityRules: ticks(), Field: &none}
	}
	for name, drop := range map[string]func(*postpilotv1.SavePostGenerationOptionsRequest){
		"no tag_count":     func(r *postpilotv1.SavePostGenerationOptionsRequest) { r.TagCount = nil },
		"no use_memory":    func(r *postpilotv1.SavePostGenerationOptionsRequest) { r.UseMemory = nil },
		"no quality_rules": func(r *postpilotv1.SavePostGenerationOptionsRequest) { r.QualityRules = nil },
		"no field":         func(r *postpilotv1.SavePostGenerationOptionsRequest) { r.Field = nil },
		"target_length 0":  func(r *postpilotv1.SavePostGenerationOptionsRequest) { r.TargetLength = i32(0) },
	} {
		req := changed()
		drop(req)
		refused(name, req, connect.CodeInvalidArgument, "POST_CONTENT_INVALID")
	}
	for name, field := range map[string]postpilotv1.BlogField{"a number no build names": postpilotv1.BlogField(99), "a 분야 the product does not list": postpilotv1.BlogField_BLOG_FIELD_PETS} {
		req := changed()
		req.Field = &field
		refused(name, req, connect.CodeNotFound, "POST_FIELD_NOT_FOUND")
	}
	for name, metric := range map[string]postpilotv1.QualityMetric{"tick UNSPECIFIED": postpilotv1.QualityMetric_QUALITY_METRIC_UNSPECIFIED, "tick 99": postpilotv1.QualityMetric(99)} {
		req := changed()
		req.QualityRules = ticks(postpilotv1.QualityMetric_QUALITY_METRIC_COMPOSITION, metric)
		refused(name, req, connect.CodeInvalidArgument, "POST_QUALITY_RULE_INVALID")
	}

	// No target_length is natural length, no metrics clears the ticks, UNSPECIFIED is 없음.
	if err := save(&postpilotv1.SavePostGenerationOptionsRequest{TagCount: i32(4), UseMemory: &no, QualityRules: ticks(), Field: &none}); err != nil {
		t.Fatal(err)
	}
	if cleared := read(); cleared.TargetLength != nil || len(cleared.GetQualityRules()) != 0 || cleared.GetField() != none || cleared.GetTagCount() != 4 {
		t.Fatalf("cleared = %+v", cleared)
	}
}
