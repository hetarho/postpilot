package main

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/mail"
	"github.com/postpilot/backend/internal/modelcatalog"
	modelcatalogstore "github.com/postpilot/backend/internal/modelcatalog/store"
	"github.com/postpilot/backend/internal/platform/config"
	"github.com/postpilot/backend/internal/platform/db"
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
	p := &platform{cfg: cfg, db: handle, catalog: catalog, registry: &llm.Registry{}, mailer: mail.NewLog()}

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
	registerJobs(app)
	if got := handlers(app); len(got) != 23 {
		t.Fatalf("handlers = %d, want every Connect service", len(got))
	}
}
