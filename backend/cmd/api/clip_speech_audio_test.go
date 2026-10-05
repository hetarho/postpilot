package main

import (
	"context"
	"errors"
	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/clip"
	cliprpc "github.com/postpilot/backend/internal/clip/rpc"
	"net/http"
	"net/http/httptest"
	"testing"
)

func (o fixtureClipObjects) ReadClipSpeechAudio(ctx context.Context, key string, n int64) ([]byte, error) {
	return o.ReadSpokenAudio(ctx, key, n)
}

type clipPlaybackSession struct{ owner string }

func (s clipPlaybackSession) Authenticate(context.Context, string) (auth.Actor, error) {
	return auth.Actor{UserID: s.owner}, nil
}
func TestPrivateClipSpeechPlaybackRechecksOwnerHashAndProjectDeletion(t *testing.T) {
	h := newClipSpeechFixture(t)
	id := h.start(t, "audio-ticket")
	if e := h.run(t, id); e != nil {
		t.Fatal(e)
	}
	p, edit := h.plan(t)
	asset := edit.Narration.Segments[0].Speech.AssetID
	if _, e := h.s.SpeechAccess(t.Context(), "bob", p.ID, asset); !errors.Is(e, clip.ErrNotFound) {
		t.Fatal("foreign ticket", e)
	}
	a, e := h.s.SpeechAccess(t.Context(), "alice", p.ID, asset)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = h.s.ReadSpeechPlayback(t.Context(), "bob", a.ID); !errors.Is(e, clip.ErrNotFound) {
		t.Fatal("copied ticket authorized", e)
	}
	data, e := h.s.ReadSpeechPlayback(t.Context(), "alice", a.ID)
	if e != nil || len(data) == 0 || a.Bytes != int64(len(data)) {
		t.Fatal("private playback", e)
	}
	handler := cliprpc.NewSpeechAudioHandler(h.s, clipPlaybackSession{"alice"})
	request := httptest.NewRequest(http.MethodGet, cliprpc.SpeechAudioPath+a.ID, nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, request)
	if w.Code != http.StatusUnauthorized {
		t.Fatal("anonymous playback", w.Code)
	}
	request.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "session"})
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, request)
	if w.Code != 200 || w.Header().Get("Cache-Control") != "private, no-store" || w.Header().Get("Content-Type") != "audio/mpeg" {
		t.Fatal("private headers", w.Code, w.Header())
	}
	row, e := h.store.GetSpeechAsset(t.Context(), "alice", p.ID, asset)
	if e != nil {
		t.Fatal(e)
	}
	original := h.f.objects.data[row.ObjectKey]
	h.f.objects.data[row.ObjectKey] = []byte("corrupt")
	if _, e = h.s.ReadSpeechPlayback(t.Context(), "alice", a.ID); e == nil {
		t.Fatal("corrupt speech served")
	}
	h.f.objects.data[row.ObjectKey] = original
	if _, e = h.f.handle.Writer.Exec("DELETE FROM clip_projects WHERE id=? AND user_id=?", p.ID, "alice"); e != nil {
		t.Fatal(e)
	}
	if _, e = h.s.ReadSpeechPlayback(t.Context(), "alice", a.ID); !errors.Is(e, clip.ErrNotFound) {
		t.Fatal("deletion did not revoke access", e)
	}
	if _, e = h.f.library.GetVoice(t.Context(), "alice", h.voice.ID); e != nil {
		t.Fatal("project deletion removed reusable voice", e)
	}
}
