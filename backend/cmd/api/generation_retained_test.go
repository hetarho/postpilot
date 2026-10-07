package main

import (
	"testing"
	"time"

	"github.com/postpilot/backend/internal/generation"
	"github.com/postpilot/backend/internal/llm"
)

func TestRetainedUnresolvedComparisonDoesNotBlockOrdinaryGeneration(t *testing.T) {
	models := &recordingModels{}
	h := newDrainHarness(t, models)
	source := h.draft(t, "")
	_, err := h.handle.Writer.ExecContext(h.ctx, `INSERT INTO model_experiments(id,user_id,post_slug,voice_id,stage,status,input_hash,prompt_version,created_at,origin,source,review_mode) VALUES('retained','alice',?,?,'write','review','frozen-input','old-version',?,'editor','post','candidate_ranking')`, source.Slug, h.voiceID, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		t.Fatal(err)
	}
	id, err := h.generation.Start(h.ctx, generation.StartRequest{UserID: "alice", PostSlug: source.Slug, WriteModel: llm.ModelRef{ProviderID: "p", ModelID: "writer"}.String()})
	if err != nil {
		t.Fatal("retained comparison blocked generation", err)
	}
	h.waitDone(id)
	var status, hash string
	if err = h.handle.Reader.QueryRowContext(h.ctx, "SELECT status,input_hash FROM model_experiments WHERE id='retained'").Scan(&status, &hash); err != nil || status != "review" || hash != "frozen-input" {
		t.Fatal("ordinary writing changed retained evidence", err)
	}
}
