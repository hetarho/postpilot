package store_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/auth"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/voice/spoken"
	spokenrpc "github.com/postpilot/backend/internal/voice/spoken/rpc"
	"github.com/postpilot/backend/internal/voice/spoken/store"
	"google.golang.org/protobuf/encoding/protojson"
)

type profiles struct{ p spoken.Profile }

func (p profiles) ResolveSpokenProfile(context.Context, string, plan.Plan, string, int64, string) (spoken.Profile, error) {
	return p.p, nil
}

type objects struct {
	data                map[string][]byte
	failPut, failDelete bool
	afterPut            func()
}

func (o *objects) PutSpokenAudio(_ context.Context, key string, data []byte) error {
	if o.failPut {
		return errors.New("unavailable")
	}
	if _, exists := o.data[key]; exists {
		return errors.New("immutable")
	}
	o.data[key] = bytes.Clone(data)
	if o.afterPut != nil {
		f := o.afterPut
		o.afterPut = nil
		f()
	}
	return nil
}
func (o *objects) ReadSpokenAudio(_ context.Context, key string, _ int64) ([]byte, error) {
	v, ok := o.data[key]
	if !ok {
		return nil, errors.New("missing")
	}
	return bytes.Clone(v), nil
}
func (o *objects) DeleteSpokenAudio(_ context.Context, key string) error {
	if o.failDelete {
		return errors.New("unavailable")
	}
	delete(o.data, key)
	return nil
}

type fixture struct {
	t       *testing.T
	handle  *db.DB
	path    string
	store   *store.Store
	svc     *spoken.Service
	objects *objects
	profile profiles
}

func fresh(t *testing.T) *fixture {
	t.Helper()
	path := filepath.Join(t.TempDir(), "library.db")
	h, err := db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = h.Close() })
	if err := db.Migrate(t.Context(), h.Writer); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"alice", "bob"} {
		if _, err := h.Writer.Exec("INSERT INTO users(id,password_hash,plan,created_at) VALUES (?,?,'free',?)", id, "hash", time.Now().Format(time.RFC3339Nano)); err != nil {
			t.Fatal(err)
		}
	}
	p := profiles{spoken.Profile{ID: "curated", Revision: 2, Design: llm.ModelRef{ProviderID: "speech", ModelID: "design"}, Synthesis: llm.ModelRef{ProviderID: "speech", ModelID: "tts"}, DesignLabel: "Design", SpeechLabel: "Korean", Grade: "balanced", Settings: llm.SpeechSettings{Speed: 1, Stability: .5, SimilarityBoost: .75}, DescriptionMax: 1000, PreviewMax: 1000, SpeechMax: 1000, OutputFormat: llm.SpeechOutputFormat}}
	o := &objects{data: map[string][]byte{}}
	s := store.New(h.Writer, h.Reader)
	return &fixture{t: t, handle: h, path: path, store: s, svc: spoken.NewService(s, p, o), objects: o, profile: p}
}
func draftInput() spoken.DraftInput {
	return spoken.DraftInput{Name: "  My sound  ", Description: strings.Repeat("한국어로 차분하게 말합니다. ", 3), PreviewText: strings.Repeat("오늘의 이야기를 함께 들어 보세요. ", 7), ProfileID: "curated", ProfileRevision: 2}
}
func (f *fixture) draft(key string) spoken.Draft {
	f.t.Helper()
	d, err := f.svc.CreateDraft(f.t.Context(), "alice", plan.Free, key, draftInput())
	if err != nil {
		f.t.Fatal(err)
	}
	return d
}
func candidates() []llm.VoiceCandidate {
	out := make([]llm.VoiceCandidate, 3)
	for i := range out {
		b := []byte(fmt.Sprintf("validated-private-mp3-%d", i))
		h := sha256.Sum256(b)
		out[i] = llm.VoiceCandidate{Handle: llm.CandidateHandle(fmt.Sprintf("supplier-private-%d", i)), Audio: llm.EncodedAudio{Bytes: b, SHA256: hex.EncodeToString(h[:]), Format: llm.SpeechOutputFormat, Samples: 44100 + int64(i)*441, SampleRate: 44100, Channels: 2}}
	}
	return out
}
func (f *fixture) ready(key string) spoken.Draft {
	f.t.Helper()
	d := f.draft(key)
	d, err := f.svc.SaveCandidates(f.t.Context(), "alice", d.ID, d.Revision, key+"-job", candidates())
	if err != nil {
		f.t.Fatal(err)
	}
	return d
}
func (f *fixture) confirmed(key string) spoken.Voice {
	f.t.Helper()
	d := f.ready(key)
	c := d.Candidates[0]
	p, err := f.svc.SampleAccess(f.t.Context(), "alice", c.AssetID)
	if err != nil {
		f.t.Fatal(err)
	}
	if _, err = f.svc.ReadPlayback(f.t.Context(), "alice", p.ID); err != nil {
		f.t.Fatal(err)
	}
	d, err = f.svc.AcknowledgeCandidate(f.t.Context(), "alice", d.ID, d.Revision, key+"-ack", c.ID, p.ID)
	if err != nil {
		f.t.Fatal(err)
	}
	d, err = f.svc.SelectCandidate(f.t.Context(), "alice", d.ID, d.Revision, key+"-select", c.ID)
	if err != nil {
		f.t.Fatal(err)
	}
	v, err := f.svc.SaveConfirmation(f.t.Context(), "alice", d.ID, d.Revision, key+"-confirm", c.ID, "private-confirmed-handle")
	if err != nil {
		f.t.Fatal(err)
	}
	return v
}

func TestSpokenReloadDuplicatesAndStaleRevisions(t *testing.T) {
	f := fresh(t)
	d := f.draft("create")
	again, err := f.svc.CreateDraft(t.Context(), "alice", plan.Free, "create", draftInput())
	if err != nil || again.ID != d.ID || d.Name != "My sound" {
		t.Fatal(again, err)
	}
	in := draftInput()
	in.Name = "changed"
	if _, err := f.svc.CreateDraft(t.Context(), "alice", plan.Free, "create", in); !errors.Is(err, spoken.ErrConflict) {
		t.Fatal(err)
	}
	d, err = f.svc.UpdateDraft(t.Context(), "alice", plan.Free, d.ID, d.Revision, "update", in)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.UpdateDraft(t.Context(), "alice", plan.Free, d.ID, 1, "stale", in); !errors.Is(err, spoken.ErrConflict) {
		t.Fatal(err)
	}
	same, err := f.svc.UpdateDraft(t.Context(), "alice", plan.Free, d.ID, 1, "update", in)
	if err != nil || same.Revision != 2 {
		t.Fatal(same, err)
	}
	ready := f.ready("auditions")
	same, err = f.svc.SaveCandidates(t.Context(), "alice", ready.ID, 1, "auditions-job", candidates())
	if err != nil || same.Candidates[0].ID != ready.Candidates[0].ID || len(f.objects.data) != 3 {
		t.Fatal(same, err)
	}
	v := f.confirmed("voice")
	h, err := db.Open(f.path)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	s := spoken.NewService(store.New(h.Writer, h.Reader), f.profile, f.objects)
	reloaded, err := s.GetVoice(t.Context(), "alice", v.ID)
	if err != nil || reloaded.Profile != v.Profile || reloaded.Handle != v.Handle || reloaded.SampleDurationMS != 1000 {
		t.Fatal(reloaded, err)
	}
	d, err = s.GetDraft(t.Context(), "alice", ready.ID)
	if err != nil || d.Phase() != "candidates" || len(d.Candidates) != 3 || d.Candidates[2].DurationMS != 1020 {
		t.Fatal(d, err)
	}
	if err := s.DeleteDraft(t.Context(), "alice", d.ID, d.Revision, "delete"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteDraft(t.Context(), "alice", d.ID, d.Revision, "delete"); err != nil {
		t.Fatal("duplicate delete", err)
	}
}
func TestSpokenImmutableRenameRemovalAndRetainedSample(t *testing.T) {
	f := fresh(t)
	v := f.confirmed("sound")
	drafts, _ := f.svc.ListDrafts(t.Context(), "alice")
	d := drafts[0]
	if _, err := f.svc.UpdateDraft(t.Context(), "alice", plan.Free, d.ID, d.Revision, "alter", draftInput()); !errors.Is(err, spoken.ErrImmutable) {
		t.Fatal(err)
	}
	renamed, err := f.svc.RenameVoice(t.Context(), "alice", v.ID, v.Revision, "rename", " Renamed ")
	if err != nil || renamed.Name != "Renamed" || renamed.Handle != v.Handle || renamed.Profile != v.Profile || renamed.SampleAssetID != v.SampleAssetID {
		t.Fatal(renamed, err)
	}
	if _, err := f.svc.AcquireVoice(t.Context(), "alice", v.ID, renamed.Revision, "clip-existing"); err != nil {
		t.Fatal(err)
	}
	removed, err := f.svc.RemoveVoice(t.Context(), "alice", v.ID, renamed.Revision, "remove")
	if err != nil || removed.RemovedAt == nil {
		t.Fatal(removed, err)
	}
	if _, err := f.svc.AcquireVoice(t.Context(), "alice", v.ID, removed.Revision, "clip-new"); !errors.Is(err, spoken.ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := f.svc.AcquireVoice(t.Context(), "alice", v.ID, renamed.Revision, "clip-existing"); err != nil {
		t.Fatal("existing reference", err)
	}
	active, _ := f.svc.ListVoices(t.Context(), "alice", false)
	all, _ := f.svc.ListVoices(t.Context(), "alice", true)
	if len(active) != 0 || len(all) != 1 {
		t.Fatal(active, all)
	}
	if err := f.svc.DeleteDraft(t.Context(), "alice", d.ID, d.Revision, "delete-draft"); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Cleanup(t.Context(), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if len(f.objects.data) != 1 {
		t.Fatal("unconfirmed candidates were not collected", len(f.objects.data))
	}
	p, err := f.svc.SampleAccess(t.Context(), "alice", v.SampleAssetID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ReadPlayback(t.Context(), "alice", p.ID); err != nil {
		t.Fatal("removed voice audio", err)
	}
	separate := f.confirmed("new-sound")
	if separate.ID == v.ID {
		t.Fatal("new sound reused voice identity")
	}
}
func TestSpokenAuditionOwnerExpiryRevocationAndSoundChange(t *testing.T) {
	f := fresh(t)
	d := f.ready("auditions")
	c := d.Candidates[0]
	p, err := f.svc.SampleAccess(t.Context(), "alice", c.AssetID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.AcknowledgeCandidate(t.Context(), "alice", d.ID, d.Revision, "unplayed", c.ID, p.ID); !errors.Is(err, spoken.ErrAuditionRequired) {
		t.Fatal(err)
	}
	for _, read := range []func() error{
		func() error { _, e := f.svc.GetDraft(t.Context(), "bob", d.ID); return e }, func() error { _, e := f.svc.ReadPlayback(t.Context(), "bob", p.ID); return e }, func() error { _, e := f.svc.SampleAccess(t.Context(), "bob", c.AssetID); return e }, func() error { return f.svc.DeleteDraft(t.Context(), "bob", d.ID, d.Revision, "foreign") }, func() error {
			_, e := f.svc.SelectCandidate(t.Context(), "bob", d.ID, d.Revision, "foreign", c.ID)
			return e
		},
	} {
		if err := read(); !errors.Is(err, spoken.ErrNotFound) {
			t.Fatal("foreign ID", err)
		}
	}
	expired := spoken.Playback{ID: strings.Repeat("e", 32), OwnerID: "alice", AssetID: c.AssetID, ExpiresAt: time.Now().Add(-time.Minute)}
	if err := f.store.SavePlayback(t.Context(), expired); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ReadPlayback(t.Context(), "alice", expired.ID); !errors.Is(err, spoken.ErrNotFound) {
		t.Fatal(err)
	}
	if err := f.store.RevokeAsset(t.Context(), "alice", c.AssetID, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ReadPlayback(t.Context(), "alice", p.ID); !errors.Is(err, spoken.ErrNotFound) {
		t.Fatal(err)
	}
	in := draftInput()
	in.Name = "Name only"
	d, err = f.svc.UpdateDraft(t.Context(), "alice", plan.Free, d.ID, d.Revision, "metadata", in)
	if err != nil || len(d.Candidates) != 2 {
		t.Fatal(d, err)
	}
	in.Description = strings.Repeat("다른 목소리를 만들어 주세요. ", 4)
	d, err = f.svc.UpdateDraft(t.Context(), "alice", plan.Free, d.ID, d.Revision, "sound", in)
	if err != nil || len(d.Candidates) != 0 || d.GenerationID != "" {
		t.Fatal(d, err)
	}
	if _, err := f.svc.SampleAccess(t.Context(), "alice", d.ID); !errors.Is(err, spoken.ErrNotFound) {
		t.Fatal(err)
	}
}
func TestSpokenRemovalReferenceRaceAndAccountCleanup(t *testing.T) {
	f := fresh(t)
	v := f.confirmed("race")
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, e := f.svc.AcquireVoice(t.Context(), "alice", v.ID, v.Revision, "race-reference")
		errs <- e
	}()
	go func() {
		defer wg.Done()
		_, e := f.svc.RemoveVoice(t.Context(), "alice", v.ID, v.Revision, "race-remove")
		errs <- e
	}()
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil && !errors.Is(err, spoken.ErrNotFound) {
			t.Fatal(err)
		}
	}
	if _, err := f.svc.AcquireVoice(t.Context(), "bob", v.ID, v.Revision, "race-reference"); !errors.Is(err, spoken.ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := f.handle.Writer.Exec("DELETE FROM users WHERE id = 'alice'"); err != nil {
		t.Fatal(err)
	}
	pending, err := f.store.PendingCleanup(t.Context(), time.Now().Add(time.Hour))
	if err != nil || len(pending) != 3 {
		t.Fatal(pending, err)
	}
	f.objects.failDelete = true
	if err := f.svc.Cleanup(t.Context(), time.Now().Add(time.Hour)); err == nil {
		t.Fatal("cleanup failure discarded")
	}
	f.objects.failDelete = false
	if err := f.svc.Cleanup(t.Context(), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if len(f.objects.data) != 0 {
		t.Fatal("account audio survived", len(f.objects.data))
	}
}
func TestSpokenFailedUploadsAndStaleResultsAreCollected(t *testing.T) {
	f := fresh(t)
	d := f.draft("failed")
	f.objects.failPut = true
	if _, err := f.svc.SaveCandidates(t.Context(), "alice", d.ID, d.Revision, "fail-job", candidates()); !errors.Is(err, spoken.ErrMediaUnavailable) {
		t.Fatal(err)
	}
	f.objects.failPut = false
	d, err := f.svc.GetDraft(t.Context(), "alice", d.ID)
	if err != nil || d.Phase() != "editing" || len(d.Candidates) != 0 {
		t.Fatal(d, err)
	}
	// An old version is rejected before uploading any object.
	in := draftInput()
	in.Description = strings.Repeat("edited description ", 4)
	d, err = f.svc.UpdateDraft(t.Context(), "alice", plan.Free, d.ID, 1, "edit", in)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.SaveCandidates(t.Context(), "alice", d.ID, 1, "stale-job", candidates()); !errors.Is(err, spoken.ErrConflict) {
		t.Fatal(err)
	}
	if len(f.objects.data) != 0 {
		t.Fatal("stale result uploaded")
	}
	if err := f.svc.Cleanup(t.Context(), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	pending, _ := f.store.PendingCleanup(t.Context(), time.Now().Add(time.Hour))
	if len(pending) != 0 {
		t.Fatal(pending)
	}
	// Editing during object I/O invalidates publication and leaves all uploaded
	// bytes recoverable. The callback also proves no writer is held during I/O.
	d = f.draft("replacement")
	f.objects.afterPut = func() {
		in := draftInput()
		in.Description = strings.Repeat("new exact description ", 4)
		if _, err := f.svc.UpdateDraft(t.Context(), "alice", plan.Free, d.ID, d.Revision, "replace-during-upload", in); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.svc.SaveCandidates(t.Context(), "alice", d.ID, d.Revision, "late-result", candidates()); !errors.Is(err, spoken.ErrConflict) {
		t.Fatal(err)
	}
	if len(f.objects.data) != 3 {
		t.Fatal("expected three orphaned uploaded objects", len(f.objects.data))
	}
	if err := f.svc.Cleanup(t.Context(), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if len(f.objects.data) != 0 {
		t.Fatal("late upload orphaned audio survived")
	}
}

type sessions struct{}

// Inject account removal precisely between the retained read and intent close.
type cleanupDeletionRace struct {
	spoken.Storage
	removed bool
}

func (s *cleanupDeletionRace) AssetRetained(ctx context.Context, id string) (bool, error) {
	retained, err := s.Storage.AssetRetained(ctx, id)
	if err == nil && retained && !s.removed {
		s.removed = true
		err = s.Storage.DeleteOwner(ctx, "alice")
	}
	return retained, err
}
func TestSpokenCleanupRetainedReadCannotLoseConcurrentAccountDeletion(t *testing.T) {
	f := fresh(t)
	_ = f.confirmed("cleanup-race")
	race := &cleanupDeletionRace{Storage: f.store}
	svc := spoken.NewService(race, f.profile, f.objects)
	if err := svc.Cleanup(t.Context(), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	// The retained read was stale; the conditional close must leave that intent.
	if err := svc.Cleanup(t.Context(), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if len(f.objects.data) != 0 {
		t.Fatal("lost recoverable intent", len(f.objects.data))
	}
}

func (sessions) Authenticate(_ context.Context, token string) (auth.Actor, error) {
	if token != "alice" && token != "bob" {
		return auth.Actor{}, auth.ErrNoSession
	}
	return auth.Actor{UserID: token, Plan: plan.Free}, nil
}
func TestSpokenRPCAndPrivateHTTPBoundary(t *testing.T) {
	f := fresh(t)
	d := f.ready("rpc")
	v := f.confirmed("voice")
	handler := spokenrpc.NewHandler(f.svc)
	path, h := postpilotv1connect.NewSpokenVoiceServiceHandler(handler)
	r := httptest.NewRequest(http.MethodPost, path+"ListSpokenDrafts", strings.NewReader("{}"))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatal(w.Code, w.Body.String())
	}
	bob := auth.WithActor(t.Context(), auth.Actor{UserID: "bob", Plan: plan.Free})
	alice := auth.WithActor(t.Context(), auth.Actor{UserID: "alice", Plan: plan.Free})
	if _, err := handler.GetSpokenDraft(bob, connect.NewRequest(&v1.SpokenIDRequest{Id: d.ID})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatal(err)
	}
	if _, err := handler.RenameSpokenVoice(bob, connect.NewRequest(&v1.RenameSpokenVoiceRequest{Id: v.ID, ExpectedRevision: v.Revision, IdempotencyKey: "foreign", Name: "name"})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatal(err)
	}
	response, err := handler.GetSpokenVoice(alice, connect.NewRequest(&v1.SpokenIDRequest{Id: v.ID}))
	if err != nil {
		t.Fatal(err)
	}
	payload, err := protojson.Marshal(response.Msg)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{"private-confirmed-handle", "supplier-private", "private/spoken/", "ownerId", "sha256", "provenance", "USD"} {
		if bytes.Contains(payload, []byte(private)) {
			t.Fatal("private RPC field", string(payload))
		}
	}
	access, err := handler.GetSpokenSampleAccess(alice, connect.NewRequest(&v1.SpokenIDRequest{Id: d.Candidates[0].AssetID}))
	if err != nil {
		t.Fatal(err)
	}
	audio := spokenrpc.NewAudioHandler(f.svc, sessions{})
	for _, tc := range []struct {
		cookie string
		status int
	}{{"", 401}, {"revoked-session", 401}, {"bob", 404}, {"alice", 200}} {
		r := httptest.NewRequest(http.MethodGet, access.Msg.Url, nil)
		if tc.cookie != "" {
			r.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: tc.cookie})
		}
		w := httptest.NewRecorder()
		audio.ServeHTTP(w, r)
		if w.Code != tc.status || w.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatal(tc, w.Code, w.Body.String())
		}
		if tc.status == 200 && w.Header().Get("Content-Type") != "audio/mpeg" {
			t.Fatal(w.Header())
		}
	}
	if err := f.store.RevokeAsset(t.Context(), "alice", d.Candidates[0].AssetID, time.Now()); err != nil {
		t.Fatal(err)
	}
	r = httptest.NewRequest(http.MethodGet, access.Msg.Url, nil)
	r.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "alice"})
	w = httptest.NewRecorder()
	audio.ServeHTTP(w, r)
	if w.Code != 404 {
		t.Fatal(w.Code)
	}
}
