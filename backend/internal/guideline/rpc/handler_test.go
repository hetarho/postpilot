package rpc

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	"github.com/postpilot/backend/internal/auth"
	authstore "github.com/postpilot/backend/internal/auth/store"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/guideline"
	guidelinestore "github.com/postpilot/backend/internal/guideline/store"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/db"
)

// A1/A2: every refusal the user can provoke has a stable code and reason, and the two
// not-found cases stay indistinguishable from unknown ids.
func TestConnectCodesAndStableReasons(t *testing.T) {
	for name, tc := range map[string]struct {
		err    error
		code   connect.Code
		reason string
	}{
		"duplicate text":     {guideline.ErrDuplicateText, connect.CodeAlreadyExists, "GUIDELINE_TEXT_TAKEN"},
		"unknown or foreign": {guideline.ErrNotFound, connect.CodeNotFound, "GUIDELINE_NOT_FOUND"},
		"foreign template":   {guideline.ErrTemplateNotFound, connect.CodeNotFound, "GUIDELINE_TEMPLATE_NOT_FOUND"},
		"blank text":         {guideline.ErrInvalidText, connect.CodeInvalidArgument, "GUIDELINE_TEXT_REQUIRED"},
		"scope shape":        {guideline.ErrScopeShape, connect.CodeInvalidArgument, "GUIDELINE_SCOPE_INVALID"},
		"text too long":      {&guideline.TextTooLongError{Chars: 301, Max: 300}, connect.CodeInvalidArgument, "GUIDELINE_TEXT_TOO_LONG"},
		"title too long":     {&guideline.TitleTooLongError{Chars: 41, Max: 40}, connect.CodeInvalidArgument, "GUIDELINE_TITLE_TOO_LONG"},
		"account cap":        {&guideline.AccountCapError{Max: 100}, connect.CodeFailedPrecondition, "GUIDELINE_LIMIT_REACHED"},
		"unknown candidate":  {guideline.ErrCandidateNotFound, connect.CodeNotFound, "GUIDELINE_CANDIDATE_NOT_FOUND"},
		"unknown 분야":         {guideline.ErrFieldNotFound, connect.CodeNotFound, "GUIDELINE_FIELD_NOT_FOUND"},
		"unknown default":    {guideline.ErrDefaultNotFound, connect.CodeNotFound, "GUIDELINE_DEFAULT_NOT_FOUND"},
	} {
		t.Run(name, func(t *testing.T) {
			// A service error arrives wrapped as often as bare, and both must keep the reason.
			wrapped := toConnectError("op", errors.Join(errors.New("private context"), tc.err))
			if connect.CodeOf(wrapped) != tc.code || appErrorDetail(t, wrapped).GetReason() != tc.reason {
				t.Fatalf("wrapped = %v, want %v %s", wrapped, tc.code, tc.reason)
			}
			mapped := toConnectError("op", tc.err)
			if connect.CodeOf(mapped) != tc.code {
				t.Fatalf("code = %v, want %v", connect.CodeOf(mapped), tc.code)
			}
			detail := appErrorDetail(t, mapped)
			if got := detail.GetReason(); got != tc.reason {
				t.Fatalf("reason = %q, want %q", got, tc.reason)
			}
			switch tc.reason {
			case "GUIDELINE_TITLE_TOO_LONG":
				if detail.GetParams()["max"] != "40" || detail.GetParams()["actual"] != "41" {
					t.Fatalf("params = %#v", detail.GetParams())
				}
			case "GUIDELINE_TEXT_TOO_LONG":
				if detail.GetParams()["max"] != "300" || detail.GetParams()["actual"] != "301" {
					t.Fatalf("params = %#v", detail.GetParams())
				}
			case "GUIDELINE_LIMIT_REACHED":
				// The message the user reads has to name the cap, so it travels as a param.
				if detail.GetParams()["max"] != "100" {
					t.Fatalf("params = %#v", detail.GetParams())
				}
			}
		})
	}

	leaky := toConnectError("list guidelines", errors.New("no such column: secret_internal_detail"))
	if connect.CodeOf(leaky) != connect.CodeInternal || strings.Contains(leaky.Error(), "secret_internal_detail") {
		t.Fatalf("internal error leaked: %v", leaky)
	}
	if detail := appErrorDetail(t, leaky); detail.GetReason() != "UNKNOWN_FAILURE" || len(detail.GetParams()) != 0 {
		t.Fatalf("internal detail = %#v", detail)
	}
}

func appErrorDetail(t *testing.T, err error) *postpilotv1.AppErrorDetail {
	t.Helper()
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		t.Fatalf("error type = %T, want *connect.Error", err)
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
	return detail
}

// A1: the account comes from the session on every procedure, and the contract gives a caller
// nowhere to claim one.
func TestEveryProcedureRequiresASessionAndNoRequestCarriesAUserID(t *testing.T) {
	handler := NewHandler(guideline.NewService(nil, knownFields{}, guideline.Limits{TextMaxChars: 1, TitleMaxChars: 40, MaxPerAccount: 1}, 1))
	anonymous := context.Background()

	if _, err := handler.ListGuidelines(anonymous, connect.NewRequest(&postpilotv1.ListGuidelinesRequest{})); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("list = %v", err)
	}
	if _, err := handler.CreateGuideline(anonymous, connect.NewRequest(&postpilotv1.CreateGuidelineRequest{})); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("create = %v", err)
	}
	if _, err := handler.UpdateGuideline(anonymous, connect.NewRequest(&postpilotv1.UpdateGuidelineRequest{})); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("update = %v", err)
	}
	if _, err := handler.DeleteGuideline(anonymous, connect.NewRequest(&postpilotv1.DeleteGuidelineRequest{})); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("delete = %v", err)
	}
	if _, err := handler.ListGuidelineCandidates(anonymous, connect.NewRequest(&postpilotv1.ListGuidelineCandidatesRequest{})); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("list candidates = %v", err)
	}
	if _, err := handler.DismissGuidelineCandidate(anonymous, connect.NewRequest(&postpilotv1.DismissGuidelineCandidateRequest{})); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("dismiss candidate = %v", err)
	}
	if _, err := handler.SetDefaultGuidelineEnabled(anonymous, connect.NewRequest(&postpilotv1.SetDefaultGuidelineEnabledRequest{})); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("set default = %v", err)
	}

	for _, message := range []proto.Message{
		&postpilotv1.ListGuidelinesRequest{}, &postpilotv1.CreateGuidelineRequest{},
		&postpilotv1.UpdateGuidelineRequest{}, &postpilotv1.DeleteGuidelineRequest{},
		&postpilotv1.ListGuidelineCandidatesRequest{}, &postpilotv1.DismissGuidelineCandidateRequest{},
		&postpilotv1.SetDefaultGuidelineEnabledRequest{},
	} {
		fields := message.ProtoReflect().Descriptor().Fields()
		for i := 0; i < fields.Len(); i++ {
			switch name := string(fields.Get(i).Name()); name {
			case "user_id", "account_id", "owner_id":
				t.Fatalf("%s carries %s", message.ProtoReflect().Descriptor().FullName(), name)
			}
		}
	}
}

// A2: an unset scope is a refusal, never a silent "global" that would apply the rule to every
// post of the account.
func TestUnspecifiedScopeIsRefused(t *testing.T) {
	if _, err := fromProtoScope(postpilotv1.GuidelineScope_GUIDELINE_SCOPE_UNSPECIFIED); !errors.Is(err, guideline.ErrScopeShape) {
		t.Fatalf("unspecified scope err = %v", err)
	}
	for wire, want := range map[postpilotv1.GuidelineScope]guideline.Scope{
		postpilotv1.GuidelineScope_GUIDELINE_SCOPE_GLOBAL:    guideline.ScopeGlobal,
		postpilotv1.GuidelineScope_GUIDELINE_SCOPE_TEMPLATES: guideline.ScopeTemplates,
	} {
		got, err := fromProtoScope(wire)
		if err != nil || got != want {
			t.Fatalf("%v mapped to %q (%v)", wire, got, err)
		}
	}
}

// A2: the projection carries the authored text and the NAME projection, never the raw ids.
func TestGuidelineProjectionCarriesTextScopeAndProjectedNames(t *testing.T) {
	at := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	projected := toProtoGuideline(guideline.Guideline{
		ID: "g1", Text: "CCTV 언급 금지", Scope: guideline.ScopeTemplates,
		TemplateIDs: []string{"p1"}, Templates: []guideline.TemplateRef{{ID: "p1", Name: "리뷰"}},
		CreatedAt: at, UpdatedAt: at,
	})
	if projected.GetId() != "g1" || projected.GetText() != "CCTV 언급 금지" {
		t.Fatalf("projection = %+v", projected)
	}
	if projected.GetScope() != postpilotv1.GuidelineScope_GUIDELINE_SCOPE_TEMPLATES {
		t.Fatalf("scope = %v", projected.GetScope())
	}
	if len(projected.GetTemplates()) != 1 || projected.GetTemplates()[0].GetName() != "리뷰" {
		t.Fatalf("templates = %+v", projected.GetTemplates())
	}
	// An orphaned scoped guideline is a real state: templates empty, scope still PURPOSES.
	orphan := toProtoGuideline(guideline.Guideline{ID: "g2", Scope: guideline.ScopeTemplates, CreatedAt: at, UpdatedAt: at})
	if orphan.GetScope() != postpilotv1.GuidelineScope_GUIDELINE_SCOPE_TEMPLATES || len(orphan.GetTemplates()) != 0 {
		t.Fatalf("orphan projection = %+v", orphan)
	}
	if toProtoGuideline(guideline.Guideline{}) != nil {
		t.Fatal("an empty guideline projected as a message instead of unset")
	}
}

// A candidate carries no scope on the wire, by design: scope is chosen at approval, so there is
// no field a client could set that would apply a rule to every post without that choice.
func TestGuidelineCandidateCarriesNoScope(t *testing.T) {
	fields := (&postpilotv1.GuidelineCandidate{}).ProtoReflect().Descriptor().Fields()
	for i := 0; i < fields.Len(); i++ {
		switch name := string(fields.Get(i).Name()); name {
		case "scope", "template_ids":
			t.Fatalf("GuidelineCandidate carries %s", name)
		}
	}
}

// knownFields is a field directory that knows every 분야, for tests about everything else.
type knownFields struct{}

func (knownFields) Known(string) bool { return true }

// ARCH-3, both ways: every named scope but UNSPECIFIED is one domain kind and back, UNSPECIFIED
// and any unnamed number are refused, and a kind the edge cannot name goes out as UNSPECIFIED,
// never as GLOBAL.
func TestGuidelineScopeWalksTheGeneratedEnum(t *testing.T) {
	seen := map[guideline.Scope]bool{}
	for number := range postpilotv1.GuidelineScope_name {
		wire := postpilotv1.GuidelineScope(number)
		scope, err := fromProtoScope(wire)
		if wire == postpilotv1.GuidelineScope_GUIDELINE_SCOPE_UNSPECIFIED {
			if !errors.Is(err, guideline.ErrScopeShape) {
				t.Fatalf("UNSPECIFIED mapped to %q (%v)", scope, err)
			}
			continue
		}
		if err != nil || seen[scope] || !scope.Valid() {
			t.Fatalf("%s mapped to %q (%v, seen=%v)", wire, scope, err, seen[scope])
		}
		seen[scope] = true
		if back := toProtoScope(scope); back != wire {
			t.Fatalf("%q came back as %s", scope, back)
		}
	}
	if len(seen) != 3 {
		t.Fatalf("the enum names %d kinds, want global, templates and fields", len(seen))
	}
	if _, err := fromProtoScope(postpilotv1.GuidelineScope(99)); !errors.Is(err, guideline.ErrScopeShape) {
		t.Fatalf("an unnamed number mapped: %v", err)
	}
	if got := toProtoScope("voice"); got != postpilotv1.GuidelineScope_GUIDELINE_SCOPE_UNSPECIFIED {
		t.Fatalf("an unknown kind went out as %s", got)
	}
}

// 분야 cross the edge only through the shared mapper: out, an id it cannot name is dropped.
func TestFieldsProjectThroughTheSharedMapper(t *testing.T) {
	at := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	projected := toProtoGuideline(guideline.Guideline{
		ID: "g1", Text: "메뉴 가격은 쓰지 않기", Scope: guideline.ScopeFields,
		Fields: []string{"cafe", "retired", "pets"}, CreatedAt: at, UpdatedAt: at,
	})
	if projected.GetScope() != postpilotv1.GuidelineScope_GUIDELINE_SCOPE_FIELDS {
		t.Fatalf("scope = %s", projected.GetScope())
	}
	if want := []postpilotv1.BlogField{postpilotv1.BlogField_BLOG_FIELD_CAFE, postpilotv1.BlogField_BLOG_FIELD_PETS}; !reflect.DeepEqual(projected.GetFields(), want) {
		t.Fatalf("fields = %v, want %v", projected.GetFields(), want)
	}
}

// GUIDE-14: ListGuidelines answers the owner's guidelines alone, in injection order — the global
// group, then the 분야 group — and the response has no member beside them.
func TestListGuidelinesAnswersTheOwnersGuidelinesAlone(t *testing.T) {
	handle, err := db.Open(filepath.Join(t.TempDir(), "guidelines.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { handle.Close() })
	ctx := context.Background()
	if err := db.Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	users := authstore.New(handle.Writer, handle.Reader)
	for _, id := range []string{"alice", "bob"} {
		if err := users.CreateUser(ctx, auth.User{ID: id, PasswordHash: "hash", Plan: plan.Free, CreatedAt: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	handler := NewHandler(guideline.NewService(guidelinestore.New(handle.Writer, handle.Reader), knownFields{}, guideline.Limits{TextMaxChars: 300, TitleMaxChars: 40, MaxPerAccount: 10}, 5))
	alice, bob := auth.WithUser(ctx, "alice"), auth.WithUser(ctx, "bob")
	create := func(user context.Context, request *postpilotv1.CreateGuidelineRequest) {
		t.Helper()
		if _, err := handler.CreateGuideline(user, connect.NewRequest(request)); err != nil {
			t.Fatal(err)
		}
	}
	fields := postpilotv1.GuidelineScope_GUIDELINE_SCOPE_FIELDS
	global := postpilotv1.GuidelineScope_GUIDELINE_SCOPE_GLOBAL
	create(alice, &postpilotv1.CreateGuidelineRequest{Text: "메뉴 가격은 쓰지 않기", Scope: fields, Fields: []postpilotv1.BlogField{postpilotv1.BlogField_BLOG_FIELD_CAFE}})
	create(alice, &postpilotv1.CreateGuidelineRequest{Text: "과장 금지", Title: "  과장  ", Scope: global})
	create(bob, &postpilotv1.CreateGuidelineRequest{Text: "bob의 지침", Scope: global})

	listed, err := handler.ListGuidelines(alice, connect.NewRequest(&postpilotv1.ListGuidelinesRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	var texts []string
	for _, g := range listed.Msg.GetGuidelines() {
		texts = append(texts, g.GetText())
	}
	if want := []string{"과장 금지", "메뉴 가격은 쓰지 않기"}; !reflect.DeepEqual(texts, want) {
		t.Fatalf("listed = %q, want %q", texts, want)
	}
	// GUIDE-46: the title is trimmed on the way in and read back; an untitled rule reads empty, and
	// the update applies it by presence.
	exaggeration := listed.Msg.GetGuidelines()[0]
	if exaggeration.GetTitle() != "과장" || listed.Msg.GetGuidelines()[1].GetTitle() != "" {
		t.Fatalf("titles = %q, %q", exaggeration.GetTitle(), listed.Msg.GetGuidelines()[1].GetTitle())
	}
	renamed, err := handler.UpdateGuideline(alice, connect.NewRequest(&postpilotv1.UpdateGuidelineRequest{Id: exaggeration.GetId(), Title: proto.String("과장 표현")}))
	if err != nil || renamed.Msg.GetGuideline().GetTitle() != "과장 표현" || renamed.Msg.GetGuideline().GetText() != "과장 금지" {
		t.Fatalf("rename = %v, %v", renamed, err)
	}
	if _, err := handler.UpdateGuideline(alice, connect.NewRequest(&postpilotv1.UpdateGuidelineRequest{Id: exaggeration.GetId(), Title: proto.String(strings.Repeat("가", 41))})); connect.CodeOf(err) != connect.CodeInvalidArgument || appErrorDetail(t, err).GetReason() != "GUIDELINE_TITLE_TOO_LONG" {
		t.Fatalf("an over-long title = %v", err)
	}
	response := listed.Msg.ProtoReflect().Descriptor().Fields()
	if response.Len() != 2 || response.Get(0).Name() != "guidelines" || response.Get(1).Name() != "defaults" {
		t.Fatalf("ListGuidelinesResponse carries %d fields, want guidelines and defaults", response.Len())
	}

	// GUIDE-14, GUIDE-43: the post list carries every 기본 지침 with its switch, both copies, and a
	// switch saves on change and reads back; the clip kind lists its own defaults and no owner row.
	defaults := listed.Msg.GetDefaults()
	if len(defaults) != 14 || defaults[0].GetKey() != "facts" || !defaults[0].GetEnabled() || defaults[0].GetKo().GetName() != "재료에 있는 사실만" || defaults[0].GetEn().GetName() != "Facts from the material only" || !defaults[12].GetKoreanTargetOnly() || !defaults[13].GetKoreanTargetOnly() {
		t.Fatalf("defaults = %v", defaults)
	}
	// GEN-73: the memories default says so on the wire, and it alone does.
	for i, d := range defaults {
		if d.GetMemoriesOnly() != (i == 2 && d.GetKey() == "memory_impressions") {
			t.Errorf("%s memories_only = %v at %d", d.GetKey(), d.GetMemoriesOnly(), i)
		}
	}
	switched, err := handler.SetDefaultGuidelineEnabled(alice, connect.NewRequest(&postpilotv1.SetDefaultGuidelineEnabledRequest{Key: "tags", Enabled: false}))
	if err != nil || switched.Msg.GetDefaultGuideline().GetKey() != "tags" || switched.Msg.GetDefaultGuideline().GetEnabled() {
		t.Fatalf("switch = %v, %v", switched, err)
	}
	again, _ := handler.ListGuidelines(alice, connect.NewRequest(&postpilotv1.ListGuidelinesRequest{Kind: postpilotv1.GuidelineKind_GUIDELINE_KIND_POST}))
	for _, d := range again.Msg.GetDefaults() {
		if d.GetEnabled() != (d.GetKey() != "tags") {
			t.Errorf("%s enabled = %v after the switch", d.GetKey(), d.GetEnabled())
		}
	}
	other, _ := handler.ListGuidelines(bob, connect.NewRequest(&postpilotv1.ListGuidelinesRequest{}))
	for _, d := range other.Msg.GetDefaults() {
		if d.GetKey() == "tags" && !d.GetEnabled() {
			t.Fatal("alice's switch reached bob")
		}
	}
	clip, err := handler.ListGuidelines(alice, connect.NewRequest(&postpilotv1.ListGuidelinesRequest{Kind: postpilotv1.GuidelineKind_GUIDELINE_KIND_CLIP}))
	if err != nil || len(clip.Msg.GetDefaults()) != 7 || len(clip.Msg.GetGuidelines()) != 0 {
		t.Fatalf("clip list = %v, %v", clip, err)
	}
	// GUIDE-2: a 영상 지침 is created of its kind, listed with it and never in the post list — even
	// with a post guideline's text.
	create(alice, &postpilotv1.CreateGuidelineRequest{Text: "과장 금지", Scope: global, Kind: postpilotv1.GuidelineKind_GUIDELINE_KIND_CLIP})
	clip, _ = handler.ListGuidelines(alice, connect.NewRequest(&postpilotv1.ListGuidelinesRequest{Kind: postpilotv1.GuidelineKind_GUIDELINE_KIND_CLIP}))
	if got := clip.Msg.GetGuidelines(); len(got) != 1 || got[0].GetKind() != postpilotv1.GuidelineKind_GUIDELINE_KIND_CLIP || got[0].GetText() != "과장 금지" {
		t.Fatalf("clip guidelines = %v", got)
	}
	posts, _ := handler.ListGuidelines(alice, connect.NewRequest(&postpilotv1.ListGuidelinesRequest{}))
	for _, g := range posts.Msg.GetGuidelines() {
		if g.GetKind() != postpilotv1.GuidelineKind_GUIDELINE_KIND_POST {
			t.Fatalf("the post list carries %v", g)
		}
	}
	if _, err := handler.CreateGuideline(alice, connect.NewRequest(&postpilotv1.CreateGuidelineRequest{
		Text: "가격 크게", Scope: fields, Fields: []postpilotv1.BlogField{postpilotv1.BlogField_BLOG_FIELD_CAFE},
		Kind: postpilotv1.GuidelineKind_GUIDELINE_KIND_CLIP,
	})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("a 분야 scope on a 영상 지침 = %v", err)
	}
	// GUIDE-7: the candidate queue is per kind and a clip candidate names its project.
	service := guideline.NewService(guidelinestore.New(handle.Writer, handle.Reader), knownFields{}, guideline.Limits{TextMaxChars: 300, TitleMaxChars: 40, MaxPerAccount: 10}, 5)
	if err := service.RecordCandidate(ctx, "alice", guideline.KindClip, "project-1", "자막은 짧게"); err != nil {
		t.Fatal(err)
	}
	queued, err := handler.ListGuidelineCandidates(alice, connect.NewRequest(&postpilotv1.ListGuidelineCandidatesRequest{Kind: postpilotv1.GuidelineKind_GUIDELINE_KIND_CLIP}))
	if err != nil || len(queued.Msg.GetCandidates()) != 1 || queued.Msg.GetCandidates()[0].GetClipId() != "project-1" ||
		queued.Msg.GetCandidates()[0].GetKind() != postpilotv1.GuidelineKind_GUIDELINE_KIND_CLIP {
		t.Fatalf("clip candidates = %v, %v", queued, err)
	}
	if postQueue, _ := handler.ListGuidelineCandidates(alice, connect.NewRequest(&postpilotv1.ListGuidelineCandidatesRequest{})); len(postQueue.Msg.GetCandidates()) != 0 {
		t.Fatalf("the post queue carries a clip candidate: %v", postQueue.Msg.GetCandidates())
	}
	if _, err := handler.SetDefaultGuidelineEnabled(alice, connect.NewRequest(&postpilotv1.SetDefaultGuidelineEnabledRequest{Key: "no_such_key"})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("unknown key = %v", err)
	}
}

// ARCH-3: the kind mapping is walked against the generated enum: UNSPECIFIED reads as POST, and
// each named kind maps to its own.
func TestGuidelineKindWalksTheGeneratedEnum(t *testing.T) {
	want := map[postpilotv1.GuidelineKind]guideline.Kind{
		postpilotv1.GuidelineKind_GUIDELINE_KIND_UNSPECIFIED: guideline.KindPost,
		postpilotv1.GuidelineKind_GUIDELINE_KIND_POST:        guideline.KindPost,
		postpilotv1.GuidelineKind_GUIDELINE_KIND_CLIP:        guideline.KindClip,
	}
	if len(postpilotv1.GuidelineKind_name) != len(want) {
		t.Fatalf("generated kinds = %d, want %d; update the mapping", len(postpilotv1.GuidelineKind_name), len(want))
	}
	for number := range postpilotv1.GuidelineKind_name {
		kind := postpilotv1.GuidelineKind(number)
		if got := fromProtoKind(kind); got != want[kind] {
			t.Errorf("%s = %s, want %s", kind, got, want[kind])
		}
	}
}

// The edge refuses what names no 분야 itself — UNSPECIFIED and unnamed numbers — rather than
// relying on the service's blank-id refusal behind it.
func TestFromProtoFieldsRefusesWhatNamesNoField(t *testing.T) {
	for name, values := range map[string][]postpilotv1.BlogField{
		"UNSPECIFIED":       {postpilotv1.BlogField_BLOG_FIELD_CAFE, postpilotv1.BlogField_BLOG_FIELD_UNSPECIFIED},
		"an unnamed number": {postpilotv1.BlogField(99)},
	} {
		if ids, err := fromProtoFields(values); !errors.Is(err, guideline.ErrFieldNotFound) || ids != nil {
			t.Errorf("%s: %v, %v", name, ids, err)
		}
	}
	if ids, err := fromProtoFields([]postpilotv1.BlogField{postpilotv1.BlogField_BLOG_FIELD_PETS, postpilotv1.BlogField_BLOG_FIELD_CAFE}); err != nil || !reflect.DeepEqual(ids, []string{"pets", "cafe"}) {
		t.Fatalf("named 분야 = %v, %v", ids, err)
	}
}
