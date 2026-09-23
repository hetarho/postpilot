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
		"account cap":        {&guideline.AccountCapError{Max: 100}, connect.CodeFailedPrecondition, "GUIDELINE_LIMIT_REACHED"},
		"unknown candidate":  {guideline.ErrCandidateNotFound, connect.CodeNotFound, "GUIDELINE_CANDIDATE_NOT_FOUND"},
		"unknown 분야":         {guideline.ErrFieldNotFound, connect.CodeNotFound, "GUIDELINE_FIELD_NOT_FOUND"},
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
	handler := NewHandler(guideline.NewService(nil, knownFields{}, guideline.Limits{TextMaxChars: 1, MaxPerAccount: 1}, 1))
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
	// The preset answers only a session too. Its signed-in behaviour needs a store, which this
	// handler's service does not have; the presence-mapping test covers it instead.
	enabled := true
	preset := connect.NewRequest(&postpilotv1.UpdateGuidelinePresetRequest{Enabled: &enabled})
	if _, err := handler.UpdateGuidelinePreset(anonymous, preset); connect.CodeOf(err) != connect.CodeUnauthenticated || appErrorDetail(t, err).GetReason() != "AUTH_REQUIRED" {
		t.Fatalf("update preset = %v", err)
	}

	for _, message := range []proto.Message{
		&postpilotv1.ListGuidelinesRequest{}, &postpilotv1.CreateGuidelineRequest{},
		&postpilotv1.UpdateGuidelineRequest{}, &postpilotv1.DeleteGuidelineRequest{},
		&postpilotv1.ListGuidelineCandidatesRequest{}, &postpilotv1.DismissGuidelineCandidateRequest{},
		&postpilotv1.UpdateGuidelinePresetRequest{},
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

// 분야 cross the edge only through the shared mapper: out, an id it cannot name is dropped;
// the preset carries the product's text with the account's state.
func TestFieldsAndThePresetProject(t *testing.T) {
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
	preset := toProtoPreset(guideline.Preset{Enabled: true, Fields: []string{"restaurant"}})
	if preset.GetText() != guideline.PresetText || !preset.GetEnabled() || !reflect.DeepEqual(preset.GetFields(), []postpilotv1.BlogField{postpilotv1.BlogField_BLOG_FIELD_RESTAURANT}) {
		t.Fatalf("preset = %+v", preset)
	}
	if off := toProtoPreset(guideline.Preset{}); off.GetText() != guideline.PresetText || off.GetEnabled() || len(off.GetFields()) != 0 {
		t.Fatalf("an untouched preset = %+v", off)
	}
}

// UpdateGuidelinePreset is a presence patch over the wire: optional `enabled`, and the `fields`
// MESSAGE, whose presence replaces the set — present and empty clears it. A 분야 the mapper cannot
// name is GUIDELINE_FIELD_NOT_FOUND and applies nothing.
func TestUpdateGuidelinePresetMapsPresence(t *testing.T) {
	handle, err := db.Open(filepath.Join(t.TempDir(), "guidelines.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { handle.Close() })
	ctx := context.Background()
	if err := db.Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	if err := authstore.New(handle.Writer, handle.Reader).CreateUser(ctx, auth.User{ID: "alice", PasswordHash: "hash", Plan: plan.Free, CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(guideline.NewService(guidelinestore.New(handle.Writer, handle.Reader), knownFields{}, guideline.Limits{TextMaxChars: 300, MaxPerAccount: 10}, 5))
	alice := auth.WithUser(ctx, "alice")
	on, off := true, false
	update := func(request *postpilotv1.UpdateGuidelinePresetRequest) (*postpilotv1.GuidelinePreset, error) {
		response, err := handler.UpdateGuidelinePreset(alice, connect.NewRequest(request))
		if err != nil {
			return nil, err
		}
		return response.Msg.GetPreset(), nil
	}
	expect := func(step string, got *postpilotv1.GuidelinePreset, enabled bool, fields ...postpilotv1.BlogField) {
		t.Helper()
		if got.GetText() != guideline.PresetText || got.GetEnabled() != enabled || len(got.GetFields()) != len(fields) || (len(fields) > 0 && !reflect.DeepEqual(got.GetFields(), fields)) {
			t.Fatalf("%s: preset = %+v, want enabled=%v fields=%v", step, got, enabled, fields)
		}
	}
	cafe, pets := postpilotv1.BlogField_BLOG_FIELD_CAFE, postpilotv1.BlogField_BLOG_FIELD_PETS

	got, err := update(&postpilotv1.UpdateGuidelinePresetRequest{Enabled: &on})
	if err != nil {
		t.Fatal(err)
	}
	expect("switched on", got, true)
	got, err = update(&postpilotv1.UpdateGuidelinePresetRequest{Fields: &postpilotv1.GuidelinePresetFields{Fields: []postpilotv1.BlogField{pets, cafe, pets}}})
	if err != nil {
		t.Fatal(err)
	}
	expect("fields alone keep the switch", got, true, cafe, pets)
	got, err = update(&postpilotv1.UpdateGuidelinePresetRequest{Enabled: &off})
	if err != nil {
		t.Fatal(err)
	}
	expect("the switch alone keeps the fields", got, false, cafe, pets)

	for name, bad := range map[string]postpilotv1.BlogField{"UNSPECIFIED": postpilotv1.BlogField_BLOG_FIELD_UNSPECIFIED, "an unnamed number": postpilotv1.BlogField(99)} {
		_, err := update(&postpilotv1.UpdateGuidelinePresetRequest{Enabled: &on, Fields: &postpilotv1.GuidelinePresetFields{Fields: []postpilotv1.BlogField{cafe, bad}}})
		if connect.CodeOf(err) != connect.CodeNotFound || appErrorDetail(t, err).GetReason() != "GUIDELINE_FIELD_NOT_FOUND" {
			t.Fatalf("%s: err = %v", name, err)
		}
	}
	listed, err := handler.ListGuidelines(alice, connect.NewRequest(&postpilotv1.ListGuidelinesRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	expect("a refused update applied nothing", listed.Msg.GetPreset(), false, cafe, pets)

	got, err = update(&postpilotv1.UpdateGuidelinePresetRequest{Fields: &postpilotv1.GuidelinePresetFields{}})
	if err != nil {
		t.Fatal(err)
	}
	expect("present and empty clears", got, false)
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
