package main

import (
	"context"
	"github.com/postpilot/backend/internal/modelcatalog"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/voice/spoken"
)

type spokenProfiles struct{ catalog *modelcatalog.SpeechService }

func (a spokenProfiles) ResolveSpokenProfile(ctx context.Context, owner string, tier plan.Plan, id string, revision int64, session string) (spoken.Profile, error) {
	p, err := a.catalog.ResolveSpeechProfile(ctx, owner, tier, id, revision, session, false)
	if err != nil {
		return spoken.Profile{}, err
	}
	b := p.Binding
	return spoken.Profile{ConnectionScope: b.ConnectionScope, ID: p.ID, Revision: p.Revision, Design: b.Design, Synthesis: b.Synthesis, DesignLabel: b.DesignModel.Label, SpeechLabel: b.SpeechModel.Label, Grade: string(p.Level), Settings: b.Settings, DescriptionMax: b.DescriptionMax, PreviewMax: b.PreviewMax, SpeechMax: b.SpeechMax, OutputFormat: b.OutputFormat}, nil
}
