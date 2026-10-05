package rpc

import (
	"connectrpc.com/connect"
	"context"
	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/clip"
	clipapp "github.com/postpilot/backend/internal/clip/app"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"net/http"
	"strings"
	"time"
)

const SpeechAudioPath = "/clip/speech/"

type SpeechAudioSessions interface {
	Authenticate(context.Context, string) (auth.Actor, error)
}

func NewSpeechAudioHandler(svc *clipapp.SpeechService, sessions SpeechAudioSessions) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		cookie, e := r.Cookie(auth.SessionCookieName)
		if e != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		actor, e := sessions.Authenticate(r.Context(), cookie.Value)
		if e != nil || actor.UserID == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, SpeechAudioPath)
		if len(id) != 32 || strings.Contains(id, "/") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		data, e := svc.ReadSpeechPlayback(ctx, actor.UserID, id)
		if e != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Header().Set("Content-Disposition", "inline")
		_, _ = w.Write(data)
	})
}
func (h *Handler) GetClipSpeechAccess(ctx context.Context, r *connect.Request[v1.GetClipSpeechAccessRequest]) (*connect.Response[v1.ClipSpeechAccess], error) {
	owner, e := actingUser(ctx)
	if e != nil {
		return nil, e
	}
	if h.speech == nil {
		return nil, toConnectError(clip.ErrCompositionUnavailable)
	}
	a, e := h.speech.SpeechAccess(ctx, owner, r.Msg.ProjectId, r.Msg.AssetId)
	if e != nil {
		return nil, toConnectError(e)
	}
	return connect.NewResponse(&v1.ClipSpeechAccess{Url: SpeechAudioPath + a.ID, ExpiresAt: a.ExpiresAt.UTC().Format(time.RFC3339Nano), AudioHash: a.AudioHash, Bytes: a.Bytes}), nil
}
