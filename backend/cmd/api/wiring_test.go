package main

import (
	"connectrpc.com/connect"
	"context"
	authrpc "github.com/postpilot/backend/internal/auth/rpc"
	"github.com/postpilot/backend/internal/platform/rpcserver"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/mail"
	"github.com/postpilot/backend/internal/modelcatalog"
	modelcatalogstore "github.com/postpilot/backend/internal/modelcatalog/store"
	"github.com/postpilot/backend/internal/platform/config"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/post"
)

// TestBuildContextsWiresEveryRequiredCollaborator constructs the whole graph the server
// boots with (ARCH-40): every constructor that panics on a missing collaborator runs here
// over the defaults config.Load serves, so a wire the composition root forgot fails this
// test instead of the first request. Object storage is the one platform piece stubbed
// (a nil bucket behind the interfaces): nothing dereferences it at construction.
func TestBuildContextsWiresEveryRequiredCollaborator(t *testing.T) {
	t.Setenv("MAIL_DRIVER", "log")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	p := wiringPlatform(t, cfg)

	app, err := buildContexts(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	// Every context the server exposes is constructed; a nil here is a wire main forgot.
	v := reflect.ValueOf(app).Elem()
	for i := 0; i < v.NumField(); i++ {
		f, name := v.Field(i), v.Type().Field(i).Name
		if name == "payments" {
			continue // nil is the documented shape while billing is disabled
		}
		if (f.Kind() == reflect.Pointer || f.Kind() == reflect.Interface || f.Kind() == reflect.Func) && f.IsNil() {
			t.Errorf("contexts.%s is nil after buildContexts", name)
		}
	}
	// Template's number bounds are post's own (TMPL-47), handed over by the command.
	if limits := app.template.Limits(); limits.TargetLengthMin != post.TargetLengthMin || limits.TargetLengthMax != post.TargetLengthMax ||
		limits.TagCountMin != post.TagCountRange.Min || limits.TagCountMax != post.TagCountRange.Max {
		t.Fatalf("template limits = %+v, want post's target-length and tag-count ranges", limits)
	}
	registerJobs(app)
	got := handlers(app)
	if len(got) != 27 {
		t.Fatalf("handlers = %d, want every Connect service", len(got))
	}
	for _, register := range got {
		path, _ := register()
		if strings.Contains(path, "Publishing") {
			t.Fatalf("retired publishing route is still registered: %s", path)
		}
		if strings.Contains(path, "VoiceLearningService") || strings.Contains(path, "VoiceValidationService") {
			t.Fatalf("retired voice learning route is still registered: %s", path)
		}
	}
	public := rpcserver.New(cfg, "test", rpcserver.Options{Handlers: got, Interceptors: []connect.Interceptor{authrpc.NewInterceptor(app.auth, app.throttle, cfg.ClientIPHeader)}})
	for path, want := range map[string]int{
		"/postpilot.v1.ClipMediaWorkerService/GetMediaRuntimeStatus":      http.StatusNotFound,
		"/postpilot.v1.PostService/ListPosts":                             http.StatusUnauthorized,
		"/postpilot.v1.SpeechProfileService/ListSpeechProfiles":           http.StatusUnauthorized,
		"/postpilot.v1.SpokenVoiceService/ListSpokenDrafts":               http.StatusUnauthorized,
		"/postpilot.v1.SpokenVoiceGenerationService/QuoteVoiceCandidates": http.StatusUnauthorized,
	} {
		r := httptest.NewRequest(http.MethodPost, path, strings.NewReader("{}"))
		r.Header.Set("X-Media-Worker-ID", "prod-worker")
		r.Header.Set("Authorization", "Bearer worker-secret")
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		public.Handler.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("worker authority on public %s = %d, want %d", path, w.Code, want)
		}
	}
}

// wiringPlatform is the platform the server boots over, from cfg: a migrated throwaway
// database, the curated catalog, the bundled fonts and no billing. Object storage is the one
// piece stubbed (a nil bucket behind the interfaces): nothing dereferences it at construction.
func wiringPlatform(t *testing.T, cfg *config.Config) *platform {
	t.Helper()
	cfg.ClipWorkRoot = filepath.Join(t.TempDir(), "work")
	cfg.BillingEnabled = false
	// The renderer pins the bundled faces by checksum; on a host they live in the repo.
	cfg.ClipFontPaths = map[string]string{}
	for key, name := range map[string]string{
		"wantedsans":        "wantedsans/WantedSansVariable.ttf",
		"paperlogy":         "paperlogy/Paperlogy-8ExtraBold.ttf",
		"jua":               "jua/Jua-Regular.ttf",
		"nanummyeongjo":     "nanummyeongjo/NanumMyeongjo-Regular.ttf",
		"nanummyeongjo-800": "nanummyeongjo/NanumMyeongjo-ExtraBold.ttf",
	} {
		path, err := filepath.Abs("../../assets/fonts/" + name)
		if err != nil {
			t.Fatal(err)
		}
		cfg.ClipFontPaths[key] = path
	}
	handle, err := db.Open(filepath.Join(t.TempDir(), "wiring.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { handle.Close() })
	ctx := context.Background()
	if err := db.Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	catalog := modelcatalog.NewService(modelcatalogstore.New(handle.Writer, handle.Reader))
	if err := catalog.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	registry := &llm.Registry{}
	return &platform{cfg: cfg, db: handle, catalog: catalog, speechCatalog: modelcatalog.NewSpeechService(modelcatalogstore.New(handle.Writer, handle.Reader), registry), registry: registry, mailer: mail.NewLog()}
}

// VOICE-4: the account bootstraps the server boots with create no voice — a verified signup
// lists none and receives no signup credit grant.
func TestAVerifiedSignupListsNoVoice(t *testing.T) {
	t.Setenv("MAIL_DRIVER", "log")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	p := wiringPlatform(t, cfg)
	mailer := &captureMailer{}
	p.mailer = mailer
	app, err := buildContexts(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.auth.Signup(ctx, "alice@example.com", "password1"); err != nil {
		t.Fatal(err)
	}
	if len(mailer.sent) != 1 {
		t.Fatalf("verification mails = %d", len(mailer.sent))
	}
	if err := app.auth.VerifyEmail(ctx, mailToken(t, mailer.sent[0])); err != nil {
		t.Fatal(err)
	}
	voices, err := app.voice.ListVoices(ctx, "alice@example.com")
	if err != nil || len(voices) != 0 {
		t.Fatalf("a verified signup lists %+v, %v", voices, err)
	}
	var lots int
	if err := p.db.Reader.QueryRowContext(ctx, "SELECT COUNT(*) FROM credit_lots WHERE user_id = ?", "alice@example.com").Scan(&lots); err != nil || lots != 0 {
		t.Fatalf("signup opened %d credit lots, error %v", lots, err)
	}
}
