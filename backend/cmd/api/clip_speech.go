package main

import (
	"context"
	"database/sql"
	"github.com/postpilot/backend/internal/clip"
	clipapp "github.com/postpilot/backend/internal/clip/app"
	clipstore "github.com/postpilot/backend/internal/clip/store"
	jobstore "github.com/postpilot/backend/internal/job/store"
	modelcatalogapp "github.com/postpilot/backend/internal/modelcatalog/app"
	"github.com/postpilot/backend/internal/plan"
)

type clipSpeechVoices struct {
	voices   clipSpokenVoices
	profiles spokenProfiles
}

func (a clipSpeechVoices) ResolveSpeechVoice(ctx context.Context, owner string, tier plan.Plan, id string) (clipapp.SpeechVoice, error) {
	v, e := a.voices.library.GetVoice(ctx, owner, id)
	if e != nil {
		return clipapp.SpeechVoice{}, e
	}
	binding, e := a.voices.ResolveClipVoice(ctx, owner, id)
	if e != nil {
		return clipapp.SpeechVoice{}, e
	}
	current, e := a.profiles.ResolveSpokenProfile(ctx, owner, tier, v.Profile.ID, v.Profile.Revision, "")
	if e != nil {
		return clipapp.SpeechVoice{}, e
	}
	if current != v.Profile {
		return clipapp.SpeechVoice{}, clip.ErrPlanConflict
	}
	return clipapp.SpeechVoice{Binding: binding, ProfileID: current.ID, ProfileRevision: current.Revision, Model: current.Synthesis, Handle: v.Handle, Settings: current.Settings}, nil
}

type clipSpeechTransactions struct{ writer *sql.DB }

func (a clipSpeechTransactions) WriteSpeech(ctx context.Context, fn func(clipapp.SpeechTxPorts) error) error {
	tx, e := a.writer.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if e = fn(clipapp.SpeechTxPorts{Store: clipstore.NewTx(tx), Jobs: jobstore.NewTx(tx, jobKinds())}); e != nil {
		return e
	}
	return tx.Commit()
}
func newClipSpeech(c *contexts) *clipapp.SpeechService {
	p := c.platform
	return clipapp.NewSpeechService(clipapp.SpeechDeps{Store: c.clipStore, Voices: clipSpeechVoices{clipSpokenVoices{c.spoken}, spokenProfiles{p.speechCatalog}}, Plans: c.auth, Prices: modelcatalogapp.SpeechBudgets{Profiles: p.speechCatalog}, Ledger: c.ledger, Models: c.metered, Objects: p.bucket, Queue: c.jobs, Transactions: clipSpeechTransactions{p.db.Writer}})
}
