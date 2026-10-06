package analysisquality

import (
	"context"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/ai"
	"github.com/postpilot/backend/internal/llm"
)

type completionSpy struct {
	policy   llm.CallPolicy
	usage    llm.Usage
	err      error
	raws     []string
	requests []llm.Request
}

func (s *completionSpy) Resolve(ref llm.ModelRef) (llm.ModelInfo, bool) {
	return llm.ModelInfo{Vision: true, VideoInput: true, StructuredOutput: true, Stages: []string{llm.StageNameObserve}}, ref == s.policy.Ref
}
func (s *completionSpy) Complete(_ context.Context, _ llm.ModelRef, r llm.Request) (llm.Response, error) {
	s.requests = append(s.requests, r)
	raw := rawObservation("8,900원", "")
	if len(s.requests) <= len(s.raws) {
		raw = s.raws[len(s.requests)-1]
	}
	return llm.Response{Text: raw, Usage: s.usage, FinishReason: "stop"}, s.err
}
func productionInput(c Corpus) clip.ChunkInput {
	in := c.Inputs[0]
	return clip.ChunkInput{Source: c.Cases[0].Source, Language: c.Language, Index: in.Copy.Index, OffsetMS: in.Copy.OffsetMS, DurationMS: in.Copy.DurationMS, Policy: c.Policy, Video: llm.InlineVideo{MIME: "video/mp4", Size: 5, DurationMS: 5000, Sampling: llm.VideoSamplingFixed, Open: func(context.Context) (io.ReadCloser, error) { return io.NopCloser(strings.NewReader("proxy")), nil }}}
}
func capture(t *testing.T, c Corpus, raws []string) []llm.Request {
	t.Helper()
	s := &completionSpy{policy: c.Policy, usage: llm.Usage{CostReported: true, CostMicrousd: 10}, raws: raws}
	service, e := ai.New(s, ai.DefaultConfig(clip.Environment{}))
	if e != nil {
		t.Fatal(e)
	}
	if _, _, e = service.ObserveChunk(t.Context(), c.Policy.Ref, productionInput(c)); e != nil {
		t.Fatal(e)
	}
	return s.requests
}
func approval(t *testing.T, c Corpus, root string, now time.Time, requests []llm.Request) Approval {
	t.Helper()
	upper, ok := c.Policy.QuoteMicrousd()
	if !ok {
		t.Fatal("bad fixture policy")
	}
	maximum := upper * int64(len(requests))
	a := Approval{SessionID: "synthetic_session", StatePath: filepath.Join(root, "session.json"), ApprovedBy: "synthetic operator", EvidenceDigest: hash([]byte("explicit synthetic approval")), PlanDigest: planDigest(c), PolicyDigest: digest(c.Policy), ExpiresAt: now.Add(time.Hour), MaximumMicrousd: &maximum}
	for i, r := range requests {
		a.Calls = append(a.Calls, ApprovedCall{ID: strings.ReplaceAll(key("call", i), "/", "-"), RequestDigest: RequestDigest(r)})
	}
	return a
}
func openSpy(t *testing.T, c Corpus, root string, now time.Time, a Approval, s *completionSpy) (*BudgetedModels, *int, error) {
	t.Helper()
	factories := 0
	b, e := OpenBudgetedModels(t.Context(), filepath.Join(root, "session.json"), a, c.Policy, now, func(context.Context, Approval) error { return nil }, func() (ai.Models, error) { factories++; return s, nil })
	return b, &factories, e
}

func TestBudgetBadApprovalAndRequestConstructNoDependency(t *testing.T) {
	for _, bad := range []string{"missing_cap", "expired", "policy_digest", "absent_admission", "invalid_call", "overflow_quote", "unapproved_path", "insufficient_cap"} {
		t.Run(bad, func(t *testing.T) {
			c, root, now := fixture(t)
			request := capture(t, c, nil)[0]
			a := approval(t, c, root, now, []llm.Request{request})
			admit := func(context.Context, Approval) error { return nil }
			switch bad {
			case "missing_cap":
				a.MaximumMicrousd = nil
			case "expired":
				a.ExpiresAt = now
			case "policy_digest":
				a.PolicyDigest = strings.Repeat("b", 64)
			case "absent_admission":
				admit = nil
			case "invalid_call":
				a.Calls[0].ID = "../call"
			case "overflow_quote":
				c.Policy.InputUSDPerMillion = "1e60"
				a.PolicyDigest = digest(c.Policy)
			case "unapproved_path":
				a.StatePath = filepath.Join(root, "other.json")
			case "insufficient_cap":
				v := int64(0)
				a.MaximumMicrousd = &v
			}
			factories := 0
			b, e := OpenBudgetedModels(t.Context(), filepath.Join(root, "session.json"), a, c.Policy, now, admit, func() (ai.Models, error) { factories++; panic("network-capable factory on refusal") })
			if e == nil || b != nil || factories != 0 {
				t.Fatal("bad approval constructed dependency", e)
			}
		})
	}
	c, root, now := fixture(t)
	req := capture(t, c, nil)[0]
	a := approval(t, c, root, now, []llm.Request{req})
	s := &completionSpy{policy: c.Policy, usage: llm.Usage{CostReported: true, CostMicrousd: 10}}
	b, _, e := openSpy(t, c, root, now, a, s)
	if e != nil {
		t.Fatal(e)
	}
	defer b.Close()
	changed := req
	changed.System += "changed"
	if _, e = b.Complete(t.Context(), c.Policy.Ref, changed); !errors.Is(e, ErrBudget) || len(s.requests) != 0 {
		t.Fatal("unapproved request entered model boundary", e)
	}
}

func TestBudgetCumulativeSpendLockRestartAndExplicitZero(t *testing.T) {
	c, root, now := fixture(t)
	req := capture(t, c, nil)[0]
	a := approval(t, c, root, now, []llm.Request{req, req})
	upper, _ := c.Policy.QuoteMicrousd()
	s := &completionSpy{policy: c.Policy, usage: llm.Usage{CostReported: true, CostMicrousd: upper}}
	b, _, e := openSpy(t, c, root, now, a, s)
	if e != nil {
		t.Fatal(e)
	}
	if _, factories, e := openSpy(t, c, root, now, a, s); e == nil || *factories != 0 {
		t.Fatal("parallel process acquired whole cap")
	}
	if _, e = b.Complete(t.Context(), c.Policy.Ref, req); e != nil || b.state.ConfirmedMeasuredSpend != upper {
		t.Fatal("known cost not measured", e)
	}
	if e = b.Close(); e != nil {
		t.Fatal(e)
	}
	b, _, e = openSpy(t, c, root, now, a, s)
	if e != nil {
		t.Fatal("clean restart lost cumulative spend", e)
	}
	if _, e = b.Complete(t.Context(), c.Policy.Ref, req); e != nil || b.state.ConfirmedMeasuredSpend != *a.MaximumMicrousd {
		t.Fatal(e)
	}
	if _, e = b.Complete(t.Context(), c.Policy.Ref, req); !errors.Is(e, ErrBudget) || len(s.requests) != 2 {
		t.Fatal("exhausted matrix/cap issued further request", e)
	}
	b.Close()
	if _, factories, e := openSpy(t, c, root, now, a, s); e == nil || *factories != 0 {
		t.Fatal("exhausted restart constructed model dependency")
	}
	t.Run("explicit_zero", func(t *testing.T) {
		c, root, now := fixture(t)
		c.Policy.InputUSDPerMillion = "0"
		c.Policy.OutputUSDPerMillion = "0"
		c.Policy.Pricing.PromptUSDPerMillion = "0"
		c.Policy.Pricing.CompletionUSDPerMillion = "0"
		req := capture(t, c, nil)[0]
		a := approval(t, c, root, now, []llm.Request{req})
		s := &completionSpy{policy: c.Policy, usage: llm.Usage{CostReported: true}}
		b, _, e := openSpy(t, c, root, now, a, s)
		if e != nil {
			t.Fatal(e)
		}
		defer b.Close()
		if _, e = b.Complete(t.Context(), c.Policy.Ref, req); e != nil || b.state.Calls[0].MeasuredCost == nil || *b.state.Calls[0].MeasuredCost != 0 {
			t.Fatal("explicit reported zero became missing", e)
		}
	})
}

func TestBudgetFailedUnknownAboveBoundAndWriteFailuresHalt(t *testing.T) {
	for _, failure := range []string{"paid_error", "absent_cost", "above_bound", "negative_usage", "sum_overflow", "write_before", "write_after", "lost_lock", "expiry"} {
		t.Run(failure, func(t *testing.T) {
			c, root, now := fixture(t)
			req := capture(t, c, nil)[0]
			a := approval(t, c, root, now, []llm.Request{req, req})
			upper, _ := c.Policy.QuoteMicrousd()
			s := &completionSpy{policy: c.Policy, usage: llm.Usage{CostReported: true, CostMicrousd: 10}}
			if failure == "paid_error" {
				s.err = llm.ErrRateLimited
			}
			if failure == "absent_cost" {
				s.usage = llm.Usage{}
			}
			if failure == "above_bound" {
				s.usage.CostMicrousd = upper + 1
			}
			if failure == "negative_usage" {
				s.usage.PromptTokens = -1
			}
			if failure == "sum_overflow" {
				s.usage.CostMicrousd = math.MaxInt64
			}
			b, _, e := openSpy(t, c, root, now, a, s)
			if e != nil {
				t.Fatal(e)
			}
			if failure == "write_before" {
				b.persist = func(BudgetState) error { return ErrOutput }
			}
			if failure == "write_after" {
				writes := 0
				b.persist = func(state BudgetState) error {
					writes++
					if writes > 1 {
						return ErrOutput
					}
					return b.writeState(state)
				}
			}
			if failure == "lost_lock" {
				os.Remove(b.lockPath)
			}
			if failure == "expiry" {
				b.now = func() time.Time { return a.ExpiresAt }
			}
			if failure == "sum_overflow" {
				b.state.ConfirmedMeasuredSpend = 1
			}
			if _, e = b.Complete(t.Context(), c.Policy.Ref, req); e == nil {
				t.Fatal("failed/unknown accounting continued")
			}
			calls := len(s.requests)
			if _, e = b.Complete(t.Context(), c.Policy.Ref, req); e == nil || len(s.requests) != calls {
				t.Fatal("failure sent an additional request", e)
			}
			if failure == "paid_error" && b.state.ConfirmedMeasuredSpend != 10 {
				t.Fatal("failed paid response erased cost")
			}
			if failure == "absent_cost" && (b.state.ConfirmedMeasuredSpend != 0 || b.state.UnresolvedWorstCase != upper || b.state.Calls[0].MeasuredCost != nil) {
				t.Fatal("missing cost converted into measured zero")
			}
			if failure == "write_before" || failure == "lost_lock" || failure == "expiry" {
				if calls != 0 {
					t.Fatal("preflight refusal called downstream")
				}
			}
			b.Close()
			if failure == "absent_cost" || failure == "write_after" || failure == "paid_error" || failure == "above_bound" {
				if _, factories, e := openSpy(t, c, root, now, a, s); e == nil || *factories != 0 {
					t.Fatal("uncertain/failed restart rebuilt provider dependency")
				}
			}
		})
	}
}

func TestProductionResponseCorrectionMetersEachUnderlyingCompletion(t *testing.T) {
	c, root, now := fixture(t)
	c.Policy.ResponseRetries = 3
	raws := []string{`{}`, rawObservation("8,900원", "")}
	requests := capture(t, c, raws)
	if len(requests) != 2 {
		t.Fatal("production correction fixture did not correct once")
	}
	a := approval(t, c, root, now, requests)
	s := &completionSpy{policy: c.Policy, usage: llm.Usage{CostReported: true, CostMicrousd: 10}, raws: raws}
	b, _, e := openSpy(t, c, root, now, a, s)
	if e != nil {
		t.Fatal(e)
	}
	defer b.Close()
	service, e := ai.New(b, ai.DefaultConfig(clip.Environment{}))
	if e != nil {
		t.Fatal(e)
	}
	got, usage, e := service.ObserveChunk(t.Context(), c.Policy.Ref, productionInput(c))
	if e != nil || got.Segments[0].StartMS != 60000 || usage.CostMicrousd != 20 || len(s.requests) != 2 || len(b.state.Calls) != 2 || b.state.ConfirmedMeasuredSpend != 20 {
		t.Fatal("underlying correction cost/slot lost", e, usage, b.state)
	}
	if _, e = b.Complete(t.Context(), c.Policy.Ref, requests[1]); !errors.Is(e, ErrBudget) || len(s.requests) != 2 {
		t.Fatal("unplanned additional correction sent")
	}
}

func TestBudgetOwnsImmutableApprovalAcrossCallerAndCallbackMutation(t *testing.T) {
	c, root, now := fixture(t)
	req := capture(t, c, nil)[0]
	changed := req
	changed.System += "unapproved substitution"
	a := approval(t, c, root, now, []llm.Request{req})
	originalDigest := digest(a)
	originalCap := *a.MaximumMicrousd
	s := &completionSpy{policy: c.Policy, usage: llm.Usage{CostReported: true, CostMicrousd: 10}}
	b, e := OpenBudgetedModels(t.Context(), a.StatePath, a, c.Policy, now, func(_ context.Context, received Approval) error {
		received.Calls[0].RequestDigest = RequestDigest(changed)
		*received.MaximumMicrousd = math.MaxInt64
		return nil
	}, func() (ai.Models, error) { return s, nil })
	if e != nil {
		t.Fatal(e)
	}
	defer b.Close()
	a.Calls[0].RequestDigest = RequestDigest(changed)
	*a.MaximumMicrousd = math.MaxInt64
	if _, e = b.Complete(t.Context(), c.Policy.Ref, changed); !errors.Is(e, ErrBudget) || len(s.requests) != 0 {
		t.Fatal("caller/callback changed frozen request", e)
	}
	if digest(b.approval) != originalDigest || b.state.ApprovalDigest != originalDigest || *b.approval.MaximumMicrousd != originalCap {
		t.Fatal("approval digest/cap mutated")
	}
	if _, e = b.Complete(t.Context(), c.Policy.Ref, req); e != nil || len(s.requests) != 1 {
		t.Fatal("unchanged original approval stopped working", e)
	}
}
