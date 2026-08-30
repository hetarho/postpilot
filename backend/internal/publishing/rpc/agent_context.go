package rpc

import (
	"context"

	"github.com/postpilot/backend/internal/publishing"
)

type agentKeyType struct{}

var agentKey agentKeyType

func withAgent(ctx context.Context, agent publishing.Agent) context.Context {
	return context.WithValue(ctx, agentKey, agent)
}

func agentFromContext(ctx context.Context) (publishing.Agent, bool) {
	agent, ok := ctx.Value(agentKey).(publishing.Agent)
	return agent, ok && agent.ID != ""
}
