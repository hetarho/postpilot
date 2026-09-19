// Package health implements HealthService — the liveness slice of the transport
// contract. It has no dependencies beyond the generated types on purpose: it is the
// end-to-end wire test (browser → Connect → Go), not a product feature.
package health

import (
	"context"

	"connectrpc.com/connect"

	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
)

// Handler implements postpilotv1connect.HealthServiceHandler.
type Handler struct {
	version string
}

// NewHandler returns the HealthService implementation reporting the given build version.
func NewHandler(version string) *Handler {
	return &Handler{version: version}
}

// Ping answers with a fixed greeting and the server build version.
func (h *Handler) Ping(_ context.Context, _ *connect.Request[postpilotv1.PingRequest]) (*connect.Response[postpilotv1.PingResponse], error) {
	return connect.NewResponse(&postpilotv1.PingResponse{
		Message: "pong",
		Version: h.version,
	}), nil
}
