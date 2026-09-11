package main

import (
	"context"
	"errors"
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/job"
	jobstore "github.com/postpilot/backend/internal/job/store"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/usage"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestClipMeteringFailsClosedBeforeProviderOrLedger(t *testing.T) {
	for _, kind := range []string{job.KindGenerateClip, job.KindRenderClip} {
		ctx := usage.WithWork(context.Background(), usage.Work{UserID: "alice", JobID: "clip", Kind: kind})
		// Nil dependencies intentionally panic if the fail-closed guard ever runs too late.
		_, err := (meteredRegistry{}).Complete(ctx, llm.ModelRef{ProviderID: "p", ModelID: "o"}, llm.Request{MaxTokens: 8192})
		if !errors.Is(err, job.ErrCreditAllowance) {
			t.Fatal(err)
		}
	}
}

type clipMeterAdmission struct{}

func (clipMeterAdmission) Hold(context.Context, job.Start) error       { return nil }
func (clipMeterAdmission) Release(context.Context, string)             {}
func (clipMeterAdmission) Settle(context.Context, string, string)      {}
func (clipMeterAdmission) OpenHolds(context.Context) ([]string, error) { return nil, nil }

func TestClipMeteringRequiresTheCompleteAdmittedExecutionPolicy(t *testing.T) {
	for _, mode := range []string{"missing policy", "fallback", "parameters", "ref", "stage", "budget", "reasoning", "reasoning disabled", "fingerprint", "modality price", "delivery", "extra call", "missing work"} {
		t.Run(mode, func(t *testing.T) {
			d, err := db.Open(filepath.Join(t.TempDir(), "meter.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer d.Close()
			ctx := t.Context()
			if err = db.Migrate(ctx, d.Writer); err != nil {
				t.Fatal(err)
			}
			now := time.Now().UTC().Format(time.RFC3339Nano)
			if _, err = d.Writer.Exec("INSERT INTO users(id,password_hash,created_at) VALUES ('alice','hash',?)", now); err != nil {
				t.Fatal(err)
			}
			if _, err = d.Writer.Exec("INSERT INTO clip_projects(id,user_id,title,ratio,target_duration_ms,created_at,updated_at) VALUES ('clip','alice','test','square',15000,?,?)", now, now); err != nil {
				t.Fatal(err)
			}
			st := jobstore.New(d.Writer, d.Reader)
			q := job.New(st, time.Millisecond)
			q.Admit(clipMeterAdmission{})
			id, err := q.Enqueue(ctx, job.NewJob{UserID: "alice", ClipProjectID: "clip", Kind: job.KindGenerateClip, ObserveModel: "p/o", WriteModel: "p/w"})
			if err != nil {
				t.Fatal(err)
			}
			if err = q.ActivateClip(ctx, "alice", id); err != nil {
				t.Fatal(err)
			}
			if _, err = st.PickNextQueued(ctx, time.Now()); err != nil {
				t.Fatal(err)
			}
			p := llm.CallPolicy{Ref: llm.ModelRef{ProviderID: "p", ModelID: "o"}, Stage: "observe", CompletionTokens: 8192, InputUSDPerMillion: "1", OutputUSDPerMillion: "2", Pricing: llm.CallPricing{Version: llm.CallPricingVersion, Fingerprint: strings.Repeat("a", 64), Delivery: llm.ExecutionInlineStatic, Endpoint: "leaf", RequiredParameters: "max_tokens", PromptUSDPerMillion: "1", CompletionUSDPerMillion: "2", RequestUSD: "0", ImageUSD: "0.000001", AudioUSDPerToken: "0.000001"}}
			w := p
			w.Ref.ModelID = "w"
			w.Stage = "write"
			w.CompletionTokens = 32768
			w.Pricing.Delivery = llm.ExecutionTextOnly
			ctx, err = (clipJobs{queue: q}).ReserveApproved(ctx, "alice", id, clip.GenerationApproval{QuoteID: "quote", MaxCredits: 100, Pricing: clip.GenerationPricing{Version: clip.PricingPolicyVersion, Observe: p, Plan: w, ObservationCalls: 1, MaxCredits: 100}}, 1)
			if err != nil {
				t.Fatal(err)
			}
			ctx = usage.WithWork(ctx, usage.Work{UserID: "alice", JobID: id, Kind: job.KindGenerateClip})
			ref := p.Ref
			r := llm.Request{Stage: p.Stage, MaxTokens: p.CompletionTokens, Execution: &llm.ExecutionPolicy{Call: p, Delivery: llm.ExecutionInlineStatic, NoFallback: true, RequireParameters: true}, Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.InlineVideoPart(llm.InlineVideo{MIME: "video/mp4", Size: 3, DurationMS: 1000, Sampling: llm.VideoSamplingFixed, Open: func(context.Context) (io.ReadCloser, error) { t.Fatal("guard opened media"); return nil, nil }})}}}}
			switch mode {
			case "missing policy":
				r.Execution = nil
			case "fallback":
				r.Execution.NoFallback = false
			case "parameters":
				r.Execution.RequireParameters = false
			case "ref":
				ref.ModelID = "changed"
			case "stage":
				r.Stage = "write"
			case "budget":
				r.MaxTokens++
			case "reasoning":
				r.Reasoning = llm.ReasoningHigh
			case "reasoning disabled":
				r.DisableReasoning = true
			case "fingerprint":
				r.Execution.Call.Pricing.Fingerprint = strings.Repeat("b", 64)
			case "modality price":
				r.Execution.Call.Pricing.ImageUSD = "0"
			case "delivery":
				r.Execution.Delivery = llm.ExecutionTextOnly
			case "extra call":
				if _, err = job.ConsumeClipPolicy(ctx, "alice", id, ref.String(), 8192, "observe"); err != nil {
					t.Fatal(err)
				}
			case "missing work":
				ctx = t.Context()
			}
			// Nil dependencies prove all guard failures happen before registry,
			// provider, media reading or usage writes, not after a paid operation.
			if _, err = (meteredRegistry{}).Complete(ctx, ref, r); !errors.Is(err, job.ErrCreditAllowance) {
				t.Fatal(err)
			}
		})
	}
}
