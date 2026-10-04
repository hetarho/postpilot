package rpc

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/voice/spoken"
)

const SamplePath = "/spoken/audio/"

type SessionReader interface {
	Authenticate(context.Context, string) (auth.Actor, error)
}

func NewAudioHandler(svc *spoken.Service, sessions SessionReader) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		c, err := r.Cookie(auth.SessionCookieName)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		caller, err := sessions.Authenticate(r.Context(), c.Value)
		if err != nil || caller.UserID == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, SamplePath)
		if len(id) != 32 || strings.Contains(id, "/") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		data, err := svc.ReadPlayback(ctx, caller.UserID, id)
		if err != nil {
			status := http.StatusServiceUnavailable
			if errors.Is(err, spoken.ErrNotFound) || errors.Is(err, spoken.ErrConflict) {
				status = http.StatusNotFound
			}
			w.WriteHeader(status)
			return
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Header().Set("Content-Disposition", "inline")
		_, _ = w.Write(data)
	})
}
