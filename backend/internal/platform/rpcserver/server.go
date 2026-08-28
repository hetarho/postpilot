// Package rpcserver is server plumbing only: it wires Connect handlers onto a
// net/http mux with h2c (cleartext HTTP/2) and CORS, plus a plain /health endpoint.
// It holds NO business logic — service implementations and interceptors live in their
// own packages and are injected into New by the composition root (cmd/api).
package rpcserver

import (
	"encoding/json"
	"net/http"
	"time"

	"connectrpc.com/connect"
	connectcors "connectrpc.com/cors"
	"github.com/rs/cors"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/postpilot/backend/internal/platform/config"
)

// maxRequestBytes caps a single unary request message. Raise it deliberately when
// image upload lands — until then a generous cap only bounds cheap large-POST abuse.
const maxRequestBytes = 256 << 10 // 256 KiB

// Server hardening timeouts: ReadTimeout bounds slow-body uploads (ReadHeaderTimeout
// covers headers only), WriteTimeout bounds slow readers, IdleTimeout reaps abandoned
// keep-alive connections. Unary handlers finish in milliseconds, so generous values
// bound abuse, not real traffic.
const (
	readTimeout  = 30 * time.Second
	writeTimeout = 30 * time.Second
	idleTimeout  = 120 * time.Second
)

// Registrar mounts one Connect service, given the shared handler options. It matches
// the shape of the generated NewXxxServiceHandler constructors, so a context registers
// itself with a one-line closure and this package never imports the generated code.
type Registrar func(opts ...connect.HandlerOption) (path string, handler http.Handler)

// Options is what the composition root injects.
type Options struct {
	// Interceptors run for every registered procedure, outermost first.
	Interceptors []connect.Interceptor
	// Handlers are the Connect services to mount.
	Handlers []Registrar
}

// New builds the fully-wired HTTP server: the given Connect services plus a /health
// endpoint, wrapped in CORS and h2c. The caller owns the listen/shutdown lifecycle.
func New(cfg *config.Config, version string, opts Options) *http.Server {
	mux := http.NewServeMux()

	handlerOpts := []connect.HandlerOption{
		connect.WithReadMaxBytes(maxRequestBytes),
	}
	if len(opts.Interceptors) > 0 {
		handlerOpts = append(handlerOpts, connect.WithInterceptors(opts.Interceptors...))
	}
	for _, register := range opts.Handlers {
		mux.Handle(register(handlerOpts...))
	}

	// /health is mounted directly on the mux, so it bypasses the Connect stack — and
	// with it the auth interceptor. It is what the platform (Caddy, uptime checks,
	// the deploy's rollback gate) probes, not an RPC.
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":  "ok",
			"version": version,
		})
	})

	// h2c wraps the outermost handler so the cleartext HTTP/2 upgrade applies to all
	// traffic (TLS is terminated at the edge Caddy). Read/WriteTimeout DO reach h2c
	// streams (x/net arms per-stream deadlines from the BaseConfig *http.Server) — only
	// IdleTimeout doesn't propagate, so the http2.Server mirrors it.
	root := h2c.NewHandler(withCORS(mux, cfg.CORSOrigin), &http2.Server{
		IdleTimeout: idleTimeout,
	})

	return &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           root,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}
}

// withCORS allows exactly the configured browser origin, with credentials.
//
// AllowCredentials is what lets the browser send the HttpOnly session cookie
// cross-origin (the SPA and the API are different origins). It is also why the origin
// can never be a wildcard — the browser rejects that combination outright, so
// config.Load refuses to start with one.
func withCORS(h http.Handler, origin string) http.Handler {
	return cors.New(cors.Options{
		AllowedOrigins:   []string{origin},
		AllowCredentials: true,
		AllowedMethods:   connectcors.AllowedMethods(),
		AllowedHeaders:   connectcors.AllowedHeaders(),
		ExposedHeaders:   connectcors.ExposedHeaders(),
		MaxAge:           7200, // cache preflight (seconds)
	}).Handler(h)
}
