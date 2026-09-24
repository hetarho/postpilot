package main

// This file is compiled only into the release TEST executable. Neither the
// fixture control endpoints nor the provider below exists in /api.
import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/auth"
	authstore "github.com/postpilot/backend/internal/auth/store"
	"github.com/postpilot/backend/internal/clip"
	clipapp "github.com/postpilot/backend/internal/clip/app"
	jobstore "github.com/postpilot/backend/internal/job/store"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/llm/openaicompat"
	"github.com/postpilot/backend/internal/plan"
)

type splitSeed struct{ Cookie, Project string }
type splitState struct {
	Digests                   []string
	Stages                    []splitStage
	Balance, Holds, Deletions int
	DiskBytes                 int64
}
type splitStage struct {
	ID, Operation, State string
	Attempts             int
}

func TestMediaReleaseAPIProcess(t *testing.T) {
	if os.Getenv("MEDIA_RELEASE_CHILD") != "1" {
		t.Skip("separate release API process")
	}
	ctx := t.Context()
	p, err := loadPlatform(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer p.db.Close()
	p.registry, err = llm.Parse([]byte("providers:\n  - id: fixture\n    adapter: openai\n    base_url: http://127.0.0.1:8081/v1\n    reasoning_format: openrouter\n"), func(string) string { return "" }, map[string]llm.AdapterFactory{"openai": func(c llm.AdapterConfig) (llm.Provider, error) { return openaicompat.New(c, http.DefaultClient), nil }}, releaseModelSource{}, llm.Options{Timeout: time.Minute, MaxTokens: 8192})
	if err != nil {
		t.Fatal(err)
	}
	c, err := buildContexts(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	registerJobs(c)
	mux := http.NewServeMux()
	mux.HandleFunc("/seed", func(w http.ResponseWriter, r *http.Request) {
		// Unique disposable database only. The fixture account is never a production user.
		as := authstore.New(p.db.Writer, p.db.Reader)
		var n int
		err := p.db.Reader.QueryRow("SELECT count(*) FROM users WHERE id='release-user'").Scan(&n)
		if err == nil && n == 0 {
			err = as.CreateUser(ctx, auth.User{ID: "release-user", PasswordHash: "fixture-only", Plan: plan.Free, CreatedAt: time.Now()})
		}
		if err == nil {
			err = c.ledger.EnsureMonthlyLot(ctx, "release-user", plan.Free)
		}

		if err == nil {
			_, err = p.db.Writer.ExecContext(ctx, "INSERT OR IGNORE INTO credit_lots(id,user_id,kind,granted,remaining,created_at) VALUES ('split-extra','release-user','purchased',5000,5000,?)", time.Now().UTC().Format(time.RFC3339Nano))
		}
		cookie, hash, e := auth.NewLinkToken()
		if err == nil {
			err = e
		}
		if err == nil {
			err = as.CreateSession(ctx, auth.Session{Token: hash, UserID: "release-user", CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)})
		}
		template, e := c.clip.CreateTemplate(ctx, "release-user", clip.Recipe{Name: "split fixture", Preset: "restaurant", InformationFields: []clip.InformationField{{Label: "상호", Prompt: "name"}, {Label: "위치", Prompt: "where"}, {Label: "place", Prompt: "place"}}})
		if err == nil {
			err = e
		}
		project, e := c.clip.CreateProject(ctx, "release-user", clip.ProjectInput{Language: "ko", Title: "split release", VideoTemplateID: template.ID, Ratio: "horizontal", TargetDurationMS: 15000, Disclosure: "ad", Answers: []clip.Answer{{Label: "상호", Text: "연남 김밥"}, {Label: "위치", Text: "서울 연남동"}, {Label: "place", Text: "fixture"}}})
		if err == nil {
			err = e
		}
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		_ = json.NewEncoder(w).Encode(splitSeed{cookie, project.ID})
	})
	mux.HandleFunc("/state", func(w http.ResponseWriter, r *http.Request) {
		s := splitState{}
		rows, err := p.db.Reader.Query("SELECT worker_digest FROM clip_media_artifacts WHERE accepted_at IS NOT NULL AND slot LIKE 'analysis/%'")
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		for rows.Next() {
			var hash string
			if err = rows.Scan(&hash); err != nil {
				break
			}
			s.Digests = append(s.Digests, hash)
		}
		_ = rows.Close()
		rows, err = p.db.Reader.Query("SELECT id,operation,state,attempt_count FROM clip_media_stages ORDER BY created_at,id")
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		for rows.Next() {
			var stage splitStage
			if err = rows.Scan(&stage.ID, &stage.Operation, &stage.State, &stage.Attempts); err != nil {
				break
			}
			s.Stages = append(s.Stages, stage)
		}
		_ = rows.Close()
		for _, q := range []struct {
			sql string
			dst *int
		}{{"SELECT COALESCE(SUM(remaining),0) FROM credit_lots WHERE user_id='release-user'", &s.Balance}, {"SELECT count(*) FROM usage_admissions WHERE settled_at IS NULL AND hold_credits>0", &s.Holds}, {"SELECT count(*) FROM clip_media_deletions", &s.Deletions}} {
			if err = p.db.Reader.QueryRow(q.sql).Scan(q.dst); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
		}
		_ = filepath.Walk(filepath.Dir(p.cfg.DBPath), func(_ string, info os.FileInfo, e error) error {
			if e == nil && !info.IsDir() {
				s.DiskBytes += info.Size()
			}
			return nil
		})
		_ = json.NewEncoder(w).Encode(s)
	})
	mux.HandleFunc("/cleanup", func(w http.ResponseWriter, r *http.Request) {
		// Advance ONLY the deletion clock beyond signed PUT expiry. Never alter leases,
		// provider accounting or the actual production time source.
		cleanup := clipapp.NewMediaReconciler(p.db.Writer, c.clipPorts, c.clipStore, jobstore.New(p.db.Writer, p.db.Reader, jobKinds()), c.jobs, p.bucket, time.Second, func() time.Time { return time.Now().Add(24 * time.Hour) })
		err := cleanup.Cleanup(r.Context())
		if err != nil {
			http.Error(w, err.Error(), 503)
			return
		}
		w.WriteHeader(204)
	})
	fixture := &http.Server{Addr: "127.0.0.1:8082", Handler: mux, ReadHeaderTimeout: time.Second}
	go func() {
		if e := fixture.ListenAndServe(); e != nil && e != http.ErrServerClosed {
			panic(e)
		}
	}()
	defer fixture.Close()
	if err = serve(ctx, c); err != nil {
		t.Fatal(err)
	}
}

// The remote fixture has two networks. Only this relay joins both. It forwards
// authenticated worker RPC and signed artifacts, with no DB or shared volume.
func TestMediaReleaseRelay(t *testing.T) {
	if os.Getenv("MEDIA_RELEASE_RELAY") != "1" {
		t.Skip("isolated remote network relay")
	}
	errs := make(chan error, 2)
	for _, pair := range [][2]string{{":9002", "http://api:9002"}, {":9001", "http://minio:9000"}} {
		target, _ := url.Parse(pair[1])
		proxy := httputil.NewSingleHostReverseProxy(target)
		server := &http.Server{Addr: pair[0], Handler: proxy, ReadHeaderTimeout: 5 * time.Second}
		go func() { errs <- server.ListenAndServe() }()
	}
	t.Fatal(<-errs)
}

func splitJSON[T any](ctx context.Context, endpoint string) (T, error) {
	var value T
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return value, err
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return value, err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return value, fmt.Errorf("fixture %s: %d %s", req.URL.Path, response.StatusCode, body)
	}
	err = json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&value)
	return value, err
}

func TestMediaReleaseResources(t *testing.T) {
	if os.Getenv("MEDIA_RELEASE_RESOURCE") != "1" {
		t.Skip("container resource snapshot")
	}
	peak, err := os.ReadFile("/sys/fs/cgroup/memory.peak")
	if err != nil {
		t.Fatal(err)
	}
	var disk int64
	for _, root := range []string{"/tmp", "/var/lib/postpilot-media"} {
		_ = filepath.Walk(root, func(_ string, i os.FileInfo, e error) error {
			if e == nil && !i.IsDir() {
				disk += i.Size()
			}
			return nil
		})
	}
	t.Logf("RESOURCE_REPORT {\"memory_peak\":%s,\"disk_bytes\":%d}", strings.TrimSpace(string(peak)), disk)
}
